---
tldr: GET /api/metrics exposes chimney's internal counters in Prometheus text format — request counts by route and status class, cache hit/miss ratio, GitHub rate limit remaining, Dragonfly status, and panic count. With Coroot running on table.beerpub.dev, this is a secondary output; the primary metrics path is OTEL metrics via OTLP — see the observability spec.
category: feature
superseded_by: "[[spec - chimney observability - opentelemetry instrumentation for coroot traces metrics and logs]]"
---

> **Note:** table.beerpub.dev runs Coroot. The primary metrics output is OTEL metrics via OTLP (see [[spec - chimney observability - opentelemetry instrumentation for coroot traces metrics and logs]]).
> This Prometheus endpoint may still be served as a secondary/optional output via the OTEL Prometheus exporter, but it is no longer the recommended path.
> The counter definitions in this spec remain valid as the instrument inventory; ownership has moved to the observability spec.

# Chimney metrics — Prometheus text format endpoint for cache and request counters

## Target

A `/api/metrics` endpoint that emits chimney's runtime counters in Prometheus text exposition format.
Requires no external Prometheus server — table.beerpub.dev scrapes it directly and renders sparklines.
Gives a time-series view of cache efficiency, request load, and error rates.

## Behaviour

### Counters maintained

All counters are in-process atomics or mutex-protected maps, reset on restart.

| Metric name | Type | Labels | Description |
|---|---|---|---|
| `chimney_requests_total` | counter | `route`, `status_class` | Request count per route; status_class = 2xx/3xx/4xx/5xx |
| `chimney_request_latency_ms` | histogram buckets | `route` | Latency distribution: 5, 25, 100, 500, 2000ms |
| `chimney_cache_hits_total` | counter | — | Cache hits (Dragonfly or in-memory) |
| `chimney_cache_misses_total` | counter | — | Cache misses (upstream fetch required) |
| `chimney_github_rate_limit_remaining` | gauge | — | Last observed `X-RateLimit-Remaining` from GitHub |
| `chimney_github_rate_limit_reset` | gauge | — | Last observed `X-RateLimit-Reset` (Unix timestamp) |
| `chimney_dragonfly_connected` | gauge | — | 1 if Dragonfly is connected, 0 otherwise |
| `chimney_mem_cache_entries` | gauge | — | Current in-memory cache entry count |
| `chimney_panics_total` | counter | — | Recovered panics from panic middleware |
| `chimney_deploy_events_total` | counter | `outcome` | Deploy events received (success/failure) |
| `chimney_uptime_seconds` | gauge | — | Seconds since process start |

### `/api/metrics` response

Content-Type: `text/plain; version=0.0.4` (standard Prometheus exposition format).
No authentication — consistent with `/healthz` (same information, different format).

Example fragment:

```
# HELP chimney_requests_total Total requests by route and status class
# TYPE chimney_requests_total counter
chimney_requests_total{route="/api/github",status_class="2xx"} 1842
chimney_requests_total{route="/api/github",status_class="5xx"} 3
chimney_requests_total{route="/healthz",status_class="2xx"} 4201

# HELP chimney_cache_hits_total Total cache hits
# TYPE chimney_cache_hits_total counter
chimney_cache_hits_total 1761

# HELP chimney_github_rate_limit_remaining GitHub API rate limit remaining
# TYPE chimney_github_rate_limit_remaining gauge
chimney_github_rate_limit_remaining 4837
```

### Counter updates

- `chimney_requests_total` and `chimney_request_latency_ms` — updated by the structured logging middleware on every request completion.
- `chimney_cache_hits_total` / `chimney_cache_misses_total` — updated by `cacheGet` (same path as the existing `cacheHits`/`cacheMisses` int64 counters — promote these to the metrics layer).
- `chimney_github_rate_limit_remaining` / `_reset` — updated when a GitHub API response is received and contains these headers.
- `chimney_dragonfly_connected` — set by the async Dragonfly connect goroutine.
- `chimney_mem_cache_entries` — read from `len(memCache)` at scrape time.
- `chimney_panics_total` — incremented by the panic recovery middleware.
- `chimney_deploy_events_total` — incremented by the deploy event handler.
- `chimney_uptime_seconds` — `time.Since(startTime)` at scrape time.

## Design

- **Prometheus text format without a Prometheus server**: the format is trivially parseable; table.beerpub.dev can scrape and render sparklines with a small JS snippet or a lightweight client. No sidecar, no remote-write, no TSDB needed.
- **Promote existing counters**: `cacheHits` and `cacheMisses` are already maintained as `int64` globals — promoting them to the metrics layer costs nothing. Avoid double-counting.
- **Histograms over averages**: latency histograms let table.beerpub.dev show p50/p95 without storing raw data. The five buckets (5/25/100/500/2000ms) cover chimney's typical response range.
- **Labels on `requests_total` by route**: route-level granularity distinguishes cache-served `/api/github` traffic from static file serving and `/healthz` polling noise.
- **No cardinality explosion**: routes are a small fixed set; status_class is bucketed (not raw code). No per-path or per-IP labels.
- **`chimney_github_rate_limit_remaining` as gauge**: this is a point-in-time observation, not a cumulative count. The "dead man's switch" pattern in table can alert if it drops below a threshold.

## Interactions

- Structured logging middleware — provides `requests_total` and `latency_ms` updates.
- Panic recovery middleware — provides `panics_total` updates.
- Deploy event handler — provides `deploy_events_total` updates.
- Dragonfly connect goroutine — sets `dragonfly_connected`.
- GitHub proxy handler — extracts `X-RateLimit-Remaining` / `X-RateLimit-Reset` headers for gauges.
- `cmd/chimney/main.go` — registers `/api/metrics` route; initialises `startTime`.
- table.beerpub.dev — scrapes `/api/metrics` on a configurable interval (e.g. 15s); renders sparklines.

## Mapping

> [[cmd/chimney/main.go]]
