package mesh

import (
	"fmt"
	"maps"
	"net"
	"os"
	"slices"
	"strings"

	netroutes "github.com/atvirokodosprendimai/wgmesh/pkg/routes"
	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh"
	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

type WireGuardConfig = wireguard.FullConfig
type WGInterface = wireguard.WGInterface
type WGPeer = wireguard.WGPeer

func (m *Mesh) Deploy() error {
	// First, before any connection: refuse a state a shell or a wg-quick file
	// would give a meaning to (ADR-0006, decision 2). Not under m.mu.
	if err := m.validateForDeploy(os.Stdout); err != nil {
		return err
	}

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

	// A node that cannot be reached is recorded, not dialled again below.
	detectFailures := m.detectEndpoints()

	m.mu.Lock()
	defer m.mu.Unlock()

	results := make([]NodeResult, 0, len(m.Nodes))
	for _, hostname := range slices.Sorted(maps.Keys(m.Nodes)) {
		fmt.Printf("Deploying to %s...\n", hostname)

		result := NodeResult{Hostname: hostname, Phase: PhaseDetect, Err: detectFailures[hostname]}
		if result.Err == nil {
			result = m.deployNode(hostname, m.Nodes[hostname])
		}
		results = append(results, result)
		if result.Err != nil {
			fmt.Printf("  ✗ Failed in %s: %v\n\n", result.Phase, result.Err)
			continue
		}
		fmt.Printf("  ✓ Deployed successfully\n\n")
	}

	var warned []string
	for _, r := range results {
		for _, w := range r.Warnings {
			warned = append(warned, fmt.Sprintf("  %s: %s", r.Hostname, w))
		}
	}
	if len(warned) > 0 {
		fmt.Printf("Warnings:\n%s\n", strings.Join(warned, "\n"))
	}

	return deployResult(results)
}

// deployNode configures one node and reports how it went. It requires the
// caller to hold m.mu and calls only helpers that require it held. The wg-quick
// text is rendered before the first connection, so a node whose config cannot
// be rendered is left untouched.
func (m *Mesh) deployNode(hostname string, node *Node) NodeResult {
	result := NodeResult{Hostname: hostname}
	fail := func(phase Phase, err error) NodeResult {
		result.Phase, result.Err = phase, err
		return result
	}

	config := m.generateConfigForNode(node)
	desiredRoutes := m.collectAllRoutesForNode(node)
	configContent, err := wireguard.GenerateWgQuickConfig(config, desiredRoutes)
	if err != nil {
		return fail(PhasePersist, fmt.Errorf("failed to render config: %w", err))
	}

	client, err := ssh.NewClient(node.SSHHost, node.SSHPort)
	if err != nil {
		return fail(PhaseConnect, err)
	}
	defer client.Close()

	if err := ssh.EnsureWireGuardInstalled(client); err != nil {
		return fail(PhaseInstall, err)
	}

	currentConfig, err := wireguard.GetCurrentConfig(client, m.InterfaceName)
	if err != nil {
		fmt.Printf("  No existing config, applying fresh persistent configuration\n")
		if err := wireguard.ApplyPersistentConfig(client, m.InterfaceName, config, desiredRoutes); err != nil {
			return fail(PhaseApply, err)
		}
		return result
	}

	diff := wireguard.CalculateDiff(currentConfig, wireguard.FullConfigToConfig(config))
	if diff.HasChanges() {
		fmt.Printf("  Applying changes with persistent configuration\n")
		if err := wireguard.UpdatePersistentConfig(client, m.InterfaceName, config, desiredRoutes, diff); err != nil {
			return fail(PhaseApply, err)
		}
	} else {
		fmt.Printf("  No WireGuard peer changes needed\n")
	}

	// Always check and sync routes
	if err := m.syncRoutesForNode(client, node, desiredRoutes); err != nil {
		return fail(PhaseRoutes, err)
	}

	// Always ensure config file is up to date
	configPath := fmt.Sprintf("/etc/wireguard/%s.conf", m.InterfaceName)
	if err := client.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		if PersistFailureFatal {
			return fail(PhasePersist, fmt.Errorf("failed to update config file %s: %w", configPath, err))
		}
		fmt.Printf("  Warning: failed to update config file %s: %v\n", configPath, err)
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to update config file %s: %v", configPath, err))
	}

	return result
}

// probeResult is what one remote node answered to the public-IP probe.
type probeResult struct {
	publicIP string
	probed   bool
}

// detectEndpoints probes every node that is not local and returns the nodes it
// could not connect to, by hostname. The nodes are visited in sorted order.
// A node whose SSH host is a private IP address is not asked for its public
// IP: it can only differ. Once every node was visited, resolveEndpoint decides
// each reachable node's endpoint; the result stays in memory.
func (m *Mesh) detectEndpoints() map[string]error {
	m.mu.Lock()
	defer m.mu.Unlock()

	failures := map[string]error{}
	probes := map[string]probeResult{}
	hostnames := slices.Sorted(maps.Keys(m.Nodes))
	for _, hostname := range hostnames {
		node := m.Nodes[hostname]
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
			failures[hostname] = err
			continue
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

		if hostIsPrivate(node.SSHHost) {
			client.Close()
			continue
		}
		publicIP, err := ssh.DetectPublicIP(client)
		client.Close()
		if err != nil {
			fmt.Printf("Warning: failed to detect public IP for %s: %v\n", hostname, err)
		}
		probes[hostname] = probeResult{publicIP: publicIP, probed: true}
	}

	// A node that failed to connect is left out of the all-private rule and
	// keeps the endpoint the state holds.
	allPrivate := true
	for _, hostname := range hostnames {
		if _, failed := failures[hostname]; !failed && !hostIsPrivate(m.Nodes[hostname].SSHHost) {
			allPrivate = false
		}
	}
	for _, hostname := range hostnames {
		if _, failed := failures[hostname]; failed {
			continue
		}
		node := m.Nodes[hostname]
		probe := probes[hostname]
		d := resolveEndpoint(node, allPrivate, probe.publicIP, probe.probed)
		if d.Endpoint != "" {
			node.PublicEndpoint = d.Endpoint
		}
		if !node.IsLocal {
			node.BehindNAT = d.BehindNAT
		}
		switch {
		case d.Endpoint != "":
			fmt.Printf("Detected %s has endpoint %s (%s)\n", hostname, d.Endpoint, d.Reason)
		case !node.IsLocal:
			fmt.Printf("Detected %s has no endpoint (%s)\n", hostname, d.Reason)
		}
	}

	return failures
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

// collectAllRoutesForNode requires the caller to hold m.mu.
func (m *Mesh) collectAllRoutesForNode(node *Node) []ssh.RouteEntry {
	// A node is the gateway for its own RoutableNetworks, so they are never
	// routed into its mesh interface; peers reach them via its mesh IP.
	routes := make([]ssh.RouteEntry, 0)

	// Check if access control is enabled
	if m.isAccessControlEnabledLocked() {
		// Use policy-based route collection
		allowedPeers := m.getAllowedPeersLocked(node.Hostname)
		for peerHostname, access := range allowedPeers {
			peer := m.Nodes[peerHostname]
			if access.AllowRoutableNetworks {
				for _, network := range netroutes.ExcludeOwn(peer.RoutableNetworks, node.RoutableNetworks) {
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

			for _, network := range netroutes.ExcludeOwn(peer.RoutableNetworks, node.RoutableNetworks) {
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
			client.RunQuiet(ssh.Cmd("ip", ssh.RouteArgs("add", route.Network, route.Gateway, m.InterfaceName)...) +
				" || " + ssh.Cmd("ip", ssh.RouteArgs("replace", route.Network, route.Gateway, m.InterfaceName)...))
		}
		return nil
	}

	toAdd, toRemove := ssh.CalculateRouteDiff(currentRoutes, desiredRoutes)
	stale := ownNetworkRoutes(currentRoutes, node.RoutableNetworks)
	toRemove = append(toRemove, stale...)
	err = ssh.ApplyRouteDiff(client, m.InterfaceName, toAdd, toRemove)
	// Removals run before adds, so restore even if an add failed.
	for _, route := range stale {
		ssh.RestoreConnectedRoute(client, route.Network)
	}
	return err
}

// ownNetworkRoutes returns the gateway-less mesh-interface routes to a node's
// own networks. Older versions installed them; they blackhole the node's LAN.
// currentRoutes only holds routes on the mesh interface, so the connected LAN
// route is never matched.
func ownNetworkRoutes(currentRoutes []ssh.RouteEntry, ownNetworks []string) []ssh.RouteEntry {
	var stale []ssh.RouteEntry
	for _, route := range currentRoutes {
		if route.Gateway == "" && len(netroutes.ExcludeOwn([]string{route.Network}, ownNetworks)) == 0 {
			stale = append(stale, route)
		}
	}
	return stale
}

// generateConfigForNode requires the caller to hold m.mu.
func (m *Mesh) generateConfigForNode(node *Node) *WireGuardConfig {
	// Extract prefix length from mesh network CIDR
	prefixLen := 16 // default
	if m.Network != "" {
		_, parsedNet, err := net.ParseCIDR(m.Network)
		if err != nil {
			fmt.Printf("Warning: invalid mesh network CIDR %q, using default /16: %v\n", m.Network, err)
		} else {
			ones, _ := parsedNet.Mask.Size()
			prefixLen = ones
		}
	}

	config := &WireGuardConfig{
		Interface: WGInterface{
			PrivateKey: node.PrivateKey,
			Address:    fmt.Sprintf("%s/%d", node.MeshIP.String(), prefixLen),
			ListenPort: node.ListenPort,
		},
		Peers: make([]WGPeer, 0),
	}

	// Check if access control is enabled
	if m.isAccessControlEnabledLocked() {
		// Use policy-based peer selection
		allowedPeers := m.getAllowedPeersLocked(node.Hostname)
		for peerHostname, access := range allowedPeers {
			peer := m.Nodes[peerHostname]
			peerConfig := m.buildPeerConfig(peer, access, node.RoutableNetworks)
			config.Peers = append(config.Peers, peerConfig)
		}
	} else {
		// Default: full mesh (current behavior)
		for peerHostname, peer := range m.Nodes {
			if peerHostname == node.Hostname {
				continue
			}
			peerConfig := m.buildPeerConfigFullAccess(peer, node.RoutableNetworks)
			config.Peers = append(config.Peers, peerConfig)
		}
	}

	return config
}

// buildPeerConfig creates a WireGuard peer config with access control.
// ownNetworks are the local node's RoutableNetworks, never added for the peer.
func (m *Mesh) buildPeerConfig(peer *Node, access *PeerAccess, ownNetworks []string) WGPeer {
	allowedIPs := []string{}

	// Always include mesh /32 for handshakes when peer is configured
	allowedIPs = append(allowedIPs, fmt.Sprintf("%s/32", peer.MeshIP.String()))

	// Add routable networks only if policy permits
	if access.AllowRoutableNetworks {
		allowedIPs = append(allowedIPs, netroutes.ExcludeOwn(peer.RoutableNetworks, ownNetworks)...)
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

// buildPeerConfigFullAccess creates a WireGuard peer config with full access (backward compatibility).
// ownNetworks are the local node's RoutableNetworks, never added for the peer.
func (m *Mesh) buildPeerConfigFullAccess(peer *Node, ownNetworks []string) WGPeer {
	allowedIPs := []string{fmt.Sprintf("%s/32", peer.MeshIP.String())}
	allowedIPs = append(allowedIPs, netroutes.ExcludeOwn(peer.RoutableNetworks, ownNetworks)...)

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
