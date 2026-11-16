package daemon

import (
	"errors"
	"fmt"
	"net/netip"
	"os/exec"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/pkg/routes"
)

// targetPeers names the WireGuard peers in an observe error and a metric label.
const targetPeers = "peer"

// observedPeer is what the kernel holds for one peer.
type observedPeer struct {
	// Allowed is the canonical AllowedIPs set.
	Allowed map[string]struct{}
	// Handshake is the latest handshake, unix seconds, 0 = never.
	Handshake int64
}

// targetForward names ip_forward and the FORWARD rule in an observe error, an
// item result and a metric label.
const targetForward = "forward"

// observedForward is what the kernel holds for forwarding.
type observedForward struct {
	IPForward       bool // /proc/sys/net/ipv4/ip_forward reads 1
	IptablesPresent bool // exec.LookPath("iptables") found it
	RuleInstalled   bool // iptables -C FORWARD -i <if> -o <if> -j ACCEPT succeeded
}

// targetRoutes names the kernel routes of the mesh interface in an item
// result and a metric label; targetRoutes4 and targetRoutes6 name one family
// in an observe error.
const (
	targetRoutes  = "route"
	targetRoutes4 = "route-4"
	targetRoutes6 = "route-6"
)

// targetAddress names the addresses of the mesh interface in an observe
// error, an item result and a metric label.
const targetAddress = "address"

// kernelObserved is what one tick read from the kernel. A target that could
// not be read has an entry in Errs and must not be read as empty. Routes4 and
// Routes6 are the canonical gateway routes of the mesh interface, per family.
type kernelObserved struct {
	Peers   map[string]observedPeer
	Forward observedForward
	Routes4 []routes.Entry
	Routes6 []routes.Entry
	// Addrs are the addresses on the mesh interface, both families, each with
	// the prefix length the kernel holds it with.
	Addrs []netip.Prefix
	Errs  map[string]error
}

// fail records that one target could not be read.
func (k *kernelObserved) fail(target string, err error) {
	if k.Errs == nil {
		k.Errs = make(map[string]error)
	}
	k.Errs[target] = err
}

// readRoutes reads the gateway routes of iface for one family: IPv4 with
// `ip route show dev` (the default family of ip), IPv6 with `ip -6`. Networks
// are canonical for the family queried, so `default` is 0.0.0.0/0 or ::/0 and
// a bare host is /32 or /128; routes without a gateway are ignored.
func readRoutes(iface string, ipv6 bool) ([]routes.Entry, error) {
	var out []byte
	var err error
	if ipv6 {
		out, err = cmdExecutor.Command("ip", "-6", "route", "show", "dev", iface).Output()
	} else {
		out, err = cmdExecutor.Command("ip", "route", "show", "dev", iface).Output()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read routes: %w", err)
	}
	result := make([]routes.Entry, 0)
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
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
		result = append(result, routes.Entry{Network: routes.Canonical(parts[0], ipv6), Gateway: gateway})
	}
	return result, nil
}

// observeRoutes reads the routes of the mesh interface, both families, on
// Linux only. A family whose read fails is an entry in obs.Errs for that
// family only; it is never read as "no routes".
func (d *Daemon) observeRoutes(obs *kernelObserved) {
	if d.osName() != "linux" {
		return
	}
	iface := d.config.InterfaceName
	if entries, err := readRoutes(iface, false); err != nil {
		obs.fail(targetRoutes4, err)
	} else {
		obs.Routes4 = entries
	}
	if entries, err := readRoutes(iface, true); err != nil {
		obs.fail(targetRoutes6, err)
	} else {
		obs.Routes6 = entries
	}
}

// parseAddresses reads the output of `ip -o addr show dev <if>`: one
// netip.Prefix per inet and inet6 line. The family word is the third field
// (`5: wg0    inet 10.99.0.1/16 scope global wg0\ ...`); a point-to-point
// address printed without a prefix length is a host address.
func parseAddresses(out []byte) ([]netip.Prefix, error) {
	var addrs []netip.Prefix
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || (fields[2] != "inet" && fields[2] != "inet6") {
			continue
		}
		if len(fields) < 4 {
			return nil, fmt.Errorf("parse address line %q: no address", line)
		}
		p, err := netip.ParsePrefix(fields[3])
		if err != nil {
			a, aerr := netip.ParseAddr(fields[3])
			if aerr != nil {
				return nil, fmt.Errorf("parse address line %q: %w", line, err)
			}
			p = netip.PrefixFrom(a, a.BitLen())
		}
		addrs = append(addrs, p)
	}
	return addrs, nil
}

// observeAddresses reads the addresses of the mesh interface, both families,
// on Linux only and only when the address target is wanted. A failed read or
// an unparsable line is an entry in obs.Errs for this target; it is never read
// as "no addresses".
func (d *Daemon) observeAddresses(want addressDesired, obs *kernelObserved) {
	if !want.Wanted {
		return
	}
	out, err := cmdExecutor.Command("ip", "-o", "addr", "show", "dev", d.config.InterfaceName).Output()
	if err != nil {
		obs.fail(targetAddress, fmt.Errorf("read addresses: %w", commandFailure(out, err)))
		return
	}
	addrs, err := parseAddresses(out)
	if err != nil {
		obs.fail(targetAddress, err)
		return
	}
	obs.Addrs = addrs
}

// hostDefaultElsewhere reads the host's default routes of one family and
// returns the device of the first one that is not on iface, "" if there is
// none.
func hostDefaultElsewhere(iface string, ipv6 bool) (string, error) {
	var out []byte
	var err error
	if ipv6 {
		out, err = cmdExecutor.Command("ip", "-6", "route", "show", "default").Output()
	} else {
		out, err = cmdExecutor.Command("ip", "-4", "route", "show", "default").Output()
	}
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 0 || parts[0] != "default" {
			continue
		}
		for i, part := range parts {
			if part == "dev" && i+1 < len(parts) && parts[i+1] != iface {
				return parts[i+1], nil
			}
		}
	}
	return "", nil
}

// ipForwardPath is the sysctl net.ipv4.ip_forward as a file.
const ipForwardPath = "/proc/sys/net/ipv4/ip_forward"

// observeForward reads the forwarding state into obs, only when forwarding is
// wanted: ip_forward, whether iptables exists and, if it does, whether the
// FORWARD rule is installed. A failed read is an entry in obs.Errs for this
// target only; it is never read as "0" or "absent".
func (d *Daemon) observeForward(want forwardDesired, obs *kernelObserved) {
	if !want.Wanted {
		return
	}
	fail := func(err error) { obs.fail(targetForward, err) }
	raw, err := d.readFileSeam(ipForwardPath)
	if err != nil {
		fail(fmt.Errorf("read %s: %w", ipForwardPath, err))
		return
	}
	switch v := strings.TrimSpace(string(raw)); v {
	case "0":
	case "1":
		obs.Forward.IPForward = true
	default:
		fail(fmt.Errorf("read %s: unexpected value %q", ipForwardPath, v))
		return
	}
	if _, err := cmdExecutor.LookPath("iptables"); err != nil {
		if !errors.Is(err, exec.ErrNotFound) {
			fail(fmt.Errorf("look up iptables: %w", err))
		}
		return
	}
	obs.Forward.IptablesPresent = true
	iface := d.config.InterfaceName
	out, err := cmdExecutor.Command("iptables", append([]string{"-C"}, forwardRuleArgs(iface)...)...).CombinedOutput()
	var status interface{ ExitCode() int }
	switch {
	case err == nil:
		obs.Forward.RuleInstalled = true
	case errors.As(err, &status) && status.ExitCode() == 1:
		// -C exits 1 for a rule that is not there; that is the one failure that means "absent".
	default:
		fail(fmt.Errorf("iptables -C FORWARD: %w", commandFailure(out, err)))
	}
}

// commandFailure is the error of a failed command with its output in front
// when it printed any.
func commandFailure(out []byte, err error) error {
	if text := strings.TrimSpace(string(out)); text != "" {
		return fmt.Errorf("%s: %w", text, err)
	}
	return err
}

// observeKernel reads the kernel once (peer target only for now).
func (d *Daemon) observeKernel() kernelObserved {
	dump, err := d.dumpSeam(d.config.InterfaceName)
	if err != nil {
		return kernelObserved{Errs: map[string]error{targetPeers: err}}
	}
	obs := kernelObserved{Peers: make(map[string]observedPeer, len(dump))}
	for key, p := range dump {
		allowed := make(map[string]struct{}, len(p.AllowedIPs))
		for _, cidr := range p.AllowedIPs {
			allowed[canonicalCIDR(cidr)] = struct{}{}
		}
		obs.Peers[key] = observedPeer{Allowed: allowed, Handshake: p.LatestHandshake}
	}
	return obs
}

// canonicalCIDR is the form in which the kernel's and the desired AllowedIPs are compared.
func canonicalCIDR(cidr string) string {
	return routes.Canonical(cidr, strings.Contains(cidr, ":"))
}

// handshakes is public key -> latest handshake, from the same dump as the peers.
func (k kernelObserved) handshakes() map[string]int64 {
	out := make(map[string]int64, len(k.Peers))
	for key, p := range k.Peers {
		out[key] = p.Handshake
	}
	return out
}

// owners is canonical network -> key of the peer that holds it in the kernel.
// A network held by several peers is observed nowhere.
func (k kernelObserved) owners() map[string]string {
	out := make(map[string]string)
	held := make(map[string]int)
	for key, p := range k.Peers {
		for network := range p.Allowed {
			held[network]++
			out[network] = key
		}
	}
	for network, n := range held {
		if n > 1 {
			delete(out, network)
		}
	}
	return out
}
