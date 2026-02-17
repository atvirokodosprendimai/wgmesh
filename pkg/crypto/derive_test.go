package crypto

import (
	"fmt"
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
	if keys.MeshPrefixV6 != keys2.MeshPrefixV6 {
		t.Error("MeshPrefixV6 is not deterministic")
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
	if len(ip1) < 6 || ip1[:6] != "10.42." {
		t.Errorf("Expected IP to start with 10.42., got %s", ip1)
	}

	// Parse and verify last octet is in [1, 254]
	var a, b, c, d int
	fmt.Sscanf(ip1, "%d.%d.%d.%d", &a, &b, &c, &d)
	if d < 1 || d > 254 {
		t.Errorf("Last octet should be in [1,254], got %d (ip=%s)", d, ip1)
	}
	if a != 10 || b != 42 {
		t.Errorf("Expected 10.42.x.y, got %s", ip1)
	}
}

func TestDeriveMeshIPUsesSubnetByte1(t *testing.T) {
	// Verify meshSubnet[1] affects the output
	ip1 := DeriveMeshIP([2]byte{42, 0}, "pubkey1", "test-secret-that-is-long-enough")
	ip2 := DeriveMeshIP([2]byte{42, 99}, "pubkey1", "test-secret-that-is-long-enough")

	if ip1 == ip2 {
		t.Error("Different meshSubnet[1] should produce different IPs")
	}

	// Both should share the same first two octets
	var a1, b1, a2, b2 int
	fmt.Sscanf(ip1, "%d.%d.", &a1, &b1)
	fmt.Sscanf(ip2, "%d.%d.", &a2, &b2)
	if a1 != a2 || b1 != b2 {
		t.Errorf("First two octets should match: %s vs %s", ip1, ip2)
	}
}

func TestDeriveMeshIPNoNetworkOrBroadcast(t *testing.T) {
	// Generate many IPs and ensure none end in .0 or .255
	meshSubnet := [2]byte{10, 20}
	for i := 0; i < 1000; i++ {
		pubkey := fmt.Sprintf("pubkey-%d", i)
		ip := DeriveMeshIP(meshSubnet, pubkey, "test-secret-that-is-long-enough")
		var a, b, c, d int
		fmt.Sscanf(ip, "%d.%d.%d.%d", &a, &b, &c, &d)
		if d == 0 || d == 255 {
			t.Errorf("Generated network/broadcast address: %s (pubkey=%s)", ip, pubkey)
		}
	}
}

func TestDeriveMeshIPv6(t *testing.T) {
	prefix := [8]byte{0xfd, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde}
	ip1 := DeriveMeshIPv6(prefix, "pubkey1", "test-secret-that-is-long-enough")
	ip2 := DeriveMeshIPv6(prefix, "pubkey2", "test-secret-that-is-long-enough")

	if ip1 == ip2 {
		t.Error("Different pubkeys should produce different mesh IPv6 addresses")
	}

	ip3 := DeriveMeshIPv6(prefix, "pubkey1", "test-secret-that-is-long-enough")
	if ip1 != ip3 {
		t.Error("Same inputs should produce same mesh IPv6")
	}

	if len(ip1) < 2 || ip1[:2] != "fd" {
		t.Errorf("Expected ULA to start with fd, got %s", ip1)
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
