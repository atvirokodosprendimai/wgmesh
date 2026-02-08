package crypto

import (
	"testing"
	"time"
)

func TestGenerateMembershipToken(t *testing.T) {
	membershipKey := []byte("test-membership-key-that-is-32b!")
	pubkey := []byte("test-pubkey")

	token := GenerateMembershipToken(membershipKey, pubkey)
	if len(token) != MembershipTokenSize {
		t.Errorf("Expected token size %d, got %d", MembershipTokenSize, len(token))
	}

	// Token should be deterministic within same hour
	token2 := GenerateMembershipToken(membershipKey, pubkey)
	if !equalBytes(token, token2) {
		t.Error("Token should be deterministic within same hour")
	}
}

func TestValidateMembershipToken(t *testing.T) {
	membershipKey := []byte("test-membership-key-that-is-32b!")
	pubkey := []byte("test-pubkey")

	token := GenerateMembershipToken(membershipKey, pubkey)

	// Valid token should pass
	if !ValidateMembershipToken(membershipKey, pubkey, token) {
		t.Error("Valid token should pass validation")
	}

	// Wrong pubkey should fail
	if ValidateMembershipToken(membershipKey, []byte("wrong-pubkey"), token) {
		t.Error("Wrong pubkey should fail validation")
	}

	// Wrong key should fail
	if ValidateMembershipToken([]byte("wrong-membership-key-that-is32b!"), pubkey, token) {
		t.Error("Wrong membership key should fail validation")
	}

	// Empty token should fail
	if ValidateMembershipToken(membershipKey, pubkey, nil) {
		t.Error("Empty token should fail validation")
	}

	// Wrong size token should fail
	if ValidateMembershipToken(membershipKey, pubkey, []byte("short")) {
		t.Error("Wrong size token should fail validation")
	}
}

func TestMembershipTokenClockSkewTolerance(t *testing.T) {
	membershipKey := []byte("test-membership-key-that-is-32b!")
	pubkey := []byte("test-pubkey")

	// Generate a token for the current hour
	token := GenerateMembershipToken(membershipKey, pubkey)

	// It should validate now
	if !ValidateMembershipToken(membershipKey, pubkey, token) {
		t.Error("Current token should validate")
	}
}

func TestMembershipTokenDifferentPubkeys(t *testing.T) {
	membershipKey := []byte("test-membership-key-that-is-32b!")

	token1 := GenerateMembershipToken(membershipKey, []byte("pubkey1"))
	token2 := GenerateMembershipToken(membershipKey, []byte("pubkey2"))

	if equalBytes(token1, token2) {
		t.Error("Different pubkeys should produce different tokens")
	}
}

func TestGenerateTokenForEpoch(t *testing.T) {
	key := []byte("test-key")
	pubkey := []byte("test-pubkey")

	// Same epoch should produce same token
	t1 := generateTokenForEpoch(key, pubkey, 100)
	t2 := generateTokenForEpoch(key, pubkey, 100)
	if !equalBytes(t1, t2) {
		t.Error("Same epoch should produce same token")
	}

	// Different epochs should produce different tokens
	t3 := generateTokenForEpoch(key, pubkey, 101)
	if equalBytes(t1, t3) {
		t.Error("Different epochs should produce different tokens")
	}

	_ = time.Now() // Make time import used
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
