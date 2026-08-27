package referral

import (
	"context"
	"time"
)

// MockStore implements Store for testing.
//
// It is intentionally a simple in-memory implementation and is the only Store
// implementation that lives in the public repo. Production backends provide a
// real implementation against their own storage.
type MockStore struct {
	Referrers map[string]*Referrer // Key: ID
	Referrals map[string]*Referral // Key: refereeID
	Rewards   []Reward
}

// NewMockStore returns an initialized MockStore ready for use in tests.
func NewMockStore() *MockStore {
	return &MockStore{
		Referrers: make(map[string]*Referrer),
		Referrals: make(map[string]*Referral),
	}
}

// CreateReferrer creates a new referrer with a freshly generated code.
func (m *MockStore) CreateReferrer(ctx context.Context, id string) (*Referrer, error) {
	code, err := Generate()
	if err != nil {
		return nil, err
	}
	ref := &Referrer{
		ID:        id,
		Code:      code,
		CreatedAt: time.Now(),
	}
	m.Referrers[id] = ref
	return ref, nil
}

// GetByCode retrieves a referrer by their share code.
func (m *MockStore) GetByCode(ctx context.Context, code string) (*Referrer, error) {
	for _, ref := range m.Referrers {
		if ref.Code == code {
			return ref, nil
		}
	}
	return nil, ErrNotFound
}

// GetByID retrieves a referrer by their ID.
func (m *MockStore) GetByID(ctx context.Context, id string) (*Referrer, error) {
	ref, ok := m.Referrers[id]
	if !ok {
		return nil, ErrNotFound
	}
	return ref, nil
}

// RecordReferral records a new referrer-referee relationship at the registered
// tier and bumps the referrer's referral count.
func (m *MockStore) RecordReferral(ctx context.Context, referrerCode, refereeID string) (*Referral, error) {
	referrer, err := m.GetByCode(ctx, referrerCode)
	if err != nil {
		return nil, err
	}
	ref := &Referral{
		ReferrerCode: referrerCode,
		RefereeID:    refereeID,
		ConvertedAt:  time.Now(),
		RewardTier:   TierRegistered,
	}
	m.Referrals[refereeID] = ref
	referrer.ReferralCount++
	return ref, nil
}

// UpdateReferralTier updates a referral's reward tier.
func (m *MockStore) UpdateReferralTier(ctx context.Context, refereeID string, tier int) error {
	ref, ok := m.Referrals[refereeID]
	if !ok {
		return ErrNotFound
	}
	ref.RewardTier = tier
	return nil
}

// ListRewards lists rewards for a referrer (limited to limit results).
func (m *MockStore) ListRewards(ctx context.Context, referrerID string, limit int) ([]Reward, error) {
	if limit <= 0 {
		return nil, nil
	}
	var result []Reward
	for _, r := range m.Rewards {
		if r.ReferrerID == referrerID {
			result = append(result, r)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}
