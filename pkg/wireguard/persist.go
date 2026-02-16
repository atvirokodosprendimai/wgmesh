package wireguard

import (
	"fmt"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh"
)

// GenerateWgQuickConfig generates a wg-quick compatible configuration string.
func GenerateWgQuickConfig(config *FullConfig, routes []ssh.RouteEntry) string {
	var sb strings.Builder

	sb.WriteString("[Interface]\n")
	sb.WriteString(fmt.Sprintf("Address = %s\n", config.Interface.Address))
	sb.WriteString(fmt.Sprintf("ListenPort = %d\n", config.Interface.ListenPort))
	sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", config.Interface.PrivateKey))

	// Add PostUp commands for additional routes
	if len(routes) > 0 {
		for _, route := range routes {
			sb.WriteString(fmt.Sprintf("PostUp = ip route add %s via %s dev %%i || true\n",
				route.Network, route.Gateway))
		}
	}

	// Add PreDown commands to clean up routes
	if len(routes) > 0 {
		for _, route := range routes {
			sb.WriteString(fmt.Sprintf("PreDown = ip route del %s via %s dev %%i || true\n",
				route.Network, route.Gateway))
		}
	}

	// Enable IP forwarding
	sb.WriteString("PostUp = sysctl -w net.ipv4.ip_forward=1\n")

	sb.WriteString("\n")

	for _, peer := range config.Peers {
		sb.WriteString("[Peer]\n")
		sb.WriteString(fmt.Sprintf("PublicKey = %s\n", peer.PublicKey))

		if peer.Endpoint != "" {
			sb.WriteString(fmt.Sprintf("Endpoint = %s\n", peer.Endpoint))
		}

		if len(peer.AllowedIPs) > 0 {
			sb.WriteString(fmt.Sprintf("AllowedIPs = %s\n", strings.Join(peer.AllowedIPs, ", ")))
		}

		if peer.PersistentKeepalive > 0 {
			sb.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", peer.PersistentKeepalive))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// ApplyPersistentConfig applies a persistent WireGuard configuration via wg-quick.
func ApplyPersistentConfig(client *ssh.Client, iface string, config *FullConfig, routes []ssh.RouteEntry) error {
	configContent := GenerateWgQuickConfig(config, routes)
	configPath := fmt.Sprintf("/etc/wireguard/%s.conf", iface)

	fmt.Printf("  Writing persistent configuration to %s\n", configPath)

	if err := client.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("  Enabling wg-quick@%s service\n", iface)
	if _, err := client.Run(fmt.Sprintf("systemctl enable wg-quick@%s", iface)); err != nil {
		return fmt.Errorf("failed to enable systemd service: %w", err)
	}

	fmt.Printf("  Restarting wg-quick@%s service\n", iface)
	if _, err := client.Run(fmt.Sprintf("systemctl restart wg-quick@%s", iface)); err != nil {
		return fmt.Errorf("failed to restart service: %w", err)
	}

	return nil
}

// UpdatePersistentConfig updates a persistent WireGuard configuration with minimal disruption.
func UpdatePersistentConfig(client *ssh.Client, iface string, config *FullConfig, routes []ssh.RouteEntry, diff *ConfigDiff) error {
	if diff.InterfaceChanged || !canUseOnlineUpdate(diff) {
		fmt.Printf("  Significant changes detected, applying full persistent config\n")
		return ApplyPersistentConfig(client, iface, config, routes)
	}

	fmt.Printf("  Minor changes detected, updating online\n")
	if err := UpdatePersistentConfigFile(client, iface, config, routes); err != nil {
		return err
	}

	return nil
}

func UpdatePersistentConfigFile(client *ssh.Client, iface string, config *FullConfig, routes []ssh.RouteEntry) error {
	configContent := GenerateWgQuickConfig(config, routes)
	configPath := fmt.Sprintf("/etc/wireguard/%s.conf", iface)

	if err := client.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to update config file: %w", err)
	}

	fmt.Printf("  Updated persistent configuration at %s\n", configPath)
	return nil
}

func canUseOnlineUpdate(diff *ConfigDiff) bool {
	// Can use online update if only peers changed (no interface changes)
	return !diff.InterfaceChanged
}

// RemovePersistentConfig removes a persistent WireGuard configuration.
func RemovePersistentConfig(client *ssh.Client, iface string) error {
	fmt.Printf("  Stopping and disabling wg-quick@%s service\n", iface)

	client.RunQuiet(fmt.Sprintf("systemctl stop wg-quick@%s", iface))
	client.RunQuiet(fmt.Sprintf("systemctl disable wg-quick@%s", iface))

	configPath := fmt.Sprintf("/etc/wireguard/%s.conf", iface)
	if _, err := client.Run(fmt.Sprintf("rm -f %s", configPath)); err != nil {
		return fmt.Errorf("failed to remove config file: %w", err)
	}

	return nil
}
