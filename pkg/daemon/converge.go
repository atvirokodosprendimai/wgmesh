package daemon

import (
	"fmt"
	"log"
	"sort"
	"strings"

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
	plan := planKernel(ds.Peers, obs, d.pushedLocked(), d.config.DisableIPv6)
	d.logResults(d.applyKernel(plan))
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
		id := r.Target + " " + r.Op + " " + r.Key
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

func (d *Daemon) logf(format string, args ...any) {
	if d.logFn != nil {
		d.logFn(format, args...)
		return
	}
	log.Printf(format, args...)
}
