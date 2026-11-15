package wireguard

import (
	"fmt"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh"
)

// GenerateWgQuickConfig renders a wg-quick compatible configuration file for
// the given FullConfig and optional extra routes. It returns an error naming
// the field when a value holds a newline, a carriage return or NUL: a line
// of the file would end there and the rest would be read as a new directive
// (PostUp runs a shell). The route values of the PostUp and PreDown lines go
// through ssh.Cmd, so wg-quick's shell reads each as one word.
func GenerateWgQuickConfig(config *FullConfig, routes []ssh.RouteEntry) (string, error) {
	if err := refuseControlCharacters(config, routes); err != nil {
		return "", err
	}

	var sb strings.Builder

	sb.WriteString("[Interface]\n")
	sb.WriteString(fmt.Sprintf("Address = %s\n", config.Interface.Address))
	sb.WriteString(fmt.Sprintf("ListenPort = %d\n", config.Interface.ListenPort))
	sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", config.Interface.PrivateKey))

	// Add PostUp commands for additional routes
	if len(routes) > 0 {
		for _, route := range routes {
			sb.WriteString("PostUp = " + ssh.Cmd("ip", ssh.RouteArgs("add", route.Network, route.Gateway, "%i")...) + " || true\n")
		}
	}

	// Add PreDown commands to clean up routes
	if len(routes) > 0 {
		for _, route := range routes {
			sb.WriteString("PreDown = " + ssh.Cmd("ip", ssh.RouteArgs("del", route.Network, route.Gateway, "%i")...) + " || true\n")
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

	return sb.String(), nil
}

// refuseControlCharacters returns an error naming the first field of the
// config or of a route that holds a newline, a carriage return or NUL. It
// never prints a value: the private key is one of them.
func refuseControlCharacters(config *FullConfig, routes []ssh.RouteEntry) error {
	check := func(field, value string) error {
		if strings.ContainsAny(value, "\n\r\x00") {
			return fmt.Errorf("wg-quick config: %s holds a newline, carriage return or NUL", field)
		}
		return nil
	}
	if err := check("address", config.Interface.Address); err != nil {
		return err
	}
	if err := check("private key", config.Interface.PrivateKey); err != nil {
		return err
	}
	for i, peer := range config.Peers {
		if err := check(fmt.Sprintf("public key of peer %d", i), peer.PublicKey); err != nil {
			return err
		}
		if err := check(fmt.Sprintf("endpoint of peer %d", i), peer.Endpoint); err != nil {
			return err
		}
		for _, ip := range peer.AllowedIPs {
			if err := check(fmt.Sprintf("allowed IP of peer %d", i), ip); err != nil {
				return err
			}
		}
	}
	for i, route := range routes {
		if err := check(fmt.Sprintf("route network of route %d", i), route.Network); err != nil {
			return err
		}
		if err := check(fmt.Sprintf("route gateway of route %d", i), route.Gateway); err != nil {
			return err
		}
	}
	return nil
}

// ApplyPersistentConfig writes a wg-quick configuration file to the remote
// host and (re)starts the wg-quick systemd service.
func ApplyPersistentConfig(client *ssh.Client, iface string, config *FullConfig, routes []ssh.RouteEntry) error {
	configContent, err := GenerateWgQuickConfig(config, routes)
	if err != nil {
		return fmt.Errorf("failed to render config: %w", err)
	}
	configPath := fmt.Sprintf("/etc/wireguard/%s.conf", iface)

	fmt.Printf("  Writing persistent configuration to %s\n", configPath)

	if err := client.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("  Enabling wg-quick@%s service\n", iface)
	if _, err := client.Run(ssh.Cmd("systemctl", "enable", "wg-quick@"+iface)); err != nil {
		return fmt.Errorf("failed to enable systemd service: %w", err)
	}

	fmt.Printf("  Restarting wg-quick@%s service\n", iface)
	if _, err := client.Run(ssh.Cmd("systemctl", "restart", "wg-quick@"+iface)); err != nil {
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

	configContent, err := GenerateWgQuickConfig(config, routes)
	if err != nil {
		return fmt.Errorf("failed to render config: %w", err)
	}
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

	client.RunQuiet(ssh.Cmd("systemctl", "stop", "wg-quick@"+iface))
	client.RunQuiet(ssh.Cmd("systemctl", "disable", "wg-quick@"+iface))

	configPath := fmt.Sprintf("/etc/wireguard/%s.conf", iface)
	if _, err := client.Run(ssh.Cmd("rm", "-f", configPath)); err != nil {
		return fmt.Errorf("failed to remove config file: %w", err)
	}

	return nil
}
