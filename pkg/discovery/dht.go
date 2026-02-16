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
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/dht/v2"
	"github.com/anacrolix/dht/v2/krpc"
	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

const (
	DHTAnnounceInterval       = 15 * time.Minute
	DHTQueryInterval          = 30 * time.Second
	DHTQueryIntervalStable    = 60 * time.Second
	DHTTransitiveInterval     = 1 * time.Second
	DHTBootstrapTimeout       = 30 * time.Second
	DHTPersistInterval        = 2 * time.Minute
	DHTMethod                 = "dht"
	DHTMaxConcurrentExchanges = 10 // Limit concurrent transitive exchanges to prevent resource exhaustion
	RendezvousWindow          = 20 * time.Second
	RendezvousPhase           = 3 * time.Second
	RendezvousPunchDelay      = 500 * time.Millisecond
	RendezvousMaxIntroducers  = 3
)

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

	mu             sync.RWMutex
	running        bool
	ctx            context.Context
	cancel         context.CancelFunc
	contactedPeers map[string]time.Time // Dedup: don't spam same IP
	controlPeers   map[string]string    // peer pubkey -> exchange/control endpoint

	// Callbacks
	onPeerDiscovered func(addr net.Addr)
}

// LocalNode represents our local node information
type LocalNode struct {
	WGPubKey         string
	WGPrivateKey     string
	MeshIP           string
	WGEndpoint       string
	RoutableNetworks []string
	Introducer       bool
}

// NewDHTDiscovery creates a new DHT discovery instance
func NewDHTDiscovery(config *daemon.Config, localNode *LocalNode, peerStore *daemon.PeerStore) (*DHTDiscovery, error) {
	ctx, cancel := context.WithCancel(context.Background())

	d := &DHTDiscovery{
		config:         config,
		localNode:      localNode,
		peerStore:      peerStore,
		ctx:            ctx,
		cancel:         cancel,
		contactedPeers: make(map[string]time.Time),
		controlPeers:   make(map[string]string),
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
	go d.transitiveConnectLoop()

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
	return filepath.Join("/var/lib/wgmesh", fmt.Sprintf("%s-dht.nodes", d.config.InterfaceName))
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

	// Skip if this is our own address
	if addrStr == d.localNode.WGEndpoint {
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
	ticker := time.NewTicker(DHTTransitiveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.tryTransitivePeers()
		}
	}
}

func (d *DHTDiscovery) tryTransitivePeers() {
	peers := d.peerStore.GetActive()
	now := time.Now()

	// Use a semaphore to limit concurrent exchanges
	sem := make(chan struct{}, DHTMaxConcurrentExchanges)

	// Fire-and-forget: spawn goroutines up to semaphore limit
	// The semaphore prevents resource exhaustion by limiting concurrent exchanges
	for _, peer := range peers {
		if peer.WGPubKey == "" || peer.WGPubKey == d.localNode.WGPubKey {
			continue
		}
		if peer.Endpoint == "" {
			continue
		}
		if !d.shouldAttemptRendezvous(peer.WGPubKey, now) {
			continue
		}

		introducers := d.selectRendezvousIntroducers(peer.WGPubKey, peers, RendezvousMaxIntroducers)
		requestedViaIntroducer := false
		if len(introducers) > 0 {
			for _, introducer := range introducers {
				if introducer.ControlEndpoint == "" {
					continue
				}

				if !d.markContacted(introducer.ControlEndpoint, 20*time.Second) {
					continue
				}

				log.Printf("[NAT] Pair %s <-> %s using rendezvous introducer %s (control=%s wg=%s)", shortKey(d.localNode.WGPubKey), shortKey(peer.WGPubKey), shortKey(introducer.WGPubKey), introducer.ControlEndpoint, introducer.Endpoint)

				sem <- struct{}{}
				go func(endpoint string, target *daemon.PeerInfo) {
					defer func() { <-sem }()
					if err := d.exchange.RequestRendezvous(endpoint, target.WGPubKey, nil); err != nil {
						log.Printf("[NAT] Failed to request rendezvous via %s for %s: %v", endpoint, shortKey(target.WGPubKey), err)
						return
					}
					log.Printf("[NAT] Rendezvous request sent for pair %s <-> %s via %s", shortKey(d.localNode.WGPubKey), shortKey(target.WGPubKey), endpoint)
				}(introducer.ControlEndpoint, peer)
				requestedViaIntroducer = true
			}
		}

		if requestedViaIntroducer {
			continue
		}

		if len(introducers) > 0 {
			log.Printf("[NAT] Waiting for rendezvous throttle window for %s (introducers selected but not contacted this tick)", shortKey(peer.WGPubKey))
			continue
		}

		targetControlEndpoint := d.controlEndpointForPeer(peer)
		if targetControlEndpoint == "" {
			continue
		}

		if !d.markContacted(targetControlEndpoint, 20*time.Second) {
			continue
		}
		log.Printf("[NAT] Synchronized punch window open for %s via %s (no introducer available)", shortKey(peer.WGPubKey), targetControlEndpoint)

		// Acquire semaphore before spawning goroutine to limit concurrency
		sem <- struct{}{}
		go func(endpoint string, hasIntroducer bool) {
			defer func() { <-sem }()
			if hasIntroducer {
				time.Sleep(RendezvousPunchDelay)
			}
			d.exchangeWithAddress(endpoint, DHTMethod+"-transitive")
		}(targetControlEndpoint, false)
	}
}

func (d *DHTDiscovery) exchangeWithAddress(addrStr string, discoveryMethod string) {
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
		log.Printf("[NAT] Peer established via NAT traversal path: %s (%s)", peerInfo.WGPubKey[:8]+"...", peerInfo.Endpoint)
	}
	d.setControlEndpoint(peerInfo.WGPubKey, addrStr)
	d.peerStore.Update(peerInfo, discoveryMethod)
}

func (d *DHTDiscovery) setControlEndpoint(peerPubKey, endpoint string) {
	if peerPubKey == "" {
		return
	}
	normalized := normalizeKnownPeerEndpoint(endpoint)
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

func (d *DHTDiscovery) selectRendezvousIntroducers(remoteKey string, peers []*daemon.PeerInfo, maxCount int) []rendezvousIntroducer {
	type introducerCandidate struct {
		pubKey          string
		endpoint        string
		controlEndpoint string
	}

	candidates := make([]introducerCandidate, 0, len(peers))
	for _, p := range peers {
		if p == nil {
			continue
		}
		if p.WGPubKey == "" || p.WGPubKey == d.localNode.WGPubKey || p.WGPubKey == remoteKey {
			continue
		}
		if !p.Introducer {
			continue
		}
		if !hasDiscoveryMethod(p.DiscoveredVia, DHTMethod) {
			// Only use peers with direct DHT reachability as introducers.
			continue
		}
		if p.Endpoint == "" || !isLikelyPublicEndpoint(p.Endpoint) {
			continue
		}

		controlEndpoint := d.controlEndpointForPeer(p)
		if controlEndpoint == "" || !isLikelyPublicEndpoint(controlEndpoint) {
			continue
		}

		candidates = append(candidates, introducerCandidate{
			pubKey:          p.WGPubKey,
			endpoint:        p.Endpoint,
			controlEndpoint: controlEndpoint,
		})
	}

	if len(candidates) == 0 || maxCount <= 0 {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
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

func hasDiscoveryMethod(methods []string, method string) bool {
	for _, m := range methods {
		if m == method {
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

// SetOnPeerDiscovered sets a callback for when peers are discovered
func (d *DHTDiscovery) SetOnPeerDiscovered(callback func(addr net.Addr)) {
	d.onPeerDiscovered = callback
}
