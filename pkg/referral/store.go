package referral

import "context"

// Store defines the persistence interface for referral data.
//
// The implementation lives in a private backend repository; the public repo
// only defines this interface so that the CLI (and tests) can depend on it
// without coupling to a concrete storage backend.
type Store interface {
	// CreateReferrer creates a new referrer with a generated code.
	CreateReferrer(ctx context.Context, id string) (*Referrer, error)

	// GetByCode retrieves a referrer by their share code.
	GetByCode(ctx context.Context, code string) (*Referrer, error)

	// GetByID retrieves a referrer by their ID.
	GetByID(ctx context.Context, id string) (*Referrer, error)

	// RecordReferral records a new referrer-referee relationship.
	RecordReferral(ctx context.Context, referrerCode, refereeID string) (*Referral, error)

	// UpdateReferralTier updates a referral's reward tier.
	UpdateReferralTier(ctx context.Context, referralID string, tier int) error

	// ListRewards lists rewards for a referrer (paginated by limit).
	ListRewards(ctx context.Context, referrerID string, limit int) ([]Reward, error)
}
