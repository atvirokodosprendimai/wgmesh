package account

import "fmt"

// RewardType represents the type of reward
type RewardType string

const (
	// RewardTypeCredit is a service credit reward
	RewardTypeCredit RewardType = "credit"

	// RewardTypeExtension is a service extension reward
	RewardTypeExtension RewardType = "extension"

	// RewardTypePremiumFeature is a premium feature unlock reward
	RewardTypePremiumFeature RewardType = "premium_feature"
)

// RewardTier defines a reward tier based on referral count
type RewardTier struct {
	ReferralsRequired int        `json:"referrals_required"`
	RewardType        RewardType `json:"reward_type"`
	Value             int        `json:"value"` // Duration in months or credit amount
	Name              string     `json:"name"`  // Human-readable name
}

// Reward represents an earned reward
type Reward struct {
	Tier          RewardTier `json:"tier"`
	ReferralCount int        `json:"referral_count"`
	ClaimedAt     *int64     `json:"claimed_at,omitempty"` // Unix timestamp
}

// Default reward tiers for the referral program
var DefaultRewardTiers = []RewardTier{
	{
		ReferralsRequired: 1,
		RewardType:        RewardTypeCredit,
		Value:             1,
		Name:              "1 Month Service Credit",
	},
	{
		ReferralsRequired: 5,
		RewardType:        RewardTypeExtension,
		Value:             3,
		Name:              "3-Month Service Extension",
	},
	{
		ReferralsRequired: 10,
		RewardType:        RewardTypePremiumFeature,
		Value:             0,
		Name:              "Premium Feature Unlock",
	},
}

// CalculateRewards calculates pending rewards for an account
func (s *Store) CalculateRewards(accountID AccountID, tiers []RewardTier) ([]Reward, error) {
	referralCount, err := s.GetConvertedReferrals(accountID)
	if err != nil {
		return nil, fmt.Errorf("getting referral count: %w", err)
	}

	var rewards []Reward
	for _, tier := range tiers {
		if referralCount >= tier.ReferralsRequired {
			rewards = append(rewards, Reward{
				Tier:          tier,
				ReferralCount: referralCount,
				ClaimedAt:     nil,
			})
		}
	}

	return rewards, nil
}

// CalculateRewardsDefault calculates pending rewards using default tiers
func (s *Store) CalculateRewardsDefault(accountID AccountID) ([]Reward, error) {
	return s.CalculateRewards(accountID, DefaultRewardTiers)
}

// GetConversionRate calculates the conversion rate for a referrer
// Returns the percentage of referred accounts that completed mesh setup
func (s *Store) GetConversionRate(referrerID AccountID) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total, converted := 0, 0
	for _, r := range s.referrals {
		if r.ReferrerID == referrerID {
			total++
			if referred, ok := s.accounts[r.ReferredID]; ok && referred.ConvertedAt != nil {
				converted++
			}
		}
	}

	if total == 0 {
		return 0.0, nil
	}

	return float64(converted) / float64(total) * 100.0, nil
}

// GetPendingRewardCount returns the number of unclaimed rewards for an account
func (s *Store) GetPendingRewardCount(accountID AccountID, tiers []RewardTier) (int, error) {
	rewards, err := s.CalculateRewards(accountID, tiers)
	if err != nil {
		return 0, err
	}

	// Count unclaimed rewards
	count := 0
	for _, r := range rewards {
		if r.ClaimedAt == nil {
			count++
		}
	}
	return count, nil
}
