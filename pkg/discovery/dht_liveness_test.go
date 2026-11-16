package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

func TestIntroducerFreshnessUsesAlive(t *testing.T) {
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-introducer-alive-1"})
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	d, err := NewDHTDiscovery(context.Background(), cfg, &daemon.LocalNode{WGPubKey: "local"}, daemon.NewPeerStore())
	if err != nil {
		t.Fatalf("NewDHTDiscovery: %v", err)
	}
	d.mu.Lock()
	d.controlPeers["peer-i"] = "10.250.0.2:51821"
	d.mu.Unlock()

	now := time.Now()
	freshHS := map[string]int64{"peer-i": now.Add(-30 * time.Second).Unix()}
	tests := []struct {
		name          string
		lastSeen      time.Time
		lastDataPlane time.Time
		handshakes    map[string]int64
		want          bool
	}{
		{"old LastSeen, fresh data-plane time, fresh handshake", now.Add(-20 * time.Minute), now.Add(-30 * time.Second), freshHS, true},
		{"old LastSeen, no fresh evidence", now.Add(-20 * time.Minute), time.Time{}, freshHS, false},
		{"old LastSeen, data-plane time past 2 minutes", now.Add(-20 * time.Minute), now.Add(-3 * time.Minute), freshHS, false},
		{"fresh LastSeen, fresh handshake", now.Add(-30 * time.Second), time.Time{}, freshHS, true},
		{"fresh data-plane time, no handshake in the map", now.Add(-20 * time.Minute), now.Add(-30 * time.Second), nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &daemon.PeerInfo{WGPubKey: "peer-i", LastSeen: tt.lastSeen, LastDataPlane: tt.lastDataPlane}
			if got := d.isAutoIntroducerCandidate(p, tt.handshakes); got != tt.want {
				t.Errorf("isAutoIntroducerCandidate = %v, want %v", got, tt.want)
			}
		})
	}
}
