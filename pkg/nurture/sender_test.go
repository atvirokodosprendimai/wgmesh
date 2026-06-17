package nurture

import (
	"os"
	"testing"
	"time"
)

func TestNurtureSequenceTiming(t *testing.T) {
	tests := []struct {
		name        string
		elapsed     time.Duration
		wantPending int
		alreadySent map[string]bool
	}{
		{
			name:        "no time elapsed",
			elapsed:     0,
			wantPending: 1, // Only welcome email
			alreadySent: make(map[string]bool),
		},
		{
			name:        "1 day elapsed",
			elapsed:     24 * time.Hour,
			wantPending: 2, // Welcome and day 1
			alreadySent: make(map[string]bool),
		},
		{
			name:        "3 days elapsed",
			elapsed:     72 * time.Hour,
			wantPending: 3, // Welcome, day 1, day 3
			alreadySent: make(map[string]bool),
		},
		{
			name:        "all emails sent",
			elapsed:     30 * 24 * time.Hour,
			wantPending: 0,
			alreadySent: map[string]bool{
				"trial_welcome":  true,
				"trial_day_1":    true,
				"trial_day_3":    true,
				"trial_week_1":   true,
				"trial_reminder": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createdAt := time.Now().Add(-tt.elapsed)
			pending := GetPendingEmails(createdAt, tt.alreadySent)

			if len(pending) != tt.wantPending {
				t.Errorf("Got %d pending emails, want %d", len(pending), tt.wantPending)
			}
		})
	}
}

func TestSenderNew(t *testing.T) {
	// Test with default provider (log)
	os.Unsetenv("NURTURE_EMAIL_PROVIDER")
	sender, err := NewSender()
	if err != nil {
		t.Fatalf("NewSender() error = %v", err)
	}
	if sender.provider != "log" {
		t.Errorf("Default provider = %s, want log", sender.provider)
	}
}

func TestSenderLoadTemplates(t *testing.T) {
	sender, err := NewSender()
	if err != nil {
		t.Fatalf("NewSender() error = %v", err)
	}

	// Verify templates exist
	requiredTemplates := []string{
		"trial-welcome",
		"trial-day-1",
		"trial-day-3",
		"trial-week-1",
		"trial-reminder",
	}

	for _, tmplID := range requiredTemplates {
		if _, ok := sender.templates[tmplID]; !ok {
			t.Errorf("Template %s not loaded", tmplID)
		}
	}
}

func TestSendTemplateLog(t *testing.T) {
	// Set provider to log
	os.Setenv("NURTURE_EMAIL_PROVIDER", "log")
	defer os.Unsetenv("NURTURE_EMAIL_PROVIDER")

	sender, err := NewSender()
	if err != nil {
		t.Fatalf("NewSender() error = %v", err)
	}

	data := map[string]interface{}{
		"Subject":        "Test Subject",
		"Email":          "test@example.com",
		"InstallCmd":     []string{"curl -sSL https://get.wgmesh.dev | sh"},
		"ServiceCmd":     []string{"wgmesh service register --name my-api --port 8080"},
		"ServiceURL":     "https://my-api.cloudroof.eu",
		"UnsubscribeURL": "https://cloudroof.eu/unsubscribe?email=test@example.com",
	}

	err = sender.SendTemplate("test@example.com", "trial-welcome", data)
	if err != nil {
		t.Errorf("SendTemplate() error = %v", err)
	}
}

func TestGetPendingEmails(t *testing.T) {
	createdAt := time.Now().Add(-2 * 24 * time.Hour) // 2 days ago
	alreadySent := map[string]bool{
		"trial_welcome": true, // Welcome already sent
	}

	pending := GetPendingEmails(createdAt, alreadySent)

	// Should have day 1 email (2 days >= 1 day delay)
	// Should not have welcome (already sent)
	// Should not have day 3 (2 days < 3 days)
	if len(pending) != 1 {
		t.Errorf("Got %d pending emails, want 1", len(pending))
	}

	if len(pending) > 0 && pending[0].TrackingID != "trial_day_1" {
		t.Errorf("Got %s, want trial_day_1", pending[0].TrackingID)
	}
}
