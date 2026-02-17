package daemon

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"
)

type routeEntry struct {
	Network string
	Gateway string
}

func (d *Daemon) syncPeerRoutes(peers []*PeerInfo) error {
	if runtime.GOOS != "linux" {
		return nil
	}

	desired := make([]routeEntry, 0)
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
		for _, network := range peer.RoutableNetworks {
			network = strings.TrimSpace(network)
			if network == "" {
				continue
			}
			desired = append(desired, routeEntry{Network: network, Gateway: gateway})
		}
	}

	current, err := getCurrentRoutes(d.config.InterfaceName)
	if err != nil {
		return err
	}

	toAdd, toRemove := calculateRouteDiff(current, desired)
	return applyRouteDiff(d.config.InterfaceName, toAdd, toRemove)
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

func getCurrentRoutes(iface string) ([]routeEntry, error) {
	cmd := exec.Command("ip", "route", "show", "dev", iface)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read routes: %w", err)
	}

	routes := make([]routeEntry, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 1 {
			continue
		}

		network := normalizeNetwork(parts[0])
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

		routes = append(routes, routeEntry{Network: network, Gateway: gateway})
	}

	return routes, nil
}

func calculateRouteDiff(current, desired []routeEntry) (toAdd, toRemove []routeEntry) {
	currentMap := make(map[string]routeEntry)
	desiredMap := make(map[string]routeEntry)
	currentByNetwork := make(map[string]routeEntry)
	desiredByNetwork := make(map[string]routeEntry)

	for _, r := range current {
		key := makeRouteKey(r.Network, r.Gateway)
		currentMap[key] = r
		currentByNetwork[r.Network] = r
	}

	for _, r := range desired {
		key := makeRouteKey(r.Network, r.Gateway)
		desiredMap[key] = r
		desiredByNetwork[r.Network] = r
	}

	for key, route := range desiredMap {
		if _, exists := currentMap[key]; !exists {
			if currentRoute, networkExists := currentByNetwork[route.Network]; networkExists {
				if currentRoute.Gateway != route.Gateway {
					toRemove = append(toRemove, currentRoute)
				}
			}
			toAdd = append(toAdd, route)
		}
	}

	for key, route := range currentMap {
		if _, exactMatch := desiredMap[key]; !exactMatch {
			if _, stillNeeded := desiredByNetwork[route.Network]; !stillNeeded {
				toRemove = append(toRemove, route)
			}
		}
	}

	return toAdd, toRemove
}

func makeRouteKey(network, gateway string) string {
	return fmt.Sprintf("%s|%s", network, gateway)
}

func normalizeNetwork(network string) string {
	if !strings.Contains(network, "/") {
		if strings.Count(network, ".") == 3 {
			return network + "/32"
		}
		if strings.Contains(network, ":") {
			return network + "/128"
		}
	}
	return network
}

func applyRouteDiff(iface string, toAdd, toRemove []routeEntry) error {
	for _, route := range toRemove {
		cmd := exec.Command("ip", "route", "del", route.Network, "via", route.Gateway, "dev", iface)
		_ = cmd.Run()
	}

	for _, route := range toAdd {
		cmd := exec.Command("ip", "route", "replace", route.Network, "via", route.Gateway, "dev", iface)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to add route %s via %s: %s: %w", route.Network, route.Gateway, string(output), err)
		}
	}

	cmd := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1")
	_ = cmd.Run()

	ensureWGForwardingRule(iface)

	return nil
}

func ensureWGForwardingRule(iface string) {
	// Best-effort: allow forwarding between WG peers on this interface.
	// This is required for relay mode when traffic must pass through a public node.
	check := exec.Command("iptables", "-C", "FORWARD", "-i", iface, "-o", iface, "-j", "ACCEPT")
	if err := check.Run(); err == nil {
		return
	}

	add := exec.Command("iptables", "-A", "FORWARD", "-i", iface, "-o", iface, "-j", "ACCEPT")
	if out, err := add.CombinedOutput(); err != nil {
		log.Printf("Failed to install relay FORWARD rule for %s: %s: %v", iface, strings.TrimSpace(string(out)), err)
	}
}
