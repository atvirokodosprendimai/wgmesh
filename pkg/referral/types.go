package referral

import "time"

// Referrer represents a user who can refer others.
type Referrer struct {
	ID            string    // Unique identifier (e.g., mesh ID or account email hash)
	Code          string    // Share code (XXXXX-XXXXX)
	CreatedAt     time.Time // When the referrer joined
	ReferralCount int       // Number of successful referrals
}

// Referral represents a referrer-referee relationship.
type Referral struct {
	ReferrerCode string    // The referrer's share code
	RefereeID    string    // The referred user's identifier
	ConvertedAt  time.Time // When the referee completed a qualifying action
	RewardTier   int       // 0=registered, 1=first_deployment, 2=week_active, etc.
}

// Reward represents an earned reward.
//
// The actual reward value (credit amount, subscription tier, etc.) is stored
// separately in the private backend and intentionally never exposed here so
// that sensitive customer and revenue data does not leak into the public repo.
type Reward struct {
	ReferrerID string    // Who earned it
	ReferralID string    // Which referral triggered it
	Tier       int       // Reward tier
	GrantedAt  time.Time // When granted
}
