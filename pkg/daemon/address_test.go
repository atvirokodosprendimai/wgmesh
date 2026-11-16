package daemon

import (
	"fmt"
	"net/netip"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// addrExec is the address part of the fake Linux node: `ip -o addr show dev`
// prints the held addresses the way iproute2 does, `ip addr add` and `del`
// change them with iproute2's replies ("File exists", "Cannot assign
// requested address"), and a flush empties the interface. It reports whether
// it handled the command.
func (r *routeHost) addrExec(line string, args []string) (bool, []byte, error) {
	if rest, ok := strings.CutPrefix(line, "ip -o addr show dev "); ok {
		r.mu.Lock()
		defer r.mu.Unlock()
		if err := r.readErr[line]; err != nil {
			return true, []byte("Cannot find device \"" + rest + "\""), err
		}
		var out strings.Builder
		if !r.addrHidden {
			for _, a := range r.ifAddrs {
				family := "inet"
				if strings.Contains(a, ":") {
					family = "inet6"
				}
				fmt.Fprintf(&out, "5: %s    %s %s scope global %s\\       valid_lft forever preferred_lft forever\n", rest, family, a, rest)
			}
		}
		return true, []byte(out.String()), nil
	}
	if len(args) >= 2 && args[0] != "route" && contains(args, "addr") && contains(args, "flush") {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.ifAddrs = nil
		return true, nil, nil
	}
	if len(args) != 5 || args[0] != "addr" || args[3] != "dev" || args[4] != "wg0" {
		return false, nil, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	switch args[1] {
	case "add":
		if err := r.addrAddErr[args[2]]; err != nil {
			return true, []byte("RTNETLINK answers: Operation not permitted"), err
		}
		want, wok := heldAddr(args[2])
		for _, a := range r.ifAddrs {
			// IPv6 holds an address once, whatever its prefix length: the add
			// of fd99::1/64 over a held fd99::1/128 is "File exists". IPv4
			// allows the same address with another prefix length.
			if h, ok := heldAddr(a); a == args[2] || (wok && ok && want.Addr().Is6() && h.Addr() == want.Addr()) {
				return true, []byte("RTNETLINK answers: File exists"), exitError{2}
			}
		}
		r.ifAddrs = append(r.ifAddrs, args[2])
		return true, nil, nil
	case "del":
		if err := r.addrDelErr[args[2]]; err != nil {
			return true, []byte("RTNETLINK answers: Operation not permitted"), err
		}
		for i, a := range r.ifAddrs {
			if a != args[2] {
				continue
			}
			// Deleting the primary address of an IPv4 subnet (the first held
			// address of it) deletes the secondaries of that subnet too:
			// promote_secondaries is off.
			p, ok := heldAddr(a)
			sameSubnet := func(b string) bool {
				q, qok := heldAddr(b)
				return ok && qok && p.Addr().Is4() && q.Addr().Is4() && q.Masked() == p.Masked()
			}
			first := i
			for j, b := range r.ifAddrs {
				if sameSubnet(b) {
					first = j
					break
				}
			}
			var kept []string
			for j, b := range r.ifAddrs {
				if j != i && !(first == i && sameSubnet(b)) {
					kept = append(kept, b)
				}
			}
			r.ifAddrs = kept
			return true, nil, nil
		}
		return true, []byte("RTNETLINK answers: Cannot assign requested address"), exitError{2}
	}
	return false, nil, nil
}

// heldAddr is the prefix of an address as the fake holds it (the first word of
// the printed form).
func heldAddr(s string) (netip.Prefix, bool) {
	p, err := netip.ParsePrefix(strings.Fields(s)[0])
	return p, err == nil
}

// held is the addresses on the fake interface, sorted.
func (r *routeHost) held() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]string(nil), r.ifAddrs...)
	sort.Strings(out)
	return out
}

// addrCmds are the commands that wrote, or tried to write, an address, in
// order: add, del and any flush.
func (r *routeHost) addrCmds() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, c := range r.log {
		if strings.HasPrefix(c, "ip addr add ") || strings.HasPrefix(c, "ip addr del ") || strings.Contains(c, "addr flush") {
			out = append(out, c)
		}
	}
	return out
}

// newAddrDaemon is a Linux daemon on wg0 (mesh IP 10.99.0.1, prefix /16)
// whose interface holds addrs.
func newAddrDaemon(t *testing.T, addrs ...string) (*Daemon, *routeHost, *logSink) {
	t.Helper()
	rh := convergedHost()
	rh.ifAddrs = addrs
	ls := &logSink{}
	return newRouteDaemon(t, rh, ls, newFakeKernel()), rh, ls
}

func addressChanges() float64 {
	return testutil.ToFloat64(kernelChanges.WithLabelValues(targetAddress))
}

// enforceAddressRemoval sets the release switch of the stale-address rule for
// one test.
func enforceAddressRemoval(t *testing.T, v bool) {
	t.Helper()
	old := EnforceAddressRemoval.Load()
	EnforceAddressRemoval.Store(v)
	t.Cleanup(func() { EnforceAddressRemoval.Store(old) })
}

func sorted(s ...string) []string {
	sort.Strings(s)
	return s
}

func TestObserveAddresses(t *testing.T) {
	d, rh, ls := newAddrDaemon(t, "10.99.0.1/16", "fd99::1/64", "fe80::1/64", "192.168.1.5/24")
	var obs kernelObserved
	d.observeAddresses(addressDesired{Wanted: true}, &obs)
	var got []string
	for _, p := range obs.Addrs {
		got = append(got, p.String())
	}
	if want := sorted("10.99.0.1/16", "fd99::1/64", "fe80::1/64", "192.168.1.5/24"); !sameLines(sorted(got...), want) {
		t.Errorf("observed %q, want %q (errs %v)", got, want, obs.Errs)
	}

	// Not wanted: nothing is read.
	n := rh.count("ip -o addr show")
	obs = kernelObserved{}
	d.observeAddresses(addressDesired{}, &obs)
	if rh.count("ip -o addr show") != n || obs.Addrs != nil {
		t.Errorf("an unwanted target was read: %q", obs.Addrs)
	}

	// The kernel's printed forms: a point-to-point address (`inet A peer B/N`) is
	// not read at all, a blank line is skipped.
	parsed, err := parseAddresses([]byte("5: wg0    inet 10.99.0.1 peer 10.99.0.2/32 scope global wg0\\       valid_lft forever\n\n6: wg0    inet6 fd99::2/64 scope global \\       valid_lft forever\n7: wg0    inet6 fd99::3 peer fd99::4/128 scope global \\       valid_lft forever\n"))
	if err != nil || len(parsed) != 1 || parsed[0] != netip.MustParsePrefix("fd99::2/64") {
		t.Errorf("parseAddresses = %v, %v, want only fd99::2/64", parsed, err)
	}
	if _, err := parseAddresses([]byte("5: wg0    inet not-an-address scope global wg0\n")); err == nil {
		t.Error("an unparsable address line was read as no address")
	}

	// A failed read skips only the address target: no write, one log line, the
	// peers of the tick are still converged.
	d, rh, ls = newAddrDaemon(t)
	rh.readErr["ip -o addr show dev wg0"] = fmt.Errorf("exit status 1")
	k := newFakeKernel()
	d.dumpFn, d.setPeerFn, d.removePeerFn = k.dump, k.set, k.remove
	peers := []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820")}
	tick2 := func() {
		if d.converge(peers) == nil {
			t.Fatal("converge skipped the whole tick for an address read failure")
		}
	}
	tick2()
	tick2()
	if w := rh.addrCmds(); len(w) != 0 {
		t.Errorf("address writes after a failed read: %q", w)
	}
	if len(kernelAllowed(k, "keyA")) == 0 {
		t.Error("the peer was not converged on a tick whose address read failed")
	}
	if c := ls.count("Cannot read the addresses"); c != 1 {
		t.Errorf("the failed read was logged %d times, want once", c)
	}
}

func TestMissingAddressReadded(t *testing.T) {
	before := addressChanges()
	d, rh, ls := newAddrDaemon(t)
	tick(t, d, 1)
	if got, want := rh.addrCmds(), []string{"ip addr add 10.99.0.1/16 dev wg0"}; !sameLines(got, want) {
		t.Fatalf("tick 1 address commands %q, want %q", got, want)
	}
	if !sameLines(rh.held(), []string{"10.99.0.1/16"}) {
		t.Errorf("interface holds %q", rh.held())
	}
	tick(t, d, 2)
	if len(rh.addrCmds()) != 1 {
		t.Errorf("a converged address was written again: %q", rh.addrCmds())
	}
	if got := addressChanges() - before; got != 1 {
		t.Errorf("wgmesh_kernel_changes_total{target=address} grew by %v, want 1", got)
	}

	// The address is lost again: one more add.
	rh.mu.Lock()
	rh.ifAddrs = nil
	rh.mu.Unlock()
	tick(t, d, 1)
	if len(rh.addrCmds()) != 2 {
		t.Errorf("the lost address was not re-added: %q", rh.addrCmds())
	}

	// "File exists" is success: the address was there although the read did not show it.
	d, rh, ls = newAddrDaemon(t, "10.99.0.1/16")
	rh.addrHidden = true
	before = addressChanges()
	tick(t, d, 1)
	if len(rh.addrCmds()) != 1 {
		t.Fatalf("hidden address: commands %q", rh.addrCmds())
	}
	if ls.count("Failed to") != 0 || addressChanges() != before {
		t.Errorf("File exists was not success: failures %d, changes %v", ls.count("Failed to"), addressChanges()-before)
	}
}

func TestStaleMeshAddressRemovedOtherAddressKept(t *testing.T) {
	enforceAddressRemoval(t, true)
	d, rh, ls := newAddrDaemon(t,
		"10.99.0.1/16",   // desired
		"10.99.0.1/24",   // desired address with another prefix length: not the desired address
		"10.99.7.7/16",   // old mesh address
		"192.168.1.5/24", // an operator's, outside the mesh
		"fd99::1/64",     // desired
		"fd99::9/64",     // old mesh IPv6 address
		"fd00::5/64",     // an operator's, outside the mesh
		"fe80::1/64",     // link-local
	)
	d.localNode.MeshIPv6 = "fd99::1"
	tick(t, d, 3)
	want := sorted("ip addr del 10.99.0.1/24 dev wg0", "ip addr del 10.99.7.7/16 dev wg0", "ip addr del fd99::9/64 dev wg0")
	if got := rh.addrCmds(); !sameLines(sorted(got...), want) {
		t.Errorf("address commands %q, want %q", got, want)
	}
	if got, want := rh.held(), sorted("10.99.0.1/16", "192.168.1.5/24", "fd99::1/64", "fd00::5/64", "fe80::1/64"); !sameLines(got, want) {
		t.Errorf("interface holds %q, want %q", got, want)
	}
	if rh.count("ip -4 addr flush") != 0 || rh.count("ip -6 addr flush") != 0 || rh.count("ip addr flush") != 0 {
		t.Error("the plan flushed a family")
	}
	if ls.count("removing 10.99.7.7/16: inside the mesh network and not this node's address") != 1 {
		t.Errorf("the removal was not logged with its reason: %d lines", ls.count("inside the mesh network"))
	}
}

// At release N a stale address inside the mesh network is only warned about:
// no `ip addr del`, one log line and one counter increment per address while
// it holds, and again when it returns after it stopped holding.
func TestStaleMeshAddressWarnsAtN(t *testing.T) {
	enforceAddressRemoval(t, false)
	warnings := func() float64 { return testutil.ToFloat64(kernelWarnings.WithLabelValues("stale-address")) }
	before := warnings()
	held := []string{"10.99.0.1/16", "10.99.7.7/16", "192.168.1.5/24", "fd99::1/64", "fd99::9/64"}
	d, rh, ls := newAddrDaemon(t, held...)
	d.localNode.MeshIPv6 = "fd99::1"
	tick(t, d, 3)
	if got := rh.addrCmds(); len(got) != 0 {
		t.Errorf("address writes at release N: %q", got)
	}
	if !sameLines(rh.held(), sorted(held...)) {
		t.Errorf("interface holds %q", rh.held())
	}
	for _, a := range []string{"10.99.7.7/16", "fd99::9/64"} {
		if n := ls.count("stale address " + a + " inside the mesh network"); n != 1 {
			t.Errorf("stale %s logged %d times over 3 ticks, want once", a, n)
		}
	}
	if ls.count("stale address 192.168.1.5/24") != 0 {
		t.Error("an address outside the mesh network was warned about")
	}
	if got := warnings() - before; got != 2 {
		t.Errorf("stale-address warnings grew by %v, want 2", got)
	}

	// It stops holding, then returns: warned again.
	rh.mu.Lock()
	rh.ifAddrs = []string{"10.99.0.1/16", "fd99::1/64", "fd99::9/64"}
	rh.mu.Unlock()
	tick(t, d, 2)
	rh.mu.Lock()
	rh.ifAddrs = append(rh.ifAddrs, "10.99.7.7/16")
	rh.mu.Unlock()
	tick(t, d, 2)
	if n := ls.count("stale address 10.99.7.7/16 inside the mesh network"); n != 2 {
		t.Errorf("a returning stale address was logged %d times in all, want 2", n)
	}
	if got := warnings() - before; got != 3 {
		t.Errorf("stale-address warnings grew by %v, want 3", got)
	}
}

// The removal line is printed on every attempt, so an operator reading the log
// of a removal that keeps failing sees what is being tried; the failure itself
// is logged once by the result log.
func TestFailingRemovalIsLoggedEveryAttempt(t *testing.T) {
	enforceAddressRemoval(t, true)
	d, rh, ls := newAddrDaemon(t, "10.99.0.1/16", "10.99.7.7/16")
	rh.addrDelErr["10.99.7.7/16"] = exitError{1}
	tick(t, d, 3)
	if n := ls.count("removing 10.99.7.7/16: inside the mesh network and not this node's address"); n != 3 {
		t.Errorf("the removal line was logged %d times over 3 failing ticks, want 3", n)
	}
	if n := ls.count("Failed to del address"); n != 1 {
		t.Errorf("the failure was logged %d times, want once", n)
	}
}

// The mesh IPv6 address is wanted with the one shared prefix length, the one
// setupWireGuard sets.
func TestMeshIPv6PrefixLengthIsShared(t *testing.T) {
	d, _, _ := newAddrDaemon(t)
	d.localNode.MeshIPv6 = "fd99::1"
	want := d.desireAddresses()
	if len(want.Addrs) != 2 || want.Addrs[1] != netip.PrefixFrom(netip.MustParseAddr("fd99::1"), meshPrefixLenV6) || meshPrefixLenV6 != 64 {
		t.Errorf("desired %v, want the IPv6 address with /%d (64)", want.Addrs, meshPrefixLenV6)
	}
}

// A point-to-point address inside the mesh network is neither held nor
// removable: it is left alone even with the removal enforced.
func TestPointToPointAddressLeftAlone(t *testing.T) {
	enforceAddressRemoval(t, true)
	d, rh, _ := newAddrDaemon(t, "10.99.0.1/16", "10.99.7.7 peer 10.99.9.9/32")
	tick(t, d, 3)
	if w := rh.addrCmds(); len(w) != 0 {
		t.Errorf("a point-to-point address was written: %q", w)
	}
	if got, want := rh.held(), sorted("10.99.0.1/16", "10.99.7.7 peer 10.99.9.9/32"); !sameLines(got, want) {
		t.Errorf("interface holds %q, want %q", got, want)
	}
}

func TestAddressSteadyStateQuiet(t *testing.T) {
	before := addressChanges()
	d, rh, ls := newAddrDaemon(t, "10.99.0.1/16", "fd99::1/64", "192.168.1.5/24")
	d.localNode.MeshIPv6 = "fd99::1"
	d.config.Introducer = true // forward-scope warns on a node that neither relays nor advertises: another target's line
	tick(t, d, 5)
	if w := rh.addrCmds(); len(w) != 0 {
		t.Errorf("five ticks over the correct addresses wrote %q", w)
	}
	if got := addressChanges() - before; got != 0 {
		t.Errorf("address changes grew by %v in a steady state", got)
	}
	if n := ls.total(); n != 0 {
		t.Errorf("a quiet target logged %d lines: %q", n, ls.lines)
	}

	// IPv6 disabled: the node's mesh IPv6 is not wanted, so it is not added (the
	// interface does not hold it: a wrong add would show).
	d, rh, _ = newAddrDaemon(t, "10.99.0.1/16")
	d.localNode.MeshIPv6 = "fd99::1"
	d.config.DisableIPv6 = true
	tick(t, d, 2)
	if w := rh.addrCmds(); len(w) != 0 {
		t.Errorf("IPv6 disabled: wrote %q", w)
	}
}

func TestAddressConvergesAfterCollisionIPChange(t *testing.T) {
	enforceAddressRemoval(t, true)
	d, rh, _ := newAddrDaemon(t, "10.99.0.1/16")
	tick(t, d, 2)
	if w := rh.addrCmds(); len(w) != 0 {
		t.Fatalf("wrote %q before the change", w)
	}
	d.localNode.MeshIP = "10.99.4.4" // what collision resolution does
	tick(t, d, 3)
	if got, want := rh.addrCmds(), []string{"ip addr del 10.99.0.1/16 dev wg0", "ip addr add 10.99.4.4/16 dev wg0"}; !sameLines(got, want) {
		t.Errorf("address commands %q, want %q (the old address removed first, then the new one added)", got, want)
	}
	if got := rh.held(); !sameLines(got, []string{"10.99.4.4/16"}) {
		t.Errorf("interface holds %q", got)
	}
}

// Removals run before adds, because the kernel needs it: IPv6 holds an address
// once whatever its prefix length ("File exists" for the add of fd99::1/64
// over a held fd99::1/128, which then leaves the node without it once the old
// one is deleted), and deleting the primary IPv4 address deletes the
// secondaries of its subnet (the new address added first is lost with it).
// The fake models both; the node must end the tick holding its address.
func TestAddressRemovalsRunBeforeAdds(t *testing.T) {
	enforceAddressRemoval(t, true)
	t.Run("IPv4 primary replaced", func(t *testing.T) {
		d, rh, _ := newAddrDaemon(t, "10.99.0.1/16")
		d.localNode.MeshIP = "10.99.4.4"
		tick(t, d, 1)
		if got, want := rh.held(), []string{"10.99.4.4/16"}; !sameLines(got, want) {
			t.Errorf("after one tick the interface holds %q, want %q (cmds %q)", got, want, rh.addrCmds())
		}
	})
	t.Run("IPv6 prefix length changed", func(t *testing.T) {
		d, rh, _ := newAddrDaemon(t, "10.99.0.1/16", "fd99::1/128")
		d.localNode.MeshIPv6 = "fd99::1"
		tick(t, d, 1)
		if got, want := rh.held(), sorted("10.99.0.1/16", "fd99::1/64"); !sameLines(got, want) {
			t.Errorf("after one tick the interface holds %q, want %q (cmds %q)", got, want, rh.addrCmds())
		}
	})
	t.Run("a failed removal does not skip the add", func(t *testing.T) {
		d, rh, _ := newAddrDaemon(t, "10.99.0.1/16")
		rh.addrDelErr["10.99.0.1/16"] = exitError{1}
		d.localNode.MeshIP = "10.99.4.4"
		tick(t, d, 1)
		if got, want := rh.addrCmds(), []string{"ip addr del 10.99.0.1/16 dev wg0", "ip addr add 10.99.4.4/16 dev wg0"}; !sameLines(got, want) {
			t.Errorf("address commands %q, want %q", got, want)
		}
	})
}

// Two reconcile goroutines run while a peer claims the node's mesh IP and the
// node loses the collision: CheckAndResolveCollisions writes MeshIP, the
// address step reads it, both only under reconcileMu; a reload changes the
// config on a third goroutine. Run under -race.
func TestAddressTargetRaceFree(t *testing.T) {
	enforceAddressRemoval(t, true)
	enforceIPBinding(t, false)
	d, rh, _ := newAddrDaemon(t, nodeMeshAddr)
	d.localNode.WGPubKey = "zzz-local" // the higher key loses
	d.localNode.MeshIPv6 = "fd99::1"
	d.config.Secret = collisionSecret
	d.stateFile = filepath.Join(t.TempDir(), "wg0.json")
	d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-remote", MeshIP: "10.99.0.1"}, "test")
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			routes := []string{"192.168.50.0/24"}
			if i%2 == 0 {
				routes = []string{"192.168.60.0/24"}
			}
			d.reloadConfig(DaemonOpts{AdvertiseRoutes: routes})
		}
	}()
	for g := 0; g < 2; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				d.reconcile()
			}
		}()
	}
	wg.Wait()
	moved := d.localNode.MeshIP
	if moved == "10.99.0.1" {
		t.Fatal("the node did not move: the collision did not write MeshIP")
	}
	if got, want := rh.held(), sorted(moved+"/16", "fd99::1/64"); !sameLines(got, want) {
		t.Errorf("interface holds %q, want %q", got, want)
	}
}

// A mesh IP that is empty or not an IPv4 address wants no address; a mesh IPv6
// that is not an IPv6 address is left out, the IPv4 one is still wanted.
func TestDesireAddressesInvalidIdentity(t *testing.T) {
	cases := []struct {
		name, ip, ip6 string
		noNode        bool
		want          []string // nil: not wanted
	}{
		{"valid", "10.99.0.1", "fd99::1", false, []string{"10.99.0.1/16", "fd99::1/64"}},
		{"empty MeshIP", "", "fd99::1", false, nil},
		{"garbage MeshIP", "not-an-ip", "", false, nil},
		{"IPv6 in MeshIP", "fd99::1", "", false, nil},
		{"no local node", "", "", true, nil},
		{"bad MeshIPv6 string", "10.99.0.1", "not-an-ip", false, []string{"10.99.0.1/16"}},
		{"IPv4 in MeshIPv6", "10.99.0.1", "10.99.0.9", false, []string{"10.99.0.1/16"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, _, _ := newAddrDaemon(t)
			d.localNode.MeshIP, d.localNode.MeshIPv6 = c.ip, c.ip6
			if c.noNode {
				d.localNode = nil
			}
			got := d.desireAddresses()
			var addrs []string
			for _, p := range got.Addrs {
				addrs = append(addrs, p.String())
			}
			if got.Wanted != (c.want != nil) || !sameLines(addrs, c.want) {
				t.Errorf("desired Wanted=%v %q, want %q", got.Wanted, addrs, c.want)
			}
		})
	}
}

// Off Linux the address target is not wanted: no read, no write.
func TestAddressNotWantedOffLinux(t *testing.T) {
	enforceAddressRemoval(t, true)
	d, rh, _ := newAddrDaemon(t, "10.99.7.7/16")
	d.goos = "darwin"
	if d.desireAddresses().Wanted {
		t.Error("the address target is wanted on darwin")
	}
	tick(t, d, 2)
	if n := rh.count("ip -o addr show"); n != 0 {
		t.Errorf("%d address reads on darwin", n)
	}
	if w := rh.addrCmds(); len(w) != 0 {
		t.Errorf("address writes on darwin: %q", w)
	}
}

// A failed read of the interface skips the address target without a write,
// and the first tick that reads it again converges it, logging the recovery
// once.
func TestAddressObserveFailsThenRecovers(t *testing.T) {
	d, rh, ls := newAddrDaemon(t)
	rh.mu.Lock()
	rh.readErr["ip -o addr show dev wg0"] = fmt.Errorf("exit status 1")
	rh.mu.Unlock()
	tick(t, d, 3)
	if w := rh.addrCmds(); len(w) != 0 {
		t.Fatalf("address writes while the read fails: %q", w)
	}
	rh.mu.Lock()
	delete(rh.readErr, "ip -o addr show dev wg0")
	rh.mu.Unlock()
	tick(t, d, 2)
	if got, want := rh.addrCmds(), []string{"ip addr add 10.99.0.1/16 dev wg0"}; !sameLines(got, want) {
		t.Errorf("address commands after the recovery %q, want %q", got, want)
	}
	if n := ls.count("Cannot read the addresses"); n != 1 {
		t.Errorf("the failed read was logged %d times, want once", n)
	}
	if n := ls.count("Reading the addresses of wg0 works again"); n != 1 {
		t.Errorf("the recovery was logged %d times, want once", n)
	}
}

// One failed add or removal is a result of its own: the others still run.
func TestAddressFailureDoesNotStopTheLoop(t *testing.T) {
	enforceAddressRemoval(t, true)
	d, rh, _ := newAddrDaemon(t, "10.99.5.5/16", "10.99.7.7/16")
	d.localNode.MeshIPv6 = "fd99::1"
	rh.addrDelErr["10.99.5.5/16"] = exitError{1}
	rh.addrAddErr["10.99.0.1/16"] = exitError{1}
	tick(t, d, 1)
	want := []string{"ip addr del 10.99.5.5/16 dev wg0", "ip addr del 10.99.7.7/16 dev wg0", "ip addr add 10.99.0.1/16 dev wg0", "ip addr add fd99::1/64 dev wg0"}
	if got := rh.addrCmds(); !sameLines(got, want) {
		t.Errorf("address commands %q, want %q", got, want)
	}
	if got, want := rh.held(), sorted("10.99.5.5/16", "fd99::1/64"); !sameLines(got, want) {
		t.Errorf("interface holds %q, want %q", got, want)
	}
}

// "File exists" on an add is success in the item's result, not only in the
// log; any other failure of the add is an error.
func TestApplyAddressesResults(t *testing.T) {
	d, rh, _ := newAddrDaemon(t, "10.99.0.1/16")
	before := addressChanges()
	v4 := netip.MustParsePrefix("10.99.0.1/16")
	got := d.applyAddresses(addressPlan{Add: []netip.Prefix{v4}})
	if len(got) != 1 || got[0].Op != "add" || got[0].Key != "10.99.0.1/16" || got[0].Err != nil {
		t.Errorf("an add of a held address: %+v, want one add result with no error", got)
	}
	if addressChanges() != before {
		t.Error("File exists counted as a change")
	}
	rh.addrAddErr["10.99.8.8/16"] = exitError{1}
	got = d.applyAddresses(addressPlan{Add: []netip.Prefix{netip.MustParsePrefix("10.99.8.8/16")}})
	if len(got) != 1 || got[0].Err == nil {
		t.Errorf("a failed add: %+v, want one result with an error", got)
	}
}

// Only a longer prefix of the node's address is held: the /16 is added and,
// once removal is enforced, the /24 removed; before that the /24 is only
// warned about.
func TestOnlyWrongPrefixLengthHeld(t *testing.T) {
	t.Run("enforced", func(t *testing.T) {
		enforceAddressRemoval(t, true)
		d, rh, _ := newAddrDaemon(t, "10.99.0.1/24")
		tick(t, d, 2)
		if got, want := rh.addrCmds(), []string{"ip addr del 10.99.0.1/24 dev wg0", "ip addr add 10.99.0.1/16 dev wg0"}; !sameLines(got, want) {
			t.Errorf("address commands %q, want %q", got, want)
		}
		if got := rh.held(); !sameLines(got, []string{"10.99.0.1/16"}) {
			t.Errorf("interface holds %q", got)
		}
	})
	t.Run("release N", func(t *testing.T) {
		enforceAddressRemoval(t, false)
		d, rh, ls := newAddrDaemon(t, "10.99.0.1/24")
		tick(t, d, 2)
		if got, want := rh.addrCmds(), []string{"ip addr add 10.99.0.1/16 dev wg0"}; !sameLines(got, want) {
			t.Errorf("address commands %q, want %q", got, want)
		}
		if n := ls.count("stale address 10.99.0.1/24"); n != 1 {
			t.Errorf("the /24 was warned about %d times, want once", n)
		}
	})
}
