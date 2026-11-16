package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/node"
)

const (
	CacheSaveInterval = 5 * time.Minute
	CacheExpiration   = 24 * time.Hour
)

// PeerCacheEntry represents a cached peer entry
type PeerCacheEntry struct {
	WGPubKey         string   `json:"wg_pubkey"`
	Hostname         string   `json:"hostname,omitempty"`
	MeshIP           string   `json:"mesh_ip"`
	MeshIPv6         string   `json:"mesh_ipv6,omitempty"`
	Endpoint         string   `json:"endpoint"`
	Introducer       bool     `json:"introducer,omitempty"`
	RoutableNetworks []string `json:"routable_networks,omitempty"`
	NATType          string   `json:"nat_type,omitempty"`
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

// buildPeerCache is the content SavePeerCache writes.
func buildPeerCache(peerStore *PeerStore) *PeerCache {
	peers := peerStore.GetAll()
	now := time.Now()
	cache := &PeerCache{
		UpdatedAt: now.Unix(),
	}

	for _, p := range peers {
		// A guessed endpoint is not persisted: restored, it would rank as a
		// cache advertisement (#839). The peer itself is kept.
		endpoint := p.Endpoint
		if p.EndpointGuessed {
			endpoint = ""
		}
		// last_seen is the newest measured contact (LastContact), so a peer kept
		// connected by handshakes is saved with a fresh time while a restored
		// peer never heard from keeps its cache time and expires on schedule;
		// the field and its type are unchanged, an older version reads it as before.
		// A time in the future (the clock jumped back) is saved as now.
		contact := p.LastContact()
		if contact.After(now) {
			contact = now
		}
		cache.Peers = append(cache.Peers, PeerCacheEntry{
			WGPubKey:         p.WGPubKey,
			Hostname:         p.Hostname,
			MeshIP:           p.MeshIP,
			MeshIPv6:         p.MeshIPv6,
			Endpoint:         endpoint,
			Introducer:       p.Introducer,
			RoutableNetworks: p.RoutableNetworks,
			NATType:          p.NATType,
			LastSeen:         contact.Unix(),
		})
	}
	return cache
}

// SavePeerCache saves the peer cache to disk
func SavePeerCache(interfaceName string, peerStore *PeerStore) error {
	cache := buildPeerCache(peerStore)

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

	return restorePeers(cache, peerStore)
}

// restorePeers puts the entries of a loaded cache that have not expired into
// the peer store and returns how many it restored.
func restorePeers(cache *PeerCache, peerStore *PeerStore) int {
	now := time.Now()
	restored := 0
	var keys []string

	for _, entry := range cache.Peers {
		lastSeen := time.Unix(entry.LastSeen, 0)

		// Skip expired entries
		if now.Sub(lastSeen) > CacheExpiration {
			continue
		}
		// A time in the future (the clock jumped back) counts as now.
		if lastSeen.After(now) {
			lastSeen = now
		}

		peer := &PeerInfo{
			WGPubKey:         entry.WGPubKey,
			Hostname:         entry.Hostname,
			MeshIP:           entry.MeshIP,
			MeshIPv6:         entry.MeshIPv6,
			Endpoint:         entry.Endpoint,
			Introducer:       entry.Introducer,
			RoutableNetworks: entry.RoutableNetworks,
			NATType:          entry.NATType,
			LastSeen:         lastSeen,
		}

		// A refused key (tombstoned, or the peer cap) is not restored: it is
		// neither counted nor logged.
		if d := peerStore.Offer(Candidate{Info: peer, Source: "cache", Hand: Restored}); d.Verdict == node.Refuse {
			continue
		}
		restored++
		keys = append(keys, entry.WGPubKey)
	}

	if restored > 0 {
		alive, suspected := 0, 0
		for _, key := range keys {
			p, ok := peerStore.Get(key)
			if !ok {
				continue
			}
			switch p.Alive(now) {
			case node.Alive:
				alive++
			case node.Suspect:
				suspected++
			}
		}
		log.Printf("[Cache] Restored %d peers from cache (%d alive, %d only suspected)", restored, alive, suspected)
	}

	return restored
}

// StartCacheSaver starts a background goroutine that periodically saves the
// peer cache.  It stops when ctx is cancelled, performing a final save before
// returning.
func StartCacheSaver(ctx context.Context, interfaceName string, peerStore *PeerStore) {
	ticker := time.NewTicker(CacheSaveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
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
