package crypto

import (
	"encoding/json"
	"strings"
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

func TestRotationStateMarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		state       *RotationState
		wantErr     bool
		checkFields bool
	}{
		{
			name: "valid state with 90 minute grace period",
			state: &RotationState{
				OldSecret:   "old-secret",
				NewSecret:   "new-secret",
				GracePeriod: 90 * time.Minute,
				StartedAt:   time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Completed:   false,
			},
			checkFields: true,
		},
		{
			name: "completed state with 24 hour grace period",
			state: &RotationState{
				OldSecret:   "old",
				NewSecret:   "new",
				GracePeriod: 24 * time.Hour,
				StartedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Completed:   true,
			},
			checkFields: true,
		},
		{
			name: "state with zero duration",
			state: &RotationState{
				OldSecret:   "old",
				NewSecret:   "new",
				GracePeriod: 0,
				StartedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Completed:   false,
			},
			checkFields: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Marshal
			data, err := json.Marshal(tt.state)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Marshal error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify JSON contains human-readable duration string
			jsonStr := string(data)
			if !strings.Contains(jsonStr, `"grace_period":"`) {
				t.Errorf("JSON should contain grace_period as string: %s", jsonStr)
			}

			// Verify no duplicate grace_period keys
			count := strings.Count(jsonStr, `"grace_period"`)
			if count != 1 {
				t.Errorf("Expected exactly 1 grace_period field, found %d in: %s", count, jsonStr)
			}

			// Test Unmarshal
			var state2 RotationState
			err = json.Unmarshal(data, &state2)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.checkFields {
				// Verify round-trip preserves values
				if state2.OldSecret != tt.state.OldSecret {
					t.Errorf("OldSecret = %v, want %v", state2.OldSecret, tt.state.OldSecret)
				}
				if state2.NewSecret != tt.state.NewSecret {
					t.Errorf("NewSecret = %v, want %v", state2.NewSecret, tt.state.NewSecret)
				}
				if state2.GracePeriod != tt.state.GracePeriod {
					t.Errorf("GracePeriod = %v, want %v", state2.GracePeriod, tt.state.GracePeriod)
				}
				if !state2.StartedAt.Equal(tt.state.StartedAt) {
					t.Errorf("StartedAt = %v, want %v", state2.StartedAt, tt.state.StartedAt)
				}
				if state2.Completed != tt.state.Completed {
					t.Errorf("Completed = %v, want %v", state2.Completed, tt.state.Completed)
				}
			}
		})
	}
}

func TestRotationStateUnmarshalJSONErrors(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name:    "invalid duration format",
			json:    `{"old_secret":"old","new_secret":"new","grace_period":"invalid","started_at":"2024-01-01T12:00:00Z","completed":false}`,
			wantErr: true,
		},
		{
			name:    "valid JSON with empty duration",
			json:    `{"old_secret":"old","new_secret":"new","grace_period":"0s","started_at":"2024-01-01T12:00:00Z","completed":false}`,
			wantErr: false,
		},
		{
			name:    "missing grace_period",
			json:    `{"old_secret":"old","new_secret":"new","started_at":"2024-01-01T12:00:00Z","completed":false}`,
			wantErr: false, // Zero value is acceptable
		},
		{
			name:    "malformed JSON",
			json:    `{"old_secret":"old","new_secret":"new","grace_period":"1h","started_at":"invalid-date","completed":false}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var state RotationState
			err := json.Unmarshal([]byte(tt.json), &state)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
