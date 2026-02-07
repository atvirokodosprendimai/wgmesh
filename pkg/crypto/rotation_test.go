package crypto

import (
	"testing"
	"time"
)

func TestGenerateRotationAnnouncement(t *testing.T) {
	oldKey := []byte("old-membership-key-that-is-32b!!")
	newSecret := "new-secret-that-is-long-enough!"
	grace := 24 * time.Hour

	announcement, err := GenerateRotationAnnouncement(oldKey, newSecret, grace)
	if err != nil {
		t.Fatalf("GenerateRotationAnnouncement failed: %v", err)
	}

	if len(announcement.NewSecretHash) != 32 {
		t.Errorf("Expected 32 byte hash, got %d", len(announcement.NewSecretHash))
	}

	if announcement.GracePeriod != int64(grace.Seconds()) {
		t.Errorf("Expected grace period %d, got %d", int64(grace.Seconds()), announcement.GracePeriod)
	}

	if len(announcement.Signature) == 0 {
		t.Error("Expected non-empty signature")
	}
}

func TestValidateRotationAnnouncement(t *testing.T) {
	oldKey := []byte("old-membership-key-that-is-32b!!")
	newSecret := "new-secret-that-is-long-enough!"

	announcement, _ := GenerateRotationAnnouncement(oldKey, newSecret, time.Hour)

	// Valid announcement should pass
	if !ValidateRotationAnnouncement(oldKey, announcement) {
		t.Error("Valid announcement should pass validation")
	}

	// Wrong key should fail
	if ValidateRotationAnnouncement([]byte("wrong-key-that-is-also-32-bytes!"), announcement) {
		t.Error("Wrong key should fail validation")
	}
}

func TestVerifyNewSecret(t *testing.T) {
	oldKey := []byte("old-membership-key-that-is-32b!!")
	newSecret := "new-secret-that-is-long-enough!"

	announcement, _ := GenerateRotationAnnouncement(oldKey, newSecret, time.Hour)

	// Correct secret should verify
	if !VerifyNewSecret(newSecret, announcement) {
		t.Error("Correct secret should verify")
	}

	// Wrong secret should not verify
	if VerifyNewSecret("wrong-secret-that-is-long!", announcement) {
		t.Error("Wrong secret should not verify")
	}
}

func TestRotationState(t *testing.T) {
	state := &RotationState{
		OldSecret:   "old",
		NewSecret:   "new",
		GracePeriod: time.Hour,
		StartedAt:   time.Now(),
	}

	if !state.IsInGracePeriod() {
		t.Error("Should be in grace period")
	}

	if state.ShouldComplete() {
		t.Error("Should not be complete yet")
	}

	// Test completed state
	state.Completed = true
	if state.IsInGracePeriod() {
		t.Error("Completed state should not be in grace period")
	}
}
