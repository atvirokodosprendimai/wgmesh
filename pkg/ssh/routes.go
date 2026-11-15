package ssh

import (
	"fmt"
	"net"
	"regexp"
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

var ifaceNameRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,15}$`)

// RestoreConnectedRoute re-adds the kernel connected route for network after a
// stale `<network> dev <mesh iface>` route was removed. Older deploys installed
// that route with `ip route replace`, which overwrote the connected route.
// Without a local address in network the original route cannot be
// reconstructed, so the operator is warned instead.
func RestoreConnectedRoute(client *Client, network string) {
	_, ipnet, err := net.ParseCIDR(network)
	if err != nil {
		fmt.Printf("    Warning: not restoring route for invalid network %q: %v\n", network, err)
		return
	}
	family := "-4"
	if ipnet.IP.To4() == nil {
		family = "-6"
	}

	output, err := client.Run(fmt.Sprintf("ip %s -o addr show to %s", family, ipnet))
	iface, src, ok := "", "", false
	if err == nil {
		iface, src, ok = parseLocalAddr(output, ipnet)
	}
	if !ok {
		fmt.Printf("    Warning: no local address in %s; if it was reachable other than via the mesh interface, restore that route manually\n", ipnet)
		return
	}

	output, err = client.Run(connectedRouteCmd(ipnet, iface, src))
	if err != nil && !strings.Contains(output, "File exists") {
		fmt.Printf("    Warning: failed to restore connected route %s dev %s: %v %s\n", ipnet, iface, err, strings.TrimSpace(output))
		return
	}
	fmt.Printf("    Restored connected route: %s dev %s\n", ipnet, iface)
}

// parseLocalAddr returns the interface and address of the first line of
// `ip -o addr show to <network>` output whose address prefix is exactly network.
func parseLocalAddr(output string, network *net.IPNet) (iface, src string, ok bool) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || (fields[2] != "inet" && fields[2] != "inet6") {
			continue
		}
		name, _, _ := strings.Cut(fields[1], "@")
		ip, addrNet, err := net.ParseCIDR(fields[3])
		if err != nil || !ifaceNameRe.MatchString(name) || addrNet.String() != network.String() {
			continue
		}
		return name, ip.String(), true
	}
	return "", "", false
}

// connectedRouteCmd builds the command that recreates the route the kernel adds
// for an address. IPv6 connected routes carry metric 256 and no src.
func connectedRouteCmd(network *net.IPNet, iface, src string) string {
	if network.IP.To4() == nil {
		return fmt.Sprintf("ip -6 route add %s dev %s proto kernel metric 256", network, iface)
	}
	return fmt.Sprintf("ip route add %s dev %s proto kernel scope link src %s", network, iface, src)
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
