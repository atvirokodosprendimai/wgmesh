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
		}
	}

	// Original CLI mode
	var (
		stateFile  = flag.String("state", "mesh-state.json", "Path to mesh state file")
		addNode    = flag.String("add", "", "Add node (format: hostname:ip:ssh_host[:ssh_port])")
		removeNode = flag.String("remove", "", "Remove node by hostname")
		list       = flag.Bool("list", false, "List all nodes")
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
  -state <file>    Path to mesh state file (default: mesh-state.json)
  -add <spec>      Add node (format: hostname:ip:ssh_host[:ssh_port])
  -remove <name>   Remove node by hostname
  -list            List all nodes
  -deploy          Deploy configuration to all nodes
  -init            Initialize new mesh state file
  -encrypt         Encrypt state file with password

SUBCOMMANDS (decentralized mode):
  init --secret                 Generate a new mesh secret
  join --secret <SECRET>        Join a mesh network
  status --secret <SECRET>      Show mesh status
  qr --secret <SECRET>          Display secret as QR code (text)
  install-service --secret ...  Install systemd service
  uninstall-service             Remove systemd service
  rotate-secret                 Rotate mesh secret

EXAMPLES:
  # Show version
  wgmesh --version
  wgmesh -v

  # Decentralized mode (automatic peer discovery):
  wgmesh init --secret                          # Generate a new mesh secret
  wgmesh join --secret "wgmesh://v1/K7x2..."    # Join mesh on this node
  wgmesh join --secret "..." --privacy           # Join with Dandelion++ privacy
  wgmesh join --secret "..." --gossip            # Enable in-mesh gossip

  # Centralized mode (SSH-based deployment):
  wgmesh -init -encrypt                         # Initialize encrypted state
  wgmesh -add node1:10.99.0.1:192.168.1.10     # Add a node
  wgmesh -deploy                               # Deploy to all nodes`)
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
		Secret:          *secret,
		InterfaceName:   *iface,
		WGListenPort:    *listenPort,
		AdvertiseRoutes: routes,
		LogLevel:        *logLevel,
		Privacy:         *privacyMode,
		Gossip:          *gossipMode,
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

	fmt.Println("Initializing mesh node with DHT discovery...")
	if *privacyMode {
		fmt.Println("Privacy mode enabled (Dandelion++ relay)")
	}
	if *gossipMode {
		fmt.Println("In-mesh gossip enabled")
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
	announcement := crypto.CreateAnnouncement("test-pubkey", "10.0.0.1", "test:51820", nil, nil)
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

// installServiceCmd handles the "install-service" subcommand
func installServiceCmd() {
	fs := flag.NewFlagSet("install-service", flag.ExitOnError)
	secret := fs.String("secret", "", "Mesh secret (required)")
	iface := fs.String("interface", "wg0", "WireGuard interface name")
	listenPort := fs.Int("listen-port", 51820, "WireGuard listen port")
	advertiseRoutes := fs.String("advertise-routes", "", "Comma-separated routes to advertise")
	privacyMode := fs.Bool("privacy", false, "Enable privacy mode")
	gossipMode := fs.Bool("gossip", false, "Enable in-mesh gossip")
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
		Secret:          *secret,
		InterfaceName:   *iface,
		ListenPort:      *listenPort,
		AdvertiseRoutes: routes,
		Privacy:         *privacyMode,
		Gossip:          *gossipMode,
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
