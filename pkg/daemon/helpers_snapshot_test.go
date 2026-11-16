package daemon

// Words: saveLocalNode persists the key, MeshIP and MeshIPv6 from one
// Snapshot(), not from two accessor calls.
//
// Why: the pair was read with two lock acquisitions; a setter between them
// persisted an IPv4/IPv6 pair that never existed at one instant (ADR-0005 T8,
// M 2026-11-16: every access of a shared field is locked, reads too).
//
// Invariant: with SetMeshIPv6 running in a loop, every saved file parses and
// holds the fixed IPv4 and one of the written IPv6 values; clean under -race.
// The test cannot fail before the change (the accessors lock, so there was no
// data race, only a window between two reads): it pins the final shape.

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSaveLocalNodeWhileMeshIPv6Changes(t *testing.T) {
	n := NewLocalNode("pub", "priv", "", "10.0.0.1", "fd00::0")
	path := filepath.Join(t.TempDir(), "wg0.json")

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-done:
				return
			default:
			}
			n.SetMeshIPv6(fmt.Sprintf("fd00::%x", i%1000+1))
		}
	}()

	for i := 0; i < 200; i++ {
		if err := saveLocalNode(path, n); err != nil {
			close(done)
			wg.Wait()
			t.Fatal(err)
		}
		got, err := loadLocalNode(path)
		if err != nil {
			close(done)
			wg.Wait()
			t.Fatal(err)
		}
		if got.GetMeshIP() != "10.0.0.1" || !strings.HasPrefix(got.GetMeshIPv6(), "fd00::") {
			close(done)
			wg.Wait()
			t.Fatalf("save %d persisted %q / %q", i, got.GetMeshIP(), got.GetMeshIPv6())
		}
	}
	close(done)
	wg.Wait()
}
