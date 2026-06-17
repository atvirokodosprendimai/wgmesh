package nurture

import "time"

// EmailTemplate defines a nurture email
type EmailTemplate struct {
	Delay      time.Duration
	Subject    string
	TemplateID string
	TrackingID string
}

// NurtureSequence is the ordered list of nurture emails
var NurtureSequence = []EmailTemplate{
	{
		Delay:      0 * time.Minute,
		Subject:    "Welcome to cloudroof.eu - Your 14-Day Trial Starts Now",
		TemplateID: "trial-welcome",
		TrackingID: "trial_welcome",
	},
	{
		Delay:      1 * 24 * time.Hour,
		Subject:    "Day 1: Wire your first service to the internet",
		TemplateID: "trial-day-1",
		TrackingID: "trial_day_1",
	},
	{
		Delay:      3 * 24 * time.Hour,
		Subject:    "Day 3: Pro tip - Custom domains for your services",
		TemplateID: "trial-day-3",
		TrackingID: "trial_day_3",
	},
	{
		Delay:      7 * 24 * time.Hour,
		Subject:    "Week 1 check-in - How's your trial going?",
		TemplateID: "trial-week-1",
		TrackingID: "trial_week_1",
	},
	{
		Delay:      11 * 24 * time.Hour,
		Subject:    "3 days left - Extend your trial or upgrade",
		TemplateID: "trial-reminder",
		TrackingID: "trial_reminder",
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
