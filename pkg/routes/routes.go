// Package routes provides shared route management types and helpers used by
// both the ssh and daemon packages.  Platform-specific operations (reading the
// kernel routing table, applying route commands) remain in their respective
// packages; only the pure logic is shared here.
package routes

import (
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"sort"
	"strings"
)

// Entry represents a single kernel routing table entry.
type Entry struct {
	Network string // CIDR, e.g. "10.0.0.0/8" or "192.168.5.5/32"
	Gateway string // Next-hop IP, empty for directly-connected routes
}

// NormalizeNetwork normalizes a network string returned by the kernel.
// Linux's `ip route` displays host routes without a prefix length
// (e.g. "192.168.5.5" instead of "192.168.5.5/32").  This function adds
// the appropriate suffix so that kernel output can be compared to user-
// supplied CIDR strings.
func NormalizeNetwork(network string) string {
	if !strings.Contains(network, "/") {
		if strings.Count(network, ".") == 3 {
			// IPv4 host route
			return network + "/32"
		}
		if strings.Contains(network, ":") {
			// IPv6 host route
			return network + "/128"
		}
	}
	return network
}

// MakeKey returns a string that uniquely identifies a route by both its
// network destination and its gateway.
func MakeKey(network, gateway string) string {
	return fmt.Sprintf("%s|%s", network, gateway)
}

// CalculateDiff compares current and desired route sets and returns the
// minimal set of routes to add and remove.
//
// Rules:
//   - If a desired route already exists exactly (same network + gateway) it
//     is skipped.
//   - If a desired network exists with a *different* gateway, every old
//     gateway route of that network is removed and the new one is added.
//   - If a current network is no longer in the desired set at all, and it has
//     a non-empty gateway (i.e. it is a managed mesh route), it is removed.
//   - Directly-connected routes (empty gateway) are never removed.
//   - Networks are compared in canonical form per family, so the kernel's
//     "default" and a desired "0.0.0.0/0" are one network.
//   - Two desired entries for one network keep the one whose gateway is
//     numerically lowest, so the result does not depend on the input order
//     and does not alternate between cycles.
//
// The result is sorted by network, then gateway.
func CalculateDiff(current, desired []Entry) (toAdd, toRemove []Entry) {
	currentMap := make(map[string]Entry)       // "canonical network|gateway" -> entry
	desiredMap := make(map[string]Entry)       // "canonical network|gateway" -> entry
	desiredByNetwork := make(map[string]Entry) // canonical network -> entry

	for _, r := range current {
		currentMap[MakeKey(entryNetwork(r), r.Gateway)] = r
	}

	for _, r := range desired {
		network := entryNetwork(r)
		if prev, ok := desiredByNetwork[network]; !ok || addrLess(r.Gateway, prev.Gateway) {
			desiredByNetwork[network] = r
		}
	}
	for network, r := range desiredByNetwork {
		desiredMap[MakeKey(network, r.Gateway)] = r
	}

	// Routes to add: desired routes that do not exist yet.
	for key, route := range desiredMap {
		if _, exists := currentMap[key]; !exists {
			toAdd = append(toAdd, route)
		}
	}

	// Routes to remove: managed (gateway-routed) current routes that are not
	// desired exactly, whether their network is gone from the desired state or
	// is still desired through another gateway.
	for key, route := range currentMap {
		if _, exactMatch := desiredMap[key]; !exactMatch && route.Gateway != "" {
			toRemove = append(toRemove, route)
		}
	}

	sortEntries(toAdd)
	sortEntries(toRemove)
	return toAdd, toRemove
}

// entryNetwork returns the canonical network of e. The family is read from the
// network, or from the gateway when the network is the family-less "default".
func entryNetwork(e Entry) string {
	return Canonical(e.Network, familyOf(e.Network) || familyOf(e.Gateway))
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		ni, nj := entryNetwork(entries[i]), entryNetwork(entries[j])
		if ni != nj {
			return ni < nj
		}
		return entries[i].Gateway < entries[j].Gateway
	})
}

// addrLess orders two address strings as numbers (10.99.0.9 before
// 10.99.0.10); a string that does not parse sorts after every address, and
// two such strings fall back to string order.
func addrLess(a, b string) bool {
	pa, errA := netip.ParseAddr(strings.TrimSpace(a))
	pb, errB := netip.ParseAddr(strings.TrimSpace(b))
	switch {
	case errA == nil && errB == nil:
		if c := pa.Compare(pb); c != 0 {
			return c < 0
		}
	case errA == nil:
		return true
	case errB == nil:
		return false
	}
	return a < b
}

// Canonical returns the canonical CIDR form of network for one address
// family: a bare host becomes /32 or /128, host bits are masked, and the
// kernel's "default" becomes 0.0.0.0/0 for IPv4 and ::/0 for IPv6 (the kernel
// prints "default" for both, so the caller says which family it reads). An
// input that does not parse is returned trimmed and normalized.
func Canonical(network string, ipv6 bool) string {
	network = NormalizeNetwork(strings.TrimSpace(network))
	if network == "default" {
		if ipv6 {
			return "::/0"
		}
		return "0.0.0.0/0"
	}
	if _, ipnet, err := net.ParseCIDR(network); err == nil {
		return ipnet.String()
	}
	return network
}

// familyOf reports whether network is written as an IPv6 network. A bare
// "default" carries no family and counts as IPv4.
func familyOf(network string) bool {
	return strings.Contains(network, ":")
}

// Advertiser is one peer that advertises networks.
type Advertiser struct {
	Key      string
	MeshIP   string
	MeshIPv6 string
	Fresh    bool
	Networks []string
}

// Resolve gives every network that peers advertise exactly one owner and
// returns canonical network -> owner key. The choice is the same on every
// call and does not depend on the order of adv.
//
// Candidates for a network are the peers that advertise it after ExcludeOwn
// (a network this node advertises is never owned by a peer); blank entries are
// ignored. A peer with a fresh handshake beats one without. Among fresh
// candidates the owner already observed in current (canonical network -> owner
// key, read back from the kernel) keeps the network; otherwise the lowest
// mesh IP wins, compared as an address, then the lowest key. For an IPv6
// network the candidate's MeshIPv6 is compared when it has one, else its
// MeshIP; an address that does not parse sorts last. Resolve is pure: adv,
// own and current are not modified.
func Resolve(adv []Advertiser, own []string, current map[string]string) map[string]string {
	type candidate struct {
		key, ip string
		fresh   bool
	}
	byNetwork := make(map[string][]candidate)
	for _, a := range adv {
		for _, n := range ExcludeOwn(a.Networks, own) {
			if strings.TrimSpace(n) == "" {
				continue
			}
			v6 := familyOf(n)
			network := Canonical(n, v6)
			ip := a.MeshIP
			if v6 && strings.TrimSpace(a.MeshIPv6) != "" {
				ip = a.MeshIPv6
			}
			byNetwork[network] = append(byNetwork[network], candidate{a.Key, ip, a.Fresh})
		}
	}

	observed := make(map[string]string, len(current))
	for _, n := range sortedKeys(current) {
		if strings.TrimSpace(n) == "default" {
			// The kernel prints both default routes as the word default.
			observed["0.0.0.0/0"] = current[n]
			observed["::/0"] = current[n]
			continue
		}
		observed[Canonical(n, familyOf(n))] = current[n]
	}

	owners := make(map[string]string, len(byNetwork))
	for network, cands := range byNetwork {
		best := cands[0]
		for _, c := range cands[1:] {
			switch {
			case c.fresh != best.fresh:
				if c.fresh {
					best = c
				}
			case addrLess(c.ip, best.ip):
				best = c
			case !addrLess(best.ip, c.ip) && c.key < best.key:
				best = c
			}
		}
		if best.fresh {
			for _, c := range cands {
				if o, ok := observed[network]; c.fresh && ok && c.key == o {
					best = c
					break
				}
			}
		}
		owners[network] = best.key
	}
	return owners
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ExcludeOwn returns the entries of networks that are not in own. A node must
// never route a network it advertises itself through the mesh, so a peer's
// advertisement of the same network (HA) is dropped on this node. This holds
// for the default route too: two HA exit nodes both advertising 0.0.0.0/0 each
// drop the other's default route.
func ExcludeOwn(networks, own []string) []string {
	ownSet := make(map[string]struct{}, len(own))
	for _, n := range own {
		ownSet[Canonical(n, familyOf(n))] = struct{}{}
	}
	out := make([]string, 0, len(networks))
	for _, n := range networks {
		if _, ok := ownSet[Canonical(n, familyOf(n))]; !ok {
			out = append(out, n)
		}
	}
	return out
}

var ifaceNameRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,15}$`)

// ParseLocalAddr returns the interface and address of the first line of
// `ip -o addr show to <network>` output whose address prefix is exactly network.
func ParseLocalAddr(output string, network *net.IPNet) (iface, src string, ok bool) {
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

// ConnectedRouteCmd builds the command that recreates the route the kernel adds
// for an address. IPv6 connected routes carry metric 256 and no src.
func ConnectedRouteCmd(network *net.IPNet, iface, src string) string {
	if network.IP.To4() == nil {
		return fmt.Sprintf("ip -6 route add %s dev %s proto kernel metric 256", network, iface)
	}
	return fmt.Sprintf("ip route add %s dev %s proto kernel scope link src %s", network, iface, src)
}
