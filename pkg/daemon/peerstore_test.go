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

func TestPeerStorePrefersRendezvousEndpointOverTransitive(t *testing.T) {
	ps := NewPeerStore()

	ps.Update(&PeerInfo{
		WGPubKey: "key1",
		MeshIP:   "10.0.0.1",
		Endpoint: "203.0.113.10:51821",
	}, "dht-rendezvous")

	ps.Update(&PeerInfo{
		WGPubKey: "key1",
		Endpoint: "198.51.100.10:51820",
	}, "dht-transitive")

	got, _ := ps.Get("key1")
	if got.Endpoint != "203.0.113.10:51821" {
		t.Errorf("Expected rendezvous endpoint to be preserved, got %s", got.Endpoint)
	}
}

func TestPeerStoreRendezvousOverridesDHT(t *testing.T) {
	ps := NewPeerStore()

	ps.Update(&PeerInfo{
		WGPubKey: "key1",
		MeshIP:   "10.0.0.1",
		Endpoint: "198.51.100.10:51820",
	}, "dht")

	ps.Update(&PeerInfo{
		WGPubKey: "key1",
		Endpoint: "203.0.113.10:51821",
	}, "dht-rendezvous")

	got, _ := ps.Get("key1")
	if got.Endpoint != "203.0.113.10:51821" {
		t.Errorf("Expected rendezvous endpoint to override dht endpoint, got %s", got.Endpoint)
	}
}

func TestPeerStorePrefersIPv6EndpointAtSameRank(t *testing.T) {
	ps := NewPeerStore()

	ps.Update(&PeerInfo{
		WGPubKey: "key-v6",
		MeshIP:   "10.0.0.5",
		Endpoint: "203.0.113.10:51820",
	}, "dht")

	ps.Update(&PeerInfo{
		WGPubKey: "key-v6",
		Endpoint: "[2001:db8::10]:51820",
	}, "dht")

	got, _ := ps.Get("key-v6")
	if got.Endpoint != "[2001:db8::10]:51820" {
		t.Errorf("Expected IPv6 endpoint to be preferred, got %s", got.Endpoint)
	}
}

func TestPeerStoreKeepsIPv6OverIPv4AtSameRank(t *testing.T) {
	ps := NewPeerStore()

	ps.Update(&PeerInfo{
		WGPubKey: "key-v6-sticky",
		MeshIP:   "10.0.0.6",
		Endpoint: "[2001:db8::20]:51820",
	}, "dht")

	ps.Update(&PeerInfo{
		WGPubKey: "key-v6-sticky",
		Endpoint: "198.51.100.20:51820",
	}, "dht")

	got, _ := ps.Get("key-v6-sticky")
	if got.Endpoint != "[2001:db8::20]:51820" {
		t.Errorf("Expected IPv6 endpoint to stay preferred, got %s", got.Endpoint)
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

func TestPeerStoreCacheLastSeenPreservedOnInsert(t *testing.T) {
	ps := NewPeerStore()
	old := time.Now().Add(-30 * time.Minute).Round(time.Second)

	ps.Update(&PeerInfo{WGPubKey: "cached", LastSeen: old}, "cache")

	got, ok := ps.Get("cached")
	if !ok {
		t.Fatal("expected cached peer")
	}
	if !got.LastSeen.Equal(old) {
		t.Fatalf("expected LastSeen %v, got %v", old, got.LastSeen)
	}
}

func TestPeerStoreTransitiveDoesNotRefreshLastSeen(t *testing.T) {
	ps := NewPeerStore()
	ps.Update(&PeerInfo{WGPubKey: "peer1", Endpoint: "198.51.100.1:51820"}, "dht")

	ps.mu.Lock()
	base := time.Now().Add(-4 * time.Minute).Round(time.Second)
	ps.peers["peer1"].LastSeen = base
	ps.mu.Unlock()

	ps.Update(&PeerInfo{WGPubKey: "peer1", Endpoint: "198.51.100.2:51820"}, "gossip-transitive")

	got, _ := ps.Get("peer1")
	if !got.LastSeen.Equal(base) {
		t.Fatalf("expected LastSeen to remain %v, got %v", base, got.LastSeen)
	}
}

func TestPeerStoreSubscribe(t *testing.T) {
	ps := NewPeerStore()
	ch := ps.Subscribe()

	// New peer should emit event
	ps.Update(&PeerInfo{WGPubKey: "key1", MeshIP: "10.0.0.1", Endpoint: "1.2.3.4:51820"}, "dht")

	select {
	case ev := <-ch:
		if ev.PubKey != "key1" {
			t.Errorf("Expected pubkey key1, got %s", ev.PubKey)
		}
		if ev.Kind != PeerEventNew {
			t.Errorf("Expected PeerEventNew, got %d", ev.Kind)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timed out waiting for new peer event")
	}
}

func TestPeerStoreSubscribeUpdate(t *testing.T) {
	ps := NewPeerStore()

	// Add peer before subscribing
	ps.Update(&PeerInfo{WGPubKey: "key1", MeshIP: "10.0.0.1", Endpoint: "1.2.3.4:51820"}, "dht")

	ch := ps.Subscribe()

	// Update peer — should emit update event
	ps.Update(&PeerInfo{WGPubKey: "key1", Endpoint: "5.6.7.8:51820"}, "lan")

	select {
	case ev := <-ch:
		if ev.PubKey != "key1" {
			t.Errorf("Expected pubkey key1, got %s", ev.PubKey)
		}
		if ev.Kind != PeerEventUpdated {
			t.Errorf("Expected PeerEventUpdated, got %d", ev.Kind)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timed out waiting for update event")
	}
}

func TestPeerStoreSubscribeNonBlocking(t *testing.T) {
	ps := NewPeerStore()
	ch := ps.Subscribe()

	// Fill the channel buffer
	for i := 0; i < PeerEventBufSize+5; i++ {
		ps.Update(&PeerInfo{
			WGPubKey: "key1",
			MeshIP:   "10.0.0.1",
			Endpoint: "1.2.3.4:51820",
		}, "dht")
	}

	// Should not deadlock — Update() must be non-blocking even when
	// subscriber is slow. Drain what we can.
	drained := 0
	for {
		select {
		case <-ch:
			drained++
		default:
			goto done
		}
	}
done:
	if drained == 0 {
		t.Error("Expected at least one event to be buffered")
	}
	if drained > PeerEventBufSize {
		t.Errorf("Drained %d events, expected at most %d (buffer size)", drained, PeerEventBufSize)
	}
}

func TestPeerStoreUnsubscribe(t *testing.T) {
	ps := NewPeerStore()
	ch := ps.Subscribe()

	ps.Unsubscribe(ch)

	// After unsubscribe, updates should not panic or block
	ps.Update(&PeerInfo{WGPubKey: "key1", MeshIP: "10.0.0.1"}, "dht")

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("Expected channel to be closed after unsubscribe")
		}
	case <-time.After(50 * time.Millisecond):
		// Channel closed, no more events — also acceptable
	}
}

func TestPeerStoreMultipleSubscribers(t *testing.T) {
	ps := NewPeerStore()
	ch1 := ps.Subscribe()
	ch2 := ps.Subscribe()

	ps.Update(&PeerInfo{WGPubKey: "key1", MeshIP: "10.0.0.1"}, "dht")

	for _, tt := range []struct {
		name string
		ch   <-chan PeerEvent
	}{
		{"subscriber1", ch1},
		{"subscriber2", ch2},
	} {
		select {
		case ev := <-tt.ch:
			if ev.PubKey != "key1" {
				t.Errorf("%s: expected pubkey key1, got %s", tt.name, ev.PubKey)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("%s: timed out waiting for event", tt.name)
		}
	}
}
