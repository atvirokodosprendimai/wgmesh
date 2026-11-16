package daemon

import (
	"path/filepath"
	"sync"
	"testing"
)

// The local node is shared: collision resolution moves its mesh IP while the
// peer store reads the local claim (Policy.Local) and discovery reads the node
// to build announcements. Run under -race.
func TestMeshIPMoveIsSafeAgainstPolicyLocal(t *testing.T) {
	enforceIPBinding(t, false)
	recordAddresses(t)

	d := collisionDaemon(t, "zzz-local", "10.42.1.1")
	d.stateFile = filepath.Join(t.TempDir(), "wg0.json")
	d.peerStore.Update(&PeerInfo{WGPubKey: "aaa-remote", MeshIP: "10.42.1.1"}, "test")
	local := d.admitPolicy().Local

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		d.CheckAndResolveCollisions()
	}()
	for i := 0; i < 1000; i++ {
		local()
	}
	wg.Wait()
	if _, ip := local(); ip == "10.42.1.1" {
		t.Fatalf("the local node did not move off the contested address")
	}
}
