package mesh

import (
	"net"
	"reflect"
	"testing"
)

// Two nodes advertising the same subnet (HA): neither may route the shared
// subnet through the mesh, while disjoint networks are still routed.
func TestSharedSubnetNeverRoutedThroughMesh(t *testing.T) {
	t.Parallel()

	const shared = "192.168.10.0/24"
	tests := []struct {
		name   string
		policy bool
	}{
		{name: "no access control"},
		{name: "policy allows routable networks", policy: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			node := &Node{
				Hostname: "a", MeshIP: net.ParseIP("10.99.0.1"), ListenPort: 51820,
				RoutableNetworks: []string{shared},
			}
			peer := &Node{
				Hostname: "b", MeshIP: net.ParseIP("10.99.0.2"), PublicKey: "peer-key",
				RoutableNetworks: []string{shared, "192.168.20.0/24"},
			}
			m := &Mesh{
				InterfaceName: "wg0", Network: "10.99.0.0/16",
				Nodes: map[string]*Node{node.Hostname: node, peer.Hostname: peer},
			}
			if tt.policy {
				m.Groups = map[string]*Group{
					"x": {Members: []string{node.Hostname}},
					"y": {Members: []string{peer.Hostname}},
				}
				m.AccessPolicies = []*AccessPolicy{{
					Name: "x-to-y", FromGroups: []string{"x"}, ToGroups: []string{"y"},
					AllowMeshIPs: true, AllowRoutableNetworks: true,
				}}
			}

			m.mu.Lock()
			config := m.generateConfigForNode(node)
			routes := m.collectAllRoutesForNode(node)
			m.mu.Unlock()

			for _, r := range routes {
				if r.Network == shared {
					t.Errorf("shared subnet routed through mesh: %+v", r)
				}
			}
			if len(routes) != 1 || routes[0].Network != "192.168.20.0/24" {
				t.Errorf("routes = %v, want only the disjoint 192.168.20.0/24", routes)
			}
			want := []string{"10.99.0.2/32", "192.168.20.0/24"}
			if len(config.Peers) != 1 || !reflect.DeepEqual(config.Peers[0].AllowedIPs, want) {
				t.Errorf("peers = %+v, want AllowedIPs %v", config.Peers, want)
			}
		})
	}
}
