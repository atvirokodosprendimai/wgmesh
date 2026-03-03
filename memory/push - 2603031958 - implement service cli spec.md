---
tldr: Changes needed to implement wgmesh service add/list/remove per service CLI spec
---

# Push — implement service CLI spec

Spec: [[spec - service cli - register local services for managed ingress via lighthouse]]

## Change inventory

### 1. Mesh ID derivation (`pkg/crypto/derive.go`)
- **Status:** Missing
- **Need:** `MeshID()` function that returns first 6 bytes of NetworkID hex-encoded (12 chars)
- **Rationale:** Used in domain names: `<name>.<mesh-id>.wgmesh.dev`
- Small addition to existing file, no structural change

### 2. Lighthouse API client (`pkg/lighthouse/client.go`)
- **Status:** Does not exist
- **Need:** HTTP client for Lighthouse REST API
  - `CreateSite(domain, origin)` → calls `POST /v1/sites`
  - `ListSites()` → calls `GET /v1/sites`
  - `DeleteSite(id)` → calls `DELETE /v1/sites/{id}`
  - Auth via Bearer token (`cr_...`)
  - Lighthouse URL discovery: SRV lookup → fallback URL
- Types already exist in `pkg/lighthouse/types.go` (Site, Origin, etc.)

### 3. Local service state (`pkg/mesh/services.go`)
- **Status:** Does not exist
- **Need:** Read/write `/var/lib/wgmesh/services.json`
  - `LoadServices(path)` → read from disk
  - `SaveServices(path, services)` → write atomically
  - `ServiceEntry` type with site_id, name, domain, local_addr, protocol, registered_at

### 4. Account storage
- **Status:** Does not exist
- **Need:** Read/write `/var/lib/wgmesh/account.json`
  - Stores API key (`cr_...`) and lighthouse URL
  - Written by `wgmesh join --account <key>` (future, for now by `service add --account`)
  - Read by all `service` subcommands

### 5. CLI subcommand (`main.go`)
- **Status:** `service` subcommand does not exist
- **Need:** `serviceCmd()` dispatching to `add`, `list`, `remove`
  - Follows `peersCmd()` pattern (sub-subcommand dispatch)
  - `--secret` flag (or `WGMESH_SECRET` env var) on all subcommands
  - `service add`: `--protocol`, `--health-path`, `--health-interval`, `--account`
  - `service list`: `--json`
  - `service remove`: positional `<name>`

### 6. Tests
- Unit tests for mesh ID derivation
- Unit tests for local service state load/save
- Unit tests for lighthouse client (mock HTTP server)
- Integration-style test for service add flow
