---
tldr: Dashboard origin server — serves static HTML and proxies GitHub API with two-tier caching
category: infra
---

# Chimney

## Target
Project dashboard origin server at `chimney.beerpub.dev`. Unrelated to mesh operation.

## Behaviour
- The dashboard always has fresh-enough GitHub data without hammering the API rate limit
  - Responses served from cache within TTL; upstream only fetched on miss
  - ETag round-trips avoid downloading unchanged bodies
  - Stale cache served when GitHub is unreachable
- Cache survives Dragonfly being unavailable — in-memory fallback always active
- Server answers health checks immediately, before Dragonfly has connected
- Dashboard consumers can see live project health at a glance via `/api/pipeline/summary`
  - Aggregates: open issue count, open PR count, last merged PR, recent Goose CI runs, success rate
- Edge Caddy caches can work efficiently using forwarded ETags and `X-Cache-Age` headers

## Design
Two-tier cache: Dragonfly (primary, persistent, Redis-compatible) + in-memory map (fallback, always written).
Dragonfly connects asynchronously on startup with 30 × 1s retries; HTTP server starts immediately.
In-memory capped at 500 entries; oldest-by-fetch-time evicted on overflow.

TTL policy by endpoint type: CI runs = 30s, closed PRs = 5min, issues = 2min, pipeline summary = 60s, default = 30s.

Authenticated GitHub token (`GITHUB_TOKEN`) lifts rate limit from 60 to 5,000 req/hr.
`/api/pipeline/summary` is a purpose-built aggregation (not a generic proxy).

## Interactions
- Depends on: Dragonfly/Redis (optional), GitHub REST API, `GITHUB_TOKEN` env var
- Consumed by: dashboard frontend (static HTML in `docs/`), edge Caddy servers

## Mapping
> [[cmd/chimney/main.go]]
