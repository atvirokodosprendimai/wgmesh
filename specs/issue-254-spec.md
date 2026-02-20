# Specification: Issue #254

## Classification
feature

## Deliverables
code + documentation

## Problem Analysis

The current `Site` type in `pkg/lighthouse/types.go` holds a single `Origin` field. A production CDN requires multiple origins per site (primary + fallback) so that traffic automatically routes to a healthy upstream when the primary is unavailable.

### Current State

```go
// Origin defines where traffic should be proxied to.
type Origin struct {
    MeshIP   string `json:"mesh_ip"`
    Port     int    `json:"port"`
    Protocol string `json:"protocol"` // "http" or "https"
}

// Site represents a customer domain routed through the CDN.
type Site struct {
    // ...
    Origin Origin `json:"origin"` // single origin only
    // ...
}
```

`HandleCaddyConfig` in `pkg/lighthouse/xds.go` (lines 114–161) always generates a single `reverse_proxy` upstream per site. There is no failover or health-checking directive.

The API exposes no endpoints to add, remove, or reprioritise origins.

### Missing Capabilities

1. No way to register more than one origin per site.
2. Caddyfile generation does not emit multi-upstream `reverse_proxy` blocks or `health_uri`/`fail_duration`.
3. The Envoy xDS snapshot (`BuildSnapshot`) hard-codes one cluster per site.
4. No store operations for origin CRUD (add / remove / update priority).
5. No API endpoints for origin management.

---

## Proposed Approach

### Step 1 — Extend `Origin` and `Site` types (`pkg/lighthouse/types.go`)

Add `Priority` and `Weight` fields to `Origin`. Change `Site.Origin` (singular) to `Site.Origins` (slice).  
Preserve a compatibility shim so callers that still read the old `origin` JSON key receive the first element.

```go
type Origin struct {
    ID       string `json:"id"`       // "org_…"-style prefixed random ID
    MeshIP   string `json:"mesh_ip"`
    Port     int    `json:"port"`
    Protocol string `json:"protocol"` // "http" or "https"
    Priority int    `json:"priority"` // lower = preferred; 0 is highest
    Weight   int    `json:"weight"`   // for weighted LB within same priority tier
}

type Site struct {
    // ... existing fields unchanged ...
    Origins []Origin `json:"origins"` // replaces Origin; at least one required
    // Deprecated: use Origins[0]. Kept for backward-compatible JSON reads.
    // Omitted from serialisation if Origins is non-empty.
    Origin *Origin `json:"origin,omitempty"`
}
```

Migration note: on first `UpdateSite` for an existing record that has `origin` but no `origins`, the store should promote `origin → origins[0]` and clear the legacy field.

### Step 2 — Store operations (`pkg/lighthouse/store.go`)

Add three new methods to the `Store` interface and its Redis implementation:

| Method | Signature | Description |
|---|---|---|
| `AddOrigin` | `(ctx, siteID, orgID string, o Origin) (*Site, error)` | Appends a new origin (assigns `GenerateID("ori")`), increments site version |
| `RemoveOrigin` | `(ctx, siteID, orgID, originID string) (*Site, error)` | Removes origin by ID; errors if it would leave zero origins |
| `UpdateOrigin` | `(ctx, siteID, orgID string, o Origin) (*Site, error)` | Replaces matching origin by ID, increments version |

All three methods use the existing LWW/CRDT pattern: increment `Version`, set `NodeID`, call `UpdateSite` under the hood to propagate via `ApplySync`.

### Step 3 — Caddyfile generation (`pkg/lighthouse/xds.go`, `HandleCaddyConfig`)

Sort origins by `(Priority ASC, Weight DESC)` before emitting the Caddyfile block.

**Single-origin sites (backward compatibility):** unchanged output format.

**Multi-origin sites:**

```
example.com {
    reverse_proxy 10.77.1.50:80 10.77.1.51:80 {
        header_up Host {upstream_hostport}
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Site-ID site_abc123
        lb_policy first
        health_uri /healthz
        fail_duration 30s
    }

    header {
        X-Served-By {system.hostname}
        X-CDN cloudroof
        -Server
    }

    log {
        output stdout
    }
}
```

Key changes:
- `to` / inline upstream list uses `protocol://mesh_ip:port` for each origin in priority order.
- `lb_policy first` — routes to the first healthy upstream (priority failover).
- `health_uri /healthz` — active health check path.
- `fail_duration 30s` — how long an upstream stays marked unhealthy.

### Step 4 — Envoy xDS snapshot (`pkg/lighthouse/xds.go`, `BuildSnapshot`)

Update `BuildSnapshot` to emit one Envoy cluster per site with `lb_policy: PRIORITY` and one endpoint per origin:

```json
{
  "name": "origin_site_abc123",
  "load_assignment": {
    "cluster_name": "origin_site_abc123",
    "endpoints": [
      {
        "priority": 0,
        "lb_endpoints": [{ "endpoint": { "address": { "socket_address": { "address": "10.77.1.50", "port_value": 80 }}}}]
      },
      {
        "priority": 1,
        "lb_endpoints": [{ "endpoint": { "address": { "socket_address": { "address": "10.77.1.51", "port_value": 80 }}}}]
      }
    ]
  }
}
```

### Step 5 — API endpoints (`pkg/lighthouse/api.go`)

Add three new authenticated routes under the existing `mux`:

| Method | Path | Handler | Notes |
|---|---|---|---|
| `POST` | `/v1/sites/{site_id}/origins` | `handleAddOrigin` | Body: `{mesh_ip, port, protocol, priority, weight}`; validates same rules as site creation origin |
| `DELETE` | `/v1/sites/{site_id}/origins/{origin_id}` | `handleRemoveOrigin` | 409 if last origin |
| `PUT` | `/v1/sites/{site_id}/origins/{origin_id}` | `handleUpdateOrigin` | Partial update: priority and/or weight |

All handlers re-use the existing auth middleware and org-isolation checks from `handleUpdateSite`.

### Step 6 — Request/response validation

- `mesh_ip` must be a valid IP address (same regex as `handleCreateSite`).
- `port` must be in range 1–65535.
- `protocol` must be `"http"` or `"https"`.
- `priority` must be ≥ 0.
- `weight` must be ≥ 1 (default 1 if omitted).
- A site must always retain at least one origin after a `RemoveOrigin`.

---

## Affected Files

| File | Change |
|---|---|
| `pkg/lighthouse/types.go` | Add `Origin.ID`, `Origin.Priority`, `Origin.Weight`; change `Site.Origin → Site.Origins []Origin`; keep deprecated `Site.Origin *Origin` shim |
| `pkg/lighthouse/store.go` | Add `AddOrigin`, `RemoveOrigin`, `UpdateOrigin` methods; migration logic on `UpdateSite` |
| `pkg/lighthouse/xds.go` | Update `HandleCaddyConfig` for multi-origin Caddyfile blocks; update `BuildSnapshot` for priority-based Envoy clusters |
| `pkg/lighthouse/api.go` | Register `POST/DELETE/PUT /v1/sites/{id}/origins[/{origin_id}]` routes and handlers |
| `pkg/lighthouse/types_test.go` | Tests for `Origin` ID generation and `Site.Origins` serialisation |
| `pkg/lighthouse/api_test.go` | Tests for new origin management endpoints (validation, 409 on last-origin delete) |
| `pkg/lighthouse/xds_test.go` | Tests for multi-origin Caddyfile generation and single-origin backward compatibility |

---

## Test Strategy

### Unit tests (`xds_test.go`)

1. **Single-origin backward compatibility** — a site with one origin produces the same Caddyfile output as before (no `lb_policy`, no `health_uri`).
2. **Multi-origin failover block** — a site with two origins at different priorities produces a `reverse_proxy` block with both upstreams, `lb_policy first`, `health_uri /healthz`, `fail_duration 30s`.
3. **Priority ordering** — origins with `Priority=1` appear after `Priority=0` in the upstream list.
4. **Suspended/deleted sites skipped** — unchanged behaviour.

### Unit tests (`api_test.go`)

1. **POST /v1/sites/{id}/origins** — valid body succeeds; invalid IP/port/protocol returns 400.
2. **DELETE /v1/sites/{id}/origins/{origin_id}** — last origin returns 409.
3. **PUT /v1/sites/{id}/origins/{origin_id}** — updates priority/weight; unknown origin_id returns 404.
4. **Org isolation** — cannot manage origins for a site belonging to a different org.

### Unit tests (`types_test.go`)

1. `Site` with `Origins` serialises and deserialises correctly.
2. `Site` with legacy `origin` field deserialises into `Origins[0]` after migration shim.

### Integration / manual smoke test

```bash
# Create site
curl -X POST /v1/sites -d '{"domain":"example.com","origins":[{"mesh_ip":"10.77.1.50","port":80,"protocol":"http","priority":0}],...}'

# Add backup origin
curl -X POST /v1/sites/{id}/origins -d '{"mesh_ip":"10.77.1.51","port":80,"protocol":"http","priority":1,"weight":1}'

# Verify Caddyfile contains both upstreams
curl /v1/edges/caddy-config | grep -A10 'example.com'
```

---

## Estimated Complexity
medium
