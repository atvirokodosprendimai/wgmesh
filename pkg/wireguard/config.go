package wireguard

import (
	"fmt"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh"
)

type Config struct {
	Interface Interface
	Peers     map[string]Peer
}

type Interface struct {
	PrivateKey string
	Address    string
	ListenPort int
}

type Peer struct {
	PublicKey           string
	PresharedKey        string
	Endpoint            string
	AllowedIPs          []string
	PersistentKeepalive int
}

type ConfigDiff struct {
	InterfaceChanged bool
	AddedPeers       map[string]Peer
	RemovedPeers     []string
	ModifiedPeers    map[string]Peer
}

func GetCurrentConfig(client *ssh.Client, iface string) (*Config, error) {
	output, err := client.Run(fmt.Sprintf("wg show %s dump 2>/dev/null || true", iface))
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(output) == "" {
		return nil, fmt.Errorf("interface does not exist or no config")
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 1 {
		return nil, fmt.Errorf("invalid wg dump output")
	}

	config := &Config{
		Peers: make(map[string]Peer),
	}

	parts := strings.Fields(lines[0])
	if len(parts) >= 3 {
		config.Interface.PrivateKey = parts[0]
		fmt.Sscanf(parts[2], "%d", &config.Interface.ListenPort)
	}

	for i := 1; i < len(lines); i++ {
		parts := strings.Fields(lines[i])
		if len(parts) < 4 {
			continue
		}

		publicKey := parts[0]
		presharedKey := parts[1]
		endpoint := parts[2]
		allowedIPs := strings.Split(parts[3], ",")
		var keepalive int
		if len(parts) >= 5 {
			fmt.Sscanf(parts[4], "%d", &keepalive)
		}

		peer := Peer{
			PublicKey:           publicKey,
			PresharedKey:        presharedKey,
			Endpoint:            endpoint,
			AllowedIPs:          allowedIPs,
			PersistentKeepalive: keepalive,
		}

		config.Peers[publicKey] = peer
	}

	return config, nil
}

func CalculateDiff(current, desired *Config) *ConfigDiff {
	diff := &ConfigDiff{
		AddedPeers:    make(map[string]Peer),
		RemovedPeers:  make([]string, 0),
		ModifiedPeers: make(map[string]Peer),
	}

	if current.Interface.ListenPort != desired.Interface.ListenPort {
		diff.InterfaceChanged = true
	}

	for pubKey := range current.Peers {
		if _, exists := desired.Peers[pubKey]; !exists {
			diff.RemovedPeers = append(diff.RemovedPeers, pubKey)
		}
	}

	for pubKey, desiredPeer := range desired.Peers {
		currentPeer, exists := current.Peers[pubKey]
		if !exists {
			diff.AddedPeers[pubKey] = desiredPeer
		} else if !peersEqual(currentPeer, desiredPeer) {
			diff.ModifiedPeers[pubKey] = desiredPeer
		}
	}

	return diff
}

func (d *ConfigDiff) HasChanges() bool {
	return d.InterfaceChanged || len(d.AddedPeers) > 0 || len(d.RemovedPeers) > 0 || len(d.ModifiedPeers) > 0
}

func peersEqual(a, b Peer) bool {
	if a.PresharedKey != b.PresharedKey {
		return false
	}

	if a.Endpoint != b.Endpoint {
		return false
	}

	if a.PersistentKeepalive != b.PersistentKeepalive {
		return false
	}

	if len(a.AllowedIPs) != len(b.AllowedIPs) {
		return false
	}

	allowedIPsA := make(map[string]bool)
	for _, ip := range a.AllowedIPs {
		allowedIPsA[ip] = true
	}

	for _, ip := range b.AllowedIPs {
		if !allowedIPsA[ip] {
			return false
		}
	}

	return true
}

func ApplyDiff(client *ssh.Client, iface string, diff *ConfigDiff) error {
	if diff.InterfaceChanged {
		return fmt.Errorf("interface changes require full reconfig")
	}

	for _, pubKey := range diff.RemovedPeers {
		cmd := fmt.Sprintf("wg set %s peer %s remove", iface, pubKey)
		if _, err := client.Run(cmd); err != nil {
			return fmt.Errorf("failed to remove peer: %w", err)
		}
		fmt.Printf("    Removed peer: %s\n", pubKey[:16])
	}

	for pubKey, peer := range diff.AddedPeers {
		if err := addOrUpdatePeer(client, iface, pubKey, peer); err != nil {
			return err
		}
		fmt.Printf("    Added peer: %s\n", pubKey[:16])
	}

	for pubKey, peer := range diff.ModifiedPeers {
		if err := addOrUpdatePeer(client, iface, pubKey, peer); err != nil {
			return err
		}
		fmt.Printf("    Updated peer: %s\n", pubKey[:16])
	}

	return nil
}

func addOrUpdatePeer(client *ssh.Client, iface string, pubKey string, peer Peer) error {
	cmd := fmt.Sprintf("wg set %s peer %s", iface, pubKey)

	// Handle PSK if present
	var stdinContent string
	if peer.PresharedKey != "" && peer.PresharedKey != "(none)" {
		cmd += " preshared-key /dev/stdin"
		stdinContent = peer.PresharedKey + "\n"
	}

	if peer.Endpoint != "" && peer.Endpoint != "(none)" {
		cmd += fmt.Sprintf(" endpoint %s", peer.Endpoint)
	}

	if len(peer.AllowedIPs) > 0 {
		cmd += fmt.Sprintf(" allowed-ips %s", strings.Join(peer.AllowedIPs, ","))
	}

	if peer.PersistentKeepalive > 0 {
		cmd += fmt.Sprintf(" persistent-keepalive %d", peer.PersistentKeepalive)
	}

	// Execute command with PSK via stdin if needed
	if stdinContent != "" {
		if _, err := client.RunWithStdin(cmd, stdinContent); err != nil {
			return fmt.Errorf("failed to configure peer: %w", err)
		}
	} else {
		if _, err := client.Run(cmd); err != nil {
			return fmt.Errorf("failed to configure peer: %w", err)
		}
	}

	return nil
}
