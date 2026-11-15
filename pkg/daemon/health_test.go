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

func TestEvictionLeavesTombstone(t *testing.T) {
	d := &Daemon{
		config:           &Config{InterfaceName: "wgmesh-test-nonexistent"},
		peerStore:        NewPeerStore(),
		temporaryOffline: make(map[string]time.Time),
	}
	d.peerStore.Offer(Candidate{Info: &PeerInfo{WGPubKey: "victim"}, Source: "dht", Hand: FirstHand})

	d.evictPeerFromPool(&PeerInfo{WGPubKey: "victim"})
	if _, ok := d.peerStore.Get("victim"); ok {
		t.Fatal("eviction left the peer in the store")
	}

	dec := d.peerStore.Offer(Candidate{Info: &PeerInfo{WGPubKey: "victim", Introducer: true}, Source: "dht", Hand: SecondHand})
	if dec.Reason != "tombstoned" {
		t.Errorf("second-hand offer after eviction: %+v, want reason tombstoned", dec)
	}
	if got, ok := d.peerStore.Get("victim"); ok {
		t.Errorf("a mention brought the evicted peer back: %+v", got)
	}
}
