---
tldr: Chimney serves the wgmesh dashboard static files and a caching GitHub API proxy; responses are stored in Dragonfly (DB 0) with an in-memory fallback; ETag-aware conditional fetching and stale-on-error ensure low latency even when GitHub or Dragonfly are unavailable.
category: core
---

# Chimney — dashboard server with GitHub API proxy and two-layer cache

## Target

The origin server for `chimney.beerpub.dev` — the wgmesh project dashboard.
Serves static HTML and provides a server-side caching proxy to the GitHub REST API.
The authenticated proxy raises the rate limit from 60 to 5,000 req/hr;
caching further shields the dashboard from GitHub rate limits and network latency.

## Behaviour

### Startup

- Flags: `-addr` (default `:8080`), `-docs` (default `./docs`), `-repo` (default `atvirokodosprendimai/wgmesh`), `-redis` (default `127.0.0.1:6379`).
- `GITHUB_TOKEN` env var — if set, all GitHub requests use `Authorization: Bearer <token>` (5,000 req/hr); absent = unauthenticated (60 req/hr).
- Versions injected at build via `-ldflags`: `version`, `wgmeshVersion`.
- Dragonfly connection is attempted **asynchronously** — up to 30 retries with 1s backoff.
  The HTTP server starts immediately; `/healthz` is responsive while Dragonfly is still connecting.
  After the goroutine finishes, `redisConnDone` is set; `useRedis` is set only on success.

### Cache layer

Two tiers; the same abstraction (`cacheGet`/`cacheSet`) is used for both:

**Tier 1 — Dragonfly (Redis-compatible, DB 0):**
- All cache entries stored with `chimney:` key prefix.
- Used when `useRedis` is `true` (atomic bool, set by connect goroutine).
- R/W timeout: 200ms; dial timeout: 1s.
- On miss or error: falls through to tier 2.

**Tier 2 — In-memory map (fallback):**
- Always written on every `cacheSet` (even when Dragonfly is up), so it acts as a hot backup.
- Max 500 entries; on overflow, the entry with the oldest `fetchedAt` is evicted.
- No TTL enforcement — freshness is checked at read time via `time.Since(entry.FetchedAt) < maxAge`.

**Cached entry shape:**
- `body []byte`, `etag string`, `status_code int`, `headers map[string]string`, `fetchedAt time.Time`.
- Headers preserved: `Content-Type`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`.

### GitHub API proxy (`/api/github/*`)

Maps incoming path `GET /api/github/<path>` → `GET https://api.github.com/repos/<repo>/<path>`.

**Cache key**: `ghPath + "?" + rawQuery`.

**TTL by path:**

| Path pattern | TTL |
|---|---|
| Contains `/actions/runs` | 30s |
| Contains `/pulls` + `state=closed` in query | 5 minutes |
| Contains `/issues` | 2 minutes |
| Default | 30s |

**Request flow:**

1. Check cache (`cacheGet`).
2. **Client ETag match**: if client sent `If-None-Match` matching the cached ETag → `304 Not Modified` immediately (no upstream fetch).
3. **Fresh cache hit**: if entry is within TTL → serve from cache.
4. **Cache miss or stale**: fetch from GitHub.
   - Sends `If-None-Match` with cached ETag (if available) → GitHub may return `304`.
   - **GitHub 304**: refreshes `FetchedAt` in the cache (TTL extension without re-reading body); serves cached body.
   - **GitHub error/network failure**: serves stale cache entry if one exists; otherwise 502.
   - **GitHub 2xx+**: stores new entry (body, ETag, status code, headers) in both cache tiers.
5. Response headers always include:
   - `X-Cache-Age`: seconds since `FetchedAt`.
   - `Access-Control-Allow-Origin: *`.
   - Forwarded: `ETag`, `Content-Type`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`.

### Pipeline summary (`/api/pipeline/summary`)

A pre-aggregated view of the repository's pipeline state, cached for 60s.

Fetches (sequentially, best-effort — partial failures produce a partial response):
- Open issues: `/issues?state=open&per_page=100` — filters out PR entries (items with `pull_request` field non-nil).
- Open PRs: `/pulls?state=open&per_page=100`.
- Last merged PR: `/pulls?state=closed&per_page=10&sort=updated&direction=desc` — first entry with non-empty `merged_at`.
- Recent Goose build runs: `/actions/workflows/goose-build.yml/runs?per_page=10&status=completed` — last 10 completed runs; computes `goose_success_rate_pct` = (successes / total) × 100.

Response shape: `{wgmesh_version, open_issues, open_prs, last_merged_pr, recent_workflow_runs, goose_success_rate_pct, fetched_at}`.

### Other endpoints

| Path | Description |
|---|---|
| `GET /healthz` | Dragonfly status (connected/connecting/unavailable/error + key count + memory), in-memory entry count, cache hits/misses |
| `GET /api/version` | `{chimney_version, wgmesh_version}` from build-time ldflags |
| `GET /api/cache/stats` | hits/misses, in-memory entry count, all Dragonfly keys under `chimney:` prefix with individual TTLs (SCAN-based enumeration) |
| `GET /` | Static file server from `-docs` directory |

---

## pkg/proxy — Host-based reverse proxy

`pkg/proxy.Proxy` is a standalone HTTP handler used by lighthouse's optional reverse proxy mode (`-proxy-addr`/`-proxy-origins` flags).

- Constructed with a static `domain → upstream URL` map (e.g. `"example.com" → "http://10.0.0.2:3000"`).
- `ServeHTTP`: strips port from `Host` header, lowercases, looks up in map.
  - Unknown host → `502 Bad Gateway`.
  - Known host → `httputil.ReverseProxy` to upstream.
- Sets `X-Forwarded-Host` (original `Host`) and `X-Forwarded-Proto` (http/https from `r.TLS`).
- Sets upstream `Host` to target host (not the incoming host).
- Dial timeout: 10s.

## Design

- **Async Dragonfly connect**: the HTTP server is immediately available; dashboard visitors and health checks aren't blocked waiting for cache warmup. The flag `redisConnDone` lets `/healthz` distinguish "still connecting" from "gave up".
- **Always-write in-memory**: writes to both tiers on every cache set ensures in-memory always has a copy. If Dragonfly goes down mid-operation, the in-memory tier immediately covers misses.
- **ETag propagation**: by forwarding client `If-None-Match` to GitHub and responding with `304` when the client's ETag matches the cache, the proxy correctly participates in the browser cache chain — no redundant bytes transferred to the client even for stale-in-cache responses.
- **Stale-on-error**: serving a stale entry on GitHub network failure means the dashboard remains functional during brief GitHub API outages.
- **CORS `*` on all API routes**: the dashboard JS fetches from the same origin, but `*` means the API can also be consumed by local dev environments without CORS preflight issues.
- **In-memory eviction is oldest-first, not LRU**: eviction scans the entire map to find the minimum `fetchedAt`. Acceptable for ≤500 entries; would need a heap for larger caches.

## Interactions

- `GITHUB_TOKEN` env var — authenticated GitHub access.
- Dragonfly — shared instance; chimney uses DB 0, lighthouse uses DB 1.
- Edge Caddy — receives `ETag` response headers; caches at the edge; sends `If-None-Match` on revalidation.
- `cmd/lighthouse/main.go` — optionally starts `pkg/proxy.Proxy` alongside the lighthouse API.

## Mapping

> [[cmd/chimney/main.go]]
> [[pkg/proxy/proxy.go]]
