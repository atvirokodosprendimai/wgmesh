package daemon

import (
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
