package discovery

import (
	"net"
	"sync"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

// A SIGHUP reload replaces the local node's RoutableNetworks while discovery
// layers build announcements from the same *LocalNode. Run under -race.
func TestSetRoutableNetworksConcurrentWithSendAnnounce(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-routable-race-1"})
	if err != nil {
		t.Fatal(err)
	}
	localNode := &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}
	localNode.SetRoutableNetworks([]string{"192.168.10.0/24"})

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	pe := NewPeerExchange(cfg, localNode, daemon.NewPeerStore())
	pe.conn = conn
	target := conn.LocalAddr().(*net.UDPAddr)

	const iterations = 200
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			routes := []string{"192.168.10.0/24"}
			if i%2 == 0 {
				routes = []string{"192.168.30.0/24", "192.168.40.0/24"}
			}
			localNode.SetRoutableNetworks(routes)
		}
	}()

	for i := 0; i < iterations; i++ {
		if err := pe.SendAnnounce(target); err != nil {
			t.Errorf("SendAnnounce: %v", err)
		}
	}
	wg.Wait()
}
