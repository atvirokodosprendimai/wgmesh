package account

import "time"

// AccountID is a unique identifier for an account
type AccountID string

// ReferralCode is a unique referral code
type ReferralCode string

// Account represents a user account in the wgmesh system
type Account struct {
	ID           AccountID    `json:"id"`
	Email        string       `json:"email,omitempty"` // Optional, for reward delivery
	ReferralCode ReferralCode `json:"referral_code"`
	CreatedAt    time.Time    `json:"created_at"`
	ReferredBy   AccountID    `json:"referred_by,omitempty"`  // If this account was referred
	ConvertedAt  *time.Time   `json:"converted_at,omitempty"` // When first mesh setup completed
}

// Referral represents a successful referral from one account to another
type Referral struct {
	ReferrerID  AccountID    `json:"referrer_id"`
	ReferredID  AccountID    `json:"referred_id"`
	Code        ReferralCode `json:"code"`
	ConvertedAt time.Time    `json:"converted_at"`
}
