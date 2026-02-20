# Pull — chimney dashboard server

**Source:** `cmd/chimney/main.go`
**Date:** 2026-02-20

---

## Collected Material

### Purpose
HTTP origin server for the wgmesh project dashboard at `chimney.beerpub.dev`.
Two jobs: serve static dashboard HTML + proxy the GitHub REST API with server-side caching.

### Endpoints
- `GET /api/github/*` — GitHub API proxy
- `GET /api/pipeline/summary` — aggregated pipeline status (issues, PRs, recent CI runs)
- `GET /api/version` — chimney + wgmesh version
- `GET /healthz` — cache + Dragonfly health
- `GET /api/cache/stats` — cache key/TTL introspection
- `/` — static file server (`./docs` dir)

### Cache layer
- **Primary:** Dragonfly (Redis-compatible), `chimney:` prefix
- **Fallback:** in-memory `map[string]*memEntry`, always written alongside Dragonfly
- Dragonfly connects asynchronously (30 × 1s retries) so HTTP server starts immediately
- In-memory capped at 500 entries; oldest-by-fetch-time evicted on overflow
- Entries stored as JSON-serialised `cachedResponse` (body, ETag, status, selected headers, fetchedAt)

### TTL policy (by GitHub path)
- `/actions/runs` → 30s
- `/pulls?…state=closed` → 5min
- `/issues` → 2min
- default → 30s
- `/api/pipeline/summary` → 60s (hard-coded, handled separately)

### ETag protocol
- Client sends `If-None-Match` matching cached ETag → immediate 304, no upstream fetch
- Server sends `If-None-Match` to GitHub with cached ETag; on GitHub 304, refreshes `FetchedAt` TTL only (no body re-read)
- ETag forwarded on all responses

### Stale-on-error
On GitHub fetch failure, serve stale cached entry rather than returning an error

### Rate limit leverage
Authenticated token: 5,000 req/hr vs 60 unauthenticated. Token from `GITHUB_TOKEN` env var.

### Pipeline summary
Aggregates from 3-4 GitHub API calls: open issues (filters PRs), open PRs, last merged PR, last 10 completed Goose build runs + success rate. Not a generic proxy — purpose-built aggregation.

### Headers forwarded
`Content-Type`, `X-RateLimit-Remaining`, `X-RateLimit-Reset` + ETag from GitHub responses.
`X-Cache-Age`, `Access-Control-Allow-Origin: *` added by chimney.

---

## Intent Sketch

- The dashboard always has fresh-enough GitHub data without hammering the API
  - {>> serve from cache if within TTL; only fetch upstream on miss}
  - {>> ETag round-trip avoids downloading unchanged bodies}
  - {>> stale-on-error keeps the dashboard alive when GitHub is flaky}
- Cache survives Dragonfly restarts or unavailability without losing responses
  - {>> in-memory always written; Dragonfly is an upgrade, not a requirement}
- The server starts answering health checks immediately, even before Dragonfly is ready
  - {>> async connection with retry; /healthz reports connecting/unavailable/connected}
- Dashboard consumers can see live project health at a glance
  - {>> /api/pipeline/summary aggregates issues, PRs, last merge, CI success rate}
- Edge Caddy servers can cache efficiently using ETags and cache-age hints
  - {>> X-Cache-Age header signals staleness; ETag enables client-side 304}
