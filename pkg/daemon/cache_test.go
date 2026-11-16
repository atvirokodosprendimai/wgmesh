package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPeerCacheSaveAndLoad(t *testing.T) {
	// Use temp directory
	tmpDir := t.TempDir()
	interfaceName := "test-wg0"
	cachePath := filepath.Join(tmpDir, interfaceName+"-peers.json")

	// Create a peer store with some peers
	ps := NewPeerStore()
	ps.Update(&PeerInfo{
		WGPubKey: "pubkey1",
		MeshIP:   "10.0.0.1",
		Endpoint: "1.2.3.4:51820",
	}, "test")
	ps.Update(&PeerInfo{
		WGPubKey:         "pubkey2",
		MeshIP:           "10.0.0.2",
		Endpoint:         "5.6.7.8:51820",
		RoutableNetworks: []string{"192.168.0.0/24"},
	}, "test")

	// Save cache to custom path
	cache := &PeerCache{
		UpdatedAt: time.Now().Unix(),
	}
	for _, p := range ps.GetAll() {
		cache.Peers = append(cache.Peers, PeerCacheEntry{
			WGPubKey:         p.WGPubKey,
			MeshIP:           p.MeshIP,
			Endpoint:         p.Endpoint,
			RoutableNetworks: p.RoutableNetworks,
			LastSeen:         p.LastSeen.Unix(),
		})
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal cache: %v", err)
	}

	if err := os.WriteFile(cachePath, data, 0600); err != nil {
		t.Fatalf("Failed to write cache: %v", err)
	}

	// Load cache
	loaded, err := loadCacheFromPath(cachePath)
	if err != nil {
		t.Fatalf("Failed to load cache: %v", err)
	}

	if len(loaded.Peers) != 2 {
		t.Errorf("Expected 2 cached peers, got %d", len(loaded.Peers))
	}
}

// loadCacheFromPath loads a cache from a specific path (helper for testing)
func loadCacheFromPath(path string) (*PeerCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cache PeerCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return &cache, nil
}

func TestCacheExpiration(t *testing.T) {
	ps := NewPeerStore()

	// Add a peer with an old timestamp
	ps.SetPeerDirectly("old-key", &PeerInfo{
		WGPubKey: "old-key",
		MeshIP:   "10.0.0.1",
		LastSeen: time.Now().Add(-25 * time.Hour),
	})
	ps.SetPeerDirectly("new-key", &PeerInfo{
		WGPubKey: "new-key",
		MeshIP:   "10.0.0.2",
		LastSeen: time.Now(),
	})

	// Get all peers
	all := ps.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 peers, got %d", len(all))
	}

	// Active should only include the new one
	active := ps.GetActive()
	if len(active) != 1 {
		t.Errorf("Expected 1 active peer, got %d", len(active))
	}
}

// TestCacheRestoreOffersRestored verifies that a restored entry neither
// refreshes LastSeen nor withdraws the routes the store already holds.
func TestCacheRestoreOffersRestored(t *testing.T) {
	ps := NewPeerStore()
	seen := time.Now().Add(-2 * time.Minute)
	ps.SetPeerDirectly("k", &PeerInfo{
		WGPubKey:         "k",
		MeshIP:           "10.0.0.2",
		RoutableNetworks: []string{"192.168.0.0/24"},
		LastSeen:         seen,
	})
	cache := &PeerCache{Peers: []PeerCacheEntry{{
		WGPubKey: "k",
		MeshIP:   "10.0.0.2",
		Endpoint: "1.2.3.4:51820",
		LastSeen: time.Now().Add(-time.Hour).Unix(),
	}}}

	if n := restorePeers(cache, ps); n != 1 {
		t.Fatalf("restorePeers = %d, want 1", n)
	}

	got, ok := ps.Get("k")
	if !ok {
		t.Fatal("expected k in peer store")
	}
	if !got.LastSeen.Equal(seen) {
		t.Errorf("LastSeen = %v, want it unchanged at %v", got.LastSeen, seen)
	}
	if len(got.RoutableNetworks) != 1 {
		t.Errorf("RoutableNetworks = %v, want the stored route kept", got.RoutableNetworks)
	}
	if got.EndpointMethod != "cache" {
		t.Errorf("EndpointMethod = %q, want cache", got.EndpointMethod)
	}
}

// A guessed endpoint is not saved as an endpoint: restored it would come back
// as a cache advertisement and be announced to others (#839).
func TestSavePeerCacheDropsGuessedEndpoint(t *testing.T) {
	ps := NewPeerStore()
	now := time.Now()
	ps.SetPeerDirectly("guessed", &PeerInfo{WGPubKey: "guessed", MeshIP: "10.0.0.2", Endpoint: "203.0.113.1:51820", EndpointGuessed: true, LastSeen: now})
	ps.SetPeerDirectly("advertised", &PeerInfo{WGPubKey: "advertised", MeshIP: "10.0.0.3", Endpoint: "198.51.100.7:51821", LastSeen: now})

	restored := NewPeerStore()
	if n := restorePeers(buildPeerCache(ps), restored); n != 2 {
		t.Fatalf("restorePeers = %d, want 2", n)
	}
	guessed, ok := restored.Get("guessed")
	if !ok {
		t.Fatal("expected the guessed peer kept in the cache, key and mesh IP")
	}
	if guessed.Endpoint != "" || guessed.MeshIP != "10.0.0.2" {
		t.Errorf("restored guessed peer endpoint %q mesh IP %q, want empty endpoint and 10.0.0.2", guessed.Endpoint, guessed.MeshIP)
	}
	if adv, _ := restored.Get("advertised"); adv == nil || adv.Endpoint != "198.51.100.7:51821" {
		t.Errorf("restored advertised peer = %+v, want endpoint 198.51.100.7:51821", adv)
	}
}
