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
		WGPubKey: "pubkey2",
		MeshIP:   "10.0.0.2",
		Endpoint: "5.6.7.8:51820",
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
	ps.mu.Lock()
	ps.peers["old-key"] = &PeerInfo{
		WGPubKey: "old-key",
		MeshIP:   "10.0.0.1",
		LastSeen: time.Now().Add(-25 * time.Hour), // 25 hours ago, expired
	}
	ps.peers["new-key"] = &PeerInfo{
		WGPubKey: "new-key",
		MeshIP:   "10.0.0.2",
		LastSeen: time.Now(),
	}
	ps.mu.Unlock()

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
