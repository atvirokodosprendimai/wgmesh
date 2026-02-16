package daemon

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"net"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/privacy"
	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

const (
	ReconcileInterval = 5 * time.Second
	StatusInterval    = 30 * time.Second
)

// Daemon manages the mesh node lifecycle
type Daemon struct {
	config                 *Config
	localNode              *LocalNode
	peerStore              *PeerStore
	lastAppliedPeerConfigs map[string]string

	// Discovery layer (DHT discovery will be attached)
	dhtDiscovery DiscoveryLayer

	// Epoch manager for Dandelion++ privacy
	epochManager *EpochManager

	// Cache stop channel
	cacheStopCh chan struct{}

	ctx    context.Context
	cancel context.CancelFunc
}

// LocalNode represents our local WireGuard node
type LocalNode struct {
	WGPubKey         string
	WGPrivateKey     string
	MeshIP           string
	WGEndpoint       string
	RoutableNetworks []string
	Introducer       bool
	NATType          string // Detected NAT type: "cone", "symmetric", or "unknown"
}

// DiscoveryLayer is the interface for discovery implementations
type DiscoveryLayer interface {
	Start() error
	Stop() error
}

// NewDaemon creates a new mesh daemon
func NewDaemon(config *Config) (*Daemon, error) {
	ctx, cancel := context.WithCancel(context.Background())

	d := &Daemon{
		config:                 config,
		peerStore:              NewPeerStore(),
		lastAppliedPeerConfigs: make(map[string]string),
		ctx:                    ctx,
		cancel:                 cancel,
	}

	return d, nil
}

// SetDHTDiscovery sets the DHT discovery layer
func (d *Daemon) SetDHTDiscovery(dht DiscoveryLayer) {
	d.dhtDiscovery = dht
}

// Run starts the daemon and blocks until stopped
func (d *Daemon) Run() error {
	log.Printf("Starting wgmesh daemon...")

	// Load or create local node
	if err := d.initLocalNode(); err != nil {
		return fmt.Errorf("failed to initialize local node: %w", err)
	}

	log.Printf("Local node: %s", d.localNode.WGPubKey[:16]+"...")
	log.Printf("Mesh IP: %s", d.localNode.MeshIP)

	// Setup WireGuard interface
	if err := d.setupWireGuard(); err != nil {
		return fmt.Errorf("failed to setup WireGuard: %w", err)
	}
	d.setLocalWGEndpoint()

	// Start DHT discovery if configured
	if d.dhtDiscovery != nil {
		if err := d.dhtDiscovery.Start(); err != nil {
			return fmt.Errorf("failed to start DHT discovery: %w", err)
		}
		defer d.dhtDiscovery.Stop()
	}

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start reconciliation loop
	go d.reconcileLoop()

	// Start status printer
	go d.statusLoop()

	log.Printf("Daemon running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	select {
	case sig := <-sigCh:
		log.Printf("Received signal %v, shutting down...", sig)
	case <-d.ctx.Done():
		log.Printf("Context cancelled, shutting down...")
	}

	d.cancel()
	return nil
}

// initLocalNode loads or creates the local WireGuard node
func (d *Daemon) initLocalNode() error {
	// Try to load existing key from state file
	stateFile := fmt.Sprintf("/var/lib/wgmesh/%s.json", d.config.InterfaceName)
	node, err := loadLocalNode(stateFile)
	if err == nil && node != nil {
		d.localNode = node
		// Derive mesh IP from pubkey
		d.localNode.MeshIP = crypto.DeriveMeshIP(d.config.Keys.MeshSubnet, d.localNode.WGPubKey, d.config.Secret)
		d.localNode.RoutableNetworks = d.config.AdvertiseRoutes
		d.localNode.Introducer = d.config.Introducer
		return nil
	}

	// Generate new keypair
	privateKey, publicKey, err := wireguard.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate keypair: %w", err)
	}

	// Derive mesh IP from public key
	meshIP := crypto.DeriveMeshIP(d.config.Keys.MeshSubnet, publicKey, d.config.Secret)

	d.localNode = &LocalNode{
		WGPubKey:         publicKey,
		WGPrivateKey:     privateKey,
		MeshIP:           meshIP,
		RoutableNetworks: d.config.AdvertiseRoutes,
		Introducer:       d.config.Introducer,
	}

	// Save to state file
	if err := saveLocalNode(stateFile, d.localNode); err != nil {
		log.Printf("Warning: failed to save local node state: %v", err)
	}

	return nil
}

// setupWireGuard creates and configures the WireGuard interface
func (d *Daemon) setupWireGuard() error {
	log.Printf("Setting up WireGuard interface %s...", d.config.InterfaceName)

	// Check if interface exists
	if interfaceExists(d.config.InterfaceName) {
		// Check if existing interface already has our port
		existingPort := getWGInterfacePort(d.config.InterfaceName)
		if existingPort == d.config.WGListenPort {
			// Same interface with same port - just reset it
			log.Printf("Interface %s exists with same port, resetting...", d.config.InterfaceName)
		} else {
			log.Printf("Interface %s exists, resetting...", d.config.InterfaceName)
		}
		if err := resetInterface(d.config.InterfaceName); err != nil {
			return fmt.Errorf("failed to reset interface: %w", err)
		}
	} else {
		// Create interface
		if err := createInterface(d.config.InterfaceName); err != nil {
			return fmt.Errorf("failed to create interface: %w", err)
		}
	}

	// Check if port is in use by another interface
	listenPort := d.config.WGListenPort
	if isPortInUse(listenPort) {
		// Port is in use - find an available one
		availablePort := findAvailablePort(listenPort + 1)
		if availablePort == 0 {
			return fmt.Errorf("port %d is in use and no available ports found (try --listen-port with a different port)", listenPort)
		}
		log.Printf("Port %d is in use, using port %d instead", listenPort, availablePort)
		listenPort = availablePort
		d.config.WGListenPort = availablePort
	}

	// Configure interface with private key and listen port
	if err := configureInterface(d.config.InterfaceName, d.localNode.WGPrivateKey, listenPort); err != nil {
		return fmt.Errorf("failed to configure interface: %w", err)
	}

	// Set IP address
	if err := setInterfaceAddress(d.config.InterfaceName, d.localNode.MeshIP+"/16"); err != nil {
		return fmt.Errorf("failed to set IP address: %w", err)
	}

	// Bring interface up
	if err := setInterfaceUp(d.config.InterfaceName); err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	log.Printf("WireGuard interface %s ready on port %d", d.config.InterfaceName, listenPort)
	return nil
}

// reconcileLoop periodically reconciles the WireGuard configuration
func (d *Daemon) reconcileLoop() {
	ticker := time.NewTicker(ReconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.reconcile()
		}
	}
}

// reconcile updates WireGuard configuration based on discovered peers
func (d *Daemon) reconcile() {
	peers := d.peerStore.GetActive()
	desired := d.buildDesiredPeerConfigs(peers)
	if err := d.applyDesiredPeerConfigs(desired); err != nil {
		log.Printf("Failed to apply WireGuard peer configuration: %v", err)
	}

	if err := d.syncPeerRoutes(peers); err != nil {
		log.Printf("Failed to sync peer routes: %v", err)
	}

	// Check for mesh IP collisions
	d.CheckAndResolveCollisions()

	// Cleanup stale peers
	removed := d.peerStore.CleanupStale()
	for _, pubKey := range removed {
		if err := d.removePeer(pubKey); err != nil {
			log.Printf("Failed to remove stale peer %s: %v", pubKey[:8]+"...", err)
		}
	}
}

type desiredPeerConfig struct {
	peer    *PeerInfo
	allowed map[string]struct{}
}

func (d *Daemon) buildDesiredPeerConfigs(peers []*PeerInfo) map[string]*desiredPeerConfig {
	desired := make(map[string]*desiredPeerConfig)
	relayCandidates := make([]*PeerInfo, 0)

	for _, p := range peers {
		if p.WGPubKey == d.localNode.WGPubKey || p.WGPubKey == "" {
			continue
		}
		if p.Introducer && p.Endpoint != "" {
			relayCandidates = append(relayCandidates, p)
		}
	}

	// Query WG handshake times to detect unreachable peers
	handshakes, _ := wireguard.GetLatestHandshakes(d.config.InterfaceName)

	for _, p := range peers {
		if p.WGPubKey == d.localNode.WGPubKey || p.WGPubKey == "" || p.MeshIP == "" {
			continue
		}

		if d.shouldRelayPeer(p, relayCandidates, handshakes) {
			relay := d.selectRelayForPeer(p, relayCandidates)
			if relay != nil {
				d.addAllowedIP(desired, relay, p.MeshIP+"/32")
				for _, network := range p.RoutableNetworks {
					network = strings.TrimSpace(network)
					if network != "" {
						d.addAllowedIP(desired, relay, network)
					}
				}
				continue
			}
		}

		d.addAllowedIP(desired, p, p.MeshIP+"/32")
		for _, network := range p.RoutableNetworks {
			network = strings.TrimSpace(network)
			if network != "" {
				d.addAllowedIP(desired, p, network)
			}
		}
	}

	return desired
}

// shouldRelayPeer decides whether traffic to a peer should be routed via
// an introducer relay. Relay is used when:
//   - Both this node and the peer have symmetric NAT (hole-punch unreliable), OR
//   - The peer has been unreachable (no WG handshake for >2 minutes)
//
// Never relays: from introducers, to introducers, without relay candidates.
func (d *Daemon) shouldRelayPeer(peer *PeerInfo, relayCandidates []*PeerInfo, handshakes map[string]int64) bool {
	if d.config.Introducer {
		return false // Introducers are always direct
	}
	if peer.Introducer {
		return false // Don't relay to an introducer
	}
	if len(relayCandidates) == 0 {
		return false // No relay available
	}

	// Check WG handshake first — if we've had a recent handshake, direct
	// connectivity is confirmed regardless of NAT type.
	if handshakes != nil {
		if ts, ok := handshakes[peer.WGPubKey]; ok && ts > 0 {
			lastHandshake := time.Unix(ts, 0)
			if time.Since(lastHandshake) < 2*time.Minute {
				return false // Direct path is working
			}
			// Handshake stale >2 min — peer may be unreachable directly
			return true
		}
	}

	// Both sides symmetric → hole-punch is unreliable, relay
	if d.localNode.NATType == "symmetric" && peer.NATType == "symmetric" {
		return true
	}

	// No handshake data yet (peer just discovered) — try direct first.
	// The next reconcile cycle will check again.
	return false
}

func (d *Daemon) selectRelayForPeer(peer *PeerInfo, relayCandidates []*PeerInfo) *PeerInfo {
	if len(relayCandidates) == 0 || peer == nil {
		return nil
	}

	sorted := make([]*PeerInfo, 0, len(relayCandidates))
	for _, candidate := range relayCandidates {
		if candidate == nil || candidate.WGPubKey == "" || candidate.Endpoint == "" {
			continue
		}
		sorted = append(sorted, candidate)
	}
	if len(sorted) == 0 {
		return nil
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].WGPubKey < sorted[j].WGPubKey
	})

	h := fnv.New64a()
	_, _ = h.Write([]byte(d.localNode.WGPubKey))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(peer.WGPubKey))
	idx := int(h.Sum64() % uint64(len(sorted)))

	return sorted[idx]
}

func (d *Daemon) addAllowedIP(desired map[string]*desiredPeerConfig, peer *PeerInfo, cidr string) {
	if peer == nil || peer.WGPubKey == "" || cidr == "" {
		return
	}

	entry, exists := desired[peer.WGPubKey]
	if !exists {
		entry = &desiredPeerConfig{peer: peer, allowed: make(map[string]struct{})}
		desired[peer.WGPubKey] = entry
	}
	entry.allowed[cidr] = struct{}{}
}

func (d *Daemon) applyDesiredPeerConfigs(desired map[string]*desiredPeerConfig) error {
	existing, err := wireguard.GetPeers(d.config.InterfaceName)
	if err == nil {
		for _, current := range existing {
			if _, ok := desired[current.PublicKey]; !ok {
				if err := wireguard.RemovePeer(d.config.InterfaceName, current.PublicKey); err != nil {
					log.Printf("Failed to remove obsolete peer %s: %v", current.PublicKey[:8]+"...", err)
				}
				delete(d.lastAppliedPeerConfigs, current.PublicKey)
			}
		}
	}

	for pubKey, cfg := range desired {
		if cfg.peer.Endpoint == "" {
			continue
		}
		allowed := mapKeysSorted(cfg.allowed)
		if len(allowed) == 0 {
			continue
		}
		allowedCSV := strings.Join(allowed, ",")
		signature := cfg.peer.Endpoint + "|" + allowedCSV
		if prev, ok := d.lastAppliedPeerConfigs[pubKey]; ok && prev == signature {
			continue
		}
		if err := wireguard.SetPeer(d.config.InterfaceName, pubKey, d.config.Keys.PSK, cfg.peer.Endpoint, allowedCSV); err != nil {
			return fmt.Errorf("failed to configure peer %s: %w", pubKey[:8]+"...", err)
		}
		d.lastAppliedPeerConfigs[pubKey] = signature
	}

	return nil
}

func mapKeysSorted(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// removePeer removes a peer from the WireGuard configuration
func (d *Daemon) removePeer(pubKey string) error {
	return wireguard.RemovePeer(d.config.InterfaceName, pubKey)
}

// statusLoop periodically prints mesh status
func (d *Daemon) statusLoop() {
	ticker := time.NewTicker(StatusInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.printStatus()
		}
	}
}

// printStatus prints current mesh status
func (d *Daemon) printStatus() {
	peers := d.peerStore.GetActive()
	log.Printf("[Status] Active peers: %d", len(peers))
	for _, p := range peers {
		log.Printf("  - %s (%s) via %v", p.WGPubKey[:8]+"...", p.MeshIP, p.DiscoveredVia)
	}
}

// GetLocalNode returns the local node info
func (d *Daemon) GetLocalNode() *LocalNode {
	return d.localNode
}

// GetPeerStore returns the peer store
func (d *Daemon) GetPeerStore() *PeerStore {
	return d.peerStore
}

// GetConfig returns the daemon config
func (d *Daemon) GetConfig() *Config {
	return d.config
}

// RunWithDHTDiscovery runs the daemon with DHT discovery enabled
// This is the main entry point for the join command
func (d *Daemon) RunWithDHTDiscovery() error {
	log.Printf("Starting wgmesh daemon with DHT discovery...")

	// Load or create local node first
	if err := d.initLocalNode(); err != nil {
		return fmt.Errorf("failed to initialize local node: %w", err)
	}

	log.Printf("Local node: %s", d.localNode.WGPubKey[:16]+"...")
	log.Printf("Mesh IP: %s", d.localNode.MeshIP)
	log.Printf("Network ID: %x (both nodes must show the same ID to find each other)", d.config.Keys.NetworkID[:8])

	// Setup WireGuard interface
	if err := d.setupWireGuard(); err != nil {
		return fmt.Errorf("failed to setup WireGuard: %w", err)
	}
	d.setLocalWGEndpoint()

	// Restore peers from cache for faster startup
	RestoreFromCache(d.config.InterfaceName, d.peerStore)

	// Start peer cache saver
	d.cacheStopCh = make(chan struct{})
	go StartCacheSaver(d.config.InterfaceName, d.peerStore, d.cacheStopCh)
	defer func() {
		select {
		case <-d.cacheStopCh:
			// Already closed
		default:
			close(d.cacheStopCh)
		}
	}()

	// Now create DHT discovery with the initialized local node
	// Import is handled via interface to avoid circular dependency
	dhtFactory := GetDHTDiscoveryFactory()
	if dhtFactory != nil {
		dht, err := dhtFactory(d.config, d.localNode, d.peerStore)
		if err != nil {
			return fmt.Errorf("failed to create DHT discovery: %w", err)
		}
		d.dhtDiscovery = dht

		if err := d.dhtDiscovery.Start(); err != nil {
			return fmt.Errorf("failed to start DHT discovery: %w", err)
		}
		defer d.dhtDiscovery.Stop()
	} else {
		log.Printf("Warning: DHT discovery factory not set, running without DHT")
	}

	// Start epoch manager for privacy features
	if d.config.Privacy {
		d.epochManager = NewEpochManager(d.config.Keys.EpochSeed)
		d.epochManager.Start(d.getPrivacyPeers)
		defer d.epochManager.Stop()
		log.Printf("Privacy mode enabled (Dandelion++ relay)")
	}

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start reconciliation loop
	go d.reconcileLoop()

	// Start status printer
	go d.statusLoop()

	log.Printf("Daemon running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	select {
	case sig := <-sigCh:
		log.Printf("Received signal %v, shutting down...", sig)
	case <-d.ctx.Done():
		log.Printf("Context cancelled, shutting down...")
	}

	d.cancel()
	return nil
}

// DHTDiscoveryFactory is a function type for creating DHT discovery instances
type DHTDiscoveryFactory func(config *Config, localNode *LocalNode, peerStore *PeerStore) (DiscoveryLayer, error)

var dhtDiscoveryFactory DHTDiscoveryFactory

// SetDHTDiscoveryFactory sets the factory function for creating DHT discovery
// This is called by the discovery package to avoid circular imports
func SetDHTDiscoveryFactory(factory DHTDiscoveryFactory) {
	dhtDiscoveryFactory = factory
}

// GetDHTDiscoveryFactory returns the current DHT discovery factory
func GetDHTDiscoveryFactory() DHTDiscoveryFactory {
	return dhtDiscoveryFactory
}

func (d *Daemon) setLocalWGEndpoint() {
	if d.localNode == nil {
		return
	}
	d.localNode.WGEndpoint = net.JoinHostPort("0.0.0.0", strconv.Itoa(d.config.WGListenPort))
}

// getPrivacyPeers returns current peers formatted for the privacy layer
func (d *Daemon) getPrivacyPeers() []privacy.PeerInfo {
	peers := d.peerStore.GetActive()
	result := make([]privacy.PeerInfo, 0, len(peers))
	for _, p := range peers {
		if p.WGPubKey != d.localNode.WGPubKey {
			result = append(result, privacy.PeerInfo{
				WGPubKey: p.WGPubKey,
				MeshIP:   p.MeshIP,
				Endpoint: p.Endpoint,
			})
		}
	}
	return result
}
