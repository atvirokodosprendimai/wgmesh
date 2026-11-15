package daemon

import (
	"errors"
	"log"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Two nodes advertising the same subnet (HA): the local node must not put the
// shared subnet into a peer's AllowedIPs nor install a route for it.
func TestSharedSubnetNeverRoutedThroughMesh(t *testing.T) {
	const shared = "192.168.10.0/24"
	const disjoint = "192.168.20.0/24"

	newDaemon := func() (*Daemon, []*PeerInfo) {
		d := makeRelayTestDaemon()
		d.config.InterfaceName = "wg0"
		d.localNode.RoutableNetworks = []string{shared}
		peer := &PeerInfo{
			WGPubKey: "peer1", MeshIP: "10.0.0.20", LastSeen: time.Now(),
			RoutableNetworks: []string{shared, disjoint},
		}
		return d, []*PeerInfo{peer}
	}

	t.Run("allowed ips", func(t *testing.T) {
		d, peers := newDaemon()
		desired, _, _ := d.buildDesiredPeerConfigsWithHandshakes(peers, nil)
		allowed := desired["peer1"].allowed
		if _, ok := allowed[shared]; ok {
			t.Errorf("shared subnet in peer AllowedIPs: %v", allowed)
		}
		if _, ok := allowed[disjoint]; !ok {
			t.Errorf("disjoint network missing from AllowedIPs: %v", allowed)
		}
	})

	t.Run("routes", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("syncPeerRoutes is Linux-only")
		}
		d, peers := newDaemon()
		var cmds []string
		mock := &MockCommandExecutor{
			commandFunc: func(name string, args ...string) Command {
				cmds = append(cmds, name+" "+strings.Join(args, " "))
				return &MockCommand{}
			},
		}
		withMockExecutor(t, mock, func() {
			if err := d.syncPeerRoutes(peers); err != nil {
				t.Fatalf("syncPeerRoutes: %v", err)
			}
		})
		all := strings.Join(cmds, "\n")
		if strings.Contains(all, shared) {
			t.Errorf("route installed for shared subnet:\n%s", all)
		}
		if !strings.Contains(all, "route replace "+disjoint) {
			t.Errorf("disjoint network not routed:\n%s", all)
		}
	})
}

// The relay branch must not put the shared subnet under the relay's AllowedIPs.
func TestSharedSubnetNotRelayedThroughMesh(t *testing.T) {
	const shared = "192.168.10.0/24"
	const disjoint = "192.168.20.0/24"

	d := makeRelayTestDaemon()
	d.config.ForceRelay = true
	d.localNode.RoutableNetworks = []string{shared}
	relay := &PeerInfo{
		WGPubKey: "relay1", MeshIP: "10.0.0.10", Endpoint: "1.2.3.4:51820",
		Introducer: true, LastSeen: time.Now(),
	}
	target := &PeerInfo{
		WGPubKey: "peer1", MeshIP: "10.0.0.20", LastSeen: time.Now(),
		RoutableNetworks: []string{shared, disjoint},
	}

	desired, relayRoutes, _ := d.buildDesiredPeerConfigsWithHandshakes([]*PeerInfo{relay, target}, nil)
	if relayRoutes["peer1"] != "relay1" {
		t.Fatalf("relay branch not taken: relayRoutes = %v", relayRoutes)
	}
	allowed := desired["relay1"].allowed
	if _, ok := allowed[shared]; ok {
		t.Errorf("shared subnet in relay AllowedIPs: %v", allowed)
	}
	if _, ok := allowed[disjoint]; !ok {
		t.Errorf("disjoint network missing from relay AllowedIPs: %v", allowed)
	}
}

func TestReconnectAllowedCSVExcludesOwnNetworks(t *testing.T) {
	peer := &PeerInfo{
		WGPubKey: "peer1", MeshIP: "10.0.0.20",
		RoutableNetworks: []string{"192.168.10.0/24", "192.168.20.0/24"},
	}
	got := reconnectAllowedCSV(peer, []string{"192.168.10.0/24"})
	if want := "10.0.0.20/32,192.168.20.0/24"; got != want {
		t.Errorf("reconnectAllowedCSV() = %q, want %q", got, want)
	}
}

// An older daemon overwrote the connected LAN route with `replace <shared> via
// <peer> dev wg0`. When that via route is removed the connected route must come back.
func TestRemovedOwnViaRouteRestoresConnectedRoute(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("syncPeerRoutes is Linux-only")
	}
	const shared = "192.168.10.0/24"
	const other = "192.168.30.0/24"

	d := makeRelayTestDaemon()
	d.config.InterfaceName = "wg0"
	d.localNode.RoutableNetworks = []string{shared}
	peer := &PeerInfo{WGPubKey: "peer1", MeshIP: "10.0.0.20", LastSeen: time.Now(), RoutableNetworks: []string{shared}}

	var cmds []string
	mock := &MockCommandExecutor{
		commandFunc: func(name string, args ...string) Command {
			line := name + " " + strings.Join(args, " ")
			cmds = append(cmds, line)
			out := ""
			switch line {
			case "ip route show dev wg0":
				out = shared + " via 10.0.0.20 dev wg0\n" + other + " via 10.0.0.20 dev wg0\n"
			case "ip -4 -o addr show to " + shared:
				out = "4: eth0    inet 192.168.10.1/24 scope global eth0\\       valid_lft forever preferred_lft forever\n"
			}
			return &MockCommand{outputFunc: func() ([]byte, error) { return []byte(out), nil }}
		},
	}
	withMockExecutor(t, mock, func() {
		if err := d.syncPeerRoutes([]*PeerInfo{peer}); err != nil {
			t.Fatalf("syncPeerRoutes: %v", err)
		}
	})
	all := strings.Join(cmds, "\n")
	del := "ip route del " + shared + " via 10.0.0.20 dev wg0"
	add := "ip route add " + shared + " dev eth0 proto kernel scope link src 192.168.10.1"
	if !strings.Contains(all, del) {
		t.Errorf("via route not removed:\n%s", all)
	}
	if !strings.Contains(all, add) {
		t.Errorf("connected route not restored:\n%s", all)
	}
	if strings.Contains(all, "ip -4 -o addr show to "+other) {
		t.Errorf("connected route restore attempted for a non-own network:\n%s", all)
	}
}

// If `ip route del` fails the via route still stands: no restore, no "Restored" log.
func TestFailedDelDoesNotRestoreConnectedRoute(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("syncPeerRoutes is Linux-only")
	}
	const shared = "192.168.10.0/24"

	d := makeRelayTestDaemon()
	d.config.InterfaceName = "wg0"
	d.localNode.RoutableNetworks = []string{shared}
	peer := &PeerInfo{WGPubKey: "peer1", MeshIP: "10.0.0.20", LastSeen: time.Now(), RoutableNetworks: []string{shared}}

	var cmds []string
	mock := &MockCommandExecutor{
		commandFunc: func(name string, args ...string) Command {
			line := name + " " + strings.Join(args, " ")
			cmds = append(cmds, line)
			out := ""
			switch line {
			case "ip route show dev wg0":
				out = shared + " via 10.0.0.20 dev wg0\n"
			case "ip -4 -o addr show to " + shared:
				out = "4: eth0    inet 192.168.10.1/24 scope global eth0\\       valid_lft forever preferred_lft forever\n"
			}
			cmd := &MockCommand{outputFunc: func() ([]byte, error) { return []byte(out), nil }}
			if strings.HasPrefix(line, "ip route del ") {
				cmd.runFunc = func() error { return errors.New("exit status 2") }
			}
			return cmd
		},
	}
	var logs strings.Builder
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)
	withMockExecutor(t, mock, func() {
		if err := d.syncPeerRoutes([]*PeerInfo{peer}); err != nil {
			t.Fatalf("syncPeerRoutes: %v", err)
		}
	})
	all := strings.Join(cmds, "\n")
	if strings.Contains(all, "ip route add "+shared) {
		t.Errorf("connected route add attempted after a failed del:\n%s", all)
	}
	if strings.Contains(logs.String(), "Restored connected route") {
		t.Errorf("logged a restore after a failed del: %s", logs.String())
	}
}

func TestRestoreConnectedRouteLogsWhyItSkips(t *testing.T) {
	const shared = "192.168.10.0/24"
	tests := []struct {
		name string
		out  string
		err  error
		want string
	}{
		{name: "query fails", err: errors.New("exec: ip not found"), want: "exec: ip not found"},
		{name: "no local address", want: "no local address in " + shared},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockCommandExecutor{commandFunc: func(string, ...string) Command {
				return &MockCommand{outputFunc: func() ([]byte, error) { return []byte(tt.out), tt.err }}
			}}
			var logs strings.Builder
			log.SetOutput(&logs)
			defer log.SetOutput(os.Stderr)
			withMockExecutor(t, mock, func() { restoreConnectedRoute(shared) })
			if !strings.Contains(logs.String(), tt.want) {
				t.Errorf("log = %q, want it to contain %q", logs.String(), tt.want)
			}
		})
	}
}
