package node

import (
	"net"
	"strings"
	"sync"
	"time"
)

const (
	PeerDeadTimeout   = 5 * time.Minute
	PeerRemoveTimeout = 10 * time.Minute
	TombstoneTTL      = 5 * time.Minute
	PeerEventBufSize  = 16
	DefaultMaxPeers   = 1000

	LANMethod        = "lan"
	RendezvousMethod = "dht-rendezvous"
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

// tombstone records when and why a key was withdrawn.
type tombstone struct {
	at     time.Time
	cause  string
	logged bool
}

// PeerStore is a thread-safe store for discovered peers.
type PeerStore struct {
	mu          sync.RWMutex
	peers       map[string]*PeerInfo
	subscribers []chan PeerEvent
	policy      *Policy
	offers      uint64
	tombstones  map[string]tombstone
	now         func() time.Time
}

// NewPeerStore creates a new peer store.
func NewPeerStore() *PeerStore {
	return &PeerStore{
		peers:      make(map[string]*PeerInfo),
		tombstones: make(map[string]tombstone),
		now:        time.Now,
	}
}

func (ps *PeerStore) Subscribe() <-chan PeerEvent {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ch := make(chan PeerEvent, PeerEventBufSize)
	ps.subscribers = append(ps.subscribers, ch)
	return ch
}

// Unsubscribe removes ch. The channel is not closed: notify sends after
// releasing the lock, so a close could race with a send and panic. The caller
// must stop reading it; a range over it never ends after Unsubscribe.
func (ps *PeerStore) Unsubscribe(ch <-chan PeerEvent) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	for i, sub := range ps.subscribers {
		if sub == ch {
			ps.subscribers = append(ps.subscribers[:i], ps.subscribers[i+1:]...)
			return
		}
	}
}

func (ps *PeerStore) notify(pubKey string, kind PeerEventKind) {
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

// Update adds or updates a peer in the store. It is for tests and fixtures
// only and reads the hand from the method string; production code calls
// Offer with an explicit Source and Hand.
func (ps *PeerStore) Update(info *PeerInfo, discoveryMethod string) {
	ps.Offer(legacyCandidate(info, discoveryMethod))
}

// merge applies an admitted candidate to the store; the caller holds ps.mu.
// Merge logic: newest timestamp wins for mutable fields (endpoint, routable_networks)
func (ps *PeerStore) merge(c Candidate) (eventKey string, eventKind PeerEventKind) {
	info, discoveryMethod := c.Info, c.Method()
	now := time.Now()

	existing, exists := ps.peers[info.WGPubKey]
	if !exists {
		if info.LastSeen.IsZero() {
			info.LastSeen = now
		}
		info.DiscoveredVia = []string{discoveryMethod}
		if info.Endpoint != "" {
			info.EndpointMethod = discoveryMethod
		}
		info.HeardFirstHand = c.Hand == FirstHand
		ps.peers[info.WGPubKey] = info
		return info.WGPubKey, PeerEventNew
	}

	if info.Endpoint != "" && shouldUpdateEndpoint(existing, info.Endpoint, discoveryMethod) {
		existing.Endpoint = info.Endpoint
		existing.EndpointMethod = discoveryMethod
	}
	// A transitive mention is second-hand (another member says it knows this
	// peer), so it may fill a field the peer never announced itself but must
	// not rewrite one it did: otherwise any member can move another member's
	// mesh IP or routes, or flip its introducer flag, by naming it in
	// KnownPeers. Only the peer's own announcements replace these fields.
	transitive := c.Hand == SecondHand
	// An empty list from the peer's own announcement withdraws its routes;
	// a cache entry or a transitive mention with no routes says nothing.
	withdraw := len(info.RoutableNetworks) == 0 && c.Hand == FirstHand
	if withdraw || (len(info.RoutableNetworks) > 0 && (!transitive || len(existing.RoutableNetworks) == 0)) {
		existing.RoutableNetworks = info.RoutableNetworks
	}
	if info.MeshIP != "" && (!transitive || existing.MeshIP == "") {
		existing.MeshIP = info.MeshIP
	}
	if info.MeshIPv6 != "" && (!transitive || existing.MeshIPv6 == "") {
		existing.MeshIPv6 = info.MeshIPv6
	}
	if info.Hostname != "" {
		existing.Hostname = info.Hostname
	}
	if !transitive {
		existing.Introducer = info.Introducer
	}
	if info.NATType != "" {
		existing.NATType = info.NATType
	}

	if c.Hand == FirstHand {
		existing.LastSeen = now
		existing.HeardFirstHand = true
	} else if !info.LastSeen.IsZero() && info.LastSeen.After(existing.LastSeen) {
		existing.LastSeen = info.LastSeen
	}

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

	return info.WGPubKey, PeerEventUpdated
}

func isTransitiveMethod(discoveryMethod string) bool {
	return strings.Contains(discoveryMethod, "transitive")
}

// endpointMethodRank returns a priority rank for endpoint discovery methods.
// Higher rank means higher priority.
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

func shouldUpdateEndpoint(existing *PeerInfo, newEndpoint, discoveryMethod string) bool {
	if existing.Endpoint == "" {
		return true
	}

	newRank := endpointMethodRank(discoveryMethod)
	oldRank := endpointMethodRank(existing.EndpointMethod)

	if newRank > oldRank {
		return true
	}
	if newRank < oldRank {
		return false
	}

	newIsV6 := isIPv6EndpointValue(newEndpoint)
	oldIsV6 := isIPv6EndpointValue(existing.Endpoint)
	if newIsV6 && !oldIsV6 {
		return true
	}
	if oldIsV6 && !newIsV6 {
		return false
	}

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

// Get returns a peer by public key.
func (ps *PeerStore) Get(pubKey string) (*PeerInfo, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	peer, exists := ps.peers[pubKey]
	if !exists {
		return nil, false
	}

	peerCopy := *peer
	return &peerCopy, true
}

// GetAll returns all peers.
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

// List returns all peers.
func (ps *PeerStore) List() []*PeerInfo {
	return ps.GetAll()
}

// GetActive returns all peers that have been seen recently (not dead).
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

// Remove removes a peer by public key.
func (ps *PeerStore) Remove(pubKey string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.peers, pubKey)
}

// Withdraw removes a peer and, for TombstoneTTL, refuses second-hand and
// restored claims for its key; the peer's own first-hand claim clears that.
// A key that is not in the store leaves no tombstone. cause says why (goodbye,
// evicted, stale) and is kept for the refusal log.
func (ps *PeerStore) Withdraw(pubKey, cause string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if _, ok := ps.peers[pubKey]; !ok {
		return
	}
	delete(ps.peers, pubKey)
	ps.tombstone(pubKey, cause)
}

// tombstone records a withdrawn key; the caller holds ps.mu. The set never
// holds more than DefaultMaxPeers keys: at the cap it drops the expired
// entries, then the oldest.
func (ps *PeerStore) tombstone(pubKey, cause string) {
	now := ps.now()
	if _, ok := ps.tombstones[pubKey]; !ok && len(ps.tombstones) >= DefaultMaxPeers {
		var oldest string
		for key, t := range ps.tombstones {
			if now.Sub(t.at) >= TombstoneTTL {
				delete(ps.tombstones, key)
			} else if oldest == "" || t.at.Before(ps.tombstones[oldest].at) {
				oldest = key
			}
		}
		if len(ps.tombstones) >= DefaultMaxPeers {
			delete(ps.tombstones, oldest)
		}
	}
	ps.tombstones[pubKey] = tombstone{at: now, cause: cause}
}

// isTombstoned reports whether pubKey was withdrawn less than TombstoneTTL
// ago and drops an expired entry; the caller holds ps.mu.
func (ps *PeerStore) isTombstoned(pubKey string) bool {
	t, ok := ps.tombstones[pubKey]
	if ok && ps.now().Sub(t.at) >= TombstoneTTL {
		delete(ps.tombstones, pubKey)
		return false
	}
	return ok
}

// CleanupStale removes peers that haven't been seen for too long.
func (ps *PeerStore) CleanupStale() []string {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	var removed []string
	now := time.Now()
	for pubKey, peer := range ps.peers {
		if now.Sub(peer.LastSeen) > PeerRemoveTimeout {
			delete(ps.peers, pubKey)
			if peer.HeardFirstHand {
				ps.tombstone(pubKey, "stale")
			}
			removed = append(removed, pubKey)
		}
	}
	return removed
}

// Count returns the number of peers.
func (ps *PeerStore) Count() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.peers)
}

// SetLatency updates the measured round-trip latency for the given peer.
func (ps *PeerStore) SetLatency(pubKey string, rtt time.Duration) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	peer, exists := ps.peers[pubKey]
	if !exists {
		return
	}
	peer.Latency = &rtt
}

// IsDead checks if a peer is considered dead.
func (ps *PeerStore) IsDead(pubKey string) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	peer, exists := ps.peers[pubKey]
	if !exists {
		return true
	}
	return time.Since(peer.LastSeen) > PeerDeadTimeout
}

// shortKey safely truncates a key for logging.
func shortKey(key string) string {
	if len(key) > 16 {
		return key[:16]
	}
	return key
}

// SetEndpointMethod updates the endpoint method for a peer.
func (ps *PeerStore) SetEndpointMethod(pubKey, method string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	peer, exists := ps.peers[pubKey]
	if !exists {
		return
	}
	peer.EndpointMethod = method
}

func (ps *PeerStore) SetPeerDirectly(key string, info *PeerInfo) {
	ps.mu.Lock()
	ps.peers[key] = info
	ps.mu.Unlock()
}
