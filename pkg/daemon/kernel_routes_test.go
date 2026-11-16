package daemon

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
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
	extra      map[int][]string  // family -> lines without a gateway, printed as they are
	log        []string          // every command that ran
	readErr    map[string]error  // full command line -> what reading fails with
	replaceErr map[string]error  // canonical network -> what `ip route replace` fails with
	addrs      map[string]string // network -> `ip -o addr show to <network>` output
	delErr     map[string]error  // canonical network -> what `ip route del` fails with
	ifAddrs    []string          // addresses on the mesh interface, as `ip addr add` takes them
	addrHidden bool              // `ip -o addr show dev` prints nothing, whatever is held
	addrAddErr map[string]error  // address -> what `ip addr add` fails with
	addrDelErr map[string]error  // address -> what `ip addr del` fails with
}

// nodeMeshAddr is the mesh address of the fake node (MeshIP 10.99.0.1 of
// newConvergeDaemon, the /16 of Config.PrefixLen) that its interface holds.
const nodeMeshAddr = "10.99.0.1/16"

func newRouteHost(h *fakeHost) *routeHost {
	return &routeHost{host: h, extra: map[int][]string{}, readErr: map[string]error{}, replaceErr: map[string]error{}, addrs: map[string]string{}, delErr: map[string]error{}, addrAddErr: map[string]error{}, addrDelErr: map[string]error{}, ifAddrs: []string{nodeMeshAddr}}
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

// printGateway is how the kernel prints a gateway: IPv6 compressed and in
// lower case, whatever form it was given in.
func printGateway(gw string) string {
	if a, err := netip.ParseAddr(gw); err == nil {
		return a.String()
	}
	return gw
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
	for _, p := range []string{"ip -4 -o addr show to ", "ip -6 -o addr show to "} {
		if network, ok := strings.CutPrefix(line, p); ok {
			r.mu.Lock()
			defer r.mu.Unlock()
			return []byte(r.addrs[network]), nil
		}
	}
	if ok, out, err := r.addrExec(line, args); ok {
		return out, err
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
				if x.gw == "" {
					fmt.Fprintf(&out, "default dev %s\n", x.dev)
				} else {
					fmt.Fprintf(&out, "default via %s dev %s\n", x.gw, x.dev)
				}
			}
		}
		if a[2] == "dev" {
			for _, l := range r.extra[family] {
				out.WriteString(l + "\n")
			}
		}
		return []byte(out.String()), nil
	case "replace": // ip route replace <net> [via <gw>] dev <dev>
		// The route key is the destination (and metric), not the device: the
		// kernel replaces a route of the same destination that leaves by
		// another device (a host default route on eth0 is gone after the
		// mesh default replaced it on wg0).
		gw, dev := "", ""
		switch {
		case len(a) == 7 && a[3] == "via" && a[5] == "dev":
			gw, dev = a[4], a[6]
		case len(a) == 5 && a[3] == "dev":
			dev = a[4]
		default:
			return nil, fmt.Errorf("fake host: unexpected command %q", line)
		}
		canon := routes.Canonical(a[2], v6 || strings.Contains(a[2], ":") || strings.Contains(gw, ":"))
		if err := r.replaceErr[canon]; err != nil {
			return []byte("RTNETLINK answers: Operation not permitted"), err
		}
		var keep []kernelRoute
		for _, x := range r.table {
			if x.net != canon {
				keep = append(keep, x)
			}
		}
		r.table = keep
		r.put(kernelRoute{canon, printGateway(gw), dev})
		return nil, nil
	case "add": // ip route add <net> dev <dev> ...: the restore of a connected route; not kept
		return nil, nil
	case "del": // ip route del <net> via <gw> dev <dev>
		if len(a) != 7 {
			return nil, fmt.Errorf("fake host: unexpected command %q", line)
		}
		if err := r.delErr[canonRoute(a[2], a[4])]; err != nil {
			return []byte("RTNETLINK answers: Operation not permitted"), err
		}
		want := kernelRoute{canonRoute(a[2], a[4]), printGateway(a[4]), a[6]}
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
		if strings.HasPrefix(c, "ip route replace ") || strings.HasPrefix(c, "ip route del ") || strings.HasPrefix(c, "ip route add ") || strings.HasPrefix(c, "ip -6 route replace ") {
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
		if strings.HasPrefix(c, "sysctl -w") || strings.HasPrefix(c, "iptables -A") || strings.HasPrefix(c, "ip addr add ") || strings.HasPrefix(c, "ip addr del ") {
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
	d := newRouteDaemon(t, rh, &logSink{}, newFakeKernel())

	got := ticks(t, d, rh, nil, 3)
	if want := []string{"ip route del 10.20.0.0/24 via 10.99.0.9 dev wg0"}; !sameLines(got[0], want) || len(got[1]) != 0 || len(got[2]) != 0 {
		t.Errorf("route writes per tick = %q, want only the IPv4 removal on tick 1", got)
	}
	if !rh.has("fd00:9::/64", "fd99::9", "wg0") {
		t.Error("the IPv6 route was removed although its family could not be read")
	}
	var obs kernelObserved
	d.observeRoutes(&obs)
	if obs.Errs[targetRoutes6] == nil || obs.Errs[targetRoutes4] != nil || len(obs.Errs) != 1 {
		t.Errorf("observe errors = %v, want only the IPv6 family", obs.Errs)
	}
	if got := d.routeObserveErr[targetRoutes6]; got == "" || d.routeObserveErr[targetRoutes4] != "" {
		t.Errorf("remembered observe errors = %v, want only the IPv6 family", d.routeObserveErr)
	}
	delete(rh.readErr, "ip -6 route show dev wg0")
	if got := ticks(t, d, rh, nil, 1); !sameLines(got[0], []string{"ip route del fd00:9::/64 via fd99::9 dev wg0"}) {
		t.Errorf("after the read works again the stale route is removed, writes = %q", got)
	}
	if len(d.routeObserveErr) != 0 {
		t.Errorf("the recovery left remembered observe errors: %v", d.routeObserveErr)
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
	d := newRouteDaemon(t, rh, &logSink{}, newFakeKernel())
	peers := []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "192.168.1.0/24", "192.168.2.0/24", "192.168.3.0/24")}

	failed := map[string]bool{}
	for _, r := range d.routeResults(peers) {
		if r.Op == "replace" {
			failed[r.Key] = r.Err != nil
		}
	}
	wantFailed := map[string]bool{"192.168.1.0/24 via 10.99.0.2": false, "192.168.2.0/24 via 10.99.0.2": true, "192.168.3.0/24 via 10.99.0.2": false}
	if !reflect.DeepEqual(failed, wantFailed) {
		t.Errorf("tick 1 results (route -> failed) = %v, want all three tried and only the second failed: %v", failed, wantFailed)
	}
	want := []string{replaceCmd("192.168.1.0/24", "10.99.0.2"), replaceCmd("192.168.2.0/24", "10.99.0.2"), replaceCmd("192.168.3.0/24", "10.99.0.2")}
	if got := rh.routeWrites(); !sameLines(got, want) {
		t.Fatalf("tick 1 route writes = %q, want all three tried: %q", got, want)
	}
	got := ticks(t, d, rh, peers, 4)
	for i := range got {
		if w := []string{replaceCmd("192.168.2.0/24", "10.99.0.2")}; !sameLines(got[i], w) {
			t.Errorf("tick %d route writes = %q, want only the failed route again", i+2, got[i])
		}
	}
	if !rh.has("192.168.1.0/24", "10.99.0.2", "wg0") || !rh.has("192.168.3.0/24", "10.99.0.2", "wg0") {
		t.Error("the routes around the failed one were not installed")
	}
}

// A failed `ip route del` never skips the others, and is tried again.
func TestRouteDelFailureDoesNotAbortOthers(t *testing.T) {
	rh := convergedHost()
	for _, n := range []string{"10.20.0.0/24", "10.21.0.0/24", "10.22.0.0/24"} {
		rh.add(n, "10.99.0.9", "wg0")
	}
	rh.delErr["10.21.0.0/24"] = errors.New("exit status 2")
	d := newRouteDaemon(t, rh, &logSink{}, newFakeKernel())

	failed := map[string]bool{}
	for _, r := range d.routeResults(nil) {
		if r.Op == "del" {
			failed[r.Key] = r.Err != nil
		}
	}
	wantFailed := map[string]bool{"10.20.0.0/24 via 10.99.0.9": false, "10.21.0.0/24 via 10.99.0.9": true, "10.22.0.0/24 via 10.99.0.9": false}
	if !reflect.DeepEqual(failed, wantFailed) {
		t.Errorf("tick 1 results (route -> failed) = %v, want all three tried and only the second failed: %v", failed, wantFailed)
	}
	if rh.has("10.20.0.0/24", "10.99.0.9", "wg0") || rh.has("10.22.0.0/24", "10.99.0.9", "wg0") || !rh.has("10.21.0.0/24", "10.99.0.9", "wg0") {
		t.Errorf("the routes around the failed removal were not removed, or the failed one was: %q", rh.routeWrites())
	}
	if got := ticks(t, d, rh, nil, 1); !sameLines(got[0], []string{"ip route del 10.21.0.0/24 via 10.99.0.9 dev wg0"}) {
		t.Errorf("the failed removal was not tried again alone, writes = %q", got)
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
	d.localNode.MeshIPv6 = "fd99::1"
	rh.ifAddrs = []string{nodeMeshAddr, "fd99::1/64"} // the interface already holds the node's addresses
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
	d, rh, _, _, peers, _ := attachedFixture(t)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			EnforceRouteGuard.Store(i%2 == 0)
		}
	}()
	for i := 0; i < 200; i++ {
		ds := d.converge(peers)
		if ds == nil {
			t.Fatalf("tick %d: converge skipped the tick", i+1)
		}
		// Each tick reads the switch once: the routes that do not depend on it
		// are always wanted and installed, the two attached ones are both in or both out.
		wanted := map[string]bool{}
		for _, r := range ds.Routes {
			wanted[r.Network] = true
		}
		for _, n := range []string{"192.168.0.0/16", "172.16.0.0/16"} {
			if !wanted[n] || !rh.has(n, "10.99.0.2", "wg0") {
				t.Fatalf("tick %d: %s is wanted %v, installed %v", i+1, n, wanted[n], rh.has(n, "10.99.0.2", "wg0"))
			}
		}
		if wanted["192.168.50.0/24"] != wanted["192.168.50.128/25"] {
			t.Fatalf("tick %d: the attached networks were decided by two reads of the switch: %v", i+1, wanted)
		}
	}
	wg.Wait()
}

func TestDefaultRouteWarnsWhenHostHasIPv6Default(t *testing.T) {
	warnings := func() float64 { return testutil.ToFloat64(kernelWarnings.WithLabelValues("host-default")) }
	rh := convergedHost()
	rh.add("default", "fe80::1", "eth0")
	ls := &logSink{}
	d := newRouteDaemon(t, rh, ls, newFakeKernel())
	peer := testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "::/0")
	peer.MeshIPv6 = "fd99::2"
	before := warnings()

	got := ticks(t, d, rh, []*PeerInfo{peer}, 3)
	if want := []string{replaceCmd("::/0", "fd99::2")}; !sameLines(got[0], want) {
		t.Errorf("tick 1 route writes = %q, want the IPv6 default route still installed: %q", got[0], want)
	}
	if n := ls.count("host-default: this host already has an IPv6 default route on eth0"); n != 1 {
		t.Errorf("host-default logged %d times in 3 ticks, want 1; log: %q", n, ls.lines)
	}
	if got := warnings() - before; got != 1 {
		t.Errorf("host-default counted %v times in 3 ticks, want 1", got)
	}
	if n := rh.count("ip -6 route show default"); n != 1 {
		t.Errorf("the IPv6 host default was read %d times in 3 ticks, want once", n)
	}
	if n := rh.count("ip -4 route show default"); n != 0 {
		t.Errorf("the IPv4 host default was read %d times for an IPv6 default route", n)
	}
}

// A default route that goes away and comes back warns again.
func TestHostDefaultWarnsAgainAfterDropAndReAdd(t *testing.T) {
	warnings := func() float64 { return testutil.ToFloat64(kernelWarnings.WithLabelValues("host-default")) }
	rh := convergedHost()
	rh.add("default", "192.168.1.1", "eth0")
	ls := &logSink{}
	d := newRouteDaemon(t, rh, ls, newFakeKernel())
	peers := []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "0.0.0.0/0")}
	before := warnings()

	ticks(t, d, rh, peers, 3)
	if got := warnings() - before; got != 1 {
		t.Fatalf("host-default counted %v times before the drop, want 1", got)
	}
	ticks(t, d, rh, nil, 2)
	if rh.has("0.0.0.0/0", "10.99.0.2", "wg0") {
		t.Fatal("the mesh default route was not removed")
	}
	ticks(t, d, rh, peers, 3)
	if got := warnings() - before; got != 2 {
		t.Errorf("host-default counted %v times after the drop and the re-add, want 2", got)
	}
	if n := ls.count("host-default: this host already has an IPv4 default route"); n != 2 {
		t.Errorf("host-default logged %d times, want 2; log: %q", n, ls.lines)
	}
}

// `ip route replace default via <mesh gw> dev wg0` replaces the host's own
// default route (same destination and metric, whatever the device), so when
// the mesh default route is removed the daemon puts back the host default it
// saw before it installed the mesh one: once, logged, counted as a route
// change, per family.
func TestMeshDefaultRemovalRestoresHostDefault(t *testing.T) {
	routeChanges := func() float64 { return testutil.ToFloat64(kernelChanges.WithLabelValues(targetRoutes)) }
	cases := []struct {
		name         string
		hostGW       string // "" a default route without a gateway
		hostDev      string
		network      string // the mesh default
		meshGW       string
		meshIPv6     string
		del, restore string
	}{
		{"IPv4 with a gateway", "192.168.1.1", "eth0", "0.0.0.0/0", "10.99.0.2", "",
			"ip route del 0.0.0.0/0 via 10.99.0.2 dev wg0", "ip route replace default via 192.168.1.1 dev eth0"},
		{"IPv4 without a gateway", "", "ppp0", "0.0.0.0/0", "10.99.0.2", "",
			"ip route del 0.0.0.0/0 via 10.99.0.2 dev wg0", "ip route replace default dev ppp0"},
		{"IPv6", "fe80::1", "eth0", "::/0", "fd99::2", "fd99::2",
			"ip route del ::/0 via fd99::2 dev wg0", "ip -6 route replace default via fe80::1 dev eth0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rh := convergedHost()
			rh.add("default", c.hostGW, c.hostDev)
			ls := &logSink{}
			d := newRouteDaemon(t, rh, ls, newFakeKernel())
			peer := testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", c.network)
			peer.MeshIPv6 = c.meshIPv6
			ticks(t, d, rh, []*PeerInfo{peer}, 2)
			if !rh.has(c.network, c.meshGW, "wg0") || rh.has(c.network, c.hostGW, c.hostDev) {
				t.Fatalf("the mesh default did not replace the host default: mesh %v, host %v", rh.has(c.network, c.meshGW, "wg0"), rh.has(c.network, c.hostGW, c.hostDev))
			}
			before := routeChanges()
			got := ticks(t, d, rh, nil, 3)
			if want := []string{c.del, c.restore}; !sameLines(got[0], want) {
				t.Errorf("removal tick writes %q, want %q", got[0], want)
			}
			if len(got[1]) != 0 || len(got[2]) != 0 {
				t.Errorf("later ticks wrote %q %q, want the restore once", got[1], got[2])
			}
			if !rh.has(c.network, c.hostGW, c.hostDev) {
				t.Errorf("the host default (via %q dev %s) is not back; table after: mesh %v", c.hostGW, c.hostDev, rh.has(c.network, c.meshGW, "wg0"))
			}
			if n := ls.count("restored the host's"); n != 1 {
				t.Errorf("the restore was logged %d times, want once: %q", n, ls.lines)
			}
			if got := routeChanges() - before; got != 2 {
				t.Errorf("route changes grew by %v, want 2 (the removal and the restore)", got)
			}
		})
	}

	t.Run("no host default, nothing restored", func(t *testing.T) {
		rh := convergedHost()
		d := newRouteDaemon(t, rh, &logSink{}, newFakeKernel())
		peer := testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "0.0.0.0/0")
		ticks(t, d, rh, []*PeerInfo{peer}, 2)
		got := ticks(t, d, rh, nil, 2)
		if want := []string{"ip route del 0.0.0.0/0 via 10.99.0.2 dev wg0"}; !sameLines(got[0], want) || len(got[1]) != 0 {
			t.Errorf("writes %q %q, want only the removal", got[0], got[1])
		}
	})

	// The record is taken from a read made while the mesh default is not
	// installed, never from the mesh default itself: the mesh gateway changes
	// (another peer owns the default now) and the read that would see only
	// wg0's default is not made; the restore still brings the host's back.
	t.Run("the record survives a gateway change of the mesh default", func(t *testing.T) {
		rh := convergedHost()
		rh.add("default", "192.168.1.1", "eth0")
		d := newRouteDaemon(t, rh, &logSink{}, newFakeKernel())
		ticks(t, d, rh, []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "0.0.0.0/0")}, 2)
		d.convergeMu.Lock()
		d.hostDefaultWarned = nil // the read is allowed again
		d.convergeMu.Unlock()
		ticks(t, d, rh, []*PeerInfo{testPeer("keyB", "10.99.0.3", "203.0.113.3:51820", "0.0.0.0/0")}, 2)
		if !rh.has("0.0.0.0/0", "10.99.0.3", "wg0") {
			t.Fatalf("the mesh default did not move to keyB: %q", rh.routeWrites())
		}
		ticks(t, d, rh, nil, 1)
		if !rh.has("0.0.0.0/0", "192.168.1.1", "eth0") {
			t.Errorf("the host default is not back after the mesh default moved and went: %q", rh.routeWrites())
		}
	})
}

const (
	attachedAddrOut = "2: dum0    inet 192.168.50.1/24 brd 192.168.50.255 scope global dum0\n       valid_lft forever preferred_lft forever\n"
	restoreAttached = "ip route add 192.168.50.0/24 dev dum0 proto kernel scope link src 192.168.50.1"
)

// `ip route replace <attached net> via <gw>` overwrites the host's connected
// route for that network, so the removal of such a route restores it.
func TestAttachedNetworkWarningNamesTheOverwrite(t *testing.T) {
	enforceRouteGuard(t, false)
	d, rh, _, ls, peers, _ := attachedFixture(t)
	ticks(t, d, rh, peers, 3)
	if n := ls.count("replaces the host's own connected route"); n != 2 {
		t.Errorf("the attached-network warning names the overwritten connected route %d times, want 2 (equal and inside, once each); log: %q", n, ls.lines)
	}
}

func TestRemovedAttachedRouteRestoresConnectedRoute(t *testing.T) {
	check := func(t *testing.T, rh *routeHost, ls *logSink) {
		t.Helper()
		if n := rh.count(restoreAttached); n != 1 {
			t.Errorf("the connected route of 192.168.50.0/24 was restored %d times, want once; commands: %q", n, rh.log)
		}
		for _, other := range []string{"192.168.0.0/16", "172.16.0.0/16"} {
			if n := rh.count("ip -4 -o addr show to " + other); n != 0 {
				t.Errorf("a route %s that is not attached was looked up for a restore", other)
			}
		}
		if n := rh.count("ip route add"); n != 1 {
			t.Errorf("%d routes were added back, want only the connected route of the attached network: %q", n, rh.log)
		}
	}

	t.Run("peer gone", func(t *testing.T) {
		enforceRouteGuard(t, false)
		d, rh, _, ls, peers, _ := attachedFixture(t)
		rh.addrs["192.168.50.0/24"] = attachedAddrOut
		ticks(t, d, rh, peers, 2)
		ticks(t, d, rh, nil, 2)
		check(t, rh, ls)
	})

	t.Run("guard flipped", func(t *testing.T) {
		enforceRouteGuard(t, false)
		d, rh, _, ls, peers, _ := attachedFixture(t)
		rh.addrs["192.168.50.0/24"] = attachedAddrOut
		ticks(t, d, rh, peers, 2)
		EnforceRouteGuard.Store(true)
		ticks(t, d, rh, peers, 2)
		check(t, rh, ls)
	})
}

func TestRemovedPlainRouteIsNotRestored(t *testing.T) {
	enforceRouteGuard(t, false)
	rh := convergedHost()
	ls := &logSink{}
	d := newRouteDaemon(t, rh, ls, newFakeKernel())
	d.localSubnetsFn = func() []*net.IPNet {
		return []*net.IPNet{{IP: net.IPv4(192, 168, 50, 1), Mask: net.CIDRMask(24, 32)}}
	}
	peers := []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "192.168.7.0/24")}
	ticks(t, d, rh, peers, 2)
	got := ticks(t, d, rh, nil, 2)
	if want := []string{"ip route del 192.168.7.0/24 via 10.99.0.2 dev wg0"}; !sameLines(got[0], want) {
		t.Fatalf("removal writes = %q, want %q", got[0], want)
	}
	if n := rh.count("ip -4 -o addr show") + rh.count("ip route add"); n != 0 {
		t.Errorf("a route that is neither own nor attached triggered a restore: %q", rh.log)
	}
	if n := ls.count("no local address") + ls.count("connected route"); n != 0 {
		t.Errorf("a plain removal logged restore noise: %q", ls.lines)
	}
}

// The kernel prints a gateway compressed and in lower case; a peer announces
// its mesh IPv6 in any form. A non-canonical announcement must not make every
// tick delete and replace the route.
func TestIPv6GatewayNormalisedNoChurn(t *testing.T) {
	for _, announced := range []string{"fd12:3456:789a:0001::1", "FD12:3456:789A:1::1"} {
		t.Run(announced, func(t *testing.T) {
			rh := convergedHost()
			d := newRouteDaemon(t, rh, &logSink{}, newFakeKernel())
			peer := testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "fd00:5::/64")
			peer.MeshIPv6 = announced

			got := ticks(t, d, rh, []*PeerInfo{peer}, 3)
			if want := []string{replaceCmd("fd00:5::/64", "fd12:3456:789a:1::1")}; !sameLines(got[0], want) {
				t.Fatalf("tick 1 route writes = %q, want %q", got[0], want)
			}
			if len(got[1]) != 0 || len(got[2]) != 0 {
				t.Errorf("a non-canonical gateway churned: tick 2 %q, tick 3 %q", got[1], got[2])
			}
		})
	}
}

// The mesh interface carries the node's IPv6 /64 and a link-local /64 among
// the local subnets: a network inside either is the mesh's or the link's, not
// an attached one. A real attached IPv6 subnet still counts.
func TestAttachedRuleIgnoresMeshIPv6AndLinkLocal(t *testing.T) {
	setup := func(t *testing.T) (*Daemon, *routeHost, *fakeKernel, *logSink, []*PeerInfo) {
		rh := convergedHost()
		ls := &logSink{}
		k := newFakeKernel()
		d := newRouteDaemon(t, rh, ls, k)
		d.localNode.MeshIPv6 = "fd99:0:0:1::1"
		d.localSubnetsFn = func() []*net.IPNet {
			return []*net.IPNet{
				{IP: net.ParseIP("fd99:0:0:1::1"), Mask: net.CIDRMask(64, 128)},
				{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
				{IP: net.ParseIP("2001:db8:7::1"), Mask: net.CIDRMask(64, 128)},
			}
		}
		peer := testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "fd99:0:0:1:7::/80", "fe80::/64", "2001:db8:7::/64")
		peer.MeshIPv6 = "fd99:0:0:1::2"
		return d, rh, k, ls, []*PeerInfo{peer}
	}
	warnings := func() float64 { return testutil.ToFloat64(kernelWarnings.WithLabelValues("attached-network")) }

	t.Run("N counts only the real attached subnet", func(t *testing.T) {
		enforceRouteGuard(t, false)
		d, rh, _, ls, peers := setup(t)
		before := warnings()
		ticks(t, d, rh, peers, 3)
		if got := warnings() - before; got != 1 {
			t.Errorf("attached-network counted %v times, want 1 (2001:db8:7::/64 only); log: %q", got, ls.lines)
		}
		if n := ls.count("attached-network: fd99:") + ls.count("attached-network: fe80:"); n != 0 {
			t.Errorf("a mesh or link-local network was called attached: %q", ls.lines)
		}
	})

	t.Run("N+1 keeps the mesh and link-local networks", func(t *testing.T) {
		enforceRouteGuard(t, true)
		d, rh, k, _, peers := setup(t)
		ticks(t, d, rh, peers, 3)
		for _, n := range []string{"fd99:0:0:1:7::/80", "fe80::/64"} {
			if !rh.has(n, "fd99:0:0:1::2", "wg0") || !contains(kernelAllowed(k, "keyA"), n) {
				t.Errorf("with EnforceRouteGuard %s must stay (route %v, AllowedIPs %v)", n, rh.has(n, "fd99:0:0:1::2", "wg0"), kernelAllowed(k, "keyA"))
			}
		}
		if rh.has("2001:db8:7::/64", "fd99:0:0:1::2", "wg0") || contains(kernelAllowed(k, "keyA"), "2001:db8:7::/64") {
			t.Error("with EnforceRouteGuard the attached IPv6 subnet is still installed")
		}
	})
}

// `ip route replace` swaps a route atomically, so a gateway change is one
// replace and no del: a failed replace leaves the old route in the table. A
// network that is no longer desired is still deleted.
func TestGatewayChangeIsOneReplaceNoDel(t *testing.T) {
	peers := []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "192.168.7.0/24")}
	newHost := func() *routeHost {
		rh := convergedHost()
		rh.add("192.168.7.0/24", "10.99.0.9", "wg0")
		rh.add("192.168.8.0/24", "10.99.0.9", "wg0")
		return rh
	}

	t.Run("happy path", func(t *testing.T) {
		rh := newHost()
		d := newRouteDaemon(t, rh, &logSink{}, newFakeKernel())
		got := ticks(t, d, rh, peers, 2)
		want := []string{"ip route del 192.168.8.0/24 via 10.99.0.9 dev wg0", replaceCmd("192.168.7.0/24", "10.99.0.2")}
		if !sameLines(got[0], want) || len(got[1]) != 0 {
			t.Errorf("route writes per tick = %q, want one replace for the changed gateway and one del for the stale network on tick 1, nothing on tick 2", got)
		}
		if !rh.has("192.168.7.0/24", "10.99.0.2", "wg0") || rh.has("192.168.7.0/24", "10.99.0.9", "wg0") {
			t.Error("the route did not move to the new gateway")
		}
	})

	t.Run("failed replace keeps the old route", func(t *testing.T) {
		rh := newHost()
		rh.replaceErr["192.168.7.0/24"] = errors.New("exit status 2")
		d := newRouteDaemon(t, rh, &logSink{}, newFakeKernel())
		ticks(t, d, rh, peers, 1)
		if !rh.has("192.168.7.0/24", "10.99.0.9", "wg0") {
			t.Errorf("the failed replace left the network without a route: %q", rh.routeWrites())
		}
		if n := rh.count("ip route del 192.168.7.0/24"); n != 0 {
			t.Errorf("the old route was deleted before its replacement: %q", rh.routeWrites())
		}
		if rh.has("192.168.8.0/24", "10.99.0.9", "wg0") {
			t.Error("a network that is no longer desired was not removed")
		}
	})
}

// desire is pure: the warn-once set is part of its result and only converge
// keeps it. A desire that reconnect runs between two ticks must not make the
// next tick warn again.
func TestReconnectDoesNotResetWarnOnce(t *testing.T) {
	enforceRouteGuard(t, false)
	d, rh, _, ls, peers, _ := attachedFixture(t)
	before := testutil.ToFloat64(kernelWarnings.WithLabelValues("attached-network"))

	ticks(t, d, rh, peers, 1)
	if err := d.reconnectPeerInKernel(peers[0], nil); err != nil { // a desire over no peers
		t.Fatalf("reconnect: %v", err)
	}
	ticks(t, d, rh, peers, 2)

	if got := testutil.ToFloat64(kernelWarnings.WithLabelValues("attached-network")) - before; got != 2 {
		t.Errorf("attached-network counted %v times across a reconnect, want 2 (once each)", got)
	}
	if n := ls.count("attached-network"); n != 2 {
		t.Errorf("attached-network logged %d times across a reconnect, want 2; log: %q", n, ls.lines)
	}
}

// A host default that cannot be read is logged once per distinct error while
// the add keeps being tried, and the recovery once.
func TestHostDefaultReadErrorLoggedOnce(t *testing.T) {
	rh := convergedHost()
	rh.replaceErr["0.0.0.0/0"] = errors.New("exit status 2") // the add is tried every tick
	rh.readErr["ip -4 route show default"] = errors.New("RTNETLINK answers: Operation not supported")
	ls := &logSink{}
	d := newRouteDaemon(t, rh, ls, newFakeKernel())
	peers := []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "0.0.0.0/0")}

	ticks(t, d, rh, peers, 5)
	if n := ls.count("Cannot read the host's IPv4 default route"); n != 1 {
		t.Errorf("the failed host default read was logged %d times in 5 ticks, want 1; log: %q", n, ls.lines)
	}
	delete(rh.readErr, "ip -4 route show default")
	ticks(t, d, rh, peers, 3)
	if n := ls.count("Cannot read the host's IPv4 default route"); n != 1 {
		t.Errorf("the failed read was logged again after it worked: %d; log: %q", n, ls.lines)
	}
	if n := ls.count("Reading the host's IPv4 default route works again"); n != 1 {
		t.Errorf("the recovery was logged %d times, want 1; log: %q", n, ls.lines)
	}
	rh.readErr["ip -4 route show default"] = errors.New("a different failure")
	ticks(t, d, rh, peers, 2)
	if n := ls.count("Cannot read the host's IPv4 default route"); n != 2 {
		t.Errorf("a different failure was logged %d times in total, want 2; log: %q", n, ls.lines)
	}
}

// With --no-ipv6 the mesh interface has no IPv6: an advertised IPv6 network is
// left out of the AllowedIPs and of the routes, with one log line.
func TestDisableIPv6SkipsIPv6Networks(t *testing.T) {
	rh := convergedHost()
	ls := &logSink{}
	k := newFakeKernel()
	d := newRouteDaemon(t, rh, ls, k)
	d.config.DisableIPv6 = true
	peer := testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "fd00:5::/64", "192.168.7.0/24")
	peer.MeshIPv6 = "fd99::2"

	got := ticks(t, d, rh, []*PeerInfo{peer}, 5)
	if want := []string{replaceCmd("192.168.7.0/24", "10.99.0.2")}; !sameLines(got[0], want) {
		t.Errorf("tick 1 route writes = %q, want only the IPv4 route %q", got[0], want)
	}
	for _, c := range rh.log {
		if strings.Contains(c, "fd00:5::") {
			t.Errorf("the IPv6 network was issued with IPv6 disabled: %q", c)
		}
	}
	if allowed := kernelAllowed(k, "keyA"); contains(allowed, "fd00:5::/64") {
		t.Errorf("the IPv6 network is in the peer's AllowedIPs with IPv6 disabled: %v", allowed)
	}
	if n := ls.count("IPv6 is disabled"); n != 1 {
		t.Errorf("the skipped IPv6 network was logged %d times in 5 ticks, want 1; log: %q", n, ls.lines)
	}
}
