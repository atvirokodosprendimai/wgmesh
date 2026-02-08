package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	CacheSaveInterval = 5 * time.Minute
	CacheExpiration   = 24 * time.Hour
)

// PeerCacheEntry represents a cached peer entry
type PeerCacheEntry struct {
	WGPubKey         string   `json:"wg_pubkey"`
	MeshIP           string   `json:"mesh_ip"`
	Endpoint         string   `json:"endpoint"`
	RoutableNetworks []string `json:"routable_networks,omitempty"`
	LastSeen         int64    `json:"last_seen"`
}

// PeerCache manages persistent peer storage
type PeerCache struct {
	Peers     []PeerCacheEntry `json:"peers"`
	UpdatedAt int64            `json:"updated_at"`
}

// CacheFilePath returns the path for the peer cache file
func CacheFilePath(interfaceName string) string {
	return filepath.Join("/var/lib/wgmesh", fmt.Sprintf("%s-peers.json", interfaceName))
}

// LoadPeerCache loads the peer cache from disk
func LoadPeerCache(interfaceName string) (*PeerCache, error) {
	path := CacheFilePath(interfaceName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cache PeerCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to parse peer cache: %w", err)
	}

	return &cache, nil
}

// SavePeerCache saves the peer cache to disk
func SavePeerCache(interfaceName string, peerStore *PeerStore) error {
	peers := peerStore.GetAll()
	cache := &PeerCache{
		UpdatedAt: time.Now().Unix(),
	}

	for _, p := range peers {
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
		return fmt.Errorf("failed to marshal peer cache: %w", err)
	}

	path := CacheFilePath(interfaceName)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}

// RestoreFromCache restores peers from the cache into the peer store
func RestoreFromCache(interfaceName string, peerStore *PeerStore) int {
	cache, err := LoadPeerCache(interfaceName)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[Cache] Failed to load peer cache: %v", err)
		}
		return 0
	}

	now := time.Now()
	restored := 0

	for _, entry := range cache.Peers {
		lastSeen := time.Unix(entry.LastSeen, 0)

		// Skip expired entries
		if now.Sub(lastSeen) > CacheExpiration {
			continue
		}

		peer := &PeerInfo{
			WGPubKey:         entry.WGPubKey,
			MeshIP:           entry.MeshIP,
			Endpoint:         entry.Endpoint,
			RoutableNetworks: entry.RoutableNetworks,
			LastSeen:         lastSeen,
		}

		peerStore.Update(peer, "cache")
		restored++
	}

	if restored > 0 {
		log.Printf("[Cache] Restored %d peers from cache", restored)
	}

	return restored
}

// StartCacheSaver starts a background goroutine that periodically saves the peer cache
func StartCacheSaver(interfaceName string, peerStore *PeerStore, stopCh <-chan struct{}) {
	ticker := time.NewTicker(CacheSaveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			// Final save on shutdown
			if err := SavePeerCache(interfaceName, peerStore); err != nil {
				log.Printf("[Cache] Failed to save peer cache on shutdown: %v", err)
			}
			return
		case <-ticker.C:
			if err := SavePeerCache(interfaceName, peerStore); err != nil {
				log.Printf("[Cache] Failed to save peer cache: %v", err)
			}
		}
	}
}
