# Specification: Issue #259

## Classification
feature

## Deliverables
code + documentation

## Problem Analysis

There is no CLI flow for an origin server to register itself with the lighthouse. Origin servers must currently be registered manually by crafting raw HTTP requests to the lighthouse REST API (`POST /v1/sites`). This creates friction for the "join the mesh, that's it" experience described on cloudroof.eu.

The gap has two parts:

1. **Client-side (wgmesh CLI)**: No `origin` subcommand group exists in `main.go`. The existing subcommands (`join`, `init`, `status`, `peers`, etc.) handle mesh membership but not CDN origin registration.

2. **Server-side (lighthouse API)**: The existing `POST /v1/sites` endpoint requires the caller to know their WireGuard mesh IP in advance. There is no combined "register me as an origin" endpoint that looks up the mesh IP automatically or creates an org on first use.

## Proposed Approach

### Step 1 — Add `POST /v1/register-origin` to the lighthouse API (`pkg/lighthouse/api.go`)

Add a new convenience endpoint that:
- Accepts: `{ domain, mesh_ip, port, health_path, org_name, protocol, tls }`
- If `org_name` is provided and no `Authorization` header is given, creates the org and returns its API key alongside the site. (Bootstrap path — mirrors `POST /v1/orgs`.)
- If an `Authorization` header is present, uses the authenticated org and calls existing `store.CreateSite`.
- If the domain already exists (`already registered` conflict), returns the existing site record with a `409 Conflict` so the CLI can detect "already registered on this lighthouse" vs "first registration".
- Returns a combined response: `{ org, api_key, site, dns_instructions }`.

New route registration in `registerRoutes()`:
```go
a.mux.HandleFunc("POST /v1/register-origin", a.handleRegisterOrigin)
```

The handler validates the same fields as `handleCreateSite` (mesh_ip valid IP, port 1-65535, protocol http/https) plus `health_path` (optional, stored in `Origin`). To store `health_path`, the `Origin` struct in `pkg/lighthouse/types.go` gains one new field:
```go
type Origin struct {
    MeshIP     string `json:"mesh_ip"`
    Port       int    `json:"port"`
    Protocol   string `json:"protocol"`
    HealthPath string `json:"health_path,omitempty"`
}
```

### Step 2 — Add `origin` subcommand group to the CLI (`main.go`)

Add a `case "origin":` branch in the `main()` switch that dispatches to `originCmd()`.

`originCmd()` parses `os.Args[2]` to dispatch:
- `register` → `originRegisterCmd()`
- `list`     → `originListCmd()`
- `remove`   → `originRemoveCmd()`

#### `wgmesh origin register`

Flags:
- `--lighthouse` (required) — URL of lighthouse, e.g. `https://lighthouse.cloudroof.eu`
- `--api-key` (optional) — org API key; if absent, `--org-name` triggers unauthenticated bootstrap
- `--org-name` (optional, used only when `--api-key` is absent)
- `--domain` (required) — domain to register
- `--port` (required) — origin port
- `--protocol` (default `http`) — `http` or `https`
- `--health-path` (default `/healthz`) — origin health check path
- `--interface` (optional) — WireGuard interface name; auto-detected per platform (`wg0` on Linux, `utun20` on macOS) if not specified, matching the same logic used by `joinCmd()`
- `--tls` (default `auto`) — TLS mode

Flow:
1. Read local WireGuard mesh IP from the named interface using `net.InterfaceByName` + addr iteration (find the first non-link-local IPv4 in the mesh subnet).
2. Build request body, POST to `<lighthouse>/v1/register-origin`.
3. On success (201): print site ID, domain, DNS instructions (CNAME `<domain>` → `<dns_target>`).
4. On 409 conflict: print "Domain already registered — your origin may already be active."
5. On auth error: suggest running without `--api-key` to bootstrap a new org.

#### `wgmesh origin list`

Flags:
- `--lighthouse` (required)
- `--api-key` (required)

GETs `<lighthouse>/v1/sites`, prints a table:
```
DOMAIN              MESH IP         PORT   PROTOCOL  STATUS
blog.example.com    10.99.1.5       8080   http      active
```

#### `wgmesh origin remove`

Flags:
- `--lighthouse` (required)
- `--api-key` (required)
- `--domain` (required, or `--site-id`) — identifies site to remove

Flow:
1. GET `/v1/sites`, find site by domain.
2. DELETE `/v1/sites/{site_id}`.
3. Print confirmation.

### Step 3 — Update `printUsage()` and help text

Add the new subcommands to the usage block under a new `CDN ORIGIN SUBCOMMANDS` section.

## Affected Files

### Code changes
- `pkg/lighthouse/types.go` — add `HealthPath string` to `Origin` struct
- `pkg/lighthouse/api.go` — add `handleRegisterOrigin` handler and register `POST /v1/register-origin` route
- `pkg/lighthouse/api_test.go` — tests for `handleRegisterOrigin` (validation, unauthenticated bootstrap, authenticated path, 409 conflict)
- `main.go` — add `case "origin":` dispatch, add `originCmd`, `originRegisterCmd`, `originListCmd`, `originRemoveCmd` functions; update `printUsage()`

### Documentation changes
- `README.md` — add quick-start example showing `wgmesh origin register`

## Test Strategy

### Unit tests (pkg/lighthouse/api_test.go)

Add table-driven tests for `handleRegisterOrigin`:
- Missing domain → 400 `validation_error`
- Missing mesh_ip → 400 `validation_error`
- Invalid mesh_ip → 400 `validation_error`
- Invalid port → 400 `validation_error`
- Invalid protocol → 400 `validation_error`
- Unauthenticated with `org_name` (store nil) → verifies handler parses body correctly before hitting store
- Authenticated path (set `X-Org-ID` header, nil store) → verifies validation passes to store call

For store-dependent paths (org creation, conflict detection), integration tests would require a live Redis/Dragonfly and are out of scope for this spec; they follow the same pattern as existing store tests in `pkg/lighthouse/store.go` (no existing Redis integration tests in the repo, so these would be documented as manual test scenarios).

### CLI tests (main_test.go)

The existing `main_test.go` pattern only tests subcommand routing. Extend it to verify:
- `os.Args = ["wgmesh","origin"]` → exits non-zero with usage
- `os.Args = ["wgmesh","origin","register"]` without required flags → exits non-zero with error message

### Manual test flow

```bash
# 1. Start lighthouse (with dragonfly)
./cmd/lighthouse/main --addr :8080

# 2. On origin server, join mesh
wgmesh join --secret 'cdn-mesh-secret'

# 3. Bootstrap-register (no existing API key)
wgmesh origin register \
  --lighthouse http://localhost:8080 \
  --org-name 'my-org' \
  --domain 'blog.example.com' \
  --port 8080

# Expected: prints API key (save it), site ID, DNS instructions

# 4. List
wgmesh origin list \
  --lighthouse http://localhost:8080 \
  --api-key 'cr_...'

# 5. Remove
wgmesh origin remove \
  --lighthouse http://localhost:8080 \
  --api-key 'cr_...' \
  --domain 'blog.example.com'
```

## Estimated Complexity
medium

**Reasoning:**
- The lighthouse API change is straightforward: one new handler reusing existing validation logic and `store.CreateSite` / `auth.CreateOrgWithKey`.
- The CLI change follows the established `flag.NewFlagSet` + HTTP client pattern already used throughout `main.go` (e.g. `peersCmd`, `statusCmd`).
- WireGuard interface IP detection requires OS-level `net` package calls (cross-platform concern for macOS vs Linux).
- No new external dependencies needed — `net/http` client suffices for calling the lighthouse API.
- Estimated implementation: 3-4 hours including tests.
