---
tldr: Instrument chimney with OTEL + add deploy status and cache invalidation APIs per the four integration specs
status: active
---

# Plan: chimney integration — observability, deploy status, and cache control

## Context

- [[spec - chimney observability - opentelemetry instrumentation for coroot traces metrics and logs]]
- [[spec - chimney deploy status - deploy event ingestion and last-deploy endpoint]]
- [[spec - chimney metrics - prometheus text format endpoint for cache and request counters]]
- [[spec - chimney cache control - runtime cache invalidation api]]
- [[brainstorm - 2602211225 - chimney integration with table.beerpub.dev]]
- Implementation target: `cmd/chimney/main.go` (single-file Go program, 654 lines)

## Phases

### Phase 1 - OTEL SDK and HTTP instrumentation - status: open

Wire up the three OTEL providers and wrap the HTTP mux.
After this phase, chimney exports spans to Coroot — the trace waterfall shows real request paths.

1. [x] Add OTEL dependencies to go.mod
   - `go.opentelemetry.io/otel` + `otel/sdk/trace` + `otel/sdk/metric` + `otel/sdk/log`
   - OTLP HTTP exporters: `otel/exporters/otlp/otlptrace/otlptracehttp`, `otlpmetric/otlpmetrichttp`, `otlplog/otlploghttp`
   - `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`
   - `go.opentelemetry.io/contrib/bridges/otelslog`
   - `go get` each, then `go mod tidy`; commit go.mod + go.sum
   - => all packages resolved at otel v1.40.0 / sdk/log v0.16.0 / otelhttp v0.65.0 / otelslog v0.15.0
   - => go directive bumped 1.23→1.25.0 (toolchain required); updated CI go-version pins to 1.25 in 5 workflows
   - => `go build ./cmd/chimney/` passes

2. [x] Implement `otelSetup(ctx context.Context)` → shutdown func
   - TracerProvider via `otlptracehttp.New`
   - MeterProvider via `otlpmetrichttp.New` (15s push interval)
   - LoggerProvider via `otlploghttp.New`
   - Collector endpoint from `OTEL_EXPORTER_OTLP_ENDPOINT` (default `http://localhost:4318`)
   - Service name from `OTEL_SERVICE_NAME` (default `chimney`)
   - Returns a `shutdown(ctx)` func that calls all three providers' Shutdown with a 5s grace context
   - Add `startTime = time.Now()` package-level var
   - => `otelSetup` added to `cmd/chimney/main.go`; `main()` calls it, defers shutdown with 5s context
   - => go.sum was stale (tidy removed OTEL requires before imports existed); fixed by re-running `go get` at pinned versions after adding imports
   - => `go build` + `go vet` pass

3. [ ] Wrap mux with `otelhttp.NewHandler`; wire graceful shutdown in main
   - Replace `http.ListenAndServe` with `http.Server` + goroutine + `server.Shutdown` on signal
   - Call `otelSetup` at startup; defer the returned shutdown func
   - `X-Trace-ID` response header: read `trace.SpanFromContext(r.Context()).SpanContext().TraceID().String()` in a response wrapper or middleware
   - SIGTERM / SIGINT → flush OTEL providers then `server.Shutdown`

4. [ ] Set custom span attributes in handlers
   - `chimney.cache_hit` (bool), `chimney.cache_tier` (`"dragonfly"` | `"memory"` | `"none"`)
   - `chimney.github_path` (string) on `/api/github/*` requests
   - Use `trace.SpanFromContext(ctx).SetAttributes(...)` at decision points in `cacheGet` and `handleGitHubProxy`

5. [ ] Add `chimney.github_fetch` child span in `handleGitHubProxy`
   - `tracer.Start(ctx, "chimney.github_fetch")` around `httpClient.Do(req)` → `io.ReadAll`
   - Attrs: `github.api.path`, `github.api.status_code`, `github.conditional` (bool), `github.not_modified` (bool)
   - `span.RecordError(err)` + `span.SetStatus(codes.Error, ...)` on fetch errors

### Phase 2 - Structured logging and panic recovery - status: open

Replace log.Print* with slog + OTEL log bridge.
After this phase, log lines appear in Coroot linked to their parent traces.

6. [ ] Replace `log.Print*` with slog + OTEL log bridge
   - Init `otelslog.NewHandler(loggerProvider)` as the slog backend
   - `slog.SetDefault(slog.New(otelslogHandler))` — this also redirects Go's `log` package
   - Migrate each `log.Printf/Println/Fatal` call to `slog.InfoContext` / `slog.WarnContext` / `slog.ErrorContext`
   - Pass request context to slog calls inside handlers so trace_id / span_id attach automatically

7. [ ] Add panic recovery middleware
   - Outer-most wrapper (added before `otelhttp.NewHandler` in the chain)
   - On panic: recover, get current span from context, `span.RecordError`, `span.SetStatus(codes.Error, "panic")`
   - Log at ERROR via slog with `"stack"` attribute (runtime.Stack, truncated to 4KB)
   - Increment `panicsCounter` (OTEL counter, registered in Phase 3) — use atomic int64 for Phase 2, promote in Phase 3
   - Write 500 response

8. [ ] Add request log middleware
   - Wraps handlers inside `otelhttp`; captures status code, bytes written, latency
   - After handler returns: emit `slog.InfoContext(ctx, "request", ...)` with method, route, status, latency_ms, cache_hit, cache_tier, bytes
   - Stores per-request cache metadata (hit/tier) via context key set in `cacheGet`

### Phase 3 - OTEL metrics - status: open

Promote existing int64 counters to OTEL instruments.
After this phase, Coroot shows chimney metric charts (cache ratio, rate limit remaining, etc.).

9. [ ] Promote cache counters; add Dragonfly and rate-limit gauges
   - `chimney.cache.hits` counter (`tier` attr: `dragonfly` / `memory`) — replaces `cacheHits int64`
   - `chimney.cache.misses` counter — replaces `cacheMisses int64`
   - `chimney.cache.entries` observable gauge — `len(memCache)` at collection time
   - `chimney.dragonfly.connected` observable gauge — 1/0, read from `useRedis` atomic
   - `chimney.github.rate_limit.remaining` + `.reset` gauges — updated in `handleGitHubProxy` when GitHub response includes headers
   - Remove `counterMu sync.Mutex` + int64 globals; remove cache_hits/misses from `/healthz` JSON (already in OTEL)

10. [ ] Add request metrics; wire panics and deploy-event counters
    - `chimney.requests` counter (`route`, `status_class` attrs: 2xx/3xx/4xx/5xx) — incremented by request middleware
    - `chimney.request.duration` histogram (`route` attr, explicit boundaries: 5/25/100/500/2000ms)
    - `chimney.panics` counter — replace Phase 2 atomic int64 placeholder with OTEL counter
    - `chimney.deploy_events` counter (`outcome` attr) — placeholder counter; incremented by Phase 4 handler

### Phase 4 - Deploy status endpoint and CI hook - status: open

After this phase, `GET /api/deploy/status` shows last deploy metadata and CI writes to it on completion.

11. [ ] Add deploy event ring buffer + `POST /api/deploy/events`
    - `deployEvent` struct: `SHA string`, `Slot string`, `Outcome string`, `Timestamp time.Time`, `DurationS float64`
    - 50-element circular buffer with mutex
    - `POST /api/deploy/events`: check `Authorization: Bearer <$DEPLOY_TOKEN>`; decode JSON body; append; increment `chimney.deploy_events{outcome=...}` counter

12. [ ] Add `GET /api/deploy/status`
    - Returns last event + `age_s` (time.Since)
    - Returns success_rate_pct over all ring entries (0–100, omitted if ring empty)
    - Returns 204 with empty body if no events received yet

13. [ ] Add CI deploy hook to `chimney-deploy.yml`
    - After smoke tests pass: `curl -sf -X POST https://<origin-ip>/api/deploy/events` with JSON body `{sha, slot, outcome:"success", duration_s}`
    - On failure path (if smoke tests fail): post `outcome:"failure"` before exit
    - `DEPLOY_TOKEN` from GitHub secret; add to deploy workflow env
    - Use the origin public IP already available from the deploy loop variable

### Phase 5 - Cache invalidation API - status: open

After this phase, table.beerpub.dev can force-refresh a stale GitHub path post-deploy.

14. [ ] Implement `POST /api/cache/invalidate`
    - `Authorization: Bearer <$INVALIDATE_TOKEN>` env var
    - Body: `{"prefix": "/pulls", "all": false}`
    - `all: true` requires `INVALIDATE_ALL_ALLOWED=true` env (guard against accidental full wipe)
    - Prefix-match: SCAN Dragonfly for `chimney:<prefix>*` + delete; iterate `memCache` for key-prefix match + delete
    - Response: `{"deleted": N, "dragonfly": N, "memory": N}`
    - Log via slog + set span attribute `chimney.cache.invalidated_keys` (count)

15. [ ] Register `/api/cache/invalidate` route
    - `mux.HandleFunc("POST /api/cache/invalidate", handleCacheInvalidate)` (Go 1.22 method+path syntax)
    - Add `INVALIDATE_TOKEN` and `INVALIDATE_ALL_ALLOWED` to `compose.origin.yml` env block (empty defaults)

## Verification

- `curl https://chimney.beerpub.dev/healthz` → `{"status":"ok"}` with no regression
- All chimney HTTP responses carry `X-Trace-ID` header
- Coroot UI on table.beerpub.dev shows chimney service with spans (request waterfall), logs (linked to traces), and metrics (cache ratio, rate limit)
- `POST /api/deploy/events` with valid token → 200; `GET /api/deploy/status` → last event details
- `POST /api/cache/invalidate` with valid token → `{"deleted": N}`
- `go build ./cmd/chimney/` succeeds with no race warnings (`-race` flag locally)

## Adjustments

## Progress Log

- 2602211419 — Phase 1 / action 1: OTEL deps added (otel v1.40, go 1.25); CI go-version pins updated
- 2602211419 — Phase 1 / action 2: otelSetup() implemented; all three providers wired; build + vet pass
