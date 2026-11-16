package daemon

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/atvirokodosprendimai/wgmesh/pkg/routes"
)

// The fixtures under testdata/darwin are HAND-WRITTEN in the shape of the
// netstat man page, not recorded on a Mac (step S1 of ADR-0003 T6 replaces
// them with real output).

// darwinRoute is one row of the fake macOS table: a canonical network and the
// columns netstat prints for it.
type darwinRoute struct{ net, gw, flags, netif string }

// darwinHost is the routing table of one macOS node, reached through
// MockCommandExecutor: `netstat -rn -f inet|inet6` prints it the way netstat
// does (abbreviated destinations), `route -n add|change|delete` change it
// the way route(8) takes them.
type darwinHost struct {
	mu      sync.Mutex
	table   []darwinRoute
	hidden  map[string]bool  // canonical networks that are in the table but not printed
	noise   map[int][]string // family -> raw rows printed as they are
	log     []string
	readErr map[string]error // full command line -> what reading fails with
	addErr  map[string]error // full command line -> what `route add` fails with ("File exists")
	hold    func()           // called at the start of every command, outside the lock
}

func newDarwinHost() *darwinHost {
	return &darwinHost{hidden: map[string]bool{}, noise: map[int][]string{}, readErr: map[string]error{}, addErr: map[string]error{}}
}

// printDarwin is how netstat prints a canonical network: `default`, a host
// without its prefix, an IPv4 network with the octets it needs (`192.168.5/24`).
func printDarwin(canon string) string {
	p := netip.MustParsePrefix(canon)
	switch {
	case p.Bits() == 0:
		return "default"
	case p.Bits() == p.Addr().BitLen():
		return p.Addr().String()
	case p.Addr().Is4():
		octets := strings.Split(p.Addr().String(), ".")
		return strings.Join(octets[:(p.Bits()+7)/8], ".") + fmt.Sprintf("/%d", p.Bits())
	}
	return p.String()
}

func (h *darwinHost) add(canon, netif string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.table = append(h.table, darwinRoute{canon, "link#16", "UCS", netif})
}

func (h *darwinHost) has(canon, netif string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.table {
		if r.net == canon && r.netif == netif {
			return true
		}
	}
	return false
}

func (h *darwinHost) command(name string, args ...string) Command {
	return &MockCommand{
		runFunc:            func() error { _, err := h.exec(name, args...); return err },
		combinedOutputFunc: func() ([]byte, error) { return h.exec(name, args...) },
		outputFunc:         func() ([]byte, error) { return h.exec(name, args...) },
	}
}

func (h *darwinHost) exec(name string, args ...string) ([]byte, error) {
	if h.hold != nil {
		h.hold()
	}
	line := name + " " + strings.Join(args, " ")
	h.mu.Lock()
	defer h.mu.Unlock()
	h.log = append(h.log, line)
	switch name {
	case "netstat":
		if len(args) != 3 || args[0] != "-rn" || args[1] != "-f" || (args[2] != "inet" && args[2] != "inet6") {
			return nil, fmt.Errorf("fake host: unexpected command %q", line)
		}
		if err := h.readErr[line]; err != nil {
			return nil, err
		}
		v6 := args[2] == "inet6"
		family := 4
		if v6 {
			family = 6
		}
		var out strings.Builder
		out.WriteString("Routing tables\n\nInternet:\nDestination        Gateway            Flags               Netif Expire\n")
		for _, r := range h.table {
			if strings.Contains(r.net, ":") == v6 && !h.hidden[r.net] {
				fmt.Fprintf(&out, "%-18s %-18s %-18s %8s\n", printDarwin(r.net), r.gw, r.flags, r.netif)
			}
		}
		for _, l := range h.noise[family] {
			out.WriteString(l + "\n")
		}
		return []byte(out.String()), nil
	case "route":
		if len(args) < 5 || args[0] != "-n" {
			return nil, fmt.Errorf("fake host: unexpected command %q", line)
		}
		verb, rest := args[1], args[2:]
		if rest[0] == "-inet6" {
			rest = rest[1:]
		}
		if len(rest) != 4 || rest[0] != "-net" || rest[2] != "-interface" {
			return nil, fmt.Errorf("fake host: unexpected command %q", line)
		}
		canon, netif := rest[1], rest[3]
		if _, err := netip.ParsePrefix(canon); err != nil || netip.MustParsePrefix(canon).String() != canon {
			return nil, fmt.Errorf("fake host: %q: not a canonical network", line)
		}
		idx := -1
		for i, r := range h.table {
			if r.net == canon && r.netif == netif {
				idx = i
			}
		}
		switch verb {
		case "add":
			if err := h.addErr[line]; err != nil {
				return []byte("route: writing to routing socket: File exists"), err
			}
			if idx >= 0 {
				return []byte("route: writing to routing socket: File exists"), exitError{1}
			}
			h.table = append(h.table, darwinRoute{canon, "link#16", "UCS", netif})
			return []byte("add net " + canon + ": gateway " + netif), nil
		case "change":
			if idx < 0 {
				return []byte("route: writing to routing socket: not in table"), exitError{1}
			}
			return []byte("change net " + canon), nil
		case "delete":
			if idx < 0 {
				return []byte("route: writing to routing socket: not in table"), exitError{1}
			}
			h.table = append(h.table[:idx], h.table[idx+1:]...)
			return []byte("delete net " + canon), nil
		}
	}
	return nil, fmt.Errorf("fake host: unexpected command %q", line)
}

// routeCommands are the commands that write routes, in the order they ran.
func (h *darwinHost) routeCommands() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []string
	for _, c := range h.log {
		if strings.HasPrefix(c, "route -n ") {
			out = append(out, c)
		}
	}
	return out
}

func (h *darwinHost) count(prefix string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.log {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

// newDarwinDaemon is a macOS daemon on mesh interface utun4 (mesh 10.99.0.0/16
// and fd99::/64) whose commands go to dh. The tests that use it swap the
// package executor and so never run in parallel.
func newDarwinDaemon(t *testing.T, dh *darwinHost, ls *logSink, k *fakeKernel) *Daemon {
	t.Helper()
	d := newConvergeDaemon(t, k, ls)
	d.goos = "darwin"
	d.config.InterfaceName = "utun4"
	_, subnet, _ := net.ParseCIDR("10.99.0.0/16")
	d.config.CustomSubnet = subnet
	d.localNode.MeshIPv6 = "fd99::1"
	old := cmdExecutor
	cmdExecutor = &MockCommandExecutor{commandFunc: dh.command}
	t.Cleanup(func() { cmdExecutor = old })
	return d
}

func darwinPeer(key, meshIP, meshIPv6 string, networks ...string) *PeerInfo {
	p := testPeer(key, meshIP, "203.0.113.2:51820", networks...)
	p.MeshIPv6 = meshIPv6
	return p
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	out, err := os.ReadFile("testdata/darwin/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func networksOf(entries []routes.Entry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.Network)
	}
	sort.Strings(out)
	return out
}

func TestParseNetstatDarwin(t *testing.T) {
	cases := []struct {
		name, file string
		ipv6       bool
		iface      string
		want       []string
	}{
		{"inet on utun4", "netstat-inet.handwritten.txt", false, "utun4",
			[]string{"0.0.0.0/0", "10.100.0.5/32", "10.99.0.0/16", "128.0.0.0/1", "192.168.5.0/24"}},
		{"inet6 on utun4", "netstat-inet6.handwritten.txt", true, "utun4",
			[]string{"::/0", "fd00:5::/48", "fd00:7::9/128", "fd99::/64"}},
		{"inet on en0 is not the mesh interface", "netstat-inet.handwritten.txt", false, "utun9", nil},
		{"inet on en0", "netstat-inet.handwritten.txt", false, "en0",
			[]string{"169.254.0.0/16", "172.20.1.0/24", "192.168.1.0/24", "192.168.1.1/32", "255.255.255.255/32"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseNetstatDarwin(readFixture(t, c.file), c.iface, c.ipv6)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range got {
				if e.Gateway != "" {
					t.Errorf("route %s carries gateway %q, macOS routes have none", e.Network, e.Gateway)
				}
			}
			if !reflect.DeepEqual(networksOf(got), c.want) {
				t.Errorf("networks = %q, want %q", networksOf(got), c.want)
			}
		})
	}

	t.Run("a destination that does not parse is an error, not a skipped row", func(t *testing.T) {
		out := []byte("Internet:\nDestination Gateway Flags Netif Expire\nnot-a-network link#16 UCS utun4\n")
		if got, err := parseNetstatDarwin(out, "utun4", false); err == nil {
			t.Errorf("parsed %v, want an error", got)
		}
	})
	t.Run("a row of another interface is not parsed at all", func(t *testing.T) {
		out := []byte("Internet:\nDestination Gateway Flags Netif Expire\nnot-a-network link#14 UCS en0\n")
		if got, err := parseNetstatDarwin(out, "utun4", false); err != nil || len(got) != 0 {
			t.Errorf("got %v, %v; want nothing", got, err)
		}
	})
}

func TestDarwinRouteAddUsesInterface(t *testing.T) {
	dh := newDarwinHost()
	ls := &logSink{}
	k := newFakeKernel()
	d := newDarwinDaemon(t, dh, ls, k)
	peers := []*PeerInfo{darwinPeer("keyA", "10.99.0.2", "", "192.168.50.0/24", "0.0.0.0/0")}

	if d.converge(peers) == nil {
		t.Fatal("converge skipped the tick")
	}
	got := dh.routeCommands()
	want := []string{
		"route -n add -net 0.0.0.0/0 -interface utun4",
		"route -n add -net 192.168.50.0/24 -interface utun4",
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("route commands = %q, want %q", got, want)
	}
	for _, c := range got {
		if strings.Contains(c, " via ") || strings.Contains(c, "-gateway") {
			t.Errorf("%q names a gateway; WireGuard picks the peer by AllowedIPs", c)
		}
	}
	if n := dh.count("ip "); n != 0 {
		t.Errorf("%d ip commands ran on macOS", n)
	}
	if !contains(kernelAllowed(k, "keyA"), "192.168.50.0/24") {
		t.Errorf("AllowedIPs %v lack the route", kernelAllowed(k, "keyA"))
	}
	before := len(dh.routeCommands())
	if d.converge(peers) == nil {
		t.Fatal("converge skipped the second tick")
	}
	if after := dh.routeCommands(); len(after) != before {
		t.Errorf("a second tick wrote %q", after[before:])
	}
}

func TestDarwinRouteDeleteUsesInterface(t *testing.T) {
	dh := newDarwinHost()
	dh.add("172.20.0.0/16", "utun4")   // ours, no longer wanted
	dh.add("172.21.0.0/16", "en0")     // another interface: never touched
	dh.add("192.168.50.0/24", "utun4") // wanted: stays
	ls := &logSink{}
	d := newDarwinDaemon(t, dh, ls, newFakeKernel())
	peers := []*PeerInfo{darwinPeer("keyA", "10.99.0.2", "", "192.168.50.0/24")}

	if d.converge(peers) == nil {
		t.Fatal("converge skipped the tick")
	}
	got := dh.routeCommands()
	want := []string{"route -n delete -net 172.20.0.0/16 -interface utun4"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("route commands = %q, want %q", got, want)
	}
	if dh.has("172.20.0.0/16", "utun4") {
		t.Error("the stale route is still in the table")
	}
	if !dh.has("172.21.0.0/16", "en0") || !dh.has("192.168.50.0/24", "utun4") {
		t.Error("a route that must stay was removed")
	}
}

func TestDarwinAttachedNetworkSkippedFromFirstRelease(t *testing.T) {
	enforceRouteGuard(t, false)
	warnings := func() float64 { return testutil.ToFloat64(kernelWarnings.WithLabelValues("attached-network")) }
	dh := newDarwinHost()
	ls := &logSink{}
	k := newFakeKernel()
	d := newDarwinDaemon(t, dh, ls, k)
	d.localSubnetsFn = func() []*net.IPNet {
		return []*net.IPNet{
			{IP: net.IPv4(192, 168, 50, 1), Mask: net.CIDRMask(24, 32)},
			{IP: net.IPv4(10, 99, 0, 1), Mask: net.CIDRMask(16, 32)},
		}
	}
	peers := []*PeerInfo{darwinPeer("keyA", "10.99.0.2", "", "192.168.50.0/24", "192.168.50.128/25", "192.168.0.0/16", "10.50.0.0/16")}
	before := warnings()
	for i := 0; i < 3; i++ {
		if d.converge(peers) == nil {
			t.Fatalf("tick %d skipped", i+1)
		}
	}
	for _, n := range []string{"192.168.50.0/24", "192.168.50.128/25"} {
		if dh.has(n, "utun4") {
			t.Errorf("the attached network %s is routed into the tunnel", n)
		}
		if contains(kernelAllowed(k, "keyA"), n) {
			t.Errorf("the attached network %s is in AllowedIPs %v", n, kernelAllowed(k, "keyA"))
		}
	}
	for _, n := range []string{"192.168.0.0/16", "10.50.0.0/16"} {
		if !dh.has(n, "utun4") || !contains(kernelAllowed(k, "keyA"), n) {
			t.Errorf("%s must stay (route %v, AllowedIPs %v)", n, dh.has(n, "utun4"), kernelAllowed(k, "keyA"))
		}
	}
	if got := warnings() - before; got != 2 {
		t.Errorf("attached-network counted %v times over 3 ticks, want 2 (equal and inside, once each)", got)
	}
	if got := ls.count("attached-network"); got != 2 {
		t.Errorf("attached-network logged %d times over 3 ticks, want 2; log %q", got, ls.lines)
	}
}

func TestDarwinV6RouteUsesInet6(t *testing.T) {
	dh := newDarwinHost()
	dh.add("fd00:6::/64", "utun4") // ours, no longer wanted
	d := newDarwinDaemon(t, dh, &logSink{}, newFakeKernel())
	peers := []*PeerInfo{darwinPeer("keyA", "10.99.0.2", "fd99::2", "fd00:5::/48", "192.168.50.0/24")}

	if d.converge(peers) == nil {
		t.Fatal("converge skipped the tick")
	}
	got := dh.routeCommands()
	sort.Strings(got)
	want := []string{
		"route -n add -inet6 -net fd00:5::/48 -interface utun4",
		"route -n add -net 192.168.50.0/24 -interface utun4",
		"route -n delete -inet6 -net fd00:6::/64 -interface utun4",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("route commands = %q, want %q", got, want)
	}
	if n := dh.count("netstat -rn -f inet6"); n != 1 {
		t.Errorf("the IPv6 table was read %d times, want 1", n)
	}
}

func TestSyncRoutesRunsOnDarwin(t *testing.T) {
	peers := []*PeerInfo{darwinPeer("keyA", "10.99.0.2", "", "192.168.50.0/24")}
	t.Run("darwin reads and writes the table", func(t *testing.T) {
		dh := newDarwinHost()
		d := newDarwinDaemon(t, dh, &logSink{}, newFakeKernel())
		if d.converge(peers) == nil {
			t.Fatal("converge skipped the tick")
		}
		if dh.count("netstat -rn -f inet ") != 1 || dh.count("netstat -rn -f inet6") != 1 {
			t.Errorf("the table was not read once per family: %q", dh.log)
		}
		if !dh.has("192.168.50.0/24", "utun4") {
			t.Errorf("the peer's network is not routed; commands %q", dh.log)
		}
	})
	t.Run("another OS touches no route", func(t *testing.T) {
		dh := newDarwinHost()
		d := newDarwinDaemon(t, dh, &logSink{}, newFakeKernel())
		d.goos = "plan9"
		if d.converge(peers) == nil {
			t.Fatal("converge skipped the tick")
		}
		if len(dh.log) != 0 {
			t.Errorf("commands ran on an unsupported OS: %q", dh.log)
		}
	})
}

func TestDarwinFileExistsBecomesChange(t *testing.T) {
	// A route the table read does not show (hidden) makes the add answer
	// File exists, and the daemon changes the route instead.
	t.Run("IPv4", func(t *testing.T) {
		dh := newDarwinHost()
		dh.add("192.168.50.0/24", "utun4")
		dh.hidden["192.168.50.0/24"] = true
		ls := &logSink{}
		d := newDarwinDaemon(t, dh, ls, newFakeKernel())
		d.converge([]*PeerInfo{darwinPeer("keyA", "10.99.0.2", "", "192.168.50.0/24")})
		got := dh.routeCommands()
		want := []string{
			"route -n add -net 192.168.50.0/24 -interface utun4",
			"route -n change -net 192.168.50.0/24 -interface utun4",
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("route commands = %q, want %q", got, want)
		}
		if n := ls.count("Failed to"); n != 0 {
			t.Errorf("File exists logged as a failure: %q", ls.lines)
		}
	})
	t.Run("IPv6", func(t *testing.T) {
		dh := newDarwinHost()
		dh.add("fd00:5::/48", "utun4")
		dh.hidden["fd00:5::/48"] = true
		d := newDarwinDaemon(t, dh, &logSink{}, newFakeKernel())
		d.converge([]*PeerInfo{darwinPeer("keyA", "10.99.0.2", "fd99::2", "fd00:5::/48")})
		got := dh.routeCommands()
		want := []string{
			"route -n add -inet6 -net fd00:5::/48 -interface utun4",
			"route -n change -inet6 -net fd00:5::/48 -interface utun4",
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("route commands = %q, want %q", got, want)
		}
	})
	t.Run("a failing change is a failure of that route only", func(t *testing.T) {
		dh := newDarwinHost()
		// add says File exists, but the route is not in the table, so the
		// change is refused too.
		dh.addErr["route -n add -net 192.168.50.0/24 -interface utun4"] = exitError{1}
		ls := &logSink{}
		d := newDarwinDaemon(t, dh, ls, newFakeKernel())
		d.converge([]*PeerInfo{darwinPeer("keyA", "10.99.0.2", "", "192.168.50.0/24", "10.50.0.0/16")})
		if !dh.has("10.50.0.0/16", "utun4") {
			t.Error("the other route was skipped after one failed")
		}
		if n := ls.count("Failed to change route"); n != 1 {
			t.Errorf("failed change logged %d times, want 1: %q", n, ls.lines)
		}
	})
}

func TestDarwinPlanLeavesMeshSubnetAlone(t *testing.T) {
	keep := []netip.Prefix{netip.MustParsePrefix("10.99.0.0/16"), netip.MustParsePrefix("fd99::/64")}
	entry := func(n string) routes.Entry { return routes.Entry{Network: n} }
	obs := kernelObserved{
		Routes4: []routes.Entry{entry("10.99.0.0/16"), entry("172.20.0.0/16"), entry("192.168.50.0/24")},
		Routes6: []routes.Entry{entry("fd99::/64"), entry("fd00:6::/64")},
	}
	want := []routes.Entry{entry("192.168.50.0/24"), entry("10.50.0.0/16"), entry("fd00:5::/48")}
	plan := planRoutesDarwin(want, obs, keep)
	if got, w := networksOf(plan.Add), []string{"10.50.0.0/16", "fd00:5::/48"}; !reflect.DeepEqual(got, w) {
		t.Errorf("add = %q, want %q", got, w)
	}
	if got, w := networksOf(plan.Remove), []string{"172.20.0.0/16", "fd00:6::/64"}; !reflect.DeepEqual(got, w) {
		t.Errorf("remove = %q, want %q", got, w)
	}

	t.Run("a family whose read failed plans nothing, removes nothing", func(t *testing.T) {
		failed := obs
		failed.Errs = map[string]error{targetRoutes4: fmt.Errorf("netstat failed")}
		plan := planRoutesDarwin(want, failed, keep)
		if got, w := networksOf(plan.Remove), []string{"fd00:6::/64"}; !reflect.DeepEqual(got, w) {
			t.Errorf("remove = %q, want %q (IPv4 unread)", got, w)
		}
		if got, w := networksOf(plan.Add), []string{"fd00:5::/48"}; !reflect.DeepEqual(got, w) {
			t.Errorf("add = %q, want %q (IPv4 unread)", got, w)
		}
	})

	t.Run("through converge the mesh routes stay", func(t *testing.T) {
		dh := newDarwinHost()
		dh.add("10.99.0.0/16", "utun4")
		dh.add("fd99::/64", "utun4")
		d := newDarwinDaemon(t, dh, &logSink{}, newFakeKernel())
		d.converge([]*PeerInfo{darwinPeer("keyA", "10.99.0.2", "fd99::2", "192.168.50.0/24")})
		if !dh.has("10.99.0.0/16", "utun4") || !dh.has("fd99::/64", "utun4") {
			t.Errorf("a mesh connected route was removed: %q", dh.routeCommands())
		}
		for _, c := range dh.routeCommands() {
			if strings.HasPrefix(c, "route -n delete") {
				t.Errorf("unexpected delete %q", c)
			}
		}
	})
}

func TestDarwinObserveFailureRemovesNothing(t *testing.T) {
	dh := newDarwinHost()
	dh.add("172.20.0.0/16", "utun4")
	dh.readErr["netstat -rn -f inet"] = fmt.Errorf("netstat: not found")
	ls := &logSink{}
	d := newDarwinDaemon(t, dh, ls, newFakeKernel())
	peers := []*PeerInfo{darwinPeer("keyA", "10.99.0.2", "", "192.168.50.0/24")}
	d.converge(peers)
	d.converge(peers)
	if cmds := dh.routeCommands(); len(cmds) != 0 {
		t.Errorf("IPv4 routes written although the table could not be read: %q", cmds)
	}
	if n := ls.count("Cannot read the IPv4 routes"); n != 1 {
		t.Errorf("read failure logged %d times over 2 ticks, want 1: %q", n, ls.lines)
	}
}

// reconcile runs from the ticker and from handleSIGHUP under reconcileMu, and
// the route target of one tick under convergeMu: two ticks at once never
// overlap in the kernel commands, so the second sees the first's route and
// writes nothing. Run with -race.
func TestDarwinRouteTickRaceFree(t *testing.T) {
	dh := newDarwinHost()
	var inside, maxInside, calls atomic.Int32
	dh.hold = func() {
		n := inside.Add(1)
		defer inside.Add(-1)
		for {
			m := maxInside.Load()
			if n <= m || maxInside.CompareAndSwap(m, n) {
				break
			}
		}
		if calls.Add(1) == 1 {
			time.Sleep(50 * time.Millisecond)
		}
	}
	d := newDarwinDaemon(t, dh, &logSink{}, newFakeKernel())
	d.peerStore.Update(darwinPeer("keyA", "10.99.0.2", "", "192.168.50.0/24"), "test")
	var wg sync.WaitGroup
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.reconcile()
		}()
	}
	wg.Wait()
	if calls.Load() == 0 {
		t.Fatal("no command ran")
	}
	if m := maxInside.Load(); m != 1 {
		t.Errorf("%d commands ran at the same time, want 1", m)
	}
	if got := dh.routeCommands(); !reflect.DeepEqual(got, []string{"route -n add -net 192.168.50.0/24 -interface utun4"}) {
		t.Errorf("route commands = %q, want exactly one add", got)
	}
}
