package privacy

import (
	"testing"
)

func TestShouldFluff(t *testing.T) {
	// Max hops should always fluff
	if !ShouldFluff(MaxStemHops) {
		t.Error("Should always fluff at max hops")
	}
	if !ShouldFluff(MaxStemHops + 1) {
		t.Error("Should always fluff beyond max hops")
	}

	// Test probability distribution (statistical test)
	fluffCount := 0
	iterations := 10000
	for i := 0; i < iterations; i++ {
		if ShouldFluff(1) {
			fluffCount++
		}
	}

	// With 10% probability, expect ~1000 fluffs ±300
	ratio := float64(fluffCount) / float64(iterations)
	if ratio < 0.05 || ratio > 0.20 {
		t.Errorf("Fluff probability out of expected range: %.2f (expected ~0.10)", ratio)
	}
}

func TestSelectRelayPeers(t *testing.T) {
	seed := [32]byte{1, 2, 3, 4, 5}
	peers := []PeerInfo{
		{WGPubKey: "peer-a", MeshIP: "10.0.0.1"},
		{WGPubKey: "peer-b", MeshIP: "10.0.0.2"},
		{WGPubKey: "peer-c", MeshIP: "10.0.0.3"},
	}

	// Deterministic selection
	relay1 := selectRelayPeers(seed, 1, peers, 2)
	relay2 := selectRelayPeers(seed, 1, peers, 2)

	if len(relay1) != 2 || len(relay2) != 2 {
		t.Fatalf("Expected 2 relay peers, got %d and %d", len(relay1), len(relay2))
	}

	if relay1[0].WGPubKey != relay2[0].WGPubKey || relay1[1].WGPubKey != relay2[1].WGPubKey {
		t.Error("Relay selection should be deterministic")
	}

	// Different epoch should produce different selection (usually)
	relay3 := selectRelayPeers(seed, 2, peers, 2)
	// Note: there's a small chance they could be the same, so we don't assert inequality
	_ = relay3
}

func TestSelectRelayPeersEmptyPeers(t *testing.T) {
	seed := [32]byte{1, 2, 3}
	result := selectRelayPeers(seed, 1, nil, 2)
	if result != nil {
		t.Error("Expected nil for empty peers")
	}
}

func TestSelectRelayPeersMoreCountThanPeers(t *testing.T) {
	seed := [32]byte{1, 2, 3}
	peers := []PeerInfo{
		{WGPubKey: "peer-a", MeshIP: "10.0.0.1"},
	}

	result := selectRelayPeers(seed, 1, peers, 5)
	if len(result) != 1 {
		t.Errorf("Expected 1 relay peer (capped at peer count), got %d", len(result))
	}
}

func TestDandelionRouterHandleAnnounce(t *testing.T) {
	seed := [32]byte{1, 2, 3, 4, 5}
	router := NewDandelionRouter(seed)

	var fluffed bool
	router.SetFluffHandler(func(msg DandelionAnnounce) {
		fluffed = true
	})

	// Test with max hops (should always fluff)
	msg := DandelionAnnounce{
		OriginPubkey: "test-pubkey-long-enough-to-slice",
		HopCount:     MaxStemHops - 1, // Will become MaxStemHops after increment
	}
	router.HandleAnnounce(msg)
	if !fluffed {
		t.Error("Should have fluffed at max hops")
	}
}

func TestDandelionRouterRotateEpoch(t *testing.T) {
	seed := [32]byte{1, 2, 3, 4, 5}
	router := NewDandelionRouter(seed)

	peers := []PeerInfo{
		{WGPubKey: "peer-a", MeshIP: "10.0.0.1"},
		{WGPubKey: "peer-b", MeshIP: "10.0.0.2"},
		{WGPubKey: "peer-c", MeshIP: "10.0.0.3"},
	}

	router.RotateEpoch(peers)
	epoch := router.GetEpoch()

	if epoch.ID != 1 {
		t.Errorf("Expected epoch ID 1, got %d", epoch.ID)
	}
	if len(epoch.RelayPeers) != 2 {
		t.Errorf("Expected 2 relay peers, got %d", len(epoch.RelayPeers))
	}
}

func TestCreateAnnounce(t *testing.T) {
	msg := CreateAnnounce("pubkey", "10.0.0.1", "1.2.3.4:51820", []string{"192.168.0.0/24"})

	if msg.OriginPubkey != "pubkey" {
		t.Error("Wrong pubkey")
	}
	if msg.HopCount != 0 {
		t.Error("Initial hop count should be 0")
	}
	if len(msg.Nonce) != 16 {
		t.Errorf("Expected 16 byte nonce, got %d", len(msg.Nonce))
	}
	if msg.Timestamp == 0 {
		t.Error("Timestamp should be set")
	}
}

func TestNeedsEpochRotation(t *testing.T) {
	seed := [32]byte{1, 2, 3}
	router := NewDandelionRouter(seed)

	// Fresh epoch should not need rotation
	if router.NeedsEpochRotation() {
		t.Error("Fresh epoch should not need rotation")
	}
}

func TestFormatEpochInfo(t *testing.T) {
	seed := [32]byte{1, 2, 3}
	router := NewDandelionRouter(seed)

	info := router.FormatEpochInfo()
	if info == "" {
		t.Error("Expected non-empty epoch info")
	}
}
