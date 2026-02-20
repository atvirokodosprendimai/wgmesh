# Specification: Issue #257

## Classification
feature

## Deliverables
code + documentation

## Problem Analysis

Setting up a new edge node today is a multi-step manual process:

1. Install `wgmesh` on the VPS
2. Run `wgmesh join --secret ...` to join the mesh
3. Run `apt-get install caddy` (or follow the Caddy docs)
4. Configure `LIGHTHOUSE_URL` and `EDGE_NAME` environment variables
5. Copy or curl `deploy/edge/setup.sh` and execute it
6. Manually verify everything is wired up

`deploy/edge/setup.sh` automates steps 3-6, but it **requires the mesh to already be running** and **requires manual variable export** before invocation. It cannot be driven non-interactively by a pipeline that provisions a fresh VPS.

As a result, autonomous CDN scaling (spin up a new cloud instance → immediately add it to the edge pool) is not possible without significant orchestration glue outside this repository.

The missing piece is a single `wgmesh edge init` command that encapsulates the entire bootstrap flow, accepts all required parameters as flags, and can be run on a freshly imaged machine with no prior configuration.

### Missing Lighthouse endpoint

The `GET /v1/edges` endpoint exists in `pkg/lighthouse/api.go` (line 58) and the `Edge` struct is defined in `pkg/lighthouse/types.go` (lines 86-96), but there is **no `POST /v1/edges`** endpoint. Edge nodes currently cannot self-register with the lighthouse; registration must be done externally (e.g., by the setup script importing data directly into the store). This must be added as part of this feature.

## Proposed Approach

### Step 1 — Add `edge` subcommand dispatcher in `main.go`

Extend the existing subcommand switch in `main()` with a new `"edge"` case that delegates to a new `edgeCmd()` dispatcher:

```
wgmesh edge init   --secret <S> --lighthouse <URL> --api-key <K> [--location <L>] [--name <N>]
wgmesh edge status
wgmesh edge sync   [--lighthouse <URL>] [--api-key <K>]
```

The dispatcher reads `os.Args[2]` to route to `edgeInitCmd()`, `edgeStatusCmd()`, or `edgeSyncCmd()`.

### Step 2 — Implement `edge init` flow

`edgeInitCmd()` executes the following sequential steps, printing progress for each:

**a. Join mesh**
Call `daemon.NewConfig()` + `daemon.NewDaemon()` + `d.RunWithDHTDiscovery()` in a background goroutine (mirroring `joinCmd()`). Wait until the WireGuard interface is up and an IP has been assigned before proceeding to the next step (poll `wg show <iface>` or use the RPC socket).

**b. Discover public IP**
Call the existing `discovery.DiscoverExternalEndpoint(0)` (from `pkg/discovery/stun.go`). This returns the server-reflexive IPv4 address seen by STUN servers. Fall back to a configurable `--public-ip` flag if the caller already knows it (e.g., from cloud metadata).

**c. Detect location**
Default to auto-detection via `https://ip-api.com/json/<public-ip>` (returns `countryCode`, `city`). Allow override via `--location <code>` flag (e.g., `nbg1`, `fsn1`). If the HTTP call fails for any reason (service unavailable, non-200 response, rate-limit 429, parse error, or network timeout), location defaults to `"unknown"` and a warning is printed; `edge init` continues without failing. ip-api.com allows 45 req/min unauthenticated; since this is called once per bootstrap, rate-limiting is not a concern in normal operation.

**d. Install Caddy**
Check `caddy version`; if not found, run the Debian/Ubuntu apt install sequence already present in `deploy/edge/setup.sh` (lines 23-32). Use `os/exec` to run `apt-get`. Fail with a clear error on non-Debian systems or if the install fails.

**e. Register with lighthouse**
Call `POST /v1/edges` (new endpoint, see Step 4) with:
```json
{
  "name": "<hostname or --name flag>",
  "mesh_ip": "<wg IP>",
  "public_ip": "<public IP from STUN>",
  "location": "<auto-detected or --location>"
}
```
The request uses `Authorization: Bearer <api-key>`. The response returns the assigned `edge_id`.

**f. Pull initial site config**
Call `GET /v1/xds/caddyfile` (already exists in `pkg/lighthouse/xds.go`) with `Authorization: Bearer <api-key>`. Write the response to `/etc/caddy/Caddyfile`. Validate with `caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile`.

**g. Start Caddy**
Run `systemctl enable --now caddy` or, if systemd is not available, `caddy start`.

**h. Start heartbeat goroutine (if available)**
If the heartbeat loop from issue #247 is present (detected via a build tag or interface check), start it as a background goroutine that calls `POST /v1/edges/<edge-id>/heartbeat` on the configured interval. If the heartbeat implementation is absent, log a warning (`"heartbeat not available — upgrade wgmesh after #247 is merged"`) and continue; `edge init` must not block on this dependency.

**i. Install systemd service for wgmesh**
Reuse the existing `daemon.InstallSystemdService()` from `pkg/daemon/systemd.go` to persist the mesh join across reboots.

**j. Install edge-config-pull systemd timer**
Write `/etc/systemd/system/edge-config-pull.service` and `/etc/systemd/system/edge-config-pull.timer` (30-second interval). The service unit runs a lightweight Go helper (or shells out to curl) to pull the Caddyfile from lighthouse and reload Caddy on change. This replaces the bash script in `deploy/edge/setup.sh`.

### Step 3 — Implement `edge status`

`edgeStatusCmd()` queries:
- `systemctl is-active wgmesh` (existing `daemon.ServiceStatus()`)
- `systemctl is-active caddy`
- `GET /v1/edges/<edge-id>` on the lighthouse to get `LastHeartbeat` and `SiteCount`
- `wg show <iface>` peer count

Prints a human-readable table:

```
Edge Status
===========
Mesh service:    active
Caddy:           active
Mesh IP:         10.55.3.7
Public IP:       1.2.3.4
Location:        nbg1
Sites served:    12
Last heartbeat:  2s ago
```

The `edge_id` and lighthouse URL are read from a state file written by `edge init` (e.g., `/var/lib/wgmesh/edge.json`).

### Step 4 — Add `POST /v1/edges` to lighthouse API

In `pkg/lighthouse/api.go`, register a new authenticated route:
```
POST /v1/edges  →  handleRegisterEdge()
```

Request body:
```json
{
  "name":       "edge-nbg1",
  "mesh_ip":    "10.55.0.7",
  "public_ip":  "1.2.3.4",
  "location":   "nbg1"
}
```

Response (`201 Created`):
```json
{
  "id":         "edge_a1b2c3...",
  "name":       "edge-nbg1",
  "mesh_ip":    "10.55.0.7",
  "public_ip":  "1.2.3.4",
  "location":   "nbg1",
  "status":     "connected",
  "created_at": "..."
}
```

In `pkg/lighthouse/store.go`, add `CreateEdge(ctx, *Edge) error` (following the same Redis key pattern as `CreateSite`).

### Step 5 — Add `pkg/edge/` package (new)

Create `pkg/edge/bootstrap.go` containing:
- `BootstrapConfig` struct (all flags as fields)
- `Bootstrap(ctx context.Context, cfg BootstrapConfig) error` — orchestrates steps a–i
- `WaitForMeshIP(iface string, timeout time.Duration) (net.IP, error)` — polls WireGuard until the interface has an address
- `DetectLocation(publicIP net.IP) (string, error)` — calls ip-api.com
- `InstallCaddy() error` — runs apt-get sequence
- `RegisterWithLighthouse(cfg BootstrapConfig, meshIP, publicIP net.IP) (string, error)` — POST /v1/edges
- `WriteCaddyfile(lighthouseURL, apiKey string) error` — GET /v1/xds/caddyfile → write + validate
- `InstallEdgeTimer(lighthouseURL, apiKey string) error` — writes systemd timer unit files

This keeps `main.go` thin and keeps the bootstrap logic independently testable.

### Step 6 — `edge sync` subcommand

`edgeSyncCmd()` reads `/var/lib/wgmesh/edge.json` and calls `WriteCaddyfile()` immediately, then reloads Caddy. Useful for forcing a config refresh without waiting for the timer.

## Affected Files

### New files
- `pkg/edge/bootstrap.go` — core bootstrap logic and sub-step functions
- `pkg/edge/bootstrap_test.go` — unit tests with mock HTTP server for lighthouse API
- `pkg/edge/edge.json` schema (documented in README, not a code file)

### Modified files
- `main.go`
  - Add `"edge"` case to the subcommand switch (around line 66)
  - Add `edgeCmd()` dispatcher function
  - Add `edgeInitCmd()`, `edgeStatusCmd()`, `edgeSyncCmd()` functions
  - Update `printUsage()` to document the new subcommands
- `pkg/lighthouse/api.go`
  - Register `POST /v1/edges` route (in `registerRoutes()`)
  - Add `handleRegisterEdge()` handler
- `pkg/lighthouse/store.go`
  - Add `CreateEdge(ctx context.Context, e *Edge) error` method
- `pkg/lighthouse/api_test.go`
  - Add tests for `POST /v1/edges` (valid registration, duplicate, invalid body)

### Documentation
- `README.md` — add `wgmesh edge` section under the Subcommands heading
- `deploy/edge/setup.sh` — add deprecation notice at top pointing to `wgmesh edge init`

## Test Strategy

### Unit tests (`pkg/edge/bootstrap_test.go`)

Use `net/http/httptest` to create a mock lighthouse server. Test each sub-step in isolation:

1. **`TestRegisterWithLighthouse`** — happy path, verify correct JSON body and auth header
2. **`TestRegisterWithLighthouse_Unauthorized`** — 401 response is surfaced as error
3. **`TestWriteCaddyfile`** — mock returns a valid Caddyfile; verify file is written to temp path
4. **`TestWriteCaddyfile_InvalidConfig`** — mock returns garbage; verify error is returned
5. **`TestDetectLocation`** — mock ip-api.com response; verify parsed location string
6. **`TestDetectLocation_RateLimit`** — mock 429 response; verify graceful fallback to "unknown"
7. **`TestWaitForMeshIP_Timeout`** — interface never comes up; verify timeout error

### Unit tests (`pkg/lighthouse/api_test.go`)

8. **`TestHandleRegisterEdge`** — POST with valid body returns 201 and edge object
9. **`TestHandleRegisterEdge_MissingFields`** — missing `mesh_ip` or `name` returns 400
10. **`TestHandleRegisterEdge_Unauthenticated`** — no Bearer token returns 401

### Integration / CLI tests (`main_test.go`)

11. **`TestEdgeCmdHelp`** — `wgmesh edge` with no subcommand prints usage without panic

### Manual acceptance tests (documented in README)

- Run `wgmesh edge init` on a fresh Ubuntu 22.04 VPS; verify:
  - WireGuard interface is up (`wg show`)
  - Caddy is running (`systemctl status caddy`)
  - `wgmesh edge status` shows expected output
  - Edge appears in lighthouse `GET /v1/edges`
- Run `wgmesh edge sync` and verify Caddyfile is updated

## Estimated Complexity
high

**Reasoning:**
- Spans three separate layers: CLI (`main.go`), a new `pkg/edge` package, and a new lighthouse API endpoint
- Bootstrap flow has 9 sequential steps, each with its own error handling and rollback concern
- External dependencies: STUN (already exists), ip-api.com (new HTTP call), apt-get (OS-specific), systemd (OS-specific)
- The `POST /v1/edges` endpoint requires both the API handler and the Redis store method
- Integration testing requires a running WireGuard-capable environment (Linux with `CAP_NET_ADMIN`)
- Dependency on issue #247 (edge heartbeat) means the heartbeat goroutine referenced in step h is a soft dependency — `edge init` should succeed without it but log a warning if the heartbeat loop is absent
- Estimated effort: 3-5 days for a complete, tested implementation
