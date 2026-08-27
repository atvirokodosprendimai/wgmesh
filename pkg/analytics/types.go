package analytics

import "time"

// ConversionEventType represents the type of analytics event.
type ConversionEventType string

const (
	EventTrialSignup     ConversionEventType = "trial_signup"
	EventTrialActivation ConversionEventType = "trial_activation"
	EventTrialConversion ConversionEventType = "trial_conversion"
	EventPromoRedeemed   ConversionEventType = "promo_redeemed"
	EventCommunityClick  ConversionEventType = "community_click"
)

// ConversionEvent represents an analytics event for tracking trials and conversions.
type ConversionEvent struct {
	ID         string            // Unique event ID
	Type       ConversionEventType         // ConversionEvent type
	Timestamp  time.Time         // When the event occurred
	Properties map[string]string // ConversionEvent properties
}

// Common property keys
const (
	PropCode        = "code"         // Promo code used
	PropCampaignID  = "campaign_id"  // Campaign identifier
	PropSource      = "source"       // Acquisition source (discord, slack, etc.)
	PropAccountID   = "account_id"   // Account identifier
	PropTrialDays   = "trial_days"   // Trial duration
	PropNodeLimit   = "node_limit"   // Node limit
	PropCommunityID = "community_id" // Community identifier
	PropClickURL    = "click_url"    // Click-through URL
)

// ConversionFunnel represents metrics for the trial conversion funnel.
type ConversionFunnel struct {
	SignupCount     int       // Number of trial signups
	ActivationCount int       // Number of activated trials
	ConversionCount int       // Number of paid conversions
	LastUpdated     time.Time // When metrics were last updated
}
