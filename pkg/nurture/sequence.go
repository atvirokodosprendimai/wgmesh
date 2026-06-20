package nurture

import "time"

// EmailTemplate defines a nurture email
type EmailTemplate struct {
	Delay      time.Duration
	Subject    string
	TemplateID string
	TrackingID string
}

// NurtureSequence is the ordered list of nurture emails for trial onboarding
var NurtureSequence = []EmailTemplate{
	{
		Delay:      0 * time.Minute,
		Subject:    "Welcome to Cloudroof - Let's get your mesh running",
		TemplateID: "email-01-welcome",
		TrackingID: "trial_welcome",
	},
	{
		Delay:      2 * 24 * time.Hour,
		Subject:    "How's your mesh coming along?",
		TemplateID: "email-02-usecases",
		TrackingID: "trial_usecases",
	},
	{
		Delay:      5 * 24 * time.Hour,
		Subject:    "Beyond basic meshing: Advanced Cloudroof features",
		TemplateID: "email-03-features",
		TrackingID: "trial_features",
	},
	{
		Delay:      12 * 24 * time.Hour,
		Subject:    "Your trial is halfway done - Here's what you're missing",
		TemplateID: "email-04-comparison",
		TrackingID: "trial_comparison",
	},
	{
		Delay:      18 * 24 * time.Hour,
		Subject:    "Last chance to upgrade your Cloudroof trial",
		TemplateID: "email-05-final",
		TrackingID: "trial_final",
	},
}

// GetPendingEmails returns emails that should be sent for a trial
func GetPendingEmails(createdAt time.Time, alreadySent map[string]bool) []EmailTemplate {
	var pending []EmailTemplate
	elapsed := time.Since(createdAt)

	for _, email := range NurtureSequence {
		// Skip if already sent
		if alreadySent[email.TrackingID] {
			continue
		}

		// Add if enough time has elapsed
		if elapsed >= email.Delay {
			pending = append(pending, email)
		}
	}

	return pending
}
