# Decision — decouple Lighthouse from wgmesh into separate repo

Date: 2026-03-15

## Context

Lighthouse (`cmd/lighthouse/`, `pkg/lighthouse/`) is the CDN control plane — a multi-tenant REST API with Dragonfly store, xDS config, federated sync, DNS verification, and origin health checking.
It currently lives inside the wgmesh module alongside the mesh networking code.

The `wgmesh service` CLI subcommand (in `service.go`) imports `pkg/lighthouse` to talk to the Lighthouse API.
The `--account` flag on `join` and `install-service` saves a Lighthouse API key to disk.

Both products are evolving and will increasingly diverge in release cadence, dependencies, and deployment targets.

## Options

### 1. Keep everything in one repo (status quo)

**Gains:**
- Single `go.mod`, single CI pipeline
- Cross-cutting refactors are atomic
- No versioning overhead between client and server

**Loses:**
- Lighthouse inherits all of wgmesh's dependencies (WireGuard, DHT, gossip, crypto)
- wgmesh binary includes Lighthouse types even for mesh-only users
- Release cycles are coupled — a Lighthouse API change forces a wgmesh release
- `cmd/lighthouse/` imports wgmesh packages it doesn't need
- Contributors to one product must clone and build the other

### 2. Extract Lighthouse to a separate repo (own binary, own SDK)

**Gains:**
- Independent release cycles, CI, and deployment
- Lighthouse can evolve its API without touching wgmesh
- wgmesh stays focused: mesh networking only
- `lighthouse-go` client SDK gets explicit versioning — breaking changes are visible in `go.mod`
- Each repo has minimal dependency surface

**Loses:**
- Cross-repo coordination for API changes
- Need a published client SDK package (`lighthouse-go` or similar)
- `wgmesh service` would import an external versioned SDK instead of in-tree types
- Two repos to maintain, two CI configs
- Shared types (if any) need a decision on ownership

### 3. Monorepo with Go workspace (multi-module)

**Gains:**
- Single repo, but separate `go.mod` files — independent dependency trees
- Atomic cross-cutting changes still possible
- `replace` directives for local development

**Loses:**
- Go workspaces add tooling complexity
- CI must handle selective builds
- Doesn't solve the conceptual coupling — `service.go` still directly imports lighthouse types
- Defers the real question: are these one product or two?

### 4. Remove `wgmesh service` — let Lighthouse own its CLI surface

**Gains:**
- wgmesh has zero Lighthouse dependency — clean separation
- `--account` flag disappears from `join` — mesh joining is purely about mesh
- Lighthouse publishes its own CLI (e.g. `lighthouse service add`) with its own auth flow
- Each product is fully self-contained

**Loses:**
- Users run two CLIs instead of one
- The "one command to register a service" UX story weakens
- Lighthouse CLI needs to derive mesh IP — must import `pkg/crypto` or duplicate derivation logic

## Decision

**Option 2 — extract Lighthouse to a separate repo, with a published client SDK.**

**Why:** wgmesh and Lighthouse are different products for different users.
wgmesh is infrastructure (mesh networking for sysadmins/operators).
Lighthouse is a platform service (CDN control plane for application developers).
Coupling them in one module means every Lighthouse API change drags wgmesh along, and every wgmesh crypto change risks breaking Lighthouse.

Option 4 is cleaner architecturally but worse for UX — the integrated `wgmesh service add` flow is a key first-customer experience.
The compromise: keep `wgmesh service` but have it import a separately-versioned Lighthouse client SDK, making the API contract explicit.

Option 3 adds complexity without addressing the conceptual coupling — it's a half-measure.

## Consequences

### Repos after extraction

- `wgmesh` — mesh networking (daemon, discovery, crypto, RPC, centralized mode)
- `lighthouse` — CDN control plane (API, store, sync, xDS, DNS, health)
- `lighthouse-go` (or `lighthouse/sdk`) — Go client SDK for the Lighthouse API

### Changes to wgmesh

1. **Remove `cmd/lighthouse/`** — moves to lighthouse repo
2. **Remove `pkg/lighthouse/`** — server code moves to lighthouse repo; `client.go` becomes the seed of the SDK
3. **`service.go`** — import the published `lighthouse-go` SDK instead of in-tree `pkg/lighthouse`
4. **`--account` flag** — keep on `join`/`install-service` for now; it saves a credential that `service` commands use. This is wgmesh storing a config value, not importing Lighthouse server code
5. **`pkg/mesh/account.go`** — stays in wgmesh (it's just JSON file I/O for credential storage)
6. **Specs to update:**
   - [[spec - lighthouse - cdn control plane with rest api dragonfly store xds and federated sync]] — moves to lighthouse repo
   - [[spec - service cli - register local services for managed ingress via lighthouse]] — update imports, add SDK dependency
   - [[spec - cli entry point - dual mode dispatch with daemon wiring and rpc server]] — remove lighthouse references

### Shared code question

`pkg/crypto` has mesh IP derivation (`DeriveMeshIP`, `DeriveMeshIPInSubnet`) that both wgmesh and Lighthouse need.
Options:
- Lighthouse SDK includes a minimal derivation function (copy, not import)
- Extract `crypto` primitives to a tiny shared module
- Lighthouse queries mesh IP from the running daemon via RPC instead of re-deriving

This is a follow-up decision — doesn't block extraction.

### Migration order

1. ~~Publish `lighthouse-go` client SDK (extract from `pkg/lighthouse/client.go` + types)~~ — done: v0.1.0 published
2. ~~Update `service.go` to import the SDK~~ — done: `2e36371`
3. Move `cmd/lighthouse/` + `pkg/lighthouse/` server code to lighthouse repo
4. Remove from wgmesh, update specs

## Status

in-progress (steps 1-2 complete)
