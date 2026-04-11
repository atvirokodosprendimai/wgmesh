# Specification: Issue #505

## Classification
feature

## Deliverables
code

## Problem Analysis

The wgmesh daemon currently measures round-trip time (RTT) to peers via a TCP ping/pong probe loop (`probePeer()` in `pkg/daemon/daemon.go`) and records it in a Prometheus histogram (`wgmesh_probe_rtt_seconds`). However, three gaps remain:

1. **RTT is not stored in `PeerInfo`.** `probePeer()` calls `ObserveProbeRTT()` but never writes back to `PeerInfo.Latency`, which exists as `Latency *time.Duration` (peerstore.go line 49) but is always `nil`.
2. **RTT is invisible over RPC.** `RPCPeerData` (`pkg/daemon/daemon.go` line 1737), `rpc.PeerData` (`pkg/rpc/server.go` line 17), and `rpc.PeerInfo` (`pkg/rpc/protocol.go` line 41) all omit the latency field. The `peers.list` and `peers.get` RPC methods therefore never send latency to the client.
3. **Latency is not visible in the CLI.** `handlePeersList()` in `main.go` (line 947) formats a table without a latency column. `wgmesh peers list` is the existing per-peer status view; this is the natural place to surface RTT.
4. **No pre-computed P95/P99 quantiles in the scrape output.** `wgmesh_probe_rtt_seconds` is a histogram, so quantiles are only computed server-side in Prometheus (via `histogram_quantile()`). Operators who do not have a Prometheus instance cannot directly see P95/P99. Adding a Prometheus `SummaryVec` alongside the histogram pre-computes P50/P95/P99 in the scrape output.

Existing infrastructure that is already correct and must not be changed:
- `PeerInfo.Latency *time.Duration` field in `pkg/daemon/peerstore.go`
- `probeRTT` histogram and `ObserveProbeRTT()` in `pkg/daemon/metrics.go`
- `probePeer()` in `pkg/daemon/daemon.go` that times the ping/pong and calls `ObserveProbeRTT()`
- `meshProbeLoop()` / `probePeersOverMesh()` driving periodic probes at `MeshProbeInterval`

## Implementation Tasks

### Task 1: Add `SetLatency` method to `PeerStore` in `pkg/daemon/peerstore.go`

Add the following method to `PeerStore` immediately after the `IsDead` method (after line 373):

```go
// SetLatency updates the measured round-trip latency for the given peer.
// It is a no-op if the peer is not present in the store.
func (ps *PeerStore) SetLatency(pubKey string, rtt time.Duration) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	peer, exists := ps.peers[pubKey]
	if !exists {
		return
	}
	peer.Latency = &rtt
}
```

No imports need to be added; `time` is already imported.

### Task 2: Call `SetLatency` from `probePeer` in `pkg/daemon/daemon.go`

In `probePeer()` (around line 1063), after the existing call to `ObserveProbeRTT` and before `return true`, add:

```go
rtt := time.Since(start)
ObserveProbeRTT(peer.WGPubKey[:8], start)
d.peerStore.SetLatency(peer.WGPubKey, rtt)
return true
```

Replace the current two-line block:
```go
ObserveProbeRTT(peer.WGPubKey[:8], start)
return true
```
with:
```go
rtt := time.Since(start)
ObserveProbeRTT(peer.WGPubKey[:8], start)
d.peerStore.SetLatency(peer.WGPubKey, rtt)
return true
```

`start` is already computed earlier in the function (before `session.conn.Write`). Do NOT record `SetLatency` on failure paths — only on the successful ping/pong round-trip.

### Task 3: Add latency field to `pkg/daemon/metrics.go` — P95/P99 Summary

In `pkg/daemon/metrics.go`, add a `SummaryVec` alongside the existing `probeRTT` histogram in the `var` block, immediately after the `probeRTT` definition:

```go
probeRTTSummary = prometheus.NewSummaryVec(prometheus.SummaryOpts{
    Name: "wgmesh_probe_rtt_summary_seconds",
    Help: "Round-trip time of mesh health probes (pre-computed quantiles)",
    Objectives: map[float64]float64{
        0.50: 0.01,
        0.95: 0.005,
        0.99: 0.001,
    },
    MaxAge:     10 * time.Minute,
    AgeBuckets: 5,
}, []string{"peer_key"})
```

In `RegisterMetrics()`, add after the existing `prometheus.MustRegister(probeRTT)` line:

```go
prometheus.MustRegister(probeRTTSummary)
```

Add a new exported function immediately after `ObserveProbeRTT`:

```go
// ObserveProbeRTTSummary records the round-trip time for a mesh probe to the given peer
// in the pre-computed summary (P50/P95/P99).
// peerKey should be the first 8 characters of the WireGuard public key.
func ObserveProbeRTTSummary(peerKey string, rtt time.Duration) {
	probeRTTSummary.WithLabelValues(peerKey).Observe(rtt.Seconds())
}
```

### Task 4: Call `ObserveProbeRTTSummary` from `probePeer` in `pkg/daemon/daemon.go`

In the same block added in Task 2, call the new function so both instruments are updated:

```go
rtt := time.Since(start)
ObserveProbeRTT(peer.WGPubKey[:8], start)
ObserveProbeRTTSummary(peer.WGPubKey[:8], rtt)
d.peerStore.SetLatency(peer.WGPubKey, rtt)
return true
```

### Task 5: Add `LatencyMs` field to `RPCPeerData` in `pkg/daemon/daemon.go`

In the `RPCPeerData` struct definition (around line 1737), add a `LatencyMs` field:

```go
type RPCPeerData struct {
	WGPubKey         string
	Hostname         string
	MeshIP           string
	Endpoint         string
	LastSeen         time.Time
	DiscoveredVia    []string
	RoutableNetworks []string
	LatencyMs        *float64 // nil when no probe has succeeded yet
}
```

In `GetRPCPeers()` (around line 1679), populate the new field when converting from `PeerInfo`:

```go
func (d *Daemon) GetRPCPeers() []*RPCPeerData {
	peers := d.peerStore.GetActive()
	result := make([]*RPCPeerData, 0, len(peers))
	for _, p := range peers {
		rpcPeer := &RPCPeerData{
			WGPubKey:         p.WGPubKey,
			Hostname:         p.Hostname,
			MeshIP:           p.MeshIP,
			Endpoint:         p.Endpoint,
			LastSeen:         p.LastSeen,
			DiscoveredVia:    p.DiscoveredVia,
			RoutableNetworks: p.RoutableNetworks,
		}
		if p.Latency != nil {
			ms := float64(p.Latency.Milliseconds())
			rpcPeer.LatencyMs = &ms
		}
		result = append(result, rpcPeer)
	}
	return result
}
```

Apply the same pattern in `GetRPCPeer()` (around line 1697):

```go
func (d *Daemon) GetRPCPeer(pubKey string) (*RPCPeerData, bool) {
	peer, exists := d.peerStore.Get(pubKey)
	if !exists {
		return nil, false
	}
	rpcPeer := &RPCPeerData{
		WGPubKey:         peer.WGPubKey,
		Hostname:         peer.Hostname,
		MeshIP:           peer.MeshIP,
		Endpoint:         peer.Endpoint,
		LastSeen:         peer.LastSeen,
		DiscoveredVia:    peer.DiscoveredVia,
		RoutableNetworks: peer.RoutableNetworks,
	}
	if peer.Latency != nil {
		ms := float64(peer.Latency.Milliseconds())
		rpcPeer.LatencyMs = &ms
	}
	return rpcPeer, true
}
```

### Task 6: Add `LatencyMs` to RPC protocol types in `pkg/rpc/`

**In `pkg/rpc/server.go`**, add `LatencyMs *float64` to `PeerData`:

```go
type PeerData struct {
	WGPubKey         string
	Hostname         string
	MeshIP           string
	Endpoint         string
	LastSeen         time.Time
	DiscoveredVia    []string
	RoutableNetworks []string
	LatencyMs        *float64
}
```

**In `pkg/rpc/protocol.go`**, add `LatencyMs *float64` to `PeerInfo` (the JSON-serialized type):

```go
type PeerInfo struct {
	PubKey           string   `json:"pubkey"`
	Hostname         string   `json:"hostname,omitempty"`
	MeshIP           string   `json:"mesh_ip"`
	Endpoint         string   `json:"endpoint"`
	LastSeen         string   `json:"last_seen"` // ISO 8601 format
	DiscoveredVia    []string `json:"discovered_via"`
	RoutableNetworks []string `json:"routable_networks,omitempty"`
	LatencyMs        *float64 `json:"latency_ms,omitempty"`
}
```

**In `pkg/rpc/server.go`**, update `handlePeersList()` and `handlePeersGet()` to copy `LatencyMs` from `PeerData` into `PeerInfo`:

In `handlePeersList()`:
```go
for _, peer := range peers {
    result.Peers = append(result.Peers, &PeerInfo{
        PubKey:           peer.WGPubKey,
        Hostname:         peer.Hostname,
        MeshIP:           peer.MeshIP,
        Endpoint:         peer.Endpoint,
        LastSeen:         peer.LastSeen.Format(time.RFC3339),
        DiscoveredVia:    peer.DiscoveredVia,
        RoutableNetworks: peer.RoutableNetworks,
        LatencyMs:        peer.LatencyMs,
    })
}
```

In `handlePeersGet()`:
```go
return &PeerInfo{
    PubKey:           peer.WGPubKey,
    Hostname:         peer.Hostname,
    MeshIP:           peer.MeshIP,
    Endpoint:         peer.Endpoint,
    LastSeen:         peer.LastSeen.Format(time.RFC3339),
    DiscoveredVia:    peer.DiscoveredVia,
    RoutableNetworks: peer.RoutableNetworks,
    LatencyMs:        peer.LatencyMs,
}, nil
```

### Task 7: Propagate `LatencyMs` through `main.go`

**In `createRPCServer()`** (around line 847), copy `LatencyMs` when building `rpc.PeerData` inside the `GetPeers` and `GetPeer` callbacks:

In the `GetPeers` closure:
```go
result[i] = &rpc.PeerData{
    WGPubKey:         p.WGPubKey,
    Hostname:         p.Hostname,
    MeshIP:           p.MeshIP,
    Endpoint:         p.Endpoint,
    LastSeen:         p.LastSeen,
    DiscoveredVia:    p.DiscoveredVia,
    RoutableNetworks: p.RoutableNetworks,
    LatencyMs:        p.LatencyMs,
}
```

In the `GetPeer` closure:
```go
return &rpc.PeerData{
    WGPubKey:         peer.WGPubKey,
    Hostname:         peer.Hostname,
    MeshIP:           peer.MeshIP,
    Endpoint:         peer.Endpoint,
    LastSeen:         peer.LastSeen,
    DiscoveredVia:    peer.DiscoveredVia,
    RoutableNetworks: peer.RoutableNetworks,
    LatencyMs:        peer.LatencyMs,
}, true
```

**In `handlePeersList()`** (around line 947), add a `LATENCY` column to the output table. Change the header line from:

```go
fmt.Printf("%-20s %-19s %-15s %-25s %-10s %s\n", "HOSTNAME", "PUBLIC KEY", "MESH IP", "ENDPOINT", "LAST SEEN", "DISCOVERED VIA")
fmt.Println(strings.Repeat("-", 120))
```

to:

```go
fmt.Printf("%-20s %-19s %-15s %-25s %-10s %-10s %s\n", "HOSTNAME", "PUBLIC KEY", "MESH IP", "ENDPOINT", "LAST SEEN", "LATENCY", "DISCOVERED VIA")
fmt.Println(strings.Repeat("-", 130))
```

Add latency extraction inside the peer loop, after reading `lastSeen`:

```go
latencyStr := "-"
if v, ok := peer["latency_ms"]; ok && v != nil {
    if ms, ok := v.(float64); ok {
        latencyStr = fmt.Sprintf("%.1fms", ms)
    }
}
```

Change the final `fmt.Printf` line from:

```go
fmt.Printf("%-20s %-19s %-15s %-25s %-10s %s\n", hostname, pubkeyShort, meshIP, endpoint, lastSeenStr, strings.Join(discoveredViaStr, ","))
```

to:

```go
fmt.Printf("%-20s %-19s %-15s %-25s %-10s %-10s %s\n", hostname, pubkeyShort, meshIP, endpoint, lastSeenStr, latencyStr, strings.Join(discoveredViaStr, ","))
```

**In `handlePeersGet()`** (around line 1050), display latency after the existing fields. After the `lastSeen` print line, add:

```go
if v, ok := peer["latency_ms"]; ok && v != nil {
    if ms, ok := v.(float64); ok {
        fmt.Printf("Latency:        %.1f ms\n", ms)
    }
} else {
    fmt.Printf("Latency:        -\n")
}
```

### Task 8: Add unit tests in `pkg/daemon/peerstore_test.go`

Append two test functions to the end of `pkg/daemon/peerstore_test.go`:

```go
func TestPeerStoreSetLatency(t *testing.T) {
	ps := NewPeerStore()
	ps.Update(&PeerInfo{
		WGPubKey: "key1",
		MeshIP:   "10.0.0.1",
	}, "dht")

	rtt := 42 * time.Millisecond
	ps.SetLatency("key1", rtt)

	got, ok := ps.Get("key1")
	if !ok {
		t.Fatal("expected to find peer key1")
	}
	if got.Latency == nil {
		t.Fatal("expected Latency to be set")
	}
	if *got.Latency != rtt {
		t.Errorf("expected Latency %v, got %v", rtt, *got.Latency)
	}
}

func TestPeerStoreSetLatencyUnknownPeer(t *testing.T) {
	ps := NewPeerStore()
	// SetLatency on a non-existent peer must not panic.
	ps.SetLatency("nonexistent", 10*time.Millisecond)
}
```

### Task 9: Add unit tests in `pkg/daemon/metrics_test.go`

Append a test function to the end of `pkg/daemon/metrics_test.go` to verify the RTT Summary:

```go
func TestProbeRTTSummary(t *testing.T) {
	rtt := 15 * time.Millisecond
	ObserveProbeRTTSummary("testkey1", rtt)

	ch := make(chan prometheus.Metric, 1)
	probeRTTSummary.WithLabelValues("testkey1").Collect(ch)
	m := <-ch
	var metric dto.Metric
	if err := m.Write(&metric); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	s := metric.GetSummary()
	if s == nil {
		t.Fatal("expected summary metric")
	}
	if s.GetSampleCount() == 0 {
		t.Error("expected at least one RTT observation in summary")
	}
}
```

Ensure the existing imports (`dto "github.com/prometheus/client_model/go"` and `"github.com/prometheus/client_golang/prometheus"`) are already present in `metrics_test.go`; add them if missing.

## Affected Files

| File | Change |
|---|---|
| `pkg/daemon/peerstore.go` | Add `SetLatency(pubKey string, rtt time.Duration)` method |
| `pkg/daemon/peerstore_test.go` | Add `TestPeerStoreSetLatency` and `TestPeerStoreSetLatencyUnknownPeer` |
| `pkg/daemon/daemon.go` | In `probePeer()`: compute `rtt`, call `ObserveProbeRTTSummary`, call `d.peerStore.SetLatency`; add `LatencyMs *float64` to `RPCPeerData`; populate in `GetRPCPeers()` and `GetRPCPeer()` |
| `pkg/daemon/metrics.go` | Add `probeRTTSummary` SummaryVec; add `ObserveProbeRTTSummary()` function; register in `RegisterMetrics()` |
| `pkg/daemon/metrics_test.go` | Add `TestProbeRTTSummary` |
| `pkg/rpc/server.go` | Add `LatencyMs *float64` to `PeerData`; copy into `PeerInfo` in `handlePeersList` and `handlePeersGet` |
| `pkg/rpc/protocol.go` | Add `LatencyMs *float64 \`json:"latency_ms,omitempty"\`` to `PeerInfo` |
| `main.go` | Copy `LatencyMs` in `createRPCServer()` `GetPeers`/`GetPeer` callbacks; add `LATENCY` column to `handlePeersList()`; print latency in `handlePeersGet()` |

## Test Strategy

1. Run `go test ./pkg/daemon/...` — existing tests plus `TestPeerStoreSetLatency`, `TestPeerStoreSetLatencyUnknownPeer`, and `TestProbeRTTSummary` must pass.
2. Run `go test -race ./...` — no race detector warnings (mutex in `SetLatency` prevents races).
3. Run `go build ./...` — compilation must succeed.
4. Manual smoke test: start `wgmesh join --secret <secret> --metrics :9090`, wait for a peer to connect, then:
   - `curl http://localhost:9090/metrics | grep wgmesh_probe_rtt_summary_seconds` — verify P50/P95/P99 quantile lines appear for at least one `peer_key` label.
   - `wgmesh peers list` — verify the `LATENCY` column shows a value like `2.3ms` for connected peers and `-` for peers not yet probed.
   - `wgmesh peers get <pubkey>` — verify `Latency:` line appears.

## Estimated Complexity
low
