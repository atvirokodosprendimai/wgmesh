package daemon

// Words: reconcile() runs from reconcileLoop and from handleSIGHUP, two
// goroutines; CheckAndResolveCollisions inside it is check-then-set (read the
// address, move, set the interface), so reconcileMu serializes a whole reconcile.
//
// Why: two concurrent runs could both see the collision and both move the
// local node, setting the interface address twice (ADR-0005 T8, M 2026-11-16).
//
// Invariant: of N concurrent reconciles on one collision, exactly one move;
// clean under -race.

import (
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestCheckAndResolveCollisionsIsSerialized(t *testing.T) {
	enforceIPBinding(t, false)

	// The seam is set once here, before any goroutine starts, and restored
	// after they end; the counter is atomic.
	var applied atomic.Int64
	old := applyInterfaceAddress.Load()
	seam := func(name, address string) error {
		applied.Add(1)
		return nil
	}
	applyInterfaceAddress.Store(&seam)
	t.Cleanup(func() { applyInterfaceAddress.Store(old) })

	const rounds, runners = 40, 4
	for round := 0; round < rounds; round++ {
		applied.Store(0)
		d := collisionDaemon(t, "zzz-local", "10.42.1.1")
		d.stateFile = filepath.Join(t.TempDir(), "wg0.json")
		d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-remote", MeshIP: "10.42.1.1"}, "test")

		start := make(chan struct{})
		var wg sync.WaitGroup
		for g := 0; g < runners; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				d.reconcile()
			}()
		}
		close(start)
		wg.Wait()

		if got := applied.Load(); got != 1 {
			t.Fatalf("round %d: the interface address was set %d times, want exactly one move", round, got)
		}
		if d.localNode.GetMeshIP() != nonceOne(d) {
			t.Fatalf("round %d: local IP = %s, want the nonce-1 address %s", round, d.localNode.GetMeshIP(), nonceOne(d))
		}
	}
}
