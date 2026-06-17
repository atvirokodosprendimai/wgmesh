package analytics

import "time"

// EventType represents the type of analytics event.
type EventType string

const (
	EventTrialSignup     EventType = "trial_signup"
	EventTrialActivation EventType = "trial_activation"
	EventTrialConversion EventType = "trial_conversion"
	EventPromoRedeemed   EventType = "promo_redeemed"
	EventCommunityClick  EventType = "community_click"
)

// Event represents an analytics event for tracking trials and conversions.
type Event struct {
	ID         string            // Unique event ID
	Type       EventType         // Event type
	Timestamp  time.Time         // When the event occurred
	Properties map[string]string // Event properties
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
