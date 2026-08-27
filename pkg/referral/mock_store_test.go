package referral

import (
	"context"
	"testing"
)

func TestMockStoreCreateReferrer(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	ref, err := store.CreateReferrer(ctx, "user123")
	if err != nil {
		t.Fatalf("CreateReferrer failed: %v", err)
	}
	if ref.ID != "user123" {
		t.Errorf("expected ID 'user123', got %q", ref.ID)
	}
	if !Validate(ref.Code) {
		t.Errorf("generated invalid code: %s", ref.Code)
	}
	if ref.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestMockStoreGetByCode(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	ref, _ := store.CreateReferrer(ctx, "user1")

	found, err := store.GetByCode(ctx, ref.Code)
	if err != nil {
		t.Fatalf("GetByCode failed: %v", err)
	}
	if found.ID != "user1" {
		t.Errorf("wrong referrer: got %q", found.ID)
	}
}

func TestMockStoreGetByCodeNotFound(t *testing.T) {
	store := NewMockStore()

	_, err := store.GetByCode(context.Background(), "NONEXISTENT")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMockStoreRecordReferral(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	ref, _ := store.CreateReferrer(ctx, "referrer")

	rf, err := store.RecordReferral(ctx, ref.Code, "referee1")
	if err != nil {
		t.Fatalf("RecordReferral failed: %v", err)
	}
	if rf.ReferrerCode != ref.Code {
		t.Errorf("wrong referrer code")
	}
	if rf.RefereeID != "referee1" {
		t.Errorf("wrong referee ID")
	}
	if rf.RewardTier != TierRegistered {
		t.Errorf("expected TierRegistered, got %d", rf.RewardTier)
	}

	// Referrer count should be incremented.
	updated, _ := store.GetByID(ctx, "referrer")
	if updated.ReferralCount != 1 {
		t.Errorf("expected referral count 1, got %d", updated.ReferralCount)
	}
}

func TestMockStoreRecordReferralInvalidCode(t *testing.T) {
	store := NewMockStore()

	_, err := store.RecordReferral(context.Background(), "INVALID", "referee1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for unknown referrer, got %v", err)
	}
}

func TestMockStoreUpdateReferralTier(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	ref, _ := store.CreateReferrer(ctx, "referrer")
	store.RecordReferral(ctx, ref.Code, "referee1")

	err := store.UpdateReferralTier(ctx, "referee1", TierWeekActive)
	if err != nil {
		t.Fatalf("UpdateReferralTier failed: %v", err)
	}

	rf := store.Referrals["referee1"]
	if rf.RewardTier != TierWeekActive {
		t.Errorf("expected tier %d, got %d", TierWeekActive, rf.RewardTier)
	}
}

func TestMockStoreUpdateReferralTierNotFound(t *testing.T) {
	store := NewMockStore()

	err := store.UpdateReferralTier(context.Background(), "nonexistent", TierDeployed)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMockStoreListRewards(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	// Add some rewards.
	store.Rewards = append(store.Rewards,
		Reward{ReferrerID: "user1", ReferralID: "r1", Tier: TierRegistered},
		Reward{ReferrerID: "user1", ReferralID: "r2", Tier: TierDeployed},
		Reward{ReferrerID: "user2", ReferralID: "r3", Tier: TierRegistered},
	)

	rewards, err := store.ListRewards(ctx, "user1", 10)
	if err != nil {
		t.Fatalf("ListRewards failed: %v", err)
	}
	if len(rewards) != 2 {
		t.Errorf("expected 2 rewards, got %d", len(rewards))
	}
}

func TestMockStoreListRewardsLimit(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	store.Rewards = append(store.Rewards,
		Reward{ReferrerID: "user1", ReferralID: "r1"},
		Reward{ReferrerID: "user1", ReferralID: "r2"},
		Reward{ReferrerID: "user1", ReferralID: "r3"},
	)

	rewards, err := store.ListRewards(ctx, "user1", 2)
	if err != nil {
		t.Fatalf("ListRewards failed: %v", err)
	}
	if len(rewards) != 2 {
		t.Errorf("expected 2 rewards (limit), got %d", len(rewards))
	}
}

func TestMockStoreListRewardsEmpty(t *testing.T) {
	store := NewMockStore()

	rewards, err := store.ListRewards(context.Background(), "nobody", 10)
	if err != nil {
		t.Fatalf("ListRewards failed: %v", err)
	}
	if len(rewards) != 0 {
		t.Errorf("expected 0 rewards, got %d", len(rewards))
	}
}
