package wireguard

import (
	"fmt"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh"
)

// GenerateWgQuickConfig renders a wg-quick compatible configuration file for
// the given FullConfig and optional extra routes.
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

// ApplyPersistentConfig writes a wg-quick configuration file to the remote
// host and (re)starts the wg-quick systemd service.
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

// UpdatePersistentConfig applies a diff to a running WireGuard interface and
// updates the persistent wg-quick config file. Falls back to a full config
// apply when interface parameters change.
func UpdatePersistentConfig(client *ssh.Client, iface string, config *FullConfig, routes []ssh.RouteEntry, diff *ConfigDiff) error {
	if diff.InterfaceChanged || !canUseOnlineUpdate(diff) {
		fmt.Printf("  Significant changes detected, applying full persistent config\n")
		return ApplyPersistentConfig(client, iface, config, routes)
	}

	fmt.Printf("  Applying online peer updates and updating persistent config\n")

	configContent := GenerateWgQuickConfig(config, routes)
	configPath := fmt.Sprintf("/etc/wireguard/%s.conf", iface)

	if err := client.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	if err := ApplyDiff(client, iface, diff); err != nil {
		return fmt.Errorf("failed to apply diff: %w", err)
	}

	// Note: Routes are now synced separately in the deploy logic

	return nil
}

func canUseOnlineUpdate(diff *ConfigDiff) bool {
	// Can use online update if only peers changed (no interface changes)
	return !diff.InterfaceChanged
}

// RemovePersistentConfig stops the wg-quick service and deletes the
// WireGuard configuration file from the remote host.
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
