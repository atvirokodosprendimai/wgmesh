package wireguard

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh"
)

type FullConfig struct {
	Interface WGInterface
	Peers     []WGPeer
}

type WGInterface struct {
	PrivateKey string
	Address    string
	ListenPort int
}

type WGPeer struct {
	PublicKey           string
	Endpoint            string
	AllowedIPs          []string
	PersistentKeepalive int
}

func ApplyFullConfiguration(client *ssh.Client, iface string, config *FullConfig) error {
	fmt.Println("  Creating fresh WireGuard configuration...")

	if _, err := client.Run(fmt.Sprintf("ip link del %s 2>/dev/null || true", iface)); err != nil {
	}

	if _, err := client.Run(fmt.Sprintf("ip link add %s type wireguard", iface)); err != nil {
		return fmt.Errorf("failed to create interface: %w", err)
	}

	tmpKeyFile := fmt.Sprintf("/tmp/wg-key-%s", iface)
	if err := client.WriteFile(tmpKeyFile, []byte(config.Interface.PrivateKey), 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}
	defer client.Run(fmt.Sprintf("rm -f %s", tmpKeyFile))

	cmd := fmt.Sprintf("wg set %s private-key %s listen-port %d",
		iface, tmpKeyFile, config.Interface.ListenPort)
	if _, err := client.Run(cmd); err != nil {
		return fmt.Errorf("failed to set interface config: %w", err)
	}

	if _, err := client.Run(fmt.Sprintf("ip addr add %s dev %s", config.Interface.Address, iface)); err != nil {
		return fmt.Errorf("failed to set IP address: %w", err)
	}

	if _, err := client.Run(fmt.Sprintf("ip link set %s up", iface)); err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	for _, peer := range config.Peers {
		peerCmd := fmt.Sprintf("wg set %s peer %s", iface, peer.PublicKey)

		if peer.Endpoint != "" {
			peerCmd += fmt.Sprintf(" endpoint %s", peer.Endpoint)
		}

		if len(peer.AllowedIPs) > 0 {
			peerCmd += fmt.Sprintf(" allowed-ips %s", strings.Join(peer.AllowedIPs, ","))
		}

		if peer.PersistentKeepalive > 0 {
			peerCmd += fmt.Sprintf(" persistent-keepalive %d", peer.PersistentKeepalive)
		}

		if _, err := client.Run(peerCmd); err != nil {
			return fmt.Errorf("failed to add peer %s: %w", peer.PublicKey[:16], err)
		}

		fmt.Printf("    Added peer: %s\n", peer.PublicKey[:16])
	}

	return nil
}

// SetPeer adds or updates a peer on the local WireGuard interface
func SetPeer(iface, pubKey string, psk [32]byte, endpoint, allowedIPs string) error {
	// Build wg set command
	args := []string{"set", iface, "peer", pubKey}
	var stdin strings.Reader
	hasStdin := false

	// Add PSK if non-zero
	var zeroKey [32]byte
	if psk != zeroKey {
		pskB64 := base64.StdEncoding.EncodeToString(psk[:])
		args = append(args, "preshared-key", "/dev/stdin")
		stdin = *strings.NewReader(pskB64 + "\n")
		hasStdin = true
	}

	if endpoint != "" {
		args = append(args, "endpoint", endpoint)
	}

	if allowedIPs != "" {
		args = append(args, "allowed-ips", allowedIPs)
	}

	// Add persistent keepalive for NAT traversal
	args = append(args, "persistent-keepalive", "25")

	cmd := exec.Command("wg", args...)
	if hasStdin {
		cmd.Stdin = &stdin
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("wg set failed: %s: %w", string(output), err)
	}

	return nil
}

// RemovePeer removes a peer from the local WireGuard interface
func RemovePeer(iface, pubKey string) error {
	cmd := exec.Command("wg", "set", iface, "peer", pubKey, "remove")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("wg set peer remove failed: %s: %w", string(output), err)
	}
	return nil
}

// GetPeers returns the list of peers on the local WireGuard interface
func GetPeers(iface string) ([]WGPeer, error) {
	cmd := exec.Command("wg", "show", iface, "peers")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("wg show peers failed: %w", err)
	}

	var peers []WGPeer
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			peers = append(peers, WGPeer{PublicKey: line})
		}
	}

	return peers, nil
}

// GetLatestHandshakes returns the most recent handshake time for each WG peer.
// Returns a map of public key → Unix timestamp (0 means no handshake yet).
func GetLatestHandshakes(iface string) (map[string]int64, error) {
	cmd := exec.Command("wg", "show", iface, "latest-handshakes")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("wg show latest-handshakes failed: %w", err)
	}

	result := make(map[string]int64)
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: <pubkey>\t<unix_timestamp>
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		var ts int64
		fmt.Sscanf(parts[1], "%d", &ts)
		result[parts[0]] = ts
	}

	return result, nil
}
