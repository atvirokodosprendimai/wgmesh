package daemon

import (
	"strings"
	"testing"
	"time"
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
}

// The first move of a node is never held back by the cap.
func TestLocalFirstMoveIsNotCapped(t *testing.T) {
	set := recordAddresses(t)
	d, _ := capDaemon(t)
	d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-remote", MeshIP: "10.42.1.1"}, "test")
	d.CheckAndResolveCollisions()
	if len(*set) != 1 {
		t.Errorf("addresses set %v, want one", *set)
	}
}
