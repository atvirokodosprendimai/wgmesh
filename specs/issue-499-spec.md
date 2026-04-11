# Specification: Issue #499

## Classification
feature

## Problem Analysis

Issue #470 partially implemented Prometheus metrics: `pkg/daemon/metrics.go` and `pkg/daemon/metrics_test.go` exist, and `main.go` already wires up `--metrics` flag + `/metrics` HTTP endpoint. However several requirements from the original issue remain unimplemented:

1. **Go runtime metrics are missing.** `RegisterMetrics()` in `pkg/daemon/metrics.go` registers only five custom metrics; it never registers `collectors.NewGoCollector()` or `collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})`. Those provide goroutine counts, heap/stack memory, GC pause durations, and OS process metrics.

2. **Probe latency (RTT) metric is missing.** `probePeer()` in `pkg/daemon/daemon.go` does a ping/pong over TCP but discards elapsed time. There is no `wgmesh_probe_rtt_seconds` histogram.

3. **NAT traversal counters are missing.** `exchangeWithAddress()` in `pkg/discovery/dht.go` logs `"[DHT] SUCCESS! Found wgmesh peer"` on success, logs nothing on failure, but never increments any counter. There is no `wgmesh_nat_traversal_attempts_total` or `wgmesh_nat_traversal_successes_total`.

4. **`RecordDiscoveryEvent()` is never called.** The function exists in `pkg/daemon/metrics.go` and its test exercises it directly, but no discovery layer (`dht.go`, `lan.go`, `gossip.go`, `registry.go`) ever calls it. Since `pkg/discovery` imports `pkg/daemon` (see `pkg/discovery/init.go`), calling `daemon.RecordDiscoveryEvent("dht")` etc. from discovery code is safe.

5. **No documentation.** README.md and `docs/quickstart.md` have no metrics section. Users cannot discover the `--metrics` flag or know what metrics are available without reading source.

## Implementation Tasks

### Task 1: Register Go runtime collectors in `pkg/daemon/metrics.go`

Add two collector variables and register them in `RegisterMetrics()`.

In the import block, add `"github.com/prometheus/client_golang/prometheus/collectors"`.

At the top of the file alongside the existing `var` block, add:

```go
var (
    goCollector      = collectors.NewGoCollector()
    processCollector = collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})
)
```

Inside `RegisterMetrics()`, after the five existing `prometheus.MustRegister(...)` lines, add:

```go
prometheus.MustRegister(goCollector)
prometheus.MustRegister(processCollector)
```

These collectors expose the following metric families automatically:
- `go_goroutines` — current number of goroutines
- `go_memstats_alloc_bytes` — bytes of allocated heap objects
- `go_gc_duration_seconds` — GC pause duration histogram
- `process_resident_memory_bytes`, `process_open_fds`, `process_cpu_seconds_total`

No test changes are required for this task because the `prometheus/testutil` package and existing `TestMetricsRegistered` already verify that `Gather()` works without panic.

### Task 2: Add probe RTT histogram to `pkg/daemon/metrics.go`

Add a new histogram variable in the existing `var` block:

```go
probeRTT = prometheus.NewHistogramVec(prometheus.HistogramOpts{
    Name:    "wgmesh_probe_rtt_seconds",
    Help:    "Round-trip time of mesh health probes",
    Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
}, []string{"peer_key"})
```

Add a label with the short key (first 8 characters) so per-peer latency is visible.

In `RegisterMetrics()`, add:

```go
prometheus.MustRegister(probeRTT)
```

Add a new exported function:

```go
// ObserveProbeRTT records the round-trip time for a mesh probe to the given peer.
// peerKey should be the first 8 characters of the WireGuard public key.
func ObserveProbeRTT(peerKey string, start time.Time) {
    probeRTT.WithLabelValues(peerKey).Observe(time.Since(start).Seconds())
}
```

### Task 3: Call `ObserveProbeRTT` in `pkg/daemon/daemon.go`

In `probePeer()` (line ~1034), record the time before the write and call `ObserveProbeRTT` before returning `true`. The function signature stays the same (`func (d *Daemon) probePeer(peer *PeerInfo) bool`).

Before the `session.conn.Write` call, add:

```go
start := time.Now()
```

Before the final `return true` at the end of the function (after the `ReadString` check), add:

```go
ObserveProbeRTT(peer.WGPubKey[:8], start)
```

Do **not** record RTT on failure paths — only on the successful ping/pong.

### Task 4: Add NAT traversal counters to `pkg/daemon/metrics.go`

Add two counter vec variables in the existing `var` block:

```go
natTraversalAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "wgmesh_nat_traversal_attempts_total",
    Help: "NAT traversal attempts by method",
}, []string{"method"})

natTraversalSuccesses = prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "wgmesh_nat_traversal_successes_total",
    Help: "Successful NAT traversal exchanges by method",
}, []string{"method"})
```

In `RegisterMetrics()`, add:

```go
prometheus.MustRegister(natTraversalAttempts)
prometheus.MustRegister(natTraversalSuccesses)
```

Add two exported functions:

```go
// RecordNATTraversalAttempt increments the attempt counter for the given method.
// method is the discovery method string, e.g. "dht", "dht-rendezvous", "dht-ipv6-sync".
func RecordNATTraversalAttempt(method string) {
    natTraversalAttempts.WithLabelValues(method).Inc()
}

// RecordNATTraversalSuccess increments the success counter for the given method.
func RecordNATTraversalSuccess(method string) {
    natTraversalSuccesses.WithLabelValues(method).Inc()
}
```

### Task 5: Call NAT traversal counters from `pkg/discovery/dht.go`

`exchangeWithAddress()` (line ~1054) already has the right structure: it calls `d.exchange.ExchangeWithPeer(addrStr)` and branches on `err` and `peerInfo == nil`. The `discoveryMethod` argument carries the method label.

At the top of `exchangeWithAddress()`, immediately after the early returns for IPv6 checks, add:

```go
daemon.RecordNATTraversalAttempt(discoveryMethod)
```

After the `log.Printf("[DHT] SUCCESS! ...")` line, add:

```go
daemon.RecordNATTraversalSuccess(discoveryMethod)
```

This import of `daemon` already exists in `pkg/discovery/init.go`; add `"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"` to the imports of `dht.go`.

### Task 6: Call `RecordDiscoveryEvent` from each discovery layer

The function `daemon.RecordDiscoveryEvent(layer string)` exists in `pkg/daemon/metrics.go` but is never called. Wire it in four files. The label string to pass matches the existing log tag in each file.

**`pkg/discovery/dht.go`** — In `exchangeWithAddress()`, after `RecordNATTraversalSuccess` (from Task 5), add:

```go
daemon.RecordDiscoveryEvent("dht")
```

**`pkg/discovery/lan.go`** — Find the code path where a valid LAN peer is found after a UDP multicast receive and a successful exchange. After adding the peer to the store, add:

```go
daemon.RecordDiscoveryEvent("lan")
```

**`pkg/discovery/gossip.go`** — Find the code path where a gossip message is successfully decoded and a peer is updated. After the peer update, add:

```go
daemon.RecordDiscoveryEvent("gossip")
```

**`pkg/discovery/registry.go`** — Find the code path where a peer is retrieved from the GitHub Issue registry and added. After the peer update, add:

```go
daemon.RecordDiscoveryEvent("registry")
```

In each file, add the import `"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"` if it is not already present.

### Task 7: Add tests in `pkg/daemon/metrics_test.go`

Append the following test functions to the existing `pkg/daemon/metrics_test.go`:

```go
func TestProbeRTTHistogram(t *testing.T) {
    // Record a probe RTT and verify histogram has at least one observation.
    start := time.Now()
    time.Sleep(1 * time.Millisecond)
    ObserveProbeRTT("abcdefgh", start)

    ch := make(chan prometheus.Metric, 1)
    probeRTT.WithLabelValues("abcdefgh").Collect(ch)
    m := <-ch
    var metric dto.Metric
    if err := m.Write(&metric); err != nil {
        t.Fatalf("write metric: %v", err)
    }
    h := metric.GetHistogram()
    if h == nil {
        t.Fatal("expected histogram metric")
    }
    if h.GetSampleCount() == 0 {
        t.Error("expected at least one RTT observation")
    }
}

func TestNATTraversalCounters(t *testing.T) {
    natTraversalAttempts.DeleteLabelValues("dht")
    natTraversalSuccesses.DeleteLabelValues("dht")

    RecordNATTraversalAttempt("dht")
    RecordNATTraversalAttempt("dht")
    RecordNATTraversalSuccess("dht")

    attempts := testutil.ToFloat64(natTraversalAttempts.WithLabelValues("dht"))
    if attempts != 2 {
        t.Errorf("expected 2 dht attempts, got %v", attempts)
    }
    successes := testutil.ToFloat64(natTraversalSuccesses.WithLabelValues("dht"))
    if successes != 1 {
        t.Errorf("expected 1 dht success, got %v", successes)
    }
}
```

### Task 8: Document metrics in `README.md`

Add a new `## Metrics` section immediately after the `### Querying the Daemon` subsection (which ends around line 137 of README.md). The content must be:

```markdown
### Metrics

wgmesh exposes a Prometheus-compatible `/metrics` endpoint. Enable it with the `--metrics` flag on `join`:

```bash
wgmesh join --secret "wgmesh://v1/<your-secret>" --metrics :9090
```

Then scrape `http://<host>:9090/metrics`.

#### Available metrics

| Metric | Type | Description |
|---|---|---|
| `wgmesh_active_peers` | Gauge | Current active peers in the mesh |
| `wgmesh_relayed_peers` | Gauge | Peers routed via relay (not direct) |
| `wgmesh_nat_type{type}` | Gauge | Local NAT type — `type` is `cone`, `symmetric`, or `unknown`; value is 1 for the current type |
| `wgmesh_discovery_events_total{layer}` | Counter | Peer-discovery events by layer — `layer` is `dht`, `lan`, `gossip`, or `registry` |
| `wgmesh_nat_traversal_attempts_total{method}` | Counter | NAT traversal attempts by method |
| `wgmesh_nat_traversal_successes_total{method}` | Counter | Successful NAT traversal exchanges by method |
| `wgmesh_probe_rtt_seconds{peer_key}` | Histogram | Mesh probe round-trip time per peer (first 8 chars of pubkey) |
| `wgmesh_reconcile_duration_seconds` | Histogram | Time spent in the reconcile loop |
| `go_goroutines` | Gauge | Number of active goroutines (Go runtime) |
| `go_memstats_alloc_bytes` | Gauge | Allocated heap bytes (Go runtime) |
| `process_resident_memory_bytes` | Gauge | Resident memory (OS process) |

#### Example Prometheus scrape config

```yaml
scrape_configs:
  - job_name: wgmesh
    static_configs:
      - targets: ['<node1>:9090', '<node2>:9090']
```
```

### Task 9: Document metrics in `docs/quickstart.md`

Find the section that describes running `wgmesh join` (Step 3 of the quickstart). After the code block showing the `join` command, add a callout paragraph:

```markdown
> **Tip:** Add `--metrics :9090` to expose Prometheus metrics at `http://<host>:9090/metrics`.
> See the [Metrics section in README.md](../README.md#metrics) for all available metric names.
```

## Affected Files

| File | Change |
|---|---|
| `pkg/daemon/metrics.go` | Add `probeRTT`, `natTraversalAttempts`, `natTraversalSuccesses` metrics; add `ObserveProbeRTT`, `RecordNATTraversalAttempt`, `RecordNATTraversalSuccess` functions; register `GoCollector` and `ProcessCollector` in `RegisterMetrics()` |
| `pkg/daemon/metrics_test.go` | Add `TestProbeRTTHistogram` and `TestNATTraversalCounters` test functions |
| `pkg/daemon/daemon.go` | Call `ObserveProbeRTT` in `probePeer()` |
| `pkg/discovery/dht.go` | Call `RecordNATTraversalAttempt`, `RecordNATTraversalSuccess`, and `RecordDiscoveryEvent("dht")` in `exchangeWithAddress()`; add daemon import |
| `pkg/discovery/lan.go` | Call `RecordDiscoveryEvent("lan")` after successful peer exchange; add daemon import if absent |
| `pkg/discovery/gossip.go` | Call `RecordDiscoveryEvent("gossip")` after successful peer decode; add daemon import if absent |
| `pkg/discovery/registry.go` | Call `RecordDiscoveryEvent("registry")` after peer retrieved from registry; add daemon import if absent |
| `README.md` | Add `### Metrics` subsection with metric table and scrape config example |
| `docs/quickstart.md` | Add tip callout after the `join` command |

## Test Strategy

1. Run `go test ./pkg/daemon/...` — all existing tests plus `TestProbeRTTHistogram` and `TestNATTraversalCounters` must pass.
2. Run `go test -race ./...` — no race detector warnings.
3. Run `go build ./...` — compilation must succeed.
4. Manual smoke test: start `wgmesh join --secret <secret> --metrics :9090`, then `curl http://localhost:9090/metrics` and verify the following lines are present:
   - `wgmesh_active_peers`
   - `go_goroutines`
   - `process_resident_memory_bytes`
   - `wgmesh_reconcile_duration_seconds`

## Estimated Complexity
low
