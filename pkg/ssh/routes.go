package ssh

import (
	"fmt"
	"strings"
)

type RouteEntry struct {
	Network string
	Gateway string
}

func GetCurrentRoutes(client *Client, iface string) ([]RouteEntry, error) {
	output, err := client.Run(fmt.Sprintf("ip route show dev %s", iface))
	if err != nil {
		return nil, err
	}

	routes := make([]RouteEntry, 0)
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 1 {
			continue
		}

		network := parts[0]
		gateway := ""

		for i, part := range parts {
			if part == "via" && i+1 < len(parts) {
				gateway = parts[i+1]
				break
			}
		}

		// Normalize network format: add /32 for host routes if not present
		network = normalizeNetwork(network)

		routes = append(routes, RouteEntry{
			Network: network,
			Gateway: gateway,
		})
	}

	return routes, nil
}

func CalculateRouteDiff(current, desired []RouteEntry) (toAdd, toRemove []RouteEntry) {
	// Build maps for exact matching (network+gateway) and network-only lookups
	currentMap := make(map[string]RouteEntry)       // "network|gateway" -> route
	desiredMap := make(map[string]RouteEntry)       // "network|gateway" -> route
	currentByNetwork := make(map[string]RouteEntry) // "network" -> route
	desiredByNetwork := make(map[string]RouteEntry) // "network" -> route

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

	// Process desired routes
	for key, route := range desiredMap {
		if _, exists := currentMap[key]; !exists {
			// Route with this exact network+gateway doesn't exist

			// Check if the same network exists with a DIFFERENT gateway (VPN IP change)
			if currentRoute, networkExists := currentByNetwork[route.Network]; networkExists {
				if currentRoute.Gateway != route.Gateway && currentRoute.Gateway != "" {
					// Gateway changed - must remove old route first
					toRemove = append(toRemove, currentRoute)
				}
			}

			// Add the new/updated route
			toAdd = append(toAdd, route)
		}
		// else: exact route already exists, no action needed (this is the optimization!)
	}

	// Find routes that should be removed (network no longer in desired state)
	for key, route := range currentMap {
		if _, exactMatch := desiredMap[key]; !exactMatch {
			// Check if this network is still needed but with different gateway
			if _, stillNeeded := desiredByNetwork[route.Network]; !stillNeeded {
				// Network completely removed from desired state
				// Only remove gateway routes (routes we manage via mesh)
				if route.Gateway != "" {
					toRemove = append(toRemove, route)
				}
			}
			// else: network still needed with different gateway, already handled above
		}
	}

	return toAdd, toRemove
}

func makeRouteKey(network, gateway string) string {
	// Key includes both network and gateway to ensure exact matching
	// Example: "192.168.10.0/24|10.99.0.1"
	return fmt.Sprintf("%s|%s", network, gateway)
}

func normalizeNetwork(network string) string {
	// If network doesn't contain '/', it's a host route shown by 'ip route'
	// Linux displays /32 routes as just the IP (e.g., "192.168.5.5" instead of "192.168.5.5/32")
	// We need to add /32 to match our desired state format
	if !strings.Contains(network, "/") {
		// Check if it's an IPv4 address
		if strings.Count(network, ".") == 3 {
			return network + "/32"
		}
		// For IPv6, add /128
		if strings.Contains(network, ":") {
			return network + "/128"
		}
	}
	return network
}

func ApplyRouteDiff(client *Client, iface string, toAdd, toRemove []RouteEntry) error {
	totalChanges := len(toAdd) + len(toRemove)
	if totalChanges == 0 {
		fmt.Printf("  No route changes needed (all routes already correct)\n")
		return nil
	}

	fmt.Printf("  Route changes: %d to remove, %d to add\n", len(toRemove), len(toAdd))

	if len(toRemove) > 0 {
		for _, route := range toRemove {
			var cmd string
			if route.Gateway != "" {
				cmd = fmt.Sprintf("ip route del %s via %s dev %s 2>/dev/null || true",
					route.Network, route.Gateway, iface)
			} else {
				cmd = fmt.Sprintf("ip route del %s dev %s 2>/dev/null || true",
					route.Network, iface)
			}

			if err := client.RunQuiet(cmd); err != nil {
				fmt.Printf("    Warning: failed to remove route %s: %v\n", route.Network, err)
			} else {
				if route.Gateway != "" {
					fmt.Printf("    Removed route: %s via %s\n", route.Network, route.Gateway)
				} else {
					fmt.Printf("    Removed route: %s\n", route.Network)
				}
			}
		}
	}

	if len(toAdd) > 0 {
		for _, route := range toAdd {
			var cmd string
			if route.Gateway != "" {
				cmd = fmt.Sprintf("ip route add %s via %s dev %s || ip route replace %s via %s dev %s",
					route.Network, route.Gateway, iface, route.Network, route.Gateway, iface)
			} else {
				cmd = fmt.Sprintf("ip route add %s dev %s || ip route replace %s dev %s",
					route.Network, iface, route.Network, iface)
			}

			if err := client.RunQuiet(cmd); err != nil {
				return fmt.Errorf("failed to add route for %s: %w", route.Network, err)
			}

			if route.Gateway != "" {
				fmt.Printf("    Added route: %s via %s\n", route.Network, route.Gateway)
			} else {
				fmt.Printf("    Added route: %s\n", route.Network)
			}
		}
	}

	cmd := "sysctl -w net.ipv4.ip_forward=1 > /dev/null"
	if err := client.RunQuiet(cmd); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %w", err)
	}

	return nil
}
