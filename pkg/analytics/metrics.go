package analytics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Calculator computes metrics from analytics logs.
type Calculator struct {
	logPath string
}

// NewCalculator creates a metrics calculator.
func NewCalculator(logPath string) *Calculator {
	return &Calculator{logPath: logPath}
}

// CampaignMetrics represents metrics for a promotional campaign.
type CampaignMetrics struct {
	CampaignID      string
	Source          string
	PromosGenerated int
	PromosRedeemed  int
	Signups         int
	Activations     int
	Conversions     int
	ConversionRate  float64
}

// ComputeCampaignMetrics calculates metrics for a campaign.
func (c *Calculator) ComputeCampaignMetrics(campaignID string) (*CampaignMetrics, error) {
	f, err := os.Open(c.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No events yet
			return &CampaignMetrics{CampaignID: campaignID}, nil
		}
		return nil, fmt.Errorf("open log: %w", err)
	}
	defer f.Close()

	metrics := &CampaignMetrics{CampaignID: campaignID}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue // Skip malformed lines
		}

		if event.Properties[PropCampaignID] != campaignID {
			continue
		}

		// Track source
		if metrics.Source == "" && event.Properties[PropSource] != "" {
			metrics.Source = event.Properties[PropSource]
		}

		switch event.Type {
		case EventPromoRedeemed:
			metrics.PromosRedeemed++
		case EventTrialSignup:
			metrics.Signups++
		case EventTrialActivation:
			metrics.Activations++
		case EventTrialConversion:
			metrics.Conversions++
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan log: %w", err)
	}

	// Calculate conversion rate
	if metrics.Signups > 0 {
		metrics.ConversionRate = float64(metrics.Conversions) / float64(metrics.Signups) * 100
	}

	return metrics, nil
}

// ComputeFunnelMetrics calculates overall conversion funnel metrics.
func (c *Calculator) ComputeFunnelMetrics() (*ConversionFunnel, error) {
	f, err := os.Open(c.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ConversionFunnel{LastUpdated: time.Now()}, nil
		}
		return nil, fmt.Errorf("open log: %w", err)
	}
	defer f.Close()

	funnel := &ConversionFunnel{LastUpdated: time.Now()}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}

		switch event.Type {
		case EventTrialSignup:
			funnel.SignupCount++
		case EventTrialActivation:
			funnel.ActivationCount++
		case EventTrialConversion:
			funnel.ConversionCount++
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan log: %w", err)
	}

	return funnel, nil
}
