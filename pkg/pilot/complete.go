package pilot

import (
	"fmt"
	"strings"
	"time"
)

// FinalReport represents the final pilot summary
type FinalReport struct {
	PilotID      string
	Organization string
	ContactEmail string
	StartDate    time.Time
	EndDate      time.Time
	DurationDays int
	Mode         string
	NodeCount    int
	Milestones   map[string]*Milestone
	Summary      *ReportSummary
	CompletedAt  time.Time
}

// ReportSummary provides a summary of pilot results
type ReportSummary struct {
	AllMilestonesCompleted bool
	MeshConnectivityAvg    float64
	PeerDiscoverySuccess   float64
	TotalDaemonRestarts    int
	TotalWGRestarts        int
	Recommendation         string
	OverallRating          string // "excellent", "good", "fair", "poor"
}

// Complete finalizes the pilot and generates final summary
func (p *Pilot) Complete() (*FinalReport, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.state.Started {
		return nil, fmt.Errorf("pilot not started")
	}

	if p.state.Completed {
		return nil, fmt.Errorf("pilot already completed")
	}

	// Get final metrics
	metricsSnapshot := p.metrics.Snapshot()

	// Generate final report
	report := &FinalReport{
		PilotID:      p.state.Config.PilotID,
		Organization: p.state.Config.Organization,
		ContactEmail: p.state.Config.ContactEmail,
		StartDate:    p.state.Config.StartDate,
		EndDate:      p.state.Config.EndDate,
		DurationDays: int(p.state.Config.EndDate.Sub(p.state.Config.StartDate).Hours() / 24),
		Mode:         p.state.Config.Mode,
		NodeCount:    p.state.Config.NodeCount,
		Milestones:   p.state.Config.Milestones,
		Summary:      p.generateReportSummary(metricsSnapshot),
		CompletedAt:  time.Now(),
	}

	// Mark pilot as completed
	p.state.Completed = true

	return report, nil
}

// generateReportSummary creates a summary of pilot results
func (p *Pilot) generateReportSummary(metrics *Metrics) *ReportSummary {
	summary := &ReportSummary{
		MeshConnectivityAvg:  metrics.MeshUptimePercent,
		PeerDiscoverySuccess: metrics.PeerDiscoverySuccess,
		TotalDaemonRestarts:  metrics.DaemonRestarts,
		TotalWGRestarts:      metrics.WireGuardRestarts,
	}

	// Check if all milestones completed
	allCompleted := true
	for _, milestone := range p.state.Config.Milestones {
		if !milestone.Completed {
			allCompleted = false
			break
		}
	}
	summary.AllMilestonesCompleted = allCompleted

	// Determine overall rating and recommendation
	summary.OverallRating, summary.Recommendation = p.evaluatePilot(summary, metrics)

	return summary
}

// evaluatePilot determines the overall rating and recommendation
func (p *Pilot) evaluatePilot(summary *ReportSummary, metrics *Metrics) (string, string) {
	score := 0

	// Mesh connectivity score (30 points)
	if summary.MeshConnectivityAvg >= 99.9 {
		score += 30
	} else if summary.MeshConnectivityAvg >= 99.0 {
		score += 25
	} else if summary.MeshConnectivityAvg >= 95.0 {
		score += 20
	}

	// Peer discovery score (20 points)
	if summary.PeerDiscoverySuccess >= 0.95 {
		score += 20
	} else if summary.PeerDiscoverySuccess >= 0.80 {
		score += 15
	} else if summary.PeerDiscoverySuccess >= 0.60 {
		score += 10
	}

	// Stability score (30 points)
	if summary.TotalDaemonRestarts == 0 && summary.TotalWGRestarts == 0 {
		score += 30
	} else if summary.TotalDaemonRestarts <= 2 && summary.TotalWGRestarts <= 2 {
		score += 20
	} else if summary.TotalDaemonRestarts <= 5 && summary.TotalWGRestarts <= 5 {
		score += 10
	}

	// Milestone completion score (20 points)
	if summary.AllMilestonesCompleted {
		score += 20
	} else {
		completedCount := 0
		for _, m := range p.state.Config.Milestones {
			if m.Completed {
				completedCount++
			}
		}
		completionRate := float64(completedCount) / float64(len(p.state.Config.Milestones))
		score += int(completionRate * 20)
	}

	// Determine rating
	var rating, recommendation string
	switch {
	case score >= 90:
		rating = "excellent"
		recommendation = "Ready for production deployment"
	case score >= 70:
		rating = "good"
		recommendation = "Suitable for production with monitoring"
	case score >= 50:
		rating = "fair"
		recommendation = "Requires investigation before production"
	default:
		rating = "poor"
		recommendation = "Not recommended for production"
	}

	return rating, recommendation
}

// FormatConsole formats the final report for console output
func (r *FinalReport) FormatConsole() string {
	output := fmt.Sprintf("wgmesh Pilot Final Report: %s\n", r.PilotID)
	output += fmt.Sprintf("%s\n\n", strings.Repeat("=", 70))

	output += fmt.Sprintf("Organization: %s\n", r.Organization)
	output += fmt.Sprintf("Contact: %s\n\n", r.ContactEmail)

	output += fmt.Sprintf("PILOT DURATION\n")
	output += fmt.Sprintf("Start: %s\n", r.StartDate.Format("2006-01-02"))
	output += fmt.Sprintf("End: %s\n", r.EndDate.Format("2006-01-02"))
	output += fmt.Sprintf("Duration: %d days\n\n", r.DurationDays)

	output += fmt.Sprintf("CONFIGURATION\n")
	output += fmt.Sprintf("Mode: %s\n", r.Mode)
	output += fmt.Sprintf("Node Count: %d\n\n", r.NodeCount)

	output += fmt.Sprintf("MILESTONE COMPLETION\n")
	for _, milestone := range r.Milestones {
		status := "✗ Not completed"
		if milestone.Completed {
			status = fmt.Sprintf("✓ Completed (Day %d)",
				int(milestone.CompletedAt.Sub(r.StartDate).Hours()/24))
		}
		output += fmt.Sprintf("  %s: %s\n", milestone.Name, status)
	}
	output += "\n"

	output += fmt.Sprintf("SUMMARY METRICS\n")
	output += fmt.Sprintf("  Mesh Connectivity Avg: %.2f%%\n", r.Summary.MeshConnectivityAvg)
	output += fmt.Sprintf("  Peer Discovery Success: %.1f%%\n", r.Summary.PeerDiscoverySuccess*100)
	output += fmt.Sprintf("  Total Daemon Restarts: %d\n", r.Summary.TotalDaemonRestarts)
	output += fmt.Sprintf("  Total WireGuard Restarts: %d\n\n", r.Summary.TotalWGRestarts)

	output += fmt.Sprintf("EVALUATION\n")
	output += fmt.Sprintf("  Overall Rating: %s\n", strings.ToUpper(r.Summary.OverallRating))
	output += fmt.Sprintf("  Recommendation: %s\n\n", r.Summary.Recommendation)

	output += fmt.Sprintf("Completed at: %s\n", r.CompletedAt.Format(time.RFC3339))

	return output
}

// FormatJSON formats the final report as JSON
func (r *FinalReport) FormatJSON() string {
	return fmt.Sprintf(`{
  "pilot_id": "%s",
  "organization": "%s",
  "contact_email": "%s",
  "start_date": "%s",
  "end_date": "%s",
  "duration_days": %d,
  "mode": "%s",
  "node_count": %d,
  "summary": {
    "all_milestones_completed": %t,
    "mesh_connectivity_avg": %.2f,
    "peer_discovery_success": %.2f,
    "total_daemon_restarts": %d,
    "total_wg_restarts": %d,
    "overall_rating": "%s",
    "recommendation": "%s"
  },
  "completed_at": "%s"
}`,
		r.PilotID,
		r.Organization,
		r.ContactEmail,
		r.StartDate.Format(time.RFC3339),
		r.EndDate.Format(time.RFC3339),
		r.DurationDays,
		r.Mode,
		r.NodeCount,
		r.Summary.AllMilestonesCompleted,
		r.Summary.MeshConnectivityAvg,
		r.Summary.PeerDiscoverySuccess,
		r.Summary.TotalDaemonRestarts,
		r.Summary.TotalWGRestarts,
		r.Summary.OverallRating,
		r.Summary.Recommendation,
		r.CompletedAt.Format(time.RFC3339),
	)
}
