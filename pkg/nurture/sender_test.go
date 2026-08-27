package nurture

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewSender(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
	}{
		{
			name:    "default config",
			envVars: map[string]string{},
			wantErr: false,
		},
		{
			name: "custom config",
			envVars: map[string]string{
				"NURTURE_FROM":           "test@example.com",
				"NURTURE_REPLY_TO":       "support@example.com",
				"NURTURE_EMAIL_PROVIDER": "smtp",
				"SMTP_HOST":              "smtp.example.com",
				"SMTP_PORT":              "2525",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			s, err := NewSender()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSender() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if s == nil {
					t.Error("NewSender() returned nil sender")
				}
				if s.templates == nil {
					t.Error("NewSender() templates map is nil")
				}
				if len(s.templates) != 5 {
					t.Errorf("NewSender() loaded %d templates, want 5", len(s.templates))
				}
			}
		})
	}
}

func TestGetPendingEmails(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		createdAt   time.Time
		alreadySent map[string]bool
		want        int
	}{
		{
			name:        "no emails sent yet, day 0",
			createdAt:   now,
			alreadySent: map[string]bool{},
			want:        1, // Only welcome email
		},
		{
			name:        "day 3, no emails sent",
			createdAt:   now.Add(-3 * 24 * time.Hour),
			alreadySent: map[string]bool{},
			want:        2, // welcome and usecases
		},
		{
			name:        "day 7, no emails sent",
			createdAt:   now.Add(-7 * 24 * time.Hour),
			alreadySent: map[string]bool{},
			want:        3, // welcome, usecases, features
		},
		{
			name:        "day 15, no emails sent",
			createdAt:   now.Add(-15 * 24 * time.Hour),
			alreadySent: map[string]bool{},
			want:        4, // welcome, usecases, features, comparison (final is at day 18)
		},
		{
			name:        "day 3, welcome already sent",
			createdAt:   now.Add(-3 * 24 * time.Hour),
			alreadySent: map[string]bool{"trial_welcome": true},
			want:        1, // Only usecases
		},
		{
			name:      "all emails already sent",
			createdAt: now.Add(-20 * 24 * time.Hour),
			alreadySent: map[string]bool{
				"trial_welcome":    true,
				"trial_usecases":   true,
				"trial_features":   true,
				"trial_comparison": true,
				"trial_final":      true,
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pending := GetPendingEmails(tt.createdAt, tt.alreadySent)
			if len(pending) != tt.want {
				t.Errorf("GetPendingEmails() returned %d emails, want %d", len(pending), tt.want)
			}
		})
	}
}

func TestSendEmail(t *testing.T) {
	s, err := NewSender()
	if err != nil {
		t.Fatalf("NewSender() error = %v", err)
	}

	// Override provider to log for testing
	s.provider = "log"

	data := &TemplateData{
		TrialID:             "test-trial-123",
		Email:               "test@example.com",
		StartDate:           time.Now(),
		ExpiryDate:          time.Now().Add(14 * 24 * time.Hour),
		DaysRemaining:       14,
		UpgradeLink:         "https://cloudroof.eu/upgrade",
		UnsubscribeLink:     "https://cloudroof.eu/unsubscribe?trial=test-trial-123",
		QuickstartLink:      "https://docs.cloudroof.eu/quickstart",
		UseCasesLink:        "https://docs.cloudroof.eu/use-cases",
		FeaturesLink:        "https://docs.cloudroof.eu/features",
		FAQLink:             "https://docs.cloudroof.eu/faq",
		TroubleshootingLink: "https://docs.cloudroof.eu/troubleshooting",
	}

	tests := []struct {
		name       string
		templateID string
		subject    string
		wantErr    bool
	}{
		{
			name:       "welcome email",
			templateID: "email-01-welcome",
			subject:    "Test Welcome",
			wantErr:    false,
		},
		{
			name:       "usecases email",
			templateID: "email-02-usecases",
			subject:    "Test Use Cases",
			wantErr:    false,
		},
		{
			name:       "invalid template",
			templateID: "nonexistent",
			subject:    "Test",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.SendEmail(tt.templateID, tt.subject, data)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTemplateRendering(t *testing.T) {
	s, err := NewSender()
	if err != nil {
		t.Fatalf("NewSender() error = %v", err)
	}

	data := &TemplateData{
		TrialID:             "test-trial-123",
		Email:               "test@example.com",
		ReplyTo:             "support@cloudroof.eu",
		StartDate:           time.Now(),
		ExpiryDate:          time.Now().Add(14 * 24 * time.Hour),
		DaysRemaining:       14,
		UpgradeLink:         "https://cloudroof.eu/upgrade",
		UnsubscribeLink:     "https://cloudroof.eu/unsubscribe",
		QuickstartLink:      "https://docs.cloudroof.eu/quickstart",
		UseCasesLink:        "https://docs.cloudroof.eu/use-cases",
		FeaturesLink:        "https://docs.cloudroof.eu/features",
		FAQLink:             "https://docs.cloudroof.eu/faq",
		TroubleshootingLink: "https://docs.cloudroof.eu/troubleshooting",
	}

	templates := []string{
		"email-01-welcome",
		"email-02-usecases",
		"email-03-features",
		"email-04-comparison",
		"email-05-final",
	}

	for _, tmplID := range templates {
		t.Run(tmplID, func(t *testing.T) {
			tmpl, ok := s.templates[tmplID]
			if !ok {
				t.Errorf("Template %s not found", tmplID)
				return
			}

			var buf strings.Builder
			err := tmpl.Execute(&buf, data)
			if err != nil {
				t.Errorf("Template %s execution error: %v", tmplID, err)
				return
			}

			result := buf.String()

			// Check that all placeholders were replaced
			if strings.Contains(result, "{{.") {
				t.Errorf("Template %s contains unreplaced placeholders", tmplID)
			}

			// Check that email address is present
			if !strings.Contains(result, "test@example.com") {
				t.Errorf("Template %s does not contain email address", tmplID)
			}

			// Check that unsubscribe link is present
			if !strings.Contains(result, "https://cloudroof.eu/unsubscribe") {
				t.Errorf("Template %s does not contain unsubscribe link", tmplID)
			}

			// Check for required sections based on email type
			switch tmplID {
			case "email-01-welcome":
				if !strings.Contains(result, "Welcome") && !strings.Contains(result, "QUICK START") {
					t.Errorf("Welcome template missing expected content")
				}
			case "email-02-usecases":
				if !strings.Contains(result, "use cases") && !strings.Contains(result, "SITE-TO-SITE") {
					t.Errorf("Use cases template missing expected content")
				}
			case "email-03-features":
				if !strings.Contains(result, "NAT Traversal") && !strings.Contains(result, "MANAGED INGRESS") {
					t.Errorf("Features template missing expected content")
				}
			case "email-04-comparison":
				if !strings.Contains(result, "FREE TIER") && !strings.Contains(result, "PAID TIER") {
					t.Errorf("Comparison template missing expected content")
				}
			case "email-05-final":
				if !strings.Contains(result, "Last chance") && !strings.Contains(result, "EXTEND") {
					t.Errorf("Final template missing expected content")
				}
			}
		})
	}
}

func TestNurtureSequence(t *testing.T) {
	if len(NurtureSequence) != 5 {
		t.Errorf("NurtureSequence has %d emails, want 5", len(NurtureSequence))
	}

	// Check that all required fields are present
	requiredTrackingIDs := []string{
		"trial_welcome",
		"trial_usecases",
		"trial_features",
		"trial_comparison",
		"trial_final",
	}

	for i, email := range NurtureSequence {
		if email.Subject == "" {
			t.Errorf("Email %d has empty subject", i)
		}
		if email.TemplateID == "" {
			t.Errorf("Email %d has empty template ID", i)
		}
		if email.TrackingID == "" {
			t.Errorf("Email %d has empty tracking ID", i)
		}
		if email.Delay < 0 {
			t.Errorf("Email %d has negative delay", i)
		}
	}

	// Check tracking IDs are unique
	seen := make(map[string]bool)
	for _, email := range NurtureSequence {
		if seen[email.TrackingID] {
			t.Errorf("Duplicate tracking ID: %s", email.TrackingID)
		}
		seen[email.TrackingID] = true
	}

	// Check all required tracking IDs are present
	for _, reqID := range requiredTrackingIDs {
		if !seen[reqID] {
			t.Errorf("Missing required tracking ID: %s", reqID)
		}
	}
}
