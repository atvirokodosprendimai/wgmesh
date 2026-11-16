package daemon

import (
	"sync"
	"testing"
	"time"
)

func hasOfflineEntry(d *Daemon, key string) bool {
	d.offlineMu.Lock()
	defer d.offlineMu.Unlock()
	_, ok := d.temporaryOffline[key]
	return ok
}

func TestSweepTemporaryOfflineDropsExpiredKeepsLive(t *testing.T) {
	d := &Daemon{temporaryOffline: make(map[string]time.Time)}
	d.offlineMu.Lock()
	d.temporaryOffline["expired"] = time.Now().Add(-time.Second)
	d.offlineMu.Unlock()
	d.markTemporarilyOffline("live")

	d.sweepTemporaryOffline()

	if hasOfflineEntry(d, "expired") {
		t.Fatal("expired entry survived the sweep")
	}
	if !hasOfflineEntry(d, "live") {
		t.Fatal("live entry was swept")
	}
	if !d.isTemporarilyOffline("live") {
		t.Fatal("isTemporarilyOffline(live) = false after the sweep")
	}
}

func TestSweepTemporaryOfflineConcurrentWithMarks(t *testing.T) {
	d := &Daemon{temporaryOffline: make(map[string]time.Time)}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				d.markTemporarilyOffline("k")
				d.clearTemporarilyOffline("k")
				d.isTemporarilyOffline("k")
			}
		}()
	}
	for j := 0; j < 200; j++ {
		d.sweepTemporaryOffline()
	}
	wg.Wait()

	// Without -race the loop above asserts nothing: the map must still work.
	d.markTemporarilyOffline("after")
	d.offlineMu.Lock()
	d.temporaryOffline["old"] = time.Now().Add(-time.Second)
	d.offlineMu.Unlock()
	d.sweepTemporaryOffline()
	if !d.isTemporarilyOffline("after") {
		t.Fatal("isTemporarilyOffline(after) = false after the concurrent run")
	}
	if hasOfflineEntry(d, "old") {
		t.Fatal("expired entry survived a sweep after the concurrent run")
	}
}

func TestStaleCleanupTickRunsOfflineSweep(t *testing.T) {
	d := &Daemon{
		peerStore:        NewPeerStore(),
		temporaryOffline: map[string]time.Time{"expired": time.Now().Add(-time.Second)},
	}

	d.staleCleanupTick()

	if hasOfflineEntry(d, "expired") {
		t.Fatal("staleCleanupTick left an expired temporaryOffline entry")
	}
}
