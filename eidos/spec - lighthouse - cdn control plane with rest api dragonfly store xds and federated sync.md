---
tldr: Lighthouse is the multi-tenant CDN control plane: a REST API manages orgs/sites/edges backed by a local Dragonfly instance; mutations replicate over the WireGuard mesh via UDP LWW sync; edge nodes pull xDS config or a generated Caddyfile; origin health is tracked via local HTTP probes and edge-reported results.
category: core
---

# Lighthouse — CDN control plane with REST API, Dragonfly store, xDS, and federated sync

## Target

The centralized control plane for the cloudroof.eu edge CDN.
Each lighthouse instance manages site registrations, origin configurations, and edge topology.
Multiple lighthouse nodes stay consistent via last-writer-wins replication over the existing WireGuard mesh.

## Behaviour

### Domain model

- **Org** — tenant; owns API keys and sites.
- **APIKey** — bearer credential; format `cr_` + hex(32 bytes); stored as SHA-256 hash with 8-char prefix index.
- **Site** — a registered domain with an origin and TLS mode.
  - Lifecycle: `pending_dns → pending_verify → active` (or `suspended`, `dns_failed`, `deleted`).
  - TLS modes: `auto` (Let's Encrypt), `custom`, `off`.
  - Origin fields: `mesh_ip` (WireGuard mesh IP), `port`, `protocol` (http/https), `HealthCheck{path, interval, timeout, healthy, unhealthy}`.
  - `DNSTarget` — the CNAME target the customer must set (e.g. `edge.cloudroof.eu`).
  - CRDT fields: `Version int64`, `NodeID string` — used for LWW merge.
- **Edge** — a registered edge node (read-only via API).
- **ID generation** — `GenerateID(prefix)` = `prefix_` + hex(12 random bytes).

### REST API (`pkg/lighthouse/api.go`)

All responses are JSON; errors use RFC 7807 Problem Details (`application/problem+json`).

**Public (no auth):**

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | `{"status":"ok","service":"lighthouse"}` |
| `GET` | `/v1/openapi.json` | OpenAPI 3.1 spec (described as "Designed for LLM agents") |
| `POST` | `/v1/orgs` | Create org + first API key (bootstrap) |

**Authenticated (Bearer `cr_…`):**

| Method | Path | Description |
|---|---|---|
| `GET` | `/v1/orgs/{org_id}` | Get own org (scoped to caller's org) |
| `POST` | `/v1/orgs/{org_id}/keys` | Create additional API key |
| `POST` | `/v1/sites` | Register domain; starts in `pending_dns` |
| `GET` | `/v1/sites` | List own sites |
| `GET` | `/v1/sites/{site_id}` | Get site + per-edge health from reporter |
| `PATCH` | `/v1/sites/{site_id}` | Update origin or TLS mode |
| `DELETE` | `/v1/sites/{site_id}` | Soft-delete (tombstone kept for sync) |
| `GET` | `/v1/sites/{site_id}/dns-status` | Current DNS verification state |
| `GET` | `/v1/edges` | List all edges |
| `POST` | `/v1/sites/{site_id}/health` | Internal — edges report probe results here |

**Auth flow:** `requireAuth` extracts the Bearer token, validates via `Auth.Authenticate`, and stores the resolved `org_id` in `X-Org-ID`. The `rateLimit` middleware reads `X-Org-ID` — it must be chained after auth.

**Rate limiting:** per-org token bucket. Headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`. On `429`: adds `Retry-After` header (whole seconds, rounded up).

**Site validation:** domain lowercased/trimmed; `mesh_ip` must be a valid IP; port 1–65535; protocol `http`/`https` (default `http`); TLS `auto`/`custom`/`off` (default `auto`).

### Authentication (`pkg/lighthouse/auth.go`)

- Keys stored as `SHA-256(rawKey)` — not bcrypt; justified by high-entropy format.
- Lookup by 8-char prefix (avoids full scan); constant-time comparison via `crypto/subtle`.
- `last_used_at` updated asynchronously (best-effort — fire-and-forget goroutine).
- `CreateOrgWithKey`: atomic org + key creation.

### Storage (`pkg/lighthouse/store.go`)

Backed by a local Dragonfly instance (Redis protocol), **DB 1** (DB 0 is chimney's).
Read/write timeout: 200ms; dial timeout: 2s.

**Key schema:**

| Key | Value |
|---|---|
| `lh:org:<id>` | JSON Org |
| `lh:site:<id>` | JSON Site (tombstones kept on delete) |
| `lh:key:<prefix>` | JSON APIKey |
| `lh:edge:<id>` | JSON Edge |
| `lh:idx:orgs` | SET of org IDs |
| `lh:idx:sites:<org_id>` | SET of site IDs per org |
| `lh:idx:allsites` | SET of all site IDs (for xDS) |
| `lh:idx:keys:<org_id>` | SET of key IDs per org |
| `lh:idx:edges` | SET of edge IDs |
| `lh:domain:<domain>` | `site_id` (domain uniqueness map) |

- **Domain uniqueness**: `SetNX` (atomic set-if-not-exists).
- **Soft delete**: site status set to `deleted`, tombstone kept at `lh:site:<id>`; removed from `lh:idx:allsites` and `lh:domain:<domain>`.
- **Pub-sub**: `OnWrite(fn)` registers listeners; every mutation calls `notify(SyncMessage)` — used by the sync layer.

**LWW merge rules (`ApplySync`):**

- **Sites**: `remote.Version > local.Version` wins; tie-break on `UpdatedAt` (later wins); final tie-break on `NodeID` (lexicographic, higher wins).
- **Orgs**: append-only; latest `UpdatedAt` wins.
- **Keys**: append-only; accept if not already present.

### xDS endpoint (`pkg/lighthouse/xds.go`)

REST xDS — no gRPC, no protobuf. Edge nodes and proxies pull config via HTTP.

- `GET /v1/xds/config` → `EnvoySnapshot{version, timestamp, clusters[], routes[]}`.
  - Includes only sites with status `active`, `pending_dns`, or `pending_verify`.
  - Each cluster entry maps a site's domain to its origin `mesh_ip:port`.
- `GET /v1/xds/caddyfile` → dynamically generated Caddyfile for edge Caddy instances.
  - Each site block: `reverse_proxy` to `mesh_ip:port`, `health_uri` from `HealthCheck.Path`, adds `X-Served-By` and `X-CDN` response headers.

### DNS verification (`cmd/lighthouse/main.go`)

Background loop (60s interval):

- Queries all `pending_dns` sites.
- For each: calls `VerifyDNS` — checks CNAME suffix match (`domain CNAME → …dnsTarget`) or A/AAAA record match against expected IPs.
- On success: transitions site to `active` via `UpdateSite`.
- On timeout (site `CreatedAt` > 24h old): transitions to `dns_failed`.

`Resolver` interface in `dns.go` is injected for testability.

### Origin health checking (`pkg/lighthouse/health.go`)

Two independent systems:

**1. `Checker` — local HTTP probes (lighthouse-originated):**
- One goroutine per site that has a non-empty `HealthCheck.Path`.
- Default interval: 10s; default timeout: 5s (per-site config overrides).
- Probe: `GET protocol://mesh_ip:port/health.path`; 2xx–3xx = pass, else fail.
- Thresholds: 2 consecutive failures → `unhealthy`; 2 consecutive passes → `healthy`.
- State is in-memory (not persisted).

**2. `HealthReporter` — edge-reported results:**
- Edges POST their probe results to `POST /v1/sites/{site_id}/health`.
- Reporter stores latest result per `(siteID, edgeID)` pair, in memory only.
- `GET /v1/sites/{site_id}` includes `origin_health: {edgeID: OriginHealth}` from the reporter.

### Federated sync (`pkg/lighthouse/sync.go`)

Replication between lighthouse instances over the WireGuard mesh.

- **Transport**: UDP on port 51821 (SyncPort), bound to the lighthouse's mesh IP; falls back to `0.0.0.0` if bind fails.
- **Max message size**: 65,000 bytes (WireGuard MTU safe).
- **Push on mutation**: every `Store.notify()` call triggers an immediate UDP push to all registered peers.
- **Periodic full-state push**: every 15s, all non-deleted sites are pushed to all peers (catches lost datagrams).
- Full-state push covers sites only — not orgs or keys.
- Incoming messages from own `nodeID` are discarded.
- Peers registered statically via `-peer <mesh_ip>` flags at startup.

### Command-line startup (`cmd/lighthouse/main.go`)

Flags:
- `-addr` — HTTP listen address.
- `-redis` — Dragonfly address.
- `-mesh-ip` — this node's mesh IP (enables sync; optional).
- `-node-id` — unique ID for LWW tie-breaking.
- `-dns-target` — CNAME target for site DNS validation.
- `-rate-limit-rps`, `-rate-limit-burst` — token bucket parameters.
- `-proxy-addr`, `-proxy-origins` — optional reverse proxy (`pkg/proxy`).
- `-peer` (repeatable) — mesh IPs of other lighthouse instances.

Mesh sync starts only if `-mesh-ip` is provided.

## Design

- **Dragonfly DB 1 isolation**: lighthouse and chimney share the same Dragonfly instance but use separate DBs — no key namespace collisions.
- **REST xDS over gRPC**: avoids protobuf dependency; edge nodes and Caddy instances can pull config with a plain HTTP client. The trade-off is polling-based rather than push-based config delivery.
- **LWW tie-breaking on NodeID**: deterministic conflict resolution without coordination — any pair of nodes independently arrives at the same winner.
- **Tombstones on delete**: deleted sites are kept in Redis with `status=deleted` so sync can propagate deletes to other nodes. The tombstone is needed because absence from the remote's index is ambiguous (could be a new node that hasn't received the site yet).
- **Two health systems**: the `Checker` probes from the lighthouse's perspective (useful for control plane decisions); the `HealthReporter` aggregates edge-perspective probes (useful for knowing what the edge nodes actually see at each PoP).
- **SHA-256 not bcrypt for API keys**: keys are 32 random bytes (`cr_` + hex) — brute-forcing the hash is computationally equivalent to guessing the key directly; PBKDF2/bcrypt's iteration cost buys nothing against high-entropy inputs.
- **OpenAPI 3.1 "for LLM agents"**: the spec is hand-written and served at runtime, making the API self-describing for automated clients without an external docs site.

## Interactions

- `pkg/crypto.MembershipKey` — `ValidateMembershipToken` used for node admission (not in this package directly; used at the proxy layer or a future auth path).
- `pkg/ratelimit` — `IPRateLimiter` with per-org rate limiting keyed on org ID.
- `pkg/proxy` — optional reverse proxy spawned alongside lighthouse.
- Edge nodes — pull xDS config or Caddyfile; POST health reports.
- WireGuard mesh — sync messages travel as UDP over mesh tunnels; Sync binds to mesh IP.

## Mapping

> [[pkg/lighthouse/types.go]]
> [[pkg/lighthouse/auth.go]]
> [[pkg/lighthouse/dns.go]]
> [[pkg/lighthouse/xds.go]]
> [[pkg/lighthouse/api.go]]
> [[pkg/lighthouse/store.go]]
> [[pkg/lighthouse/health.go]]
> [[pkg/lighthouse/sync.go]]
> [[cmd/lighthouse/main.go]]
