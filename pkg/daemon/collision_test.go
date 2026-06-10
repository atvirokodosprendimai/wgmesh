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

	// Legacy mode (nil custom subnet, empty salt)
	ip := DeriveMeshIPWithCollisionCheck(meshSubnet, "pubkey1", secret, "", existingIPs, nil)
	if ip == "" {
		t.Error("Expected non-empty IP")
	}

	// Test with a collision
	existingIPs[ip] = "other-pubkey"
	ip2 := DeriveMeshIPWithCollisionCheck(meshSubnet, "pubkey1", secret, "", existingIPs, nil)

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

	ip := DeriveMeshIPWithCollisionCheck(meshSubnet, "pubkey1", secret, "", existingIPs, customSubnet)
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
	ip2 := DeriveMeshIPWithCollisionCheck(meshSubnet, "pubkey1", secret, "", existingIPs, customSubnet)
	if ip == ip2 {
		t.Error("Should derive different IP when collision exists")
	}
	parsed2 := net.ParseIP(ip2)
	if !customSubnet.Contains(parsed2) {
		t.Errorf("Collision-resolved IP %s not in custom subnet %s", ip2, customSubnet)
	}
}

func TestDeriveMeshIPWithSaltAndNonce(t *testing.T) {
	meshSubnet := [2]byte{42, 0}
	secret := "test-secret-that-is-long-enough"
	salt := "test-salt"

	// Salt with nonce=0 should differ from salt with nonce=1
	ip0 := DeriveMeshIPWithSaltAndNonce(meshSubnet, "pubkey1", secret, salt, 0)
	ip1 := DeriveMeshIPWithSaltAndNonce(meshSubnet, "pubkey1", secret, salt, 1)

	if ip0 == ip1 {
		t.Error("Different nonces with same salt should produce different IPs")
	}

	// Should be deterministic
	ip1b := DeriveMeshIPWithSaltAndNonce(meshSubnet, "pubkey1", secret, salt, 1)
	if ip1 != ip1b {
		t.Error("Same salt and nonce should produce same IP")
	}

	// Different salts should produce different IPs even with same nonce
	differentSalt := "different-salt"
	ipDifferentSalt := DeriveMeshIPWithSaltAndNonce(meshSubnet, "pubkey1", secret, differentSalt, 1)
	if ip1 == ipDifferentSalt {
		t.Error("Different salts should produce different IPs with same nonce")
	}
}

func TestResolveCollisionWithSalt(t *testing.T) {
	meshSubnet := [2]byte{42, 0}
	secret := "test-secret-that-is-long-enough"
	salt := "collision-test-salt"

	peer1 := &PeerInfo{WGPubKey: "aaa", MeshIP: "10.42.1.1"}
	peer2 := &PeerInfo{WGPubKey: "bbb", MeshIP: "10.42.1.1"} // Collision

	collision := CollisionInfo{
		MeshIP: "10.42.1.1",
		Peer1:  peer1,
		Peer2:  peer2,
	}

	// Resolve with salt
	newIP := ResolveCollision(collision, meshSubnet, secret, salt, nil)
	if newIP == "" {
		t.Error("Expected non-empty IP from collision resolution")
	}

	// Should differ from the collided IP
	if newIP == collision.MeshIP {
		t.Error("Resolved IP should differ from original collision IP")
	}

	// Resolution should be deterministic
	newIP2 := ResolveCollision(collision, meshSubnet, secret, salt, nil)
	if newIP != newIP2 {
		t.Error("Collision resolution with salt should be deterministic")
	}

	// Different salt should produce different resolution
	differentSalt := "different-salt"
	newIPDifferentSalt := ResolveCollision(collision, meshSubnet, secret, differentSalt, nil)
	if newIP == newIPDifferentSalt {
		t.Error("Different salts should produce different collision resolutions")
	}
}

func TestDeriveMeshIPWithCollisionCheckWithSalt(t *testing.T) {
	meshSubnet := [2]byte{42, 0}
	secret := "test-secret-that-is-long-enough"
	salt := "collision-check-salt"

	existingIPs := map[string]string{}

	// No collision - should return base derivation
	ip1 := DeriveMeshIPWithCollisionCheck(meshSubnet, "pubkey1", secret, salt, existingIPs, nil)
	if ip1 == "" {
		t.Error("Expected non-empty IP")
	}

	// Same pubkey should get same IP (no self-collision)
	ip2 := DeriveMeshIPWithCollisionCheck(meshSubnet, "pubkey1", secret, salt, existingIPs, nil)
	if ip1 != ip2 {
		t.Error("Same pubkey should get same IP with salt")
	}

	// Now create a collision
	existingIPs[ip1] = "other-pubkey"
	ip3 := DeriveMeshIPWithCollisionCheck(meshSubnet, "pubkey1", secret, salt, existingIPs, nil)
	if ip3 == "" {
		t.Error("Expected non-empty IP after collision")
	}

	// Collision-resolved IP should differ from original
	if ip1 == ip3 {
		t.Error("Collision should produce different IP with salt")
	}

	// Resolution should be deterministic
	ip4 := DeriveMeshIPWithCollisionCheck(meshSubnet, "pubkey1", secret, salt, existingIPs, nil)
	if ip3 != ip4 {
		t.Error("Collision resolution with salt should be deterministic")
	}
}
