package daemon

import (
	"errors"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// indexOf is the position of the first logged command with the prefix, -1 if none.
func (r *routeHost) indexOf(prefix string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, c := range r.log {
		if strings.HasPrefix(c, prefix) {
			return i
		}
	}
	return -1
}

// A graceful exit while the mesh default route is installed puts back the
// host default it replaced, before the interface is deleted (the kernel drops
// the mesh default with the device, so without the restore the host has no
// default route). The restore is not behind EnforceRevertOnExit. Run calls
// revertKernel as a plain statement and teardownWireGuard is deferred
// (TestRunRevertsKernelAfterWait), so the order below is Run's.
func TestExitRestoresHostDefault(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("teardownWireGuard runs `ip link del` on Linux only")
	}
	cases := []struct {
		name     string
		hostGW   string
		hostDev  string
		network  string
		meshGW   string
		meshIPv6 string
		restore  string
	}{
		{"IPv4 with a gateway", "192.168.1.1", "eth0", "0.0.0.0/0", "10.99.0.2", "", "ip route replace default via 192.168.1.1 dev eth0 metric 0"},
		{"IPv4 without a gateway", "", "ppp0", "0.0.0.0/0", "10.99.0.2", "", "ip route replace default dev ppp0 metric 0"},
		{"IPv6", "fe80::1", "eth0", "::/0", "fd99::2", "fd99::2", "ip -6 route replace default via fe80::1 dev eth0 metric 1024"},
	}
	for _, c := range cases {
		for _, enforce := range []bool{false, true} {
			name := c.name + " EnforceRevertOnExit=false"
			if enforce {
				name = c.name + " EnforceRevertOnExit=true"
			}
			t.Run(name, func(t *testing.T) {
				enforceRevert(t, enforce)
				rh := convergedHost()
				rh.add("default", c.hostGW, c.hostDev)
				ls := &logSink{}
				d := newRouteDaemon(t, rh, ls, newFakeKernel())
				peer := testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", c.network)
				peer.MeshIPv6 = c.meshIPv6
				ticks(t, d, rh, []*PeerInfo{peer}, 2)
				if !rh.has(c.network, c.meshGW, "wg0") || rh.has(c.network, c.hostGW, c.hostDev) {
					t.Fatalf("setup: the mesh default did not replace the host default: %q", rh.routeWrites())
				}
				before := len(rh.routeWrites())

				d.revertKernel()
				d.teardownWireGuard()

				if got, want := rh.routeWrites()[before:], []string{c.restore}; !sameLines(got, want) {
					t.Errorf("exit route writes = %q, want %q", got, want)
				}
				restore, del := rh.indexOf(c.restore), rh.indexOf("ip link del dev wg0")
				if restore < 0 || del < 0 || restore > del {
					t.Errorf("restore at command %d, link del at %d: the restore must come first; commands %q", restore, del, rh.log)
				}
				if !rh.has(c.network, c.hostGW, c.hostDev) {
					t.Errorf("the host has no default route via %q dev %s after the exit", c.hostGW, c.hostDev)
				}
				if n := ls.count("restored the host's"); n != 1 || ls.count("at exit, before wg0 is deleted") != 1 {
					t.Errorf("the exit restore was logged %d times, want once with its reason: %q", n, ls.lines)
				}
				d.revertKernel()
				if got := len(rh.routeWrites()) - before; got != 1 {
					t.Errorf("a second revert wrote again: %q", rh.routeWrites()[before:])
				}
			})
		}
	}

	t.Run("no host default, nothing restored", func(t *testing.T) {
		rh := convergedHost()
		d := newRouteDaemon(t, rh, &logSink{}, newFakeKernel())
		ticks(t, d, rh, []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "0.0.0.0/0")}, 2)
		before := len(rh.routeWrites())
		d.revertKernel()
		if got := rh.routeWrites()[before:]; len(got) != 0 {
			t.Errorf("exit wrote %q without a host default", got)
		}
	})

	t.Run("already restored by a tick, nothing at exit", func(t *testing.T) {
		rh := convergedHost()
		rh.add("default", "192.168.1.1", "eth0")
		d := newRouteDaemon(t, rh, &logSink{}, newFakeKernel())
		ticks(t, d, rh, []*PeerInfo{testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", "0.0.0.0/0")}, 2)
		ticks(t, d, rh, nil, 1)
		before := len(rh.routeWrites())
		d.revertKernel()
		if got := rh.routeWrites()[before:]; len(got) != 0 {
			t.Errorf("exit wrote %q after the tick restored the host default", got)
		}
	})
}

// A host default of another metric than the mesh default's has another route
// key: the mesh default is added next to it and replaces nothing, so it is
// not recorded, not claimed as replaced in the log, and nothing is restored
// when the mesh default goes, by a tick or at exit.
func TestHostDefaultOfOtherMetricIsNotReplaced(t *testing.T) {
	warnings := func() float64 { return testutil.ToFloat64(kernelWarnings.WithLabelValues("host-default")) }
	cases := []struct {
		name, network, hostGW, meshGW, meshIPv6 string
		metric                                  int
		del                                     string
	}{
		{"IPv4 DHCP metric 100", "0.0.0.0/0", "192.168.1.1", "10.99.0.2", "", 100, "ip route del 0.0.0.0/0 via 10.99.0.2 dev wg0"},
		{"IPv6 metric 100", "::/0", "fe80::1", "fd99::2", "fd99::2", 100, "ip route del ::/0 via fd99::2 dev wg0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			enforceRevert(t, true)
			rh := convergedHost()
			rh.addMetric(c.network, c.hostGW, "eth0", c.metric)
			ls := &logSink{}
			d := newRouteDaemon(t, rh, ls, newFakeKernel())
			peer := testPeer("keyA", "10.99.0.2", "203.0.113.2:51820", c.network)
			peer.MeshIPv6 = c.meshIPv6
			before := warnings()
			ticks(t, d, rh, []*PeerInfo{peer}, 3)
			if !rh.has(c.network, c.meshGW, "wg0") || !rh.hasMetric(c.network, c.hostGW, "eth0", c.metric) {
				t.Fatalf("want the mesh default next to the host's: %q", rh.routeWrites())
			}
			if n := ls.count("replaces it"); n != 0 {
				t.Errorf("the log claims a replacement that did not happen: %q", ls.lines)
			}
			if n := ls.count("host-default: this host already has"); n != 1 || ls.count("the host's stays") != 1 {
				t.Errorf("want one host-default line saying the host's stays: %q", ls.lines)
			}
			if got := warnings() - before; got != 1 {
				t.Errorf("host-default counted %v times, want 1", got)
			}
			d.convergeMu.Lock()
			n := len(d.hostDefaults)
			d.convergeMu.Unlock()
			if n != 0 {
				t.Errorf("a host default of metric %d was recorded for a restore", c.metric)
			}

			got := ticks(t, d, rh, nil, 2)
			if want := []string{c.del}; !sameLines(got[0], want) || len(got[1]) != 0 {
				t.Errorf("removal writes %q %q, want only %q", got[0], got[1], want)
			}
			before2 := len(rh.routeWrites())
			d.revertKernel()
			if got := rh.routeWrites()[before2:]; len(got) != 0 {
				t.Errorf("exit wrote %q, want nothing", got)
			}
			if !rh.hasMetric(c.network, c.hostGW, "eth0", c.metric) || ls.count("restored the host's") != 0 {
				t.Errorf("the host default of metric %d was touched: %q", c.metric, rh.routeWrites())
			}
		})
	}
}

// hostDefaultElsewhere keeps the metric and the onlink flag, and picks the
// host default the mesh default replaces (the family's default metric) over
// one of another metric.
func TestHostDefaultElsewhereParses(t *testing.T) {
	cases := []struct {
		name    string
		ipv6    bool
		out     string
		want    hostRoute
		wantErr bool
	}{
		{"static, metric 0 not printed", false, "default via 192.168.1.1 dev eth0\n", hostRoute{Dev: "eth0", Via: "192.168.1.1"}, false},
		{"DHCP", false, "default via 192.168.1.1 dev eth0 proto dhcp src 192.168.1.10 metric 100\n", hostRoute{Dev: "eth0", Via: "192.168.1.1", Metric: 100}, false},
		{"onlink", false, "default via 172.31.1.1 dev eth0 onlink\n", hostRoute{Dev: "eth0", Via: "172.31.1.1", Onlink: true}, false},
		{"the replaced one first", false, "default via 10.0.0.1 dev eth1 proto dhcp metric 100\ndefault via 192.168.1.1 dev eth0\n", hostRoute{Dev: "eth0", Via: "192.168.1.1"}, false},
		{"another metric only", false, "default via 10.0.0.1 dev eth1 metric 100\ndefault via 10.0.0.2 dev eth2 metric 600\n", hostRoute{Dev: "eth1", Via: "10.0.0.1", Metric: 100}, false},
		{"the mesh interface's is skipped", false, "default via 10.99.0.2 dev wg0\n", hostRoute{}, false},
		{"IPv6 RA", true, "default via fe80::1 dev eth0 proto ra metric 1024 expires 1799sec pref medium\n", hostRoute{Dev: "eth0", Via: "fe80::1", Metric: 1024}, false},
		{"bad metric", false, "default via 192.168.1.1 dev eth0 metric x\n", hostRoute{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			old := cmdExecutor.swap(&MockCommandExecutor{commandFunc: func(name string, args ...string) Command {
				return &MockCommand{outputFunc: func() ([]byte, error) { return []byte(c.out), nil }}
			}})
			defer cmdExecutor.swap(old)
			got, err := hostDefaultElsewhere("wg0", c.ipv6)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, want error %v", err, c.wantErr)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

// The restore puts the metric (and onlink) back, so it re-creates the route
// that was replaced and not a new one of another route key.
func TestRestoreHostDefaultKeepsMetric(t *testing.T) {
	cases := []struct {
		name string
		r    hostRoute
		ipv6 bool
		want string
	}{
		{"IPv4", hostRoute{Dev: "eth0", Via: "192.168.1.1"}, false, "ip route replace default via 192.168.1.1 dev eth0 metric 0"},
		{"IPv4 onlink", hostRoute{Dev: "eth0", Via: "172.31.1.1", Onlink: true}, false, "ip route replace default via 172.31.1.1 dev eth0 onlink metric 0"},
		{"IPv6", hostRoute{Dev: "eth0", Via: "fe80::1", Metric: 1024}, true, "ip -6 route replace default via fe80::1 dev eth0 metric 1024"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			old := cmdExecutor.swap(&MockCommandExecutor{commandFunc: func(name string, args ...string) Command {
				got = append(got, name+" "+strings.Join(args, " "))
				return &MockCommand{combinedOutputFunc: func() ([]byte, error) { return nil, nil }}
			}})
			defer cmdExecutor.swap(old)
			if res := restoreHostDefault(c.r, c.ipv6); res.Err != nil {
				t.Fatal(res.Err)
			}
			if want := []string{c.want}; !reflect.DeepEqual(got, want) {
				t.Errorf("commands = %q, want %q", got, want)
			}
		})
	}

	t.Run("a failure is a result", func(t *testing.T) {
		old := cmdExecutor.swap(&MockCommandExecutor{commandFunc: func(name string, args ...string) Command {
			return &MockCommand{combinedOutputFunc: func() ([]byte, error) {
				return []byte("Error: Nexthop has invalid gateway."), errors.New("exit status 2")
			}}
		}})
		defer cmdExecutor.swap(old)
		if res := restoreHostDefault(hostRoute{Dev: "eth0", Via: "172.31.1.1"}, false); res.Err == nil {
			t.Error("a failed restore is not an error result")
		}
	})
}
