package daemon

import (
	"net"
	"testing"
)

func TestDetectCollisions(t *testing.T) {
	ps := NewPeerStore()

	// No collisions with empty store
	collisions := DetectCollisions(ps)
	if len(collisions) != 0 {
		t.Errorf("Expected 0 collisions, got %d", len(collisions))
	}

	ps.Update(&PeerInfo{WGPubKey: "key1", MeshIP: "10.0.0.1"}, "test")
	ps.Update(&PeerInfo{WGPubKey: "key2", MeshIP: "10.0.0.2"}, "test")

	collisions = DetectCollisions(ps)
	if len(collisions) != 0 {
		t.Errorf("Expected 0 collisions, got %d", len(collisions))
	}

	ps.Update(&PeerInfo{WGPubKey: "key3", MeshIP: "10.0.0.1"}, "test")

	collisions = DetectCollisions(ps)
	if len(collisions) != 1 {
		t.Errorf("Expected 1 collision, got %d", len(collisions))
	}

	if len(collisions) > 0 && collisions[0].MeshIP != "10.0.0.1" {
		t.Errorf("Expected collision on 10.0.0.1, got %s", collisions[0].MeshIP)
	}
}

func TestDeterministicWinner(t *testing.T) {
	peer1 := &PeerInfo{WGPubKey: "aaa"}
	peer2 := &PeerInfo{WGPubKey: "bbb"}

	winner, loser := DeterministicWinner(peer1, peer2)
	if winner.WGPubKey != "aaa" {
		t.Error("Lower pubkey should win")
	}
	if loser.WGPubKey != "bbb" {
		t.Error("Higher pubkey should lose")
	}

	// Test reverse order
	winner, loser = DeterministicWinner(peer2, peer1)
	if winner.WGPubKey != "aaa" {
		t.Error("Lower pubkey should win regardless of order")
	}
}

func TestDeriveMeshIPWithNonce(t *testing.T) {
	meshSubnet := [2]byte{42, 0}

	ip0 := DeriveMeshIPWithNonce(meshSubnet, "pubkey", "secret-that-is-long-enough!", 0)
	ip1 := DeriveMeshIPWithNonce(meshSubnet, "pubkey", "secret-that-is-long-enough!", 1)

	if ip0 == ip1 {
		t.Error("Different nonces should produce different IPs")
	}

	// Should be deterministic
	ip1b := DeriveMeshIPWithNonce(meshSubnet, "pubkey", "secret-that-is-long-enough!", 1)
	if ip1 != ip1b {
		t.Error("Same nonce should produce same IP")
	}
}

func TestDeriveMeshIPWithCollisionCheck(t *testing.T) {
	meshSubnet := [2]byte{42, 0}
	secret := "test-secret-that-is-long-enough"

	existingIPs := map[string]string{} // No existing IPs

	// Legacy mode (nil custom subnet)
	ip := DeriveMeshIPWithCollisionCheck(meshSubnet, "pubkey1", secret, existingIPs, nil)
	if ip == "" {
		t.Error("Expected non-empty IP")
	}

	// Test with a collision
	existingIPs[ip] = "other-pubkey"
	ip2 := DeriveMeshIPWithCollisionCheck(meshSubnet, "pubkey1", secret, existingIPs, nil)

	// Should get a different IP due to nonce
	if ip == ip2 {
		t.Error("Should derive different IP when collision exists")
	}
}

func TestDeriveMeshIPWithCollisionCheckCustomSubnet(t *testing.T) {
	meshSubnet := [2]byte{42, 0}
	secret := "test-secret-that-is-long-enough"
	_, customSubnet, _ := net.ParseCIDR("192.168.100.0/24")

	existingIPs := map[string]string{}

	ip := DeriveMeshIPWithCollisionCheck(meshSubnet, "pubkey1", secret, existingIPs, customSubnet)
	if ip == "" {
		t.Error("Expected non-empty IP")
	}

	// Must be in custom subnet
	parsed := net.ParseIP(ip)
	if !customSubnet.Contains(parsed) {
		t.Errorf("IP %s not in custom subnet %s", ip, customSubnet)
	}

	// Test collision within custom subnet
	existingIPs[ip] = "other-pubkey"
	ip2 := DeriveMeshIPWithCollisionCheck(meshSubnet, "pubkey1", secret, existingIPs, customSubnet)
	if ip == ip2 {
		t.Error("Should derive different IP when collision exists")
	}
	parsed2 := net.ParseIP(ip2)
	if !customSubnet.Contains(parsed2) {
		t.Errorf("Collision-resolved IP %s not in custom subnet %s", ip2, customSubnet)
	}
}
