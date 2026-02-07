package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"time"
)

// MembershipTokenSize is the size of the membership token in bytes
const MembershipTokenSize = 32

// GenerateMembershipToken creates a membership token for a given public key.
// The token proves that the node possesses the shared secret.
// Token = HMAC-SHA256(membershipKey, pubkey || hourEpoch)
func GenerateMembershipToken(membershipKey []byte, myPubkey []byte) []byte {
	hourEpoch := time.Now().UTC().Unix() / 3600
	return generateTokenForEpoch(membershipKey, myPubkey, hourEpoch)
}

// ValidateMembershipToken validates a membership token for a given public key.
// It checks the current hour and the previous hour (±1 hour tolerance).
func ValidateMembershipToken(membershipKey []byte, theirPubkey, token []byte) bool {
	if len(token) != MembershipTokenSize {
		return false
	}

	hourEpoch := time.Now().UTC().Unix() / 3600

	// Check current hour
	expected := generateTokenForEpoch(membershipKey, theirPubkey, hourEpoch)
	if hmac.Equal(token, expected) {
		return true
	}

	// Check previous hour (clock skew tolerance)
	expected = generateTokenForEpoch(membershipKey, theirPubkey, hourEpoch-1)
	if hmac.Equal(token, expected) {
		return true
	}

	// Check next hour (clock skew tolerance)
	expected = generateTokenForEpoch(membershipKey, theirPubkey, hourEpoch+1)
	return hmac.Equal(token, expected)
}

// generateTokenForEpoch generates a token for a specific hour epoch
func generateTokenForEpoch(membershipKey, pubkey []byte, hourEpoch int64) []byte {
	mac := hmac.New(sha256.New, membershipKey)
	mac.Write(pubkey)
	mac.Write([]byte(fmt.Sprintf("|%d", hourEpoch)))
	return mac.Sum(nil)
}
