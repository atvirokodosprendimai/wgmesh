package mesh

import (
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh"
	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

func TestDeploymentConfigUnderLock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		groups        bool
		policy        bool
		allowNetworks bool
		wantPeers     int
		wantNetworks  bool
	}{
		{name: "full mesh", wantPeers: 1, wantNetworks: true},
		{name: "groups without policies", groups: true},
		{name: "mesh access only", groups: true, policy: true, wantPeers: 1},
		{
			name: "routed access", groups: true, policy: true,
			allowNetworks: true, wantPeers: 1, wantNetworks: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			node := &Node{
				Hostname: "node1", MeshIP: net.ParseIP("10.99.0.1"), ListenPort: 51820,
				RoutableNetworks: []string{"192.168.10.0/24"},
			}
			peer := &Node{
				Hostname: "node2", MeshIP: net.ParseIP("10.99.0.2"), PublicKey: "peer-public-key",
				RoutableNetworks: []string{"192.168.20.0/24"},
			}
			m := &Mesh{
				InterfaceName: "wg0", Network: "10.99.0.0/16",
				Nodes: map[string]*Node{node.Hostname: node, peer.Hostname: peer},
			}
			if tt.groups {
				m.Groups = map[string]*Group{
					"clients": {Members: []string{node.Hostname}},
					"servers": {Members: []string{peer.Hostname}},
				}
			}
			if tt.policy {
				m.AccessPolicies = []*AccessPolicy{{
					Name: "clients-to-servers", FromGroups: []string{"clients"}, ToGroups: []string{"servers"},
					AllowMeshIPs: true, AllowRoutableNetworks: tt.allowNetworks,
				}}
			}

			var config *WireGuardConfig
			var routes []ssh.RouteEntry
			done := make(chan struct{})
			go func() {
				defer close(done)
				// Deploy holds the write lock before calling these helpers.
				m.mu.Lock()
				defer m.mu.Unlock()
				config = m.generateConfigForNode(node)
				routes = m.collectAllRoutesForNode(node)
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("deployment helpers deadlocked while the mesh lock was held")
			}

			if config.Interface.Address != "10.99.0.1/16" || len(config.Peers) != tt.wantPeers {
				t.Fatalf("unexpected configuration: %+v", config)
			}
			wantAllowedIPs := []string{"10.99.0.2/32"}
			var wantRoutes []ssh.RouteEntry
			if tt.wantNetworks {
				wantAllowedIPs = append(wantAllowedIPs, "192.168.20.0/24")
				wantRoutes = []ssh.RouteEntry{{Network: "192.168.20.0/24", Gateway: "10.99.0.2"}}
			}
			if tt.wantPeers > 0 && !reflect.DeepEqual(config.Peers[0].AllowedIPs, wantAllowedIPs) {
				t.Errorf("AllowedIPs = %v, want %v", config.Peers[0].AllowedIPs, wantAllowedIPs)
			}
			// The node's own 192.168.10.0/24 must never become a route on it.
			if len(routes) != len(wantRoutes) || (len(wantRoutes) > 0 && !reflect.DeepEqual(routes, wantRoutes)) {
				t.Errorf("routes = %v, want %v", routes, wantRoutes)
			}
			if conf := wireguard.GenerateWgQuickConfig(config, routes); strings.Contains(conf, "via  dev") {
				t.Errorf("wg-quick config has a gateway-less route:\n%s", conf)
			}
		})
	}
}

func TestOwnNetworkRoutes(t *testing.T) {
	t.Parallel()

	own := []string{"192.168.10.0/24"}
	tests := []struct {
		name    string
		current []ssh.RouteEntry
		want    []ssh.RouteEntry
	}{
		{name: "no routes"},
		{
			name:    "stale own network on mesh interface",
			current: []ssh.RouteEntry{{Network: "192.168.10.0/24"}},
			want:    []ssh.RouteEntry{{Network: "192.168.10.0/24"}},
		},
		{
			name: "mesh subnet and peer networks kept",
			current: []ssh.RouteEntry{
				{Network: "10.99.0.0/16"},
				{Network: "192.168.20.0/24", Gateway: "10.99.0.2"},
			},
		},
		{
			name:    "own network via a peer kept",
			current: []ssh.RouteEntry{{Network: "192.168.10.0/24", Gateway: "10.99.0.2"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ownNetworkRoutes(tt.current, own); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ownNetworkRoutes() = %v, want %v", got, tt.want)
			}
		})
	}
}
