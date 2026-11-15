package daemon

import (
	"fmt"
	"log"
	"net"
	"runtime"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/pkg/routes"
)

func (d *Daemon) syncPeerRoutes(peers []*PeerInfo) error {
	if runtime.GOOS != "linux" {
		return nil
	}

	desired := make([]routes.Entry, 0)
	relayRoutes := d.currentRelayRoutesSnapshot()
	meshIPByPubKey := make(map[string]string, len(peers))
	for _, p := range peers {
		if p != nil && p.WGPubKey != "" && p.MeshIP != "" {
			meshIPByPubKey[p.WGPubKey] = p.MeshIP
		}
	}
	for _, peer := range peers {
		if peer.WGPubKey == d.localNode.WGPubKey || peer.MeshIP == "" {
			continue
		}
		if d.isTemporarilyOffline(peer.WGPubKey) {
			continue
		}
		gateway := peer.MeshIP
		if relayPubKey, ok := relayRoutes[peer.WGPubKey]; ok {
			if relayIP := meshIPByPubKey[relayPubKey]; relayIP != "" {
				gateway = relayIP
			}
		}
		for _, network := range routes.ExcludeOwn(peer.RoutableNetworks, d.localNode.RoutableNetworks) {
			network = strings.TrimSpace(network)
			if network == "" {
				continue
			}
			desired = append(desired, routes.Entry{Network: network, Gateway: gateway})
		}
	}

	current, err := getCurrentRoutes(d.config.InterfaceName)
	if err != nil {
		return err
	}

	toAdd, toRemove := routes.CalculateDiff(current, desired)
	err = applyRouteDiff(d.config.InterfaceName, toAdd, toRemove)
	// Removals run before adds, so restore even if an add failed.
	for _, r := range toRemove {
		if len(routes.ExcludeOwn([]string{r.Network}, d.localNode.RoutableNetworks)) == 0 {
			restoreConnectedRoute(r.Network)
		}
	}
	return err
}

// restoreConnectedRoute re-adds the kernel connected route for an own network
// whose `<network> via <peer> dev <mesh iface>` route was just removed: older
// daemons installed that route with `ip route replace`, overwriting it.
// Best effort; without a local address in network there is nothing to restore.
func restoreConnectedRoute(network string) {
	_, ipnet, err := net.ParseCIDR(network)
	if err != nil {
		return
	}
	family := "-4"
	if ipnet.IP.To4() == nil {
		family = "-6"
	}
	out, err := cmdExecutor.Command("ip", family, "-o", "addr", "show", "to", ipnet.String()).Output()
	if err != nil {
		return
	}
	iface, src, ok := routes.ParseLocalAddr(string(out), ipnet)
	if !ok {
		return
	}
	args := strings.Fields(routes.ConnectedRouteCmd(ipnet, iface, src))
	if out, err := cmdExecutor.Command(args[0], args[1:]...).CombinedOutput(); err != nil && !strings.Contains(string(out), "File exists") {
		log.Printf("Failed to restore connected route %s dev %s: %s: %v", ipnet, iface, strings.TrimSpace(string(out)), err)
		return
	}
	log.Printf("Restored connected route: %s dev %s", ipnet, iface)
}

func (d *Daemon) currentRelayRoutesSnapshot() map[string]string {
	d.relayMu.RLock()
	defer d.relayMu.RUnlock()
	out := make(map[string]string, len(d.relayRoutes))
	for k, v := range d.relayRoutes {
		out[k] = v
	}
	return out
}

func (d *Daemon) directStableCyclesSnapshot() map[string]int {
	d.relayMu.RLock()
	defer d.relayMu.RUnlock()
	out := make(map[string]int, len(d.directStableCycles))
	for k, v := range d.directStableCycles {
		out[k] = v
	}
	return out
}

func getCurrentRoutes(iface string) ([]routes.Entry, error) {
	cmd := cmdExecutor.Command("ip", "route", "show", "dev", iface)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read routes: %w", err)
	}

	result := make([]routes.Entry, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 1 {
			continue
		}

		network := routes.NormalizeNetwork(parts[0])
		gateway := ""
		for i, part := range parts {
			if part == "via" && i+1 < len(parts) {
				gateway = parts[i+1]
				break
			}
		}

		if gateway == "" {
			continue
		}

		result = append(result, routes.Entry{Network: network, Gateway: gateway})
	}

	return result, nil
}

func applyRouteDiff(iface string, toAdd, toRemove []routes.Entry) error {
	for _, route := range toRemove {
		cmd := cmdExecutor.Command("ip", "route", "del", route.Network, "via", route.Gateway, "dev", iface)
		_ = cmd.Run()
	}

	for _, route := range toAdd {
		cmd := cmdExecutor.Command("ip", "route", "replace", route.Network, "via", route.Gateway, "dev", iface)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to add route %s via %s: %s: %w", route.Network, route.Gateway, string(output), err)
		}
	}

	cmd := cmdExecutor.Command("sysctl", "-w", "net.ipv4.ip_forward=1")
	_ = cmd.Run()

	ensureWGForwardingRule(iface)

	return nil
}

func ensureWGForwardingRule(iface string) {
	// Best-effort: allow forwarding between WG peers on this interface.
	// This is required for relay mode when traffic must pass through a public node.
	check := cmdExecutor.Command("iptables", "-C", "FORWARD", "-i", iface, "-o", iface, "-j", "ACCEPT")
	if err := check.Run(); err == nil {
		return
	}

	add := cmdExecutor.Command("iptables", "-A", "FORWARD", "-i", iface, "-o", iface, "-j", "ACCEPT")
	if out, err := add.CombinedOutput(); err != nil {
		log.Printf("Failed to install relay FORWARD rule for %s: %s: %v", iface, strings.TrimSpace(string(out)), err)
	}
}
