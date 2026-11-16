package discovery

import (
	"context"
	"sync"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

// Start assigns gossip, lan, server and dhtPort while Stop, called from another
// goroutine, reads them. The writer below does what Start does, with the log
// calls left out (a log call is a lock that would hide the race from -race).
// No access may be bare. Run under -race.
func TestStopWhileStartAssignsFields(t *testing.T) {
	cfg := newTestConfig(t)
	store := daemon.NewPeerStore()
	localNode := &daemon.LocalNode{WGPubKey: "local-key", MeshIP: "10.0.0.1"}
	d, err := NewDHTDiscovery(context.Background(), cfg, localNode, store)
	if err != nil {
		t.Fatal(err)
	}
	g, err := NewMeshGossipWithExchange(cfg, localNode, store, NewPeerExchange(cfg, localNode, store))
	if err != nil {
		t.Fatal(err)
	}
	d.exchange = nil

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			d.gossip = g
			d.lan = nil
			d.server = nil
			d.dhtPort = i
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			d.mu.Lock()
			d.running = true
			d.mu.Unlock()
			_ = d.Stop()
		}
	}()
	wg.Wait()
}
