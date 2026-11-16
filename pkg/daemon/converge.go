package daemon

import (
	"fmt"
	"log"
	"net/netip"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/atvirokodosprendimai/wgmesh/pkg/routes"
	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

// converge.go is the one place that writes WireGuard peers to the kernel.
// One tick is: observe the kernel (one `wg show dump`), desire (desired.go),
// plan (pure), apply (one item at a time). The health paths (reconnect,
// eviction, stale cleanup) write through the helpers below, under the same
// lock, so a tick's apply and a health write never interleave.

// peerWrite is one `wg set peer` of a plan.
type peerWrite struct {
	Key string
	// Endpoint is pushed with the set; "" leaves the kernel's endpoint alone.
	Endpoint string
	Allowed  []string
}

// kernelPlan is what one tick will write: removals, then shrinking sets, then
// growing sets.
type kernelPlan struct {
	Remove []string
	Set    []peerWrite
}

// itemResult is the outcome of one item of a plan.
type itemResult struct {
	Target string
	Op     string // "set" or "remove"
	Key    string
	Err    error
}

// planKernel is pure: the peers to remove and to set so that the kernel holds
// want. A peer is set when it is missing in the kernel, when its AllowedIPs
// set differs, or when its desired endpoint is not the one last pushed
// (pushed); a drift-only set carries no endpoint, so a roamed endpoint is not
// overwritten. A peer without an endpoint, or with an IPv6 endpoint while
// disableIPv6, is not configured. Shrinking peers come before growing ones; an
// observed peer that is not wanted is removed.
func planKernel(want map[string]*desiredPeerConfig, kernel kernelObserved, pushed map[string]string, disableIPv6 bool) kernelPlan {
	var plan kernelPlan
	for key := range kernel.Peers {
		if _, ok := want[key]; !ok {
			plan.Remove = append(plan.Remove, key)
		}
	}
	sort.Strings(plan.Remove)

	keys := make([]string, 0, len(want))
	for key := range want {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var shrinking, growing []peerWrite
	for _, key := range keys {
		cfg := want[key]
		if !cfg.configurable(disableIPv6) {
			continue
		}
		allowed := mapKeysSorted(cfg.allowed)
		wanted := make(map[string]struct{}, len(allowed))
		for _, cidr := range allowed {
			wanted[canonicalCIDR(cidr)] = struct{}{}
		}
		obs, present := kernel.Peers[key]
		endpointChanged := pushed[key] != cfg.peer.Endpoint
		if present && !endpointChanged && sameSet(obs.Allowed, wanted) {
			continue
		}
		w := peerWrite{Key: key, Allowed: allowed}
		if !present || endpointChanged {
			w.Endpoint = cfg.peer.Endpoint
		}
		if present && !subset(obs.Allowed, wanted) {
			shrinking = append(shrinking, w)
		} else {
			growing = append(growing, w)
		}
	}
	plan.Set = append(shrinking, growing...)
	return plan
}

// configurable reports whether the peer can be written to the kernel: it has an
// endpoint (an IPv6 one only when IPv6 is on) and something to allow.
func (cfg *desiredPeerConfig) configurable(disableIPv6 bool) bool {
	ep := cfg.peer.Endpoint
	return ep != "" && !(disableIPv6 && isIPv6Endpoint(ep)) && len(cfg.allowed) > 0
}

// sameSet reports whether a and b hold the same members.
func sameSet(a, b map[string]struct{}) bool {
	return len(a) == len(b) && subset(a, b)
}

// subset reports whether every member of a is in b.
func subset(a, b map[string]struct{}) bool {
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// EnforceForwardScope is the release switch of the rule forward-scope: false
// (release N) forces forwarding on every Linux node and only counts a node
// that neither relays nor advertises routes; true (N+1) leaves such a node
// alone. Every access is Load or Store.
var EnforceForwardScope atomic.Bool

// EnforceRouteGuard is the release switch of the rule attached-network: false
// (release N) installs a route to a network this host is directly attached to
// and only counts it; true (N+1) drops it from the routes and the AllowedIPs.
// Every access is Load or Store.
var EnforceRouteGuard atomic.Bool

// EnforceAddressRemoval is the release switch of the rule stale-address: false
// (release N) only warns about a held address inside the mesh network that is
// not this node's (once per address while it holds) and removes nothing; true
// (N+1) removes it. Adding a missing address is not behind it. Every access is
// Load or Store.
var EnforceAddressRemoval atomic.Bool

// forwardRuleArgs is the FORWARD rule of a mesh interface as iptables
// arguments after the -C or -A that checks or appends it.
func forwardRuleArgs(iface string) []string {
	return []string{"FORWARD", "-i", iface, "-o", iface, "-j", "ACCEPT"}
}

// forwardPlan is what one tick will write for forwarding.
type forwardPlan struct {
	Sysctl     bool // write net.ipv4.ip_forward=1
	Rule       bool // append the FORWARD rule
	NoIptables bool // forwarding is wanted but there is no iptables binary
}

// planForward is pure: the sysctl is written only when it reads 0, the rule
// appended only when -C found it absent, and a wanted node without iptables
// is an outcome. A target whose observe failed, or that is not wanted, plans
// nothing.
func planForward(want forwardDesired, k kernelObserved) forwardPlan {
	var plan forwardPlan
	if !want.Wanted || k.Errs[targetForward] != nil {
		return plan
	}
	plan.Sysctl = !k.Forward.IPForward
	plan.NoIptables = !k.Forward.IptablesPresent
	plan.Rule = k.Forward.IptablesPresent && !k.Forward.RuleInstalled
	return plan
}

// applyForward writes the forwarding plan, one item at a time; a failure of
// one is a result, never an abort. A missing iptables is logged once, not
// every tick, and is not a result; the flag that remembers it is touched only
// on a tick that observed the target. The caller holds convergeMu.
func (d *Daemon) applyForward(plan forwardPlan, observed bool) []itemResult {
	if observed && plan.NoIptables != d.forwardNoIptables {
		d.forwardNoIptables = plan.NoIptables
		if plan.NoIptables {
			d.logf("iptables not found: the FORWARD rule for %s is not managed on this node", d.config.InterfaceName)
		}
	}
	iface := d.config.InterfaceName
	var results []itemResult
	if plan.Sysctl {
		out, err := cmdExecutor.Command("sysctl", "-w", "net.ipv4.ip_forward=1").CombinedOutput()
		results = append(results, d.forwardResult("set", "ip_forward", out, err))
	}
	if plan.Rule {
		out, err := cmdExecutor.Command("iptables", append([]string{"-A"}, forwardRuleArgs(iface)...)...).CombinedOutput()
		results = append(results, d.forwardResult("append", "FORWARD", out, err))
	}
	return results
}

// forwardResult is the result of one forwarding write; a success is counted
// as a kernel change.
func (d *Daemon) forwardResult(op, key string, out []byte, err error) itemResult {
	if err != nil {
		err = commandFailure(out, err)
	} else {
		RecordKernelChange(targetForward)
	}
	return itemResult{Target: targetForward, Op: op, Key: key, Err: err}
}

// noteForwardObserve logs a failed forwarding read once, and the recovery
// once. The caller holds convergeMu.
func (d *Daemon) noteForwardObserve(err error) {
	if err == nil {
		if d.forwardObserveErr != "" {
			d.logf("Reading the forwarding state works again")
			d.forwardObserveErr = ""
		}
		return
	}
	if msg := err.Error(); msg != d.forwardObserveErr {
		d.logf("Cannot read the forwarding state, skipping it this tick: %v", err)
		d.forwardObserveErr = msg
	}
}

// noteForwardScope counts the rule forward-scope: a Linux node that is forced
// to forward although it neither relays nor advertises routes (possible only
// while EnforceForwardScope is off). One log line and one counter increment
// per transition into that state, not per tick. The caller holds convergeMu.
func (d *Daemon) noteForwardScope(f forwardDesired) {
	warn := f.Wanted && !f.Needed
	if warn && !d.forwardScopeWarned {
		d.logf("forward-scope: this node neither relays (introducer) nor advertises routes (advertise-routes), yet ip_forward and the FORWARD rule on %s are still forced; a later release stops forcing them (EnforceForwardScope); set --introducer or advertise-routes if this node must forward", d.config.InterfaceName)
		RecordKernelWarning("forward-scope")
	}
	d.forwardScopeWarned = warn
}

// routePlan is what one tick will write for routes: the removals, then the adds.
type routePlan struct {
	Add, Remove []routes.Entry
}

// planRoutes is pure: per family, the difference between the observed
// canonical routes and the wanted ones (routes.CalculateDiff). A family whose
// observe failed plans nothing, so it removes nothing. A network that is
// added is not removed: `ip route replace` swaps its gateway atomically, and
// a delete first would leave the network without a route when the replace
// fails.
func planRoutes(want []routes.Entry, k kernelObserved) routePlan {
	var plan routePlan
	for _, f := range []struct {
		target  string
		ipv6    bool
		current []routes.Entry
	}{{targetRoutes4, false, k.Routes4}, {targetRoutes6, true, k.Routes6}} {
		if k.Errs[f.target] != nil {
			continue
		}
		var desired []routes.Entry
		for _, e := range want {
			if strings.Contains(e.Network, ":") == f.ipv6 {
				desired = append(desired, e)
			}
		}
		add, remove := routes.CalculateDiff(f.current, desired)
		plan.Add = append(plan.Add, add...)
		for _, r := range remove {
			if !hasNetwork(add, routes.Canonical(r.Network, f.ipv6)) {
				plan.Remove = append(plan.Remove, r)
			}
		}
	}
	return plan
}

// noteRouteObserve logs a failed route read once per family, and the recovery
// once. The caller holds convergeMu.
func (d *Daemon) noteRouteObserve(obs kernelObserved) {
	if d.routeObserveErr == nil {
		d.routeObserveErr = make(map[string]string)
	}
	for _, f := range []struct{ target, name string }{{targetRoutes4, "IPv4"}, {targetRoutes6, "IPv6"}} {
		err, prev := obs.Errs[f.target], d.routeObserveErr[f.target]
		switch {
		case err == nil && prev != "":
			d.logf("Reading the %s routes of %s works again", f.name, d.config.InterfaceName)
			delete(d.routeObserveErr, f.target)
		case err != nil && err.Error() != prev:
			d.logf("Cannot read the %s routes of %s, skipping them this tick: %v", f.name, d.config.InterfaceName, err)
			d.routeObserveErr[f.target] = err.Error()
		}
	}
}

// hasNetwork reports whether entries hold the canonical network.
func hasNetwork(entries []routes.Entry, canonical string) bool {
	for _, e := range entries {
		if routes.Canonical(e.Network, strings.Contains(e.Network, ":")) == canonical {
			return true
		}
	}
	return false
}

// hostRoute is a host default route: the device it leaves by and its
// gateway ("" for a route without one).
type hostRoute struct{ Dev, Via string }

// defaultFamilies are the two default routes the host-default rule watches.
var defaultFamilies = []struct {
	name, network string
	ipv6          bool
}{{"IPv4", "0.0.0.0/0", false}, {"IPv6", "::/0", true}}

// restoreHostDefault puts the host default route back with `ip [-6] route
// replace default [via gw] dev dev`; a success is counted as a route change.
func restoreHostDefault(r hostRoute, ipv6 bool) itemResult {
	var args []string
	if ipv6 {
		args = append(args, "-6")
	}
	args = append(args, "route", "replace", "default")
	if r.Via != "" {
		args = append(args, "via", r.Via)
	}
	args = append(args, "dev", r.Dev)
	out, err := cmdExecutor.Command("ip", args...).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("failed to restore the host default route dev %s: %w", r.Dev, commandFailure(out, err))
	} else {
		RecordKernelChange(targetRoutes)
	}
	return itemResult{Target: targetRoutes, Op: "replace", Key: "default (host) dev " + r.Dev, Err: err}
}

// noteHostDefault is the rule host-default: before a default route is added
// (and while the mesh default route of that family is not installed), one
// read of the host's default route of that family; a default on a device
// other than the mesh interface is logged once and counted, remembered (device
// and gateway) so restoreHostDefaults can put it back, and the mesh route is
// still installed (a peer's 0.0.0.0/0 is a feature; `ip route replace` swaps a
// default route of the same destination and metric, so it replaces the
// host's). A read that shows no host default forgets the record. A family
// that no longer wants a default route is forgotten for the warning, so it
// warns again when it returns; its record stays until the mesh default is
// removed. The caller holds convergeMu.
func (d *Daemon) noteHostDefault(want []routes.Entry, plan routePlan, obs kernelObserved) {
	if d.hostDefaultWarned == nil {
		d.hostDefaultWarned = make(map[string]struct{})
	}
	if d.hostDefaultReadErr == nil {
		d.hostDefaultReadErr = make(map[string]string)
	}
	if d.hostDefaults == nil {
		d.hostDefaults = make(map[string]hostRoute)
	}
	for _, f := range defaultFamilies {
		if !hasNetwork(want, f.network) {
			delete(d.hostDefaultWarned, f.name)
			delete(d.hostDefaultReadErr, f.name)
			continue
		}
		installed := obs.Routes4
		if f.ipv6 {
			installed = obs.Routes6
		}
		// A read while the mesh default is installed would show no host
		// default (it was replaced): it must not forget the record.
		if _, warned := d.hostDefaultWarned[f.name]; warned || !hasNetwork(plan.Add, f.network) || hasNetwork(installed, f.network) {
			continue
		}
		host, err := hostDefaultElsewhere(d.config.InterfaceName, f.ipv6)
		if err != nil {
			if msg := err.Error(); msg != d.hostDefaultReadErr[f.name] {
				d.logf("Cannot read the host's %s default route: %v", f.name, err)
				d.hostDefaultReadErr[f.name] = msg
			}
			continue
		}
		if d.hostDefaultReadErr[f.name] != "" {
			d.logf("Reading the host's %s default route works again", f.name)
			delete(d.hostDefaultReadErr, f.name)
		}
		if host.Dev == "" {
			delete(d.hostDefaults, f.name)
			continue
		}
		d.logf("host-default: this host already has an %s default route on %s; the mesh's %s default route on %s replaces it (same route key) and the daemon puts it back when the mesh default route is removed", f.name, host.Dev, f.name, d.config.InterfaceName)
		RecordKernelWarning("host-default")
		d.hostDefaultWarned[f.name] = struct{}{}
		d.hostDefaults[f.name] = host
	}
}

// restoreHostDefaults puts back, once, the host default route of each family
// whose mesh default route was just removed, from the record noteHostDefault
// took; the record is forgotten whatever the outcome, a failure is a result.
// The caller holds convergeMu.
func (d *Daemon) restoreHostDefaults(removed []routes.Entry) []itemResult {
	var results []itemResult
	for _, f := range defaultFamilies {
		host, ok := d.hostDefaults[f.name]
		if !ok || !hasNetwork(removed, f.network) {
			continue
		}
		delete(d.hostDefaults, f.name)
		res := restoreHostDefault(host, f.ipv6)
		if res.Err == nil {
			d.logf("host-default: restored the host's %s default route via %q dev %s after the mesh default route was removed", f.name, host.Via, host.Dev)
		}
		results = append(results, res)
	}
	return results
}

// convergeRoutes is the route target of one tick: it plans the difference
// between the observed routes and ds.Routes and applies it, one route at a
// time, with the adapter of the OS (Linux `ip`, macOS `route`); another OS has
// no route target. The caller holds convergeMu.
func (d *Daemon) convergeRoutes(ds *desiredState, obs kernelObserved) []itemResult {
	switch d.routeAdapter() {
	case routesLinux:
		return d.convergeRoutesLinux(ds, obs)
	case routesDarwin:
		return d.convergeRoutesDarwin(ds, obs)
	}
	return nil
}

// convergeRoutesDarwin is the route target on macOS: the routes leave by the
// mesh interface with no gateway (WireGuard picks the peer by AllowedIPs), so
// it has its own plan. There is no host default to warn about
// (`noteHostDefault` reads Linux's `ip route`) and no connected route to
// restore: an attached network is never installed on macOS. The caller holds
// convergeMu.
func (d *Daemon) convergeRoutesDarwin(ds *desiredState, obs kernelObserved) []itemResult {
	d.noteRouteObserve(obs)
	plan, foreign := planRoutesDarwin(ds.Routes, obs, d.meshPrefixes(), d.darwinInstalled)
	d.noteDarwinWarnings(obs, foreign)
	results := applyRouteDiffDarwin(d.config.InterfaceName, plan)
	d.noteDarwinInstalled(results)
	return results
}

// noteDarwinInstalled keeps the set of networks this daemon run installed on
// the mesh interface (the only ones it may delete): a successful add or
// change puts a network in, a successful delete takes it out. The interface
// is recreated at every daemon start, so the in-memory set is exact. The
// caller holds convergeMu.
func (d *Daemon) noteDarwinInstalled(results []itemResult) {
	if d.darwinInstalled == nil {
		d.darwinInstalled = make(map[string]struct{})
	}
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		switch r.Op {
		case "add", "change":
			d.darwinInstalled[r.Key] = struct{}{}
		case "delete":
			delete(d.darwinInstalled, r.Key)
		}
	}
}

// noteDarwinWarnings logs and counts, once while the case holds, a wanted
// network that another interface already routes (rule foreign-route), and
// each distinct mesh-interface netstat row the parser does not recognise: it is
// not routed into the mesh, because `route change` would re-point the host's
// default or another tunnel's route. The set is replaced by the cases of this
// tick; a family whose observe failed keeps its entries, since nothing is
// known about it. The caller holds convergeMu.
func (d *Daemon) noteDarwinWarnings(obs kernelObserved, foreign []darwinForeign) {
	now := make(map[string]struct{}, len(foreign))
	for key := range d.darwinWarned {
		fam := strings.SplitN(key, "|", 3)[1]
		if (fam == "4" && obs.Errs[targetRoutes4] != nil) || (fam == "6" && obs.Errs[targetRoutes6] != nil) {
			now[key] = struct{}{}
		}
	}
	for _, f := range foreign {
		fam := "4"
		if strings.Contains(f.Network, ":") {
			fam = "6"
		}
		key := "foreign|" + fam + "|" + f.Network
		if _, was := d.darwinWarned[key]; !was {
			// Unverified on a Mac, S1 decides: `route add` is believed to
			// answer File exists for a network another interface holds.
			d.logf("foreign-route: the network %s of a peer is already routed by %s on this Mac; the peer's network is not routed on this Mac and the route of %s is left alone", f.Network, f.Iface, f.Iface)
			RecordKernelWarning("foreign-route")
		}
		now[key] = struct{}{}
	}
	for _, rows := range []struct {
		fam  string
		rows []string
	}{{"4", obs.Unrecognised4}, {"6", obs.Unrecognised6}} {
		for _, row := range rows.rows {
			key := "row|" + rows.fam + "|" + row
			if _, was := d.darwinWarned[key]; !was {
				d.logf("unrecognised route row of %s: %q is skipped (the parser knows no such shape; S1 decides whether it must)", d.config.InterfaceName, row)
			}
			now[key] = struct{}{}
		}
	}
	d.darwinWarned = now
}

// convergeRoutesLinux is the route target on Linux.
func (d *Daemon) convergeRoutesLinux(ds *desiredState, obs kernelObserved) []itemResult {
	d.noteRouteObserve(obs)
	plan := planRoutes(ds.Routes, obs)
	d.noteHostDefault(ds.Routes, plan, obs)
	removed, results := applyRouteDiff(d.config.InterfaceName, plan.Add, plan.Remove)
	// Removals run before adds, so restore even if an add failed. Only a
	// removed route can be replaced by the connected route: an own network's,
	// or an attached one's (`ip route replace` overwrote the host's connected
	// route for that network).
	localSubnets := d.getLocalSubnets()
	for _, r := range removed {
		if d.isOwnNetwork(r.Network) || d.attachedSubnet(r.Network, localSubnets) != nil {
			restoreConnectedRoute(r.Network)
		}
	}
	// The mesh default replaced the host's (same route key): when the mesh
	// default is removed, the host's goes back, once.
	return append(results, d.restoreHostDefaults(removed)...)
}

// applyRouteDiff removes and adds the routes, one at a time with one result
// each: a failure of one never skips the others. It returns the routes whose
// `ip route del` succeeded. Forwarding is the forwarding target's, not this
// function's.
func applyRouteDiff(iface string, toAdd, toRemove []routes.Entry) ([]routes.Entry, []itemResult) {
	var removed []routes.Entry
	results := make([]itemResult, 0, len(toAdd)+len(toRemove))
	for _, route := range toRemove {
		err := cmdExecutor.Command("ip", "route", "del", route.Network, "via", route.Gateway, "dev", iface).Run()
		if err == nil {
			removed = append(removed, route)
			RecordKernelChange(targetRoutes)
		} else {
			err = fmt.Errorf("failed to remove route %s via %s: %w", route.Network, route.Gateway, err)
		}
		results = append(results, itemResult{Target: targetRoutes, Op: "del", Key: route.Network + " via " + route.Gateway, Err: err})
	}
	for _, route := range toAdd {
		out, err := cmdExecutor.Command("ip", "route", "replace", route.Network, "via", route.Gateway, "dev", iface).CombinedOutput()
		if err != nil {
			err = fmt.Errorf("failed to add route %s via %s: %w", route.Network, route.Gateway, commandFailure(out, err))
		} else {
			RecordKernelChange(targetRoutes)
		}
		results = append(results, itemResult{Target: targetRoutes, Op: "replace", Key: route.Network + " via " + route.Gateway, Err: err})
	}
	return removed, results
}

// applyRouteDiffDarwin removes and adds the routes of a plan, one at a time
// with one result each (Op delete, add or change; Key the canonical network):
// a failure of one never skips the others. `route -n add` that answers "File
// exists" is repeated as `route -n change`, as setInterfaceAddress does for
// the mesh route.
func applyRouteDiffDarwin(iface string, plan routePlan) []itemResult {
	results := make([]itemResult, 0, len(plan.Add)+len(plan.Remove))
	for _, r := range plan.Remove {
		out, err := darwinRouteCommand("delete", r.Network, iface)
		results = append(results, darwinRouteResult("delete", r.Network, out, err))
	}
	for _, r := range plan.Add {
		op := "add"
		out, err := darwinRouteCommand("add", r.Network, iface)
		if err != nil && strings.Contains(string(out), "File exists") {
			op = "change"
			out, err = darwinRouteCommand("change", r.Network, iface)
		}
		results = append(results, darwinRouteResult(op, r.Network, out, err))
	}
	return results
}

// darwinRouteCommand runs `route -n <verb> [-inet6] -net <network> -interface
// <iface>`: no gateway, the interface alone, as setInterfaceAddress adds the
// mesh route. `-inet6` is for an IPv6 network.
func darwinRouteCommand(verb, network, iface string) ([]byte, error) {
	args := []string{"-n", verb}
	if strings.Contains(network, ":") {
		args = append(args, "-inet6")
	}
	args = append(args, "-net", network, "-interface", iface)
	return cmdExecutor.Command("route", args...).CombinedOutput()
}

// darwinRouteResult is the result of one `route` write; a success is counted
// as a kernel change.
func darwinRouteResult(op, network string, out []byte, err error) itemResult {
	if err != nil {
		err = fmt.Errorf("failed to %s route %s: %w", op, network, commandFailure(out, err))
	} else {
		RecordKernelChange(targetRoutes)
	}
	return itemResult{Target: targetRoutes, Op: op, Key: network, Err: err}
}

// addressPlan is what one tick will write for addresses: the adds, then the
// removals.
type addressPlan struct {
	Add, Remove []netip.Prefix
	// Skipped: the observe failed, so nothing was planned and nothing is
	// known about what holds.
	Skipped bool
}

// planAddresses is pure: a desired address the interface does not hold (same
// address and same prefix length) is added; a held address that is not
// desired and lies inside a mesh network (the masked prefix of a desired
// address, same family) is removed; every other address is left alone. An
// IPv6 address held with ANOTHER prefix length is not added while enforce is
// off: the kernel refuses it ("ipv6: address already assigned"), so the add
// would fail every tick, and the held one is only warned about; with enforce
// on the plan removes the held one and adds the wanted one. A target whose
// observe failed, or that is not wanted, plans nothing.
func planAddresses(want addressDesired, k kernelObserved, enforce bool) addressPlan {
	var plan addressPlan
	if k.Errs[targetAddress] != nil {
		plan.Skipped = true
		return plan
	}
	if !want.Wanted {
		return plan
	}
	wanted := make(map[netip.Prefix]bool, len(want.Addrs))
	for _, a := range want.Addrs {
		wanted[a] = true
	}
	held := make(map[netip.Prefix]bool, len(k.Addrs))
	for _, a := range k.Addrs {
		held[a] = true
	}
	for _, a := range want.Addrs {
		if held[a] || (!enforce && a.Addr().Is6() && heldOtherLength(k.Addrs, a)) {
			continue
		}
		plan.Add = append(plan.Add, a)
	}
	for a := range held {
		if wanted[a] {
			continue
		}
		for _, w := range want.Addrs {
			if w.Masked().Contains(a.Addr()) {
				plan.Remove = append(plan.Remove, a)
				break
			}
		}
	}
	sort.Slice(plan.Remove, func(i, j int) bool { return plan.Remove[i].String() < plan.Remove[j].String() })
	return plan
}

// heldOtherLength reports whether held has the address of a with another
// prefix length.
func heldOtherLength(held []netip.Prefix, a netip.Prefix) bool {
	for _, h := range held {
		if h.Addr() == a.Addr() && h != a {
			return true
		}
	}
	return false
}

// applyAddresses writes the address plan, one item at a time: the removals
// first, then the adds. The order is the kernel's: IPv6 holds an address once,
// whatever its prefix length, so the add of fd99::1/64 over a held fd99::1/128
// is refused ("address already assigned") and the later removal would leave the node without it; and
// deleting the primary IPv4 address deletes the secondaries of its subnet, the
// new address among them. A failure of one is a result, never an abort (a
// failed removal still tries the adds); "File exists" on an add is success.
// An operator's address inside the mesh network is removed too once the rule
// is enforced, so the removal says why; before that it is only warned about.
// The caller holds convergeMu.
func (d *Daemon) applyAddresses(plan addressPlan) []itemResult {
	iface := d.config.InterfaceName
	var results []itemResult
	if EnforceAddressRemoval.Load() {
		for _, p := range plan.Remove {
			r := itemResult{Target: targetAddress, Op: "del", Key: p.String()}
			d.logf("removing %s: inside the mesh network and not this node's address", p)
			out, err := cmdExecutor.Command("ip", "addr", "del", p.String(), "dev", iface).CombinedOutput()
			if err != nil {
				r.Err = commandFailure(out, err)
			} else {
				RecordKernelChange(targetAddress)
			}
			results = append(results, r)
		}
	} else {
		d.noteStaleAddresses(plan)
	}
	for _, p := range plan.Add {
		out, err := cmdExecutor.Command("ip", "addr", "add", p.String(), "dev", iface).CombinedOutput()
		switch {
		case err != nil && (strings.Contains(string(out), "File exists") || strings.Contains(string(out), "address already assigned")):
			err = nil
		case err != nil:
			err = commandFailure(out, err)
		default:
			RecordKernelChange(targetAddress)
		}
		results = append(results, itemResult{Target: targetAddress, Op: "add", Key: p.String(), Err: err})
	}
	return results
}

// noteStaleAddresses is the rule stale-address at release N: each held address
// inside the mesh network that is not this node's is logged once and counted
// once while it holds; one that stops holding leaves the set, so it warns
// again when it returns. An unread interface changes nothing. The caller holds
// convergeMu.
func (d *Daemon) noteStaleAddresses(plan addressPlan) {
	if plan.Skipped {
		return
	}
	holding := make(map[string]struct{}, len(plan.Remove))
	for _, p := range plan.Remove {
		key := p.String()
		holding[key] = struct{}{}
		if _, was := d.addressWarned[key]; was {
			continue
		}
		d.logf("stale address %s inside the mesh network, not this node's address: a later release removes it (EnforceAddressRemoval); remove it yourself if it is not an operator's", key)
		RecordKernelWarning("stale-address")
	}
	d.addressWarned = holding
}

// noteAddressObserve logs a failed address read once, and the recovery once.
// The caller holds convergeMu.
func (d *Daemon) noteAddressObserve(err error) {
	if err == nil {
		if d.addressObserveErr != "" {
			d.logf("Reading the addresses of %s works again", d.config.InterfaceName)
			d.addressObserveErr = ""
		}
		return
	}
	if msg := err.Error(); msg != d.addressObserveErr {
		d.logf("Cannot read the addresses of %s, skipping them this tick: %v", d.config.InterfaceName, err)
		d.addressObserveErr = msg
	}
}

// id names an item in the result log.
func (r itemResult) id() string {
	return r.Target + " " + r.Op + " " + r.Key
}

// converge runs one tick against the kernel and returns the desired state it
// applied, or nil when the observe failed and nothing was decided or written.
// The caller holds reconcileMu.
func (d *Daemon) converge(peers []*PeerInfo) *desiredState {
	d.convergeMu.Lock()
	defer d.convergeMu.Unlock()

	obs := d.observeKernel()
	if err := obs.Errs[targetPeers]; err != nil {
		d.noteObserve(err)
		return nil
	}
	d.noteObserve(nil)

	ds := d.desire(peers, obs.handshakes(), obs.owners())
	d.routeWarned = ds.routeWarned
	d.noteForwardScope(ds.Forward)
	d.observeForward(ds.Forward, &obs)
	d.observeRoutes(&obs)
	ds.Addresses = d.desireAddresses()
	d.observeAddresses(ds.Addresses, &obs)
	if ds.Forward.Wanted {
		d.noteForwardObserve(obs.Errs[targetForward])
	}
	if ds.Addresses.Wanted {
		d.noteAddressObserve(obs.Errs[targetAddress])
	}
	plan := planKernel(ds.Peers, obs, d.pushedLocked(), d.config.DisableIPv6)
	results := d.applyKernel(plan)
	// The addresses come before the routes: a gateway is reached over the
	// connected route of the interface's address.
	results = append(results, d.applyAddresses(planAddresses(ds.Addresses, obs, EnforceAddressRemoval.Load()))...)
	results = append(results, d.applyForward(planForward(ds.Forward, obs), ds.Forward.Wanted && obs.Errs[targetForward] == nil)...)
	results = append(results, d.convergeRoutes(ds, obs)...)
	d.logResults(results)
	return ds
}

// applyKernel writes the plan, one item at a time; an error on one item never
// skips the others. The caller holds convergeMu.
func (d *Daemon) applyKernel(plan kernelPlan) []itemResult {
	results := make([]itemResult, 0, len(plan.Remove)+len(plan.Set))
	for _, key := range plan.Remove {
		results = append(results, itemResult{Target: targetPeers, Op: "remove", Key: key, Err: d.removeLocked(key)})
	}
	for _, w := range plan.Set {
		results = append(results, itemResult{Target: targetPeers, Op: "set", Key: w.Key, Err: d.setLocked(w.Key, w.Endpoint, strings.Join(w.Allowed, ","))})
	}
	return results
}

// removeLocked removes one peer from the kernel; on success the pushed
// endpoint is forgotten and the change counted. The caller holds convergeMu.
func (d *Daemon) removeLocked(key string) error {
	err := d.removeSeam(d.config.InterfaceName, key)
	if err == nil {
		delete(d.pushedLocked(), key)
		RecordKernelChange(targetPeers)
	}
	return err
}

// setLocked sets one peer in the kernel; on success a non-empty endpoint is
// recorded as pushed and the change counted. The caller holds convergeMu.
func (d *Daemon) setLocked(key, endpoint, allowedCSV string) error {
	err := d.setSeam(d.config.InterfaceName, key, d.config.Keys.PSK, endpoint, allowedCSV)
	if err == nil {
		if endpoint != "" {
			d.pushedLocked()[key] = endpoint
		}
		RecordKernelChange(targetPeers)
	}
	return err
}

// removePeerFromKernel removes one peer (stale cleanup, eviction). The caller
// does not hold convergeMu.
func (d *Daemon) removePeerFromKernel(pubKey string) error {
	d.convergeMu.Lock()
	defer d.convergeMu.Unlock()
	return d.removeLocked(pubKey)
}

// reconnectPeerInKernel pushes peer's endpoint and its desired AllowedIPs
// again. Only a peer that has its own entry in the desired state is touched: a
// relayed peer's networks sit under its relay, and setting its mesh addresses
// here would pull them off the relay. The caller does not hold convergeMu.
func (d *Daemon) reconnectPeerInKernel(peer *PeerInfo, peers []*PeerInfo) error {
	d.convergeMu.Lock()
	defer d.convergeMu.Unlock()

	obs := d.observeKernel()
	if err := obs.Errs[targetPeers]; err != nil {
		return fmt.Errorf("read kernel: %w", err)
	}
	cfg, ok := d.desire(peers, obs.handshakes(), obs.owners()).Peers[peer.WGPubKey]
	if !ok {
		d.logf("[Health] Peer %s... has no WireGuard peer entry of its own (relayed or not desired); not touching it", shortKey(peer.WGPubKey))
		return nil
	}
	if !cfg.configurable(d.config.DisableIPv6) {
		d.logf("[Health] Peer %s... is not configurable (no endpoint, IPv6 endpoint with IPv6 disabled, or nothing to allow); not touching it", shortKey(peer.WGPubKey))
		return nil
	}
	return d.setLocked(peer.WGPubKey, cfg.peer.Endpoint, strings.Join(mapKeysSorted(cfg.allowed), ","))
}

// logResults prints a result only when it differs from the last one of the
// same item: a failure once however many ticks it repeats, a recovery once.
// The caller holds convergeMu.
func (d *Daemon) logResults(results []itemResult) {
	if d.lastResults == nil {
		d.lastResults = make(map[string]string)
	}
	seen := make(map[string]struct{}, len(results))
	for _, r := range results {
		id := r.id()
		seen[id] = struct{}{}
		prev, failedBefore := d.lastResults[id]
		if r.Err != nil {
			msg := r.Err.Error()
			if !failedBefore || prev != msg {
				d.logf("Failed to %s %s %s...: %v", r.Op, r.Target, shortKey(r.Key), r.Err)
			}
			d.lastResults[id] = msg
			continue
		}
		if failedBefore {
			d.logf("%s %s %s... recovered", r.Op, r.Target, shortKey(r.Key))
		} else {
			d.logf("%s %s %s... done", r.Op, r.Target, shortKey(r.Key))
		}
		delete(d.lastResults, id)
	}
	for id := range d.lastResults {
		if _, ok := seen[id]; !ok {
			delete(d.lastResults, id)
		}
	}
}

// noteObserve logs a failed observe once, and the recovery once. The caller holds convergeMu.
func (d *Daemon) noteObserve(err error) {
	if err == nil {
		if d.observeErrLogged != "" {
			d.logf("Reading the WireGuard interface works again")
			d.observeErrLogged = ""
		}
		return
	}
	if msg := err.Error(); msg != d.observeErrLogged {
		d.logf("Cannot read the WireGuard interface, skipping this tick: %v", err)
		d.observeErrLogged = msg
	}
}

// pushedLocked returns pushedEndpoint, creating it. The caller holds convergeMu.
func (d *Daemon) pushedLocked() map[string]string {
	if d.pushedEndpoint == nil {
		d.pushedEndpoint = make(map[string]string)
	}
	return d.pushedEndpoint
}

// The seams are Daemon fields set at construction; a nil field is the real
// function.

func (d *Daemon) setSeam(iface, pubKey string, psk [32]byte, endpoint, allowedIPs string) error {
	if d.setPeerFn != nil {
		return d.setPeerFn(iface, pubKey, psk, endpoint, allowedIPs)
	}
	return wireguard.SetPeer(iface, pubKey, psk, endpoint, allowedIPs)
}

func (d *Daemon) removeSeam(iface, pubKey string) error {
	if d.removePeerFn != nil {
		return d.removePeerFn(iface, pubKey)
	}
	return wireguard.RemovePeer(iface, pubKey)
}

func (d *Daemon) dumpSeam(iface string) (wireguard.DumpPeers, error) {
	if d.dumpFn != nil {
		return d.dumpFn(iface)
	}
	return wireguard.Dump(iface)
}

func (d *Daemon) readFileSeam(path string) ([]byte, error) {
	if d.readFileFn != nil {
		return d.readFileFn(path)
	}
	return os.ReadFile(path)
}

// routeAdapter names the kernel adapter of the route target.
type routeAdapter int

const (
	routesNone routeAdapter = iota
	routesLinux
	routesDarwin
)

// routeAdapter is the one decision of the OS-to-adapter map of the route
// target: Linux `ip`, macOS `netstat` and `route`, another OS has none.
// observeRoutes, convergeRoutes, attachedSubnet and desire all ask it.
func (d *Daemon) routeAdapter() routeAdapter {
	switch d.osName() {
	case "linux":
		return routesLinux
	case "darwin":
		return routesDarwin
	}
	return routesNone
}

// osName is the OS the desired state is computed for.
func (d *Daemon) osName() string {
	if d.goos != "" {
		return d.goos
	}
	return runtime.GOOS
}

func (d *Daemon) logf(format string, args ...any) {
	if d.logFn != nil {
		d.logFn(format, args...)
		return
	}
	log.Printf(format, args...)
}
