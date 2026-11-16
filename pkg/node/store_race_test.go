package node

import (
	"sync"
	"testing"
	"time"
)

// The store owns what it holds: a caller that keeps the PeerInfo it offered
// (handleReply hands it to the waiting ExchangeWithPeer caller) must not share
// memory with the entry later offers and SetLatency change.
func TestOfferedInfoIsNotSharedWithTheStore(t *testing.T) {
	ps := NewPeerStore()
	info := &PeerInfo{WGPubKey: "k1", MeshIP: "10.1.0.1", Endpoint: "192.0.2.1:51820"}
	ps.Offer(Candidate{Info: info, Source: "dht", Hand: FirstHand})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			ps.Offer(Candidate{Info: &PeerInfo{WGPubKey: "k1", MeshIP: "10.1.0.1", Endpoint: "198.51.100.7:51820"}, Source: "lan", Hand: FirstHand})
			ps.SetLatency("k1", time.Duration(i))
			ps.SetEndpointMethod("k1", "lan")
		}
	}()
	for i := 0; i < 300; i++ {
		_ = info.Endpoint
		_ = info.LastSeen
		_ = info.EndpointMethod
		_ = info.Latency
	}
	wg.Wait()
}
