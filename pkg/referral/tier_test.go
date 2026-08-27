package referral

import "testing"

func TestShouldUpgradeTier(t *testing.T) {
	tests := []struct {
		name        string
		currentTier int
		newTier     int
		want        bool
	}{
		{"upgrade to higher tier", TierRegistered, TierDeployed, true},
		{"skip multiple tiers", TierRegistered, TierSubscribed, true},
		{"same tier is no-op", TierDeployed, TierDeployed, false},
		{"downgrade rejected", TierWeekActive, TierDeployed, false},
		{"from registered to registered", TierRegistered, TierRegistered, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldUpgradeTier(tt.currentTier, tt.newTier)
			if got != tt.want {
				t.Errorf("ShouldUpgradeTier(%d, %d) = %v, want %v",
					tt.currentTier, tt.newTier, got, tt.want)
			}
		})
	}
}

func TestTierOrdering(t *testing.T) {
	// Tiers must be strictly increasing constants so that
	// ShouldUpgradeTier's monotonic assumption holds.
	if !(TierRegistered < TierDeployed && TierDeployed < TierWeekActive &&
		TierWeekActive < TierMonthActive && TierMonthActive < TierSubscribed) {
		t.Error("tier constants are not strictly increasing")
	}
}
