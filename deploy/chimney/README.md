# chimney — beerpub.dev Dashboard Origin

chimney is the origin server for the wgmesh pipeline dashboard at `chimney.beerpub.dev`.

## Architecture

```
                    ┌──────────────────────┐
         ┌────────►│  chimney.beerpub.dev  │◄────────┐
         │         │        (DNS)          │         │
         │         └──────────────────────┘         │
         │                                           │
    ┌────┴────┐                           ┌──────────┴──┐
    │ edge-eu │  Helsinki (hel1)          │   edge-us   │  Ashburn (ash)
    │  Caddy  │◄──── wgmesh tunnel ──────►│    Caddy    │
    └────┬────┘                           └──────┬──────┘
         │                                       │
         └──────────┬────────────────────────────┘
                    │  (load balanced, WireGuard IPs)
         ┌──────────┴──────────────────────────────────┐
         │                                              │
    ┌────┴───────┐                           ┌──────────┴──┐
    │ chimney-   │  Nuremberg (nbg1)         │  chimney-   │  Helsinki (hel1)
    │ nbg1       │  blue/green origin        │  fsn1       │  blue/green origin
    │ :8081/8082 │                           │  :8081/8082 │
    └────┬───────┘                           └──────┬──────┘
         │                                          │
    ┌────┴──────┐                           ┌───────┴─────┐
    │ Dragonfly │  Redis-compat cache        │  Dragonfly  │
    │ 128MB RAM │  127.0.0.1:6379            │  128MB RAM  │
    └────┬──────┘                           └──────┬───────┘
         │                                         │
         └───────────────────┬─────────────────────┘
                             │
                       ┌─────┴─────┐
                       │ GitHub API │  5,000 req/hr (authenticated)
                       └───────────┘
```

> **Note:** `chimney-fsn1` is provisioned in hel1 — the name is historical;
> arm64 capacity in fsn1 was unavailable at provision time.

## Components

- **cmd/chimney/** — Go origin server: caching GitHub API proxy + static dashboard serving
- **deploy/chimney/Caddyfile.origin** — Origin Caddy config (HTTP-only, imports `upstream.conf`)
- **deploy/chimney/Caddyfile.edge** — Edge Caddy config (TLS termination, reverse proxy to origins)
- **deploy/chimney/compose.origin.yml** — Docker Compose for origin: chimney-blue, chimney-green, Dragonfly, Caddy
- **deploy/chimney/compose.edge.yml** — Docker Compose for edge: wgmesh, Caddy
- **deploy/chimney/bluegreen.sh** — Blue/green slot switch and Caddy upstream.conf update
- **deploy/chimney/setup-origin.sh** — Origin server bootstrap (idempotent)
- **deploy/chimney/setup-edge.sh** — Edge server bootstrap (idempotent)
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

## Blue/Green Deployment

Origins run two chimney slots (blue on `:8081`, green on `:8082`).
`bluegreen.sh` starts the inactive slot, waits for it to pass a health check,
then atomically rewrites `/etc/caddy/upstream.conf` and hot-reloads Caddy.

The deploy workflow (`chimney-deploy.yml`) discovers origin servers by the
`service=chimney,role=origin` label, runs `bluegreen.sh` via SSH, and then
runs smoke tests against the deployed slot.

## DNS

`chimney.beerpub.dev` → A records pointing to both edge server IPs.
All servers are labeled `service=chimney` at provision time.

## Deploy Constraints

These constraints are not obvious from the code alone — they caused the 2602211013
incident and must not be reverted.

### 1. Origin Caddy uses `auto_https off` and HTTP-only

Origins (`chimney-nbg1`, `chimney-fsn1`) serve plain HTTP on `:80`.
They are only reachable from edge Caddy nodes via the WireGuard mesh — not from
the public internet.
`auto_https off` in `Caddyfile.origin` prevents Caddy from requesting a TLS
certificate. Enabling TLS on origins causes 308 HTTP→HTTPS redirects; edge
health checks expect 200 and mark all backends unhealthy, producing 503s.

### 2. Caddy must run with `network_mode: host`

wgmesh runs with `cap_add: NET_ADMIN` and rewrites the host's iptables rules
as part of WireGuard interface management.
When Caddy runs in Docker bridge mode, Docker's DNAT rules (which map host
ports 80/443 to the container) are overwritten by wgmesh and port traffic no
longer reaches the container.
Setting `network_mode: host` in `compose.origin.yml` avoids bridge-mode DNAT
entirely — Caddy binds directly to the host network stack.

### 3. Caddyfile global block must precede all site blocks

Caddy requires the global options block (`{ ... }` with no host label) to
appear before any site blocks. Placing it after a site block causes Caddy to
exit with a parse error at startup.
This is validated in CI by the `validate-caddyfiles` job in `chimney-build.yml`
(`caddy validate` on both Caddyfiles before the Docker image is built).
