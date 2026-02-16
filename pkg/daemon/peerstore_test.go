package daemon

import (
	"testing"
	"time"
)

func TestPeerStoreUpdate(t *testing.T) {
	ps := NewPeerStore()

	peer := &PeerInfo{
		WGPubKey: "key1",
		MeshIP:   "10.0.0.1",
		Endpoint: "1.2.3.4:51820",
	}

	ps.Update(peer, "test")

	if ps.Count() != 1 {
		t.Errorf("Expected 1 peer, got %d", ps.Count())
	}

	got, ok := ps.Get("key1")
	if !ok {
		t.Fatal("Expected to find peer key1")
	}
	if got.MeshIP != "10.0.0.1" {
		t.Errorf("Expected MeshIP 10.0.0.1, got %s", got.MeshIP)
	}
}

func TestPeerStoreUpdateMerge(t *testing.T) {
	ps := NewPeerStore()

	ps.Update(&PeerInfo{
		WGPubKey: "key1",
		MeshIP:   "10.0.0.1",
		Endpoint: "1.2.3.4:51820",
	}, "dht")

	ps.Update(&PeerInfo{
		WGPubKey: "key1",
		Endpoint: "5.6.7.8:51820",
	}, "lan")

	got, _ := ps.Get("key1")
	if got.Endpoint != "5.6.7.8:51820" {
		t.Errorf("Expected updated endpoint, got %s", got.Endpoint)
	}
	if got.MeshIP != "10.0.0.1" {
		t.Errorf("MeshIP should be preserved, got %s", got.MeshIP)
	}
	if len(got.DiscoveredVia) != 2 {
		t.Errorf("Expected 2 discovery methods, got %d", len(got.DiscoveredVia))
	}
}

func TestPeerStorePrefersLANEndpoint(t *testing.T) {
	ps := NewPeerStore()

	ps.Update(&PeerInfo{
		WGPubKey: "key1",
		MeshIP:   "10.0.0.1",
		Endpoint: "192.168.1.10:51820",
	}, "lan")

	ps.Update(&PeerInfo{
		WGPubKey: "key1",
		Endpoint: "203.0.113.10:51820",
	}, "dht")

	got, _ := ps.Get("key1")
	if got.Endpoint != "192.168.1.10:51820" {
		t.Errorf("Expected LAN endpoint to be preserved, got %s", got.Endpoint)
	}
}

func TestPeerStoreGetActive(t *testing.T) {
	ps := NewPeerStore()

	ps.Update(&PeerInfo{WGPubKey: "active", MeshIP: "10.0.0.1"}, "test")

	// Directly manipulate to add stale peer
	ps.mu.Lock()
	ps.peers["stale"] = &PeerInfo{
		WGPubKey: "stale",
		MeshIP:   "10.0.0.2",
		LastSeen: time.Now().Add(-10 * time.Minute),
	}
	ps.mu.Unlock()

	active := ps.GetActive()
	if len(active) != 1 {
		t.Errorf("Expected 1 active peer, got %d", len(active))
	}
	if active[0].WGPubKey != "active" {
		t.Error("Expected active peer to be 'active'")
	}
}

func TestPeerStoreRemove(t *testing.T) {
	ps := NewPeerStore()

	ps.Update(&PeerInfo{WGPubKey: "key1"}, "test")
	ps.Update(&PeerInfo{WGPubKey: "key2"}, "test")

	ps.Remove("key1")

	if ps.Count() != 1 {
		t.Errorf("Expected 1 peer after remove, got %d", ps.Count())
	}
}

func TestPeerStoreCleanupStale(t *testing.T) {
	ps := NewPeerStore()

	ps.Update(&PeerInfo{WGPubKey: "recent"}, "test")

	ps.mu.Lock()
	ps.peers["old"] = &PeerInfo{
		WGPubKey: "old",
		LastSeen: time.Now().Add(-15 * time.Minute), // Beyond PeerRemoveTimeout
	}
	ps.mu.Unlock()

	removed := ps.CleanupStale()
	if len(removed) != 1 {
		t.Errorf("Expected 1 removed peer, got %d", len(removed))
	}
	if removed[0] != "old" {
		t.Error("Expected 'old' to be removed")
	}
}

func TestPeerStoreIsDead(t *testing.T) {
	ps := NewPeerStore()

	ps.Update(&PeerInfo{WGPubKey: "alive"}, "test")

	if ps.IsDead("alive") {
		t.Error("Recently updated peer should not be dead")
	}

	if !ps.IsDead("nonexistent") {
		t.Error("Non-existent peer should be dead")
	}
}
