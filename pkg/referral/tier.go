package referral

// Reward tier definitions.
//
// Tiers are monotonically increasing: a referral's tier only moves forward as
// the referred user completes qualifying actions.
const (
	// TierRegistered is assigned when a user registers with a referral code.
	TierRegistered = 0
	// TierDeployed is assigned when a user completes their first mesh deployment.
	TierDeployed = 1
	// TierWeekActive is assigned when the daemon has run for 7+ days.
	TierWeekActive = 2
	// TierMonthActive is assigned when the daemon has run for 30+ days.
	TierMonthActive = 3
	// TierSubscribed is assigned when a user activates a paid subscription.
	TierSubscribed = 4
)

// ShouldUpgradeTier reports whether a referral should be upgraded from
// currentTier to newTier. Tiers only move forward, so a lower or equal
// proposed tier is rejected.
func ShouldUpgradeTier(currentTier, newTier int) bool {
	return newTier > currentTier
}
