package daemon

import (
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	PeerDeadTimeout   = 5 * time.Minute  // Consider peer dead after no updates
	PeerRemoveTimeout = 10 * time.Minute // Remove peer from WG config after grace period
	LANMethod         = "lan"
	RendezvousMethod  = "dht-rendezvous"
	PeerEventBufSize  = 16

	// DefaultMaxPeers is the maximum number of peers the store will hold.
	// New peer insertions are rejected once this limit is reached.
	// Existing peer updates are always allowed through.
	// A legitimate mesh is unlikely to have more than 1000 nodes; the cap
	// exists to bound memory use under flood attacks.
	DefaultMaxPeers = 1000
)

type PeerEventKind int

const (
	PeerEventNew     PeerEventKind = iota
	PeerEventUpdated PeerEventKind = iota
)

type PeerEvent struct {
	PubKey string
	Kind   PeerEventKind
}

// PeerInfo represents a discovered mesh peer
type PeerInfo struct {
	WGPubKey         string
	Hostname         string
	MeshIP           string
	MeshIPv6         string
	Endpoint         string // best known endpoint (ip:port)
	Introducer       bool
	RoutableNetworks []string
	LastSeen         time.Time
	DiscoveredVia    []string       // ["lan", "dht", "gossip"]
	Latency          *time.Duration // measured via WG handshake
	NATType          string         // "cone", "symmetric", or "unknown"
	endpointMethod   string
}

// PeerStore is a thread-safe store for discovered peers
type PeerStore struct {
	mu          sync.RWMutex
	peers       map[string]*PeerInfo // keyed by WG pubkey
	subscribers []chan PeerEvent
}

// NewPeerStore creates a new peer store
func NewPeerStore() *PeerStore {
	return &PeerStore{
		peers: make(map[string]*PeerInfo),
	}
}

func (ps *PeerStore) Subscribe() <-chan PeerEvent {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ch := make(chan PeerEvent, PeerEventBufSize)
	ps.subscribers = append(ps.subscribers, ch)
	return ch
}

func (ps *PeerStore) Unsubscribe(ch <-chan PeerEvent) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	for i, sub := range ps.subscribers {
		// Compare the receive-only channel with the bidirectional channel
		// by checking if they point to the same underlying channel value
		if sub == ch {
			ps.subscribers = append(ps.subscribers[:i], ps.subscribers[i+1:]...)
			close(sub)
			return
		}
	}
}

func (ps *PeerStore) notify(pubKey string, kind PeerEventKind) {
	// Take a snapshot of subscribers under the lock to avoid races,
	// then send outside the lock to prevent deadlock (D7).
	ps.mu.RLock()
	subs := make([]chan PeerEvent, len(ps.subscribers))
	copy(subs, ps.subscribers)
	ps.mu.RUnlock()

	ev := PeerEvent{PubKey: pubKey, Kind: kind}
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Update adds or updates a peer in the store
// Merge logic: newest timestamp wins for mutable fields (endpoint, routable_networks)
func (ps *PeerStore) Update(info *PeerInfo, discoveryMethod string) {
	var eventKey string
	var eventKind PeerEventKind

	func() {
		ps.mu.Lock()
		defer ps.mu.Unlock()
		now := time.Now()

		existing, exists := ps.peers[info.WGPubKey]
		if !exists {
			// Reject new peers when the store is at capacity.
			if len(ps.peers) >= DefaultMaxPeers {
				log.Printf("[PeerStore] peer cap reached (%d); dropping new peer %s... via %s",
					DefaultMaxPeers, shortKey(info.WGPubKey), discoveryMethod)
				return
			}
			// New peer
			if info.LastSeen.IsZero() {
				info.LastSeen = now
			}
			info.DiscoveredVia = []string{discoveryMethod}
			if info.Endpoint != "" {
				info.endpointMethod = discoveryMethod
			}
			ps.peers[info.WGPubKey] = info
			eventKey = info.WGPubKey
			eventKind = PeerEventNew
			return
		}

		// Update existing peer - newer info wins
		if info.Endpoint != "" && shouldUpdateEndpoint(existing, info.Endpoint, discoveryMethod) {
			existing.Endpoint = info.Endpoint
			existing.endpointMethod = discoveryMethod
		}
		if len(info.RoutableNetworks) > 0 {
			existing.RoutableNetworks = info.RoutableNetworks
		}
		if info.MeshIP != "" {
			existing.MeshIP = info.MeshIP
		}
		if info.MeshIPv6 != "" {
			existing.MeshIPv6 = info.MeshIPv6
		}
		if info.Hostname != "" {
			existing.Hostname = info.Hostname
		}
		// Always update Introducer flag from the latest announcement.
		// A node can stop being an introducer if reconfigured.
		existing.Introducer = info.Introducer
		if info.NATType != "" {
			existing.NATType = info.NATType
		}

		if shouldRefreshLastSeen(discoveryMethod) {
			existing.LastSeen = now
		} else if !info.LastSeen.IsZero() && info.LastSeen.After(existing.LastSeen) {
			existing.LastSeen = info.LastSeen
		}

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

		eventKey = info.WGPubKey
		eventKind = PeerEventUpdated
	}()

	// Notify outside the lock to prevent deadlock if subscribers call back (D7)
	if eventKey != "" {
		ps.notify(eventKey, eventKind)
	}
}

func shouldRefreshLastSeen(discoveryMethod string) bool {
	if discoveryMethod == "cache" {
		return false
	}
	if isTransitiveMethod(discoveryMethod) {
		return false
	}
	return true
}

func isTransitiveMethod(discoveryMethod string) bool {
	return strings.Contains(discoveryMethod, "transitive")
}

func shouldUpdateEndpoint(existing *PeerInfo, newEndpoint, discoveryMethod string) bool {
	if existing.Endpoint == "" {
		return true
	}

	newRank := endpointMethodRank(discoveryMethod)
	oldRank := endpointMethodRank(existing.endpointMethod)

	if newRank > oldRank {
		return true
	}
	if newRank < oldRank {
		return false
	}

	// Equal-rank preference: keep IPv6 when available to prioritize direct IPv6 paths.
	newIsV6 := isIPv6EndpointValue(newEndpoint)
	oldIsV6 := isIPv6EndpointValue(existing.Endpoint)
	if newIsV6 && !oldIsV6 {
		return true
	}
	if oldIsV6 && !newIsV6 {
		return false
	}

	// Equal rank: allow refresh from same family, but still protect LAN endpoint.
	if oldRank == endpointMethodRank(LANMethod) {
		return discoveryMethod == LANMethod
	}
	return true
}

func isIPv6EndpointValue(endpoint string) bool {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.To4() == nil
}

func endpointMethodRank(method string) int {
	if method == "" {
		return 0
	}
	if method == LANMethod {
		return 100
	}
	if strings.Contains(method, RendezvousMethod) {
		return 90
	}
	if method == "dht" {
		return 70
	}
	if strings.Contains(method, "dht-transitive") {
		return 40
	}
	if strings.HasPrefix(method, "gossip") {
		if strings.Contains(method, "transitive") {
			return 35
		}
		return 65
	}
	if method == "cache" {
		return 20
	}
	return 30
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
