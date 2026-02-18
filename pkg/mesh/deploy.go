package mesh

import (
	"fmt"
	"os"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh"
	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

type WireGuardConfig = wireguard.FullConfig
type WGInterface = wireguard.WGInterface
type WGPeer = wireguard.WGPeer

func (m *Mesh) Deploy() error {
	// Validate groups and policies if access control is enabled
	if m.IsAccessControlEnabled() {
		fmt.Println("Validating access control configuration...")

		if err := m.ValidateGroups(); err != nil {
			return fmt.Errorf("groups validation failed: %w", err)
		}

		if err := m.ValidatePolicies(); err != nil {
			return fmt.Errorf("policies validation failed: %w", err)
		}

		// Warn if groups exist without policies
		if m.HasGroups() && !m.HasPolicies() {
			fmt.Println("Warning: Groups are defined but no access policies exist.")
			fmt.Println("         Nodes in groups will have no connectivity unless policies are added.")
		}

		fmt.Println("Access control configuration valid.")
	}

	if err := m.detectEndpoints(); err != nil {
		return fmt.Errorf("failed to detect endpoints: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for hostname, node := range m.Nodes {
		fmt.Printf("Deploying to %s...\n", hostname)

		client, err := ssh.NewClient(node.SSHHost, node.SSHPort)
		if err != nil {
			return fmt.Errorf("failed to connect to %s: %w", hostname, err)
		}
		defer client.Close()

		if err := ssh.EnsureWireGuardInstalled(client); err != nil {
			return fmt.Errorf("failed to ensure WireGuard on %s: %w", hostname, err)
		}

		config := m.generateConfigForNode(node)
		desiredRoutes := m.collectAllRoutesForNode(node)

		currentConfig, err := wireguard.GetCurrentConfig(client, m.InterfaceName)
		if err != nil {
			fmt.Printf("  No existing config, applying fresh persistent configuration\n")
			if err := wireguard.ApplyPersistentConfig(client, m.InterfaceName, config, desiredRoutes); err != nil {
				return fmt.Errorf("failed to apply config to %s: %w", hostname, err)
			}
		} else {
			diff := wireguard.CalculateDiff(currentConfig, wireguard.FullConfigToConfig(config))
			if diff.HasChanges() {
				fmt.Printf("  Applying changes with persistent configuration\n")
				if err := wireguard.UpdatePersistentConfig(client, m.InterfaceName, config, desiredRoutes, diff); err != nil {
					return fmt.Errorf("failed to update config on %s: %w", hostname, err)
				}
			} else {
				fmt.Printf("  No WireGuard peer changes needed\n")
			}

			// Always check and sync routes
			if err := m.syncRoutesForNode(client, node, desiredRoutes); err != nil {
				return fmt.Errorf("failed to sync routes on %s: %w", hostname, err)
			}

			// Always ensure config file is up to date
			configContent := wireguard.GenerateWgQuickConfig(config, desiredRoutes)
			configPath := fmt.Sprintf("/etc/wireguard/%s.conf", m.InterfaceName)
			if err := client.WriteFile(configPath, []byte(configContent), 0600); err != nil {
				fmt.Printf("  Warning: failed to update config file: %v\n", err)
			}
		}

		fmt.Printf("  ✓ Deployed successfully\n\n")
	}

	return nil
}

func (m *Mesh) detectEndpoints() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for hostname, node := range m.Nodes {
		if node.IsLocal {
			// For local node, get hostname directly
			if node.ActualHostname == "" {
				if h, err := os.Hostname(); err == nil {
					node.ActualHostname = h
				}
			}
			continue
		}

		client, err := ssh.NewClient(node.SSHHost, node.SSHPort)
		if err != nil {
			return fmt.Errorf("failed to connect to %s: %w", hostname, err)
		}

		// Collect hostname and FQDN
		if actualHostname, err := ssh.GetHostname(client); err == nil {
			node.ActualHostname = actualHostname
		} else {
			fmt.Printf("Warning: failed to get hostname for %s: %v\n", hostname, err)
		}

		// FQDN may not be configured on all systems, silently ignore errors to avoid cluttering output
		if fqdn, err := ssh.GetFQDN(client); err == nil {
			node.FQDN = fqdn
		}

		publicIP, err := ssh.DetectPublicIP(client)
		client.Close()

		if err != nil {
			fmt.Printf("Warning: failed to detect public IP for %s: %v\n", hostname, err)
			node.BehindNAT = true
			continue
		}

		if publicIP != "" && publicIP != node.SSHHost {
			node.BehindNAT = true
			fmt.Printf("Detected %s is behind NAT (public IP: %s)\n", hostname, publicIP)
		} else {
			node.PublicEndpoint = fmt.Sprintf("%s:%d", node.SSHHost, node.ListenPort)
			fmt.Printf("Detected %s has public endpoint: %s\n", hostname, node.PublicEndpoint)
		}
	}

	return nil
}

func (m *Mesh) collectRoutesForNode(node *Node) []ssh.RouteEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	routes := make([]ssh.RouteEntry, 0)

	for peerHostname, peer := range m.Nodes {
		if peerHostname == node.Hostname {
			continue
		}

		for _, network := range peer.RoutableNetworks {
			routes = append(routes, ssh.RouteEntry{
				Network: network,
				Gateway: peer.MeshIP.String(),
			})
		}
	}

	return routes
}

func (m *Mesh) collectAllRoutesForNode(node *Node) []ssh.RouteEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	routes := make([]ssh.RouteEntry, 0)

	// Add this node's own networks (direct routes, no gateway)
	for _, network := range node.RoutableNetworks {
		routes = append(routes, ssh.RouteEntry{
			Network: network,
			Gateway: "",
		})
	}

	// Check if access control is enabled
	if m.IsAccessControlEnabled() {
		// Use policy-based route collection
		allowedPeers := m.GetAllowedPeers(node.Hostname)
		for peerHostname, access := range allowedPeers {
			peer := m.Nodes[peerHostname]
			if access.AllowRoutableNetworks {
				for _, network := range peer.RoutableNetworks {
					routes = append(routes, ssh.RouteEntry{
						Network: network,
						Gateway: peer.MeshIP.String(),
					})
				}
			}
		}
	} else {
		// Default: all nodes' networks (current behavior)
		for peerHostname, peer := range m.Nodes {
			if peerHostname == node.Hostname {
				continue
			}

			for _, network := range peer.RoutableNetworks {
				routes = append(routes, ssh.RouteEntry{
					Network: network,
					Gateway: peer.MeshIP.String(),
				})
			}
		}
	}

	return routes
}

func (m *Mesh) syncRoutesForNode(client *ssh.Client, node *Node, desiredRoutes []ssh.RouteEntry) error {
	currentRoutes, err := ssh.GetCurrentRoutes(client, m.InterfaceName)
	if err != nil {
		fmt.Printf("  Warning: could not get current routes, will try to add all: %v\n", err)
		// If we can't get current routes, just try to add desired ones
		for _, route := range desiredRoutes {
			var cmd string
			if route.Gateway != "" {
				cmd = fmt.Sprintf("ip route add %s via %s dev %s || ip route replace %s via %s dev %s",
					route.Network, route.Gateway, m.InterfaceName, route.Network, route.Gateway, m.InterfaceName)
			} else {
				cmd = fmt.Sprintf("ip route add %s dev %s || ip route replace %s dev %s",
					route.Network, m.InterfaceName, route.Network, m.InterfaceName)
			}
			client.RunQuiet(cmd)
		}
		return nil
	}

	toAdd, toRemove := ssh.CalculateRouteDiff(currentRoutes, desiredRoutes)
	return ssh.ApplyRouteDiff(client, m.InterfaceName, toAdd, toRemove)
}

func (m *Mesh) generateConfigForNode(node *Node) *WireGuardConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config := &WireGuardConfig{
		Interface: WGInterface{
			PrivateKey: node.PrivateKey,
			Address:    fmt.Sprintf("%s/16", node.MeshIP.String()),
			ListenPort: node.ListenPort,
		},
		Peers: make([]WGPeer, 0),
	}

	// Check if access control is enabled
	if m.IsAccessControlEnabled() {
		// Use policy-based peer selection
		allowedPeers := m.GetAllowedPeers(node.Hostname)
		for peerHostname, access := range allowedPeers {
			peer := m.Nodes[peerHostname]
			peerConfig := m.buildPeerConfig(peer, access)
			config.Peers = append(config.Peers, peerConfig)
		}
	} else {
		// Default: full mesh (current behavior)
		for peerHostname, peer := range m.Nodes {
			if peerHostname == node.Hostname {
				continue
			}
			peerConfig := m.buildPeerConfigFullAccess(peer)
			config.Peers = append(config.Peers, peerConfig)
		}
	}

	return config
}

// buildPeerConfig creates a WireGuard peer config with access control
func (m *Mesh) buildPeerConfig(peer *Node, access *PeerAccess) WGPeer {
	allowedIPs := []string{}

	// Always include mesh /32 for handshakes when peer is configured
	allowedIPs = append(allowedIPs, fmt.Sprintf("%s/32", peer.MeshIP.String()))

	// Add routable networks only if policy permits
	if access.AllowRoutableNetworks {
		allowedIPs = append(allowedIPs, peer.RoutableNetworks...)
	}

	peerConfig := WGPeer{
		PublicKey:  peer.PublicKey,
		AllowedIPs: allowedIPs,
	}

	if peer.PublicEndpoint != "" {
		peerConfig.Endpoint = peer.PublicEndpoint
	}

	peerConfig.PersistentKeepalive = 5

	return peerConfig
}

// buildPeerConfigFullAccess creates a WireGuard peer config with full access (backward compatibility)
func (m *Mesh) buildPeerConfigFullAccess(peer *Node) WGPeer {
	allowedIPs := []string{fmt.Sprintf("%s/32", peer.MeshIP.String())}
	allowedIPs = append(allowedIPs, peer.RoutableNetworks...)

	peerConfig := WGPeer{
		PublicKey:  peer.PublicKey,
		AllowedIPs: allowedIPs,
	}

	if peer.PublicEndpoint != "" {
		peerConfig.Endpoint = peer.PublicEndpoint
	}

	peerConfig.PersistentKeepalive = 5

	return peerConfig
}
