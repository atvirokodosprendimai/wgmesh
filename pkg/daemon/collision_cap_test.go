package daemon

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/node"
)

// capDaemon is a node on an address no key derives, with a controllable clock.
func capDaemon(t *testing.T) (*Daemon, *time.Time) {
	t.Helper()
	enforceIPBinding(t, false)
	d := collisionDaemon(t, "zzz-local", "10.42.1.1")
	d.stateFile = t.TempDir() + "/wg0.json"
	if err := saveLocalNode(d.stateFile, d.localNode); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	d.collisionNow = func() time.Time { return now }
	return d, &now
}

// A second move inside the window is suppressed and logged once, with no
// address, state file or interface change; it is allowed after the window.
func TestLocalMoveIsCappedPerWindow(t *testing.T) {
	set := recordAddresses(t)
	d, now := capDaemon(t)
	buf := captureLog(t)

	d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-remote", MeshIP: "10.42.1.1"}, "test")
	d.CheckAndResolveCollisions()
	if len(*set) != 1 {
		t.Fatalf("first move: addresses set %v, want one", *set)
	}

	// The node is back on an unbound address and loses again.
	d.localNode.SetMeshIP("10.42.2.2")
	if err := saveLocalNode(d.stateFile, d.localNode); err != nil {
		t.Fatal(err)
	}
	stateBefore := readState(t, d)
	d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-remote2", MeshIP: "10.42.2.2"}, "test")

	*now = now.Add(collisionMoveInterval - time.Second)
	for tick := 0; tick < 3; tick++ {
		d.CheckAndResolveCollisions()
	}
	if d.localNode.GetMeshIP() != "10.42.2.2" || len(*set) != 1 || string(readState(t, d)) != string(stateBefore) {
		t.Errorf("a move inside the window was not suppressed: IP %s, addresses set %v", d.localNode.GetMeshIP(), *set)
	}
	if n := strings.Count(buf.String(), "move suppressed"); n != 1 {
		t.Errorf("suppression logged %d times over 3 ticks, want 1", n)
	}

	*now = now.Add(time.Second)
	d.CheckAndResolveCollisions()
	if d.localNode.GetMeshIP() == "10.42.2.2" || len(*set) != 2 {
		t.Errorf("no move after the window: IP %s, addresses set %v", d.localNode.GetMeshIP(), *set)
	}

	// A new collision inside the next window is held back and logged again.
	d.localNode.SetMeshIP("10.42.3.3")
	d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-remote3", MeshIP: "10.42.3.3"}, "test")
	d.CheckAndResolveCollisions()
	if d.localNode.GetMeshIP() != "10.42.3.3" || len(*set) != 2 || strings.Count(buf.String(), "move suppressed") != 2 {
		t.Errorf("second window: IP %s, addresses set %v, suppression logged %d times, want held back and 2",
			d.localNode.GetMeshIP(), *set, strings.Count(buf.String(), "move suppressed"))
	}
}

// When every nonce address is claimed and the first one is the address the
// node holds, there is nothing to move to: no state write, no interface call,
// and the cap window does not start.
func TestLocalNodeWithNowhereToMoveWritesNothing(t *testing.T) {
	set := recordAddresses(t)
	d, _ := capDaemon(t)
	n1 := nonceOne(d)
	d.localNode.SetMeshIP(n1)
	if err := saveLocalNode(d.stateFile, d.localNode); err != nil {
		t.Fatal(err)
	}
	before := readState(t, d)

	pol := *d.peerStore.Policy()
	base := pol.Derive
	pol.Derive = func(key string, nonce int) string {
		if key == "aaa-remote" && nonce == 0 {
			return n1
		}
		return base(key, nonce)
	}
	d.peerStore.SetPolicy(&pol)
	d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-remote", MeshIP: n1}, "test")
	for n := 2; n <= node.MaxNonce; n++ {
		d.peerStore.Update(&PeerInfo{
			WGPubKey: fmt.Sprintf("aaa-n%02d", n),
			MeshIP:   DeriveMeshIPWithNonce(d.config.Keys.MeshSubnet, "zzz-local", collisionSecret, n),
		}, "test")
	}
	for tick := 0; tick < 3; tick++ {
		d.CheckAndResolveCollisions()
	}
	if len(*set) != 0 || string(readState(t, d)) != string(before) || !d.collisionLastMove.IsZero() {
		t.Errorf("addresses set %v, state changed %v, window started %v", *set, string(readState(t, d)) != string(before), !d.collisionLastMove.IsZero())
	}
}
