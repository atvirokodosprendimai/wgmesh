package daemon

import (
	"testing"
	"time"
)

func TestShouldRelayPeer_IntroducerNeverRelays(t *testing.T) {
	d := &Daemon{
		config:    &Config{Introducer: true},
		localNode: &LocalNode{NATType: "symmetric"},
	}
	peer := &PeerInfo{WGPubKey: "peer1", NATType: "symmetric"}
	relays := []*PeerInfo{{WGPubKey: "relay1", Introducer: true, Endpoint: "1.2.3.4:51820"}}

	if d.shouldRelayPeer(peer, relays, nil) {
		t.Error("introducer node should never relay through others")
	}
}

func TestShouldRelayPeer_NeverRelayToIntroducer(t *testing.T) {
	d := &Daemon{
		config:    &Config{},
		localNode: &LocalNode{NATType: "symmetric"},
	}
	peer := &PeerInfo{WGPubKey: "intro1", Introducer: true, NATType: "cone"}
	relays := []*PeerInfo{{WGPubKey: "relay1", Introducer: true, Endpoint: "1.2.3.4:51820"}}

	if d.shouldRelayPeer(peer, relays, nil) {
		t.Error("should never relay to an introducer")
	}
}

func TestShouldRelayPeer_NoRelayCandidates(t *testing.T) {
	d := &Daemon{
		config:    &Config{},
		localNode: &LocalNode{NATType: "symmetric"},
	}
	peer := &PeerInfo{WGPubKey: "peer1", NATType: "symmetric"}

	if d.shouldRelayPeer(peer, nil, nil) {
		t.Error("should not relay without relay candidates")
	}
}

func TestShouldRelayPeer_BothSymmetric(t *testing.T) {
	d := &Daemon{
		config:    &Config{},
		localNode: &LocalNode{NATType: "symmetric"},
	}
	peer := &PeerInfo{WGPubKey: "peer1", NATType: "symmetric"}
	relays := []*PeerInfo{{WGPubKey: "relay1", Introducer: true, Endpoint: "1.2.3.4:51820"}}

	if !d.shouldRelayPeer(peer, relays, nil) {
		t.Error("should relay when both sides are symmetric NAT")
	}
}

func TestShouldRelayPeer_ConeNAT_NeverRelays(t *testing.T) {
	tests := []struct {
		name      string
		localNAT  string
		remoteNAT string
	}{
		{"both_cone", "cone", "cone"},
		{"local_cone_remote_symmetric", "cone", "symmetric"},
		{"local_symmetric_remote_cone", "symmetric", "cone"},
		{"both_unknown", "unknown", "unknown"},
		{"local_cone_remote_unknown", "cone", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{
				config:    &Config{},
				localNode: &LocalNode{NATType: tt.localNAT},
			}
			peer := &PeerInfo{WGPubKey: "peer1", NATType: tt.remoteNAT}
			relays := []*PeerInfo{{WGPubKey: "relay1", Introducer: true, Endpoint: "1.2.3.4:51820"}}

			// With no handshake data (new peer), should try direct first
			if d.shouldRelayPeer(peer, relays, nil) {
				t.Errorf("should not relay for NAT types local=%q remote=%q without handshake failure", tt.localNAT, tt.remoteNAT)
			}
		})
	}
}

func TestShouldRelayPeer_StaleHandshake(t *testing.T) {
	d := &Daemon{
		config:    &Config{},
		localNode: &LocalNode{NATType: "cone"},
	}
	peer := &PeerInfo{WGPubKey: "peer1", NATType: "cone"}
	relays := []*PeerInfo{{WGPubKey: "relay1", Introducer: true, Endpoint: "1.2.3.4:51820"}}

	// Handshake 5 minutes ago → stale → relay
	staleTS := time.Now().Add(-5 * time.Minute).Unix()
	handshakes := map[string]int64{"peer1": staleTS}

	if !d.shouldRelayPeer(peer, relays, handshakes) {
		t.Error("should relay when WG handshake is stale (>2 min)")
	}
}

func TestShouldRelayPeer_RecentHandshake(t *testing.T) {
	d := &Daemon{
		config:    &Config{},
		localNode: &LocalNode{NATType: "symmetric"},
	}
	peer := &PeerInfo{WGPubKey: "peer1", NATType: "symmetric"}
	relays := []*PeerInfo{{WGPubKey: "relay1", Introducer: true, Endpoint: "1.2.3.4:51820"}}

	// Handshake 30 seconds ago → fresh → don't relay even though both symmetric
	recentTS := time.Now().Add(-30 * time.Second).Unix()
	handshakes := map[string]int64{"peer1": recentTS}

	if d.shouldRelayPeer(peer, relays, handshakes) {
		t.Error("should not relay when WG handshake is recent (<2 min)")
	}
}

func TestShouldRelayPeer_NoHandshakeYet(t *testing.T) {
	d := &Daemon{
		config:    &Config{},
		localNode: &LocalNode{NATType: "cone"},
	}
	peer := &PeerInfo{WGPubKey: "peer1", NATType: "cone"}
	relays := []*PeerInfo{{WGPubKey: "relay1", Introducer: true, Endpoint: "1.2.3.4:51820"}}

	// Peer exists in handshakes but timestamp is 0 (no handshake yet)
	handshakes := map[string]int64{"peer1": 0}

	// Should try direct first — no evidence of unreachability yet
	if d.shouldRelayPeer(peer, relays, handshakes) {
		t.Error("should try direct first when no handshake has occurred yet")
	}
}

func TestPeerStoreUpdate_MergesNATType(t *testing.T) {
	ps := NewPeerStore()

	// First update — no NATType
	ps.Update(&PeerInfo{
		WGPubKey: "pk1",
		MeshIP:   "10.0.0.1",
		Endpoint: "1.2.3.4:51820",
	}, "dht")

	got, ok := ps.Get("pk1")
	if !ok {
		t.Fatal("peer not found")
	}
	if got.NATType != "" {
		t.Errorf("NATType = %q, want empty", got.NATType)
	}

	// Second update — with NATType
	ps.Update(&PeerInfo{
		WGPubKey: "pk1",
		MeshIP:   "10.0.0.1",
		NATType:  "symmetric",
	}, "gossip")

	got, _ = ps.Get("pk1")
	if got.NATType != "symmetric" {
		t.Errorf("NATType = %q, want symmetric", got.NATType)
	}

	// Third update — empty NATType shouldn't overwrite
	ps.Update(&PeerInfo{
		WGPubKey: "pk1",
		MeshIP:   "10.0.0.1",
	}, "dht")

	got, _ = ps.Get("pk1")
	if got.NATType != "symmetric" {
		t.Errorf("NATType = %q, want symmetric (should not be overwritten by empty)", got.NATType)
	}
}
