package discovery

import (
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

// Stop closes the socket while the old listenLoop is still running; Start then
// replaces conn, stopCh and running. The old loop must not read those fields
// bare. Run under -race.
func TestPeerExchangeRestartWhileOldLoopDrains(t *testing.T) {
	cfg := newTestConfig(t)
	store := daemon.NewPeerStore()
	localNode := daemon.NewLocalNode("local-key", "", "", "10.0.0.1", "")
	pe := NewPeerExchange(cfg, localNode, store)

	for i := 0; i < 5; i++ {
		if err := pe.Start(); err != nil {
			t.Fatalf("Start %d: %v", i, err)
		}
		pe.Stop()
	}
	if err := pe.Start(); err != nil {
		t.Fatalf("last Start: %v", err)
	}
	pe.Stop()
}
