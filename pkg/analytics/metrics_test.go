package analytics

import (
	"path/filepath"
	"testing"
	"time"
)

func tempLogWithEvents(t *testing.T, events []Event) string {
	t.Helper()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "analytics.log")

	logger, err := NewLogger(LoggerConfig{LogPath: logPath, BufSize: 1})
	if err != nil {
		t.Fatalf("NewLogger() failed: %v", err)
	}

	for _, event := range events {
		if err := logger.Log(event); err != nil {
			t.Fatalf("Log() failed: %v", err)
		}
	}

	if err := logger.Flush(); err != nil {
		t.Fatalf("Flush() failed: %v", err)
	}

	return logPath
}

func TestComputeCampaignMetrics(t *testing.T) {
	events := []Event{
		{
			Type: EventPromoRedeemed,
			Properties: map[string]string{
				PropCampaignID: "campaign-1",
				PropSource:     "discord",
				PropAccountID:  "account-1",
			},
		},
		{
			Type: EventTrialSignup,
			Properties: map[string]string{
				PropCampaignID: "campaign-1",
				PropAccountID:  "account-1",
			},
		},
		{
			Type: EventTrialSignup,
			Properties: map[string]string{
				PropCampaignID: "campaign-1",
				PropAccountID:  "account-2",
			},
		},
		{
			Type: EventTrialActivation,
			Properties: map[string]string{
				PropCampaignID: "campaign-1",
				PropAccountID:  "account-1",
			},
		},
		{
			Type: EventTrialConversion,
			Properties: map[string]string{
				PropCampaignID: "campaign-1",
				PropAccountID:  "account-1",
			},
		},
		// Different campaign
		{
			Type: EventTrialSignup,
			Properties: map[string]string{
				PropCampaignID: "campaign-2",
				PropAccountID:  "account-3",
			},
		},
	}

	logPath := tempLogWithEvents(t, events)

	calc := NewCalculator(logPath)
	metrics, err := calc.ComputeCampaignMetrics("campaign-1")
	if err != nil {
		t.Fatalf("ComputeCampaignMetrics() failed: %v", err)
	}

	if metrics.CampaignID != "campaign-1" {
		t.Errorf("campaign ID = %q, want campaign-1", metrics.CampaignID)
	}

	if metrics.Source != "discord" {
		t.Errorf("source = %q, want discord", metrics.Source)
	}

	if metrics.PromosRedeemed != 1 {
		t.Errorf("promos redeemed = %d, want 1", metrics.PromosRedeemed)
	}

	if metrics.Signups != 2 {
		t.Errorf("signups = %d, want 2", metrics.Signups)
	}

	if metrics.Activations != 1 {
		t.Errorf("activations = %d, want 1", metrics.Activations)
	}

	if metrics.Conversions != 1 {
		t.Errorf("conversions = %d, want 1", metrics.Conversions)
	}

	expectedRate := 50.0 // 1 conversion / 2 signups * 100
	if metrics.ConversionRate != expectedRate {
		t.Errorf("conversion rate = %f, want %f", metrics.ConversionRate, expectedRate)
	}
}

func TestComputeCampaignMetricsNoEvents(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "analytics.log")

	// Don't create any events
	calc := NewCalculator(logPath)
	metrics, err := calc.ComputeCampaignMetrics("campaign-1")
	if err != nil {
		t.Fatalf("ComputeCampaignMetrics() failed: %v", err)
	}

	if metrics.CampaignID != "campaign-1" {
		t.Errorf("campaign ID = %q, want campaign-1", metrics.CampaignID)
	}

	if metrics.Signups != 0 {
		t.Errorf("signups = %d, want 0", metrics.Signups)
	}
}

func TestComputeFunnelMetrics(t *testing.T) {
	events := []Event{
		{Type: EventTrialSignup},
		{Type: EventTrialSignup},
		{Type: EventTrialSignup},
		{Type: EventTrialActivation},
		{Type: EventTrialActivation},
		{Type: EventTrialConversion},
	}

	logPath := tempLogWithEvents(t, events)

	calc := NewCalculator(logPath)
	funnel, err := calc.ComputeFunnelMetrics()
	if err != nil {
		t.Fatalf("ComputeFunnelMetrics() failed: %v", err)
	}

	if funnel.SignupCount != 3 {
		t.Errorf("signup count = %d, want 3", funnel.SignupCount)
	}

	if funnel.ActivationCount != 2 {
		t.Errorf("activation count = %d, want 2", funnel.ActivationCount)
	}

	if funnel.ConversionCount != 1 {
		t.Errorf("conversion count = %d, want 1", funnel.ConversionCount)
	}

	if funnel.LastUpdated.IsZero() {
		t.Error("last updated not set")
	}

	// Verify LastUpdated is recent
	if time.Since(funnel.LastUpdated) > time.Second {
		t.Error("last updated too old")
	}
}

func TestComputeFunnelMetricsNoEvents(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "analytics.log")

	// Don't create any events
	calc := NewCalculator(logPath)
	funnel, err := calc.ComputeFunnelMetrics()
	if err != nil {
		t.Fatalf("ComputeFunnelMetrics() failed: %v", err)
	}

	if funnel.SignupCount != 0 {
		t.Errorf("signup count = %d, want 0", funnel.SignupCount)
	}

	if funnel.LastUpdated.IsZero() {
		t.Error("last updated not set")
	}
}
