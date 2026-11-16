package daemon

import (
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/pkg/routes"
)

// darwin_routes.go is the macOS side of the route target (ADR-0003 decision
// 7): the observe (`netstat`), the parser and the pure plan. The writes
// (`route -n add|change|delete`) are converge.go's, like every kernel write.
// The file is not named *_darwin.go: that suffix is a build constraint and
// the code would not compile or test on Linux.
//
// A macOS route has no gateway: it leaves by the mesh interface and
// WireGuard picks the peer by AllowedIPs, so the Entry of a Darwin route
// carries only its canonical Network.

// darwinTable is one family of `netstat -rn`: the routes of the mesh
// interface, and the networks that OTHER interfaces route (canonical
// network -> interface name), which this daemon must never re-point.
type darwinTable struct {
	Mesh    []routes.Entry
	Foreign map[string]string
}

// readRoutesDarwin reads the routes of iface for one family with
// `netstat -rn -f inet` or `-f inet6`.
func readRoutesDarwin(iface string, ipv6 bool) (darwinTable, error) {
	family := "inet"
	if ipv6 {
		family = "inet6"
	}
	out, err := cmdExecutor.Command("netstat", "-rn", "-f", family).Output()
	if err != nil {
		return darwinTable{}, fmt.Errorf("failed to read routes: %w", commandFailure(out, err))
	}
	return parseNetstatDarwin(out, iface, ipv6)
}

// observeRoutesDarwin reads the routes of the mesh interface, both families.
// A family whose read or parse fails is an entry in obs.Errs for that family
// only; it is never read as "no routes".
func (d *Daemon) observeRoutesDarwin(obs *kernelObserved) {
	iface := d.config.InterfaceName
	if table, err := readRoutesDarwin(iface, false); err != nil {
		obs.fail(targetRoutes4, err)
	} else {
		obs.Routes4, obs.Foreign4 = table.Mesh, table.Foreign
	}
	if table, err := readRoutesDarwin(iface, true); err != nil {
		obs.fail(targetRoutes6, err)
	} else {
		obs.Routes6, obs.Foreign6 = table.Mesh, table.Foreign
	}
}

// parseNetstatDarwin reads the output of `netstat -rn -f inet` (ipv6 false) or
// `-f inet6`: the columns are Destination, Gateway, Flags, Netif, Expire. It
// returns the routes that leave by iface and that this daemon can install
// with `-interface`: a link-level gateway (`link#N`) and none of the flags
// m (multicast), b (broadcast), L (link-layer entry) or W (cloned), not a
// multicast or link-local destination, not a scoped one (`fe80::%utun4`). A
// row of another interface is not a route of the mesh: it is kept in
// Foreign (network -> interface, any gateway), so the plan never takes a
// network that another interface routes. A row of iface whose
// destination does not parse is an error. Destinations are expanded to the
// canonical form of the family queried: `default` is 0.0.0.0/0 or ::/0,
// `192.168.5/24` is 192.168.5.0/24, a bare `127` is 127.0.0.0/8, a bare host
// is /32 or /128.
func parseNetstatDarwin(out []byte, iface string, ipv6 bool) (darwinTable, error) {
	table := darwinTable{Mesh: make([]routes.Entry, 0), Foreign: make(map[string]string)}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		dest, gateway, flags, netif := fields[0], fields[1], fields[2], fields[3]
		mesh := netif == iface
		if strings.ContainsAny(flags, "mbLW") || strings.Contains(dest, "%") || (mesh && !strings.HasPrefix(gateway, "link#")) {
			continue
		}
		canonical, err := canonicalNetstatDest(dest, ipv6)
		if err != nil {
			if mesh {
				return darwinTable{}, fmt.Errorf("parse netstat row %q: %w", strings.TrimSpace(line), err)
			}
			continue
		}
		if p := netip.MustParsePrefix(canonical); p.Addr().IsMulticast() || p.Addr().IsLinkLocalUnicast() {
			continue
		}
		if mesh {
			table.Mesh = append(table.Mesh, routes.Entry{Network: canonical})
		} else if _, seen := table.Foreign[canonical]; !seen {
			table.Foreign[canonical] = netif
		}
	}
	return table, nil
}

// canonicalNetstatDest expands a destination as netstat prints it. An
// IPv4 destination without a prefix length takes the mask of its octets (one
// octet is /8, three are /24, four are a host); one with a prefix length is
// padded with zero octets.
func canonicalNetstatDest(dest string, ipv6 bool) (string, error) {
	if dest == "default" {
		return routes.Canonical("default", ipv6), nil
	}
	name, bits, hasBits := strings.Cut(dest, "/")
	if ipv6 {
		addr, err := netip.ParseAddr(name)
		if err != nil || !addr.Is6() {
			return "", fmt.Errorf("%q is not an IPv6 destination", dest)
		}
		length := addr.BitLen()
		if hasBits {
			if length, err = strconv.Atoi(bits); err != nil || length < 0 || length > 128 {
				return "", fmt.Errorf("%q has no valid prefix length", dest)
			}
		}
		return netip.PrefixFrom(addr, length).Masked().String(), nil
	}
	octets := strings.Split(name, ".")
	if len(octets) > 4 {
		return "", fmt.Errorf("%q is not an IPv4 destination", dest)
	}
	length := 8 * len(octets)
	for len(octets) < 4 {
		octets = append(octets, "0")
	}
	addr, err := netip.ParseAddr(strings.Join(octets, "."))
	if err != nil {
		return "", fmt.Errorf("%q is not an IPv4 destination", dest)
	}
	if hasBits {
		if length, err = strconv.Atoi(bits); err != nil || length < 0 || length > 32 {
			return "", fmt.Errorf("%q has no valid prefix length", dest)
		}
	}
	return netip.PrefixFrom(addr, length).Masked().String(), nil
}

// meshPrefixes are the connected routes of the mesh interface that
// setInterfaceAddress installs: the mesh subnet and the mesh IPv6 /64.
func (d *Daemon) meshPrefixes() []netip.Prefix {
	var keep []netip.Prefix
	if p, ok := prefixOf(d.meshNet()); ok {
		keep = append(keep, p)
	}
	if p := d.meshPrefixV6(); p.IsValid() {
		keep = append(keep, p)
	}
	return keep
}

// darwinForeign is a wanted network that another interface already routes.
type darwinForeign struct{ Network, Iface string }

// planRoutesDarwin is pure: per family, a wanted network the interface does
// not hold is added, unless another interface already routes it (foreign: it
// is returned instead, never added, so the host's default route or another
// tunnel's route is not re-pointed), and a held network that is not wanted
// is removed, except the prefixes in keep (the mesh connected routes). The
// gateway of a wanted entry is ignored. A family whose observe failed plans
// nothing, so it removes nothing. Adds, removals and foreign are sorted, IPv4
// first.
func planRoutesDarwin(want []routes.Entry, k kernelObserved, keep []netip.Prefix) (routePlan, []darwinForeign) {
	var plan routePlan
	var skipped []darwinForeign
	for _, f := range []struct {
		target  string
		ipv6    bool
		current []routes.Entry
		foreign map[string]string
	}{{targetRoutes4, false, k.Routes4, k.Foreign4}, {targetRoutes6, true, k.Routes6, k.Foreign6}} {
		if k.Errs[f.target] != nil {
			continue
		}
		wanted := make(map[string]struct{})
		for _, e := range want {
			if strings.Contains(e.Network, ":") == f.ipv6 {
				wanted[routes.Canonical(e.Network, f.ipv6)] = struct{}{}
			}
		}
		held := make(map[string]struct{})
		var remove []string
		for _, e := range f.current {
			network := routes.Canonical(e.Network, f.ipv6)
			held[network] = struct{}{}
			if _, ok := wanted[network]; !ok && !keptPrefix(network, keep) {
				remove = append(remove, network)
			}
		}
		var add []string
		var foreign []darwinForeign
		for network := range wanted {
			if _, ok := held[network]; ok {
				continue
			}
			if iface, ok := f.foreign[network]; ok {
				foreign = append(foreign, darwinForeign{network, iface})
				continue
			}
			add = append(add, network)
		}
		sort.Strings(add)
		sort.Strings(remove)
		sort.Slice(foreign, func(i, j int) bool { return foreign[i].Network < foreign[j].Network })
		for _, n := range add {
			plan.Add = append(plan.Add, routes.Entry{Network: n})
		}
		for _, n := range remove {
			plan.Remove = append(plan.Remove, routes.Entry{Network: n})
		}
		skipped = append(skipped, foreign...)
	}
	return plan, skipped
}

// keptPrefix reports whether the canonical network is one of keep.
func keptPrefix(network string, keep []netip.Prefix) bool {
	p, err := netip.ParsePrefix(network)
	if err != nil {
		return false
	}
	for _, k := range keep {
		if k == p {
			return true
		}
	}
	return false
}
