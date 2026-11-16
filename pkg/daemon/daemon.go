package daemon

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/node"
	"github.com/atvirokodosprendimai/wgmesh/pkg/privacy"
	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

const (
	ReconcileInterval        = 5 * time.Second
	StatusInterval           = 30 * time.Second
	RelayCandidateMaxAge     = 90 * time.Second
	StaleCleanupInterval     = 1 * time.Minute
	HealthCheckInterval      = 20 * time.Second
	HandshakeStaleAfter      = 150 * time.Second
	MeshProbeInterval        = 1 * time.Second
	MeshProbeDialTimeout     = 1200 * time.Millisecond // Increased from 800ms for cross-DC tolerance
	MeshProbeFailLimit       = 8
	MeshProbePortOffset      = 2000
	TemporaryOfflineTTL      = 30 * time.Second
	soBindToDevice           = 25 // Linux SO_BINDTODEVICE
	RelayHysteresisThreshold = 3  // Require 3 consecutive stable cycles before switching relay→direct
)

type peerProbeSession struct {
	conn   net.Conn
	reader *bufio.Reader
}

// Daemon manages the mesh node lifecycle
type Daemon struct {
	stateFile          string // local node state file, set by initLocalNode
	config             *Config
	localNode          *LocalNode
	peerStore          *PeerStore
	relayRoutes        map[string]string // target pubkey -> relay pubkey
	relayMu            sync.RWMutex
	directStableCycles map[string]int // pubkey -> consecutive cycles with working direct path (relay hysteresis)
	localSubnetsFn     func() []*net.IPNet
	// reconcileMu serialises reconcile: the ticker and handleSIGHUP call it.
	// Lock order of the daemon, outermost first: reconcileMu, configMu (reconcile
	// reads config through the RLock accessors), convergeMu, collisionMu (collision
	// resolution runs inside reconcile), then the leaf locks (LocalNode stateMu,
	// endpointMu, routesMu; Config logMu; relayMu); nothing holding configMu calls reconcile.
	reconcileMu sync.Mutex
	// convergeMu serialises every write to the WireGuard peers of the kernel
	// (see converge.go) and guards pushedEndpoint, lastResults,
	// observeErrLogged and the forwarding target's state (forwardScopeWarned,
	// forwardNoIptables, forwardObserveErr, ruleOwner, ipForwardOwner, revertWarned) and the route target's state
	// (routeObserveErr, routeWarned, hostDefaultWarned, hostDefaultReadErr, hostDefaults,
	// darwinWarned). Lock order: reconcileMu, then
	// convergeMu; the health paths (attemptPeerReconnect, evictPeerFromPool,
	// staleCleanupLoop) take only convergeMu; never the reverse.
	// configuredMu guards configured, the peers the last apply left in the
	// kernel (isConfiguredPeer reads it from the probe goroutine; written only
	// under convergeMu, so the lock order is convergeMu, then configuredMu).
	configuredMu     sync.RWMutex
	configured       map[string]struct{}
	convergeMu       sync.Mutex
	pushedEndpoint   map[string]string // peer key -> endpoint last pushed with wg set
	lastResults      map[string]string // item -> last failure text, for log on change
	observeErrLogged string            // last observe error text, for log once
	// Forwarding target (converge.go): each is logged once per change.
	forwardScopeWarned bool   // the node is forced to forward without needing to (rule forward-scope)
	forwardNoIptables  bool   // "iptables missing" was logged
	forwardObserveErr  string // last forwarding observe error text
	// Ownership of what this run wrote for forwarding (cleared only by
	// revertKernel), and the revert's logged-once flag (converge.go revertKernel).
	ruleOwner, ipForwardOwner forwardOwner
	revertWarned              bool
	// Address target (converge.go): logged once per change.
	addressObserveErr string              // last address observe error text
	addressWarned     map[string]struct{} // stale addresses warned about that still hold (rule stale-address)
	// Route target (converge.go, desired.go): each is logged once per change.
	routeObserveErr    map[string]string    // target -> last observe error text of that family
	routeWarned        map[string]struct{}  // rule cases that desire warned about and that still hold
	hostDefaultWarned  map[string]struct{}  // families whose host default route was warned about
	darwinWarned       map[string]struct{}  // macOS: kind|family|text of the cases warned about that still hold
	darwinInstalled    map[string]struct{}  // macOS: canonical networks this daemon run installed on the mesh interface
	hostDefaultReadErr map[string]string    // family -> last error text of reading the host default route
	hostDefaults       map[string]hostRoute // family -> the host default route seen before the mesh one replaced it (restored when the mesh default is removed)
	// Seams of the kernel and the log, set at construction; nil means the
	// real wireguard functions and log.Printf (see converge.go).
	setPeerFn    func(iface, pubKey string, psk [32]byte, endpoint, allowedIPs string) error
	removePeerFn func(iface, pubKey string) error
	dumpFn       func(iface string) (wireguard.DumpPeers, error)
	logFn        func(format string, args ...any)
	// readFileFn reads /proc for the forwarding target; goos is the OS the
	// desired state is computed for. nil / "" mean os.ReadFile and runtime.GOOS.
	readFileFn func(path string) ([]byte, error)
	goos       string
	// sharedLogMu guards sharedLogged, the last logged owner of each shared network.
	sharedLogMu           sync.Mutex
	sharedLogged          map[string]string
	peerHealthFailures    map[string]int
	lastPeerTransferTotal map[string]uint64
	healthMu              sync.Mutex
	healthProbePort       int
	probeMu               sync.Mutex
	probeSessions         map[string]*peerProbeSession
	probeFailures         map[string]int
	probeListeners        []net.Listener
	offlineMu             sync.Mutex
	temporaryOffline      map[string]time.Time

	// relayRouteSince holds the time each peer entered relayRoutes (T6). It is
	// guarded by relayMu, created on the first write (a &Daemon{...} literal
	// has it nil) and written only through storeRelayPlan and
	// evictPeerFromPool.
	relayRouteSince map[string]time.Time

	// clock is the daemon's time source; nil means time.Now (see now). It is
	// set before the loops start and never written after, so every goroutine
	// may read it.
	clock func() time.Time
	// resumeCh carries the resume loop's request for a reconcile to
	// reconcileLoop (capacity 1, non-blocking send); nil is never ready.
	resumeCh chan struct{}

	// configMu guards the hot-reloadable fields in config (AdvertiseRoutes,
	// AcceptRoutes). The local node guards itself: stateMu, endpointMu and
	// routesMu inside pkg/node. Callers that read AdvertiseRoutes at runtime
	// must hold at least a read lock; SIGHUP reload holds the write lock.
	// LogLevel is not under configMu: Config guards it itself (GetLogLevel).
	configMu sync.RWMutex

	// collisionMu serialises CheckAndResolveCollisions: the reconcile loop and a
	// SIGHUP reload both reach it, and it reads the mesh IP, derives a new one and
	// sets it.
	collisionMu sync.Mutex

	// layersMu guards dhtDiscovery and rpcServer: the setters may be called
	// while Run reads them.
	layersMu sync.Mutex

	// Discovery layer (DHT discovery will be attached)
	dhtDiscovery DiscoveryLayer

	// RPC server
	rpcServer RPCServer

	// startTime is the daemon start, nil until set; the RPC goroutines read it
	// and the first GetUptime call may set it. A pointer keeps the monotonic
	// clock reading of time.Now, so an NTP step does not change the uptime.
	startTime atomic.Pointer[time.Time]

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// RPCServer interface for the RPC server
type RPCServer interface {
	Start() error
	Stop() error
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
		config:                config,
		peerStore:             NewPeerStore(),
		relayRoutes:           make(map[string]string),
		directStableCycles:    make(map[string]int),
		localSubnetsFn:        detectLocalSubnets,
		peerHealthFailures:    make(map[string]int),
		lastPeerTransferTotal: make(map[string]uint64),
		healthProbePort:       int(config.Keys.GossipPort) + MeshProbePortOffset,
		probeSessions:         make(map[string]*peerProbeSession),
		probeFailures:         make(map[string]int),
		temporaryOffline:      make(map[string]time.Time),
		resumeCh:              make(chan struct{}, 1),
		ctx:                   ctx,
		cancel:                cancel,
	}

	d.peerStore.SetPolicy(d.admitPolicy())
	d.peerStore.SetLivenessDisagreeHook(recordLivenessDisagree)

	return d, nil
}

// admitPolicy is what the daemon lends the peer store: the local claim, read
// when a decision is made, the derivation of a key's mesh IP, and the warning counter.
func (d *Daemon) admitPolicy() *Policy {
	return &Policy{
		Local: func() (key, ip string) {
			if d.localNode == nil {
				return "", ""
			}
			return d.localNode.Identity()
		},
		MeshNet:      d.meshNet(),
		Derive:       d.deriveMeshIP,
		OnWarn:       RecordAdmitWarning,
		AcceptRoutes: d.acceptRoutes,
	}
}

// meshNet is the mesh network: the custom subnet, or 10.<b0>.0.0/16 from the
// first byte Keys.MeshSubnet holds (the default derivation fills the other
// two octets of every address).
func (d *Daemon) meshNet() *net.IPNet {
	if s := d.config.CustomSubnet; s != nil {
		return &net.IPNet{IP: s.IP.Mask(s.Mask), Mask: s.Mask}
	}
	return &net.IPNet{IP: net.IPv4(10, d.config.Keys.MeshSubnet[0], 0, 0).To4(), Mask: net.CIDRMask(16, 32)}
}

// deriveMeshIP is the address a key derives for a nonce: 0 is the natural
// derivation, 1 and up the collision-nonce one (a different hash input, see
// DeriveMeshIPWithNonce). It returns "" when the custom subnet is unusable.
func (d *Daemon) deriveMeshIP(pubKey string, nonce int) string {
	secret, subnet := d.config.Secret, d.config.CustomSubnet
	if subnet != nil {
		var ip string
		var err error
		if nonce == 0 {
			ip, err = crypto.DeriveMeshIPInSubnet(subnet, pubKey, secret)
		} else {
			ip, err = crypto.DeriveMeshIPInSubnetWithNonce(subnet, pubKey, secret, nonce)
		}
		if err != nil {
			return ""
		}
		return ip
	}
	if nonce == 0 {
		return crypto.DeriveMeshIP(d.config.Keys.MeshSubnet, pubKey, secret)
	}
	return DeriveMeshIPWithNonce(d.config.Keys.MeshSubnet, pubKey, secret, nonce)
}

// SetDHTDiscovery sets the DHT discovery layer
func (d *Daemon) SetDHTDiscovery(dht DiscoveryLayer) {
	d.layersMu.Lock()
	defer d.layersMu.Unlock()
	d.dhtDiscovery = dht
}

func (d *Daemon) dhtLayer() DiscoveryLayer {
	d.layersMu.Lock()
	defer d.layersMu.Unlock()
	return d.dhtDiscovery
}

func (d *Daemon) rpcLayer() RPCServer {
	d.layersMu.Lock()
	defer d.layersMu.Unlock()
	return d.rpcServer
}

// Run starts the daemon and blocks until stopped
func (d *Daemon) Run() error {
	d.markStarted()
	log.Printf("Starting wgmesh daemon...")

	// Load or create local node
	if err := d.initLocalNode(); err != nil {
		return fmt.Errorf("failed to initialize local node: %w", err)
	}

	log.Printf("Local node: %s...", shortKey(d.localNode.WGPubKey))
	log.Printf("Mesh IP: %s", d.localNode.GetMeshIP())
	if ip6 := d.localNode.GetMeshIPv6(); ip6 != "" {
		log.Printf("Mesh IPv6: %s", ip6)
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
	if dht := d.dhtLayer(); dht != nil {
		if err := dht.Start(); err != nil {
			return fmt.Errorf("failed to start DHT discovery: %w", err)
		}
		defer dht.Stop()
	}

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

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

	// Notice a wake from sleep (#829)
	d.startResumeLoop()

	log.Printf("Daemon running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	for {
		select {
		case sig := <-sigCh:
			if sig == syscall.SIGHUP {
				log.Printf("Received SIGHUP, reloading configuration...")
				d.handleSIGHUP()
				continue
			}
			log.Printf("Received signal %v, shutting down...", sig)
		case <-d.ctx.Done():
			log.Printf("Context cancelled, shutting down...")
		}
		break
	}

	d.cancel()
	log.Printf("Waiting for background tasks to complete...")
	d.wg.Wait()
	d.revertKernel()
	return nil
}

// Shutdown cancels the daemon context, signalling background goroutines
// to stop. Callers that need to wait for full shutdown completion should
// wait for Run() or RunWithDHTDiscovery() to return.
func (d *Daemon) Shutdown() {
	d.cancel()
}

// MeshIPKept returns true when a persisted mesh IP may be kept. It is the one
// rule for the daemon and for service registration.
//
// With an explicit --mesh-subnet the address must lie inside it, so changing
// the subnet moves the node. Without one the persisted address came from this
// node's own earlier derivation (default /16 from a secret, possibly an older
// secret, or a --mesh-subnet used on an earlier run) and stays authoritative:
// the operator changed an option, not the address, and peers, DNS and firewall
// rules refer to it (issue #827). Any usable unicast IPv4 address is kept.
func MeshIPKept(meshIP string, customSubnet *net.IPNet) bool {
	ip := net.ParseIP(meshIP)
	if ip == nil || ip.To4() == nil {
		return false
	}
	if customSubnet != nil {
		return customSubnet.Contains(ip)
	}
	// Without a subnet any usable unicast address is kept; addresses that
	// cannot be a node address are re-derived.
	return !ip.IsUnspecified() && !ip.IsLoopback() && !ip.IsMulticast() && !ip.Equal(net.IPv4bcast)
}

func meshIPInSubnet(meshIP string, cfg *Config) bool {
	return MeshIPKept(meshIP, cfg.CustomSubnet)
}

// announceableHostname returns hostname if peers will accept it in an
// announcement (printable ASCII, at most crypto.MaxHostnameLength bytes, as
// checked by PeerAnnouncement.Validate); otherwise it logs and returns "" so
// the node is announced without a hostname instead of being dropped.
func announceableHostname(hostname string) string {
	valid := len(hostname) <= crypto.MaxHostnameLength
	for i := 0; valid && i < len(hostname); i++ {
		valid = hostname[i] >= 32 && hostname[i] <= 126
	}
	if !valid {
		log.Printf("Warning: hostname %q is not printable ASCII or is too long; announcing without a hostname", hostname)
		return ""
	}
	return hostname
}

// localNodeStateDir holds the persisted local node state; a variable so tests
// can point initLocalNode at a temp directory.
// Tests that override it must not run in parallel.
var localNodeStateDir = "/var/lib/wgmesh"

// initLocalNode loads or creates the local WireGuard node
func (d *Daemon) initLocalNode() error {
	hostname, hostErr := os.Hostname()
	if hostErr != nil {
		hostname = ""
	}
	hostname = announceableHostname(hostname)

	// Try to load existing key from state file
	d.stateFile = filepath.Join(localNodeStateDir, d.config.InterfaceName+".json")
	stateFile := d.stateFile
	quarantined := false
	node, err := loadLocalNode(stateFile)
	if err != nil {
		switch {
		case errors.Is(err, ErrStateMissing):
			// No identity yet: generate one below.
		case errors.Is(err, ErrStateCorrupt):
			if RefuseCorruptState {
				return fmt.Errorf("local node state file %s is corrupt and RefuseCorruptState is set, so it was left untouched; to start, move the file away (a new identity is then created) or restore a good copy of it, then restart: %w", stateFile, err)
			}
			moved, qerr := quarantineState(stateFile)
			if qerr != nil {
				return fmt.Errorf("local node state file %s is corrupt and could not be moved aside: %w", stateFile, qerr)
			}
			quarantined = true
			log.Printf("Local node state file %s is corrupt and was replaced by a new identity (new key, new mesh IP; peers will see a new node). The old file is kept at %s; to restore it, stop the daemon and rename that file back to %s", stateFile, moved, stateFile)
		default:
			return fmt.Errorf("failed to read local node state file %s: %w", stateFile, err)
		}
	}
	// On the missing and quarantined paths err stays non-nil, and that is what
	// skips this load and falls through to generating a new identity.
	if err == nil && node != nil {
		d.localNode = node

		// Use the persisted mesh IP when it is present and falls within the
		// expected subnet. Re-derive only when the field is absent (old state
		// file) or the configured subnet has changed.
		if cur := node.GetMeshIP(); cur != "" && meshIPInSubnet(cur, d.config) {
			// Persisted IP is valid — keep it so that secret rotation does not
			// change the node's address.
		} else {
			if d.config.CustomSubnet != nil {
				ip, err := crypto.DeriveMeshIPInSubnet(d.config.CustomSubnet, d.localNode.WGPubKey, d.config.Secret)
				if err != nil {
					return fmt.Errorf("failed to derive mesh IP in custom subnet: %w", err)
				}
				d.localNode.SetMeshIP(ip)
			} else {
				d.localNode.SetMeshIP(crypto.DeriveMeshIP(d.config.Keys.MeshSubnet, d.localNode.WGPubKey, d.config.Secret))
			}
			// Persist the newly derived IP so subsequent starts reuse it.
			if err := saveLocalNode(stateFile, d.localNode); err != nil {
				log.Printf("Warning: failed to save local node state: %v", err)
			}
		}

		// Always re-derive IPv6; IPv6 addresses are not globally routable and
		// subnet pinning is not applicable for the /64 ULA prefix.
		if node.GetMeshIPv6() == "" {
			d.localNode.SetMeshIPv6(crypto.DeriveMeshIPv6(d.config.Keys.MeshPrefixV6, d.localNode.WGPubKey, d.config.Secret))
			if err := saveLocalNode(stateFile, d.localNode); err != nil {
				log.Printf("Warning: failed to save local node state: %v", err)
			}
		}

		d.localNode.SetRoutableNetworks(d.config.AdvertiseRoutes)
		d.localNode.SetIntroducer(d.config.Introducer)
		d.localNode.Hostname = hostname
		return nil
	}

	// Generate new keypair
	privateKey, publicKey, err := wireguard.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate keypair: %w", err)
	}

	// Derive mesh IP from public key
	var meshIP string
	if d.config.CustomSubnet != nil {
		ip, err := crypto.DeriveMeshIPInSubnet(d.config.CustomSubnet, publicKey, d.config.Secret)
		if err != nil {
			return fmt.Errorf("failed to derive mesh IP in custom subnet: %w", err)
		}
		meshIP = ip
	} else {
		meshIP = crypto.DeriveMeshIP(d.config.Keys.MeshSubnet, publicKey, d.config.Secret)
	}
	meshIPv6 := crypto.DeriveMeshIPv6(d.config.Keys.MeshPrefixV6, publicKey, d.config.Secret)

	d.localNode = NewLocalNode(publicKey, privateKey, hostname, meshIP, meshIPv6)
	d.localNode.SetRoutableNetworks(d.config.AdvertiseRoutes)
	d.localNode.SetIntroducer(d.config.Introducer)

	// Save to state file
	if err := saveLocalNode(stateFile, d.localNode); err != nil {
		if quarantined {
			// The old identity was moved aside: running on a key that is not on
			// disk would create another new identity at every start.
			return fmt.Errorf("the new identity could not be saved to %s after the corrupt file was moved aside: %w", stateFile, err)
		}
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

	// Set IP address with correct prefix length
	if err := setInterfaceAddress(d.config.InterfaceName, fmt.Sprintf("%s/%d", d.localNode.GetMeshIP(), d.config.PrefixLen())); err != nil {
		return fmt.Errorf("failed to set IP address: %w", err)
	}
	if ip6 := d.localNode.GetMeshIPv6(); ip6 != "" {
		if err := setInterfaceAddress(d.config.InterfaceName, fmt.Sprintf("%s/%d", ip6, meshPrefixLenV6)); err != nil {
			// IPv6 may be disabled by sysctl; the IPv4 mesh still works.
			log.Printf("Warning: failed to set mesh IPv6 address, continuing IPv4-only: %v", err)
			d.localNode.SetMeshIPv6("")
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
		case <-d.resumeCh:
			d.reconcile()
		}
	}
}

// reconcile updates WireGuard configuration based on discovered peers
func (d *Daemon) reconcile() {
	d.reconcileMu.Lock()
	defer d.reconcileMu.Unlock()
	start := time.Now()

	// An unreadable kernel skips the tick: no relay state, peers or routes are
	// decided from nothing. The tick records the handshakes as evidence first,
	// so a peer whose only sign of life is a handshake is already Alive when
	// the active set is read (ADR-0004), and stores the relay plan before it
	// applies it.
	d.convergeTick()

	// Check for mesh IP collisions
	d.CheckAndResolveCollisions()

	// Stale peer cleanup is handled by staleCleanupLoop (W3: avoid double cleanup)

	// Update Prometheus metrics
	UpdateMetrics(d)
	ObserveReconcileDuration(start)
}

type desiredPeerConfig struct {
	peer      *PeerInfo
	allowed   map[string]struct{}
	probeOnly bool // configured with its endpoint and no AllowedIPs: a relayed peer (#852)
}

func (d *Daemon) buildDesiredPeerConfigsWithHandshakes(peers []*PeerInfo, handshakes map[string]int64) (map[string]*desiredPeerConfig, map[string]string, map[string]int) {
	ds := d.desire(peers, handshakes, map[string]string{})
	return ds.Peers, ds.relayRoutes, ds.directStable
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

	now := d.now()
	// Check WG handshake first — if we've had a recent handshake, direct
	// connectivity is confirmed regardless of NAT type or IPv6.
	if handshakes != nil {
		if ts, ok := handshakes[peer.WGPubKey]; ok && ts > 0 {
			lastHandshake := time.Unix(ts, 0)
			if now.Sub(lastHandshake) < HandshakeStaleAfter {
				return false // Direct path is working
			}
			// Handshake is stale (> HandshakeStaleAfter). Fall back to relay so
			// traffic is not blacked out while WireGuard attempts to re-establish.
			//
			// HandshakeStaleAfter (150 s) is far longer than any WG rekey window
			// (< 5 s), so a stale timestamp reliably indicates a real connectivity
			// failure — not a transient rekey.
			//
			// The second guard previously excluded peers discovered via "dht" (added
			// when a control-path UDP exchange succeeds without a WG handshake).
			// Removing it means the confirmed WireGuard handshake — not the UDP
			// control exchange — is the sole gate for dropping relay.
			//
			// The relay→direct hysteresis (RelayHysteresisThreshold cycles) will
			// transition back to direct once a fresh handshake is confirmed.
			// A wake makes the handshake age meaningless (it includes the time
			// asleep): for ResumeGrace the stale handshake does not choose the
			// relay, and the decision falls through the cases below with the
			// handshake entry still present, so the dht-transitive case, which
			// needs no entry, does not apply either.
			if !d.resumeGraceBlocks(now) {
				return true
			}
		}
	}

	// No handshake data yet — IPv6 endpoints should try direct first
	if !d.config.DisableIPv6 && isIPv6Endpoint(peer.Endpoint) {
		return false
	}

	// Both sides symmetric → hole-punch is unreliable, relay
	if d.localNode.GetNATType() == "symmetric" && peer.NATType == "symmetric" {
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

// hasUsableEndpoint reports whether the apply step can send p to WireGuard: it
// has an endpoint, and not an IPv6 one while IPv6 is disabled. The desired set
// holds a probe-only entry only for such a peer, since the apply step skips
// the others and a skipped entry would never be removed.
func (d *Daemon) hasUsableEndpoint(p *PeerInfo) bool {
	if p.Endpoint == "" {
		return false
	}
	return !(d.config.DisableIPv6 && isIPv6Endpoint(p.Endpoint))
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
			d.staleCleanupTick()
		}
	}
}

// staleCleanupTick removes stale peers from the store and from WireGuard, then
// drops the temporaryOffline entries past their deadline.
func (d *Daemon) staleCleanupTick() {
	removed := d.peerStore.CleanupStale()
	for _, pubKey := range removed {
		if err := d.removePeerFromKernel(pubKey); err != nil {
			log.Printf("[Peers] Failed to remove stale peer %s: %v", shortKey(pubKey), err)
		}
	}
	if len(removed) > 0 {
		log.Printf("[Peers] Removed %d stale peers", len(removed))
	}
	d.sweepTemporaryOffline()
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
	if d.localNode == nil {
		return fmt.Errorf("local mesh IP not initialized")
	}
	meshIP, ip6 := d.localNode.GetMeshIP(), d.localNode.GetMeshIPv6()
	if meshIP == "" {
		return fmt.Errorf("local mesh IP not initialized")
	}

	listenAddrs := []string{net.JoinHostPort(meshIP, strconv.Itoa(d.healthProbePort))}
	if ip6 != "" {
		listenAddrs = append(listenAddrs, net.JoinHostPort(ip6, strconv.Itoa(d.healthProbePort)))
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

// probeMaxLineBytes bounds a single probe request line. The protocol only
// sends "ping\n"; anything longer without a newline is rejected so a peer
// cannot make the server buffer unbounded data.
const probeMaxLineBytes = 64

func handleProbeConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReaderSize(conn, probeMaxLineBytes)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		line, err := reader.ReadSlice('\n')
		if err != nil {
			return // includes bufio.ErrBufferFull for oversize lines
		}
		if strings.TrimSpace(string(line)) != "ping" {
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
	// Suspect peers are visited only while configured or relay-routed; peers
	// kept Alive by evidence are visited too. A pong makes a peer Alive again.
	peers := d.peerStore.GetProbeable()
	now := d.now()
	activeSet := make(map[string]struct{}, len(peers))
	// Up to ReconcileInterval (5 s) old: ts is compared with HandshakeStaleAfter (150 s) below.
	st, _ := wireguard.ReadState(d.config.InterfaceName, ReconcileInterval)
	handshakes := st.Handshakes()

	for _, p := range peers {
		if p == nil || p.WGPubKey == "" || p.WGPubKey == d.localNode.WGPubKey || p.MeshIP == "" {
			continue
		}
		// A Suspect peer that reconcile already removed from the interface cannot
		// answer through the tunnel: it returns by an announcement or a handshake.
		if p.Alive(now) != node.Alive && !d.isConfiguredPeer(p.WGPubKey) && !d.isRelayRoutedPeer(p.WGPubKey) {
			continue
		}
		activeSet[p.WGPubKey] = struct{}{}

		// If WG has a recent handshake, treat the peer as healthy and do not let
		// probe jitter flap routes/AllowedIPs.
		ts := handshakes[p.WGPubKey]
		if ts > 0 && now.Sub(time.Unix(ts, 0)) < HandshakeStaleAfter {
			d.clearTemporarilyOffline(p.WGPubKey)
			d.probeMu.Lock()
			d.probeFailures[p.WGPubKey] = 0
			d.probeMu.Unlock()
			d.closeProbeSession(p.WGPubKey)
			continue
		}

		// Avoid evicting brand-new peers too early, but still enforce for relay-routed
		// peers (no direct handshake entry by design) and for stale entries.
		enforce := ts > 0 || d.isRelayRoutedPeer(p.WGPubKey) || DecideByLiveness(SiteProbeEnforce, p, now, func(age time.Duration) bool { return age > 45*time.Second })
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
			// Inside the wake grace the failures are an artefact of the sleep:
			// start counting again instead of evicting.
			if d.resumeGraceBlocks(now) {
				d.probeMu.Lock()
				d.probeFailures[p.WGPubKey] = 0
				d.probeMu.Unlock()
				continue
			}
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
	start := time.Now()
	if _, err := session.conn.Write([]byte("ping\n")); err != nil {
		d.closeProbeSession(peer.WGPubKey)
		return false
	}

	_ = session.conn.SetReadDeadline(time.Now().Add(MeshProbeDialTimeout))
	reply, err := readProbeReply(session.reader)
	if err != nil {
		d.closeProbeSession(peer.WGPubKey)
		return false
	}

	if reply != "pong" {
		d.closeProbeSession(peer.WGPubKey)
		return false
	}

	rtt := time.Since(start)
	ObserveProbeRTT(peer.WGPubKey[:8], start)
	ObserveProbeRTTSummary(peer.WGPubKey[:8], rtt)
	d.peerStore.SetLatency(peer.WGPubKey, rtt)
	d.peerStore.ObserveDataPlane(peer.WGPubKey, time.Now())
	return true
}

// readProbeReply reads one probe reply line through a reader sized to
// probeMaxLineBytes. A responder that never sends a newline fails with
// bufio.ErrBufferFull instead of making the prober buffer unbounded data.
func readProbeReply(r *bufio.Reader) (string, error) {
	line, err := r.ReadSlice('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(line)), nil
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
		session := &peerProbeSession{conn: conn, reader: bufio.NewReaderSize(conn, probeMaxLineBytes)}
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
	// A failure count can outlive its peer without a session: a pass that took
	// its snapshot before removePeer writes the count after removePeer deleted it.
	for pubKey := range d.probeFailures {
		if _, ok := activeSet[pubKey]; !ok {
			delete(d.probeFailures, pubKey)
		}
	}
	d.probeMu.Unlock()

	for _, pubKey := range keys {
		d.closeProbeSession(pubKey)
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
		if d.localNode != nil {
			if ip4 := d.localNode.GetMeshIP(); ip4 != "" {
				return &net.TCPAddr{IP: net.ParseIP(ip4)}
			}
		}
		return nil
	}
	if d.localNode != nil {
		if ip6 := d.localNode.GetMeshIPv6(); ip6 != "" {
			return &net.TCPAddr{IP: net.ParseIP(ip6)}
		}
	}
	return nil
}

func (d *Daemon) checkPeerHealth() {
	// Up to ReconcileInterval (5 s) old: the handshake is compared with HandshakeStaleAfter (150 s) in shouldTreatPeerAsStale.
	st, err := wireguard.ReadState(d.config.InterfaceName, ReconcileInterval)
	if err != nil {
		return
	}
	handshakes := make(map[string]int64, len(st.Peers))
	transfers := make(map[string]wireguard.PeerTransfer, len(st.Peers))
	for _, sp := range st.Peers {
		handshakes[sp.PublicKey] = sp.LatestHandshake
		transfers[sp.PublicKey] = wireguard.PeerTransfer{RxBytes: sp.RxBytes, TxBytes: sp.TxBytes}
	}
	d.evaluatePeerHealth(handshakes, transfers)
}

// evaluatePeerHealth takes the health decisions of one cycle from the
// handshake and transfer readings; checkPeerHealth reads them from WireGuard.
func (d *Daemon) evaluatePeerHealth(handshakes map[string]int64, transfers map[string]wireguard.PeerTransfer) {
	peers := d.peerStore.GetActive()
	now := d.now()
	activeSet := make(map[string]struct{}, len(peers))

	for _, p := range peers {
		if p == nil || p.WGPubKey == "" || p.WGPubKey == d.localNode.WGPubKey {
			continue
		}
		activeSet[p.WGPubKey] = struct{}{}

		// A relay-routed peer has no handshake of its own and a probe-only peer
		// is kept on the interface without AllowedIPs only to handshake: a
		// reconnect would give it its mesh IP and take the address from the
		// relay, an eviction would drop it. Neither is judged here.
		if d.isRelayRoutedPeer(p.WGPubKey) {
			d.healthMu.Lock()
			delete(d.peerHealthFailures, p.WGPubKey)
			delete(d.lastPeerTransferTotal, p.WGPubKey)
			d.healthMu.Unlock()
			continue
		}

		ts := handshakes[p.WGPubKey]
		transfer := transfers[p.WGPubKey]
		currentTotal := transfer.RxBytes + transfer.TxBytes

		d.healthMu.Lock()
		prevTotal := d.lastPeerTransferTotal[p.WGPubKey]
		d.lastPeerTransferTotal[p.WGPubKey] = currentTotal
		isStale := shouldTreatPeerAsStale(ts, prevTotal, currentTotal, now)
		// Inside the wake grace the handshake age includes the time asleep: not
		// stale, so neither the reconnect nor the eviction below happens.
		if isStale && d.resumeGraceBlocks(now) {
			isStale = false
		}
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
	if d.isRelayRoutedPeer(peer.WGPubKey) {
		return // the relay carries its traffic; see evaluatePeerHealth
	}
	log.Printf("[Health] Peer %s... stale handshake >%v with no transfer growth, forcing reconnect", shortKey(peer.WGPubKey), HandshakeStaleAfter)
	if peer.Endpoint == "" {
		return
	}

	if err := d.reconnectPeerInKernel(peer, d.peerStore.GetActive()); err != nil {
		log.Printf("[Health] Failed to reconnect peer %s...: %v", shortKey(peer.WGPubKey), err)
	}
}

func (d *Daemon) evictPeerFromPool(peer *PeerInfo) {
	if peer == nil || peer.WGPubKey == "" {
		return
	}
	log.Printf("[Health] Evicting unresponsive peer %s... from active pool; hidden from mentions for %s unless it speaks first", shortKey(peer.WGPubKey), TombstoneTTL)
	d.markTemporarilyOffline(peer.WGPubKey)
	d.peerStore.Withdraw(peer.WGPubKey, "evicted")
	if err := d.removePeerFromKernel(peer.WGPubKey); err != nil {
		log.Printf("[Health] Failed to remove evicted peer %s... from WireGuard: %v", shortKey(peer.WGPubKey), err)
	}
	d.relayMu.Lock()
	delete(d.relayRoutes, peer.WGPubKey)
	delete(d.directStableCycles, peer.WGPubKey)
	delete(d.relayRouteSince, peer.WGPubKey)
	relayRouteAge.DeleteLabelValues(peerLabel(peer.WGPubKey))
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
	d.temporaryOffline[pubKey] = d.now().Add(TemporaryOfflineTTL)
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
	now := d.now()
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

// sweepTemporaryOffline deletes the entries past their deadline without a
// lookup, so a key never asked about again does not stay (ADR-0007 Decision 4).
func (d *Daemon) sweepTemporaryOffline() {
	now := d.now() // the daemon clock, as markTemporarilyOffline and isTemporarilyOffline read it
	d.offlineMu.Lock()
	defer d.offlineMu.Unlock()
	for k, until := range d.temporaryOffline {
		if now.After(until) {
			delete(d.temporaryOffline, k)
		}
	}
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

// isConfiguredPeer reports whether the last apply left the peer in the kernel
// (a successful set and no later removal).
func (d *Daemon) isConfiguredPeer(pubKey string) bool {
	d.configuredMu.RLock()
	_, ok := d.configured[pubKey]
	d.configuredMu.RUnlock()
	return ok
}

// setConfigured records that the peer is in the kernel or no longer is.
func (d *Daemon) setConfigured(pubKey string, in bool) {
	d.configuredMu.Lock()
	defer d.configuredMu.Unlock()
	if !in {
		delete(d.configured, pubKey)
		return
	}
	if d.configured == nil {
		d.configured = make(map[string]struct{})
	}
	d.configured[pubKey] = struct{}{}
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
	log.Printf("Mesh IP: %s", d.localNode.GetMeshIP())
	if ip6 := d.localNode.GetMeshIPv6(); ip6 != "" {
		log.Printf("Mesh IPv6: %s", ip6)
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
		d.SetDHTDiscovery(dht)

		if err := dht.Start(); err != nil {
			return fmt.Errorf("failed to start DHT discovery: %w", err)
		}
		defer dht.Stop()
	} else {
		log.Printf("Warning: DHT discovery factory not set, running without DHT")
	}

	// Start epoch manager for privacy features
	if d.config.Privacy {
		epochManager := NewEpochManager(d.config.Keys.EpochSeed)
		epochManager.Start(d.ctx, d.getPrivacyPeers)
		defer epochManager.Stop()
		log.Printf("Privacy mode enabled (Dandelion++ relay)")
	}

	// Start RPC server if one is set
	if rpcServer := d.rpcLayer(); rpcServer != nil {
		if err := rpcServer.Start(); err != nil {
			log.Printf("Warning: failed to start RPC server: %v", err)
		} else {
			defer func() {
				if err := rpcServer.Stop(); err != nil {
					log.Printf("Warning: failed to stop RPC server: %v", err)
				}
			}()
		}
	}

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

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

	// Notice a wake from sleep (#829)
	d.startResumeLoop()

	log.Printf("Daemon running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	for {
		select {
		case sig := <-sigCh:
			if sig == syscall.SIGHUP {
				log.Printf("Received SIGHUP, reloading configuration...")
				d.handleSIGHUP()
				continue
			}
			log.Printf("Received signal %v, shutting down...", sig)
		case <-d.ctx.Done():
			log.Printf("Context cancelled, shutting down...")
		}
		break
	}

	d.cancel()

	log.Printf("Waiting for background tasks to complete...")
	d.wg.Wait()
	d.revertKernel()
	return nil
}

// DHTDiscoveryFactory is a function type for creating DHT discovery instances.
// The ctx parameter should be the daemon's context so that DHT goroutines are
// cancelled when the daemon shuts down.
type DHTDiscoveryFactory func(ctx context.Context, config *Config, localNode *LocalNode, peerStore *PeerStore) (DiscoveryLayer, error)

// dhtDiscoveryFactory is atomic: the setter is public and Run reads it.
var dhtDiscoveryFactory atomic.Pointer[DHTDiscoveryFactory]

// SetDHTDiscoveryFactory sets the factory function for creating DHT discovery
// This is called by the discovery package to avoid circular imports
func SetDHTDiscoveryFactory(factory DHTDiscoveryFactory) {
	dhtDiscoveryFactory.Store(&factory)
}

// GetDHTDiscoveryFactory returns the current DHT discovery factory
func GetDHTDiscoveryFactory() DHTDiscoveryFactory {
	if f := dhtDiscoveryFactory.Load(); f != nil {
		return *f
	}
	return nil
}

// handleSIGHUP reads the reload file for the current interface and applies
// any changed reloadable options, then triggers an immediate reconcile.
// If no reload file exists the call is a no-op (warning is logged).
func (d *Daemon) handleSIGHUP() {
	path := ReloadConfigPath(d.config.InterfaceName)
	opts, err := LoadReloadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[Reload] No reload file found at %s (create it to change advertise-routes, accept-routes or log-level)", path)
		} else {
			log.Printf("[Reload] Failed to load reload file %s: %v", path, err)
		}
		return
	}
	d.reloadConfig(opts)
	d.reconcile()
}

// GetAdvertiseRoutes returns the current advertised routes (thread-safe).
func (d *Daemon) GetAdvertiseRoutes() []string {
	d.configMu.RLock()
	defer d.configMu.RUnlock()
	out := make([]string, len(d.config.AdvertiseRoutes))
	copy(out, d.config.AdvertiseRoutes)
	return out
}

// acceptRoutes is the operator's accept list as networks (R9), read under the
// config lock; nil means no list. The entries were validated when they were set.
func (d *Daemon) acceptRoutes() []*net.IPNet {
	d.configMu.RLock()
	defer d.configMu.RUnlock()
	var out []*net.IPNet
	for _, s := range d.config.AcceptRoutes {
		if _, n, err := net.ParseCIDR(s); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// ownRoutableNetworks returns a copy of the local node's RoutableNetworks,
// which SIGHUP reload replaces.
func (d *Daemon) ownRoutableNetworks() []string {
	if d.localNode == nil {
		return nil
	}
	return d.localNode.GetRoutableNetworks()
}

// GetLogLevel returns the current log level (thread-safe).
func (d *Daemon) GetLogLevel() string {
	return d.config.GetLogLevel()
}

// reloadConfig applies a new set of reloadable options without restarting
// the WireGuard interface or DHT connections.  It updates:
//   - AdvertiseRoutes (triggers immediate reconcile on next tick)
//   - LogLevel
//   - AcceptRoutes (judges later offers; routes already stored change when the
//     peer next announces)
//
// Non-reloadable fields (Secret, InterfaceName, WGListenPort, Privacy, Gossip)
// are silently ignored.
func (d *Daemon) reloadConfig(opts DaemonOpts) {
	d.configMu.Lock()
	defer d.configMu.Unlock()

	if cur := d.config.GetLogLevel(); cur != opts.LogLevel && opts.LogLevel != "" {
		log.Printf("[Reload] LogLevel: %q → %q", cur, opts.LogLevel)
		d.config.SetLogLevel(opts.LogLevel)
	}

	if opts.AdvertiseRoutes != nil && !routeSlicesEqual(d.config.AdvertiseRoutes, opts.AdvertiseRoutes) {
		log.Printf("[Reload] AdvertiseRoutes: %v → %v", d.config.AdvertiseRoutes, opts.AdvertiseRoutes)
		d.config.AdvertiseRoutes = opts.AdvertiseRoutes
		if d.localNode != nil {
			d.localNode.SetRoutableNetworks(opts.AdvertiseRoutes)
		}
	}

	// nil = the key is absent from the reload file (no change); non-nil and
	// empty = "accept-routes=" (clear the list, open the node).
	if opts.AcceptRoutes != nil && !routeSlicesEqual(d.config.AcceptRoutes, opts.AcceptRoutes) {
		log.Printf("[Reload] AcceptRoutes: %v → %v", d.config.AcceptRoutes, opts.AcceptRoutes)
		d.config.AcceptRoutes = opts.AcceptRoutes
	}
}

// routeSlicesEqual reports whether two route slices have identical contents,
// ignoring order.
func routeSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]struct{}, len(a))
	for _, r := range a {
		set[r] = struct{}{}
	}
	for _, r := range b {
		if _, ok := set[r]; !ok {
			return false
		}
	}
	return true
}

func (d *Daemon) setLocalWGEndpoint() {
	if d.localNode == nil {
		return
	}
	d.localNode.SetEndpoint(net.JoinHostPort("0.0.0.0", strconv.Itoa(d.config.WGListenPort)))
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
	d.layersMu.Lock()
	defer d.layersMu.Unlock()
	d.rpcServer = server
}

// markStarted records the start time that GetUptime reports from.
func (d *Daemon) markStarted() {
	now := time.Now()
	d.startTime.Store(&now)
}

// GetUptime returns the daemon uptime
func (d *Daemon) GetUptime() time.Duration {
	start := d.startTime.Load()
	if start == nil {
		// Lazily initialize startTime to avoid incorrect uptime when it was not set on startup
		now := time.Now()
		d.startTime.CompareAndSwap(nil, &now)
		return 0
	}
	return time.Since(*start)
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
		rpcPeer := &RPCPeerData{
			WGPubKey:         p.WGPubKey,
			Hostname:         p.Hostname,
			MeshIP:           p.MeshIP,
			Endpoint:         p.Endpoint,
			LastSeen:         p.LastSeen,
			DiscoveredVia:    p.DiscoveredVia,
			RoutableNetworks: p.RoutableNetworks,
		}
		if p.Latency != nil {
			ms := float64(p.Latency.Milliseconds())
			rpcPeer.LatencyMs = &ms
		}
		result = append(result, rpcPeer)
	}
	return result
}

// GetRPCPeer returns a single peer for RPC
func (d *Daemon) GetRPCPeer(pubKey string) (*RPCPeerData, bool) {
	peer, exists := d.peerStore.Get(pubKey)
	if !exists {
		return nil, false
	}
	rpcPeer := &RPCPeerData{
		WGPubKey:         peer.WGPubKey,
		Hostname:         peer.Hostname,
		MeshIP:           peer.MeshIP,
		Endpoint:         peer.Endpoint,
		LastSeen:         peer.LastSeen,
		DiscoveredVia:    peer.DiscoveredVia,
		RoutableNetworks: peer.RoutableNetworks,
	}
	if peer.Latency != nil {
		ms := float64(peer.Latency.Milliseconds())
		rpcPeer.LatencyMs = &ms
	}
	return rpcPeer, true
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
	pubKey, meshIP := d.localNode.Identity()
	return &RPCStatusData{
		MeshIP:    meshIP,
		PubKey:    pubKey,
		Uptime:    d.GetUptime(),
		Interface: d.config.InterfaceName,
	}
}

// RPCPeerData represents peer info for RPC (matches rpc.PeerData)
type RPCPeerData struct {
	WGPubKey         string
	Hostname         string
	MeshIP           string
	Endpoint         string
	LastSeen         time.Time
	DiscoveredVia    []string
	RoutableNetworks []string
	LatencyMs        *float64 // nil when no probe has succeeded yet
}

// RPCStatusData represents daemon status for RPC (matches rpc.StatusData)
type RPCStatusData struct {
	MeshIP    string
	PubKey    string
	Uptime    time.Duration
	Interface string
}
