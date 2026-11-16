package daemon

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"reflect"
	"runtime"
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
	delErr  map[string]error // full command line -> what `route delete` fails with
	hold    func()           // called at the start of every command, outside the lock
}

func newDarwinHost() *darwinHost {
	return &darwinHost{hidden: map[string]bool{}, noise: map[int][]string{}, readErr: map[string]error{}, addErr: map[string]error{}, delErr: map[string]error{}}
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

// addRow puts a row with its own gateway and flags in the table, for a route
// that another interface (or the host) owns.
func (h *darwinHost) addRow(canon, gw, flags, netif string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.table = append(h.table, darwinRoute{canon, gw, flags, netif})
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
			if err := h.delErr[line]; err != nil {
				return []byte("route: writing to routing socket: Operation not permitted"), err
			}
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
			[]string{"172.20.1.0/24", "192.168.1.0/24", "192.168.1.1/32", "255.255.255.255/32"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			table, err := parseNetstatDarwin(readFixture(t, c.file), c.iface, c.ipv6)
			if err != nil {
				t.Fatal(err)
			}
			got := table.Mesh
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
		if got, err := parseNetstatDarwin(out, "utun4", false); err != nil || len(got.Mesh) != 0 {
			t.Errorf("got %v, %v; want nothing", got, err)
		}
	})
	t.Run("the networks of other interfaces are kept as foreign", func(t *testing.T) {
		for _, c := range []struct {
			file string
			ipv6 bool
			want map[string]string
		}{
			{"netstat-inet.handwritten.txt", false, map[string]string{
				"0.0.0.0/0": "en0", "127.0.0.0/8": "lo0", "127.0.0.1/32": "lo0", "172.20.1.0/24": "en0",
				"192.168.1.0/24": "en0", "192.168.1.1/32": "en0", "255.255.255.255/32": "en0"}},
			{"netstat-inet6.handwritten.txt", true, map[string]string{"::/0": "en0", "fd12:3456::/32": "en0"}},
		} {
			table, err := parseNetstatDarwin(readFixture(t, c.file), "utun4", c.ipv6)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(table.Foreign, c.want) {
				t.Errorf("%s: foreign = %v, want %v", c.file, table.Foreign, c.want)
			}
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
	dh.add("172.21.0.0/16", "en0") // another interface: never touched
	d := newDarwinDaemon(t, dh, &logSink{}, newFakeKernel())
	if d.converge([]*PeerInfo{darwinPeer("keyA", "10.99.0.2", "", "172.20.0.0/16", "192.168.50.0/24")}) == nil {
		t.Fatal("converge skipped the first tick")
	}
	dh.add("172.22.0.0/16", "utun4") // added by hand later: not ours
	before := len(dh.routeCommands())

	if d.converge([]*PeerInfo{darwinPeer("keyA", "10.99.0.2", "", "192.168.50.0/24")}) == nil {
		t.Fatal("converge skipped the second tick")
	}
	got := dh.routeCommands()[before:]
	want := []string{"route -n delete -net 172.20.0.0/16 -interface utun4"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("route commands = %q, want %q", got, want)
	}
	if dh.has("172.20.0.0/16", "utun4") {
		t.Error("the route this daemon installed is still in the table after its peer left")
	}
	if !dh.has("172.21.0.0/16", "en0") || !dh.has("172.22.0.0/16", "utun4") || !dh.has("192.168.50.0/24", "utun4") {
		t.Error("a route that must stay was removed")
	}
}

// Only a route this daemon installed in this run is ever deleted: a route
// somebody added by hand to the mesh interface, and one left by an earlier
// run of the daemon, stay.
func TestDarwinHandAddedRouteIsNeverDeleted(t *testing.T) {
	dh := newDarwinHost()
	dh.add("172.20.0.0/16", "utun4") // by hand, before any tick
	d := newDarwinDaemon(t, dh, &logSink{}, newFakeKernel())
	peers := []*PeerInfo{darwinPeer("keyA", "10.99.0.2", "", "192.168.50.0/24")}
	d.converge(peers)
	d.converge(peers)
	for _, c := range dh.routeCommands() {
		if strings.HasPrefix(c, "route -n delete") {
			t.Errorf("deleted a route this daemon did not install: %q", c)
		}
	}
	if !dh.has("172.20.0.0/16", "utun4") {
		t.Error("the hand-added route is gone")
	}
}

func TestDarwinRestartDoesNotDeleteTheRoutesOfTheEarlierRun(t *testing.T) {
	dh := newDarwinHost()
	first := newDarwinDaemon(t, dh, &logSink{}, newFakeKernel())
	first.converge([]*PeerInfo{darwinPeer("keyA", "10.99.0.2", "", "172.20.0.0/16")})
	if !dh.has("172.20.0.0/16", "utun4") {
		t.Fatal("the first run did not install the route")
	}
	restarted := newDarwinDaemon(t, dh, &logSink{}, newFakeKernel())
	restarted.converge([]*PeerInfo{darwinPeer("keyA", "10.99.0.2", "", "192.168.50.0/24")})
	for _, c := range dh.routeCommands() {
		if strings.HasPrefix(c, "route -n delete") {
			t.Errorf("the restarted daemon deleted %q", c)
		}
	}
	if !dh.has("172.20.0.0/16", "utun4") {
		t.Error("the route of the earlier run was deleted")
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
	if got, want := routeWarnedKeys(d), []string{"attached 192.168.50.0/24", "attached 192.168.50.128/25"}; !reflect.DeepEqual(got, want) {
		t.Errorf("warn-once set = %q, want %q", got, want)
	}
}

func TestDarwinV6RouteUsesInet6(t *testing.T) {
	dh := newDarwinHost()
	d := newDarwinDaemon(t, dh, &logSink{}, newFakeKernel())
	if d.converge([]*PeerInfo{darwinPeer("keyA", "10.99.0.2", "fd99::2", "fd00:6::/64")}) == nil {
		t.Fatal("converge skipped the first tick")
	}
	before := len(dh.routeCommands())

	if d.converge([]*PeerInfo{darwinPeer("keyA", "10.99.0.2", "fd99::2", "fd00:5::/48", "192.168.50.0/24")}) == nil {
		t.Fatal("converge skipped the second tick")
	}
	got := dh.routeCommands()[before:]
	sort.Strings(got)
	want := []string{
		"route -n add -inet6 -net fd00:5::/48 -interface utun4",
		"route -n add -net 192.168.50.0/24 -interface utun4",
		"route -n delete -inet6 -net fd00:6::/64 -interface utun4",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("route commands = %q, want %q", got, want)
	}
	if n := dh.count("netstat -rn -f inet6"); n != 2 {
		t.Errorf("the IPv6 table was read %d times over 2 ticks, want 2", n)
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
		if dh.count("netstat") != 2 || dh.count("netstat -rn -f inet6") != 1 {
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
		if keys := resultKeys(d); len(keys) != 0 {
			t.Errorf("File exists is a failure of %q", keys)
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
		if got, want := resultKeys(d), []string{"route change 192.168.50.0/24"}; !reflect.DeepEqual(got, want) {
			t.Errorf("failed items = %q, want %q", got, want)
		}
	})
}

func TestDarwinPlanLeavesMeshSubnetAlone(t *testing.T) {
	keep := []netip.Prefix{netip.MustParsePrefix("10.99.0.0/16"), netip.MustParsePrefix("fd99::/64")}
	entry := func(n string) routes.Entry { return routes.Entry{Network: n} }
	// 172.20.0.0/16 and fd00:6::/64 were installed by this daemon; 10.77.0.0/16
	// and fd00:7::/64 are on the interface but not ours.
	installed := map[string]struct{}{"172.20.0.0/16": {}, "fd00:6::/64": {}, "10.99.0.0/16": {}}
	obs := kernelObserved{
		Routes4: []routes.Entry{entry("10.99.0.0/16"), entry("172.20.0.0/16"), entry("192.168.50.0/24"), entry("10.77.0.0/16")},
		Routes6: []routes.Entry{entry("fd99::/64"), entry("fd00:6::/64"), entry("fd00:7::/64")},
	}
	want := []routes.Entry{entry("192.168.50.0/24"), entry("10.50.0.0/16"), entry("fd00:5::/48")}
	plan, _ := planRoutesDarwin(want, obs, keep, installed)
	if got, w := networksOf(plan.Add), []string{"10.50.0.0/16", "fd00:5::/48"}; !reflect.DeepEqual(got, w) {
		t.Errorf("add = %q, want %q", got, w)
	}
	if got, w := networksOf(plan.Remove), []string{"172.20.0.0/16", "fd00:6::/64"}; !reflect.DeepEqual(got, w) {
		t.Errorf("remove = %q, want %q (not the mesh subnet, not a route that is not ours)", got, w)
	}

	t.Run("a family whose read failed plans nothing, removes nothing", func(t *testing.T) {
		failed := obs
		failed.Errs = map[string]error{targetRoutes4: fmt.Errorf("netstat failed")}
		plan, _ := planRoutesDarwin(want, failed, keep, installed)
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
	afterFirst := ls.total()
	d.converge(peers)
	if cmds := dh.routeCommands(); len(cmds) != 0 {
		t.Errorf("IPv4 routes written although the table could not be read: %q", cmds)
	}
	if observeErr(d, targetRoutes4) == "" || observeErr(d, targetRoutes6) != "" {
		t.Errorf("observe errors: IPv4 %q, IPv6 %q; want only IPv4 set", observeErr(d, targetRoutes4), observeErr(d, targetRoutes6))
	}
	if got := ls.total(); got != afterFirst {
		t.Errorf("tick 2 logged %d more lines, want none (the failure is logged once)", got-afterFirst)
	}
}

// reconcile runs from the ticker and from handleSIGHUP under reconcileMu, and
// the route target of one tick under convergeMu: two ticks at once never
// overlap in the kernel commands, so the second sees the first's route and
// writes nothing. The first command blocks until the second goroutine is
// inside reconcile (seen in the goroutine stacks), so the overlap is forced,
// not hoped for; every wait is bounded. Run with -race.
func TestDarwinRouteTickRaceFree(t *testing.T) {
	const bound = 10 * time.Second
	dh := newDarwinHost()
	release, blocked := make(chan struct{}), make(chan struct{})
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
			close(blocked)
			select {
			case <-release:
			case <-time.After(bound):
			}
		}
	}
	d := newDarwinDaemon(t, dh, &logSink{}, newFakeKernel())
	d.peerStore.Update(darwinPeer("keyA", "10.99.0.2", "", "192.168.50.0/24"), "test")
	var wg sync.WaitGroup
	start := func() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.reconcile()
		}()
	}
	start()
	select {
	case <-blocked:
	case <-time.After(bound):
		close(release)
		t.Fatal("the first tick ran no command")
	}
	start()
	insideReconcile := func() int {
		buf := make([]byte, 1<<20)
		return strings.Count(string(buf[:runtime.Stack(buf, true)]), "(*Daemon).reconcile(")
	}
	for deadline := time.Now().Add(bound); insideReconcile() < 2; runtime.Gosched() {
		if time.Now().After(deadline) {
			close(release)
			t.Fatal("the second goroutine never entered reconcile")
		}
	}
	close(release)
	wg.Wait()
	if m := maxInside.Load(); m != 1 {
		t.Errorf("%d commands ran at the same time, want 1", m)
	}
	if got := dh.routeCommands(); !reflect.DeepEqual(got, []string{"route -n add -net 192.168.50.0/24 -interface utun4"}) {
		t.Errorf("route commands = %q, want exactly one add", got)
	}
}

func foreignWarnings() float64 {
	return testutil.ToFloat64(kernelWarnings.WithLabelValues("foreign-route"))
}

// A network that another interface already routes (the host's default via
// en0, a route of another VPN) is never re-pointed into the mesh: no route
// command at all, one warning however many ticks.
func TestDarwinForeignRouteIsNeverRepointed(t *testing.T) {
	for _, c := range []struct{ name, network, gw, flags, netif string }{
		{"the host's default via en0", "0.0.0.0/0", "192.168.1.1", "UGScg", "en0"},
		{"a network of another VPN", "10.50.0.0/16", "link#20", "UCS", "utun2"},
		{"an IPv6 default via en0", "::/0", "fe80::1%en0", "UGcg", "en0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dh := newDarwinHost()
			dh.addRow(c.network, c.gw, c.flags, c.netif)
			d := newDarwinDaemon(t, dh, &logSink{}, newFakeKernel())
			peers := []*PeerInfo{darwinPeer("keyA", "10.99.0.2", "fd99::2", c.network)}
			before := foreignWarnings()
			for i := 0; i < 3; i++ {
				if d.converge(peers) == nil {
					t.Fatalf("tick %d skipped", i+1)
				}
			}
			if cmds := dh.routeCommands(); len(cmds) != 0 {
				t.Errorf("route commands ran for a foreign network: %q", cmds)
			}
			if !dh.has(c.network, c.netif) || dh.has(c.network, "utun4") {
				t.Errorf("the route of %s moved", c.netif)
			}
			if got := foreignWarnings() - before; got != 1 {
				t.Errorf("foreign-route counted %v times over 3 ticks, want 1", got)
			}
		})
	}
}

// The warn-once set holds only what still holds: a peer that leaves and comes
// back is warned about again.
func TestDarwinForeignWarningRepeatsAfterTheCaseEnds(t *testing.T) {
	dh := newDarwinHost()
	dh.addRow("10.50.0.0/16", "link#20", "UCS", "utun2")
	d := newDarwinDaemon(t, dh, &logSink{}, newFakeKernel())
	peers := []*PeerInfo{darwinPeer("keyA", "10.99.0.2", "", "10.50.0.0/16")}
	before := foreignWarnings()
	d.converge(peers)
	d.converge(nil)
	d.converge(peers)
	if got := foreignWarnings() - before; got != 2 {
		t.Errorf("foreign-route counted %v times, want 2 (once per time the case holds)", got)
	}
}
func warnedKeys(d *Daemon) []string {
	d.convergeMu.Lock()
	defer d.convergeMu.Unlock()
	var keys []string
	for k := range d.darwinWarned {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// The column layout is read from the header line; both layouts below are
// hand-written from the man page and unverified on a Mac (S1 decides).
func TestParseNetstatDarwinTolerance(t *testing.T) {
	for _, c := range []struct {
		name, file string
		want       []string
	}{
		{"the old layout with Refs and Use before Netif", "netstat-inet-oldlayout.handwritten.txt",
			[]string{"0.0.0.0/0", "10.100.0.5/32", "10.99.0.0/16", "192.168.5.0/24"}},
		{"the interface name as the gateway", "netstat-inet-ifname-gateway.handwritten.txt",
			[]string{"10.100.0.5/32", "10.99.0.0/16", "192.168.5.0/24"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			table, err := parseNetstatDarwin(readFixture(t, c.file), "utun4", false)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(networksOf(table.Mesh), c.want) {
				t.Errorf("networks = %q, want %q", networksOf(table.Mesh), c.want)
			}
			if table.Foreign["0.0.0.0/0"] != "en0" {
				t.Errorf("foreign = %v, want the default route of en0", table.Foreign)
			}
			if len(table.Unrecognised) != 0 {
				t.Errorf("unrecognised rows %q", table.Unrecognised)
			}
		})
	}
	t.Run("no header line falls back to the current layout", func(t *testing.T) {
		out := []byte("192.168.5/24 link#16 UCS utun4\n10.7/16 link#14 UCS en0\n")
		table, err := parseNetstatDarwin(out, "utun4", false)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := networksOf(table.Mesh), []string{"192.168.5.0/24"}; !reflect.DeepEqual(got, want) {
			t.Errorf("networks = %q, want %q", got, want)
		}
		if table.Foreign["10.7.0.0/16"] != "en0" {
			t.Errorf("foreign = %v", table.Foreign)
		}
	})
	t.Run("a mesh row of no accepted shape is skipped and reported", func(t *testing.T) {
		table, err := parseNetstatDarwin(readFixture(t, "netstat-inet.handwritten.txt"), "utun4", false)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"10.99.0.1 10.99.0.1 UH utun4"}; !reflect.DeepEqual(table.Unrecognised, want) {
			t.Errorf("unrecognised = %q, want %q", table.Unrecognised, want)
		}
	})
}

func TestDarwinUnrecognisedRowIsReportedOnce(t *testing.T) {
	dh := newDarwinHost()
	dh.noise[4] = []string{"10.5.0.0/16 10.99.0.9 UGS utun4"}
	ls := &logSink{}
	d := newDarwinDaemon(t, dh, ls, newFakeKernel())
	peers := []*PeerInfo{darwinPeer("keyA", "10.99.0.2", "", "192.168.50.0/24")}
	d.converge(peers)
	first := ls.total()
	d.converge(peers)
	d.converge(peers)
	if got := ls.total(); got != first {
		t.Errorf("ticks 2 and 3 logged %d more lines, want none", got-first)
	}
	if got, want := warnedKeys(d), []string{"row|4|10.5.0.0/16 10.99.0.9 UGS utun4"}; !reflect.DeepEqual(got, want) {
		t.Errorf("warned = %q, want %q", got, want)
	}
	if got, want := dh.routeCommands(), []string{"route -n add -net 192.168.50.0/24 -interface utun4"}; !reflect.DeepEqual(got, want) {
		t.Errorf("route commands = %q, want %q", got, want)
	}
}

// The state the tests read back, each read under convergeMu.
func routeWarnedKeys(d *Daemon) []string {
	d.convergeMu.Lock()
	defer d.convergeMu.Unlock()
	var keys []string
	for k := range d.routeWarned {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// resultKeys are the items whose last write failed.
func resultKeys(d *Daemon) []string {
	d.convergeMu.Lock()
	defer d.convergeMu.Unlock()
	var keys []string
	for k := range d.lastResults {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func observeErr(d *Daemon, target string) string {
	d.convergeMu.Lock()
	defer d.convergeMu.Unlock()
	return d.routeObserveErr[target]
}

// Each filter of the parser is pinned alone: a row that only one rule
// excludes is not a route, whether it belongs to the mesh interface or not.
func TestParseNetstatDarwinEachFilterAlone(t *testing.T) {
	const header = "Destination Gateway Flags Netif Expire\n"
	for _, c := range []struct {
		name, row string
		ipv6      bool
	}{
		{"flag m", "10.99.0.11 link#16 UHm utun4", false},
		{"flag b", "10.99.255.255 link#16 UHb utun4", false},
		{"flag L without a scope", "10.99.0.9 link#16 UHL utun4", false},
		{"flag W", "10.99.0.10 link#16 UHW utun4", false},
		{"an IPv4 multicast address without flag m", "224.0.0.251 link#16 UHS utun4", false},
		{"an IPv6 multicast address without flag m", "ff02::fb link#16 UHS utun4", true},
		{"an IPv4 link-local network without a scope", "169.254.0.0/16 link#16 UCS utun4", false},
		{"an IPv6 link-local network without a scope", "fe80::/64 link#16 UCS utun4", true},
		{"a scope in the destination", "fd00:9::%utun4/64 link#16 UCS utun4", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			control, controlNet := "10.7.0.0/16 link#16 UCS utun4\n", "10.7.0.0/16"
			if c.ipv6 {
				control, controlNet = "fd00:7::/48 link#16 UCS utun4\n", "fd00:7::/48"
			}
			table, err := parseNetstatDarwin([]byte(header+control+c.row+"\n"), "utun4", c.ipv6)
			if err != nil {
				t.Fatal(err)
			}
			if got := networksOf(table.Mesh); !reflect.DeepEqual(got, []string{controlNet}) {
				t.Errorf("mesh routes = %q, want only the control row %q", got, controlNet)
			}
			other := strings.Replace(c.row, "utun4", "en0", 1)
			table, err = parseNetstatDarwin([]byte(header+strings.Replace(control, "utun4", "en0", 1)+other+"\n"), "utun4", c.ipv6)
			if err != nil {
				t.Fatal(err)
			}
			if want := map[string]string{controlNet: "en0"}; !reflect.DeepEqual(table.Foreign, want) {
				t.Errorf("foreign = %v, want only the control row %v", table.Foreign, want)
			}
		})
	}
}

// A failing delete is that route's failure only: the adds still run, and the
// route stays in the installed set, so the next tick tries again.
func TestDarwinFailingDeleteDoesNotStopTheAdds(t *testing.T) {
	dh := newDarwinHost()
	dh.delErr["route -n delete -net 172.20.0.0/16 -interface utun4"] = exitError{1}
	d := newDarwinDaemon(t, dh, &logSink{}, newFakeKernel())
	d.converge([]*PeerInfo{darwinPeer("keyA", "10.99.0.2", "", "172.20.0.0/16")})
	next := []*PeerInfo{darwinPeer("keyA", "10.99.0.2", "", "192.168.50.0/24", "10.50.0.0/16")}
	d.converge(next)
	if !dh.has("192.168.50.0/24", "utun4") || !dh.has("10.50.0.0/16", "utun4") {
		t.Error("an add was skipped after a delete failed")
	}
	if !dh.has("172.20.0.0/16", "utun4") {
		t.Error("the route whose delete failed is gone")
	}
	if got, want := resultKeys(d), []string{"route delete 172.20.0.0/16"}; !reflect.DeepEqual(got, want) {
		t.Errorf("failed items = %q, want %q", got, want)
	}
	before := len(dh.routeCommands())
	d.converge(next)
	if got, want := dh.routeCommands()[before:], []string{"route -n delete -net 172.20.0.0/16 -interface utun4"}; !reflect.DeepEqual(got, want) {
		t.Errorf("the next tick ran %q, want the delete again %q", got, want)
	}
}

// A row of the mesh interface whose destination does not parse makes that
// family's table unreadable: nothing is written for the family (and its error
// is kept once), while the other family goes on.
func TestDarwinParseErrorRowIsAFamilyErrorEndToEnd(t *testing.T) {
	dh := newDarwinHost()
	dh.noise[4] = []string{"not-a-network link#16 UCS utun4"}
	ls := &logSink{}
	d := newDarwinDaemon(t, dh, ls, newFakeKernel())
	peers := []*PeerInfo{darwinPeer("keyA", "10.99.0.2", "fd99::2", "192.168.50.0/24", "fd00:5::/48")}
	d.converge(peers)
	afterFirst := ls.total()
	d.converge(peers)
	d.converge(peers)
	if got, want := dh.routeCommands(), []string{"route -n add -inet6 -net fd00:5::/48 -interface utun4"}; !reflect.DeepEqual(got, want) {
		t.Errorf("route commands = %q, want only the IPv6 add %q", got, want)
	}
	if observeErr(d, targetRoutes4) == "" || observeErr(d, targetRoutes6) != "" {
		t.Errorf("observe errors: IPv4 %q, IPv6 %q; want only IPv4 set", observeErr(d, targetRoutes4), observeErr(d, targetRoutes6))
	}
	if got := ls.total(); got != afterFirst {
		t.Errorf("ticks 2 and 3 logged %d more lines, want none", got-afterFirst)
	}
}

func TestRouteAdapterIsOneDecisionPerOS(t *testing.T) {
	for goos, want := range map[string]routeAdapter{"linux": routesLinux, "darwin": routesDarwin, "plan9": routesNone, "windows": routesNone} {
		if got := (&Daemon{goos: goos}).routeAdapter(); got != want {
			t.Errorf("routeAdapter on %s = %d, want %d", goos, got, want)
		}
	}
}
