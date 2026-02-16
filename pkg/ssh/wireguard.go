package ssh

import (
	"fmt"
	"strings"
)

func EnsureWireGuardInstalled(client *Client) error {
	output, err := client.Run("which wg")
	if err == nil && strings.Contains(output, "/wg") {
		return nil
	}

	fmt.Println("  Installing WireGuard...")

	commands := []string{
		"apt update -qq",
		"DEBIAN_FRONTEND=noninteractive apt install -y -qq wireguard wireguard-tools",
		"modprobe wireguard || true",
	}

	for _, cmd := range commands {
		if _, err := client.Run(cmd); err != nil {
			return fmt.Errorf("failed to run %q: %w", cmd, err)
		}
	}

	return nil
}

func DetectPublicIP(client *Client) (string, error) {
	output, err := client.Run("curl -s -4 ifconfig.me || curl -s -4 icanhazip.com || true")
	if err != nil {
		return "", err
	}

	ip := strings.TrimSpace(output)
	if ip == "" {
		return "", fmt.Errorf("could not detect public IP")
	}

	return ip, nil
}

func GetHostname(client *Client) (string, error) {
	output, err := client.Run("hostname")
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}

	hostname := strings.TrimSpace(output)
	if hostname == "" {
		return "", fmt.Errorf("hostname is empty")
	}

	return hostname, nil
}

func GetFQDN(client *Client) (string, error) {
	output, err := client.Run("hostname -f")
	if err != nil {
		return "", fmt.Errorf("failed to get FQDN: %w", err)
	}

	fqdn := strings.TrimSpace(output)
	if fqdn == "" {
		return "", fmt.Errorf("FQDN is empty")
	}

	return fqdn, nil
}

func UpdateRoutingTable(client *Client, iface string, networks []string) error {
	if err := ValidateIface(iface); err != nil {
		return fmt.Errorf("UpdateRoutingTable: %w", err)
	}
	fmt.Printf("  Updating routing table...\n")

	for _, network := range networks {
		if err := ValidateCIDR(network); err != nil {
			return fmt.Errorf("unsafe network %q: %w", network, err)
		}
		cmd := fmt.Sprintf("ip route add %s dev %s || ip route replace %s dev %s",
			network, iface, network, iface)
		if err := client.RunQuiet(cmd); err != nil {
			return fmt.Errorf("failed to add route for %s: %w", network, err)
		}
		fmt.Printf("    Added route: %s\n", network)
	}

	cmd := "sysctl -w net.ipv4.ip_forward=1 > /dev/null"
	if err := client.RunQuiet(cmd); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %w", err)
	}

	return nil
}

func UpdateRoutingTableWithGateways(client *Client, iface string, routes []RouteEntry) error {
	if err := ValidateIface(iface); err != nil {
		return fmt.Errorf("UpdateRoutingTableWithGateways: %w", err)
	}
	fmt.Printf("  Updating routing table with gateways...\n")

	for _, route := range routes {
		if err := ValidateCIDR(route.Network); err != nil {
			return fmt.Errorf("unsafe network %q: %w", route.Network, err)
		}
		if err := ValidateEndpoint(route.Gateway); err != nil {
			return fmt.Errorf("unsafe gateway %q: %w", route.Gateway, err)
		}
		cmd := fmt.Sprintf("ip route add %s via %s dev %s || ip route replace %s via %s dev %s",
			route.Network, route.Gateway, iface, route.Network, route.Gateway, iface)
		if err := client.RunQuiet(cmd); err != nil {
			return fmt.Errorf("failed to add route for %s via %s: %w", route.Network, route.Gateway, err)
		}
		fmt.Printf("    Added route: %s via %s\n", route.Network, route.Gateway)
	}

	cmd := "sysctl -w net.ipv4.ip_forward=1 > /dev/null"
	if err := client.RunQuiet(cmd); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %w", err)
	}

	return nil
}
