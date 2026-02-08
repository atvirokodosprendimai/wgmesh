package crypto

import (
	"testing"
	"time"
)

func TestDeriveKeys(t *testing.T) {
	secret := "test-secret-that-is-long-enough"

	keys, err := DeriveKeys(secret)
	if err != nil {
		t.Fatalf("DeriveKeys failed: %v", err)
	}

	// Verify deterministic derivation
	keys2, err := DeriveKeys(secret)
	if err != nil {
		t.Fatalf("DeriveKeys failed second time: %v", err)
	}

	if keys.NetworkID != keys2.NetworkID {
		t.Error("NetworkID is not deterministic")
	}
	if keys.GossipKey != keys2.GossipKey {
		t.Error("GossipKey is not deterministic")
	}
	if keys.MeshSubnet != keys2.MeshSubnet {
		t.Error("MeshSubnet is not deterministic")
	}
	if keys.PSK != keys2.PSK {
		t.Error("PSK is not deterministic")
	}
	if keys.RendezvousID != keys2.RendezvousID {
		t.Error("RendezvousID is not deterministic")
	}
	if keys.MembershipKey != keys2.MembershipKey {
		t.Error("MembershipKey is not deterministic")
	}
	if keys.EpochSeed != keys2.EpochSeed {
		t.Error("EpochSeed is not deterministic")
	}
	if keys.GossipPort != keys2.GossipPort {
		t.Error("GossipPort is not deterministic")
	}
}

func TestDeriveKeysMinLength(t *testing.T) {
	_, err := DeriveKeys("short")
	if err == nil {
		t.Error("Expected error for short secret")
	}
}

func TestDeriveKeysDifferentSecrets(t *testing.T) {
	keys1, _ := DeriveKeys("secret-one-that-is-long-enough")
	keys2, _ := DeriveKeys("secret-two-that-is-long-enough")

	if keys1.NetworkID == keys2.NetworkID {
		t.Error("Different secrets produced same NetworkID")
	}
	if keys1.GossipKey == keys2.GossipKey {
		t.Error("Different secrets produced same GossipKey")
	}
	if keys1.RendezvousID == keys2.RendezvousID {
		t.Error("Different secrets produced same RendezvousID")
	}
	if keys1.MembershipKey == keys2.MembershipKey {
		t.Error("Different secrets produced same MembershipKey")
	}
}

func TestDeriveNetworkIDWithTime(t *testing.T) {
	secret := "test-secret-that-is-long-enough"

	now := time.Now().UTC()
	id1, err := DeriveNetworkIDWithTime(secret, now)
	if err != nil {
		t.Fatalf("DeriveNetworkIDWithTime failed: %v", err)
	}

	// Same hour should produce same ID
	id2, err := DeriveNetworkIDWithTime(secret, now.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("DeriveNetworkIDWithTime failed: %v", err)
	}

	// They should be the same within the same hour (if we're not near a boundary)
	// Use the beginning of the hour to be safe
	hourStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC)
	id3, _ := DeriveNetworkIDWithTime(secret, hourStart)
	id4, _ := DeriveNetworkIDWithTime(secret, hourStart.Add(30*time.Minute))

	if id3 != id4 {
		t.Error("Same hour should produce same network ID")
	}

	// Different hour should produce different ID
	id5, _ := DeriveNetworkIDWithTime(secret, hourStart.Add(-1*time.Hour))
	if id3 == id5 {
		t.Error("Different hours should produce different network IDs")
	}

	_ = id1
	_ = id2
}

func TestGetCurrentAndPreviousNetworkIDs(t *testing.T) {
	secret := "test-secret-that-is-long-enough"

	current, previous, err := GetCurrentAndPreviousNetworkIDs(secret)
	if err != nil {
		t.Fatalf("GetCurrentAndPreviousNetworkIDs failed: %v", err)
	}

	// Current and previous should be different
	if current == previous {
		t.Error("Current and previous network IDs should be different")
	}

	// Both should be non-zero
	var zero [20]byte
	if current == zero {
		t.Error("Current network ID is zero")
	}
	if previous == zero {
		t.Error("Previous network ID is zero")
	}
}

func TestDeriveMeshIP(t *testing.T) {
	meshSubnet := [2]byte{42, 0}
	ip1 := DeriveMeshIP(meshSubnet, "pubkey1", "test-secret-that-is-long-enough")
	ip2 := DeriveMeshIP(meshSubnet, "pubkey2", "test-secret-that-is-long-enough")

	// Different pubkeys should produce different IPs
	if ip1 == ip2 {
		t.Error("Different pubkeys should produce different mesh IPs")
	}

	// Same inputs should produce same IP
	ip3 := DeriveMeshIP(meshSubnet, "pubkey1", "test-secret-that-is-long-enough")
	if ip1 != ip3 {
		t.Error("Same inputs should produce same mesh IP")
	}

	// IP should start with 10.42.
	if ip1[:6] != "10.42." {
		t.Errorf("Expected IP to start with 10.42., got %s", ip1)
	}
}

func TestGossipPortRange(t *testing.T) {
	// Test that gossip port is in expected range
	keys, _ := DeriveKeys("test-secret-that-is-long-enough")
	if keys.GossipPort < 51821 || keys.GossipPort >= 51821+1000 {
		t.Errorf("GossipPort %d outside expected range [51821, 52821)", keys.GossipPort)
	}
}

func TestRendezvousIDLength(t *testing.T) {
	keys, _ := DeriveKeys("test-secret-that-is-long-enough")
	// RendezvousID should be 8 bytes
	if len(keys.RendezvousID) != 8 {
		t.Errorf("RendezvousID should be 8 bytes, got %d", len(keys.RendezvousID))
	}

	// Should be non-zero
	var zero [8]byte
	if keys.RendezvousID == zero {
		t.Error("RendezvousID should not be zero")
	}
}
