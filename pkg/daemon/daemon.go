package daemon

import (
	"bufio"
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/privacy"
	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

const (
	ReconcileInterval    = 5 * time.Second
	StatusInterval       = 30 * time.Second
	RelayCandidateMaxAge = 90 * time.Second
	StaleCleanupInterval = 1 * time.Minute
	HealthCheckInterval  = 20 * time.Second
	HandshakeStaleAfter  = 150 * time.Second
	MeshProbeInterval    = 1 * time.Second
	MeshProbeDialTimeout = 1200 * time.Millisecond // Increased from 800ms for cross-DC tolerance
	MeshProbeFailLimit   = 8
	MeshProbePortOffset  = 2000
	TemporaryOfflineTTL  = 30 * time.Second
	soBindToDevice       = 25 // Linux SO_BINDTODEVICE
)

type peerProbeSession struct {
	conn   net.Conn
	reader *bufio.Reader
}

// Daemon manages the mesh node lifecycle
type Daemon struct {
	config                 *Config
	localNode              *LocalNode
	peerStore              *PeerStore
	lastAppliedPeerConfigs map[string]string
	appliedMu              sync.Mutex
	relayRoutes            map[string]string // target pubkey -> relay pubkey
	relayMu                sync.RWMutex
	localSubnetsFn         func() []*net.IPNet
	peerHealthFailures     map[string]int
	lastPeerTransferTotal  map[string]uint64
	healthMu               sync.Mutex
	healthProbePort        int
	probeMu                sync.Mutex
	probeSessions          map[string]*peerProbeSession
	probeFailures          map[string]int
	probeListeners         []net.Listener
	offlineMu              sync.Mutex
	temporaryOffline       map[string]time.Time

	// Discovery layer (DHT discovery will be attached)
	dhtDiscovery DiscoveryLayer

	// Epoch manager for Dandelion++ privacy
	epochManager *EpochManager

	// RPC server
	rpcServer RPCServer

	// startTime is recorded when the daemon starts, used for uptime reporting.
	startTime time.Time

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// RPCServer interface for the RPC server
type RPCServer interface {
	Start() error
	Stop() error
}

// LocalNode represents our local WireGuard node
type LocalNode struct {
	WGPubKey         string
	WGPrivateKey     string
	MeshIP           string
	MeshIPv6         string
	WGEndpoint       string
	RoutableNetworks []string
	Introducer       bool
	NATType          string // Detected NAT type: "cone", "symmetric", or "unknown"
	Hostname         string
}

// DiscoveryLayer is the interface for discovery implementations
type DiscoveryLayer interface {
	Start() error
	Stop() error
}

// parseLogLevel converts a log level string to slog.Level.
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ConfigureLogging sets up the global logger with the given level.
// All existing log.Printf calls are redirected through slog at the
// configured level so they are always visible regardless of the filter.
// This should be called once at program startup (e.g. from main) before
// creating a Daemon; it must not be called from library code.
func ConfigureLogging(level string) {
	configureLogging(level)
}

func configureLogging(level string) {
	lvl := parseLogLevel(level)
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: lvl,
	})
	slog.SetDefault(slog.New(handler))

	// Redirect stdlib log.Printf → slog at the configured level so that
	// legacy log.Printf calls are never silenced by a stricter filter.
	// e.g. --log-level warn: log.Printf emits at WARN, still visible.
	log.SetOutput(&slogWriter{level: lvl})
	log.SetFlags(0) // slog adds its own timestamp
}

// slogWriter adapts log.Printf output to slog at a fixed level.
type slogWriter struct {
	level slog.Level
}

func (w *slogWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimRight(string(p), "\n")
	slog.Log(context.Background(), w.level, msg)
	return len(p), nil
}

// NewDaemon creates a new mesh daemon
func NewDaemon(config *Config) (*Daemon, error) {
	ctx, cancel := context.WithCancel(context.Background())

	d := &Daemon{
		config:                 config,
		peerStore:              NewPeerStore(),
		lastAppliedPeerConfigs: make(map[string]string),
		relayRoutes:            make(map[string]string),
		localSubnetsFn:         detectLocalSubnets,
		peerHealthFailures:     make(map[string]int),
		lastPeerTransferTotal:  make(map[string]uint64),
		healthProbePort:        int(config.Keys.GossipPort) + MeshProbePortOffset,
		probeSessions:          make(map[string]*peerProbeSession),
		probeFailures:          make(map[string]int),
		temporaryOffline:       make(map[string]time.Time),
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
	d.startTime = time.Now()
	log.Printf("Starting wgmesh daemon...")

	// Load or create local node
	if err := d.initLocalNode(); err != nil {
		return fmt.Errorf("failed to initialize local node: %w", err)
	}

	log.Printf("Local node: %s...", shortKey(d.localNode.WGPubKey))
	log.Printf("Mesh IP: %s", d.localNode.MeshIP)
	if d.localNode.MeshIPv6 != "" {
		log.Printf("Mesh IPv6: %s", d.localNode.MeshIPv6)
	}

	// Setup WireGuard interface
	if err := d.setupWireGuard(); err != nil {
		return fmt.Errorf("failed to setup WireGuard: %w", err)
	}
	defer d.teardownWireGuard()
	d.setLocalWGEndpoint()
	if err := d.startMeshProbeServer(); err != nil {
		log.Printf("[Health] Failed to start mesh probe server: %v", err)
	}

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
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.reconcileLoop()
	}()

	// Start status printer
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.statusLoop()
	}()

	// Periodically remove long-stale peers from memory/cache
	go d.staleCleanupLoop()

	// Monitor WG handshakes/transfer and quickly evict dead peers
	go d.healthMonitorLoop()

	// Keep persistent mesh-VPN health connections to peers
	go d.meshProbeLoop()

	log.Printf("Daemon running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	select {
	case sig := <-sigCh:
		log.Printf("Received signal %v, shutting down...", sig)
	case <-d.ctx.Done():
		log.Printf("Context cancelled, shutting down...")
	}

	d.cancel()
	log.Printf("Waiting for background tasks to complete...")
	d.wg.Wait()
	return nil
}

// Shutdown cancels the daemon context, signalling background goroutines
// to stop. Callers that need to wait for full shutdown completion should
// wait for Run() or RunWithDHTDiscovery() to return.
func (d *Daemon) Shutdown() {
	d.cancel()
}

// initLocalNode loads or creates the local WireGuard node
func (d *Daemon) initLocalNode() error {
	hostname, hostErr := os.Hostname()
	if hostErr != nil {
		hostname = ""
	}

	// Try to load existing key from state file
	stateFile := fmt.Sprintf("/var/lib/wgmesh/%s.json", d.config.InterfaceName)
	node, err := loadLocalNode(stateFile)
	if err == nil && node != nil {
		d.localNode = node
		// Derive mesh IP from pubkey
		d.localNode.MeshIP = crypto.DeriveMeshIP(d.config.Keys.MeshSubnet, d.localNode.WGPubKey, d.config.Secret)
		d.localNode.MeshIPv6 = crypto.DeriveMeshIPv6(d.config.Keys.MeshPrefixV6, d.localNode.WGPubKey, d.config.Secret)
		d.localNode.RoutableNetworks = d.config.AdvertiseRoutes
		d.localNode.Introducer = d.config.Introducer
		d.localNode.Hostname = hostname
		return nil
	}

	// Generate new keypair
	privateKey, publicKey, err := wireguard.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate keypair: %w", err)
	}

	// Derive mesh IP from public key
	meshIP := crypto.DeriveMeshIP(d.config.Keys.MeshSubnet, publicKey, d.config.Secret)
	meshIPv6 := crypto.DeriveMeshIPv6(d.config.Keys.MeshPrefixV6, publicKey, d.config.Secret)

	d.localNode = &LocalNode{
		WGPubKey:         publicKey,
		WGPrivateKey:     privateKey,
		MeshIP:           meshIP,
		MeshIPv6:         meshIPv6,
		RoutableNetworks: d.config.AdvertiseRoutes,
		Introducer:       d.config.Introducer,
		Hostname:         hostname,
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
	if d.localNode.MeshIPv6 != "" {
		if err := setInterfaceAddress(d.config.InterfaceName, d.localNode.MeshIPv6+"/64"); err != nil {
			return fmt.Errorf("failed to set IPv6 address: %w", err)
		}
	}

	// Bring interface up
	if err := setInterfaceUp(d.config.InterfaceName); err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	log.Printf("WireGuard interface %s ready on port %d", d.config.InterfaceName, listenPort)
	return nil
}

func (d *Daemon) teardownWireGuard() {
	if d == nil || d.config == nil || d.config.InterfaceName == "" {
		return
	}

	if err := setInterfaceDown(d.config.InterfaceName); err != nil {
		log.Printf("[Shutdown] Failed to bring down interface %s: %v", d.config.InterfaceName, err)
	}
	if err := deleteInterface(d.config.InterfaceName); err != nil {
		log.Printf("[Shutdown] Failed to delete interface %s: %v", d.config.InterfaceName, err)
		return
	}
	log.Printf("[Shutdown] WireGuard interface %s removed", d.config.InterfaceName)
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
	desired, relayRoutes := d.buildDesiredPeerConfigs(peers)
	d.relayMu.Lock()
	d.relayRoutes = relayRoutes
	d.relayMu.Unlock()
	if err := d.applyDesiredPeerConfigs(desired); err != nil {
		log.Printf("Failed to apply WireGuard peer configuration: %v", err)
	}

	if err := d.syncPeerRoutes(peers); err != nil {
		log.Printf("Failed to sync peer routes: %v", err)
	}

	// Check for mesh IP collisions
	d.CheckAndResolveCollisions()

	// Stale peer cleanup is handled by staleCleanupLoop (W3: avoid double cleanup)
}

type desiredPeerConfig struct {
	peer    *PeerInfo
	allowed map[string]struct{}
}

func (d *Daemon) buildDesiredPeerConfigs(peers []*PeerInfo) (map[string]*desiredPeerConfig, map[string]string) {
	desired := make(map[string]*desiredPeerConfig)
	relayRoutes := make(map[string]string)
	relayCandidates := make([]*PeerInfo, 0)
	now := time.Now()
	localSubnets := d.getLocalSubnets()

	for _, p := range peers {
		if p.WGPubKey == d.localNode.WGPubKey || p.WGPubKey == "" {
			continue
		}
		if d.isTemporarilyOffline(p.WGPubKey) {
			continue
		}
		if p.Introducer && p.Endpoint != "" && now.Sub(p.LastSeen) <= RelayCandidateMaxAge {
			relayCandidates = append(relayCandidates, p)
		}
	}

	// Query WG handshake times to detect unreachable peers
	handshakes, _ := wireguard.GetLatestHandshakes(d.config.InterfaceName)

	for _, p := range peers {
		if p.WGPubKey == d.localNode.WGPubKey || p.WGPubKey == "" || p.MeshIP == "" {
			continue
		}
		if d.isTemporarilyOffline(p.WGPubKey) {
			continue
		}

		if d.shouldRelayPeerWithSubnets(p, relayCandidates, handshakes, localSubnets) {
			relay := d.selectRelayForPeer(p, relayCandidates)
			if relay != nil {
				relayRoutes[p.WGPubKey] = relay.WGPubKey
				d.addAllowedIP(desired, relay, p.MeshIP+"/32")
				if p.MeshIPv6 != "" {
					d.addAllowedIP(desired, relay, p.MeshIPv6+"/128")
				}
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
		if p.MeshIPv6 != "" {
			d.addAllowedIP(desired, p, p.MeshIPv6+"/128")
		}
		for _, network := range p.RoutableNetworks {
			network = strings.TrimSpace(network)
			if network != "" {
				d.addAllowedIP(desired, p, network)
			}
		}
	}

	return desired, relayRoutes
}

// shouldRelayPeer decides whether traffic to a peer should be routed via
// an introducer relay. Relay is used when:
//   - Both this node and the peer have symmetric NAT (hole-punch unreliable), OR
//   - The peer has been unreachable (no WG handshake for >2 minutes)
//
// Never relays: from introducers, to introducers, without relay candidates.
func (d *Daemon) shouldRelayPeer(peer *PeerInfo, relayCandidates []*PeerInfo, handshakes map[string]int64) bool {
	return d.shouldRelayPeerWithSubnets(peer, relayCandidates, handshakes, d.getLocalSubnets())
}

func (d *Daemon) shouldRelayPeerWithSubnets(peer *PeerInfo, relayCandidates []*PeerInfo, handshakes map[string]int64, localSubnets []*net.IPNet) bool {
	if d.config.Introducer {
		return false // Introducers are always direct
	}
	if peer.Introducer {
		return false // Don't relay to an introducer
	}
	if hasDiscoveryMethod(peer.DiscoveredVia, LANMethod) {
		return false // LAN peers should stay direct
	}
	if endpointOnAnyLocalSubnet(peer.Endpoint, localSubnets) {
		return false // Local subnet peers should stay direct
	}
	if d.config.ForceRelay {
		return len(relayCandidates) > 0
	}
	if len(relayCandidates) == 0 {
		return false // No relay available
	}

	// Check WG handshake first — if we've had a recent handshake, direct
	// connectivity is confirmed regardless of NAT type or IPv6.
	if handshakes != nil {
		if ts, ok := handshakes[peer.WGPubKey]; ok && ts > 0 {
			lastHandshake := time.Unix(ts, 0)
			if time.Since(lastHandshake) < HandshakeStaleAfter {
				return false // Direct path is working
			}
			// Handshake stale — but only relay if NAT situation warrants it.
			// For cone/unknown NAT or IPv6, the staleness is likely transient
			// (e.g., WG rekey timing). Only relay for symmetric+symmetric.
			if d.localNode.NATType == "symmetric" && peer.NATType == "symmetric" {
				return true
			}
			// For transitive-only peers with stale handshake, relay to avoid blackhole
			if hasDiscoveryMethod(peer.DiscoveredVia, "dht-transitive") &&
				!hasDiscoveryMethod(peer.DiscoveredVia, "dht") {
				return true
			}
			return false
		}
	}

	// No handshake data yet — IPv6 endpoints should try direct first
	if !d.config.DisableIPv6 && isIPv6Endpoint(peer.Endpoint) {
		return false
	}

	// Both sides symmetric → hole-punch is unreliable, relay
	if d.localNode.NATType == "symmetric" && peer.NATType == "symmetric" {
		return true
	}

	// If peer is only reachable via transitive discovery and has no WG handshake yet,
	// prefer relay to avoid prolonged blackholes for NATed peers.
	if handshakes != nil {
		if ts, ok := handshakes[peer.WGPubKey]; !ok || ts == 0 {
			if hasDiscoveryMethod(peer.DiscoveredVia, "dht-transitive") {
				return true
			}
		}
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
					log.Printf("Failed to remove obsolete peer %s...: %v", shortKey(current.PublicKey), err)
				}
				d.appliedMu.Lock()
				delete(d.lastAppliedPeerConfigs, current.PublicKey)
				d.appliedMu.Unlock()
			}
		}
	}

	for pubKey, cfg := range desired {
		if cfg.peer.Endpoint == "" {
			continue
		}
		if d.config.DisableIPv6 && isIPv6Endpoint(cfg.peer.Endpoint) {
			continue
		}
		allowed := mapKeysSorted(cfg.allowed)
		if len(allowed) == 0 {
			continue
		}
		allowedCSV := strings.Join(allowed, ",")
		signature := cfg.peer.Endpoint + "|" + allowedCSV

		// Check-and-mark under the same lock to avoid TOCTOU (W4)
		d.appliedMu.Lock()
		prev, ok := d.lastAppliedPeerConfigs[pubKey]
		if ok && prev == signature {
			d.appliedMu.Unlock()
			continue
		}
		d.lastAppliedPeerConfigs[pubKey] = signature
		d.appliedMu.Unlock()

		if err := wireguard.SetPeer(d.config.InterfaceName, pubKey, d.config.Keys.PSK, cfg.peer.Endpoint, allowedCSV); err != nil {
			// Rollback the optimistic write on failure
			d.appliedMu.Lock()
			delete(d.lastAppliedPeerConfigs, pubKey)
			d.appliedMu.Unlock()
			return fmt.Errorf("failed to configure peer %s: %w", shortKey(pubKey), err)
		}
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

func hasDiscoveryMethod(methods []string, target string) bool {
	for _, m := range methods {
		if m == target {
			return true
		}
	}
	return false
}

func isIPv6Endpoint(endpoint string) bool {
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

func (d *Daemon) getLocalSubnets() []*net.IPNet {
	if d.localSubnetsFn != nil {
		return d.localSubnetsFn()
	}
	return detectLocalSubnets()
}

func detectLocalSubnets() []*net.IPNet {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	out := make([]*net.IPNet, 0)
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				out = append(out, ipNet)
			}
		}
	}

	return out
}

func endpointOnAnyLocalSubnet(endpoint string, subnets []*net.IPNet) bool {
	if endpoint == "" || len(subnets) == 0 {
		return false
	}
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	for _, subnet := range subnets {
		if subnet != nil && subnet.Contains(ip) {
			return true
		}
	}
	return false
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

func (d *Daemon) staleCleanupLoop() {
	ticker := time.NewTicker(StaleCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			removed := d.peerStore.CleanupStale()
			for _, pubKey := range removed {
				if err := d.removePeer(pubKey); err != nil {
					log.Printf("[Peers] Failed to remove stale peer %s: %v", shortKey(pubKey), err)
				}
			}
			if len(removed) > 0 {
				log.Printf("[Peers] Removed %d stale peers", len(removed))
			}
		}
	}
}

func (d *Daemon) healthMonitorLoop() {
	ticker := time.NewTicker(HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.checkPeerHealth()
		}
	}
}

func (d *Daemon) startMeshProbeServer() error {
	if d.localNode == nil || d.localNode.MeshIP == "" {
		return fmt.Errorf("local mesh IP not initialized")
	}

	listenAddrs := []string{net.JoinHostPort(d.localNode.MeshIP, strconv.Itoa(d.healthProbePort))}
	if d.localNode.MeshIPv6 != "" {
		listenAddrs = append(listenAddrs, net.JoinHostPort(d.localNode.MeshIPv6, strconv.Itoa(d.healthProbePort)))
	}

	started := 0
	for _, addr := range listenAddrs {
		ln, err := d.listenProbeOnInterface(addr)
		if err != nil {
			log.Printf("[Health] Probe listener bind failed on %s: %v", addr, err)
			continue
		}
		started++
		d.probeMu.Lock()
		d.probeListeners = append(d.probeListeners, ln)
		d.probeMu.Unlock()
		go d.acceptProbeConnections(ln)
	}

	if started == 0 {
		return fmt.Errorf("unable to bind probe listener")
	}

	go func() {
		<-d.ctx.Done()
		d.probeMu.Lock()
		listeners := d.probeListeners
		d.probeListeners = nil
		d.probeMu.Unlock()
		for _, ln := range listeners {
			_ = ln.Close()
		}
	}()

	log.Printf("[Health] Mesh probe server listening on tcp/%d", d.healthProbePort)
	return nil
}

func (d *Daemon) acceptProbeConnections(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-d.ctx.Done():
				return
			default:
			}
			continue
		}
		go handleProbeConnection(conn)
	}
}

func handleProbeConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		if strings.TrimSpace(line) != "ping" {
			continue
		}
		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if _, err := conn.Write([]byte("pong\n")); err != nil {
			return
		}
	}
}

func (d *Daemon) meshProbeLoop() {
	ticker := time.NewTicker(MeshProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.probePeersOverMesh()
		}
	}
}

func (d *Daemon) probePeersOverMesh() {
	peers := d.peerStore.GetActive()
	activeSet := make(map[string]struct{}, len(peers))
	handshakes, _ := wireguard.GetLatestHandshakes(d.config.InterfaceName)

	for _, p := range peers {
		if p == nil || p.WGPubKey == "" || p.WGPubKey == d.localNode.WGPubKey || p.MeshIP == "" {
			continue
		}
		activeSet[p.WGPubKey] = struct{}{}

		// If WG has a recent handshake, treat the peer as healthy and do not let
		// probe jitter flap routes/AllowedIPs.
		ts := handshakes[p.WGPubKey]
		if ts > 0 && time.Since(time.Unix(ts, 0)) < HandshakeStaleAfter {
			d.clearTemporarilyOffline(p.WGPubKey)
			d.probeMu.Lock()
			d.probeFailures[p.WGPubKey] = 0
			d.probeMu.Unlock()
			d.closeProbeSession(p.WGPubKey)
			continue
		}

		// Avoid evicting brand-new peers too early, but still enforce for relay-routed
		// peers (no direct handshake entry by design) and for stale entries.
		enforce := ts > 0 || d.isRelayRoutedPeer(p.WGPubKey) || time.Since(p.LastSeen) > 45*time.Second
		if !enforce {
			d.probeMu.Lock()
			d.probeFailures[p.WGPubKey] = 0
			d.probeMu.Unlock()
			d.closeProbeSession(p.WGPubKey)
			continue
		}

		if d.probePeer(p) {
			d.clearTemporarilyOffline(p.WGPubKey)
			d.probeMu.Lock()
			d.probeFailures[p.WGPubKey] = 0
			d.probeMu.Unlock()
			continue
		}

		d.probeMu.Lock()
		d.probeFailures[p.WGPubKey]++
		failures := d.probeFailures[p.WGPubKey]
		d.probeMu.Unlock()

		if failures >= MeshProbeFailLimit {
			log.Printf("[Health] Probe failed %d times for %s..., marking temporarily offline", failures, shortKey(p.WGPubKey))
			d.evictPeerFromPool(p)
		}
	}

	d.cleanupProbeSessions(activeSet)
}

func (d *Daemon) probePeer(peer *PeerInfo) bool {
	if peer == nil || peer.WGPubKey == "" {
		return false
	}

	session := d.getOrDialProbeSession(peer)
	if session == nil {
		return false
	}

	_ = session.conn.SetWriteDeadline(time.Now().Add(MeshProbeDialTimeout))
	if _, err := session.conn.Write([]byte("ping\n")); err != nil {
		d.closeProbeSession(peer.WGPubKey)
		return false
	}

	_ = session.conn.SetReadDeadline(time.Now().Add(MeshProbeDialTimeout))
	line, err := session.reader.ReadString('\n')
	if err != nil {
		d.closeProbeSession(peer.WGPubKey)
		return false
	}

	if strings.TrimSpace(line) != "pong" {
		d.closeProbeSession(peer.WGPubKey)
		return false
	}

	return true
}

func (d *Daemon) getOrDialProbeSession(peer *PeerInfo) *peerProbeSession {
	d.probeMu.Lock()
	s := d.probeSessions[peer.WGPubKey]
	d.probeMu.Unlock()
	if s != nil {
		return s
	}

	addrs := []string{net.JoinHostPort(peer.MeshIP, strconv.Itoa(d.healthProbePort))}
	if !d.config.DisableIPv6 && peer.MeshIPv6 != "" {
		addrs = append([]string{net.JoinHostPort(peer.MeshIPv6, strconv.Itoa(d.healthProbePort))}, addrs...)
	}

	for _, addr := range addrs {
		conn, err := d.dialProbeOnInterface(addr)
		if err != nil {
			continue
		}
		session := &peerProbeSession{conn: conn, reader: bufio.NewReader(conn)}
		d.probeMu.Lock()
		d.probeSessions[peer.WGPubKey] = session
		d.probeMu.Unlock()
		return session
	}

	return nil
}

func (d *Daemon) closeProbeSession(pubKey string) {
	d.probeMu.Lock()
	s := d.probeSessions[pubKey]
	delete(d.probeSessions, pubKey)
	d.probeMu.Unlock()
	if s != nil {
		_ = s.conn.Close()
	}
}

func (d *Daemon) cleanupProbeSessions(activeSet map[string]struct{}) {
	d.probeMu.Lock()
	keys := make([]string, 0)
	for pubKey := range d.probeSessions {
		if _, ok := activeSet[pubKey]; !ok {
			keys = append(keys, pubKey)
		}
	}
	d.probeMu.Unlock()

	for _, pubKey := range keys {
		d.closeProbeSession(pubKey)
		d.probeMu.Lock()
		delete(d.probeFailures, pubKey)
		d.probeMu.Unlock()
	}
}

func (d *Daemon) listenProbeOnInterface(addr string) (net.Listener, error) {
	lc := net.ListenConfig{}
	if runtime.GOOS == "linux" && d.config.InterfaceName != "" {
		iface := d.config.InterfaceName
		lc.Control = func(network, address string, c syscall.RawConn) error {
			var sockErr error
			err := c.Control(func(fd uintptr) {
				sockErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, soBindToDevice, iface)
			})
			if err != nil {
				return err
			}
			return sockErr
		}
	}
	return lc.Listen(d.ctx, "tcp", addr)
}

func (d *Daemon) dialProbeOnInterface(addr string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: MeshProbeDialTimeout}
	if local := d.probeLocalAddrForRemote(addr); local != nil {
		dialer.LocalAddr = local
	}
	if runtime.GOOS == "linux" && d.config.InterfaceName != "" {
		iface := d.config.InterfaceName
		dialer.Control = func(network, address string, c syscall.RawConn) error {
			var sockErr error
			err := c.Control(func(fd uintptr) {
				sockErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, soBindToDevice, iface)
			})
			if err != nil {
				return err
			}
			return sockErr
		}
	}
	return dialer.DialContext(d.ctx, "tcp", addr)
}

func (d *Daemon) probeLocalAddrForRemote(remote string) net.Addr {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}
	if ip.To4() != nil {
		if d.localNode != nil && d.localNode.MeshIP != "" {
			return &net.TCPAddr{IP: net.ParseIP(d.localNode.MeshIP)}
		}
		return nil
	}
	if d.localNode != nil && d.localNode.MeshIPv6 != "" {
		return &net.TCPAddr{IP: net.ParseIP(d.localNode.MeshIPv6)}
	}
	return nil
}

func (d *Daemon) checkPeerHealth() {
	handshakes, err := wireguard.GetLatestHandshakes(d.config.InterfaceName)
	if err != nil {
		return
	}
	transfers, err := wireguard.GetPeerTransfers(d.config.InterfaceName)
	if err != nil {
		return
	}

	peers := d.peerStore.GetActive()
	now := time.Now()
	activeSet := make(map[string]struct{}, len(peers))

	for _, p := range peers {
		if p == nil || p.WGPubKey == "" || p.WGPubKey == d.localNode.WGPubKey {
			continue
		}
		activeSet[p.WGPubKey] = struct{}{}

		ts := handshakes[p.WGPubKey]
		transfer := transfers[p.WGPubKey]
		currentTotal := transfer.RxBytes + transfer.TxBytes

		d.healthMu.Lock()
		prevTotal := d.lastPeerTransferTotal[p.WGPubKey]
		d.lastPeerTransferTotal[p.WGPubKey] = currentTotal
		isStale := shouldTreatPeerAsStale(ts, prevTotal, currentTotal, now)
		if isStale {
			d.peerHealthFailures[p.WGPubKey]++
		} else {
			d.peerHealthFailures[p.WGPubKey] = 0
		}
		failures := d.peerHealthFailures[p.WGPubKey]
		d.healthMu.Unlock()

		if !isStale {
			continue
		}

		if failures == 1 {
			d.attemptPeerReconnect(p)
			continue
		}
		if failures >= 2 {
			d.evictPeerFromPool(p)
		}
	}

	d.healthMu.Lock()
	for pubKey := range d.peerHealthFailures {
		if _, ok := activeSet[pubKey]; !ok {
			delete(d.peerHealthFailures, pubKey)
			delete(d.lastPeerTransferTotal, pubKey)
		}
	}
	d.healthMu.Unlock()
}

func shouldTreatPeerAsStale(handshakeTS int64, prevTransferTotal, currentTransferTotal uint64, now time.Time) bool {
	if handshakeTS <= 0 {
		return false
	}
	if now.Sub(time.Unix(handshakeTS, 0)) <= HandshakeStaleAfter {
		return false
	}
	// Transfer counters are cumulative; increase means peer is still active.
	return currentTransferTotal <= prevTransferTotal
}

func (d *Daemon) attemptPeerReconnect(peer *PeerInfo) {
	if peer == nil || peer.WGPubKey == "" {
		return
	}
	log.Printf("[Health] Peer %s... stale handshake >%v with no transfer growth, forcing reconnect", shortKey(peer.WGPubKey), HandshakeStaleAfter)
	if peer.Endpoint == "" {
		return
	}

	allowed := make(map[string]struct{})
	if peer.MeshIP != "" {
		allowed[peer.MeshIP+"/32"] = struct{}{}
	}
	if peer.MeshIPv6 != "" {
		allowed[peer.MeshIPv6+"/128"] = struct{}{}
	}
	for _, route := range peer.RoutableNetworks {
		r := strings.TrimSpace(route)
		if r != "" {
			allowed[r] = struct{}{}
		}
	}
	allowedCSV := strings.Join(mapKeysSorted(allowed), ",")
	if allowedCSV == "" {
		return
	}

	if err := wireguard.SetPeer(d.config.InterfaceName, peer.WGPubKey, d.config.Keys.PSK, peer.Endpoint, allowedCSV); err != nil {
		log.Printf("[Health] Failed to reconnect peer %s...: %v", shortKey(peer.WGPubKey), err)
		return
	}
	d.appliedMu.Lock()
	delete(d.lastAppliedPeerConfigs, peer.WGPubKey)
	d.appliedMu.Unlock()
}

func (d *Daemon) evictPeerFromPool(peer *PeerInfo) {
	if peer == nil || peer.WGPubKey == "" {
		return
	}
	log.Printf("[Health] Evicting unresponsive peer %s... from active pool", shortKey(peer.WGPubKey))
	d.markTemporarilyOffline(peer.WGPubKey)
	d.peerStore.Remove(peer.WGPubKey)
	if err := wireguard.RemovePeer(d.config.InterfaceName, peer.WGPubKey); err != nil {
		log.Printf("[Health] Failed to remove evicted peer %s... from WireGuard: %v", shortKey(peer.WGPubKey), err)
	}
	d.appliedMu.Lock()
	delete(d.lastAppliedPeerConfigs, peer.WGPubKey)
	d.appliedMu.Unlock()
	d.relayMu.Lock()
	delete(d.relayRoutes, peer.WGPubKey)
	d.relayMu.Unlock()
	d.healthMu.Lock()
	delete(d.peerHealthFailures, peer.WGPubKey)
	delete(d.lastPeerTransferTotal, peer.WGPubKey)
	d.healthMu.Unlock()
	d.closeProbeSession(peer.WGPubKey)
	d.probeMu.Lock()
	delete(d.probeFailures, peer.WGPubKey)
	d.probeMu.Unlock()
}

func (d *Daemon) markTemporarilyOffline(pubKey string) {
	if pubKey == "" {
		return
	}
	d.offlineMu.Lock()
	d.temporaryOffline[pubKey] = time.Now().Add(TemporaryOfflineTTL)
	d.offlineMu.Unlock()
}

func (d *Daemon) clearTemporarilyOffline(pubKey string) {
	if pubKey == "" {
		return
	}
	d.offlineMu.Lock()
	delete(d.temporaryOffline, pubKey)
	d.offlineMu.Unlock()
}

func (d *Daemon) isTemporarilyOffline(pubKey string) bool {
	if pubKey == "" {
		return false
	}
	now := time.Now()
	d.offlineMu.Lock()
	until, ok := d.temporaryOffline[pubKey]
	if !ok {
		d.offlineMu.Unlock()
		return false
	}
	if now.After(until) {
		delete(d.temporaryOffline, pubKey)
		d.offlineMu.Unlock()
		return false
	}
	d.offlineMu.Unlock()
	return true
}

func (d *Daemon) isRelayRoutedPeer(pubKey string) bool {
	if pubKey == "" {
		return false
	}
	d.relayMu.RLock()
	_, ok := d.relayRoutes[pubKey]
	d.relayMu.RUnlock()
	return ok
}

// printStatus prints current mesh status
func (d *Daemon) printStatus() {
	peers := d.peerStore.GetActive()
	localSubnets := d.getLocalSubnets()
	d.relayMu.RLock()
	relayRoutes := make(map[string]string, len(d.relayRoutes))
	for target, relay := range d.relayRoutes {
		relayRoutes[target] = relay
	}
	d.relayMu.RUnlock()

	log.Printf("[Status] Active peers: %d", len(peers))
	for _, p := range peers {
		name := p.Hostname
		if name == "" {
			name = shortKey(p.WGPubKey) + "..."
		}
		route := "direct"
		if hasDiscoveryMethod(p.DiscoveredVia, LANMethod) || endpointOnAnyLocalSubnet(p.Endpoint, localSubnets) {
			route = "direct-lan"
		}
		if relayKey, ok := relayRoutes[p.WGPubKey]; ok {
			relayName := shortKey(relayKey) + "..."
			for _, rp := range peers {
				if rp.WGPubKey == relayKey {
					if rp.Hostname != "" {
						relayName = rp.Hostname
					}
					break
				}
			}
			route = "relay:" + relayName
		}
		log.Printf("  - %s (%s) route=%s via %v endpoint=%s", name, p.MeshIP, route, p.DiscoveredVia, p.Endpoint)
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

	log.Printf("Local node: %s...", shortKey(d.localNode.WGPubKey))
	log.Printf("Mesh IP: %s", d.localNode.MeshIP)
	if d.localNode.MeshIPv6 != "" {
		log.Printf("Mesh IPv6: %s", d.localNode.MeshIPv6)
	}
	log.Printf("Network ID: %x (both nodes must show the same ID to find each other)", d.config.Keys.NetworkID[:8])

	// Setup WireGuard interface
	if err := d.setupWireGuard(); err != nil {
		return fmt.Errorf("failed to setup WireGuard: %w", err)
	}
	defer d.teardownWireGuard()
	d.setLocalWGEndpoint()
	if err := d.startMeshProbeServer(); err != nil {
		log.Printf("[Health] Failed to start mesh probe server: %v", err)
	}

	// Restore peers from cache for faster startup
	RestoreFromCache(d.config.InterfaceName, d.peerStore)

	// Start peer cache saver (cancelled via daemon context)
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		StartCacheSaver(d.ctx, d.config.InterfaceName, d.peerStore)
	}()

	// Now create DHT discovery with the initialized local node
	// Import is handled via interface to avoid circular dependency
	dhtFactory := GetDHTDiscoveryFactory()
	if dhtFactory != nil {
		dht, err := dhtFactory(d.ctx, d.config, d.localNode, d.peerStore)
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
		d.epochManager.Start(d.ctx, d.getPrivacyPeers)
		defer d.epochManager.Stop()
		log.Printf("Privacy mode enabled (Dandelion++ relay)")
	}

	// Start RPC server if one is set
	if d.rpcServer != nil {
		if err := d.rpcServer.Start(); err != nil {
			log.Printf("Warning: failed to start RPC server: %v", err)
		} else {
			defer d.rpcServer.Stop()
		}
	}

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start reconciliation loop
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.reconcileLoop()
	}()

	// Start status printer
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.statusLoop()
	}()

	// Periodically remove long-stale peers from memory/cache
	go d.staleCleanupLoop()

	// Monitor WG handshakes/transfer and quickly evict dead peers
	go d.healthMonitorLoop()

	// Keep persistent mesh-VPN health connections to peers
	go d.meshProbeLoop()

	log.Printf("Daemon running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	select {
	case sig := <-sigCh:
		log.Printf("Received signal %v, shutting down...", sig)
	case <-d.ctx.Done():
		log.Printf("Context cancelled, shutting down...")
	}

	d.cancel()

	log.Printf("Waiting for background tasks to complete...")
	d.wg.Wait()
	return nil
}

// DHTDiscoveryFactory is a function type for creating DHT discovery instances.
// The ctx parameter should be the daemon's context so that DHT goroutines are
// cancelled when the daemon shuts down.
type DHTDiscoveryFactory func(ctx context.Context, config *Config, localNode *LocalNode, peerStore *PeerStore) (DiscoveryLayer, error)

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

// SetRPCServer sets the RPC server for the daemon
func (d *Daemon) SetRPCServer(server RPCServer) {
	d.rpcServer = server
}

// GetUptime returns the daemon uptime
func (d *Daemon) GetUptime() time.Duration {
	return time.Since(d.startTime)
}

// GetInterfaceName returns the WireGuard interface name
func (d *Daemon) GetInterfaceName() string {
	return d.config.InterfaceName
}

// GetRPCPeers returns active peers for RPC (converts daemon PeerInfo to RPC PeerData)
func (d *Daemon) GetRPCPeers() []*RPCPeerData {
	peers := d.peerStore.GetActive()
	result := make([]*RPCPeerData, 0, len(peers))
	for _, p := range peers {
		result = append(result, &RPCPeerData{
			WGPubKey:         p.WGPubKey,
			MeshIP:           p.MeshIP,
			Endpoint:         p.Endpoint,
			LastSeen:         p.LastSeen,
			DiscoveredVia:    p.DiscoveredVia,
			RoutableNetworks: p.RoutableNetworks,
		})
	}
	return result
}

// GetRPCPeer returns a single peer for RPC
func (d *Daemon) GetRPCPeer(pubKey string) (*RPCPeerData, bool) {
	peer, exists := d.peerStore.Get(pubKey)
	if !exists {
		return nil, false
	}
	return &RPCPeerData{
		WGPubKey:         peer.WGPubKey,
		MeshIP:           peer.MeshIP,
		Endpoint:         peer.Endpoint,
		LastSeen:         peer.LastSeen,
		DiscoveredVia:    peer.DiscoveredVia,
		RoutableNetworks: peer.RoutableNetworks,
	}, true
}

// GetRPCPeerCounts returns peer counts for RPC
func (d *Daemon) GetRPCPeerCounts() (active, total, dead int) {
	allPeers := d.peerStore.GetAll()
	activePeers := d.peerStore.GetActive()
	total = len(allPeers)
	active = len(activePeers)
	dead = total - active
	return
}

// GetRPCStatus returns daemon status for RPC
func (d *Daemon) GetRPCStatus() *RPCStatusData {
	if d.localNode == nil {
		// Return nil if local node is not initialized yet
		return nil
	}
	return &RPCStatusData{
		MeshIP:    d.localNode.MeshIP,
		PubKey:    d.localNode.WGPubKey,
		Uptime:    d.GetUptime(),
		Interface: d.config.InterfaceName,
	}
}

// RPCPeerData represents peer info for RPC (matches rpc.PeerData)
type RPCPeerData struct {
	WGPubKey         string
	MeshIP           string
	Endpoint         string
	LastSeen         time.Time
	DiscoveredVia    []string
	RoutableNetworks []string
}

// RPCStatusData represents daemon status for RPC (matches rpc.StatusData)
type RPCStatusData struct {
	MeshIP    string
	PubKey    string
	Uptime    time.Duration
	Interface string
}
