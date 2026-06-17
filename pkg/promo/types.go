package promo

import "time"

// Code is a promo code for trial offers.
// Format: [base32(source_id)][checksum][version]
// Length: 12 characters, URL-safe, case-insensitive
type Code string

// Source identifies where a promo code was distributed.
// Used for tracking community-sourced trial conversions.
type Source string

const (
	SourceDiscord  Source = "discord"
	SourceSlack    Source = "slack"
	SourceReddit   Source = "reddit"
	SourceTwitter  Source = "twitter"
	SourceShowHN   Source = "showhn"
	SourceDirect   Source = "direct"
	SourceReferral Source = "referral"
	SourceUnknown  Source = "unknown"
)

// Campaign represents a promotional campaign.
type Campaign struct {
	ID         string    // Campaign identifier
	Name       string    // Human-readable name
	Source     Source    // Distribution channel
	CodePrefix string    // Optional prefix for generated codes
	CreatedAt  time.Time // When campaign was created
	ExpiresAt  time.Time // When campaign ends (zero for no expiry)
	TrialDays  int       // Trial duration in days
	NodeLimit  int       // Maximum nodes in trial
}

// Promo represents a generated promo code with metadata.
type Promo struct {
	Code       Code      // The promo code
	CampaignID string    // Which campaign this belongs to
	Redeemed   bool      // Whether code has been used
	RedeemedAt time.Time // When code was redeemed (zero if not redeemed)
	CreatedAt  time.Time // When code was generated
}

// Redemption represents a successful promo code redemption.
type Redemption struct {
	Code        Code      // The code that was redeemed
	CampaignID  string    // Which campaign
	AccountID   string    // Account that redeemed (optional identifier)
	RedeemedAt  time.Time // When redemption occurred
	TrialEndsAt time.Time // When trial expires
}
