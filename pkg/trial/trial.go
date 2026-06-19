package trial

import (
	"fmt"
	"time"
)

const (
	// TrialDuration is the length of the trial period
	TrialDuration = 14 * 24 * time.Hour

	// GraceDuration is the length of the grace period after dismissing the upgrade prompt
	GraceDuration = 24 * time.Hour

	// StatusActive indicates the trial is currently active
	StatusActive = "active"

	// StatusExpired indicates the trial has expired
	StatusExpired = "expired"

	// StatusUpgraded indicates the user has upgraded
	StatusUpgraded = "upgraded"

	// StatusDismissed indicates the user has dismissed the upgrade prompt
	StatusDismissed = "dismissed"
)

// TrialState represents the current state of a trial
type TrialState struct {
	MeshID      string     `json:"mesh_id"`
	StartedAt   time.Time  `json:"started_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	Status      string     `json:"status"` // "active", "expired", "upgraded", "dismissed"
	DismissedAt *time.Time `json:"dismissed_at,omitempty"`
	UpgradedAt  *time.Time `json:"upgraded_at,omitempty"`
	GraceUntil  *time.Time `json:"grace_until,omitempty"` // Temporary grace period after dismissal
}

// StartTrial creates a new trial state for the given mesh ID
func StartTrial(meshID string) (*TrialState, error) {
	if meshID == "" {
		return nil, fmt.Errorf("mesh ID cannot be empty")
	}

	now := time.Now()
	return &TrialState{
		MeshID:    meshID,
		StartedAt: now,
		ExpiresAt: now.Add(TrialDuration),
		Status:    StatusActive,
	}, nil
}

// DaysRemaining calculates the number of days until expiration
// Returns negative values if expired
func DaysRemaining(state *TrialState) int {
	if state == nil {
		return 0
	}
	duration := state.ExpiresAt.Sub(time.Now())
	return int(duration.Hours() / 24)
}

// CheckExpired checks if the trial has expired
func CheckExpired(state *TrialState) bool {
	if state == nil {
		return false
	}

	// If upgraded, never expire
	if state.Status == StatusUpgraded {
		return false
	}

	// If in grace period, not expired yet
	if state.GraceUntil != nil && time.Now().Before(*state.GraceUntil) {
		return false
	}

	return time.Now().After(state.ExpiresAt)
}

// Dismiss dismisses the upgrade prompt with a grace period
func Dismiss(state *TrialState) error {
	if state == nil {
		return fmt.Errorf("trial state is nil")
	}

	now := time.Now()
	graceEnd := now.Add(GraceDuration)

	state.Status = StatusDismissed
	state.DismissedAt = &now
	state.GraceUntil = &graceEnd

	return nil
}

// Upgrade marks the trial as upgraded
func Upgrade(state *TrialState) error {
	if state == nil {
		return fmt.Errorf("trial state is nil")
	}

	now := time.Now()
	state.Status = StatusUpgraded
	state.UpgradedAt = &now

	return nil
}

// IsInGracePeriod checks if the trial is currently in a grace period
func IsInGracePeriod(state *TrialState) bool {
	if state == nil || state.GraceUntil == nil {
		return false
	}
	return time.Now().Before(*state.GraceUntil)
}

// ShouldPauseMesh checks if mesh operations should be paused due to trial expiration
func ShouldPauseMesh(state *TrialState) bool {
	if state == nil {
		return false
	}

	// Don't pause if upgraded
	if state.Status == StatusUpgraded {
		return false
	}

	// Don't pause if in grace period
	if IsInGracePeriod(state) {
		return false
	}

	// Pause if expired
	return CheckExpired(state)
}
