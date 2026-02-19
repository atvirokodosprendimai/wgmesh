# chimney — cloudroof.eu Dashboard Origin

chimney is the origin server for the wgmesh pipeline dashboard at `chimney.cloudroof.eu`.

## Architecture

```
                    ┌──────────────────────┐
         ┌────────►│ chimney.cloudroof.eu  │◄────────┐
         │         │       (DNS)           │         │
         │         └──────────────────────┘         │
         │                                      │
    ┌────┴────┐                           ┌────┴────┐
    │ edge-eu │  Nuremberg (nbg1)         │ edge-us │  Ashburn (ash)
    │  Caddy  │◄──── wgmesh tunnel ──────►│  Caddy  │
    └────┬────┘                           └────┬────┘
         │                                      │
         └──────────┬───────────────────────────┘
                    │
               ┌─────┴─────┐
               │  chimney   │  Origin (runs on edge-eu)
               │  :8080     │  Go binary: cache proxy + static HTML
               └─────┬──────┘
                     │
               ┌─────┴──────┐
               │ Dragonfly   │  Redis-compatible cache (127.0.0.1:6379)
               │ 128MB RAM   │  Shared, persistent, TTL-based eviction
               └─────┬──────┘
                     │
               ┌─────┴─────┐
               │ GitHub API │  5,000 req/hr (authenticated)
               └───────────┘
```

## Components

- **cmd/chimney/** — Go origin server: caching GitHub API proxy + static dashboard serving
- **deploy/chimney/Caddyfile** — Edge reverse proxy with TLS
- **deploy/chimney/setup.sh** — Server bootstrap script (idempotent)
- **docs/index.html** — Dashboard HTML (served by chimney)

## Server-side Caching

Two-tier cache architecture prevents multiple dashboard clients from burning GitHub API rate limits:

**Tier 1: Dragonfly (primary)**
- Redis-compatible in-memory store running on `127.0.0.1:6379`
- Persistent across chimney restarts (data survives process restarts)
- Shared across multiple chimney instances on the same box
- TTL-based automatic eviction — no manual LRU needed
- 128MB memory limit, 1 proactor thread (sized for cax11)

**Tier 2: In-memory (fallback)**
- Go map with 500-entry cap and LRU eviction
- Used when Dragonfly is unavailable (graceful degradation)

**Both tiers benefit from:**
- **Authenticated requests** — 5,000 req/hr (vs 60 unauthenticated)
- **ETag conditional requests** — 304s don't consume rate limit
- **Tiered TTLs** — 30s for workflow runs, 2min for issues, 5min for closed PRs
- **Stale-while-revalidate** — serves cached data if GitHub is down

## DNS

`chimney.cloudroof.eu` → A records pointing to both edge server IPs.
Provisioned via blinkinglight.
