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

	const (
		aptUpdate  = "apt update -qq"
		aptInstall = "DEBIAN_FRONTEND=noninteractive apt install -y -qq wireguard wireguard-tools"
		modprobe   = "modprobe wireguard || true"
	)

	if _, err := client.Run(aptUpdate); err != nil {
		return fmt.Errorf("failed to run %q: %w", aptUpdate, err)
	}
	if _, err := client.Run(aptInstall); err != nil {
		return fmt.Errorf("failed to run %q: %w", aptInstall, err)
	}
	if _, err := client.Run(modprobe); err != nil {
		return fmt.Errorf("failed to run %q: %w", modprobe, err)
	}

	return nil
}

// detectPublicIPCommand asks two services for the node's public IPv4; curl is bounded so a node without egress fails fast.
const detectPublicIPCommand = "curl -s -4 --max-time 5 ifconfig.me || curl -s -4 --max-time 5 icanhazip.com || true"

func DetectPublicIP(client *Client) (string, error) {
	output, err := client.Run(detectPublicIPCommand)
	if err != nil {
		return "", err
	}

	ip := strings.TrimSpace(output)
	if ip == "" {
		return "", fmt.Errorf("could not detect public IP (each curl call is bounded to 5 s; a slow or offline node fails)")
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
	fmt.Printf("  Updating routing table...\n")

	for _, network := range networks {
		if err := client.RunQuiet(Cmd("ip", RouteArgs("add", network, "", iface)...) +
			" || " + Cmd("ip", RouteArgs("replace", network, "", iface)...)); err != nil {
			return fmt.Errorf("failed to add route for %s: %w", network, err)
		}
		fmt.Printf("    Added route: %s\n", network)
	}

	const cmd = "sysctl -w net.ipv4.ip_forward=1 > /dev/null"
	if err := client.RunQuiet(cmd); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %w", err)
	}

	return nil
}

func UpdateRoutingTableWithGateways(client *Client, iface string, routes []RouteEntry) error {
	fmt.Printf("  Updating routing table with gateways...\n")

	for _, route := range routes {
		if err := client.RunQuiet(Cmd("ip", RouteArgs("add", route.Network, route.Gateway, iface)...) +
			" || " + Cmd("ip", RouteArgs("replace", route.Network, route.Gateway, iface)...)); err != nil {
			return fmt.Errorf("failed to add route for %s via %s: %w", route.Network, route.Gateway, err)
		}
		fmt.Printf("    Added route: %s via %s\n", route.Network, route.Gateway)
	}

	const cmd = "sysctl -w net.ipv4.ip_forward=1 > /dev/null"
	if err := client.RunQuiet(cmd); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %w", err)
	}

	return nil
}
