package node

import (
	"sync"
	"testing"
)

// The enforcement switches are read by every Offer, from every discovery
// goroutine, and flipped by a release or a test: they are atomic.
func TestSwitchesFlippedWhileOffersRun(t *testing.T) {
	ps := NewPeerStore()
	old1, old2 := EnforceIPBinding.Load(), EnforceRouteBinding.Load()
	t.Cleanup(func() { EnforceIPBinding.Store(old1); EnforceRouteBinding.Store(old2) })

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			EnforceIPBinding.Store(i%2 == 0)
			EnforceRouteBinding.Store(i%2 == 1)
		}
	}()
	for i := 0; i < 200; i++ {
		ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: "k", MeshIP: "10.1.0.1", RoutableNetworks: []string{"192.168.0.0/24"}}, Source: "dht", Hand: FirstHand})
	}
	wg.Wait()
}
