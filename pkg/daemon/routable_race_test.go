package daemon

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// A SIGHUP reload replaces localNode.RoutableNetworks under configMu while
// reconcile reads it to leave out the node's own networks. Run under -race.
func TestReloadConcurrentWithOwnNetworkReads(t *testing.T) {
	d := makeRelayTestDaemon()
	d.config.InterfaceName = "wg0"
	d.localNode.SetRoutableNetworks([]string{"192.168.10.0/24"})
	peers := []*PeerInfo{{
		WGPubKey: "peer1", MeshIP: "10.0.0.20", LastSeen: time.Now(),
		RoutableNetworks: []string{"192.168.10.0/24", "192.168.20.0/24"},
	}}

	const iterations = 200
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			routes := []string{"192.168.10.0/24"}
			if i%2 == 0 {
				routes = []string{"192.168.30.0/24"}
			}
			d.reloadConfig(DaemonOpts{AdvertiseRoutes: routes})
		}
	}()

	mock := &MockCommandExecutor{
		commandFunc: func(name string, args ...string) Command { return &MockCommand{} },
	}
	withMockExecutor(t, mock, func() {
		for i := 0; i < iterations; i++ {
			d.buildDesiredPeerConfigsWithHandshakes(peers, nil)
			if runtime.GOOS == "linux" {
				if err := d.syncPeerRoutes(peers); err != nil {
					t.Errorf("syncPeerRoutes: %v", err)
				}
			}
		}
	})
	wg.Wait()
}
