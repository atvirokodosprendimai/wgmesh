package daemon

import (
	"fmt"
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

func TestPeerStoreMaxPeers(t *testing.T) {
	ps := NewPeerStore()

	// Add exactly DefaultMaxPeers
	for i := 0; i < DefaultMaxPeers; i++ {
		peer := &PeerInfo{
			WGPubKey: fmt.Sprintf("key%d", i),
			MeshIP:   fmt.Sprintf("10.0.%d.%d", i/255, i%255),
		}
		ps.Update(peer, "test")
	}

	if ps.Count() != DefaultMaxPeers {
		t.Errorf("Expected %d peers, got %d", DefaultMaxPeers, ps.Count())
	}

	// Try to add one more - should fail
	extraPeer := &PeerInfo{
		WGPubKey: "extra",
		MeshIP:   "10.1.0.1",
	}
	result := ps.Update(extraPeer, "test")

	if ps.Count() != DefaultMaxPeers {
		t.Errorf("Store should still have %d peers after rejection, got %d",
			DefaultMaxPeers, ps.Count())
	}

	if result {
		t.Error("Update should return false when at capacity")
	}

	if _, exists := ps.Get("extra"); exists {
		t.Error("Extra peer should not be in store")
	}
}

func TestPeerStoreCapacityRejection(t *testing.T) {
	ps := NewPeerStore()

	// Fill to capacity
	for i := 0; i < DefaultMaxPeers; i++ {
		peer := &PeerInfo{
			WGPubKey: fmt.Sprintf("key%d", i),
			MeshIP:   fmt.Sprintf("10.0.0.%d", i),
		}
		result := ps.Update(peer, "test")
		if !result {
			t.Errorf("Expected to add peer %d successfully", i)
		}
	}

	// Try to add beyond capacity - all should fail
	for i := 0; i < 10; i++ {
		extraPeer := &PeerInfo{
			WGPubKey: fmt.Sprintf("extra%d", i),
			MeshIP:   "10.1.0.1",
		}
		result := ps.Update(extraPeer, "test")
		if result {
			t.Errorf("Expected peer %d to be rejected", i)
		}
	}

	// Count should still be DefaultMaxPeers
	if ps.Count() != DefaultMaxPeers {
		t.Errorf("Expected %d peers, got %d", DefaultMaxPeers, ps.Count())
	}
}

func TestPeerStoreUpdateExistingAtCapacity(t *testing.T) {
	ps := NewPeerStore()

	// Fill to capacity
	for i := 0; i < DefaultMaxPeers; i++ {
		peer := &PeerInfo{
			WGPubKey: fmt.Sprintf("key%d", i),
			MeshIP:   fmt.Sprintf("10.0.0.%d", i),
			Endpoint: "1.2.3.4:51820",
		}
		ps.Update(peer, "test")
	}

	// Update an existing peer - should succeed
	updated := &PeerInfo{
		WGPubKey: "key0",
		Endpoint: "5.6.7.8:51820",
	}
	result := ps.Update(updated, "lan")

	if !result {
		t.Error("Update of existing peer should succeed at capacity")
	}

	peer, _ := ps.Get("key0")
	if peer.Endpoint != "5.6.7.8:51820" {
		t.Error("Should be able to update existing peer when at capacity")
	}

	// Count should remain DefaultMaxPeers
	if ps.Count() != DefaultMaxPeers {
		t.Errorf("Expected %d peers, got %d", DefaultMaxPeers, ps.Count())
	}
}

func TestPeerStoreCapacityAfterCleanup(t *testing.T) {
	ps := NewPeerStore()

	// Fill to capacity with old peers
	for i := 0; i < DefaultMaxPeers; i++ {
		ps.mu.Lock()
		ps.peers[fmt.Sprintf("old%d", i)] = &PeerInfo{
			WGPubKey: fmt.Sprintf("old%d", i),
			LastSeen: time.Now().Add(-15 * time.Minute),
		}
		ps.mu.Unlock()
	}

	// Cleanup should remove all stale peers
	removed := ps.CleanupStale()
	if len(removed) != DefaultMaxPeers {
		t.Errorf("Expected %d removed, got %d", DefaultMaxPeers, len(removed))
	}

	// Should now be able to add new peers
	newPeer := &PeerInfo{
		WGPubKey: "fresh",
		MeshIP:   "10.0.0.1",
	}
	result := ps.Update(newPeer, "test")

	if !result {
		t.Error("Should be able to add peer after cleanup")
	}

	if _, exists := ps.Get("fresh"); !exists {
		t.Error("Fresh peer should be in store after cleanup")
	}
}
