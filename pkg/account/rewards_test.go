package account

import (
	"testing"
)

func TestCalculateRewards(t *testing.T) {
	store := NewStore("")

	referrer, _ := store.CreateAccount("referrer@example.com")

	// No referrals yet
	rewards, err := store.CalculateRewardsDefault(referrer.ID)
	if err != nil {
		t.Fatalf("CalculateRewardsDefault() error = %v", err)
	}
	if len(rewards) != 0 {
		t.Errorf("Expected 0 rewards, got %d", len(rewards))
	}

	// Add 1 converted referral
	referred1, _ := store.CreateAccountWithReferral("referred1@example.com", referrer.ReferralCode)
	store.RecordReferral(referrer.ID, referred1.ID, referred1.ReferralCode)
	store.MarkConverted(referred1.ID)

	rewards, err = store.CalculateRewardsDefault(referrer.ID)
	if err != nil {
		t.Fatalf("CalculateRewardsDefault() error = %v", err)
	}
	if len(rewards) != 1 {
		t.Errorf("Expected 1 reward, got %d", len(rewards))
	}
	if rewards[0].Tier.Name != "1 Month Service Credit" {
		t.Errorf("Unexpected reward name: %s", rewards[0].Tier.Name)
	}

	// Add 4 more converted referrals (total 5)
	for i := 0; i < 4; i++ {
		referred, _ := store.CreateAccountWithReferral("referred@example.com", referrer.ReferralCode)
		store.RecordReferral(referrer.ID, referred.ID, referred.ReferralCode)
		store.MarkConverted(referred.ID)
	}

	rewards, err = store.CalculateRewardsDefault(referrer.ID)
	if err != nil {
		t.Fatalf("CalculateRewardsDefault() error = %v", err)
	}
	if len(rewards) != 2 {
		t.Errorf("Expected 2 rewards, got %d", len(rewards))
	}

	// Add 5 more converted referrals (total 10)
	for i := 0; i < 5; i++ {
		referred, _ := store.CreateAccountWithReferral("referred@example.com", referrer.ReferralCode)
		store.RecordReferral(referrer.ID, referred.ID, referred.ReferralCode)
		store.MarkConverted(referred.ID)
	}

	rewards, err = store.CalculateRewardsDefault(referrer.ID)
	if err != nil {
		t.Fatalf("CalculateRewardsDefault() error = %v", err)
	}
	if len(rewards) != 3 {
		t.Errorf("Expected 3 rewards, got %d", len(rewards))
	}
}

func TestCalculateRewardsCustomTiers(t *testing.T) {
	store := NewStore("")

	referrer, _ := store.CreateAccount("referrer@example.com")

	// Create custom tiers
	customTiers := []RewardTier{
		{
			ReferralsRequired: 2,
			RewardType:        RewardTypeCredit,
			Value:             5,
			Name:              "5 Month Credit",
		},
		{
			ReferralsRequired: 8,
			RewardType:        RewardTypeExtension,
			Value:             12,
			Name:              "1 Year Extension",
		},
	}

	// Add 1 converted referral (below first tier)
	referred1, _ := store.CreateAccountWithReferral("referred1@example.com", referrer.ReferralCode)
	store.RecordReferral(referrer.ID, referred1.ID, referred1.ReferralCode)
	store.MarkConverted(referred1.ID)

	rewards, err := store.CalculateRewards(referrer.ID, customTiers)
	if err != nil {
		t.Fatalf("CalculateRewards() error = %v", err)
	}
	if len(rewards) != 0 {
		t.Errorf("Expected 0 rewards below first tier, got %d", len(rewards))
	}

	// Add 1 more (total 2, exactly first tier)
	referred2, _ := store.CreateAccountWithReferral("referred2@example.com", referrer.ReferralCode)
	store.RecordReferral(referrer.ID, referred2.ID, referred2.ReferralCode)
	store.MarkConverted(referred2.ID)

	rewards, err = store.CalculateRewards(referrer.ID, customTiers)
	if err != nil {
		t.Fatalf("CalculateRewards() error = %v", err)
	}
	if len(rewards) != 1 {
		t.Errorf("Expected 1 reward at first tier, got %d", len(rewards))
	}

	// Add 6 more (total 8, exactly second tier)
	for i := 0; i < 6; i++ {
		referred, _ := store.CreateAccountWithReferral("referred@example.com", referrer.ReferralCode)
		store.RecordReferral(referrer.ID, referred.ID, referred.ReferralCode)
		store.MarkConverted(referred.ID)
	}

	rewards, err = store.CalculateRewards(referrer.ID, customTiers)
	if err != nil {
		t.Fatalf("CalculateRewards() error = %v", err)
	}
	if len(rewards) != 2 {
		t.Errorf("Expected 2 rewards at second tier, got %d", len(rewards))
	}
}

func TestGetConversionRate(t *testing.T) {
	store := NewStore("")

	referrer, _ := store.CreateAccount("referrer@example.com")

	// No referrals
	rate, err := store.GetConversionRate(referrer.ID)
	if err != nil {
		t.Fatalf("GetConversionRate() error = %v", err)
	}
	if rate != 0.0 {
		t.Errorf("Expected 0%% conversion rate, got %.2f%%", rate)
	}

	// Add 5 referrals, convert 3
	for i := 0; i < 5; i++ {
		referred, _ := store.CreateAccountWithReferral("referred@example.com", referrer.ReferralCode)
		store.RecordReferral(referrer.ID, referred.ID, referred.ReferralCode)
		if i < 3 {
			store.MarkConverted(referred.ID)
		}
	}

	rate, err = store.GetConversionRate(referrer.ID)
	if err != nil {
		t.Fatalf("GetConversionRate() error = %v", err)
	}
	if rate != 60.0 {
		t.Errorf("Expected 60%% conversion rate, got %.2f%%", rate)
	}
}

func TestGetConversionRateAllConverted(t *testing.T) {
	store := NewStore("")

	referrer, _ := store.CreateAccount("referrer@example.com")

	// Add 3 referrals, convert all
	for i := 0; i < 3; i++ {
		referred, _ := store.CreateAccountWithReferral("referred@example.com", referrer.ReferralCode)
		store.RecordReferral(referrer.ID, referred.ID, referred.ReferralCode)
		store.MarkConverted(referred.ID)
	}

	rate, err := store.GetConversionRate(referrer.ID)
	if err != nil {
		t.Fatalf("GetConversionRate() error = %v", err)
	}
	if rate != 100.0 {
		t.Errorf("Expected 100%% conversion rate, got %.2f%%", rate)
	}
}

func TestGetPendingRewardCount(t *testing.T) {
	store := NewStore("")

	referrer, _ := store.CreateAccount("referrer@example.com")

	// No referrals yet
	count, err := store.GetPendingRewardCount(referrer.ID, DefaultRewardTiers)
	if err != nil {
		t.Fatalf("GetPendingRewardCount() error = %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 pending rewards, got %d", count)
	}

	// Add 1 converted referral (earns first reward)
	referred1, _ := store.CreateAccountWithReferral("referred1@example.com", referrer.ReferralCode)
	store.RecordReferral(referrer.ID, referred1.ID, referred1.ReferralCode)
	store.MarkConverted(referred1.ID)

	count, err = store.GetPendingRewardCount(referrer.ID, DefaultRewardTiers)
	if err != nil {
		t.Fatalf("GetPendingRewardCount() error = %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 pending reward, got %d", count)
	}
}

func TestCalculateRewardsUnconverted(t *testing.T) {
	store := NewStore("")

	referrer, _ := store.CreateAccount("referrer@example.com")

	// Add 5 referrals but don't convert any
	for i := 0; i < 5; i++ {
		referred, _ := store.CreateAccountWithReferral("referred@example.com", referrer.ReferralCode)
		store.RecordReferral(referrer.ID, referred.ID, referred.ReferralCode)
		// Don't mark as converted
	}

	rewards, err := store.CalculateRewardsDefault(referrer.ID)
	if err != nil {
		t.Fatalf("CalculateRewardsDefault() error = %v", err)
	}
	if len(rewards) != 0 {
		t.Errorf("Expected 0 rewards for unconverted referrals, got %d", len(rewards))
	}
}

func TestCalculateRewardsEdgeCases(t *testing.T) {
	store := NewStore("")

	referrer, _ := store.CreateAccount("referrer@example.com")

	// Test with 0 referrals
	rewards, err := store.CalculateRewardsDefault(referrer.ID)
	if err != nil {
		t.Fatalf("CalculateRewardsDefault() error = %v", err)
	}
	if len(rewards) != 0 {
		t.Errorf("Expected 0 rewards for 0 referrals, got %d", len(rewards))
	}

	// Test with exact tier boundary (1 referral = first tier)
	referred1, _ := store.CreateAccountWithReferral("referred1@example.com", referrer.ReferralCode)
	store.RecordReferral(referrer.ID, referred1.ID, referred1.ReferralCode)
	store.MarkConverted(referred1.ID)

	rewards, err = store.CalculateRewardsDefault(referrer.ID)
	if err != nil {
		t.Fatalf("CalculateRewardsDefault() error = %v", err)
	}
	if len(rewards) != 1 {
		t.Errorf("Expected 1 reward at exact tier boundary, got %d", len(rewards))
	}
}

func TestRewardTierValues(t *testing.T) {
	// Verify default reward tiers are correctly configured
	if len(DefaultRewardTiers) != 3 {
		t.Errorf("Expected 3 default reward tiers, got %d", len(DefaultRewardTiers))
	}

	// Check first tier
	if DefaultRewardTiers[0].ReferralsRequired != 1 {
		t.Errorf("First tier referrals required = %d, want 1", DefaultRewardTiers[0].ReferralsRequired)
	}
	if DefaultRewardTiers[0].RewardType != RewardTypeCredit {
		t.Errorf("First tier type = %s, want %s", DefaultRewardTiers[0].RewardType, RewardTypeCredit)
	}

	// Check second tier
	if DefaultRewardTiers[1].ReferralsRequired != 5 {
		t.Errorf("Second tier referrals required = %d, want 5", DefaultRewardTiers[1].ReferralsRequired)
	}
	if DefaultRewardTiers[1].RewardType != RewardTypeExtension {
		t.Errorf("Second tier type = %s, want %s", DefaultRewardTiers[1].RewardType, RewardTypeExtension)
	}

	// Check third tier
	if DefaultRewardTiers[2].ReferralsRequired != 10 {
		t.Errorf("Third tier referrals required = %d, want 10", DefaultRewardTiers[2].ReferralsRequired)
	}
	if DefaultRewardTiers[2].RewardType != RewardTypePremiumFeature {
		t.Errorf("Third tier type = %s, want %s", DefaultRewardTiers[2].RewardType, RewardTypePremiumFeature)
	}
}
