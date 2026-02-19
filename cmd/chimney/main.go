// chimney is the origin server for the wgmesh dashboard at chimney.cloudroof.eu.
//
// It serves the static dashboard HTML and provides a caching proxy for the
// GitHub REST API. Server-side caching with an authenticated GitHub token
// gives us 5,000 req/hr instead of 60 req/hr unauthenticated, and the proxy
// returns ETag-aware responses so edge Caddy servers can cache efficiently.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	githubAPI   = "https://api.github.com"
	defaultRepo = "atvirokodosprendimai/wgmesh"
)

// cacheEntry holds a cached GitHub API response.
type cacheEntry struct {
	body       []byte
	etag       string
	statusCode int
	headers    http.Header
	fetchedAt  time.Time
}

var (
	cache   = make(map[string]*cacheEntry)
	cacheMu sync.RWMutex

	githubToken string
	repo        string
)

func main() {
	addr := flag.String("addr", ":8080", "Listen address")
	docsDir := flag.String("docs", "./docs", "Path to static dashboard files")
	flag.StringVar(&repo, "repo", defaultRepo, "GitHub owner/repo")
	flag.Parse()

	githubToken = os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		log.Println("WARNING: GITHUB_TOKEN not set — using unauthenticated API (60 req/hr)")
	} else {
		log.Println("GitHub token configured — 5,000 req/hr")
	}

	mux := http.NewServeMux()

	// API proxy: /api/github/* → GitHub REST API with server-side caching
	mux.HandleFunc("/api/github/", handleGitHubProxy)

	// Health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cacheMu.RLock()
		entries := len(cache)
		cacheMu.RUnlock()
		fmt.Fprintf(w, `{"status":"ok","cache_entries":%d,"repo":"%s"}`, entries, repo)
	})

	// Cache stats
	mux.HandleFunc("/api/cache/stats", handleCacheStats)

	// Static dashboard files (fallback)
	fs := http.FileServer(http.Dir(*docsDir))
	mux.Handle("/", fs)

	log.Printf("chimney starting on %s (docs=%s, repo=%s)", *addr, *docsDir, repo)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

// handleGitHubProxy proxies requests to GitHub API with server-side caching.
// Path: /api/github/pulls?state=open → github.com/repos/{repo}/pulls?state=open
func handleGitHubProxy(w http.ResponseWriter, r *http.Request) {
	// Strip /api/github prefix to get the GitHub API path
	ghPath := strings.TrimPrefix(r.URL.Path, "/api/github")
	if ghPath == "" {
		ghPath = "/"
	}
	// Preserve query string
	ghURL := fmt.Sprintf("%s/repos/%s%s", githubAPI, repo, ghPath)
	if r.URL.RawQuery != "" {
		ghURL += "?" + r.URL.RawQuery
	}

	cacheKey := ghPath + "?" + r.URL.RawQuery

	// Check cache — serve if fresh enough (30s for hot data, 5m for cold)
	cacheMu.RLock()
	entry, found := cache[cacheKey]
	cacheMu.RUnlock()

	maxAge := 30 * time.Second
	if strings.Contains(ghPath, "/actions/runs") {
		maxAge = 30 * time.Second // workflow runs change often
	} else if strings.Contains(ghPath, "/pulls") && strings.Contains(r.URL.RawQuery, "state=closed") {
		maxAge = 5 * time.Minute // closed PRs rarely change
	} else if strings.Contains(ghPath, "/issues") {
		maxAge = 2 * time.Minute
	}

	// Client sent If-None-Match? Check against our cache
	clientETag := r.Header.Get("If-None-Match")
	if found && clientETag != "" && clientETag == entry.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Serve from cache if fresh
	if found && time.Since(entry.fetchedAt) < maxAge {
		writeResponse(w, entry)
		return
	}

	// Fetch from GitHub (with conditional request if we have an ETag)
	req, err := http.NewRequestWithContext(r.Context(), "GET", ghURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "chimney/1.0 (cloudroof.eu)")
	if githubToken != "" {
		req.Header.Set("Authorization", "Bearer "+githubToken)
	}
	if found && entry.etag != "" {
		req.Header.Set("If-None-Match", entry.etag)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// If GitHub is down but we have stale cache, serve it
		if found {
			writeResponse(w, entry)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 304 — GitHub says data hasn't changed, refresh our cache timestamp
	if resp.StatusCode == http.StatusNotModified && found {
		cacheMu.Lock()
		entry.fetchedAt = time.Now()
		cacheMu.Unlock()
		writeResponse(w, entry)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	// Cache the response
	newEntry := &cacheEntry{
		body:       body,
		etag:       resp.Header.Get("ETag"),
		statusCode: resp.StatusCode,
		headers:    make(http.Header),
		fetchedAt:  time.Now(),
	}
	// Copy relevant headers
	for _, h := range []string{"Content-Type", "X-RateLimit-Remaining", "X-RateLimit-Reset"} {
		if v := resp.Header.Get(h); v != "" {
			newEntry.headers.Set(h, v)
		}
	}

	cacheMu.Lock()
	cache[cacheKey] = newEntry
	// Evict old entries (keep cache bounded)
	if len(cache) > 500 {
		oldest := ""
		oldestTime := time.Now()
		for k, v := range cache {
			if v.fetchedAt.Before(oldestTime) {
				oldest = k
				oldestTime = v.fetchedAt
			}
		}
		if oldest != "" {
			delete(cache, oldest)
		}
	}
	cacheMu.Unlock()

	writeResponse(w, newEntry)
}

func writeResponse(w http.ResponseWriter, entry *cacheEntry) {
	for k, vals := range entry.headers {
		for _, v := range vals {
			w.Header().Set(k, v)
		}
	}
	if entry.etag != "" {
		w.Header().Set("ETag", entry.etag)
	}
	w.Header().Set("X-Cache-Age", fmt.Sprintf("%.0f", time.Since(entry.fetchedAt).Seconds()))
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(entry.statusCode)
	w.Write(entry.body)
}

func handleCacheStats(w http.ResponseWriter, r *http.Request) {
	cacheMu.RLock()
	defer cacheMu.RUnlock()

	type stat struct {
		Key       string `json:"key"`
		Age       string `json:"age"`
		HasETag   bool   `json:"has_etag"`
		BodyBytes int    `json:"body_bytes"`
	}
	stats := make([]stat, 0, len(cache))
	for k, v := range cache {
		stats = append(stats, stat{
			Key:       k,
			Age:       time.Since(v.fetchedAt).Truncate(time.Second).String(),
			HasETag:   v.etag != "",
			BodyBytes: len(v.body),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries": len(stats),
		"details": stats,
	})
}
