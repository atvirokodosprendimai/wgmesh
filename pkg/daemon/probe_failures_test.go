package daemon

import "testing"

// A probe pass that took its peer snapshot before removePeer can write
// probeFailures[K] after removePeer deleted it; K then has no session. The
// next cleanup must drop it, or one entry stays per race.
func TestCleanupProbeSessionsDropsFailuresOfInactiveKeysWithoutSession(t *testing.T) {
	d := &Daemon{probeFailures: map[string]int{"gone": 3, "active": 2}}

	d.cleanupProbeSessions(map[string]struct{}{"active": {}})

	d.probeMu.Lock()
	defer d.probeMu.Unlock()
	if _, ok := d.probeFailures["gone"]; ok {
		t.Fatal("probeFailures kept a key that has no session and is not active")
	}
	if d.probeFailures["active"] != 2 {
		t.Fatalf("probeFailures[active] = %d, want 2", d.probeFailures["active"])
	}
}
