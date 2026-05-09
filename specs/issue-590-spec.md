# Specification: Issue #590

## Classification
feature

## Deliverables
code

## Problem Analysis

wgmesh exposes a Prometheus `/metrics` endpoint (started via `--metrics <addr>`) with the
following gauges/counters already in `pkg/daemon/metrics.go`:

| Metric | Type | Description |
|---|---|---|
| `wgmesh_active_peers` | Gauge | Current active peers |
| `wgmesh_relayed_peers` | Gauge | Peers reachable only via relay |
| `wgmesh_nat_type` | GaugeVec | Local NAT type |
| `wgmesh_discovery_events_total` | CounterVec | Events per discovery layer |
| `wgmesh_reconcile_duration_seconds` | Histogram | Reconcile loop latency |
| `wgmesh_probe_rtt_seconds` | HistogramVec | Per-peer RTT histogram |
| `wgmesh_probe_rtt_summary_seconds` | SummaryVec | Per-peer RTT P50/P95/P99 |
| `wgmesh_nat_traversal_attempts_total` | CounterVec | NAT hole-punch attempts |
| `wgmesh_nat_traversal_successes_total` | CounterVec | NAT hole-punch successes |

Three categories of usage analytics are absent:

1. **Data transfer**: `wireguard.GetPeerTransfers()` is already called every 20 s inside
   `checkPeerHealth()` in `pkg/daemon/daemon.go` but the `RxBytes`/`TxBytes` values are
   consumed only for stale-peer detection and never exposed as Prometheus metrics.

2. **Daemon / session uptime**: `d.startTime` is set in `Run()` and `GetUptime()` already
   returns the duration, but there is no `wgmesh_daemon_uptime_seconds` gauge that Prometheus
   scrapers can track.

3. **Peer churn**: There is no counter recording how many peers have joined or left the mesh
   since the daemon started. These are the primary engagement signals (long-lived meshes with
   stable peer counts indicate healthy adoption).

No external telemetry is sent anywhere; all analytics are self-hosted on the Prometheus
endpoint the operator already runs. Privacy is preserved by design: only aggregate byte
counters (not payload content) are exported, and the `peer_key` label uses an 8-character
key prefix that is already used throughout the codebase (e.g. `ObserveProbeRTT`).

A Grafana dashboard JSON (`deploy/grafana/wgmesh-usage.json`) that wires up all existing
and new metrics into customer-success panels completes the "dashboards" acceptance criterion.

## Proposed Approach

1. Add five new Prometheus instruments in `pkg/daemon/metrics.go`.
2. Call the new recording functions from the two existing code paths in `pkg/daemon/daemon.go`
   that already have the required data.
3. Subscribe to the PeerStore event channel in `daemon.Run()` and increment join/leave
   counters from the goroutine that drains it.
4. Ship a ready-to-import Grafana dashboard at `deploy/grafana/wgmesh-usage.json`.
5. Add unit tests for the new metrics in `pkg/daemon/metrics_test.go`.

No new dependencies, no external network calls, no changes to the wire protocol.

## Implementation Tasks

### Task 1: Add new metrics to `pkg/daemon/metrics.go`

In the `var (...)` block, add five new instruments immediately after the existing
`natTraversalSuccesses` definition (before the `goCollector` line):

```go
peerTxBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "wgmesh_peer_tx_bytes_total",
    Help: "Cumulative bytes transmitted to each peer (WireGuard kernel counter)",
}, []string{"peer_key"})
peerRxBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "wgmesh_peer_rx_bytes_total",
    Help: "Cumulative bytes received from each peer (WireGuard kernel counter)",
}, []string{"peer_key"})
daemonUptime = prometheus.NewGauge(prometheus.GaugeOpts{
    Name: "wgmesh_daemon_uptime_seconds",
    Help: "Seconds since the daemon was started",
})
peerJoinsTotal = prometheus.NewCounter(prometheus.CounterOpts{
    Name: "wgmesh_peer_joins_total",
    Help: "Total number of new peers discovered since daemon start",
})
peerLeavesTotal = prometheus.NewCounter(prometheus.CounterOpts{
    Name: "wgmesh_peer_leaves_total",
    Help: "Total number of peers evicted or cleaned up since daemon start",
})
```

In `RegisterMetrics()`, register all five immediately before the existing
`prometheus.MustRegister(goCollector)` line:

```go
prometheus.MustRegister(peerTxBytes)
prometheus.MustRegister(peerRxBytes)
prometheus.MustRegister(daemonUptime)
prometheus.MustRegister(peerJoinsTotal)
prometheus.MustRegister(peerLeavesTotal)
```

Add three new exported functions at the end of the file, after the existing
`RecordNATTraversalSuccess` function:

```go
// UpdateTransferMetrics updates the per-peer transfer byte counters from the
// raw WireGuard kernel counters. prevTx / prevRx are the values observed in
// the previous call; delta = current - prev is added to the Prometheus counter.
// If current < prev (counter reset after interface restart) the delta is skipped.
func UpdateTransferMetrics(pubKey string, prevRx, currRx, prevTx, currTx uint64) {
    key := pubKey
    if len(key) > 8 {
        key = key[:8]
    }
    if currRx > prevRx {
        peerRxBytes.WithLabelValues(key).Add(float64(currRx - prevRx))
    }
    if currTx > prevTx {
        peerTxBytes.WithLabelValues(key).Add(float64(currTx - prevTx))
    }
}

// RecordPeerJoin increments the peer-joins counter.
func RecordPeerJoin() {
    peerJoinsTotal.Inc()
}

// RecordPeerLeave increments the peer-leaves counter.
func RecordPeerLeave() {
    peerLeavesTotal.Inc()
}
```

Also extend `UpdateMetrics(d *Daemon)` to set the uptime gauge. Append the following
lines inside `UpdateMetrics` at the end of the function body, after the existing NAT type
block:

```go
// Daemon uptime
if !d.startTime.IsZero() {
    daemonUptime.Set(time.Since(d.startTime).Seconds())
}
```

No new imports are needed; `time` is already imported.

### Task 2: Track per-peer transfer deltas in `pkg/daemon/daemon.go` — `checkPeerHealth()`

`checkPeerHealth()` already reads per-peer `RxBytes` and `TxBytes` from
`wireguard.GetPeerTransfers()`. It stores a running total in
`d.lastPeerTransferTotal[p.WGPubKey]` (keyed by pubKey), but does not store per-direction
values.

**Step 2a — Add two tracking maps to the `Daemon` struct** (alongside
`lastPeerTransferTotal`, in the struct definition in `daemon.go`):

```go
lastPeerTransferRx map[string]uint64
lastPeerTransferTx map[string]uint64
```

**Step 2b — Initialise the maps in `NewDaemon()`** (around line 194, in the struct
literal inside `NewDaemon`):

```go
lastPeerTransferRx: make(map[string]uint64),
lastPeerTransferTx: make(map[string]uint64),
```

**Step 2c — Update `checkPeerHealth()`** to read per-direction previous values, call
`UpdateTransferMetrics`, and store the new values.

Locate the block in `checkPeerHealth()` that currently reads:

```go
transfer := transfers[p.WGPubKey]
currentTotal := transfer.RxBytes + transfer.TxBytes

d.healthMu.Lock()
prevTotal := d.lastPeerTransferTotal[p.WGPubKey]
d.lastPeerTransferTotal[p.WGPubKey] = currentTotal
isStale := shouldTreatPeerAsStale(ts, prevTotal, currentTotal, now)
if isStale {
    d.peerHealthFailures[p.WGPubKey]++
} else {
    d.peerHealthFailures[p.WGPubKey] = 0
}
failures := d.peerHealthFailures[p.WGPubKey]
d.healthMu.Unlock()
```

Replace that entire block with:

```go
transfer := transfers[p.WGPubKey]
currentTotal := transfer.RxBytes + transfer.TxBytes

d.healthMu.Lock()
prevTotal := d.lastPeerTransferTotal[p.WGPubKey]
prevRx    := d.lastPeerTransferRx[p.WGPubKey]
prevTx    := d.lastPeerTransferTx[p.WGPubKey]
d.lastPeerTransferTotal[p.WGPubKey] = currentTotal
d.lastPeerTransferRx[p.WGPubKey]    = transfer.RxBytes
d.lastPeerTransferTx[p.WGPubKey]    = transfer.TxBytes
isStale := shouldTreatPeerAsStale(ts, prevTotal, currentTotal, now)
if isStale {
    d.peerHealthFailures[p.WGPubKey]++
} else {
    d.peerHealthFailures[p.WGPubKey] = 0
}
failures := d.peerHealthFailures[p.WGPubKey]
d.healthMu.Unlock()

UpdateTransferMetrics(p.WGPubKey, prevRx, transfer.RxBytes, prevTx, transfer.TxBytes)
```

The lock/unlock structure is unchanged; the only additions are the two `prevRx`/`prevTx`
reads, the two new map writes inside the lock, and the single call to
`UpdateTransferMetrics` after the lock is released.

**Step 2d — Clean up the new maps when a peer is removed.** In `evictPeerFromPool()`
(around line 1354), after the existing `delete(d.lastPeerTransferTotal, peer.WGPubKey)`
line, add:

```go
delete(d.lastPeerTransferRx, peer.WGPubKey)
delete(d.lastPeerTransferTx, peer.WGPubKey)
```

In `staleCleanupLoop()`, locate the call to `peerStore.CleanupStale()` (returns a slice
of removed pubkeys). After the existing `delete(d.lastPeerTransferTotal, pubKey)` line
(inside the cleanup loop over removed keys), add:

```go
delete(d.lastPeerTransferRx, pubKey)
delete(d.lastPeerTransferTx, pubKey)
```

### Task 3: Track peer joins and leaves in `pkg/daemon/daemon.go` — `Run()`

**Peer joins** — subscribe to the PeerStore's event channel and count `PeerEventNew`.

In `Run()`, after the existing goroutine that calls `d.staleCleanupLoop()` (around line 264),
add a new goroutine:

```go
// Count new peer joins for analytics.
peerEvents := d.peerStore.Subscribe()
d.wg.Add(1)
go func() {
    defer d.wg.Done()
    defer d.peerStore.Unsubscribe(peerEvents)
    for {
        select {
        case ev, ok := <-peerEvents:
            if !ok {
                return
            }
            if ev.Kind == PeerEventNew {
                RecordPeerJoin()
            }
        case <-d.ctx.Done():
            return
        }
    }
}()
```

**Peer leaves** — call `RecordPeerLeave()` at the two existing eviction/cleanup code paths.

1. In `evictPeerFromPool()` (the function body, right before `d.peerStore.Remove(peer.WGPubKey)`
   on the current line 1341), add:

   ```go
   RecordPeerLeave()
   ```

2. In `staleCleanupLoop()`, find the loop that iterates over the keys returned by
   `peerStore.CleanupStale()`. Inside that loop, where the peer key is processed, add:

   ```go
   RecordPeerLeave()
   ```

### Task 4: Create Grafana dashboard `deploy/grafana/wgmesh-usage.json`

Create the directory `deploy/grafana/` and place a single file `wgmesh-usage.json` there.
The file must be a valid Grafana 10.x dashboard JSON that can be imported via
**Dashboards → Import → Upload JSON file**. It must contain exactly the following panels
(use type `"timeseries"` for all time-series panels and `"stat"` for single-value panels):

**Row 1 — Mesh Size & Engagement**

| Panel title | Metric expression | Panel type |
|---|---|---|
| Active Peers | `wgmesh_active_peers` | stat |
| Peer Joins (rate) | `rate(wgmesh_peer_joins_total[$__rate_interval])` | timeseries |
| Peer Leaves (rate) | `rate(wgmesh_peer_leaves_total[$__rate_interval])` | timeseries |
| Relayed Peers | `wgmesh_relayed_peers` | stat |

**Row 2 — Data Transfer**

| Panel title | Metric expression | Panel type |
|---|---|---|
| Total Rx Rate (all peers) | `sum(rate(wgmesh_peer_rx_bytes_total[$__rate_interval]))` | timeseries |
| Total Tx Rate (all peers) | `sum(rate(wgmesh_peer_tx_bytes_total[$__rate_interval]))` | timeseries |
| Per-Peer Rx Rate | `rate(wgmesh_peer_rx_bytes_total[$__rate_interval])` | timeseries |
| Per-Peer Tx Rate | `rate(wgmesh_peer_tx_bytes_total[$__rate_interval])` | timeseries |

**Row 3 — Connection Health**

| Panel title | Metric expression | Panel type |
|---|---|---|
| Daemon Uptime | `wgmesh_daemon_uptime_seconds` | stat |
| P95 Probe RTT | `wgmesh_probe_rtt_summary_seconds{quantile="0.95"}` | timeseries |
| Discovery Events (rate) | `sum by (layer)(rate(wgmesh_discovery_events_total[$__rate_interval]))` | timeseries |
| NAT Traversal Success Rate | `rate(wgmesh_nat_traversal_successes_total[$__rate_interval]) / rate(wgmesh_nat_traversal_attempts_total[$__rate_interval])` | timeseries |

The dashboard JSON must include:
- `"title": "wgmesh Usage Analytics"`
- `"uid": "wgmesh-usage-v1"`
- `"schemaVersion": 36`
- A templating variable `datasource` of type `datasource` with `"type": "prometheus"` so
  the user can select their Prometheus instance on import.
- Default time range `"from": "now-1h"`, `"to": "now"`.

The full JSON structure (abbreviated) must follow this skeleton — fill in the full
`panels` array according to the table above:

```json
{
  "title": "wgmesh Usage Analytics",
  "uid": "wgmesh-usage-v1",
  "schemaVersion": 36,
  "time": { "from": "now-1h", "to": "now" },
  "templating": {
    "list": [
      {
        "name": "datasource",
        "type": "datasource",
        "pluginId": "prometheus",
        "label": "Prometheus"
      }
    ]
  },
  "panels": [ /* ... see table above ... */ ]
}
```

### Task 5: Add unit tests in `pkg/daemon/metrics_test.go`

Append two test functions to the end of `pkg/daemon/metrics_test.go`:

```go
func TestUpdateTransferMetrics(t *testing.T) {
    // Reset counters
    peerTxBytes.DeleteLabelValues("abcdefgh")
    peerRxBytes.DeleteLabelValues("abcdefgh")

    // First call: counters start at zero, so delta = current - 0.
    UpdateTransferMetrics("abcdefghXXXXX", 0, 1024, 0, 512)

    rxVal := testutil.ToFloat64(peerRxBytes.WithLabelValues("abcdefgh"))
    if rxVal != 1024 {
        t.Errorf("expected rx 1024, got %v", rxVal)
    }
    txVal := testutil.ToFloat64(peerTxBytes.WithLabelValues("abcdefgh"))
    if txVal != 512 {
        t.Errorf("expected tx 512, got %v", txVal)
    }

    // Second call: counter advanced by another 256 rx, 128 tx.
    UpdateTransferMetrics("abcdefghXXXXX", 1024, 1280, 512, 640)

    rxVal = testutil.ToFloat64(peerRxBytes.WithLabelValues("abcdefgh"))
    if rxVal != 1280 {
        t.Errorf("expected cumulative rx 1280, got %v", rxVal)
    }
    txVal = testutil.ToFloat64(peerTxBytes.WithLabelValues("abcdefgh"))
    if txVal != 640 {
        t.Errorf("expected cumulative tx 640, got %v", txVal)
    }

    // Counter-reset guard: if current < prev, skip.
    UpdateTransferMetrics("abcdefghXXXXX", 1280, 10, 640, 5) // simulated reset

    rxVal = testutil.ToFloat64(peerRxBytes.WithLabelValues("abcdefgh"))
    if rxVal != 1280 {
        t.Errorf("counter reset should not decrease metric; got %v", rxVal)
    }
}

func TestPeerJoinLeaveCounters(t *testing.T) {
    before := testutil.ToFloat64(peerJoinsTotal)
    RecordPeerJoin()
    RecordPeerJoin()
    after := testutil.ToFloat64(peerJoinsTotal)
    if after-before != 2 {
        t.Errorf("expected 2 join increments, got %v", after-before)
    }

    beforeLeave := testutil.ToFloat64(peerLeavesTotal)
    RecordPeerLeave()
    afterLeave := testutil.ToFloat64(peerLeavesTotal)
    if afterLeave-beforeLeave != 1 {
        t.Errorf("expected 1 leave increment, got %v", afterLeave-beforeLeave)
    }
}
```

No new imports are required; `testutil` and `prometheus` are already imported in
`metrics_test.go`.

## Affected Files

| File | Change |
|---|---|
| `pkg/daemon/metrics.go` | Add `peerTxBytes`, `peerRxBytes`, `daemonUptime`, `peerJoinsTotal`, `peerLeavesTotal` vars; register all five; add `UpdateTransferMetrics`, `RecordPeerJoin`, `RecordPeerLeave`; extend `UpdateMetrics` with uptime gauge |
| `pkg/daemon/daemon.go` | Add `lastPeerTransferRx`/`Tx` maps to `Daemon` struct; initialise in `NewDaemon`; call `UpdateTransferMetrics` in `checkPeerHealth`; clean up maps in `evictPeerFromPool` and `staleCleanupLoop`; subscribe to PeerStore events in `Run()` and call `RecordPeerJoin`/`RecordPeerLeave` |
| `pkg/daemon/metrics_test.go` | Add `TestUpdateTransferMetrics` and `TestPeerJoinLeaveCounters` |
| `deploy/grafana/wgmesh-usage.json` | New file: importable Grafana 10.x dashboard JSON |

## Test Strategy

1. `go test ./pkg/daemon/...` — existing tests plus `TestUpdateTransferMetrics` and
   `TestPeerJoinLeaveCounters` must pass.
2. `go test -race ./pkg/daemon/...` — no data races (all new map accesses are inside the
   existing `d.healthMu` lock; counter increments are atomic inside prometheus).
3. `go build ./...` — compilation must succeed.
4. Manual smoke test: start `wgmesh join --secret <secret> --metrics :9090`, wait for a
   peer to connect, then:
   - `curl -s http://localhost:9090/metrics | grep wgmesh_peer_` — verify `_rx_bytes_total`
     and `_tx_bytes_total` lines appear with non-zero values after traffic flows.
   - `curl -s http://localhost:9090/metrics | grep wgmesh_daemon_uptime_seconds` — verify a
     positive float appears.
   - `curl -s http://localhost:9090/metrics | grep wgmesh_peer_joins_total` — verify counter
     increments each time a new peer announces itself.
5. Dashboard smoke test: import `deploy/grafana/wgmesh-usage.json` into a local Grafana
   instance pointed at the Prometheus that scrapes the daemon. Verify all 12 panels render
   without "No data" errors after several minutes of traffic.

## Estimated Complexity
medium
