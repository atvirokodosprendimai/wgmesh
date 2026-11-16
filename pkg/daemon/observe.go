package daemon

import (
	"fmt"
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

// kernelObserved is what one tick read from the kernel. A target that could
// not be read has an entry in Errs and must not be read as empty.
type kernelObserved struct {
	Peers   map[string]observedPeer
	Forward observedForward
	Errs    map[string]error
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
	fail := func(err error) {
		if obs.Errs == nil {
			obs.Errs = make(map[string]error)
		}
		obs.Errs[targetForward] = err
	}
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
		return
	}
	obs.Forward.IptablesPresent = true
	iface := d.config.InterfaceName
	obs.Forward.RuleInstalled = cmdExecutor.Command("iptables", "-C", "FORWARD", "-i", iface, "-o", iface, "-j", "ACCEPT").Run() == nil
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
