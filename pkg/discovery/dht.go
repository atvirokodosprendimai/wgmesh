package discovery

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/dht/v2"
	"github.com/anacrolix/dht/v2/krpc"
	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

const (
	DHTAnnounceInterval       = 15 * time.Minute
	DHTQueryInterval          = 30 * time.Second
	DHTQueryIntervalStable    = 60 * time.Second
	DHTTransitiveInterval     = 1 * time.Second // Legacy: used only for initial backfill
	DHTBootstrapTimeout       = 30 * time.Second
	DHTPersistInterval        = 2 * time.Minute
	DHTMethod                 = "dht"
	DHTMaxConcurrentExchanges = 10 // Limit concurrent transitive exchanges to prevent resource exhaustion
	RendezvousWindow          = 20 * time.Second
	RendezvousPhase           = 4 * time.Second
	RendezvousPunchDelay      = 500 * time.Millisecond
	RendezvousMaxIntroducers  = 3
	RendezvousMinBackoff      = 3 * time.Second
	RendezvousMaxBackoff      = 30 * time.Second
	RendezvousStaleCheck      = 10 * time.Second // How often to check for stale handshakes
)

type backoffEntry struct {
	nextAttempt time.Time
	duration    time.Duration
}

// Well-known BitTorrent DHT bootstrap nodes
var DHTBootstrapNodes = []string{
	"router.bittorrent.com:6881",
	"router.utorrent.com:6881",
	"dht.transmissionbt.com:6881",
	"dht.libtorrent.org:25401",
}

// DHTDiscovery handles peer discovery via BitTorrent Mainline DHT
type DHTDiscovery struct {
	config    *daemon.Config
	localNode *LocalNode
	peerStore *daemon.PeerStore
	exchange  *PeerExchange
	gossip    *MeshGossip
	lan       *LANDiscovery
	server    *dht.Server
	dhtPort   int

	mu                sync.RWMutex
	running           bool
	ctx               context.Context
	cancel            context.CancelFunc
	contactedPeers    map[string]time.Time    // Dedup: don't spam same IP
	controlPeers      map[string]string       // peer pubkey -> exchange/control endpoint
	rendezvousBackoff map[string]backoffEntry // peer pubkey -> backoff state
}

// LocalNode represents our local node information
type LocalNode struct {
	WGPubKey         string
	Hostname         string
	WGPrivateKey     string
	MeshIP           string
	MeshIPv6         string
	RoutableNetworks []string
	Introducer       bool
	NATType          NATType // Detected NAT behavior (cone/symmetric/unknown)

	endpointMu sync.RWMutex
	wgEndpoint string
}

// GetEndpoint returns the current WG endpoint (thread-safe).
func (n *LocalNode) GetEndpoint() string {
	n.endpointMu.RLock()
	defer n.endpointMu.RUnlock()
	return n.wgEndpoint
}

// SetEndpoint updates the WG endpoint (thread-safe).
func (n *LocalNode) SetEndpoint(ep string) {
	n.endpointMu.Lock()
	defer n.endpointMu.Unlock()
	n.wgEndpoint = ep
}

// NewDHTDiscovery creates a new DHT discovery instance
func NewDHTDiscovery(config *daemon.Config, localNode *LocalNode, peerStore *daemon.PeerStore) (*DHTDiscovery, error) {
	ctx, cancel := context.WithCancel(context.Background())

	d := &DHTDiscovery{
		config:            config,
		localNode:         localNode,
		peerStore:         peerStore,
		ctx:               ctx,
		cancel:            cancel,
		contactedPeers:    make(map[string]time.Time),
		controlPeers:      make(map[string]string),
		rendezvousBackoff: make(map[string]backoffEntry),
	}

	// Create peer exchange handler
	d.exchange = NewPeerExchange(config, localNode, peerStore)

	return d, nil
}

// Start begins DHT discovery
func (d *DHTDiscovery) Start() error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return fmt.Errorf("DHT discovery already running")
	}
	d.running = true
	d.mu.Unlock()

	// Discover external endpoint via STUN before announcing to peers.
	// Uses an ephemeral port (WG owns the listen port) — the external IP
	// is the same regardless of source port on most NATs. We combine the
	// STUN-discovered IP with the WG listen port.
	d.discoverExternalEndpoint()

	// Create in-mesh gossip and wire announce handler BEFORE starting exchange
	// to avoid a race between the exchange listener goroutine and handler setup.
	if d.config.Gossip {
		gossip, err := NewMeshGossipWithExchange(d.config, d.localNode, d.peerStore, d.exchange)
		if err != nil {
			return fmt.Errorf("failed to create gossip: %w", err)
		}
		d.gossip = gossip
		d.exchange.SetAnnounceHandler(d.gossip.HandleAnnounceFrom)
	}

	// Start the peer exchange server (listens for incoming connections)
	if err := d.exchange.Start(); err != nil {
		return fmt.Errorf("failed to start peer exchange: %w", err)
	}

	if d.config.LANDiscovery {
		lan, err := NewLANDiscovery(d.config, d.localNode, d.peerStore)
		if err != nil {
			log.Printf("[LAN] Failed to initialize LAN discovery: %v", err)
		} else {
			d.lan = lan
			if err := d.lan.Start(); err != nil {
				log.Printf("[LAN] Failed to start LAN discovery: %v", err)
				d.lan = nil
			}
		}
	} else {
		log.Printf("[LAN] LAN discovery disabled by configuration")
	}

	// Start gossip loop after exchange is listening
	if d.gossip != nil {
		if err := d.gossip.Start(); err != nil {
			d.exchange.Stop()
			return fmt.Errorf("failed to start gossip: %w", err)
		}
	}

	// Initialize DHT server
	if err := d.initDHTServer(); err != nil {
		if d.gossip != nil {
			d.gossip.Stop()
		}
		d.exchange.Stop()
		return fmt.Errorf("failed to initialize DHT server: %w", err)
	}

	// Start background goroutines
	go d.announceLoop()
	go d.queryLoop()
	go d.persistLoop()
	if d.config.DisablePunching {
		log.Printf("[NAT] NAT punching disabled by configuration")
	} else {
		go d.transitiveConnectLoop()
	}
	go d.stunRefreshLoop()

	log.Printf("[DHT] Discovery started, listening on port %d", d.exchange.Port())
	return nil
}

// Stop stops DHT discovery
func (d *DHTDiscovery) Stop() error {
	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return nil
	}
	d.running = false
	d.mu.Unlock()

	d.broadcastGoodbye()

	d.cancel()

	if d.server != nil {
		d.persistNodes()
		d.server.Close()
	}

	if d.lan != nil {
		d.lan.Stop()
	}

	if d.gossip != nil {
		d.gossip.Stop()
	}

	if d.exchange != nil {
		d.exchange.Stop()
	}

	log.Printf("[DHT] Discovery stopped")
	return nil
}

func (d *DHTDiscovery) broadcastGoodbye() {
	if d.exchange == nil {
		return
	}

	peers := d.peerStore.GetAll()
	targets := make(map[string]struct{})
	for _, p := range peers {
		if p == nil || p.WGPubKey == "" || p.WGPubKey == d.localNode.WGPubKey {
			continue
		}
		if endpoint := d.controlEndpointForPeer(p); endpoint != "" {
			targets[endpoint] = struct{}{}
			continue
		}
		if endpoint := toControlEndpoint(p.Endpoint, int(d.config.Keys.GossipPort)); endpoint != "" {
			targets[endpoint] = struct{}{}
		}
	}

	for endpoint := range targets {
		if err := d.exchange.SendGoodbye(endpoint); err != nil {
			d.debugf("[Exchange] Failed to send GOODBYE to %s: %v", endpoint, err)
		}
	}
}

// discoverExternalEndpoint queries two STUN servers to find this node's
// server-reflexive address and detect NAT type. Updates localNode.WGEndpoint
// and localNode.NATType. Falls back to the existing endpoint if STUN fails.
// Also discovers IPv6 endpoint if available (no NAT, preferred for direct connection).
func (d *DHTDiscovery) discoverExternalEndpoint() {
	if d.config.DisableIPv6 {
		log.Printf("[STUN] IPv6 discovery disabled by configuration")
	} else {
		// First try IPv6 - no NAT traversal needed
		if ipv6Endpoint := d.discoverIPv6Endpoint(); ipv6Endpoint != "" {
			log.Printf("[STUN] IPv6 endpoint discovered: %s (no NAT)", ipv6Endpoint)
			d.localNode.SetEndpoint(ipv6Endpoint)
			d.localNode.NATType = NATUnknown // IPv6 has no NAT
			return
		}
	}

	servers := DefaultSTUNServers
	if len(servers) < 2 {
		// Need at least 2 servers for NAT type detection; fall back to simple query
		ip, _, err := DiscoverExternalEndpoint(0)
		if err != nil {
			log.Printf("[STUN] Failed to discover external endpoint: %v (keeping %s)", err, d.localNode.GetEndpoint())
			return
		}
		endpoint := net.JoinHostPort(ip.String(), strconv.Itoa(d.config.WGListenPort))
		log.Printf("[STUN] External endpoint discovered: %s (NAT type unknown — need 2 servers)", endpoint)
		d.localNode.SetEndpoint(endpoint)
		d.localNode.NATType = NATUnknown
		return
	}

	natType, ip, _, err := DetectNATType(servers[0], servers[1], 0, 3000)
	if err != nil {
		log.Printf("[STUN] Failed to discover external endpoint: %v (keeping %s)", err, d.localNode.GetEndpoint())
		return
	}

	endpoint := net.JoinHostPort(ip.String(), strconv.Itoa(d.config.WGListenPort))
	log.Printf("[STUN] External endpoint: %s, NAT type: %s", endpoint, natType)
	d.localNode.SetEndpoint(endpoint)
	d.localNode.NATType = natType
}

func (d *DHTDiscovery) discoverIPv6Endpoint() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	type ipv6Candidate struct {
		ip    net.IP
		score int // higher is better
	}
	var candidates []ipv6Candidate

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP
			if ip == nil || ip.To4() != nil {
				continue
			}
			if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}
			if !ip.IsGlobalUnicast() {
				continue
			}
			if !isPublicIPv6(ip) {
				continue
			}
			score := scoreIPv6(ip)
			candidates = append(candidates, ipv6Candidate{ip: ip, score: score})
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	// Sort: highest score first, then lexicographic for deterministic tiebreak.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].ip.String() < candidates[j].ip.String()
	})

	return net.JoinHostPort(candidates[0].ip.String(), strconv.Itoa(d.config.WGListenPort))
}

// scoreIPv6 ranks an IPv6 address for endpoint selection.
// Higher scores are preferred. Stable GUA (non-temporary, non-EUI-64) is best.
func scoreIPv6(ip net.IP) int {
	if len(ip) != 16 {
		return 0
	}
	score := 10 // base score for any valid public IPv6

	// Prefer 2000::/3 global unicast addresses
	if ip[0]&0xe0 == 0x20 {
		score += 20
	}

	// Penalize EUI-64 addresses (ff:fe in bytes 11-12) — they embed MAC,
	// are less stable across hardware changes, and leak hardware identity.
	if ip[11] == 0xff && ip[12] == 0xfe {
		score -= 5
	}

	return score
}

func isPublicIPv6(ip net.IP) bool {
	if ip == nil || len(ip) != 16 {
		return false
	}
	if ip.To4() != nil {
		return false
	}
	if ip.IsUnspecified() {
		return false
	}

	var specialPrefixes = []struct {
		prefix net.IP
		bits   int
	}{
		{net.ParseIP("200::"), 7},       // 200::/7 — unassigned (Yggdrasil overlay, etc.)
		{net.ParseIP("2001:db8::"), 32}, // Documentation
		{net.ParseIP("fc00::"), 7},      // ULA
		{net.ParseIP("fd00::"), 8},      // ULA
		{net.ParseIP("fe80::"), 10},     // Link-local
		{net.ParseIP("ff00::"), 8},      // Multicast
		{net.ParseIP("::1"), 128},       // Loopback
	}

	for _, sp := range specialPrefixes {
		if sp.prefix == nil {
			continue
		}
		mask := net.CIDRMask(sp.bits, 128)
		if ip.Mask(mask).Equal(sp.prefix.Mask(mask)) {
			return false
		}
	}

	return true
}

// stunRefreshLoop periodically re-queries STUN servers to track NAT mapping
// and type changes. It runs DetectNATType (two-server probe) to keep NATType
// up-to-date, falling back to single-server DiscoverExternalEndpoint when
// fewer than two servers are available.
func (d *DHTDiscovery) stunRefreshLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if !d.config.DisableIPv6 {
				// Prefer IPv6 if available
				if ipv6Endpoint := d.discoverIPv6Endpoint(); ipv6Endpoint != "" {
					if ipv6Endpoint != d.localNode.GetEndpoint() {
						log.Printf("[STUN] IPv6 endpoint available: %s", ipv6Endpoint)
						d.localNode.SetEndpoint(ipv6Endpoint)
					}
					d.localNode.NATType = NATUnknown // IPv6 has no NAT
					continue
				}
			}

			servers := DefaultSTUNServers
			if len(servers) >= 2 {
				// Full NAT type re-detection with two servers
				natType, ip, _, err := DetectNATType(servers[0], servers[1], 0, 3000)
				if err != nil {
					log.Printf("[STUN] Refresh failed: %v", err)
					continue
				}
				newEndpoint := net.JoinHostPort(ip.String(), strconv.Itoa(d.config.WGListenPort))
				currentEP := d.localNode.GetEndpoint()
				oldNAT := d.localNode.NATType
				if newEndpoint != currentEP {
					log.Printf("[STUN] External endpoint changed: %s -> %s", currentEP, newEndpoint)
					d.localNode.SetEndpoint(newEndpoint)
				}
				if natType != oldNAT {
					log.Printf("[STUN] NAT type changed: %s -> %s", oldNAT, natType)
				}
				d.localNode.NATType = natType
			} else {
				// Fallback: single-server IP-only refresh
				ip, _, err := DiscoverExternalEndpoint(0)
				if err != nil {
					log.Printf("[STUN] Refresh failed: %v", err)
					continue
				}
				newEndpoint := net.JoinHostPort(ip.String(), strconv.Itoa(d.config.WGListenPort))
				currentEP := d.localNode.GetEndpoint()
				if newEndpoint != currentEP {
					log.Printf("[STUN] External endpoint changed: %s -> %s", currentEP, newEndpoint)
					d.localNode.SetEndpoint(newEndpoint)
				}
			}
		case <-d.ctx.Done():
			return
		}
	}
}

// initDHTServer initializes the BitTorrent DHT server
func (d *DHTDiscovery) initDHTServer() error {
	// Use a separate port for DHT (exchange port + 1)
	// This avoids conflicts with peer exchange read deadlines
	dhtPort := d.exchange.Port() + 1
	dhtAddr := &net.UDPAddr{Port: dhtPort}
	dhtConn, err := net.ListenUDP("udp", dhtAddr)
	if err != nil {
		// Try another port if this one is in use
		dhtAddr = &net.UDPAddr{Port: 0} // Let OS pick
		dhtConn, err = net.ListenUDP("udp", dhtAddr)
		if err != nil {
			return fmt.Errorf("failed to bind DHT port: %w", err)
		}
	}
	d.dhtPort = dhtConn.LocalAddr().(*net.UDPAddr).Port

	// Configure DHT server
	cfg := dht.NewDefaultServerConfig()
	cfg.Conn = dhtConn
	cfg.NoSecurity = false

	// Resolve bootstrap nodes
	var bootstrapAddrs []dht.Addr
	for _, node := range DHTBootstrapNodes {
		addr, err := net.ResolveUDPAddr("udp", node)
		if err != nil {
			log.Printf("[DHT] Failed to resolve bootstrap node %s: %v", node, err)
			continue
		}
		bootstrapAddrs = append(bootstrapAddrs, dht.NewAddr(addr))
		log.Printf("[DHT] Added bootstrap node: %s", addr.String())
	}
	if len(bootstrapAddrs) == 0 {
		dhtConn.Close()
		return fmt.Errorf("no bootstrap nodes resolved")
	}

	cfg.StartingNodes = func() ([]dht.Addr, error) {
		return bootstrapAddrs, nil
	}

	server, err := dht.NewServer(cfg)
	if err != nil {
		dhtConn.Close()
		return fmt.Errorf("failed to create DHT server: %w", err)
	}

	d.server = server
	d.loadPersistedNodes()

	log.Printf("[DHT] Bootstrapping into DHT network on port %d...", d.dhtPort)

	// Actively bootstrap by doing a lookup for a random ID
	// This forces the DHT to contact bootstrap nodes and populate routing table
	go func() {
		ctx, cancel := context.WithTimeout(d.ctx, 30*time.Second)
		defer cancel()

		// Do a self-lookup to bootstrap
		var randomID [20]byte
		copy(randomID[:], d.config.Keys.NetworkID[:])

		// Use Announce with port 0 to do a get_peers which bootstraps the routing table
		a, err := d.server.Announce(randomID, 0, false)
		if err != nil {
			log.Printf("[DHT] Bootstrap lookup failed: %v", err)
			return
		}
		defer a.Close()

		// Drain the channel to complete the bootstrap
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-a.Peers:
				if !ok {
					return
				}
			}
		}
	}()

	// Wait for some nodes to be discovered
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		nodes := server.NumNodes()
		if nodes > 0 {
			log.Printf("[DHT] Bootstrap complete, DHT has %d nodes", nodes)
			return nil
		}
		log.Printf("[DHT] Waiting for bootstrap... (%d/10)", i+1)
	}

	// Continue anyway even if bootstrap seems slow
	log.Printf("[DHT] Bootstrap timeout, continuing with %d nodes (discovery may be slow)", server.NumNodes())
	return nil
}

func (d *DHTDiscovery) persistLoop() {
	ticker := time.NewTicker(DHTPersistInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.persistNodes()
		}
	}
}

func (d *DHTDiscovery) nodesFilePath() string {
	networkTag := fmt.Sprintf("%x", d.config.Keys.NetworkID[:8])
	return filepath.Join("/var/lib/wgmesh", fmt.Sprintf("%s-%s-dht.nodes", d.config.InterfaceName, networkTag))
}

func (d *DHTDiscovery) loadPersistedNodes() {
	if d.server == nil {
		return
	}

	file := d.nodesFilePath()
	added, err := d.server.AddNodesFromFile(file)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[DHT] Failed to load persisted nodes from %s: %v", file, err)
		}
		return
	}

	if added > 0 {
		log.Printf("[DHT] Loaded %d persisted DHT nodes from %s", added, file)
	}
}

func (d *DHTDiscovery) persistNodes() {
	if d.server == nil {
		return
	}

	nodes := d.server.Nodes()
	if len(nodes) == 0 {
		return
	}

	file := d.nodesFilePath()
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		log.Printf("[DHT] Failed to create DHT state directory: %v", err)
		return
	}

	if err := dht.WriteNodesToFile(nodes, file); err != nil {
		log.Printf("[DHT] Failed to persist DHT nodes to %s: %v", file, err)
		return
	}
}

// announceLoop periodically announces our presence to the DHT
func (d *DHTDiscovery) announceLoop() {
	// Initial announce
	d.announce()

	ticker := time.NewTicker(DHTAnnounceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.announce()
		}
	}
}

// announce publishes our presence to the DHT under the network ID
func (d *DHTDiscovery) announce() {
	// Get current and previous network IDs (for hourly rotation)
	current, previous, err := crypto.GetCurrentAndPreviousNetworkIDs(d.config.Secret)
	if err != nil {
		log.Printf("[DHT] Failed to derive network IDs: %v", err)
		return
	}

	port := d.exchange.Port()

	log.Printf("[DHT] Announcing to network ID %x on exchange port %d (DHT port %d)", current[:8], port, d.dhtPort)

	// Announce to current network ID
	d.announceToInfohash(current, port)

	// Also announce to previous hour's ID during transition
	if current != previous {
		log.Printf("[DHT] Also announcing to previous network ID %x", previous[:8])
		d.announceToInfohash(previous, port)
	}
}

// announceToInfohash announces our port to a specific infohash
func (d *DHTDiscovery) announceToInfohash(infohash [20]byte, port int) {
	ctx, cancel := context.WithTimeout(d.ctx, 30*time.Second)
	defer cancel()

	announce, err := d.server.Announce(infohash, port, false)
	if err != nil {
		log.Printf("[DHT] Failed to start announce: %v", err)
		return
	}
	defer announce.Close()

	// Wait for some responses
	var responseCount int
	for {
		select {
		case <-ctx.Done():
			log.Printf("[DHT] Announced to %d nodes", responseCount)
			return
		case _, ok := <-announce.Peers:
			if !ok {
				log.Printf("[DHT] Announced to %d nodes", responseCount)
				return
			}
			responseCount++
		}
	}
}

// queryLoop periodically queries the DHT for peers
func (d *DHTDiscovery) queryLoop() {
	// Initial query
	d.queryPeers()

	// Start with faster queries, slow down once mesh is stable
	interval := DHTQueryInterval

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.queryPeers()

			// Slow down queries once we have enough peers
			if d.peerStore.Count() >= 3 && interval == DHTQueryInterval {
				interval = DHTQueryIntervalStable
				ticker.Reset(interval)
				log.Printf("[DHT] Mesh stable, slowing query interval to %v", interval)
			}
		}
	}
}

// queryPeers queries the DHT for other peers in our mesh
func (d *DHTDiscovery) queryPeers() {
	// Get current and previous network IDs
	current, previous, err := crypto.GetCurrentAndPreviousNetworkIDs(d.config.Secret)
	if err != nil {
		log.Printf("[DHT] Failed to derive network IDs: %v", err)
		return
	}

	log.Printf("[DHT] Querying network ID %x (DHT has %d nodes)", current[:8], d.server.NumNodes())

	// Query current network ID
	d.queryInfohash(current)

	// Also query previous hour's ID during transition
	if current != previous {
		d.queryInfohash(previous)
	}
}

// queryInfohash queries a specific infohash for peers
func (d *DHTDiscovery) queryInfohash(infohash [20]byte) {
	ctx, cancel := context.WithTimeout(d.ctx, 30*time.Second)
	defer cancel()

	peers, err := d.server.Announce(infohash, 0, false) // port=0, false = get_peers only, no announce
	if err != nil {
		log.Printf("[DHT] Failed to query peers: %v", err)
		return
	}
	defer peers.Close()

	var discovered int
	for {
		select {
		case <-ctx.Done():
			log.Printf("[DHT] Query complete, discovered %d peer addresses", discovered)
			return
		case peerAddrs, ok := <-peers.Peers:
			if !ok {
				log.Printf("[DHT] Query complete, discovered %d peer addresses", discovered)
				return
			}
			for _, addr := range peerAddrs.Peers {
				discovered++
				go d.contactPeer(addr)
			}
		}
	}
}

// contactPeer initiates peer exchange with a discovered address
func (d *DHTDiscovery) contactPeer(addr krpc.NodeAddr) {
	addrStr := addr.String()
	if d.config.DisableIPv6 && isIPv6Endpoint(addrStr) {
		return
	}

	// Skip if this is our own address
	if addrStr == d.localNode.GetEndpoint() {
		return
	}

	if !d.markContacted(addrStr, 60*time.Second) {
		return
	}

	log.Printf("[DHT] Contacting potential peer at %s", addrStr)

	// Attempt peer exchange
	d.exchangeWithAddress(addrStr, DHTMethod)
}

func (d *DHTDiscovery) transitiveConnectLoop() {
	// Subscribe to peer store events for immediate reaction
	peerEventCh := d.peerStore.Subscribe()
	defer d.peerStore.Unsubscribe(peerEventCh)

	// Stale handshake check ticker (replaces 1s poll with 30s check)
	staleTicker := time.NewTicker(RendezvousStaleCheck)
	defer staleTicker.Stop()

	// Initial backfill: process existing peers once at startup
	d.tryTransitivePeersWithBackoff(nil)

	for {
		select {
		case <-d.ctx.Done():
			return
		case ev, ok := <-peerEventCh:
			if !ok {
				return
			}
			// New or updated peer — check if rendezvous needed
			d.handlePeerEvent(ev)
		case <-staleTicker.C:
			// Periodic check for peers with stale handshakes
			d.checkStaleHandshakes()
		}
	}
}

func (d *DHTDiscovery) handlePeerEvent(ev daemon.PeerEvent) {
	if ev.PubKey == "" || ev.PubKey == d.localNode.WGPubKey {
		return
	}

	peer, ok := d.peerStore.Get(ev.PubKey)
	if !ok {
		return
	}

	// Skip if recently established via rendezvous
	if hasDiscoveryMethod(peer.DiscoveredVia, "dht-rendezvous") && time.Since(peer.LastSeen) < 2*time.Minute {
		return
	}

	// Check backoff
	if !d.canAttemptRendezvous(ev.PubKey) {
		return
	}

	// Trigger rendezvous for this specific peer
	d.tryRendezvousForPeer(peer)
}

func (d *DHTDiscovery) checkStaleHandshakes() {
	peers := d.peerStore.GetActive()
	for _, peer := range peers {
		if peer.WGPubKey == "" || peer.WGPubKey == d.localNode.WGPubKey {
			continue
		}
		if peer.Endpoint == "" {
			continue
		}
		if !d.canAttemptRendezvous(peer.WGPubKey) {
			continue
		}
		// Only trigger if in rendezvous window
		if !d.shouldAttemptRendezvous(peer.WGPubKey, time.Now()) {
			continue
		}

		d.tryRendezvousForPeer(peer)
	}
}

func (d *DHTDiscovery) canAttemptRendezvous(pubKey string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	entry, ok := d.rendezvousBackoff[pubKey]
	if !ok {
		return true
	}
	return time.Now().After(entry.nextAttempt)
}

func (d *DHTDiscovery) recordRendezvousAttempt(pubKey string, success bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if success {
		// Reset backoff on success
		delete(d.rendezvousBackoff, pubKey)
		return
	}

	// Exponential backoff: double the previous duration, capped at max
	existing, ok := d.rendezvousBackoff[pubKey]
	var nextBackoff time.Duration
	if !ok {
		nextBackoff = RendezvousMinBackoff
	} else {
		nextBackoff = existing.duration * 2
		if nextBackoff < RendezvousMinBackoff {
			nextBackoff = RendezvousMinBackoff
		}
		if nextBackoff > RendezvousMaxBackoff {
			nextBackoff = RendezvousMaxBackoff
		}
	}

	d.rendezvousBackoff[pubKey] = backoffEntry{
		nextAttempt: time.Now().Add(nextBackoff),
		duration:    nextBackoff,
	}
}

func (d *DHTDiscovery) tryRendezvousForPeer(peer *daemon.PeerInfo) {
	if d.config.DisablePunching {
		return
	}
	if peer.WGPubKey == "" {
		return
	}

	// Skip peers we already have a recent WG handshake with — no rendezvous needed.
	hs := getPeerHandshakeTS(d.config.InterfaceName, peer.WGPubKey)
	if hs > 0 && time.Since(time.Unix(hs, 0)) < 2*time.Minute {
		return
	}

	// Skip IPv6 endpoints only for direct punch (rendezvous doesn't need direct route)
	hasDirectRoute := peer.Endpoint != ""
	if hasDirectRoute && d.config.DisableIPv6 && isIPv6Endpoint(peer.Endpoint) {
		hasDirectRoute = false
	}
	if hasDirectRoute && strings.HasPrefix(peer.Endpoint, "[") && !d.hasIPv6Route() {
		log.Printf("[NAT] Skipping direct IPv6 endpoint for %s (no IPv6 route), trying rendezvous", shortKey(peer.WGPubKey))
		hasDirectRoute = false
	}

	introducers := d.selectRendezvousIntroducers(peer.WGPubKey, d.peerStore.GetActive(), RendezvousMaxIntroducers)

	if len(introducers) > 0 {
		sent := 0
		for _, introducer := range introducers {
			if introducer.ControlEndpoint == "" {
				continue
			}
			if !d.markContacted(introducer.ControlEndpoint, 20*time.Second) {
				continue
			}

			log.Printf("[NAT] Event-driven rendezvous: %s <-> %s via %s", shortKey(d.localNode.WGPubKey), shortKey(peer.WGPubKey), shortKey(introducer.WGPubKey))

			go func(endpoint string, target *daemon.PeerInfo) {
				err := d.exchange.RequestRendezvous(endpoint, target.WGPubKey, nil)
				if err != nil {
					log.Printf("[NAT] Rendezvous failed via %s for %s: %v", endpoint, shortKey(target.WGPubKey), err)
					d.recordRendezvousAttempt(target.WGPubKey, false)
					return
				}
				log.Printf("[NAT] Rendezvous request sent for pair %s <-> %s via %s", shortKey(d.localNode.WGPubKey), shortKey(target.WGPubKey), endpoint)
				// Don't record success here — sending the UDP packet doesn't mean
				// the rendezvous succeeded. Success is recorded when the WG handshake
				// completes via handleRendezvousStart → runRendezvousPunch.
			}(introducer.ControlEndpoint, peer)
			sent++
		}
		if sent == 0 {
			log.Printf("[NAT] Rendezvous throttled for %s (introducer busy)", shortKey(peer.WGPubKey))
		}
		return
	}

	// No introducer — try synchronized punch (only if we have a direct route)
	if !hasDirectRoute {
		return
	}

	targetControlEndpoint := d.controlEndpointForPeer(peer)
	if targetControlEndpoint == "" {
		return
	}

	if !d.markContacted(targetControlEndpoint, 20*time.Second) {
		return
	}

	log.Printf("[NAT] Event-driven punch: %s via %s (no introducer)", shortKey(peer.WGPubKey), targetControlEndpoint)

	go func(endpoint string, target *daemon.PeerInfo) {
		baseline := getPeerHandshakeTS(d.config.InterfaceName, target.WGPubKey)
		peerInfo, err := d.exchange.ExchangeWithPeer(endpoint)
		if err != nil {
			if !strings.Contains(err.Error(), "timeout") {
				log.Printf("[NAT] Punch failed for %s: %v", shortKey(target.WGPubKey), err)
			}
			d.recordRendezvousAttempt(target.WGPubKey, false)
			return
		}
		if peerInfo != nil {
			newHandshake := waitForPeerHandshake(d.config.InterfaceName, target.WGPubKey, baseline, HandshakeWaitTimeout)
			if newHandshake <= baseline {
				log.Printf("[NAT] Control path reached %s via %s but WG handshake not established", shortKey(target.WGPubKey), endpoint)
				d.recordRendezvousAttempt(target.WGPubKey, false)
				return
			}
			if isIPv6Endpoint(peerInfo.Endpoint) {
				log.Printf("[Path] Direct IPv6 established: %s (%s)", shortKey(peerInfo.WGPubKey), peerInfo.Endpoint)
			} else {
				log.Printf("[NAT] Punch succeeded: %s (%s)", shortKey(peerInfo.WGPubKey), peerInfo.Endpoint)
			}
			d.setControlEndpoint(peerInfo.WGPubKey, endpoint)
			d.peerStore.Update(peerInfo, DHTMethod+"-transitive")
			d.recordRendezvousAttempt(target.WGPubKey, true)
		}
	}(targetControlEndpoint, peer)
}

// tryTransitivePeersWithBackoff is the legacy path for initial backfill
func (d *DHTDiscovery) tryTransitivePeersWithBackoff(peers []*daemon.PeerInfo) {
	if peers == nil {
		peers = d.peerStore.GetActive()
	}

	for _, peer := range peers {
		if peer.WGPubKey == "" || peer.WGPubKey == d.localNode.WGPubKey {
			continue
		}
		if peer.Endpoint == "" {
			continue
		}
		if !d.canAttemptRendezvous(peer.WGPubKey) {
			continue
		}

		d.tryRendezvousForPeer(peer)
	}
}

func (d *DHTDiscovery) exchangeWithAddress(addrStr string, discoveryMethod string) {
	if d.config.DisableIPv6 && isIPv6Endpoint(addrStr) {
		return
	}
	if discoveryMethod == DHTMethod+"-transitive" {
		log.Printf("[NAT] Starting punch/exchange via transitive address %s", addrStr)
	}

	peerInfo, err := d.exchange.ExchangeWithPeer(addrStr)
	if err != nil {
		// Timeouts are expected for some addresses during NAT traversal.
		if !strings.Contains(err.Error(), "timeout") {
			log.Printf("[DHT] Peer exchange failed with %s: %v", addrStr, err)
		}
		return
	}

	if peerInfo == nil {
		return
	}

	log.Printf("[DHT] SUCCESS! Found wgmesh peer %s (%s) at %s", peerInfo.WGPubKey[:8]+"...", peerInfo.MeshIP, peerInfo.Endpoint)
	if discoveryMethod == DHTMethod+"-transitive" {
		if isIPv6Endpoint(peerInfo.Endpoint) {
			log.Printf("[Path] Peer established via direct IPv6 path: %s (%s)", shortKey(peerInfo.WGPubKey), peerInfo.Endpoint)
		} else {
			log.Printf("[NAT] Peer established via NAT traversal path: %s (%s)", shortKey(peerInfo.WGPubKey), peerInfo.Endpoint)
		}
	}
	d.setControlEndpoint(peerInfo.WGPubKey, addrStr)
	d.peerStore.Update(peerInfo, discoveryMethod)
}

func getPeerHandshakeTS(iface, peerPubKey string) int64 {
	if iface == "" || peerPubKey == "" {
		return 0
	}
	hs, err := wireguard.GetLatestHandshakes(iface)
	if err != nil {
		return 0
	}
	return hs[peerPubKey]
}

func waitForPeerHandshake(iface, peerPubKey string, baseline int64, timeout time.Duration) int64 {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		current := getPeerHandshakeTS(iface, peerPubKey)
		if current > baseline {
			return current
		}
		time.Sleep(HandshakePollInterval)
	}
	return getPeerHandshakeTS(iface, peerPubKey)
}

func (d *DHTDiscovery) setControlEndpoint(peerPubKey, endpoint string) {
	if peerPubKey == "" {
		return
	}
	normalized := normalizeKnownPeerEndpoint(endpoint)
	normalized = filterEndpointForConfig(normalized, d.config.DisableIPv6)
	if normalized == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.controlPeers[peerPubKey] = normalized
}

func (d *DHTDiscovery) controlEndpointForPeer(peer *daemon.PeerInfo) string {
	if peer == nil || peer.WGPubKey == "" {
		return ""
	}

	d.mu.RLock()
	if endpoint, ok := d.controlPeers[peer.WGPubKey]; ok {
		d.mu.RUnlock()
		return endpoint
	}
	d.mu.RUnlock()

	if endpoint := toControlEndpoint(peer.Endpoint, int(d.config.Keys.GossipPort)); endpoint != "" {
		if d.config.DisableIPv6 && isIPv6Endpoint(endpoint) {
			return ""
		}
		return endpoint
	}

	return ""
}

func (d *DHTDiscovery) shouldAttemptRendezvous(remoteKey string, now time.Time) bool {
	if remoteKey == "" {
		return false
	}

	windowSeconds := int64(RendezvousWindow / time.Second)
	if windowSeconds <= 0 {
		return true
	}
	phaseSeconds := int64(RendezvousPhase / time.Second)
	if phaseSeconds <= 0 {
		phaseSeconds = 1
	}

	seed := pairSeed(d.localNode.WGPubKey, remoteKey)
	offset := int64(seed % uint64(windowSeconds))
	position := now.Unix() % windowSeconds
	delta := position - offset
	if delta < 0 {
		delta += windowSeconds
	}

	return delta < phaseSeconds
}

type rendezvousIntroducer struct {
	WGPubKey        string
	Endpoint        string
	ControlEndpoint string
}

// isAutoIntroducerCandidate checks if a peer qualifies as an auto-detected
// introducer. The caller must provide handshakes to avoid forking `wg show`
// per candidate (D6).
func (d *DHTDiscovery) isAutoIntroducerCandidate(p *daemon.PeerInfo, handshakes map[string]int64) bool {
	if p == nil {
		return false
	}
	if time.Since(p.LastSeen) > 2*time.Minute {
		return false
	}
	d.mu.RLock()
	_, hasControl := d.controlPeers[p.WGPubKey]
	d.mu.RUnlock()
	if !hasControl {
		return false
	}

	if ts, ok := handshakes[p.WGPubKey]; ok && ts > 0 {
		if time.Since(time.Unix(ts, 0)) < 2*time.Minute {
			return true
		}
	}
	return false
}

func (d *DHTDiscovery) hasIPv6Route() bool {
	if d.config.DisableIPv6 {
		return false
	}
	conn, err := net.Dial("udp6", "[2001:4860:4860::8888]:53")
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (d *DHTDiscovery) selectRendezvousIntroducers(remoteKey string, peers []*daemon.PeerInfo, maxCount int) []rendezvousIntroducer {
	type introducerCandidate struct {
		pubKey          string
		endpoint        string
		controlEndpoint string
		isExplicit      bool
	}

	// Fetch handshakes once for all candidates (D6: avoid forking wg show per peer)
	handshakes, _ := wireguard.GetLatestHandshakes(d.config.InterfaceName)

	candidates := make([]introducerCandidate, 0, len(peers))
	for _, p := range peers {
		if p == nil {
			continue
		}
		if p.WGPubKey == "" || p.WGPubKey == d.localNode.WGPubKey || p.WGPubKey == remoteKey {
			continue
		}
		if !hasAnyDHTReachability(p.DiscoveredVia) {
			d.debugf("[NAT] DEBUG: %s skipped - no DHT reachability (via=%v)", shortKey(p.WGPubKey), p.DiscoveredVia)
			continue
		}
		if p.Endpoint == "" || !isLikelyPublicEndpoint(p.Endpoint) {
			d.debugf("[NAT] DEBUG: %s skipped - endpoint not public (%s)", shortKey(p.WGPubKey), p.Endpoint)
			continue
		}
		if d.config.DisableIPv6 && isIPv6Endpoint(p.Endpoint) {
			d.debugf("[NAT] DEBUG: %s skipped - IPv6 disabled (%s)", shortKey(p.WGPubKey), p.Endpoint)
			continue
		}

		controlEndpoint := d.controlEndpointForPeer(p)
		if controlEndpoint == "" || !isLikelyPublicEndpoint(controlEndpoint) {
			d.debugf("[NAT] DEBUG: %s skipped - control endpoint not public (%s)", shortKey(p.WGPubKey), controlEndpoint)
			continue
		}

		isExplicit := p.Introducer
		isAuto := !isExplicit && d.isAutoIntroducerCandidate(p, handshakes)

		if !isExplicit && !isAuto {
			d.debugf("[NAT] DEBUG: %s skipped - not explicit introducer and not auto-eligible (explicit=%v auto=%v)", shortKey(p.WGPubKey), isExplicit, isAuto)
			continue
		}

		d.debugf("[NAT] DEBUG: %s selected as introducer (explicit=%v auto=%v control=%s)", shortKey(p.WGPubKey), isExplicit, isAuto, controlEndpoint)
		candidates = append(candidates, introducerCandidate{
			pubKey:          p.WGPubKey,
			endpoint:        p.Endpoint,
			controlEndpoint: controlEndpoint,
			isExplicit:      isExplicit,
		})
	}

	if len(candidates) == 0 {
		d.debugf("[NAT] DEBUG: no introducer candidates for %s", shortKey(remoteKey))
	}

	if len(candidates) == 0 || maxCount <= 0 {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].isExplicit != candidates[j].isExplicit {
			return candidates[i].isExplicit
		}
		if candidates[i].pubKey == candidates[j].pubKey {
			return candidates[i].endpoint < candidates[j].endpoint
		}
		return candidates[i].pubKey < candidates[j].pubKey
	})

	seed := pairSeed(d.localNode.WGPubKey, remoteKey)
	start := int(seed % uint64(len(candidates)))

	if maxCount > len(candidates) {
		maxCount = len(candidates)
	}

	out := make([]rendezvousIntroducer, 0, maxCount)
	for i := 0; i < maxCount; i++ {
		idx := (start + i) % len(candidates)
		out = append(out, rendezvousIntroducer{
			WGPubKey:        candidates[idx].pubKey,
			Endpoint:        candidates[idx].endpoint,
			ControlEndpoint: candidates[idx].controlEndpoint,
		})
	}

	return out
}

func pairSeed(a, b string) uint64 {
	if a > b {
		a, b = b, a
	}

	h := fnv.New64a()
	_, _ = h.Write([]byte(a))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(b))
	return h.Sum64()
}

func isLikelyPublicEndpoint(endpoint string) bool {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return false
	}

	ip := net.ParseIP(host)
	if ip == nil {
		// DNS hostnames are treated as potentially public.
		return true
	}

	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() || ip.IsUnspecified() {
		return false
	}

	return ip.IsGlobalUnicast()
}

func toControlEndpoint(endpoint string, controlPort int) string {
	if controlPort <= 0 {
		return ""
	}

	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return ""
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return ""
	}

	return net.JoinHostPort(host, fmt.Sprintf("%d", controlPort))
}

func shortKey(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:8] + "..."
}

func (d *DHTDiscovery) debugf(format string, args ...interface{}) {
	if strings.EqualFold(d.config.LogLevel, "debug") {
		log.Printf(format, args...)
	}
}

func hasDiscoveryMethod(methods []string, method string) bool {
	for _, m := range methods {
		if m == method {
			return true
		}
	}
	return false
}

func hasAnyDHTReachability(methods []string) bool {
	for _, m := range methods {
		if strings.HasPrefix(m, DHTMethod) {
			return true
		}
	}
	return false
}

func (d *DHTDiscovery) markContacted(addr string, minInterval time.Duration) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if lastContact, ok := d.contactedPeers[addr]; ok {
		if time.Since(lastContact) < minInterval {
			return false
		}
	}

	d.contactedPeers[addr] = time.Now()
	return true
}
