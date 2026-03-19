# Specification: Issue #443

## Classification
feature

## Deliverables
code + documentation

## Problem Analysis

`wgmesh` provides a WireGuard mesh network but lacks a way for users to expose local AI services (e.g. Ollama, ComfyUI, LocalAI) to other mesh members via managed HTTPS ingress. Without this, users must manually configure reverse proxies and DNS, defeating the purpose of a "mesh that just works."

The `wgmesh service add` command must:
1. Accept a service name and local address (host:port or :port).
2. Derive a stable public domain name from the mesh secret and service name: `<name>.<meshID>.wgmesh.dev`.
3. Register an origin with the Lighthouse CDN control plane (via the `lighthouse-go` SDK), which provisions DNS + TLS automatically.
4. Persist registration metadata locally to `$STATE_DIR/services.json` so subsequent `service list` and `service remove` commands work offline.

The command must also handle three secondary concerns:
- **Account management**: The Lighthouse API key (`cr_...`) must be stored in `$STATE_DIR/account.json` and re-used on subsequent invocations.
- **Mesh IP derivation**: The origin address sent to Lighthouse is the node's WireGuard mesh IP. If the daemon has already run and persisted `wg0.json`, that pubkey is used; otherwise a warning is printed and a placeholder is derived so the command does not fail.
- **Custom mesh subnets**: If `--mesh-subnet` is provided, mesh IP derivation must use `crypto.DeriveMeshIPInSubnet` instead of the default subnet.

### Existing Implementation (as of this spec)

The command is fully implemented in `service.go` (package `main`) and wired into `main.go` at the `service` subcommand. The supporting data layer is in `pkg/mesh/services.go` and `pkg/mesh/account.go`. Tests exist in `service_test.go` and `pkg/mesh/services_test.go`.

This spec documents the design for review and as a reference for future changes.

## Implementation Tasks

### Task 1: CLI entry point (`service.go` — `serviceCmd` / `serviceAddCmd`)

**File:** `service.go`

Implement three sub-commands dispatched from `serviceCmd()`:

| Sub-command | Function |
|---|---|
| `service add <name> <local-addr> [flags]` | `serviceAddCmd()` |
| `service list [flags]` | `serviceListCmd()` |
| `service remove <name> [flags]` | `serviceRemoveCmd()` |

#### `serviceAddCmd` — exact behavior

1. **Flag parsing** — `flag.NewFlagSet("service add", flag.ExitOnError)`:
   - `--secret string` — mesh secret URI or raw base64 (required; fallback to `WGMESH_SECRET` env var)
   - `--mesh-subnet string` — custom IPv4 CIDR (optional; default: derived from secret)
   - `--protocol string` — `http` or `https` (default: `"http"`)
   - `--health-path string` — health check URL path (default: `"/"`)
   - `--health-interval duration` — health check interval (default: `30s`)
   - `--account string` — Lighthouse API key `cr_...` (saved for future use)
   - `--state-dir string` — state directory (default: `"/var/lib/wgmesh"`)
   - Positional args: `args[0]` = service name, `args[1]` = local address

2. **Input validation** (exit 1 on failure):
   - Secret must be non-empty after `resolveSecret()`.
   - Service name must match `^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$` (lowercase alphanumeric + hyphens, no leading/trailing hyphens).
   - Protocol must be `"http"` or `"https"`.
   - Local address parsed by `parseLocalAddr()`: accepts `:PORT`, `HOST:PORT`, `[IPv6]:PORT`; port must be 1–65535.

3. **Key derivation**:
   - Call `crypto.DeriveKeys(resolvedSecret)` → `*crypto.DerivedKeys`.
   - Call `keys.MeshID()` → 12-char hex string for domain construction.
   - Parse custom subnet with `crypto.ParseSubnetOrDefault(*meshSubnet)`.

4. **Account resolution** — `resolveAccount(accountPath, *account)`:
   - If `--account` flag provided: load existing account (ignoring not-found), update `APIKey`, save atomically to `account.json` (0600, parent dirs 0700), return updated config.
   - Else: load from disk; return error (with hint message) if not found.

5. **Lighthouse URL discovery**:
   - Use `acct.LighthouseURL` if set.
   - Else call `lighthouse.DiscoverLighthouse(meshID)` → URL string.

6. **Domain construction**: `fmt.Sprintf("%s.%s.%s", name, meshID, "wgmesh.dev")`

7. **Mesh IP derivation** — `deriveMeshIPForService(keys, resolvedSecret, customSubnet)`:
   - Read `$STATE_DIR/wg0.json`; if it contains `public_key`, call `crypto.DeriveMeshIP` (default subnet) or `crypto.DeriveMeshIPInSubnet` (custom subnet).
   - If file missing or key empty: print warning to stderr about running `wgmesh join` first, then derive placeholder using `"unjoined"` as the pubkey input.

8. **Lighthouse API call** — `lighthouse.NewClient(lighthouseURL, acct.APIKey).CreateSite(req)`:
   ```go
   lighthouse.CreateSiteRequest{
       Domain: domain,
       Origin: lighthouse.Origin{
           MeshIP:   meshIP,
           Port:     port,
           Protocol: *protocol,
           HealthCheck: lighthouse.HealthCheck{
               Path:     *healthPath,
               Interval: *healthInterval,
           },
       },
       TLS: "auto",
   }
   ```
   Exit 1 on error.

9. **Local state persistence**:
   - Load `$STATE_DIR/services.json` (ignore not-found; warn on other errors).
   - Upsert `mesh.ServiceEntry{SiteID, Name, Domain, LocalAddr, Protocol, RegisteredAt: time.Now().UTC()}`.
   - Save atomically via `mesh.SaveServices`.

10. **Success output** (stdout):
    ```
    Service registered: <name>
      URL:    https://<domain>
      Origin: <meshIP> (port <port>, <protocol>)
      Status: <site.Status>
    ```

#### `serviceListCmd` — exact behavior

1. Flags: `--secret`, `--json`, `--state-dir`.
2. Derive `meshID` from secret.
3. Attempt Lighthouse list (`client.ListSites()`); on any failure fall back to local `services.json`.
4. If no sites: print `"No services registered"`.
5. Default tabular output with header `NAME\tURL\tPORT\tSTATUS`; `--json` outputs `json.MarshalIndent`.
6. Service name extracted from domain by stripping `.<meshID>.wgmesh.dev` suffix.

#### `serviceRemoveCmd` — exact behavior

1. Flags: `--secret`, `--state-dir`.
2. Look up service in local `services.json`; exit 1 if not found.
3. If account is configured: call `client.DeleteSite(entry.SiteID)`; on error print warning and continue.
4. Remove entry from local state and save.
5. Print `"Service removed: <name>"`.

### Task 2: Data model (`pkg/mesh/services.go`)

**File:** `pkg/mesh/services.go`

```go
// ServiceEntry tracks a single registered service.
type ServiceEntry struct {
    SiteID       string    `json:"site_id"`
    Name         string    `json:"name"`
    Domain       string    `json:"domain"`
    LocalAddr    string    `json:"local_addr"`
    Protocol     string    `json:"protocol"`
    RegisteredAt time.Time `json:"registered_at"`
}

// ServiceState holds all locally registered services.
type ServiceState struct {
    Services map[string]ServiceEntry `json:"services"`
}
```

- `LoadServices(path string) (ServiceState, error)`: returns empty state (not error) if file does not exist.
- `SaveServices(path string, state ServiceState) error`: atomic write (write `.tmp` then `os.Rename`), parent dirs created at 0700, file written at 0600.

### Task 3: Account config (`pkg/mesh/account.go`)

**File:** `pkg/mesh/account.go`

```go
type AccountConfig struct {
    APIKey        string `json:"api_key"`
    LighthouseURL string `json:"lighthouse_url,omitempty"`
}
```

- `LoadAccount(path string) (AccountConfig, error)`: error if not found or `api_key` empty.
- `SaveAccount(path string, cfg AccountConfig) error`: same atomic write pattern as `SaveServices`.

### Task 4: Helper functions (`service.go`)

| Function | Signature | Behavior |
|---|---|---|
| `resolveSecret` | `(flagValue string) string` | Returns `normalizeSecret(flagValue)` or `normalizeSecret(os.Getenv("WGMESH_SECRET"))` |
| `normalizeSecret` | `(input string) string` | Strips `wgmesh://`, optional `v1/` segment, and `?...` query string |
| `parseLocalAddr` | `(addr string) (int, error)` | Parses `:PORT`, `HOST:PORT`, `[IPv6]:PORT`, bare number; validates 1–65535 |
| `validatePort` | `(port int) (int, error)` | Returns error if port < 1 or > 65535 |
| `extractServiceName` | `(domain, meshID string) string` | Strips `.<meshID>.wgmesh.dev` suffix; returns full domain if no match |
| `parsePortFromAddr` | `(addr string) int` | Calls `parseLocalAddr`; returns 0 on error |

### Task 5: Wire into `main.go`

**File:** `main.go`

Add `"service"` case to the subcommand switch:
```go
case "service":
    serviceCmd()
    return
```

Constants required in `service.go` (or `main.go` package scope):
```go
const (
    defaultStateDir  = "/var/lib/wgmesh"
    servicesFileName = "services.json"
    accountFileName  = "account.json"
    managedDomain    = "wgmesh.dev"
)

var validServiceName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)
```

### Task 6: Tests

**File:** `service_test.go` (package `main`)

Write table-driven tests for:

1. `TestParseLocalAddr` — covers `:PORT`, `HOST:PORT`, `[IPv6]:PORT`, invalid inputs (`:0`, `:70000`, bare `abc`).
2. `TestValidServiceName` — valid: `ollama`, `my-api`, `a`, `web-server-1`; invalid: `""`, `-bad`, `bad-`, `BAD`, names with spaces/dots/underscores.
3. `TestExtractServiceName` — domain with known meshID suffix, domain without suffix returns full domain.
4. `TestResolveSecret` — flag takes precedence over `WGMESH_SECRET` env var.
5. `TestResolveAccount` — flag saves and returns; subsequent call without flag loads from disk; updating key preserves existing `LighthouseURL`; missing file without flag returns error.
6. `TestServiceEndToEnd` — uses `httptest.NewServer` as a mock Lighthouse. Exercises add → list → remove cycle: verify site created with correct domain/origin, verify local `services.json` updated, verify delete removes from both Lighthouse and local state.

**File:** `pkg/mesh/services_test.go` (package `mesh`)

1. `TestLoadServicesEmpty` — non-existent path returns empty state, no error.
2. `TestSaveAndLoadServices` — round-trip preserves all fields.
3. `TestSaveServicesAtomicWrite` — no `.tmp` file after success; permissions are 0600.
4. `TestSaveServicesCreatesDirectory` — nested directories created automatically.

## Affected Files

| File | Change |
|---|---|
| `service.go` | New file — `serviceCmd`, `serviceAddCmd`, `serviceListCmd`, `serviceRemoveCmd`, helper functions |
| `service_test.go` | New file — unit + integration tests for service commands |
| `main.go` | Add `"service"` case to subcommand switch; add `defaultStateDir` constant if not already present |
| `pkg/mesh/services.go` | New file — `ServiceEntry`, `ServiceState`, `LoadServices`, `SaveServices` |
| `pkg/mesh/services_test.go` | New file — tests for services data layer |
| `pkg/mesh/account.go` | New file — `AccountConfig`, `LoadAccount`, `SaveAccount` |
| `pkg/mesh/account_test.go` | New file — tests for account data layer |

## Test Strategy

1. Run `go test ./...` — all unit tests must pass.
2. Run `go test -race ./...` — no data races.
3. Manual smoke test (requires a Lighthouse API key):
   ```
   export WGMESH_SECRET=<secret>
   wgmesh service add ollama :11434 --account cr_... --protocol http
   wgmesh service list
   wgmesh service remove ollama
   ```
4. Verify `services.json` and `account.json` are written to `$STATE_DIR` with 0600 permissions.
5. Verify warning is printed (not failure) when `wg0.json` is absent.
6. Verify invalid service names and out-of-range ports print a usage message and exit 1.

## Example Usage

```bash
# First registration — provide API key once; it is saved for future use
wgmesh service add ollama :11434 \
  --secret wgmesh://v1/K7x2... \
  --account cr_abcdefg123 \
  --protocol http \
  --health-path /api/tags

# Service registered: ollama
#   URL:    https://ollama.a1b2c3d4e5f6.wgmesh.dev
#   Origin: 10.42.15.3 (port 11434, http)
#   Status: pending_dns

# Subsequent registrations re-use saved account
wgmesh service add comfyui :8188 --secret wgmesh://v1/K7x2...

# List all registered services
wgmesh service list --secret wgmesh://v1/K7x2...

# NAME      URL                                          PORT    STATUS
# comfyui   https://comfyui.a1b2c3d4e5f6.wgmesh.dev    8188    active
# ollama    https://ollama.a1b2c3d4e5f6.wgmesh.dev      11434   active

# Remove a service
wgmesh service remove ollama --secret wgmesh://v1/K7x2...
# Service removed: ollama
```

## Estimated Complexity
low
