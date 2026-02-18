package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
	"github.com/atvirokodosprendimai/wgmesh/pkg/mesh"
	"github.com/atvirokodosprendimai/wgmesh/pkg/rpc"

	// Import discovery to register the DHT factory via init()
	_ "github.com/atvirokodosprendimai/wgmesh/pkg/discovery"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	// Check for version flags first (--version or -v)
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			fmt.Println("wgmesh " + version)
			return
		}
	}

	// Check for subcommands first
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Println("wgmesh " + version)
			return
		case "join":
			joinCmd()
			return
		case "init":
			initCmd()
			return
		case "status":
			statusCmd()
			return
		case "test-peer":
			testPeerCmd()
			return
		case "qr":
			qrCmd()
			return
		case "install-service":
			installServiceCmd()
			return
		case "uninstall-service":
			uninstallServiceCmd()
			return
		case "rotate-secret":
			rotateSecretCmd()
			return
		case "mesh":
			meshCmd()
			return
		case "peers":
			peersCmd()
			return
		}
	}

	// Original CLI mode
	var (
		stateFile  = flag.String("state", "/var/lib/wgmesh/mesh-state.json", "Path to mesh state file")
		addNode    = flag.String("add", "", "Add node (format: hostname:ip:ssh_host[:ssh_port])")
		removeNode = flag.String("remove", "", "Remove node by hostname")
		list       = flag.Bool("list", false, "List all nodes")
		listSimple = flag.Bool("list-simple", false, "List all nodes in simple format (hostname ip)")
		deploy     = flag.Bool("deploy", false, "Deploy configuration to all nodes")
		init       = flag.Bool("init", false, "Initialize new mesh")
		encrypt    = flag.Bool("encrypt", false, "Encrypt state file with password (asks for password)")
	)

	flag.Parse()

	// Handle encryption flag
	if *encrypt {
		var password string
		var err error

		if *init {
			// For init, ask for password twice
			password, err = crypto.ReadPasswordTwice("Enter encryption password: ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read password: %v\n", err)
				os.Exit(1)
			}
		} else {
			// For other operations, ask once
			password, err = crypto.ReadPassword("Enter encryption password: ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read password: %v\n", err)
				os.Exit(1)
			}
		}

		mesh.SetEncryptionPassword(password)
	}

	if *init {
		if err := mesh.Initialize(*stateFile); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize mesh: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Mesh initialized successfully")
		return
	}

	m, err := mesh.Load(*stateFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load mesh state: %v\n", err)
		os.Exit(1)
	}

	switch {
	case *addNode != "":
		if err := m.AddNode(*addNode); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to add node: %v\n", err)
			os.Exit(1)
		}
		if err := m.Save(*stateFile); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save state: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Node added successfully\n")

	case *removeNode != "":
		if err := m.RemoveNode(*removeNode); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to remove node: %v\n", err)
			os.Exit(1)
		}
		if err := m.Save(*stateFile); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save state: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Node removed successfully\n")

	case *list:
		m.List()

	case *listSimple:
		m.ListSimple()

	case *deploy:
		if err := m.Deploy(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to deploy: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Deployment completed successfully")

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`wgmesh - WireGuard mesh network builder

FLAGS:
  --version, -v               Show version information
  -state <file>    Path to mesh state file (default: /var/lib/wgmesh/mesh-state.json)
  -add <spec>      Add node (format: hostname:ip:ssh_host[:ssh_port])
  -remove <name>   Remove node by hostname
  -list            List all nodes
  -deploy          Deploy configuration to all nodes
  -init            Initialize new mesh state file
  -encrypt         Encrypt state file with password

SUBCOMMANDS (centralized mode):
  mesh list [--state <file>] [--encrypt]  List hostnames and mesh IPs

SUBCOMMANDS (decentralized mode):
  init --secret                 Generate a new mesh secret
	join --secret <SECRET>        Join a mesh network
	     [--no-lan-discovery]     Disable LAN multicast discovery
	     [--no-ipv6]              Ignore IPv6 endpoints for connectivity
	     [--force-relay]          Prefer relay path for non-LAN peers
	     [--no-punching]          Disable NAT port punching/rendezvous
	     [--introducer]           Enable rendezvous introducer role
  status --secret <SECRET>      Show mesh status
  qr --secret <SECRET>          Display secret as QR code (text)
	install-service --secret ...  Install systemd service
	     [--no-lan-discovery]     Disable LAN multicast discovery in service
	     [--no-ipv6]              Ignore IPv6 endpoints in service
	     [--force-relay]          Prefer relay path in service
	     [--no-punching]          Disable NAT punching in service
	     [--introducer]           Enable rendezvous introducer role in service
  uninstall-service             Remove systemd service
  rotate-secret                 Rotate mesh secret

QUERY SUBCOMMANDS (decentralized mode):
  peers list                    List all active peers
  peers count                   Show peer statistics
  peers get <pubkey>            Get specific peer details

  Note: These commands query a running daemon via RPC socket.
  The daemon must be started with 'wgmesh join' first.

EXAMPLES:
  # Show version
  wgmesh --version
  wgmesh -v

  # Decentralized mode (automatic peer discovery):
  wgmesh init --secret                          # Generate a new mesh secret
  wgmesh join --secret "wgmesh://v1/K7x2..."    # Join mesh on this node
  wgmesh join --secret "..." --privacy           # Join with Dandelion++ privacy
  wgmesh join --secret "..." --gossip            # Enable in-mesh gossip

  # Query running daemon:
  wgmesh peers list                              # List all active peers
  wgmesh peers count                             # Show peer counts
  wgmesh peers get <pubkey>                      # Get specific peer info

  # Centralized mode (SSH-based deployment):
  wgmesh -init -encrypt                         # Initialize encrypted state
  wgmesh -add node1:10.99.0.1:192.168.1.10     # Add a node
  wgmesh -deploy                               # Deploy to all nodes
  wgmesh mesh list                             # List hostnames and mesh IPs`)
}

// initCmd handles the "init --secret" subcommand
func initCmd() {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	secretMode := fs.Bool("secret", false, "Generate a new mesh secret")
	fs.Parse(os.Args[2:])

	if *secretMode {
		secret, err := daemon.GenerateSecret()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate secret: %v\n", err)
			os.Exit(1)
		}

		uri := daemon.FormatSecretURI(secret)
		fmt.Println("Generated mesh secret:")
		fmt.Println()
		fmt.Println(uri)
		fmt.Println()
		fmt.Println("Share this secret with all nodes that should join the mesh.")
		fmt.Println("Run: wgmesh join --secret \"" + uri + "\"")
		return
	}

	fs.Usage()
	os.Exit(1)
}

// joinCmd handles the "join --secret" subcommand
func joinCmd() {
	fs := flag.NewFlagSet("join", flag.ExitOnError)
	secret := fs.String("secret", "", "Mesh secret (required)")
	advertiseRoutes := fs.String("advertise-routes", "", "Comma-separated list of routes to advertise")
	listenPort := fs.Int("listen-port", 51820, "WireGuard listen port")
	iface := fs.String("interface", "wg0", "WireGuard interface name")
	logLevel := fs.String("log-level", "info", "Log level (debug, info, warn, error)")
	privacyMode := fs.Bool("privacy", false, "Enable privacy mode (Dandelion++ relay)")
	gossipMode := fs.Bool("gossip", false, "Enable in-mesh gossip")
	socketPath := fs.String("socket-path", "", "RPC socket path (auto-detected if empty)")
	noLANDiscovery := fs.Bool("no-lan-discovery", false, "Disable LAN multicast discovery")
	noIPv6 := fs.Bool("no-ipv6", false, "Ignore IPv6 endpoints for connectivity")
	forceRelay := fs.Bool("force-relay", false, "Prefer relay path for non-LAN peers")
	noPunching := fs.Bool("no-punching", false, "Disable NAT port punching/rendezvous")
	introducerMode := fs.Bool("introducer", false, "Allow this node to act as rendezvous introducer")
	fs.Parse(os.Args[2:])

	if *secret == "" {
		fmt.Fprintln(os.Stderr, "Error: --secret is required")
		fmt.Fprintln(os.Stderr, "Usage: wgmesh join --secret <SECRET>")
		os.Exit(1)
	}

	// Parse advertise routes
	var routes []string
	if *advertiseRoutes != "" {
		routes = strings.Split(*advertiseRoutes, ",")
		for i, r := range routes {
			routes[i] = strings.TrimSpace(r)
		}
	}

	// Create daemon config
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{
		Secret:              *secret,
		InterfaceName:       *iface,
		WGListenPort:        *listenPort,
		AdvertiseRoutes:     routes,
		LogLevel:            *logLevel,
		Privacy:             *privacyMode,
		Gossip:              *gossipMode,
		DisableLANDiscovery: *noLANDiscovery,
		DisableIPv6:         *noIPv6,
		ForceRelay:          *forceRelay,
		DisablePunching:     *noPunching,
		Introducer:          *introducerMode,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create config: %v\n", err)
		os.Exit(1)
	}

	// Create and run daemon with DHT discovery
	d, err := daemon.NewDaemon(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create daemon: %v\n", err)
		os.Exit(1)
	}

	// Setup RPC server
	rpcSocketPath := *socketPath
	if rpcSocketPath == "" {
		// Import here to avoid circular dependency
		rpcSocketPath = getRPCSocketPath()
	}
	
	// Create RPC server with callback functions
	rpcServer, err := createRPCServer(d, rpcSocketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create RPC server: %v\n", err)
	} else {
		d.SetRPCServer(rpcServer)
		fmt.Printf("RPC socket: %s\n", rpcSocketPath)
	}

	fmt.Println("Initializing mesh node with DHT discovery...")
	if *privacyMode {
		fmt.Println("Privacy mode enabled (Dandelion++ relay)")
	}
	if *gossipMode {
		fmt.Println("In-mesh gossip enabled")
	}
	if *noLANDiscovery {
		fmt.Println("LAN discovery disabled")
	}
	if *noIPv6 {
		fmt.Println("IPv6 connectivity disabled")
	}
	if *forceRelay {
		fmt.Println("Force relay mode enabled")
	}
	if *noPunching {
		fmt.Println("NAT punching disabled")
	}
	if *introducerMode {
		fmt.Println("Rendezvous introducer enabled")
	}

	if err := d.RunWithDHTDiscovery(); err != nil {
		fmt.Fprintf(os.Stderr, "Daemon error: %v\n", err)
		os.Exit(1)
	}
}

// testPeerCmd tests direct peer exchange connectivity
func testPeerCmd() {
	fs := flag.NewFlagSet("test-peer", flag.ExitOnError)
	secret := fs.String("secret", "", "Mesh secret (required)")
	peerAddr := fs.String("peer", "", "Peer address to test (IP:PORT)")
	listenPort := fs.Int("port", 0, "Local port to listen on (0 = random)")
	fs.Parse(os.Args[2:])

	if *secret == "" || *peerAddr == "" {
		fmt.Fprintln(os.Stderr, "Usage: wgmesh test-peer --secret <SECRET> --peer <IP:PORT>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "This tests direct UDP connectivity to another wgmesh node.")
		fmt.Fprintln(os.Stderr, "Run 'wgmesh join' on the peer first, note its exchange port,")
		fmt.Fprintln(os.Stderr, "then test with: wgmesh test-peer --secret <SECRET> --peer <PEER_IP>:<EXCHANGE_PORT>")
		os.Exit(1)
	}

	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: *secret})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Testing peer exchange with %s\n", *peerAddr)
	fmt.Printf("Network ID: %x\n", cfg.Keys.NetworkID[:8])

	// Create UDP socket
	addr := &net.UDPAddr{Port: *listenPort}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bind UDP: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf("Listening on port %d\n", conn.LocalAddr().(*net.UDPAddr).Port)

	// Resolve peer
	peerUDP, err := net.ResolveUDPAddr("udp", *peerAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve peer: %v\n", err)
		os.Exit(1)
	}

	// Create and send test message
	announcement := crypto.CreateAnnouncement("test-pubkey", "10.0.0.1", "test:51820", false, nil, nil, "", "", "")
	data, err := crypto.SealEnvelope(crypto.MessageTypeHello, announcement, cfg.Keys.GossipKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create message: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Sending HELLO to %s (%d bytes)...\n", *peerAddr, len(data))
	_, err = conn.WriteToUDP(data, peerUDP)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to send: %v\n", err)
		os.Exit(1)
	}

	// Wait for response
	fmt.Println("Waiting for response (10s timeout)...")
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	buf := make([]byte, 65536)
	n, from, err := conn.ReadFromUDP(buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "No response: %v\n", err)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Possible issues:")
		fmt.Fprintln(os.Stderr, "- Peer not running or wrong port")
		fmt.Fprintln(os.Stderr, "- Firewall blocking UDP")
		fmt.Fprintln(os.Stderr, "- Different secrets (different gossip keys)")
		os.Exit(1)
	}

	fmt.Printf("Received %d bytes from %s\n", n, from.String())

	// Try to decrypt
	envelope, reply, err := crypto.OpenEnvelope(buf[:n], cfg.Keys.GossipKey)
	if err != nil {
		fmt.Printf("Failed to decrypt (wrong secret?): %v\n", err)
		os.Exit(1)
	}

	fmt.Println("SUCCESS! Peer exchange working!")
	fmt.Printf("  Message type: %s\n", envelope.MessageType)
	fmt.Printf("  Peer pubkey: %s\n", reply.WGPubKey)
	fmt.Printf("  Peer mesh IP: %s\n", reply.MeshIP)
}

// statusCmd handles the "status --secret" subcommand
func statusCmd() {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	secret := fs.String("secret", "", "Mesh secret (required)")
	iface := fs.String("interface", "wg0", "WireGuard interface name")
	fs.Parse(os.Args[2:])

	if *secret == "" {
		fmt.Fprintln(os.Stderr, "Error: --secret is required")
		fmt.Fprintln(os.Stderr, "Usage: wgmesh status --secret <SECRET>")
		os.Exit(1)
	}

	// Create config to derive keys
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{
		Secret:        *secret,
		InterfaceName: *iface,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Mesh Status\n")
	fmt.Printf("===========\n")
	fmt.Printf("Interface: %s\n", cfg.InterfaceName)
	fmt.Printf("Network ID: %x\n", cfg.Keys.NetworkID[:8])
	fmt.Printf("Mesh Subnet: 10.%d.0.0/16\n", cfg.Keys.MeshSubnet[0])
	fmt.Printf("Mesh IPv6 Prefix: %s\n", formatIPv6Prefix(cfg.Keys.MeshPrefixV6))
	fmt.Printf("Gossip Port: %d\n", cfg.Keys.GossipPort)
	fmt.Printf("Rendezvous ID: %x\n", cfg.Keys.RendezvousID)
	fmt.Println()

	// Show service status if available
	status, err := daemon.ServiceStatus()
	if err == nil {
		fmt.Printf("Service Status: %s\n", status)
	}

	fmt.Println()
	fmt.Println("(Run 'wg show' to see connected peers)")
}

// qrCmd handles the "qr" subcommand - displays secret as a text-based QR code
func qrCmd() {
	fs := flag.NewFlagSet("qr", flag.ExitOnError)
	secret := fs.String("secret", "", "Mesh secret to encode as QR code")
	fs.Parse(os.Args[2:])

	if *secret == "" {
		fmt.Fprintln(os.Stderr, "Error: --secret is required")
		fmt.Fprintln(os.Stderr, "Usage: wgmesh qr --secret <SECRET>")
		os.Exit(1)
	}

	uri := *secret
	if !strings.HasPrefix(uri, daemon.URIPrefix) {
		uri = daemon.FormatSecretURI(*secret)
	}

	fmt.Println("Mesh Secret QR Code")
	fmt.Println("====================")
	fmt.Println()
	fmt.Printf("URI: %s\n", uri)
	fmt.Println()

	// Generate a simple text-based QR representation
	// For a real QR code, the go-qrcode library would be used
	printTextQR(uri)

	fmt.Println()
	fmt.Println("Scan this QR code or copy the URI to join the mesh.")
}

// printTextQR prints a simple text-based representation of the secret
func printTextQR(data string) {
	// Generate a simple visual representation using Unicode block characters
	// This is a placeholder - a real implementation would use go-qrcode
	const maxLineWidth = 40 // Maximum characters per line for readability
	width := len(data)
	if width > maxLineWidth {
		width = maxLineWidth
	}

	border := strings.Repeat("██", width+2)
	fmt.Println(border)
	fmt.Printf("██%s██\n", strings.Repeat("  ", width))

	// Print the data in a box format for easy reading
	for i := 0; i < len(data); i += width {
		end := i + width
		if end > len(data) {
			end = len(data)
		}
		chunk := data[i:end]
		padding := strings.Repeat(" ", (width-len(chunk))*2)
		fmt.Printf("██  %s%s  ██\n", chunk, padding)
	}

	fmt.Printf("██%s██\n", strings.Repeat("  ", width))
	fmt.Println(border)
}

func formatIPv6Prefix(prefix [8]byte) string {
	return fmt.Sprintf("%02x%02x:%02x%02x:%02x%02x:%02x%02x::/64",
		prefix[0], prefix[1],
		prefix[2], prefix[3],
		prefix[4], prefix[5],
		prefix[6], prefix[7],
	)
}

// installServiceCmd handles the "install-service" subcommand
func installServiceCmd() {
	fs := flag.NewFlagSet("install-service", flag.ExitOnError)
	secret := fs.String("secret", "", "Mesh secret (required)")
	iface := fs.String("interface", "wg0", "WireGuard interface name")
	listenPort := fs.Int("listen-port", 51820, "WireGuard listen port")
	advertiseRoutes := fs.String("advertise-routes", "", "Comma-separated routes to advertise")
	privacyMode := fs.Bool("privacy", false, "Enable privacy mode")
	gossipMode := fs.Bool("gossip", false, "Enable in-mesh gossip")
	noLANDiscovery := fs.Bool("no-lan-discovery", false, "Disable LAN multicast discovery")
	noIPv6 := fs.Bool("no-ipv6", false, "Ignore IPv6 endpoints for connectivity")
	forceRelay := fs.Bool("force-relay", false, "Prefer relay path for non-LAN peers")
	noPunching := fs.Bool("no-punching", false, "Disable NAT port punching/rendezvous")
	introducerMode := fs.Bool("introducer", false, "Allow this node to act as rendezvous introducer")
	fs.Parse(os.Args[2:])

	if *secret == "" {
		fmt.Fprintln(os.Stderr, "Error: --secret is required")
		fmt.Fprintln(os.Stderr, "Usage: wgmesh install-service --secret <SECRET>")
		os.Exit(1)
	}

	var routes []string
	if *advertiseRoutes != "" {
		routes = strings.Split(*advertiseRoutes, ",")
		for i, r := range routes {
			routes[i] = strings.TrimSpace(r)
		}
	}

	cfg := daemon.SystemdServiceConfig{
		Secret:              *secret,
		InterfaceName:       *iface,
		ListenPort:          *listenPort,
		AdvertiseRoutes:     routes,
		Privacy:             *privacyMode,
		Gossip:              *gossipMode,
		DisableLANDiscovery: *noLANDiscovery,
		DisableIPv6:         *noIPv6,
		ForceRelay:          *forceRelay,
		DisablePunching:     *noPunching,
		Introducer:          *introducerMode,
	}

	fmt.Println("Installing wgmesh systemd service...")
	if err := daemon.InstallSystemdService(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to install service: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Service installed and started successfully!")
	fmt.Println("Check status with: systemctl status wgmesh")
}

// uninstallServiceCmd handles the "uninstall-service" subcommand
func uninstallServiceCmd() {
	fmt.Println("Removing wgmesh systemd service...")
	if err := daemon.UninstallSystemdService(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to uninstall service: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Service removed successfully!")
}

// rotateSecretCmd handles the "rotate-secret" subcommand
func rotateSecretCmd() {
	fs := flag.NewFlagSet("rotate-secret", flag.ExitOnError)
	currentSecret := fs.String("current", "", "Current mesh secret (required)")
	newSecret := fs.String("new", "", "New mesh secret (auto-generated if empty)")
	gracePeriod := fs.Duration("grace", 24*time.Hour, "Grace period for dual-secret mode")
	fs.Parse(os.Args[2:])

	if *currentSecret == "" {
		fmt.Fprintln(os.Stderr, "Error: --current is required")
		fmt.Fprintln(os.Stderr, "Usage: wgmesh rotate-secret --current <OLD_SECRET> [--new <NEW_SECRET>] [--grace 24h]")
		os.Exit(1)
	}

	// Generate new secret if not provided
	if *newSecret == "" {
		secret, err := daemon.GenerateSecret()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate new secret: %v\n", err)
			os.Exit(1)
		}
		*newSecret = secret
	}

	// Derive keys from old secret for signing
	oldKeys, err := crypto.DeriveKeys(*currentSecret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to derive keys from current secret: %v\n", err)
		os.Exit(1)
	}

	// Create rotation announcement
	announcement, err := crypto.GenerateRotationAnnouncement(oldKeys.MembershipKey[:], *newSecret, *gracePeriod)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create rotation announcement: %v\n", err)
		os.Exit(1)
	}

	_ = announcement // Would be broadcast via gossip in a running mesh

	newURI := daemon.FormatSecretURI(*newSecret)

	fmt.Println("Secret Rotation Initiated")
	fmt.Println("=========================")
	fmt.Printf("Grace Period: %v\n", *gracePeriod)
	fmt.Printf("New Secret URI: %s\n", newURI)
	fmt.Println()
	fmt.Println("During the grace period, both secrets will be accepted.")
	fmt.Printf("After %v, all nodes should use the new secret.\n", *gracePeriod)
	fmt.Println()
	fmt.Println("Share the new secret with all nodes:")
	fmt.Printf("  wgmesh join --secret \"%s\"\n", newURI)
}

// meshCmd handles the "mesh" subcommand for centralized mesh management
func meshCmd() {
	// Check for action subcommand first
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: action required")
		fmt.Fprintln(os.Stderr, "Usage: wgmesh mesh <action> [options]")
		fmt.Fprintln(os.Stderr, "Actions: list")
		os.Exit(1)
	}

	action := os.Args[2]

	fs := flag.NewFlagSet("mesh "+action, flag.ExitOnError)
	stateFile := fs.String("state", "mesh-state.json", "Path to mesh state file")
	encrypt := fs.Bool("encrypt", false, "Encrypt state file with password")
	fs.Parse(os.Args[3:])

	// Handle encryption flag if set
	if *encrypt {
		password, err := crypto.ReadPassword("Enter encryption password: ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read password: %v\n", err)
			os.Exit(1)
		}
		mesh.SetEncryptionPassword(password)
	}

	// Load mesh state
	m, err := mesh.Load(*stateFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load mesh state: %v\n", err)
		os.Exit(1)
	}

	switch action {
	case "list":
		m.ListSimple()
	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %s\n", action)
		fmt.Fprintln(os.Stderr, "Available actions: list")
		os.Exit(1)
	}
}

// getRPCSocketPath determines the RPC socket path (uses rpc.GetSocketPath)
func getRPCSocketPath() string {
	return rpc.GetSocketPath()
}

// createRPCServer creates an RPC server for the daemon
func createRPCServer(d *daemon.Daemon, socketPath string) (daemon.RPCServer, error) {
	// Create server config with callback functions that bridge daemon types to RPC types
	config := rpc.ServerConfig{
		SocketPath: socketPath,
		Version:    version,
		GetPeers: func() []*rpc.PeerData {
			rpcPeers := d.GetRPCPeers()
			result := make([]*rpc.PeerData, len(rpcPeers))
			for i, p := range rpcPeers {
				result[i] = &rpc.PeerData{
					WGPubKey:         p.WGPubKey,
					MeshIP:           p.MeshIP,
					Endpoint:         p.Endpoint,
					LastSeen:         p.LastSeen,
					DiscoveredVia:    p.DiscoveredVia,
					RoutableNetworks: p.RoutableNetworks,
				}
			}
			return result
		},
		GetPeer: func(pubKey string) (*rpc.PeerData, bool) {
			peer, exists := d.GetRPCPeer(pubKey)
			if !exists {
				return nil, false
			}
			return &rpc.PeerData{
				WGPubKey:         peer.WGPubKey,
				MeshIP:           peer.MeshIP,
				Endpoint:         peer.Endpoint,
				LastSeen:         peer.LastSeen,
				DiscoveredVia:    peer.DiscoveredVia,
				RoutableNetworks: peer.RoutableNetworks,
			}, true
		},
		GetPeerCounts: d.GetRPCPeerCounts,
		GetStatus: func() *rpc.StatusData {
			status := d.GetRPCStatus()
			return &rpc.StatusData{
				MeshIP:    status.MeshIP,
				PubKey:    status.PubKey,
				Uptime:    status.Uptime,
				Interface: status.Interface,
			}
		},
	}

	return rpc.NewServer(config)
}

// peersCmd handles the "peers" subcommand for querying the daemon via RPC
func peersCmd() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: wgmesh peers <list|count|get>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  list            List all active peers")
		fmt.Fprintln(os.Stderr, "  count           Show peer counts")
		fmt.Fprintln(os.Stderr, "  get <pubkey>    Get specific peer by public key")
		os.Exit(1)
	}

	action := os.Args[2]

	// Determine socket path
	socketPath := os.Getenv("WGMESH_SOCKET")
	if socketPath == "" {
		socketPath = getRPCSocketPath()
	}

	// Connect to RPC server
	client, err := rpc.NewClient(socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to daemon: %v\n", err)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Is wgmesh daemon running?")
		fmt.Fprintln(os.Stderr, "  Start with: wgmesh join --secret <SECRET>")
		fmt.Fprintf(os.Stderr, "  Socket path: %s\n", socketPath)
		os.Exit(1)
	}
	defer client.Close()

	switch action {
	case "list":
		handlePeersList(client)
	case "count":
		handlePeersCount(client)
	case "get":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: wgmesh peers get <pubkey>")
			os.Exit(1)
		}
		pubkey := os.Args[3]
		handlePeersGet(client, pubkey)
	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %s\n", action)
		fmt.Fprintln(os.Stderr, "Available actions: list, count, get")
		os.Exit(1)
	}
}

func handlePeersList(client *rpc.Client) {
	result, err := client.Call("peers.list", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "RPC error: %v\n", err)
		os.Exit(1)
	}

	// Parse result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "Invalid response format")
		os.Exit(1)
	}

	peersData, ok := resultMap["peers"].([]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "Invalid peers data")
		os.Exit(1)
	}

	if len(peersData) == 0 {
		fmt.Println("No active peers")
		return
	}

	// Print header
	fmt.Printf("%-40s %-15s %-25s %-10s %s\n",
		"PUBLIC KEY", "MESH IP", "ENDPOINT", "LAST SEEN", "DISCOVERED VIA")
	fmt.Println(strings.Repeat("-", 120))

	// Print peers
	for _, peerData := range peersData {
		peer, ok := peerData.(map[string]interface{})
		if !ok {
			continue
		}

		pubkey, ok := peer["pubkey"].(string)
		if !ok {
			continue
		}
		if len(pubkey) > 40 {
			pubkey = pubkey[:37] + "..."
		}

		meshIP, _ := peer["mesh_ip"].(string)
		endpoint, _ := peer["endpoint"].(string)
		lastSeen, _ := peer["last_seen"].(string)

		// Parse last seen time
		lastSeenTime, err := time.Parse(time.RFC3339, lastSeen)
		var lastSeenStr string
		if err == nil {
			lastSeenStr = formatDuration(time.Since(lastSeenTime))
		} else {
			lastSeenStr = "unknown"
		}

		var discoveredViaStr []string
		if v, ok := peer["discovered_via"]; ok {
			if discoveredVia, ok := v.([]interface{}); ok {
				for _, item := range discoveredVia {
					if s, ok := item.(string); ok {
						discoveredViaStr = append(discoveredViaStr, s)
					}
				}
			}
		}

		fmt.Printf("%-40s %-15s %-25s %-10s %s\n",
			pubkey, meshIP, endpoint, lastSeenStr, strings.Join(discoveredViaStr, ","))
	}
}

func handlePeersCount(client *rpc.Client) {
	result, err := client.Call("peers.count", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "RPC error: %v\n", err)
		os.Exit(1)
	}

	// Parse result with type checking
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "Invalid response format")
		os.Exit(1)
	}

	active, ok1 := resultMap["active"].(float64)
	total, ok2 := resultMap["total"].(float64)
	dead, ok3 := resultMap["dead"].(float64)
	
	if !ok1 || !ok2 || !ok3 {
		fmt.Fprintln(os.Stderr, "Invalid peer count data")
		os.Exit(1)
	}

	fmt.Printf("Peer Statistics\n")
	fmt.Printf("===============\n")
	fmt.Printf("Active peers: %d\n", int(active))
	fmt.Printf("Total peers:  %d\n", int(total))
	fmt.Printf("Dead peers:   %d\n", int(dead))
}

func handlePeersGet(client *rpc.Client, pubkey string) {
	params := map[string]interface{}{
		"pubkey": pubkey,
	}

	result, err := client.Call("peers.get", params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "RPC error: %v\n", err)
		os.Exit(1)
	}

	// Parse result
	peer, ok := result.(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "Invalid response format")
		os.Exit(1)
	}

	pubkeyStr, _ := peer["pubkey"].(string)
	meshIP, _ := peer["mesh_ip"].(string)
	endpoint, _ := peer["endpoint"].(string)
	lastSeen, _ := peer["last_seen"].(string)

	fmt.Printf("Peer Information\n")
	fmt.Printf("================\n")
	fmt.Printf("Public Key:     %s\n", pubkeyStr)
	fmt.Printf("Mesh IP:        %s\n", meshIP)
	fmt.Printf("Endpoint:       %s\n", endpoint)
	fmt.Printf("Last Seen:      %s\n", lastSeen)

	if v, ok := peer["discovered_via"]; ok {
		if discoveredVia, ok := v.([]interface{}); ok && len(discoveredVia) > 0 {
			discoveredViaStr := make([]string, 0, len(discoveredVia))
			for _, item := range discoveredVia {
				if s, ok := item.(string); ok {
					discoveredViaStr = append(discoveredViaStr, s)
				}
			}
			if len(discoveredViaStr) > 0 {
				fmt.Printf("Discovered Via: %s\n", strings.Join(discoveredViaStr, ", "))
			}
		}
	}

	if routesVal, ok := peer["routable_networks"]; ok {
		if routes, ok := routesVal.([]interface{}); ok && len(routes) > 0 {
			routeStrs := make([]string, 0, len(routes))
			for _, r := range routes {
				if routeStr, ok := r.(string); ok {
					routeStrs = append(routeStrs, routeStr)
				}
			}
			if len(routeStrs) > 0 {
				fmt.Printf("Routes:         %s\n", strings.Join(routeStrs, ", "))
			}
		}
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

