package daemon

import (
	"testing"
	"time"
)

func TestShouldTreatPeerAsStale(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		hs        int64
		prev      uint64
		curr      uint64
		wantStale bool
	}{
		{name: "no handshake yet", hs: 0, prev: 100, curr: 100, wantStale: false},
		{name: "recent handshake", hs: now.Add(-30 * time.Second).Unix(), prev: 100, curr: 100, wantStale: false},
		{name: "stale handshake but transfer increased", hs: now.Add(-4 * time.Minute).Unix(), prev: 100, curr: 120, wantStale: false},
		{name: "stale handshake and no transfer growth", hs: now.Add(-4 * time.Minute).Unix(), prev: 120, curr: 120, wantStale: true},
		{name: "stale handshake and transfer dropped (counter reset)", hs: now.Add(-4 * time.Minute).Unix(), prev: 120, curr: 5, wantStale: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldTreatPeerAsStale(tt.hs, tt.prev, tt.curr, now)
			if got != tt.wantStale {
				t.Fatalf("shouldTreatPeerAsStale(%d, %d, %d) = %v, want %v", tt.hs, tt.prev, tt.curr, got, tt.wantStale)
			}
		})
	}
}
