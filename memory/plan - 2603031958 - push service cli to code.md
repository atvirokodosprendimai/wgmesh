---
tldr: Implement wgmesh service add/list/remove CLI per service CLI spec — direct Lighthouse API calls with secret-derived mesh ID
status: active
---

# Plan: Push service CLI to code

## Context

- Spec: [[spec - service cli - register local services for managed ingress via lighthouse]]
- Push doc: [[push - 2603031958 - implement service cli spec]]
- Stage 0 exit gate for [[spec - first-customer - roadmap to first paying customer]]

## Phases

### Phase 1 - Foundation types and derivation - status: open

1. [ ] Add `MeshID()` to `pkg/crypto/derive.go`
   - Returns hex(NetworkID[:6]) — 12-char deterministic mesh identifier
   - Add test in `pkg/crypto/derive_test.go`
2. [ ] Create `pkg/mesh/services.go` — local service state
   - `ServiceEntry` struct: site_id, name, domain, local_addr, protocol, registered_at
   - `ServiceState` struct with map[string]ServiceEntry
   - `LoadServices(path) (ServiceState, error)` — read from disk, return empty if missing
   - `SaveServices(path, ServiceState) error` — atomic write (write tmp + rename)
   - Add test in `pkg/mesh/services_test.go`
3. [ ] Create `pkg/mesh/account.go` — account credential storage
   - `AccountConfig` struct: api_key, lighthouse_url (discovered or explicit)
   - `LoadAccount(path) (AccountConfig, error)` — read from disk
   - `SaveAccount(path, AccountConfig) error` — atomic write
   - Add test in `pkg/mesh/account_test.go`

### Phase 2 - Lighthouse client - status: open

4. [ ] Create `pkg/lighthouse/client.go` — HTTP client for Lighthouse REST API
   - `Client` struct: base URL, API key, http.Client
   - `NewClient(baseURL, apiKey string) *Client`
   - `CreateSite(req CreateSiteRequest) (*Site, error)` — POST /v1/sites
   - `ListSites() ([]Site, error)` — GET /v1/sites
   - `DeleteSite(id string) error` — DELETE /v1/sites/{id}
   - Use existing types from `types.go` (Site, Origin, etc.)
5. [ ] Add `DiscoverLighthouse(meshID string) (string, error)` to client.go
   - SRV lookup: `_lighthouse._tcp.<mesh-id>.wgmesh.dev`
   - Fallback: `https://lighthouse.<mesh-id>.wgmesh.dev`
6. [ ] Add test in `pkg/lighthouse/client_test.go`
   - Mock HTTP server for create/list/delete
   - Test error handling (unreachable, 401, 404)

### Phase 3 - CLI wiring - status: open

7. [ ] Add `serviceCmd()` to `main.go`
   - Sub-subcommand dispatch: add, list, remove
   - Follow `peersCmd()` pattern
   - `--secret` flag + `WGMESH_SECRET` env var fallback on all subcommands
8. [ ] Implement `service add <name> <local-addr>`
   - Validate name: lowercase alphanumeric + hyphens
   - Parse local-addr: `[host]:port`
   - Derive mesh IP from secret (via daemon.NewConfig → DeriveKeys)
   - Derive mesh ID (via crypto.MeshID)
   - Build domain: `<name>.<mesh-id>.wgmesh.dev`
   - Load account from `/var/lib/wgmesh/account.json`
   - If no account: `--account` flag to provide + save
   - Call Lighthouse CreateSite
   - Save to local services.json
   - Print managed URL
9. [ ] Implement `service list`
   - Load account, discover Lighthouse
   - Call ListSites, filter by this node's mesh IP
   - Fallback to local services.json if Lighthouse unreachable
   - Tabular output (NAME, URL, PORT, STATUS)
   - `--json` flag for machine-readable output
10. [ ] Implement `service remove <name>`
    - Look up by name in local state or via ListSites
    - Call DeleteSite
    - Remove from local services.json
    - Print confirmation

### Phase 4 - Integration and verification - status: open

11. [ ] End-to-end test: add → list → remove cycle
    - Spin up mock Lighthouse HTTP server
    - Run service add, verify site created + local state written
    - Run service list, verify output
    - Run service remove, verify deleted + local state cleaned
12. [ ] Verify claim checklist against spec
    - Each Behaviour claim mapped to implementation
    - Each Verification item confirmed

## Verification

- `go build` succeeds
- `go test ./...` passes
- `wgmesh service add ollama :11434 --secret <test-secret> --account <test-key>` prints managed URL
- `wgmesh service list --secret <test-secret>` shows registered service
- `wgmesh service remove ollama --secret <test-secret>` deregisters
- Graceful errors when Lighthouse unreachable or no account configured

## Progress Log

