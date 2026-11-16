package daemon

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/atvirokodosprendimai/wgmesh/pkg/routes"
)

// kernelRoute is one route of the fake table: a canonical network, its
// gateway and the device it leaves by.
type kernelRoute struct{ net, gw, dev string }

// routeHost is the routing table and the command line of one Linux node,
// reached through MockCommandExecutor: `ip route show`, `replace` and `del`
// change or read the table the way iproute2 prints and takes them; every other
// command goes to the fakeHost (sysctl, iptables). The table keeps one entry
// per (network, device).
type routeHost struct {
	host       *fakeHost
	mu         sync.Mutex
	table      []kernelRoute
	extra      map[int][]string // family -> lines without a gateway, printed as they are
	log        []string         // every command that ran
	readErr    map[string]error // full command line -> what reading fails with
	replaceErr map[string]error // canonical network -> what `ip route replace` fails with
}

func newRouteHost(h *fakeHost) *routeHost {
	return &routeHost{host: h, extra: map[int][]string{}, readErr: map[string]error{}, replaceErr: map[string]error{}}
}

// canonRoute is the canonical network of a printed or canonical one.
func canonRoute(network, gw string) string {
	return routes.Canonical(network, strings.Contains(network, ":") || strings.Contains(gw, ":"))
}

// printRoute is how the kernel prints a network: a host without its prefix,
// a default route as the word default.
func printRoute(canon string) string {
	switch {
	case canon == "0.0.0.0/0" || canon == "::/0":
		return "default"
	case strings.HasSuffix(canon, "/32"):
		return strings.TrimSuffix(canon, "/32")
	case strings.HasSuffix(canon, "/128"):
		return strings.TrimSuffix(canon, "/128")
	}
	return canon
}

// add puts a route in the table.
func (r *routeHost) add(network, gw, dev string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.put(kernelRoute{canonRoute(network, gw), gw, dev})
}

func (r *routeHost) put(rt kernelRoute) {
	var keep []kernelRoute
	for _, x := range r.table {
		if x.net != rt.net || x.dev != rt.dev {
			keep = append(keep, x)
		}
	}
	r.table = append(keep, rt)
}

func (r *routeHost) has(network, gw, dev string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.table {
		if x == (kernelRoute{canonRoute(network, gw), gw, dev}) {
			return true
		}
	}
	return false
}

func (r *routeHost) command(name string, args ...string) Command {
	return &MockCommand{
		runFunc:            func() error { _, err := r.exec(name, args...); return err },
		combinedOutputFunc: func() ([]byte, error) { return r.exec(name, args...) },
		outputFunc:         func() ([]byte, error) { return r.exec(name, args...) },
	}
}

func (r *routeHost) exec(name string, args ...string) ([]byte, error) {
	line := name + " " + strings.Join(args, " ")
	r.mu.Lock()
	r.log = append(r.log, line)
	r.mu.Unlock()
	if name != "ip" {
		return r.host.exec(name, args...)
	}
	family := 4
	a := args
	if len(a) > 0 && (a[0] == "-4" || a[0] == "-6") {
		family = int(a[0][1] - '0')
		a = a[1:]
	}
	if len(a) < 3 || a[0] != "route" {
		return nil, fmt.Errorf("fake host: unexpected command %q", line)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	v6 := family == 6
	switch a[1] {
	case "show":
		if err := r.readErr[line]; err != nil {
			return nil, err
		}
		var out strings.Builder
		for _, x := range r.table {
			if strings.Contains(x.net, ":") != v6 {
				continue
			}
			switch {
			case a[2] == "dev" && x.dev == a[3]:
				fmt.Fprintf(&out, "%s via %s dev %s proto static\n", printRoute(x.net), x.gw, x.dev)
			case a[2] == "default" && printRoute(x.net) == "default":
				fmt.Fprintf(&out, "default via %s dev %s\n", x.gw, x.dev)
			}
		}
		if a[2] == "dev" {
			for _, l := range r.extra[family] {
				out.WriteString(l + "\n")
			}
		}
		return []byte(out.String()), nil
	case "replace": // ip route replace <net> via <gw> dev <dev>
		if len(a) != 7 {
			return nil, fmt.Errorf("fake host: unexpected command %q", line)
		}
		if err := r.replaceErr[canonRoute(a[2], a[4])]; err != nil {
			return []byte("RTNETLINK answers: Operation not permitted"), err
		}
		r.put(kernelRoute{canonRoute(a[2], a[4]), a[4], a[6]})
		return nil, nil
	case "del": // ip route del <net> via <gw> dev <dev>
		if len(a) != 7 {
			return nil, fmt.Errorf("fake host: unexpected command %q", line)
		}
		want := kernelRoute{canonRoute(a[2], a[4]), a[4], a[6]}
		for i, x := range r.table {
			if x == want {
				r.table = append(r.table[:i], r.table[i+1:]...)
				return nil, nil
			}
		}
		return []byte("RTNETLINK answers: No such process"), exitError{2}
	}
	return nil, fmt.Errorf("fake host: unexpected command %q", line)
}

// routeWrites are the route commands that changed, or tried to change, the table.
func (r *routeHost) routeWrites() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, c := range r.log {
		if strings.HasPrefix(c, "ip route replace ") || strings.HasPrefix(c, "ip route del ") || strings.HasPrefix(c, "ip route add ") {
			out = append(out, c)
		}
	}
	return out
}

// allWrites are every command that writes: routes and forwarding.
func (r *routeHost) allWrites() []string {
	out := r.routeWrites()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.log {
		if strings.HasPrefix(c, "sysctl -w") || strings.HasPrefix(c, "iptables -A") {
			out = append(out, c)
		}
	}
	return out
}

func (r *routeHost) count(prefix string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.log {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

// newRouteDaemon is a Linux daemon on mesh interface wg0 whose commands go to
// rh. The tests that use it swap the package executor and so never run in
// parallel.
func newRouteDaemon(t *testing.T, rh *routeHost, ls *logSink, k *fakeKernel) *Daemon {
	t.Helper()
	d := newConvergeDaemon(t, k, ls)
	d.goos = "linux"
	d.config.InterfaceName = "wg0"
	d.readFileFn = rh.host.readFile
	old := cmdExecutor
	cmdExecutor = &MockCommandExecutor{lookPathFunc: rh.host.lookPath, commandFunc: rh.command}
	t.Cleanup(func() { cmdExecutor = old })
	return d
}

// convergedHost is a host whose forwarding is already as converge wants it.
func convergedHost() *routeHost {
	return newRouteHost(&fakeHost{ipForward: true, ruleInstalled: true, iptablesPresent: true})
}

func enforceRouteGuard(t *testing.T, v bool) {
	t.Helper()
	old := EnforceRouteGuard.Load()
	EnforceRouteGuard.Store(v)
	t.Cleanup(func() { EnforceRouteGuard.Store(old) })
}

// ticks runs n converge ticks and returns the route writes of each.
func ticks(t *testing.T, d *Daemon, rh *routeHost, peers []*PeerInfo, n int) [][]string {
	t.Helper()
	var out [][]string
	for i := 0; i < n; i++ {
		before := len(rh.routeWrites())
		if d.converge(peers) == nil {
			t.Fatalf("tick %d: converge skipped the tick", i+1)
		}
		out = append(out, rh.routeWrites()[before:])
	}
	return out
}

func replaceCmd(network, gw string) string {
	return "ip route replace " + network + " via " + gw + " dev wg0"
}

func kernelAllowed(k *fakeKernel, key string) []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return append([]string(nil), k.peers[key].AllowedIPs...)
}

func TestObserveRoutesBothFamilies(t *testing.T) {
	rh := convergedHost()
	rh.add("default", "10.99.0.2", "wg0")
	rh.add("10.1.0.5", "10.99.0.3", "wg0")
	rh.add("192.168.1.0/24", "10.99.0.2", "wg0")
	rh.add("default", "fd99::2", "wg0")
	rh.add("fd00:5::7", "fd99::3", "wg0")
	rh.add("fd00:6::/64", "fd99::2", "wg0")
	rh.add("10.7.0.0/24", "10.99.0.2", "eth0") // another device: not read
	rh.extra[4] = []string{"10.99.0.0/16 dev wg0 proto kernel scope link src 10.99.0.1"}
	rh.extra[6] = []string{"fe80::/64 dev wg0 proto kernel metric 256 pref medium"}
	d := newRouteDaemon(t, rh, &logSink{}, newFakeKernel())

	var obs kernelObserved
	d.observeRoutes(&obs)

	want4 := []routes.Entry{{Network: "0.0.0.0/0", Gateway: "10.99.0.2"}, {Network: "10.1.0.5/32", Gateway: "10.99.0.3"}, {Network: "192.168.1.0/24", Gateway: "10.99.0.2"}}
	want6 := []routes.Entry{{Network: "::/0", Gateway: "fd99::2"}, {Network: "fd00:5::7/128", Gateway: "fd99::3"}, {Network: "fd00:6::/64", Gateway: "fd99::2"}}
	if !reflect.DeepEqual(obs.Routes4, want4) {
		t.Errorf("IPv4 routes = %v, want %v", obs.Routes4, want4)
	}
	if !reflect.DeepEqual(obs.Routes6, want6) {
		t.Errorf("IPv6 routes = %v, want %v", obs.Routes6, want6)
	}
	if len(obs.Errs) != 0 {
		t.Errorf("observe errors = %v, want none", obs.Errs)
	}
	for _, c := range []string{"ip route show dev wg0", "ip -6 route show dev wg0"} {
		if got := rh.count(c); got != 1 {
			t.Errorf("%q ran %d times, want once", c, got)
		}
	}
}

// A family whose read fails is skipped for the tick with one log line and
// removes nothing; the other family is still converged.
func TestObserveRoutesFamilyFailureSkipsOnlyThatFamily(t *testing.T) {
	rh := convergedHost()
	rh.add("10.20.0.0/24", "10.99.0.9", "wg0")
	rh.add("fd00:9::/64", "fd99::9", "wg0")
	rh.readErr["ip -6 route show dev wg0"] = errors.New("RTNETLINK answers: Operation not supported")
	ls := &logSink{}
	d := newRouteDaemon(t, rh, ls, newFakeKernel())

	got := ticks(t, d, rh, nil, 3)
	if want := []string{"ip route del 10.20.0.0/24 via 10.99.0.9 dev wg0"}; !sameLines(got[0], want) || len(got[1]) != 0 || len(got[2]) != 0 {
		t.Errorf("route writes per tick = %q, want only the IPv4 removal on tick 1", got)
	}
	if !rh.has("fd00:9::/64", "fd99::9", "wg0") {
		t.Error("the IPv6 route was removed although its family could not be read")
	}
	if n := ls.count("IPv6 routes"); n != 1 {
		t.Errorf("the failed IPv6 read was logged %d times in 3 ticks, want 1; log: %q", n, ls.lines)
	}
	delete(rh.readErr, "ip -6 route show dev wg0")
	if got := ticks(t, d, rh, nil, 1); !sameLines(got[0], []string{"ip route del fd00:9::/64 via fd99::9 dev wg0"}) {
		t.Errorf("after the read works again the stale route is removed, writes = %q", got)
	}
}

func TestSharedSubnetNoFlapOverCycles(t *testing.T) {
	rh := convergedHost()
	d := newRouteDaemon(t, rh, &logSink{}, newFakeKernel())
	peers := []*PeerInfo{
		testPeer("keyB", "10.99.0.3", "203.0.113.3:51820", "192.168.50.0/24"),
		testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "192.168.50.0/24"),
	}

	got := ticks(t, d, rh, peers, 5)
	if want := []string{replaceCmd("192.168.50.0/24", "10.99.0.2")}; !sameLines(got[0], want) {
		t.Fatalf("tick 1 route writes = %q, want %q", got[0], want)
	}
	for i := 1; i < 5; i++ {
		if len(got[i]) != 0 {
			t.Errorf("tick %d wrote routes again: %q", i+1, got[i])
		}
	}
}

func TestDefaultRouteNoChurn(t *testing.T) {
	rh := convergedHost()
	d := newRouteDaemon(t, rh, &logSink{}, newFakeKernel())
	peers := []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "0.0.0.0/0")}

	got := ticks(t, d, rh, peers, 3)
	if want := []string{replaceCmd("0.0.0.0/0", "10.99.0.2")}; !sameLines(got[0], want) {
		t.Fatalf("tick 1 route writes = %q, want %q", got[0], want)
	}
	if len(got[1]) != 0 || len(got[2]) != 0 {
		t.Errorf("a default route churned: tick 2 %q, tick 3 %q", got[1], got[2])
	}
}

func TestIPv6RouteUsesIPv6Gateway(t *testing.T) {
	rh := convergedHost()
	d := newRouteDaemon(t, rh, &logSink{}, newFakeKernel())
	peer := testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "fd00:5::/64", "192.168.7.0/24")
	peer.MeshIPv6 = "fd99::2"

	got := ticks(t, d, rh, []*PeerInfo{peer}, 3)
	want := []string{replaceCmd("192.168.7.0/24", "10.99.0.2"), replaceCmd("fd00:5::/64", "fd99::2")}
	if !sameLines(got[0], want) {
		t.Fatalf("tick 1 route writes = %q, want %q", got[0], want)
	}
	if len(got[1]) != 0 || len(got[2]) != 0 {
		t.Errorf("the IPv6 route was written again: tick 2 %q, tick 3 %q", got[1], got[2])
	}
}

func TestPeerWithoutMeshIPv6SkippedLoggedOnce(t *testing.T) {
	rh := convergedHost()
	ls := &logSink{}
	k := newFakeKernel()
	d := newRouteDaemon(t, rh, ls, k)
	peer := testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "fd00:5::/64", "192.168.7.0/24")

	got := ticks(t, d, rh, []*PeerInfo{peer}, 10)
	if want := []string{replaceCmd("192.168.7.0/24", "10.99.0.2")}; !sameLines(got[0], want) {
		t.Errorf("tick 1 route writes = %q, want only the IPv4 route %q", got[0], want)
	}
	for _, c := range rh.log {
		if strings.Contains(c, "fd00:5::") && !strings.Contains(c, "route show") {
			t.Errorf("the IPv6 network was issued without a gateway: %q", c)
		}
	}
	if n := ls.count("fd00:5::/64"); n != 1 {
		t.Errorf("the skipped IPv6 network was logged %d times in 10 ticks, want 1; log: %q", n, ls.lines)
	}
	if allowed := kernelAllowed(k, "keyA"); contains(allowed, "fd00:5::/64") {
		t.Errorf("the skipped IPv6 network is in the peer's AllowedIPs: %v", allowed)
	}
}

func TestRouteAddFailureDoesNotAbortOthers(t *testing.T) {
	rh := convergedHost()
	rh.replaceErr["192.168.2.0/24"] = errors.New("exit status 2")
	ls := &logSink{}
	d := newRouteDaemon(t, rh, ls, newFakeKernel())
	peers := []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "192.168.1.0/24", "192.168.2.0/24", "192.168.3.0/24")}

	got := ticks(t, d, rh, peers, 5)
	want := []string{replaceCmd("192.168.1.0/24", "10.99.0.2"), replaceCmd("192.168.2.0/24", "10.99.0.2"), replaceCmd("192.168.3.0/24", "10.99.0.2")}
	if !sameLines(got[0], want) {
		t.Fatalf("tick 1 route writes = %q, want all three tried: %q", got[0], want)
	}
	for i := 1; i < 5; i++ {
		if w := []string{replaceCmd("192.168.2.0/24", "10.99.0.2")}; !sameLines(got[i], w) {
			t.Errorf("tick %d route writes = %q, want only the failed route again", i+1, got[i])
		}
	}
	if !rh.has("192.168.1.0/24", "10.99.0.2", "wg0") || !rh.has("192.168.3.0/24", "10.99.0.2", "wg0") {
		t.Error("the routes around the failed one were not installed")
	}
	if n := ls.count("Failed to replace route"); n != 1 {
		t.Errorf("the failed route was logged %d times in 5 ticks, want 1; log: %q", n, ls.lines)
	}
}

// attachedFixture is a daemon whose host is attached to 192.168.50.0/24 and
// sits on the mesh subnet, and a peer that advertises, among others, networks
// equal to and inside the attached one, a supernet of it and one inside the
// mesh subnet.
func attachedFixture(t *testing.T) (*Daemon, *routeHost, *fakeKernel, *logSink, []*PeerInfo, string) {
	t.Helper()
	rh := convergedHost()
	ls := &logSink{}
	k := newFakeKernel()
	d := newRouteDaemon(t, rh, ls, k)
	mesh := d.meshNet().IP.To4()
	insideMesh := fmt.Sprintf("10.%d.7.0/24", mesh[1])
	d.localSubnetsFn = func() []*net.IPNet {
		return []*net.IPNet{
			{IP: net.IPv4(192, 168, 50, 1), Mask: net.CIDRMask(24, 32)},
			{IP: net.IPv4(10, mesh[1], 0, 1), Mask: net.CIDRMask(16, 32)},
		}
	}
	peers := []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820",
		"192.168.50.0/24", "192.168.50.128/25", "192.168.0.0/16", "172.16.0.0/16", insideMesh)}
	return d, rh, k, ls, peers, insideMesh
}

func TestAttachedNetworkWarnsThenSkips(t *testing.T) {
	warnings := func() float64 { return testutil.ToFloat64(kernelWarnings.WithLabelValues("attached-network")) }

	t.Run("N installs and counts", func(t *testing.T) {
		enforceRouteGuard(t, false)
		d, rh, k, ls, peers, insideMesh := attachedFixture(t)
		before := warnings()
		ticks(t, d, rh, peers, 5)
		for _, n := range []string{"192.168.50.0/24", "192.168.50.128/25", "192.168.0.0/16", "172.16.0.0/16", insideMesh} {
			if !rh.has(n, "10.99.0.2", "wg0") {
				t.Errorf("at N the route %s is not installed", n)
			}
			if !contains(kernelAllowed(k, "keyA"), n) {
				t.Errorf("at N %s is not in AllowedIPs %v", n, kernelAllowed(k, "keyA"))
			}
		}
		if got := warnings() - before; got != 2 {
			t.Errorf("attached-network counted %v times over 5 ticks, want 2 (equal and inside, once each)", got)
		}
		if got := ls.count("attached-network"); got != 2 {
			t.Errorf("attached-network logged %d times over 5 ticks, want 2; log: %q", got, ls.lines)
		}
	})

	t.Run("N+1 drops equal and inside, keeps the supernet", func(t *testing.T) {
		enforceRouteGuard(t, true)
		d, rh, k, _, peers, insideMesh := attachedFixture(t)
		before := warnings()
		ticks(t, d, rh, peers, 5)
		for _, n := range []string{"192.168.50.0/24", "192.168.50.128/25"} {
			if rh.has(n, "10.99.0.2", "wg0") {
				t.Errorf("with EnforceRouteGuard the route %s is installed", n)
			}
			if contains(kernelAllowed(k, "keyA"), n) {
				t.Errorf("with EnforceRouteGuard %s is in AllowedIPs %v", n, kernelAllowed(k, "keyA"))
			}
		}
		for _, n := range []string{"192.168.0.0/16", "172.16.0.0/16", insideMesh} {
			if !rh.has(n, "10.99.0.2", "wg0") || !contains(kernelAllowed(k, "keyA"), n) {
				t.Errorf("with EnforceRouteGuard %s must stay (route %v, AllowedIPs %v)", n, rh.has(n, "10.99.0.2", "wg0"), kernelAllowed(k, "keyA"))
			}
		}
		if got := warnings() - before; got != 0 {
			t.Errorf("an enforced rule is not a warning: counted %v", got)
		}
	})
}

func TestDefaultRouteWarnsWhenHostHasDefault(t *testing.T) {
	warnings := func() float64 { return testutil.ToFloat64(kernelWarnings.WithLabelValues("host-default")) }
	peers := []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "0.0.0.0/0")}

	t.Run("host default on another device", func(t *testing.T) {
		rh := convergedHost()
		rh.add("default", "192.168.1.1", "eth0")
		ls := &logSink{}
		d := newRouteDaemon(t, rh, ls, newFakeKernel())
		before := warnings()
		got := ticks(t, d, rh, peers, 3)
		if want := []string{replaceCmd("0.0.0.0/0", "10.99.0.2")}; !sameLines(got[0], want) {
			t.Errorf("tick 1 route writes = %q, want the default route still installed: %q", got[0], want)
		}
		if n := ls.count("host-default"); n != 1 {
			t.Errorf("host-default logged %d times in 3 ticks, want 1; log: %q", n, ls.lines)
		}
		if got := warnings() - before; got != 1 {
			t.Errorf("host-default counted %v times in 3 ticks, want 1", got)
		}
		if n := rh.count("ip -4 route show default"); n != 1 {
			t.Errorf("the host default was read %d times in 3 ticks, want once, when the route was added", n)
		}
	})

	t.Run("no host default", func(t *testing.T) {
		rh := convergedHost()
		ls := &logSink{}
		d := newRouteDaemon(t, rh, ls, newFakeKernel())
		before := warnings()
		ticks(t, d, rh, peers, 3)
		if n := ls.count("host-default"); n != 0 {
			t.Errorf("host-default logged without a host default: %q", ls.lines)
		}
		if got := warnings() - before; got != 0 {
			t.Errorf("host-default counted %v times without a host default", got)
		}
	})
}

func TestConvergeSteadyStateIssuesNoWrites(t *testing.T) {
	rh := newRouteHost(&fakeHost{iptablesPresent: true}) // forwarding is not set yet: tick 1 writes it
	k := newFakeKernel()
	d := newRouteDaemon(t, rh, &logSink{}, k)
	a := testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "192.168.50.0/24", "0.0.0.0/0", "fd00:5::/64")
	a.MeshIPv6 = "fd99::2"
	b := testPeer("keyB", "10.99.0.3", "203.0.113.3:51820", "192.168.50.0/24", "192.168.60.0/24")
	peers := []*PeerInfo{a, b}

	writes := func() int { return len(rh.allWrites()) + len(k.setCalls()) + len(k.removes) }
	var perTick []int
	for i := 0; i < 10; i++ {
		before := writes()
		if d.converge(peers) == nil {
			t.Fatalf("tick %d: converge skipped the tick", i+1)
		}
		perTick = append(perTick, writes()-before)
	}
	if perTick[0] == 0 {
		t.Fatal("tick 1 wrote nothing: the fake kernel was not empty")
	}
	for i := 1; i < 10; i++ {
		if perTick[i] != 0 {
			t.Errorf("tick %d wrote %d times over a converged kernel: routes %q, all %q", i+1, perTick[i], rh.routeWrites(), rh.allWrites())
		}
	}
	for _, c := range []struct{ n, gw string }{{"192.168.50.0/24", "10.99.0.2"}, {"0.0.0.0/0", "10.99.0.2"}, {"fd00:5::/64", "fd99::2"}, {"192.168.60.0/24", "10.99.0.3"}} {
		if !rh.has(c.n, c.gw, "wg0") {
			t.Errorf("the converged table lacks %s via %s", c.n, c.gw)
		}
	}
}

// The tick and the release switch run on different goroutines.
func TestRouteTickRaceFree(t *testing.T) {
	enforceRouteGuard(t, false)
	d, _, _, _, peers, _ := attachedFixture(t)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			EnforceRouteGuard.Store(i%2 == 0)
		}
	}()
	for i := 0; i < 200; i++ {
		if d.converge(peers) == nil {
			t.Fatalf("tick %d: converge skipped the tick", i+1)
		}
	}
	wg.Wait()
}
