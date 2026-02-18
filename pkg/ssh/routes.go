package ssh

import (
	"fmt"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/pkg/routes"
)

// RouteEntry is an alias for routes.Entry kept for backward compatibility with
// callers in the ssh package.
type RouteEntry = routes.Entry

// GetCurrentRoutes returns the kernel routing table entries for the given
// interface by running `ip route show dev <iface>` over the SSH client.
func GetCurrentRoutes(client *Client, iface string) ([]RouteEntry, error) {
	output, err := client.Run(fmt.Sprintf("ip route show dev %s", iface))
	if err != nil {
		return nil, err
	}

	result := make([]RouteEntry, 0)
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
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

		result = append(result, RouteEntry{
			Network: network,
			Gateway: gateway,
		})
	}

	return result, nil
}

// CalculateRouteDiff delegates to routes.CalculateDiff.
func CalculateRouteDiff(current, desired []RouteEntry) (toAdd, toRemove []RouteEntry) {
	return routes.CalculateDiff(current, desired)
}

// ApplyRouteDiff applies the computed route diff over the SSH client.
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
