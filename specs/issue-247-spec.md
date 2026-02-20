# Specification: Issue #247

## Classification
feature

## Deliverables
code

## Problem Analysis

Edge nodes (`pkg/lighthouse/types.go:86-96`) carry two lifecycle fields — `Status` and `LastHeartbeat` — that are never written by any runtime code. `Store.UpsertEdge` (`pkg/lighthouse/store.go:369-380`) already persists an `Edge` object to Dragonfly/Redis but is called nowhere: there is no HTTP handler that accepts data from an edge, and there is no staleness sweep that marks dormant edges as degraded or offline.

The knock-on effects are:

1. `GET /v1/edges` returns whatever was seeded manually into Redis; `Status` is always blank and `LastHeartbeat` is always the zero time.
2. Health-aware routing (xDS snapshot weighting, traffic steering away from failing edges) cannot be implemented without knowing which edges are alive.
3. The `?status=` filter query param mentioned in the issue cannot work because status is never populated.

The fix has two sides:
- **Server side** (lighthouse): expose a `POST /v1/edges/{id}/heartbeat` endpoint that calls `UpsertEdge` and add a background goroutine that periodically marks stale edges as `degraded` / `offline`.
- **Client side** (edge deploy): extend `deploy/edge/setup.sh` to install an `edge-heartbeat` script + systemd unit that POSTs the heartbeat on the configured interval.

## Proposed Approach

### Step 1 — Extend the `Edge` type (`pkg/lighthouse/types.go`)

Add typed constants for edge status so callers don't use raw strings:

```go
type EdgeStatus string

const (
    EdgeStatusHealthy   EdgeStatus = "healthy"
    EdgeStatusDegraded  EdgeStatus = "degraded"
    EdgeStatusOffline   EdgeStatus = "offline"
)
```

Change the `Status` field on `Edge` from `string` to `EdgeStatus`.

Add `LoadAvg float64` and `ActiveConnections int` fields to the `Edge` struct (these are reported by the heartbeat client and can be used for future load-aware routing).

### Step 2 — Heartbeat request/response types

Define a typed request struct (used in the handler):

```go
type HeartbeatRequest struct {
    MeshIP            string  `json:"mesh_ip"`
    PublicIP          string  `json:"public_ip"`
    Location          string  `json:"location"`
    LoadAvg           float64 `json:"load_avg"`
    ActiveConnections int     `json:"active_connections"`
}
```

The response returns the current site list (Caddyfile payload piggyback) — same format as the existing xDS endpoint but scoped to this edge.

### Step 3 — `POST /v1/edges/{id}/heartbeat` handler (`pkg/lighthouse/api.go`)

Register the new route:
```go
a.mux.HandleFunc("POST /v1/edges/{id}/heartbeat", a.requireAuth(a.handleEdgeHeartbeat))
```

Handler logic:
1. Parse `{id}` from path.
2. Decode `HeartbeatRequest` from request body.
3. Call `store.GetEdge` (or allow upsert-on-first-contact if not found).
4. Update `edge.MeshIP`, `edge.PublicIP`, `edge.Location`, `edge.LoadAvg`, `edge.ActiveConnections`, `edge.LastHeartbeat = time.Now().UTC()`, `edge.Status = EdgeStatusHealthy`.
5. Call `store.UpsertEdge`.
6. Return HTTP 200 with the full site list for this edge (enables config piggyback).

Validation:
- `id` must not be empty.
- `mesh_ip` must be a valid IP if provided.
- Unknown/extra fields are silently ignored.

### Step 4 — Health status filtering on `GET /v1/edges` (`pkg/lighthouse/api.go`)

Update `handleListEdges` to read an optional `?status=` query param and filter the slice returned by `store.ListEdges`. No store changes needed — filtering happens in the handler after the full list is loaded.

### Step 5 — Staleness detection (`pkg/lighthouse/api.go` or new file `pkg/lighthouse/health.go`)

Add a `StartHealthSweep(ctx context.Context, store *Store, interval time.Duration)` function that runs as a goroutine (launched from `NewAPI` or from the server's `main()`). On each tick it:

1. Calls `store.ListEdges`.
2. For each edge with `Status != EdgeStatusOffline`:
   - If `time.Since(edge.LastHeartbeat) > 300s` → set `Status = EdgeStatusOffline`.
   - Else if `time.Since(edge.LastHeartbeat) > 60s` → set `Status = EdgeStatusDegraded`.
3. Calls `store.UpsertEdge` for any edge whose status changed.

Thresholds are package-level constants so they can be tuned without changing tests:
```go
const (
    HeartbeatDegradedThreshold = 60 * time.Second
    HeartbeatOfflineThreshold  = 300 * time.Second
)
```

### Step 6 — Edge heartbeat client (`deploy/edge/setup.sh`)

Add a new `edge-heartbeat` script alongside the existing `edge-config-pull` script. The script:

1. Reads `LIGHTHOUSE_URL`, `EDGE_ID`, `EDGE_API_KEY`, `EDGE_NAME`, `EDGE_LOCATION` from env.
2. Collects `load_avg` from `/proc/loadavg` and `active_connections` from `ss -t state established | tail -n +2 | wc -l` (skipping the header line that `ss` always prints).
3. Collects `mesh_ip` from the WireGuard interface using `ip -4 addr show wg0 | grep inet | awk '{print $2}' | cut -d'/' -f1` (the `wg show` command does not output an `address` field, so `ip addr` is more reliable).
4. POSTs `{"mesh_ip":..., "public_ip":..., "location":..., "load_avg":..., "active_connections":...}` to `${LIGHTHOUSE_URL}/v1/edges/${EDGE_ID}/heartbeat` with `Authorization: Bearer ${EDGE_API_KEY}`.
5. Logs success/failure (non-fatal on failure so the timer continues).

Install as a systemd oneshot + timer (`edge-heartbeat.service` / `edge-heartbeat.timer`) running every 30 seconds.

### Step 7 — Update OpenAPI spec in `handleOpenAPI` (`pkg/lighthouse/api.go`)

Add the heartbeat path to the inline OpenAPI map:
```
POST /v1/edges/{id}/heartbeat
```

## Affected Files

| File | Change |
|---|---|
| `pkg/lighthouse/types.go` | Add `EdgeStatus` type + constants; add `LoadAvg`, `ActiveConnections` fields to `Edge` |
| `pkg/lighthouse/api.go` | Add `POST /v1/edges/{id}/heartbeat` route + handler; add `?status=` filter to `handleListEdges`; update OpenAPI map; wire up health sweep goroutine |
| `pkg/lighthouse/health.go` (new) | `StartHealthSweep` function + threshold constants |
| `pkg/lighthouse/api_test.go` | Tests for heartbeat handler (validation, upsert path) and status filtering |
| `pkg/lighthouse/health_test.go` (new) | Unit tests for staleness logic (mock store or time-injection) |
| `deploy/edge/setup.sh` | Add `edge-heartbeat` script, `edge-heartbeat.service`, `edge-heartbeat.timer`, and required env vars |

## Test Strategy

### Unit tests — heartbeat handler (no Redis required)

Following the existing pattern in `api_test.go` (nil-store + `X-Org-ID` header injection):

1. **Missing auth** → 401.
2. **Invalid JSON body** → 400.
3. **Invalid `mesh_ip`** → 400.
4. **Valid request, store returns not-found** → handler auto-creates edge, returns 200.
5. **Valid request, store exists** → handler updates fields, returns 200 with site list.

Because the nil-store pattern panics on any store call, these tests will need a simple **in-memory stub store** — a private `stubStore` struct that implements only the `GetEdge`/`UpsertEdge`/`ListSites` methods via a map. This is preferred over full dependency injection to keep the change surface minimal and consistent with the existing `testAPI()` helper pattern. The stub is defined only in `_test.go` files and is not exported.

### Unit tests — staleness detection (`health_test.go`)

Use a fake/stub store with a list of edges with controlled `LastHeartbeat` timestamps:

| Test case | `LastHeartbeat` age | Expected status after sweep |
|---|---|---|
| Fresh edge | 10s | `healthy` |
| Degraded threshold hit | 90s | `degraded` |
| Offline threshold hit | 400s | `offline` |
| Already offline, heartbeat stale | 400s | `offline` (no write) |
| Already degraded, fresh heartbeat | 10s | `healthy` |

### Integration test (manual / future CI)

1. Start lighthouse with a real Dragonfly instance.
2. POST a heartbeat for a new edge → verify `GET /v1/edges` returns `status=healthy`.
3. Wait 60s without heartbeat → verify status transitions to `degraded`.
4. `GET /v1/edges?status=degraded` → returns only the degraded edge.
5. POST another heartbeat → verify status returns to `healthy`.

### Shell script test (manual)

Run `edge-heartbeat` script in a test environment with a mock server and verify:
- Correct `Authorization` header is sent.
- JSON body contains `mesh_ip`, `load_avg`, `active_connections`.
- Script exits 0 on 200 response.
- Script exits 0 (non-fatal) on non-200 response.

## Estimated Complexity

**medium** (3-5 hours)

### Rationale

- Server changes are straightforward given existing `UpsertEdge` and `handleListEdges` scaffolding.
- Staleness sweep is a small background goroutine but requires careful handling of context cancellation and avoiding write amplification.
- Shell script client is simple but requires ensuring portability across Debian/Ubuntu edge hosts.
- Tests need a stub store pattern not currently in the test infrastructure — this is the main new effort.
- No new external dependencies are required.
