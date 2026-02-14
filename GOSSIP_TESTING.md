# How to Test Gossip

This repository includes an in-mesh gossip implementation in `pkg/discovery/gossip.go`.
Gossip is enabled by passing the `--gossip` flag to `wgmesh join`, which starts
the `MeshGossip` component alongside DHT-based discovery.

## Option 1: Local Loopback Unit Test (Recommended)

This test runs two `MeshGossip` instances on loopback and verifies that peer
information propagates. It does not require WireGuard or DHT.

1. Create `pkg/discovery/gossip_test.go` with a loopback test.
2. Run the test with `go test ./pkg/discovery -run TestMeshGossipLoopback -count=1`.

Example test skeleton:

```go
package discovery

import (
	"net"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

func TestMeshGossipLoopback(t *testing.T) {
	cfgA, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh://v1/test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	cfgB, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh://v1/test-secret"})
	if err != nil {
		t.Fatal(err)
	}

	storeA := daemon.NewPeerStore()
	storeB := daemon.NewPeerStore()

	nodeA := &LocalNode{WGPubKey: "A", MeshIP: "127.0.0.1", WGEndpoint: "127.0.0.1:51820"}
	nodeB := &LocalNode{WGPubKey: "B", MeshIP: "127.0.0.1", WGEndpoint: "127.0.0.1:51820"}

	// Seed each store with the other peer so gossip has a target.
	storeA.Update(&daemon.PeerInfo{WGPubKey: "B", MeshIP: nodeB.MeshIP}, "seed")
	storeB.Update(&daemon.PeerInfo{WGPubKey: "A", MeshIP: nodeA.MeshIP}, "seed")

	gossipA, err := NewMeshGossip(cfgA, nodeA, storeA)
	if err != nil {
		t.Fatal(err)
	}
	gossipB, err := NewMeshGossip(cfgB, nodeB, storeB)
	if err != nil {
		t.Fatal(err)
	}

	if err := gossipA.Start(); err != nil {
		t.Fatal(err)
	}
	defer gossipA.Stop()

	if err := gossipB.Start(); err != nil {
		t.Fatal(err)
	}
	defer gossipB.Stop()

	// Wait for at least one gossip interval.
	time.Sleep(2 * GossipInterval)

	// Validate that each side learned the other via gossip.
	peersA := storeA.GetActive()
	peersB := storeB.GetActive()
	if len(peersA) == 0 || len(peersB) == 0 {
		t.Fatalf("expected gossip peers; got A=%d B=%d", len(peersA), len(peersB))
	}

	// Ensure both listeners are bound (helps debug port conflicts).
	if gossipA.conn == nil || gossipB.conn == nil {
		t.Fatal("gossip did not bind UDP sockets")
	}
	if _, ok := gossipA.conn.LocalAddr().(*net.UDPAddr); !ok {
		t.Fatal("unexpected local addr type")
	}
}
```

Notes:
- This uses `LocalNode` from `pkg/discovery/dht.go`, which is also used by gossip.
- If the test flakes, reduce `GossipInterval` in the test by temporarily
  overriding it or by polling `PeerStore` with a timeout loop.

## Option 2: Manual Integration Test (Requires WireGuard)

This verifies gossip inside an actual mesh using the `--gossip` flag.

1. Build the binary: `go build -o wgmesh`.
2. On three nodes (A, B, C) run:
   - `./wgmesh init --secret` on one node and copy the secret.
   - `./wgmesh join --secret "<SECRET>" --gossip --log-level debug` on all nodes.
3. Validate behavior:
   - On A and B, confirm they connect (via DHT exchange).
   - After B is connected to A, start C. C should learn about B via gossip
     through A (watch for `[Gossip]` log entries and `DiscoveredVia` entries
     tagged with `gossip` or `gossip-transitive`).

Notes:
- The `--gossip` flag enables in-mesh peer propagation via UDP gossip messages.
- Gossip uses the same UDP socket as peer exchange, routing ANNOUNCE messages
  to the gossip handler.

## Troubleshooting

- Ensure all nodes use the same secret (gossip key and port are derived from it).
- Check UDP access on the derived gossip port:
  - `wgmesh status --secret "<SECRET>"` prints `Gossip Port`.
- If you only see `dht` discoveries and no `gossip`, confirm the gossip loop
  is started and that peers have mesh IPs assigned.
