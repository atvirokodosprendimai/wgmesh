---
tldr: How to integrate chimney fully into table.beerpub.dev — GitHub pipelines, deployments, live app errors/traces/logs
status: active
---

# Brainstorm: chimney integration with table.beerpub.dev

## Seed

chimney.beerpub.dev is the origin server for the wgmesh dashboard — it proxies GitHub API data,
aggregates pipeline summaries, and serves static HTML.
table.beerpub.dev is the integration target: a richer surface that should show chimney's full state.

Three integration dimensions:
1. **GitHub pipelines** — CI runs, PR status, deployment events
2. **Deployments** — blue/green deploy state, origin health, Caddy status
3. **Errors / traces / logs** — what's happening inside the running chimney process

The goal: table.beerpub.dev becomes the single pane of glass for chimney — you can see, understand,
and act on everything without SSHing in.

Related specs:
- [[spec - chimney - dashboard server with github api proxy and two-layer cache]]
- [[spec - lighthouse - cdn control plane with rest api dragonfly store xds and federated sync]]
- [[postmortem - 2602211013 - chimney 503 incident]]

## Ideas

- Stream structured log lines from chimney over SSE (Server-Sent Events) to table
- Expose a `/api/logs/stream` endpoint on chimney that tails the recent log ring buffer
- Add a fixed-size in-memory ring buffer (e.g. 1000 lines) to chimney that captures all `log.Print*` output
- table polls `/healthz` every N seconds and shows live Dragonfly status, cache hit/miss rate, mem entries
- table shows the full `/api/pipeline/summary` response rendered as a dashboard card
- table shows `/api/cache/stats` as a live cache inspector (expandable key list with TTLs)
- Add a `/api/deploy/status` endpoint to chimney that reports blue/green active slot, last deploy timestamp, origin IPs
- Webhook receiver in chimney: GitHub sends `workflow_run` and `deployment` events → chimney stores them → table polls
- Structured logging middleware in chimney: every request gets logged as JSON (method, path, status, latency, cache_hit)
- OpenTelemetry traces exported from chimney to a local OTLP collector → table queries via Jaeger/Tempo UI embed
- Simple error counter endpoint: chimney tracks 5xx counts per route, table shows error rate per endpoint
- Add request ID header (`X-Request-ID`) to all chimney responses; table can correlate cache misses with errors
- GitHub deployment status API: chimney posts deployment status (in_progress → success/failure) on each deploy; table reads it
- table embeds GitHub Actions iframe for the wgmesh workflow runs (simple, no backend needed)
- chimney exposes a `/api/metrics` endpoint in Prometheus text format; table scrapes and renders sparklines
- Add a `/api/events` SSE stream that emits cache eviction, Dragonfly reconnect, GitHub rate limit events in real time
- Deploy hook: when `chimney-deploy.yml` completes, it POSTs to a chimney endpoint → chimney records the event
- table shows a timeline of deployments with commit SHA, PR number, timestamp, and duration
- Structured panic recovery middleware in chimney: catches panics, logs them with stack trace, increments error counter
- Caddy access log forwarding: Caddy writes JSON access logs, chimney tails them and exposes via `/api/logs/caddy`
- table shows the current wgmesh version badge (`/api/version`) prominently
- Rate limit remaining gauge: chimney re-exposes `X-RateLimit-Remaining` from GitHub in `/healthz`
- Add `cache_age_p50/p95` to `/api/cache/stats` — how stale are we on average?
- table shows "last deploy" as a relative timestamp with commit message (from GitHub API, already cached)
- Dead man's switch: if chimney hasn't received a successful GitHub API response in >10min, table shows a warning banner
- chimney emits a heartbeat event on the SSE stream every 30s so table knows the connection is alive
- Log level API: `POST /api/log-level` to change chimney's log verbosity at runtime without restart
- table can trigger a cache bust (`POST /api/cache/invalidate`) for a specific GitHub path — useful after a deploy
- Add a `/api/github/releases` endpoint to surface wgmesh release history in table
- Health history: chimney records the last N health check results for each origin; table shows uptime percentage
- Deployment diff: table shows what changed between last two deploy SHAs (link to GitHub compare URL)
- table shows open GitHub issues that have `type:bug` label — a live bug tracker embedded in the dashboard

## Clusters

### Live process observability (what's chimney doing right now)

- In-memory log ring buffer + `/api/logs/stream` SSE endpoint
- Structured JSON request logging middleware (method, path, status, latency, cache_hit)
- `/api/metrics` Prometheus endpoint for sparklines
- `/api/events` SSE stream (cache evictions, Dragonfly reconnect, rate limit hits)
- 5xx error counters per route
- Panic recovery with stack trace logging
- Heartbeat event on SSE stream (connection keepalive)
- Log level API (runtime verbosity without restart)
- `X-Request-ID` on all responses (correlation)

### Deployment visibility (what was deployed, when, and did it succeed)

- `/api/deploy/status` — active blue/green slot, last deploy timestamp, origin IPs
- Deploy hook: `chimney-deploy.yml` POSTs to chimney on completion
- GitHub deployment status API integration (in_progress → success/failure)
- Deployment timeline with SHA, PR number, timestamp, duration
- Deployment diff — link to GitHub compare for last two SHAs
- GitHub releases endpoint

### GitHub pipeline surface (what's the project state)

- `/api/pipeline/summary` already covers most of this — rendering in table is the main gap
- table embeds or mirrors CI run list (already in `/api/github/actions/runs`)
- Open bug issues with `type:bug` label
- Rate limit remaining gauge

### Cache health (is the cache healthy and fresh)

- `/api/cache/stats` with per-key TTL (already exists)
- `cache_age_p50/p95` percentiles
- Cache bust API (`POST /api/cache/invalidate`)
- Dead man's switch (no successful GitHub response in >10min)
- Health history with uptime percentage

### table-side rendering (what table.beerpub.dev needs to consume)

- table polls `/healthz` for live Dragonfly + cache stats
- table renders pipeline summary card
- table embeds SSE stream for live logs/events
- table shows version badge prominently
- table shows deployment timeline
- Caddy access log forwarding for edge-level visibility

## Standouts

**1. In-memory log ring buffer + SSE stream**
The highest-signal addition with the lowest implementation cost.
A 1000-line ring buffer in chimney + `/api/logs/stream` (SSE) gives table a live log tail.
No external deps (no Loki, no ELK). Directly addresses "errors/logs when running app."
The ring buffer also covers the "lost logs during restarts" problem from the chimney 503 incident.

**2. Structured JSON request logging middleware**
Every request logged as JSON: method, path, status, latency, cache_hit, request_id.
Low cost, high payoff: feeds the SSE stream, enables the error rate counter, and makes
the 503-style incidents debuggable without SSH access.
`X-Request-ID` as a correlation handle is part of this.

**3. `/api/deploy/status` + deploy hook in CI**
`chimney-deploy.yml` POSTing a deploy event to chimney (with SHA, slot, timestamp) closes
the loop between the CI pipeline and the running process.
table can show "last deployed 3h ago, commit abc1234, PR #321" without any GitHub API calls.
Directly addresses the "deployment" integration dimension.

**4. `/api/metrics` Prometheus endpoint**
Exposes cache hit/miss, request counts, error counts, GitHub rate limit remaining as
Prometheus text format. table can scrape and render sparklines.
Also enables future alerting (Alertmanager or a simple threshold check).

**5. Cache bust API (`POST /api/cache/invalidate`)**
table can trigger cache invalidation for a specific path after a deploy —
e.g. force-refresh `/api/github/pulls` right after merging a PR.
Small but high-UX-value; makes table an active tool not just a passive viewer.

## Next Steps

- Spec out the observability layer (ring buffer + SSE + structured logging) → `/eidos:spec`
- Spec out the deploy status endpoint + CI hook integration → `/eidos:spec`
- Plan implementation with phases: logging middleware first, then deploy hook, then metrics → `/eidos:plan`
- Decide on SSE vs polling for live data in table (trade-off: SSE is push but requires persistent conn) → `/eidos:decision`
