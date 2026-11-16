package mesh

import (
	"strings"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh"
	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh/sshtest"
)

// syncWithRoutes runs syncRoutesForNode for a node whose own network is
// 10.1.0.0/24 and whose `ip route show dev wg0` answers current. It returns
// the commands the node received.
func syncWithRoutes(t *testing.T, current string) []string {
	t.Helper()
	s := sshtest.Start(t, map[string]sshtest.Reply{
		"ip route show dev wg0": {Out: current},
		"ip -4 -o addr show to 10.1.0.0/24": {
			Out: "2: eth0    inet 10.1.0.5/24 brd 10.1.0.255 scope global eth0\n",
		},
	})
	client, err := ssh.NewClient(s.Host, s.Port)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	m := newTestMesh(tnode{name: "a", host: s.Host, port: s.Port})
	node := m.Nodes["a"]
	node.RoutableNetworks = []string{"10.1.0.0/24"}
	if err := m.syncRoutesForNode(client, node, nil); err != nil {
		t.Fatalf("syncRoutesForNode: %v", err)
	}
	return s.Commands()
}

// indexAfter returns the index of the first command at or after from that
// starts with prefix, or -1.
func indexAfter(cmds []string, from int, prefix string) int {
	for i := from; i < len(cmds); i++ {
		if strings.HasPrefix(cmds[i], prefix) {
			return i
		}
	}
	return -1
}

func countPrefix(cmds []string, prefix string) int {
	n := 0
	for _, c := range cmds {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

const (
	addrQuery   = "ip -4 -o addr show to 10.1.0.0/24"
	connectedIP = "ip route add 10.1.0.0/24 dev eth0 proto kernel scope link src 10.1.0.5"
)

func TestSyncRoutesRestoresConnectedRouteForGatewayedOwnNetwork(t *testing.T) {
	cmds := syncWithRoutes(t, "10.1.0.0/24 via 10.99.0.3 dev wg0\n")

	del := indexAfter(cmds, 0, "ip route del 10.1.0.0/24 via 10.99.0.3 dev wg0")
	if del < 0 {
		t.Fatalf("no removal of the gatewayed route in %q", cmds)
	}
	q := indexAfter(cmds, del+1, addrQuery)
	if q < 0 {
		t.Fatalf("no address query after the removal in %q", cmds)
	}
	if indexAfter(cmds, q+1, connectedIP) < 0 {
		t.Errorf("no connected route add after the address query in %q", cmds)
	}
}

func TestSyncRoutesRestoresOwnNetworkOnceWhenRemovedTwice(t *testing.T) {
	// One network twice in toRemove: the stale gateway-less route and the
	// gatewayed one from the diff.
	cmds := syncWithRoutes(t, "10.1.0.0/24 dev wg0 proto kernel scope link src 10.1.0.5\n10.1.0.0/24 via 10.99.0.3 dev wg0 proto static\n")

	if got := countPrefix(cmds, addrQuery); got != 1 {
		t.Errorf("address queries = %d, want 1, in %q", got, cmds)
	}
	if got := countPrefix(cmds, connectedIP); got != 1 {
		t.Errorf("connected route adds = %d, want 1, in %q", got, cmds)
	}
}

func TestSyncRoutesRestoresAfterFailedAdd(t *testing.T) {
	s := sshtest.Start(t, map[string]sshtest.Reply{
		"ip route show dev wg0":             {Out: "10.1.0.0/24 via 10.99.0.3 dev wg0 proto static\n"},
		"ip route add":                      {Exit: 1},
		"ip -4 -o addr show to 10.1.0.0/24": {Out: "2: eth0    inet 10.1.0.5/24 brd 10.1.0.255 scope global eth0\n"},
	})
	client, err := ssh.NewClient(s.Host, s.Port)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	m := newTestMesh(tnode{name: "a", host: s.Host, port: s.Port})
	node := m.Nodes["a"]
	node.RoutableNetworks = []string{"10.1.0.0/24"}
	desired := []ssh.RouteEntry{{Network: "192.168.50.0/24", Gateway: "10.99.0.4"}}

	if err := m.syncRoutesForNode(client, node, desired); err == nil {
		t.Fatal("syncRoutesForNode returned nil, want the failed add")
	}

	cmds := s.Commands()
	del := indexAfter(cmds, 0, "ip route del 10.1.0.0/24 via 10.99.0.3 dev wg0")
	if del < 0 {
		t.Fatalf("no removal of the gatewayed route in %q", cmds)
	}
	q := indexAfter(cmds, del+1, addrQuery)
	if q < 0 {
		t.Fatalf("no address query after the removal in %q", cmds)
	}
	if indexAfter(cmds, q+1, connectedIP) < 0 {
		t.Errorf("no connected route add after the address query in %q", cmds)
	}
}

func TestSyncRoutesDoesNotRestoreForeignNetwork(t *testing.T) {
	cmds := syncWithRoutes(t, "10.5.0.0/24 via 10.99.0.3 dev wg0\n")

	if indexAfter(cmds, 0, "ip route del 10.5.0.0/24 via 10.99.0.3 dev wg0") < 0 {
		t.Fatalf("no removal of the foreign route in %q", cmds)
	}
	if got := countPrefix(cmds, "ip -4 -o addr show"); got != 0 {
		t.Errorf("address queries = %d, want 0, in %q", got, cmds)
	}
}
