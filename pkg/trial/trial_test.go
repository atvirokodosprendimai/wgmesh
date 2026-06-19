package trial

import (
	"testing"
	"time"
)

func TestStartTrial(t *testing.T) {
	meshID := "test-mesh-123"

	state, err := StartTrial(meshID)
	if err != nil {
		t.Fatalf("StartTrial failed: %v", err)
	}

	if state.MeshID != meshID {
		t.Errorf("expected mesh ID %s, got %s", meshID, state.MeshID)
	}

	if state.Status != StatusActive {
		t.Errorf("expected status %s, got %s", StatusActive, state.Status)
	}

	if state.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}

	if state.ExpiresAt.IsZero() {
		t.Error("expected ExpiresAt to be set")
	}

	// Check that expiration is approximately 14 days from start
	expectedExpiry := state.StartedAt.Add(TrialDuration)
	diff := expectedExpiry.Sub(state.ExpiresAt)
	if diff > time.Second {
		t.Errorf("expected expiry %s, got %s (diff %s)", expectedExpiry, state.ExpiresAt, diff)
	}
}

func TestStartTrialEmptyMeshID(t *testing.T) {
	_, err := StartTrial("")
	if err == nil {
		t.Error("expected error for empty mesh ID")
	}
}

func TestDaysRemaining(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		expires time.Time
		want    int
	}{
		{
			name:    "expires in 7 days",
			expires: now.Add(7 * 24 * time.Hour),
			want:    7,
		},
		{
			name:    "expires in 1 day",
			expires: now.Add(24 * time.Hour),
			want:    1,
		},
		{
			name:    "expired yesterday",
			expires: now.Add(-24 * time.Hour),
			want:    -1,
		},
		{
			name:    "expired 10 days ago",
			expires: now.Add(-10 * 24 * time.Hour),
			want:    -10,
		},
		{
			name:    "expires now",
			expires: now,
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &TrialState{
				MeshID:    "test",
				StartedAt: now,
				ExpiresAt: tt.expires,
				Status:    StatusActive,
			}
			got := DaysRemaining(state)
			// Allow off-by-one due to timing
			if got != tt.want && got != tt.want-1 {
				t.Errorf("DaysRemaining() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDaysRemainingNilState(t *testing.T) {
	got := DaysRemaining(nil)
	if got != 0 {
		t.Errorf("DaysRemaining(nil) = %d, want 0", got)
	}
}

func TestCheckExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		expires time.Time
		status  string
		grace   *time.Time
		want    bool
	}{
		{
			name:    "active trial not expired",
			expires: now.Add(24 * time.Hour),
			status:  StatusActive,
			want:    false,
		},
		{
			name:    "active trial expired",
			expires: now.Add(-24 * time.Hour),
			status:  StatusActive,
			want:    true,
		},
		{
			name:    "upgraded never expires",
			expires: now.Add(-100 * 24 * time.Hour),
			status:  StatusUpgraded,
			want:    false,
		},
		{
			name:    "in grace period",
			expires: now.Add(-24 * time.Hour),
			status:  StatusDismissed,
			grace:   timePtr(now.Add(1 * time.Hour)),
			want:    false,
		},
		{
			name:    "grace period expired",
			expires: now.Add(-24 * time.Hour),
			status:  StatusDismissed,
			grace:   timePtr(now.Add(-1 * time.Hour)),
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &TrialState{
				MeshID:     "test",
				StartedAt:  now,
				ExpiresAt:  tt.expires,
				Status:     tt.status,
				GraceUntil: tt.grace,
			}
			got := CheckExpired(state)
			if got != tt.want {
				t.Errorf("CheckExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckExpiredNilState(t *testing.T) {
	got := CheckExpired(nil)
	if got != false {
		t.Errorf("CheckExpired(nil) = %v, want false", got)
	}
}

func TestDismiss(t *testing.T) {
	now := time.Now()
	state := &TrialState{
		MeshID:    "test",
		StartedAt: now.Add(-15 * 24 * time.Hour), // Expired
		ExpiresAt: now.Add(-1 * time.Hour),
		Status:    StatusActive,
	}

	err := Dismiss(state)
	if err != nil {
		t.Fatalf("Dismiss failed: %v", err)
	}

	if state.Status != StatusDismissed {
		t.Errorf("expected status %s, got %s", StatusDismissed, state.Status)
	}

	if state.DismissedAt == nil {
		t.Error("expected DismissedAt to be set")
	}

	if state.GraceUntil == nil {
		t.Error("expected GraceUntil to be set")
	}

	// Check grace period is approximately 24 hours from now
	expectedGraceEnd := now.Add(GraceDuration)
	diff := expectedGraceEnd.Sub(*state.GraceUntil)
	if diff > time.Second {
		t.Errorf("expected grace end %s, got %s (diff %s)", expectedGraceEnd, *state.GraceUntil, diff)
	}
}

func TestDismissNilState(t *testing.T) {
	err := Dismiss(nil)
	if err == nil {
		t.Error("expected error for nil state")
	}
}

func TestUpgrade(t *testing.T) {
	now := time.Now()
	state := &TrialState{
		MeshID:    "test",
		StartedAt: now.Add(-15 * 24 * time.Hour),
		ExpiresAt: now.Add(-1 * time.Hour),
		Status:    StatusExpired,
	}

	err := Upgrade(state)
	if err != nil {
		t.Fatalf("Upgrade failed: %v", err)
	}

	if state.Status != StatusUpgraded {
		t.Errorf("expected status %s, got %s", StatusUpgraded, state.Status)
	}

	if state.UpgradedAt == nil {
		t.Error("expected UpgradedAt to be set")
	}

	// Check that upgraded time is approximately now (within 1 second)
	diff := now.Sub(*state.UpgradedAt)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("expected UpgradedAt ~%s, got %s (diff %s)", now, *state.UpgradedAt, diff)
	}
}

func TestUpgradeNilState(t *testing.T) {
	err := Upgrade(nil)
	if err == nil {
		t.Error("expected error for nil state")
	}
}

func TestIsInGracePeriod(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name  string
		grace *time.Time
		want  bool
	}{
		{
			name:  "no grace period",
			grace: nil,
			want:  false,
		},
		{
			name:  "grace period in future",
			grace: timePtr(now.Add(1 * time.Hour)),
			want:  true,
		},
		{
			name:  "grace period expired",
			grace: timePtr(now.Add(-1 * time.Hour)),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &TrialState{
				MeshID:     "test",
				StartedAt:  now,
				ExpiresAt:  now.Add(TrialDuration),
				Status:     StatusActive,
				GraceUntil: tt.grace,
			}
			got := IsInGracePeriod(state)
			if got != tt.want {
				t.Errorf("IsInGracePeriod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsInGracePeriodNilState(t *testing.T) {
	got := IsInGracePeriod(nil)
	if got != false {
		t.Errorf("IsInGracePeriod(nil) = %v, want false", got)
	}
}

func TestShouldPauseMesh(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		expires time.Time
		status  string
		grace   *time.Time
		want    bool
	}{
		{
			name:    "active trial not expired",
			expires: now.Add(7 * 24 * time.Hour),
			status:  StatusActive,
			want:    false,
		},
		{
			name:    "active trial expired",
			expires: now.Add(-1 * time.Hour),
			status:  StatusActive,
			want:    true,
		},
		{
			name:    "upgraded never pauses",
			expires: now.Add(-100 * 24 * time.Hour),
			status:  StatusUpgraded,
			want:    false,
		},
		{
			name:    "in grace period",
			expires: now.Add(-1 * time.Hour),
			status:  StatusDismissed,
			grace:   timePtr(now.Add(1 * time.Hour)),
			want:    false,
		},
		{
			name:    "grace period expired",
			expires: now.Add(-1 * time.Hour),
			status:  StatusDismissed,
			grace:   timePtr(now.Add(-1 * time.Hour)),
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &TrialState{
				MeshID:     "test",
				StartedAt:  now,
				ExpiresAt:  tt.expires,
				Status:     tt.status,
				GraceUntil: tt.grace,
			}
			got := ShouldPauseMesh(state)
			if got != tt.want {
				t.Errorf("ShouldPauseMesh() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldPauseMeshNilState(t *testing.T) {
	got := ShouldPauseMesh(nil)
	if got != false {
		t.Errorf("ShouldPauseMesh(nil) = %v, want false", got)
	}
}

// Helper function
func timePtr(t time.Time) *time.Time {
	return &t
}
