package ssh

import (
	"fmt"
	"net"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/pkg/routes"
)

// RouteEntry is an alias for routes.Entry kept for backward compatibility with
// callers in the ssh package.
type RouteEntry = routes.Entry

// GetCurrentRoutes returns the kernel routing table entries for the given
// interface by running `ip route show dev <iface>` over the SSH client.
func GetCurrentRoutes(client *Client, iface string) ([]RouteEntry, error) {
	output, err := client.Run(Cmd("ip", "route", "show", "dev", iface))
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

// RestoreConnectedRoute re-adds the kernel connected route for network after a
// stale `<network> dev <mesh iface>` route was removed. Older deploys installed
// that route with `ip route replace`, which overwrote the connected route.
// Without a local address in network the original route cannot be
// reconstructed, so the operator is warned instead.
func RestoreConnectedRoute(client *Client, network string) {
	if network == "default" {
		return // an exit node's stale default route has no connected route
	}
	_, ipnet, err := net.ParseCIDR(network)
	if err != nil {
		fmt.Printf("    Warning: not restoring route for invalid network %q: %v\n", network, err)
		return
	}
	family := "-4"
	if ipnet.IP.To4() == nil {
		family = "-6"
	}

	output, runErr := client.Run(Cmd("ip", family, "-o", "addr", "show", "to", ipnet.String()))
	iface, src, ok := "", "", false
	if runErr == nil {
		iface, src, ok = routes.ParseLocalAddr(output, ipnet)
	}
	if !ok {
		fmt.Println(localAddrWarning(ipnet, runErr))
		return
	}

	output, err = client.Run(Cmd(routes.ConnectedRouteCmd(ipnet, iface, src)))
	if err != nil && !strings.Contains(output, "File exists") {
		fmt.Printf("    Warning: failed to restore connected route %s dev %s: %v %s\n", ipnet, iface, err, strings.TrimSpace(output))
		return
	}
	fmt.Printf("    Restored connected route: %s dev %s\n", ipnet, iface)
}

// localAddrWarning is the operator warning for a connected route that cannot be
// restored: runErr is the failed address query, nil when it ran but found no
// local address in network.
func localAddrWarning(network *net.IPNet, runErr error) string {
	const hint = "if it was reachable other than via the mesh interface, restore that route manually"
	if runErr != nil {
		return fmt.Sprintf("    Warning: could not query local addresses in %s: %v; %s", network, runErr, hint)
	}
	return fmt.Sprintf("    Warning: no local address in %s; %s", network, hint)
}

// CalculateRouteDiff delegates to routes.CalculateDiff.
func CalculateRouteDiff(current, desired []RouteEntry) (toAdd, toRemove []RouteEntry) {
	return routes.CalculateDiff(current, desired)
}

// RouteArgs returns the arguments that follow `ip` in an `ip route <verb>`
// command: the network, `via <gateway>` when there is a gateway, and the
// interface. Pass it to Cmd, which quotes each one.
func RouteArgs(verb, network, gateway, iface string) []string {
	args := []string{"route", verb, network}
	if gateway != "" {
		args = append(args, "via", gateway)
	}
	return append(args, "dev", iface)
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
			if err := client.RunQuiet(Cmd("ip", RouteArgs("del", route.Network, route.Gateway, iface)...) + " 2>/dev/null || true"); err != nil {
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
			if err := client.RunQuiet(Cmd("ip", RouteArgs("add", route.Network, route.Gateway, iface)...) +
				" || " + Cmd("ip", RouteArgs("replace", route.Network, route.Gateway, iface)...)); err != nil {
				return fmt.Errorf("failed to add route for %s: %w", route.Network, err)
			}

			if route.Gateway != "" {
				fmt.Printf("    Added route: %s via %s\n", route.Network, route.Gateway)
			} else {
				fmt.Printf("    Added route: %s\n", route.Network)
			}
		}
	}

	const cmd = "sysctl -w net.ipv4.ip_forward=1 > /dev/null"
	if err := client.RunQuiet(cmd); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %w", err)
	}

	return nil
}
