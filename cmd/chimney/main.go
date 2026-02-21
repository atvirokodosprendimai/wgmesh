// chimney is the origin server for the wgmesh dashboard at chimney.beerpub.dev.
//
// It serves the static dashboard HTML and provides a caching proxy for the
// GitHub REST API. Server-side caching with an authenticated GitHub token
// gives us 5,000 req/hr instead of 60 req/hr unauthenticated, and the proxy
// returns ETag-aware responses so edge Caddy servers can cache efficiently.
//
// Cache layer: Dragonfly (Redis-compatible) as shared persistent cache,
// with in-memory fallback if Dragonfly is unavailable.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/codes"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	githubAPI     = "https://api.github.com"
	defaultRepo   = "atvirokodosprendimai/wgmesh"
	maxCacheSize  = 500
	clientTimeout = 10 * time.Second
	redisTimeout  = 200 * time.Millisecond
	cachePrefix   = "chimney:"
)

// version and wgmeshVersion are set at build time via -ldflags.
var (
	version       = "dev"
	wgmeshVersion = "unknown"

	startTime = time.Now() // reserved for Phase 3 metrics — chimney.uptime gauge
)

// cachedResponse is the JSON-serializable form stored in Dragonfly.
type cachedResponse struct {
	Body       []byte            `json:"body"`
	ETag       string            `json:"etag"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	FetchedAt  time.Time         `json:"fetched_at"`
}

// memEntry is the in-memory fallback cache entry.
type memEntry struct {
	data      cachedResponse
	fetchedAt time.Time
}

var (
	// In-memory fallback cache (used when Dragonfly is unavailable)
	memCache   = make(map[string]*memEntry)
	memCacheMu sync.RWMutex

	githubToken string
	repo        string

	httpClient = &http.Client{Timeout: clientTimeout}

	rdb           *redis.Client
	useRedis      atomic.Bool // true once Dragonfly is connected; set from background goroutine
	redisConnDone atomic.Bool // true once the connect goroutine has finished (success or fail)
	redisAddr     string
	cacheHits     int64
	cacheMisses   int64
	counterMu     sync.Mutex
)

func main() {
	addr := flag.String("addr", ":8080", "Listen address")
	docsDir := flag.String("docs", "./docs", "Path to static dashboard files")
	flag.StringVar(&repo, "repo", defaultRepo, "GitHub owner/repo")
	flag.StringVar(&redisAddr, "redis", "127.0.0.1:6379", "Dragonfly/Redis address")
	flag.Parse()

	// Set up OTEL telemetry providers. Non-fatal: if the collector is unreachable
	// at startup the exporters will keep retrying in the background.
	// Shutdown is called explicitly in the signal handler below.
	otelShutdown := func(context.Context) error { return nil } // no-op default
	if fn, err := otelSetup(context.Background()); err != nil {
		log.Printf("WARNING: OTEL setup failed: %v — telemetry disabled", err)
	} else {
		otelShutdown = fn
	}

	rawToken := os.Getenv("GITHUB_TOKEN")
	githubToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		log.Println("WARNING: GITHUB_TOKEN not set — using unauthenticated API (60 req/hr)")
	} else if githubToken == "" {
		log.Println("WARNING: GITHUB_TOKEN is empty or whitespace — using unauthenticated API")
	} else {
		log.Println("GitHub token configured — 5,000 req/hr")
	}

	// Connect to Dragonfly/Redis asynchronously so the HTTP server starts
	// immediately (allowing /healthz to respond during Dragonfly startup).
	// Retries up to 30 times with 1s backoff (~30s window) before giving up.
	rdb = redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		DB:           0,
		ReadTimeout:  redisTimeout,
		WriteTimeout: redisTimeout,
		DialTimeout:  time.Second,
	})

	go func() {
		const redisMaxRetries = 30
		for attempt := 1; attempt <= redisMaxRetries; attempt++ {
			pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
			err := rdb.Ping(pingCtx).Err()
			pingCancel()
			if err == nil {
				log.Printf("Dragonfly connected at %s (attempt %d)", redisAddr, attempt)
				useRedis.Store(true)
				redisConnDone.Store(true)
				return
			}
			if attempt == redisMaxRetries {
				log.Printf("WARNING: Dragonfly not available at %s after %d attempts: %v — using in-memory cache", redisAddr, redisMaxRetries, err)
				redisConnDone.Store(true)
			} else {
				log.Printf("Dragonfly not ready at %s (attempt %d/%d): %v — retrying in 1s", redisAddr, attempt, redisMaxRetries, err)
				time.Sleep(time.Second)
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/github/", handleGitHubProxy)
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/api/cache/stats", handleCacheStats)
	mux.HandleFunc("/api/version", handleVersion)
	mux.HandleFunc("/api/pipeline/summary", handlePipelineSummary)

	fs := http.FileServer(http.Dir(*docsDir))
	mux.Handle("/", fs)

	// traceIDMux injects X-Trace-ID into every response.
	// It runs inside the otelhttp span (which creates the span before calling in),
	// so trace.SpanFromContext already has a valid span ID at this point.
	traceIDMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if span := trace.SpanFromContext(r.Context()); span.SpanContext().IsValid() {
			w.Header().Set("X-Trace-ID", span.SpanContext().TraceID().String())
		}
		mux.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:    *addr,
		Handler: otelhttp.NewHandler(traceIDMux, "chimney"),
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	log.Printf("chimney starting on %s (docs=%s, repo=%s, redis=%s)", *addr, *docsDir, repo, redisAddr)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-sigCh
	log.Println("shutdown: draining in-flight telemetry...")
	{
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := otelShutdown(ctx); err != nil {
			log.Printf("OTEL shutdown: %v", err)
		}
	}
	log.Println("shutdown: stopping HTTP server...")
	{
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown: %v", err)
		}
	}
	log.Println("shutdown: complete")
}

// --- Cache abstraction ---

// cacheGet returns the cached response, whether it was found, and which tier
// served it ("dragonfly", "memory", or "" on miss).
func cacheGet(ctx context.Context, key string) (*cachedResponse, bool, string) {
	if useRedis.Load() {
		data, err := rdb.Get(ctx, cachePrefix+key).Bytes()
		if err == nil {
			var cr cachedResponse
			if json.Unmarshal(data, &cr) == nil {
				return &cr, true, "dragonfly"
			}
		}
		// Dragonfly miss or error — try in-memory fallback
	}

	memCacheMu.RLock()
	entry, found := memCache[key]
	memCacheMu.RUnlock()
	if found {
		return &entry.data, true, "memory"
	}
	return nil, false, ""
}

func cacheSet(ctx context.Context, key string, cr *cachedResponse, ttl time.Duration) {
	// Always store in memory as fallback
	memCacheMu.Lock()
	memCache[key] = &memEntry{data: *cr, fetchedAt: cr.FetchedAt}
	needEvict := len(memCache) > maxCacheSize
	memCacheMu.Unlock()

	if needEvict {
		evictOldestMemEntry()
	}

	if useRedis.Load() {
		data, err := json.Marshal(cr)
		if err != nil {
			log.Printf("cache marshal error: %v", err)
			return
		}
		if err := rdb.Set(ctx, cachePrefix+key, data, ttl).Err(); err != nil {
			log.Printf("Dragonfly SET error (degrading to in-memory): %v", err)
		}
	}
}

func evictOldestMemEntry() {
	memCacheMu.Lock()
	defer memCacheMu.Unlock()

	oldestKey := ""
	oldestTime := time.Now()
	for k, v := range memCache {
		if v.fetchedAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.fetchedAt
		}
	}
	if oldestKey != "" {
		delete(memCache, oldestKey)
	}
}

// --- TTL ---

func ttlForPath(ghPath, rawQuery string) time.Duration {
	if strings.Contains(ghPath, "/actions/runs") {
		return 30 * time.Second
	}
	if strings.Contains(ghPath, "/pulls") && strings.Contains(rawQuery, "state=closed") {
		return 5 * time.Minute
	}
	if strings.Contains(ghPath, "/issues") {
		return 2 * time.Minute
	}
	return 30 * time.Second
}

// --- Handlers ---

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]interface{}{
		"status": "ok",
		"repo":   repo,
	}

	// Report Dragonfly status
	if useRedis.Load() {
		ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
		defer cancel()
		if err := rdb.Ping(ctx).Err(); err != nil {
			resp["dragonfly"] = "error"
			resp["dragonfly_error"] = err.Error()
		} else {
			info, _ := rdb.Info(ctx, "memory").Result()
			dbSize, _ := rdb.DBSize(ctx).Result()
			resp["dragonfly"] = "connected"
			resp["dragonfly_keys"] = dbSize
			// Extract used_memory_human from info
			for _, line := range strings.Split(info, "\n") {
				if strings.HasPrefix(line, "used_memory_human:") {
					resp["dragonfly_memory"] = strings.TrimSpace(strings.TrimPrefix(line, "used_memory_human:"))
					break
				}
			}
		}
	} else if redisConnDone.Load() {
		resp["dragonfly"] = "unavailable"
	} else {
		resp["dragonfly"] = "connecting"
	}

	memCacheMu.RLock()
	resp["mem_cache_entries"] = len(memCache)
	memCacheMu.RUnlock()

	counterMu.Lock()
	resp["cache_hits"] = cacheHits
	resp["cache_misses"] = cacheMisses
	counterMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, fmt.Sprintf("marshal error: %v", err), http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(data); err != nil {
		log.Printf("writing /healthz response: %v", err)
	}
}

func handleGitHubProxy(w http.ResponseWriter, r *http.Request) {
	ghPath := strings.TrimPrefix(r.URL.Path, "/api/github")
	if ghPath == "" {
		ghPath = "/"
	}
	ghURL := fmt.Sprintf("%s/repos/%s%s", githubAPI, repo, ghPath)
	if r.URL.RawQuery != "" {
		ghURL += "?" + r.URL.RawQuery
	}

	cacheKey := ghPath + "?" + r.URL.RawQuery
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("chimney.github_path", ghPath))

	// Check cache
	entry, found, tier := cacheGet(ctx, cacheKey)
	maxAge := ttlForPath(ghPath, r.URL.RawQuery)

	// Client ETag match
	clientETag := r.Header.Get("If-None-Match")
	if found && clientETag != "" && clientETag == entry.ETag {
		span.SetAttributes(
			attribute.Bool("chimney.cache_hit", true),
			attribute.String("chimney.cache_tier", tier),
		)
		counterMu.Lock()
		cacheHits++
		counterMu.Unlock()
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Serve from cache if fresh
	if found && time.Since(entry.FetchedAt) < maxAge {
		span.SetAttributes(
			attribute.Bool("chimney.cache_hit", true),
			attribute.String("chimney.cache_tier", tier),
		)
		counterMu.Lock()
		cacheHits++
		counterMu.Unlock()
		writeResponse(w, entry)
		return
	}

	span.SetAttributes(
		attribute.Bool("chimney.cache_hit", false),
		attribute.String("chimney.cache_tier", "none"),
	)
	counterMu.Lock()
	cacheMisses++
	counterMu.Unlock()

	// Fetch from GitHub — child span covers the upstream HTTP call + body read.
	conditional := found && entry.ETag != ""
	fetchCtx, fetchSpan := otel.Tracer("chimney").Start(ctx, "chimney.github_fetch")
	fetchSpan.SetAttributes(
		attribute.String("github.api.path", ghPath),
		attribute.Bool("github.conditional", conditional),
	)
	defer fetchSpan.End()

	req, err := http.NewRequestWithContext(fetchCtx, "GET", ghURL, nil)
	if err != nil {
		fetchSpan.RecordError(err)
		fetchSpan.SetStatus(codes.Error, "build request")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "chimney/1.0 (beerpub.dev)")
	if githubToken != "" {
		req.Header.Set("Authorization", "Bearer "+githubToken)
	}
	if conditional {
		req.Header.Set("If-None-Match", entry.ETag)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		fetchSpan.RecordError(err)
		fetchSpan.SetStatus(codes.Error, "github fetch")
		// Stale cache fallback
		if found {
			writeResponse(w, entry)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	notModified := resp.StatusCode == http.StatusNotModified
	fetchSpan.SetAttributes(
		attribute.Int("github.api.status_code", resp.StatusCode),
		attribute.Bool("github.not_modified", notModified),
	)
	if resp.StatusCode/100 == 5 {
		fetchSpan.SetStatus(codes.Error, "github upstream error")
	}

	// 304 — serve cached
	if notModified && found {
		// Refresh TTL in Dragonfly without re-fetching body
		entry.FetchedAt = time.Now()
		cacheSet(ctx, cacheKey, entry, maxAge)
		writeResponse(w, entry)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fetchSpan.RecordError(err)
		fetchSpan.SetStatus(codes.Error, "read body")
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	// Build new cache entry
	headers := make(map[string]string)
	for _, h := range []string{"Content-Type", "X-RateLimit-Remaining", "X-RateLimit-Reset"} {
		if v := resp.Header.Get(h); v != "" {
			headers[h] = v
		}
	}

	newEntry := &cachedResponse{
		Body:       body,
		ETag:       resp.Header.Get("ETag"),
		StatusCode: resp.StatusCode,
		Headers:    headers,
		FetchedAt:  time.Now(),
	}

	cacheSet(ctx, cacheKey, newEntry, maxAge)
	writeResponse(w, newEntry)
}

func writeResponse(w http.ResponseWriter, entry *cachedResponse) {
	for k, v := range entry.Headers {
		w.Header().Set(k, v)
	}
	if entry.ETag != "" {
		w.Header().Set("ETag", entry.ETag)
	}
	w.Header().Set("X-Cache-Age", fmt.Sprintf("%.0f", time.Since(entry.FetchedAt).Seconds()))
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(entry.StatusCode)
	if _, err := w.Write(entry.Body); err != nil {
		log.Printf("writing response: %v", err)
	}
}

// handleVersion returns chimney's build version and the wgmesh version it was
// compiled from. The dashboard uses this to show which wgmesh release is live.
func handleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	resp := map[string]string{
		"chimney_version": version,
		"wgmesh_version":  wgmeshVersion,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("writing /api/version: %v", err)
	}
}

// pipelineSummary is the JSON shape returned by /api/pipeline/summary.
type pipelineSummary struct {
	WgmeshVersion    string            `json:"wgmesh_version"`
	OpenIssues       int               `json:"open_issues"`
	OpenPRs          int               `json:"open_prs"`
	LastMergedPR     *mergedPR         `json:"last_merged_pr,omitempty"`
	RecentRuns       []workflowRunInfo `json:"recent_workflow_runs"`
	GooseSuccessRate *float64          `json:"goose_success_rate_pct,omitempty"`
	FetchedAt        string            `json:"fetched_at"`
}

type mergedPR struct {
	Number   int    `json:"number"`
	Title    string `json:"title"`
	SHA      string `json:"sha"`
	MergedAt string `json:"merged_at"`
}

type workflowRunInfo struct {
	WorkflowName string `json:"workflow_name"`
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
	RunAt        string `json:"run_at"`
	URL          string `json:"url"`
}

// handlePipelineSummary fetches a compact view of the wgmesh pipeline state:
// open issue/PR counts, last merged PR, and recent Goose build outcomes.
// Results are cached for 60s so the dashboard can poll without hammering GitHub.
func handlePipelineSummary(w http.ResponseWriter, r *http.Request) {
	const cacheKey = "__pipeline_summary__"
	const cacheTTL = 60 * time.Second

	ctx := r.Context()

	// Serve cached summary if fresh
	if entry, ok, _ := cacheGet(ctx, cacheKey); ok && time.Since(entry.FetchedAt) < cacheTTL {
		counterMu.Lock()
		cacheHits++
		counterMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("X-Cache-Age", fmt.Sprintf("%.0f", time.Since(entry.FetchedAt).Seconds()))
		if _, err := w.Write(entry.Body); err != nil {
			log.Printf("writing /api/pipeline/summary (cached): %v", err)
		}
		return
	}

	counterMu.Lock()
	cacheMisses++
	counterMu.Unlock()

	summary := pipelineSummary{
		WgmeshVersion: wgmeshVersion,
		FetchedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	ghGet := func(path string, target interface{}) error {
		url := fmt.Sprintf("%s/repos/%s%s", githubAPI, repo, path)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "chimney/1.0 (cloudroof.eu)")
		if githubToken != "" {
			req.Header.Set("Authorization", "Bearer "+githubToken)
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", path, err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("GitHub returned %d for %s", resp.StatusCode, path)
		}
		return json.Unmarshal(body, target)
	}

	// Open issues (excludes PRs — GitHub API separates them with ?pulls=false via type param).
	var issueList []struct {
		Number      int       `json:"number"`
		PullRequest *struct{} `json:"pull_request"`
	}
	if err := ghGet("/issues?state=open&per_page=100", &issueList); err != nil {
		log.Printf("pipeline/summary: issues: %v", err)
	} else {
		for _, i := range issueList {
			if i.PullRequest == nil {
				summary.OpenIssues++
			}
		}
	}

	// Open PRs.
	var prList []struct {
		Number int `json:"number"`
	}
	if err := ghGet("/pulls?state=open&per_page=100", &prList); err != nil {
		log.Printf("pipeline/summary: pulls: %v", err)
	} else {
		summary.OpenPRs = len(prList)
	}

	// Last merged PR.
	var closedPRs []struct {
		Number   int    `json:"number"`
		Title    string `json:"title"`
		MergedAt string `json:"merged_at"`
		Head     struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := ghGet("/pulls?state=closed&per_page=10&sort=updated&direction=desc", &closedPRs); err != nil {
		log.Printf("pipeline/summary: closed pulls: %v", err)
	} else {
		for _, pr := range closedPRs {
			if pr.MergedAt != "" {
				summary.LastMergedPR = &mergedPR{
					Number:   pr.Number,
					Title:    pr.Title,
					SHA:      pr.Head.SHA,
					MergedAt: pr.MergedAt,
				}
				break
			}
		}
	}

	// Recent Goose build workflow runs (last 10).
	var runsResp struct {
		WorkflowRuns []struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			CreatedAt  string `json:"created_at"`
			HTMLURL    string `json:"html_url"`
		} `json:"workflow_runs"`
	}
	if err := ghGet("/actions/workflows/goose-build.yml/runs?per_page=10&status=completed", &runsResp); err != nil {
		log.Printf("pipeline/summary: goose runs: %v", err)
	} else {
		var total, successes int
		for _, run := range runsResp.WorkflowRuns {
			summary.RecentRuns = append(summary.RecentRuns, workflowRunInfo{
				WorkflowName: run.Name,
				Status:       run.Status,
				Conclusion:   run.Conclusion,
				RunAt:        run.CreatedAt,
				URL:          run.HTMLURL,
			})
			total++
			if run.Conclusion == "success" {
				successes++
			}
		}
		if total > 0 {
			rate := float64(successes) / float64(total) * 100
			summary.GooseSuccessRate = &rate
		}
	}

	// Encode and cache
	body, err := json.Marshal(summary)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	entry := &cachedResponse{
		Body:       body,
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		FetchedAt:  time.Now(),
	}
	cacheSet(ctx, cacheKey, entry, cacheTTL)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if _, err := w.Write(body); err != nil {
		log.Printf("writing /api/pipeline/summary: %v", err)
	}
}

// --- OTEL setup ---

// otelSetup initialises the three OTEL signal providers (trace, metric, log)
// and registers them as global defaults. The returned shutdown func must be
// called on process exit to flush in-flight telemetry.
//
// Configuration via standard OTEL env vars:
//
//	OTEL_EXPORTER_OTLP_ENDPOINT  default http://localhost:4318
//	OTEL_SERVICE_NAME             default "chimney"
//	OTEL_RESOURCE_ATTRIBUTES      optional extra resource labels
func otelSetup(ctx context.Context) (func(context.Context) error, error) {
	svcName := os.Getenv("OTEL_SERVICE_NAME")
	if svcName == "" {
		svcName = "chimney"
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(attribute.String("service.name", svcName)),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	// Trace provider — batched OTLP HTTP export
	traceExp, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("otel trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Metric provider — 15s push interval
	metricExp, err := otlpmetrichttp.New(ctx)
	if err != nil {
		_ = tp.Shutdown(ctx)
		return nil, fmt.Errorf("otel metric exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp,
			sdkmetric.WithInterval(15*time.Second))),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	// Log provider — batched OTLP HTTP export
	logExp, err := otlploghttp.New(ctx)
	if err != nil {
		_ = tp.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
		return nil, fmt.Errorf("otel log exporter: %w", err)
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(lp)

	shutdown := func(ctx context.Context) error {
		return errors.Join(
			tp.Shutdown(ctx),
			mp.Shutdown(ctx),
			lp.Shutdown(ctx),
		)
	}
	return shutdown, nil
}

func handleCacheStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result := map[string]interface{}{}

	counterMu.Lock()
	result["hits"] = cacheHits
	result["misses"] = cacheMisses
	counterMu.Unlock()

	// In-memory stats
	memCacheMu.RLock()
	memEntries := len(memCache)
	memCacheMu.RUnlock()
	result["mem_entries"] = memEntries

	// Dragonfly stats
	if useRedis.Load() {
		dbSize, err := rdb.DBSize(ctx).Result()
		if err == nil {
			result["dragonfly_keys"] = dbSize
		}

		// List cached keys with TTL
		var cursor uint64
		var keys []string
		for {
			var batch []string
			var err error
			batch, cursor, err = rdb.Scan(ctx, cursor, cachePrefix+"*", 100).Result()
			if err != nil {
				break
			}
			keys = append(keys, batch...)
			if cursor == 0 {
				break
			}
		}

		type keyDetail struct {
			Key string `json:"key"`
			TTL string `json:"ttl"`
		}
		details := make([]keyDetail, 0, len(keys))
		for _, k := range keys {
			ttl, _ := rdb.TTL(ctx, k).Result()
			details = append(details, keyDetail{
				Key: strings.TrimPrefix(k, cachePrefix),
				TTL: ttl.Truncate(time.Second).String(),
			})
		}
		result["dragonfly_entries"] = details
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("writing cache stats: %v", err)
	}
}
