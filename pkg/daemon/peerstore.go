package daemon

import (
	"sync"
	"time"
)

const (
	PeerDeadTimeout   = 5 * time.Minute  // Consider peer dead after no updates
	PeerRemoveTimeout = 10 * time.Minute // Remove peer from WG config after grace period
	LANMethod         = "lan"
)

// PeerInfo represents a discovered mesh peer
type PeerInfo struct {
	WGPubKey         string
	MeshIP           string
	Endpoint         string // best known endpoint (ip:port)
	RoutableNetworks []string
	LastSeen         time.Time
	DiscoveredVia    []string       // ["lan", "dht", "gossip"]
	Latency          *time.Duration // measured via WG handshake
}

// PeerStore is a thread-safe store for discovered peers
type PeerStore struct {
	mu    sync.RWMutex
	peers map[string]*PeerInfo // keyed by WG pubkey
}

// NewPeerStore creates a new peer store
func NewPeerStore() *PeerStore {
	return &PeerStore{
		peers: make(map[string]*PeerInfo),
	}
}

// Update adds or updates a peer in the store
// Merge logic: newest timestamp wins for mutable fields (endpoint, routable_networks)
func (ps *PeerStore) Update(info *PeerInfo, discoveryMethod string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	existing, exists := ps.peers[info.WGPubKey]
	if !exists {
		// New peer
		info.LastSeen = time.Now()
		info.DiscoveredVia = []string{discoveryMethod}
		ps.peers[info.WGPubKey] = info
		return
	}

	// Update existing peer - newer info wins
	if info.Endpoint != "" && shouldUpdateEndpoint(existing, discoveryMethod) {
		existing.Endpoint = info.Endpoint
	}
	if len(info.RoutableNetworks) > 0 {
		existing.RoutableNetworks = info.RoutableNetworks
	}
	if info.MeshIP != "" {
		existing.MeshIP = info.MeshIP
	}

	existing.LastSeen = time.Now()

	// Add discovery method if not already present
	found := false
	for _, method := range existing.DiscoveredVia {
		if method == discoveryMethod {
			found = true
			break
		}
	}
	if !found {
		existing.DiscoveredVia = append(existing.DiscoveredVia, discoveryMethod)
	}
}

func shouldUpdateEndpoint(existing *PeerInfo, discoveryMethod string) bool {
	if existing.Endpoint == "" {
		return true
	}
	if discoveryMethod == LANMethod {
		return true
	}
	return !containsMethod(existing.DiscoveredVia, LANMethod)
}

func containsMethod(methods []string, target string) bool {
	for _, method := range methods {
		if method == target {
			return true
		}
	}
	return false
}

// Get returns a peer by public key
func (ps *PeerStore) Get(pubKey string) (*PeerInfo, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	peer, exists := ps.peers[pubKey]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent race conditions
	peerCopy := *peer
	return &peerCopy, true
}

// GetAll returns all peers
func (ps *PeerStore) GetAll() []*PeerInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	result := make([]*PeerInfo, 0, len(ps.peers))
	for _, peer := range ps.peers {
		peerCopy := *peer
		result = append(result, &peerCopy)
	}
	return result
}

// GetActive returns all peers that have been seen recently (not dead)
func (ps *PeerStore) GetActive() []*PeerInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	result := make([]*PeerInfo, 0, len(ps.peers))
	now := time.Now()
	for _, peer := range ps.peers {
		if now.Sub(peer.LastSeen) < PeerDeadTimeout {
			peerCopy := *peer
			result = append(result, &peerCopy)
		}
	}
	return result
}

// Remove removes a peer by public key
func (ps *PeerStore) Remove(pubKey string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.peers, pubKey)
}

// CleanupStale removes peers that haven't been seen for too long
func (ps *PeerStore) CleanupStale() []string {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	var removed []string
	now := time.Now()
	for pubKey, peer := range ps.peers {
		if now.Sub(peer.LastSeen) > PeerRemoveTimeout {
			delete(ps.peers, pubKey)
			removed = append(removed, pubKey)
		}
	}
	return removed
}

// Count returns the number of peers
func (ps *PeerStore) Count() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.peers)
}

// IsDead checks if a peer is considered dead
func (ps *PeerStore) IsDead(pubKey string) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	peer, exists := ps.peers[pubKey]
	if !exists {
		return true
	}
	return time.Since(peer.LastSeen) > PeerDeadTimeout
}
