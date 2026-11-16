package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
	"github.com/atvirokodosprendimai/wgmesh/pkg/node"
)

// enforceLiveness turns the process-wide switch on for one test and restores
// it off. Tests that call it do not call t.Parallel.
func enforceLiveness(t *testing.T) {
	t.Helper()
	node.EnforceLiveness.Store(true)
	t.Cleanup(func() { node.EnforceLiveness.Store(false) })
}

func disagreeCount(site string) float64 {
	return testutil.ToFloat64(daemon.LivenessDisagreeCounter(site))
}

func livenessDHT(t *testing.T) *DHTDiscovery {
	t.Helper()
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-introducer-alive-1"})
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	d, err := NewDHTDiscovery(context.Background(), cfg, &daemon.LocalNode{WGPubKey: "local"}, daemon.NewPeerStore())
	if err != nil {
		t.Fatalf("NewDHTDiscovery: %v", err)
	}
	return d
}

func TestIntroducerFreshnessUsesAlive(t *testing.T) {
	d := livenessDHT(t)
	d.mu.Lock()
	d.controlPeers["peer-i"] = "10.250.0.2:51821"
	d.mu.Unlock()

	now := time.Now()
	freshHS := map[string]int64{"peer-i": now.Add(-30 * time.Second).Unix()}
	tests := []struct {
		name          string
		enforce       bool
		lastSeen      time.Time
		lastDataPlane time.Time
		handshakes    map[string]int64
		want          bool
		wantDisagree  float64
	}{
		{"on: old LastSeen, fresh data-plane time, fresh handshake", true, now.Add(-20 * time.Minute), now.Add(-30 * time.Second), freshHS, true, 0},
		{"on: old LastSeen, no fresh evidence", true, now.Add(-20 * time.Minute), time.Time{}, freshHS, false, 0},
		{"on: old LastSeen, data-plane time past 2 minutes", true, now.Add(-20 * time.Minute), now.Add(-3 * time.Minute), freshHS, false, 0},
		{"on: fresh LastSeen, fresh handshake", true, now.Add(-30 * time.Second), time.Time{}, freshHS, true, 0},
		{"on: fresh data-plane time, no handshake in the map", true, now.Add(-20 * time.Minute), now.Add(-30 * time.Second), nil, false, 0},
		{"off: old LastSeen and a fresh data-plane time stays legacy and is counted", false, now.Add(-20 * time.Minute), now.Add(-30 * time.Second), freshHS, false, 1},
		{"off: old LastSeen, no fresh evidence, nothing counted", false, now.Add(-20 * time.Minute), time.Time{}, freshHS, false, 0},
		{"off: fresh LastSeen, fresh handshake", false, now.Add(-30 * time.Second), time.Time{}, freshHS, true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.enforce {
				enforceLiveness(t)
			}
			p := &daemon.PeerInfo{WGPubKey: "peer-i", LastSeen: tt.lastSeen, LastDataPlane: tt.lastDataPlane}
			before := disagreeCount(daemon.SiteIntroducerCandidate)

			if got := d.isAutoIntroducerCandidate(p, tt.handshakes); got != tt.want {
				t.Errorf("isAutoIntroducerCandidate = %v, want %v", got, tt.want)
			}
			if got := disagreeCount(daemon.SiteIntroducerCandidate) - before; got != tt.wantDisagree {
				t.Errorf("introducer-candidate disagreements = %v, want %v", got, tt.wantDisagree)
			}
		})
	}
}

// TestRendezvousSkipKeepsLegacyUntilEnforced runs handlePeerEvent with a
// backoff that stops it after the skip test, so the counter is what shows
// which age decided.
func TestRendezvousSkipKeepsLegacyUntilEnforced(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name          string
		enforce       bool
		lastSeen      time.Time
		lastDataPlane time.Time
		wantDisagree  float64
	}{
		{"off: old LastSeen and a fresh data-plane time stays legacy and is counted", false, now.Add(-10 * time.Minute), now.Add(-30 * time.Second), 1},
		{"on: the fresh data-plane time skips the rendezvous, nothing counted", true, now.Add(-10 * time.Minute), now.Add(-30 * time.Second), 0},
		{"off: fresh LastSeen skips as before, nothing counted", false, now.Add(-30 * time.Second), time.Time{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.enforce {
				enforceLiveness(t)
			}
			d := livenessDHT(t)
			d.peerStore.SetPeerDirectly("peer-r", &daemon.PeerInfo{
				WGPubKey: "peer-r", DiscoveredVia: []string{"dht-rendezvous"},
				LastSeen: tt.lastSeen, LastDataPlane: tt.lastDataPlane,
			})
			d.mu.Lock()
			d.rendezvousBackoff["peer-r"] = backoffEntry{nextAttempt: time.Now().Add(time.Hour)}
			d.mu.Unlock()
			before := disagreeCount(daemon.SiteRendezvousSkip)

			d.handlePeerEvent(daemon.PeerEvent{PubKey: "peer-r"})

			if got := disagreeCount(daemon.SiteRendezvousSkip) - before; got != tt.wantDisagree {
				t.Errorf("rendezvous-skip disagreements = %v, want %v", got, tt.wantDisagree)
			}
		})
	}
}
