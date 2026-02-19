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

The chimney origin proxies GitHub API requests with:
- **Authenticated requests** — 5,000 req/hr (vs 60 unauthenticated)
- **ETag conditional requests** — 304s don't consume rate limit
- **Tiered TTLs** — 30s for workflow runs, 2min for issues, 5min for closed PRs
- **Stale-while-revalidate** — serves cached data if GitHub is down
- **In-memory cache** — no external dependencies, bounded at 500 entries

## DNS

`chimney.cloudroof.eu` → A records pointing to both edge server IPs.
Provisioned via blinkinglight.
