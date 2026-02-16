package daemon

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
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
	config    *Config
	localNode *LocalNode
	peerStore *PeerStore

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
		config:    config,
		peerStore: NewPeerStore(),
		ctx:       ctx,
		cancel:    cancel,
	}

	return d, nil
}

// SetDHTDiscovery sets the DHT discovery layer
func (d *Daemon) SetDHTDiscovery(dht DiscoveryLayer) {
	d.dhtDiscovery = dht
}

// Run starts the daemon and blocks until stopped
func (d *Daemon) Run() error {
	log.Printf("[Daemon] Starting wgmesh daemon...")

	// Load or create local node
	if err := d.initLocalNode(); err != nil {
		return fmt.Errorf("failed to initialize local node: %w", err)
	}

	log.Printf("[Daemon] Local node: %s", d.localNode.WGPubKey[:16]+"...")
	log.Printf("[Daemon] Mesh IP: %s", d.localNode.MeshIP)

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
	}

	// Save to state file
	if err := saveLocalNode(stateFile, d.localNode); err != nil {
		log.Printf("[Daemon] Warning: failed to save local node state: %v", err)
	}

	return nil
}

// setupWireGuard creates and configures the WireGuard interface
func (d *Daemon) setupWireGuard() error {
	log.Printf("[Daemon] Setting up WireGuard interface %s...", d.config.InterfaceName)

	// Check if interface exists
	if interfaceExists(d.config.InterfaceName) {
		// Check if existing interface already has our port
		existingPort := getWGInterfacePort(d.config.InterfaceName)
		if existingPort == d.config.WGListenPort {
			// Same interface with same port - just reset it
			log.Printf("[Daemon] Interface %s exists with same port, resetting...", d.config.InterfaceName)
		} else {
			log.Printf("[Daemon] Interface %s exists, resetting...", d.config.InterfaceName)
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
		log.Printf("[Daemon] Port %d is in use, using port %d instead", listenPort, availablePort)
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

	log.Printf("[Daemon] WireGuard interface %s ready on port %d", d.config.InterfaceName, listenPort)
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
	for _, peer := range peers {
		// Skip ourselves
		if peer.WGPubKey == d.localNode.WGPubKey {
			continue
		}

		// Add/update peer in WireGuard
		if err := d.configurePeer(peer); err != nil {
			log.Printf("[Daemon] Failed to configure peer %s: %v", peer.WGPubKey[:8]+"...", err)
		}
	}

	if err := d.syncPeerRoutes(peers); err != nil {
		log.Printf("[Daemon] Failed to sync peer routes: %v", err)
	}

	// Check for mesh IP collisions
	d.CheckAndResolveCollisions()

	// Cleanup stale peers
	removed := d.peerStore.CleanupStale()
	for _, pubKey := range removed {
		if err := d.removePeer(pubKey); err != nil {
			log.Printf("[Daemon] Failed to remove stale peer %s: %v", pubKey[:8]+"...", err)
		}
	}
}

// configurePeer adds or updates a peer in the WireGuard configuration
func (d *Daemon) configurePeer(peer *PeerInfo) error {
	// Build allowed IPs (mesh IP + routable networks)
	allowedIPs := peer.MeshIP + "/32"
	for _, net := range peer.RoutableNetworks {
		allowedIPs += "," + net
	}

	// Use wg set to add/update peer
	return wireguard.SetPeer(
		d.config.InterfaceName,
		peer.WGPubKey,
		d.config.Keys.PSK,
		peer.Endpoint,
		allowedIPs,
	)
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
	log.Printf("[Daemon] Starting wgmesh daemon with DHT discovery...")

	// Load or create local node first
	if err := d.initLocalNode(); err != nil {
		return fmt.Errorf("failed to initialize local node: %w", err)
	}

	log.Printf("[Daemon] Local node: %s", d.localNode.WGPubKey[:16]+"...")
	log.Printf("[Daemon] Mesh IP: %s", d.localNode.MeshIP)
	log.Printf("[Daemon] Network ID: %x (both nodes must show the same ID to find each other)", d.config.Keys.NetworkID[:8])

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
		log.Printf("[Daemon] Warning: DHT discovery factory not set, running without DHT")
	}

	// Start epoch manager for privacy features
	if d.config.Privacy {
		d.epochManager = NewEpochManager(d.config.Keys.EpochSeed)
		d.epochManager.Start(d.getPrivacyPeers)
		defer d.epochManager.Stop()
		log.Printf("[Daemon] Privacy mode enabled (Dandelion++ relay)")
	}

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start reconciliation loop
	go d.reconcileLoop()

	// Start status printer
	go d.statusLoop()

	log.Printf("[Daemon] Daemon running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	select {
	case sig := <-sigCh:
		log.Printf("[Daemon] Received signal %v, shutting down...", sig)
	case <-d.ctx.Done():
		log.Printf("[Daemon] Context cancelled, shutting down...")
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
