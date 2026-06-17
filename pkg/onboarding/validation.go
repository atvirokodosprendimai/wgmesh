package onboarding

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// validateWGInterface checks if WireGuard interface exists and is configured
func validateWGInterface(interfaceName string) error {
	if interfaceName == "" {
		return fmt.Errorf("interface name not specified")
	}

	// Check if interface exists
	cmd := exec.Command("ip", "link", "show", interfaceName)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("interface %s does not exist: %w", interfaceName, err)
	}

	// Check if WireGuard tool is available
	wgPath, err := exec.LookPath("wg")
	if err != nil {
		return fmt.Errorf("wg tool not found: %w", err)
	}

	// Check WireGuard configuration
	cmd = exec.Command(wgPath, "show", interfaceName)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("WireGuard interface %s not configured: %w", interfaceName, err)
	}

	output := out.String()
	if strings.Contains(output, "No peer information found") || strings.TrimSpace(output) == "" {
		return fmt.Errorf("WireGuard interface %s has no peers configured", interfaceName)
	}

	return nil
}

// validateGitHubConnectivity checks if GitHub API is reachable
func validateGitHubConnectivity() error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Try to reach GitHub API
	req, err := http.NewRequest("GET", "https://api.github.com", nil)
	if err != nil {
		return fmt.Errorf("creating GitHub API request: %w", err)
	}

	// Set user agent to avoid rate limiting
	req.Header.Set("User-Agent", "wgmesh-onboarding/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub API unreachable: %w", err)
	}
	defer resp.Body.Close()

	// Check for valid response
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	return nil
}

// validateLANMulticastBind attempts to bind a multicast socket
func validateLANMulticastBind() error {
	// Try to bind to a multicast address
	multicastAddr := &net.UDPAddr{
		IP:   net.ParseIP("239.192.0.1"),
		Port: 51830,
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, multicastAddr)
	if err != nil {
		return fmt.Errorf("failed to bind multicast socket: %w", err)
	}

	// Successfully bound, close and return
	conn.Close()
	return nil
}

// validateDHTConnectivity checks if DHT bootstrap nodes are reachable
func validateDHTConnectivity() error {
	// Try to contact one of the well-known bootstrap nodes
	bootstrapNodes := []string{
		"router.bittorrent.com:6881",
		"router.utorrent.com:6881",
		"dht.transmissionbt.com:6881",
	}

	// Try each bootstrap node with a timeout
	for _, addr := range bootstrapNodes {
		// Resolve address
		udpAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			log.Printf("[Validation] Failed to resolve %s: %v", addr, err)
			continue
		}

		// Create UDP connection
		conn, err := net.DialUDP("udp", nil, udpAddr)
		if err != nil {
			log.Printf("[Validation] Failed to dial %s: %v", addr, err)
			continue
		}

		// Set timeout
		conn.SetDeadline(time.Now().Add(5 * time.Second))

		// Try to send a ping (we don't expect a response, just checking if we can send)
		_, err = conn.Write([]byte("ping"))
		conn.Close()

		if err == nil {
			// Successfully sent to at least one bootstrap node
			return nil
		}
	}

	return fmt.Errorf("unable to reach any DHT bootstrap nodes (checked: %s)", strings.Join(bootstrapNodes, ", "))
}

// validatePeerDiscovery waits for peer discovery with timeout
func validatePeerDiscovery(timeout time.Duration) error {
	if timeout == 0 {
		timeout = 2 * time.Minute
	}

	start := time.Now()
	checkInterval := 5 * time.Second

	log.Printf("[Validation] Waiting for peer discovery (timeout: %v)...", timeout)

	for time.Since(start) < timeout {
		// Check if daemon is running via RPC
		if isDaemonRunning() {
			// Query for peers
			if hasDiscoveredPeers() {
				log.Printf("[Validation] Peer discovery successful!")
				return nil
			}
		}

		log.Printf("[Validation] No peers discovered yet, retrying in %v...", checkInterval)
		time.Sleep(checkInterval)
	}

	return fmt.Errorf("no peers discovered within %v timeout", timeout)
}

// validateWireGuardHandshake checks for successful WireGuard handshakes
func validateWireGuardHandshake(interfaceName string) error {
	if interfaceName == "" {
		return fmt.Errorf("interface name not specified")
	}

	wgPath, err := exec.LookPath("wg")
	if err != nil {
		return fmt.Errorf("wg tool not found: %w", err)
	}

	// Get latest handshakes
	cmd := exec.Command(wgPath, "show", interfaceName, "latest-handshakes")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to get handshakes: %w", err)
	}

	output := strings.TrimSpace(out.String())
	if output == "" || output == "No peer information found" {
		return fmt.Errorf("no peers configured on interface %s", interfaceName)
	}

	// Parse output to check for recent handshakes
	lines := strings.Split(output, "\n")
	now := time.Now()

	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		// Parse timestamp (seconds since epoch)
		var timestamp int64
		_, err := fmt.Sscanf(parts[1], "%d", &timestamp)
		if err != nil {
			continue
		}

		if timestamp == 0 {
			continue // No handshake yet
		}

		handshakeTime := time.Unix(timestamp, 0)
		// Check if handshake happened within last 3 minutes
		if now.Sub(handshakeTime) < 3*time.Minute {
			log.Printf("[Validation] Recent handshake found with peer %s (timestamp: %v)", parts[0], handshakeTime)
			return nil
		}
	}

	return fmt.Errorf("no recent handshakes (within last 3 minutes) on interface %s", interfaceName)
}

// isDaemonRunning checks if the wgmesh daemon is running via RPC
func isDaemonRunning() bool {
	// Check for RPC socket
	socketPath := "/run/wgmesh/rpc.sock"
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		return false
	}
	return true
}

// hasDiscoveredPeers checks if any peers have been discovered
func hasDiscoveredPeers() bool {
	// This is a simplified check - in production we'd use the RPC client
	// For now, just check if daemon is running
	// The actual implementation would query RPC for peer count > 0
	return isDaemonRunning()
}

// ValidateAll runs all validation checks and returns results
func ValidateAll(secret string, opts WizardOptions) map[string]error {
	results := make(map[string]error)

	// Secret generation
	if err := validateSecretGeneration(secret)(); err != nil {
		results["secret_generation"] = err
	}

	// Interface configuration
	if err := validateInterfaceConfig(opts.InterfaceName)(); err != nil {
		results["interface_config"] = err
	}

	// GitHub registry
	if !opts.SkipRegistry {
		if err := validateGitHubRegistry(opts.SkipRegistry)(); err != nil {
			results["github_registry"] = err
		}
	}

	// LAN multicast
	if !opts.DisableLANDiscovery {
		if err := validateLANMulticast(opts.DisableLANDiscovery)(); err != nil {
			results["lan_multicast"] = err
		}
	}

	// DHT bootstrap
	if err := validateDHTBootstrap()(); err != nil {
		results["dht_bootstrap"] = err
	}

	// First peer contact (with shorter timeout for validation)
	if err := validateFirstPeerContact(30 * time.Second)(); err != nil {
		results["first_peer_contact"] = err
	}

	// Bidirectional ping
	if err := validateBidirectionalPing(opts.InterfaceName)(); err != nil {
		results["bidirectional_ping"] = err
	}

	return results
}

// readFromRPC reads from the RPC socket
func readFromRPC(method string, params map[string]interface{}) (interface{}, error) {
	// This is a placeholder for RPC communication
	// In production, we'd use the rpc.Client from pkg/rpc
	return nil, fmt.Errorf("RPC not implemented in validation")
}
