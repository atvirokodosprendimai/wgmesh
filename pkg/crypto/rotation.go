package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// RotationAnnouncement is broadcast via gossip to coordinate secret rotation
type RotationAnnouncement struct {
	NewSecretHash []byte `json:"new_secret_hash"` // SHA256 of new secret (for verification)
	GracePeriod   int64  `json:"grace_period"`    // Duration in seconds to maintain dual-secret mode
	Timestamp     int64  `json:"timestamp"`
	Signature     []byte `json:"signature"` // HMAC-SHA256(old_membership_key, announcement)
}

// GenerateRotationAnnouncement creates a rotation announcement signed with the old secret
func GenerateRotationAnnouncement(oldMembershipKey []byte, newSecret string, gracePeriod time.Duration) (*RotationAnnouncement, error) {
	newHash := sha256.Sum256([]byte(newSecret))

	announcement := &RotationAnnouncement{
		NewSecretHash: newHash[:],
		GracePeriod:   int64(gracePeriod.Seconds()),
		Timestamp:     time.Now().Unix(),
	}

	// Sign with old membership key
	sig, err := signRotation(oldMembershipKey, announcement)
	if err != nil {
		return nil, err
	}
	announcement.Signature = sig

	return announcement, nil
}

// ValidateRotationAnnouncement validates a rotation announcement
func ValidateRotationAnnouncement(oldMembershipKey []byte, announcement *RotationAnnouncement) bool {
	// Check timestamp (within last hour)
	msgTime := time.Unix(announcement.Timestamp, 0)
	if time.Since(msgTime) > time.Hour {
		return false
	}
	if msgTime.After(time.Now().Add(time.Hour)) {
		return false
	}

	// Verify signature
	expectedSig, err := signRotation(oldMembershipKey, &RotationAnnouncement{
		NewSecretHash: announcement.NewSecretHash,
		GracePeriod:   announcement.GracePeriod,
		Timestamp:     announcement.Timestamp,
	})
	if err != nil {
		return false
	}

	return hmac.Equal(announcement.Signature, expectedSig)
}

// VerifyNewSecret checks if a new secret matches the hash in the rotation announcement
func VerifyNewSecret(newSecret string, announcement *RotationAnnouncement) bool {
	hash := sha256.Sum256([]byte(newSecret))
	return hmac.Equal(hash[:], announcement.NewSecretHash)
}

// signRotation creates an HMAC signature for a rotation announcement
func signRotation(membershipKey []byte, announcement *RotationAnnouncement) ([]byte, error) {
	data := fmt.Sprintf("%x|%d|%d", announcement.NewSecretHash, announcement.GracePeriod, announcement.Timestamp)
	mac := hmac.New(sha256.New, membershipKey)
	mac.Write([]byte(data))
	return mac.Sum(nil), nil
}

// RotationState tracks the state of an ongoing secret rotation
type RotationState struct {
	OldSecret     string    `json:"old_secret"`
	NewSecret     string    `json:"new_secret"`
	GracePeriod   time.Duration `json:"grace_period"`
	StartedAt     time.Time `json:"started_at"`
	Completed     bool      `json:"completed"`
}

// IsInGracePeriod returns true if the rotation is still in the grace period
func (rs *RotationState) IsInGracePeriod() bool {
	if rs.Completed {
		return false
	}
	return time.Since(rs.StartedAt) < rs.GracePeriod
}

// ShouldComplete returns true if the grace period has elapsed
func (rs *RotationState) ShouldComplete() bool {
	return !rs.Completed && time.Since(rs.StartedAt) >= rs.GracePeriod
}

// MarshalJSON implements json.Marshaler
func (rs *RotationState) MarshalJSON() ([]byte, error) {
	type Alias RotationState
	return json.Marshal(&struct {
		GracePeriod string `json:"grace_period"`
		*Alias
	}{
		GracePeriod: rs.GracePeriod.String(),
		Alias:       (*Alias)(rs),
	})
}
