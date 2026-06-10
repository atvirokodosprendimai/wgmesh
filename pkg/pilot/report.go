package pilot

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// ReportFormat defines the output format
type ReportFormat string

const (
	FormatConsole ReportFormat = "console"
	FormatJSON    ReportFormat = "json"
	FormatHTML    ReportFormat = "html"
)

// GenerateReport produces evaluation reports in specified format
func (p *Pilot) GenerateReport(format ReportFormat, outputPath string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.state.Started {
		return fmt.Errorf("pilot not started")
	}

	// Update state before generating report
	p.state.DaysElapsed = int(time.Since(p.state.Config.StartDate).Hours() / 24)
	p.state.CurrentPhase = getCurrentPhase(p.state.DaysElapsed)

	// Get metrics snapshot
	metricsSnapshot := p.metrics.Snapshot()
	metricsSnapshot.SetPhase(p.state.CurrentPhase)
	metricsSnapshot.SetDaysElapsed(p.state.DaysElapsed)

	var output string

	switch format {
	case FormatConsole:
		output = p.generateConsoleReport(metricsSnapshot)
	case FormatJSON:
		output = p.generateJSONReport(metricsSnapshot)
	case FormatHTML:
		output = p.generateHTMLReport(metricsSnapshot)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}

	// Write output
	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}
		fmt.Printf("Report written to %s\n", outputPath)
	} else {
		fmt.Print(output)
	}

	return nil
}

// generateConsoleReport creates a human-readable console report
func (p *Pilot) generateConsoleReport(metrics *Metrics) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("wgmesh Pilot Report: %s\n", p.state.Config.PilotID))
	sb.WriteString(strings.Repeat("=", 70))
	sb.WriteString("\n\n")

	sb.WriteString(fmt.Sprintf("Phase: %s (Day %d of %d)\n",
		p.state.CurrentPhase,
		p.state.DaysElapsed,
		int(p.state.Config.EndDate.Sub(p.state.Config.StartDate).Hours()/24)))
	sb.WriteString(fmt.Sprintf("Organization: %s | Contact: %s\n\n",
		p.state.Config.Organization,
		p.state.Config.ContactEmail))

	// Milestone status
	sb.WriteString("MILESTONE STATUS\n")
	sb.WriteString(strings.Repeat("-", 70))
	sb.WriteString("\n")

	milestoneOrder := []string{"baseline", "mesh_stability", "production_traffic", "advanced_scenarios"}
	for _, key := range milestoneOrder {
		milestone, exists := p.state.Config.Milestones[key]
		if !exists {
			continue
		}

		status := "○"
		if milestone.Completed {
			status = "✓"
		} else if p.state.DaysElapsed > int(milestone.TargetDate.Sub(p.state.Config.StartDate).Hours()/24) {
			status = "⚠" // Overdue
		}

		sb.WriteString(fmt.Sprintf("%s %s", status, milestone.Name))
		if milestone.Completed {
			sb.WriteString(fmt.Sprintf(" (completed Day %d)",
				int(milestone.CompletedAt.Sub(p.state.Config.StartDate).Hours()/24)))
		} else {
			targetDay := int(milestone.TargetDate.Sub(p.state.Config.StartDate).Hours() / 24)
			sb.WriteString(fmt.Sprintf(" (target: Day %d)", targetDay))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Key metrics
	sb.WriteString("KEY METRICS (last 24h)\n")
	sb.WriteString(strings.Repeat("-", 70))
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("Mesh Connectivity: %.2f%% (target: ≥%.1f%%)\n",
		metrics.MeshUptimePercent,
		p.state.Config.MetricsTargets.MeshConnectivity))

	totalDiscoveries := 0
	for _, count := range metrics.DiscoveryLayerCounts {
		totalDiscoveries += count
	}
	if totalDiscoveries > 0 {
		sb.WriteString(fmt.Sprintf("Peer Discovery: %.1f%% (%d/%d discovered)\n",
			metrics.PeerDiscoverySuccess,
			int(metrics.PeerDiscoverySuccess*float64(p.state.Config.NodeCount)),
			p.state.Config.NodeCount))
	} else {
		sb.WriteString(fmt.Sprintf("Peer Discovery: No discovery events recorded\n"))
	}

	sb.WriteString(fmt.Sprintf("Route Propagation: %s avg (target: ≤%ds)\n",
		formatDuration(metrics.RoutePropagationTime),
		p.state.Config.MetricsTargets.RoutePropagation))

	sb.WriteString(fmt.Sprintf("Daemon Restarts: %d\n", metrics.DaemonRestarts))
	sb.WriteString(fmt.Sprintf("WireGuard Restarts: %d\n", metrics.WireGuardRestarts))
	sb.WriteString("\n")

	// Discovery layer distribution
	if len(metrics.DiscoveryLayerCounts) > 0 {
		sb.WriteString("DISCOVERY LAYER DISTRIBUTION\n")
		sb.WriteString(strings.Repeat("-", 70))
		sb.WriteString("\n")
		for layer, count := range metrics.DiscoveryLayerCounts {
			sb.WriteString(fmt.Sprintf("  %s: %d\n", layer, count))
		}
		sb.WriteString("\n")
	}

	// NAT types
	if len(metrics.NATTypes) > 0 {
		sb.WriteString("NAT TYPES DETECTED\n")
		sb.WriteString(strings.Repeat("-", 70))
		sb.WriteString("\n")
		for natType, count := range metrics.NATTypes {
			sb.WriteString(fmt.Sprintf("  %s: %d nodes\n", natType, count))
		}
		sb.WriteString("\n")
	}

	// Issues / warnings
	sb.WriteString("ISSUES / WARNINGS\n")
	sb.WriteString(strings.Repeat("-", 70))
	sb.WriteString("\n")

	hasWarnings := false

	if metrics.MeshUptimePercent < p.state.Config.MetricsTargets.MeshConnectivity {
		sb.WriteString(fmt.Sprintf("[WARN] Mesh connectivity (%.2f%%) below target (%.1f%%)\n",
			metrics.MeshUptimePercent,
			p.state.Config.MetricsTargets.MeshConnectivity))
		hasWarnings = true
	}

	if metrics.DaemonRestarts > 0 {
		sb.WriteString(fmt.Sprintf("[WARN] %d daemon restart(s) detected\n", metrics.DaemonRestarts))
		hasWarnings = true
	}

	if metrics.WireGuardRestarts > 0 {
		sb.WriteString(fmt.Sprintf("[WARN] %d WireGuard restart(s) detected\n", metrics.WireGuardRestarts))
		hasWarnings = true
	}

	if metrics.RelayFallbackCount > 0 {
		sb.WriteString(fmt.Sprintf("[INFO] %d relay fallback(s) recorded\n", metrics.RelayFallbackCount))
		hasWarnings = true
	}

	if !hasWarnings {
		sb.WriteString("[INFO] No issues detected\n")
	}

	sb.WriteString("\n")

	// Next steps
	sb.WriteString("NEXT STEPS\n")
	sb.WriteString(strings.Repeat("-", 70))
	sb.WriteString("\n")

	// Determine next milestone
	nextMilestone := getNextMilestone(p.state.Config.Milestones, p.state.DaysElapsed)
	if nextMilestone != nil {
		if nextMilestone.Completed {
			sb.WriteString(fmt.Sprintf("→ Complete and proceed to next phase\n"))
		} else {
			targetDay := int(nextMilestone.TargetDate.Sub(p.state.Config.StartDate).Hours() / 24)
			sb.WriteString(fmt.Sprintf("→ Work towards %s milestone (target: Day %d)\n",
				nextMilestone.Name,
				targetDay))
		}
	} else {
		sb.WriteString("→ All milestones completed\n")
	}

	sb.WriteString("\n")

	return sb.String()
}

// generateJSONReport creates a JSON format report
func (p *Pilot) generateJSONReport(metrics *Metrics) string {
	report := map[string]interface{}{
		"pilot_id":      p.state.Config.PilotID,
		"organization":  p.state.Config.Organization,
		"contact_email": p.state.Config.ContactEmail,
		"mode":          p.state.Config.Mode,
		"node_count":    p.state.Config.NodeCount,
		"start_date":    p.state.Config.StartDate.Format(time.RFC3339),
		"end_date":      p.state.Config.EndDate.Format(time.RFC3339),
		"current_phase": p.state.CurrentPhase,
		"days_elapsed":  p.state.DaysElapsed,
		"milestones":    p.state.Config.Milestones,
		"metrics": map[string]interface{}{
			"mesh_uptime_percent":    metrics.MeshUptimePercent,
			"peer_discovery_success": metrics.PeerDiscoverySuccess,
			"route_propagation_time": metrics.RoutePropagationTime.String(),
			"throughput_mbps":        metrics.ThroughputMbps,
			"latency_ms":             metrics.LatencyMs,
			"connection_overhead_ms": metrics.ConnectionOverheadMs,
			"daemon_restarts":        metrics.DaemonRestarts,
			"wireguard_restarts":     metrics.WireGuardRestarts,
			"network_partitions":     metrics.NetworkPartitions,
			"recovery_time_sec":      metrics.RecoveryTimeSec,
			"discovery_layer_counts": metrics.DiscoveryLayerCounts,
			"nat_types":              metrics.NATTypes,
			"hole_punch_success":     metrics.HolePunchSuccess,
			"relay_fallback_count":   metrics.RelayFallbackCount,
		},
		"targets":      p.state.Config.MetricsTargets,
		"generated_at": time.Now().Format(time.RFC3339),
	}

	data, _ := json.MarshalIndent(report, "", "  ")
	return string(data)
}

// generateHTMLReport creates an HTML format report
func (p *Pilot) generateHTMLReport(metrics *Metrics) string {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>wgmesh Pilot Report: ` + p.state.Config.PilotID + `</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 1000px; margin: 0 auto; background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 2px solid #4CAF50; padding-bottom: 10px; }
        h2 { color: #666; margin-top: 30px; font-size: 18px; }
        .header { background: #f8f9fa; padding: 15px; border-radius: 5px; margin-bottom: 20px; }
        .metrics { display: grid; grid-template-columns: repeat(2, 1fr); gap: 15px; margin: 20px 0; }
        .metric-card { background: #f8f9fa; padding: 15px; border-radius: 5px; border-left: 4px solid #4CAF50; }
        .metric-label { font-size: 12px; color: #666; text-transform: uppercase; }
        .metric-value { font-size: 24px; font-weight: bold; color: #333; margin-top: 5px; }
        .milestone { padding: 10px; margin: 5px 0; border-radius: 3px; }
        .milestone.completed { background: #d4edda; border-left: 4px solid #28a745; }
        .milestone.pending { background: #fff3cd; border-left: 4px solid #ffc107; }
        .milestone.overdue { background: #f8d7da; border-left: 4px solid #dc3545; }
        .warning { background: #fff3cd; padding: 10px; border-radius: 5px; margin: 5px 0; border-left: 4px solid #ffc107; }
        .info { background: #d1ecf1; padding: 10px; border-radius: 5px; margin: 5px 0; border-left: 4px solid #17a2b8; }
        table { width: 100%; border-collapse: collapse; margin: 15px 0; }
        th, td { padding: 10px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background: #f8f9fa; }
        .timestamp { font-size: 12px; color: #999; }
    </style>
</head>
<body>
    <div class="container">
        <h1>wgmesh Pilot Report</h1>
        
        <div class="header">
            <strong>` + p.state.Config.PilotID + `</strong><br>
            Phase: ` + p.state.CurrentPhase + ` (Day ` + fmt.Sprintf("%d", p.state.DaysElapsed) + ` of ` + fmt.Sprintf("%d", int(p.state.Config.EndDate.Sub(p.state.Config.StartDate).Hours()/24)) + `)<br>
            Organization: ` + p.state.Config.Organization + ` | Contact: ` + p.state.Config.ContactEmail + `
        </div>

        <h2>Milestone Status</h2>
`

	milestoneOrder := []string{"baseline", "mesh_stability", "production_traffic", "advanced_scenarios"}
	for _, key := range milestoneOrder {
		milestone, exists := p.state.Config.Milestones[key]
		if !exists {
			continue
		}

		cssClass := "pending"
		if milestone.Completed {
			cssClass = "completed"
		} else if p.state.DaysElapsed > int(milestone.TargetDate.Sub(p.state.Config.StartDate).Hours()/24) {
			cssClass = "overdue"
		}

		statusText := "In Progress"
		if milestone.Completed {
			statusText = fmt.Sprintf("Completed Day %d", int(milestone.CompletedAt.Sub(p.state.Config.StartDate).Hours()/24))
		} else {
			targetDay := int(milestone.TargetDate.Sub(p.state.Config.StartDate).Hours() / 24)
			statusText = fmt.Sprintf("Target: Day %d", targetDay)
		}

		html += fmt.Sprintf(`        <div class="milestone %s">
            %s - %s
        </div>`, cssClass, milestone.Name, statusText)
	}

	html += `
        <h2>Key Metrics (Last 24h)</h2>
        <div class="metrics">
            <div class="metric-card">
                <div class="metric-label">Mesh Connectivity</div>
                <div class="metric-value">` + fmt.Sprintf("%.2f%%", metrics.MeshUptimePercent) + `</div>
            </div>
            <div class="metric-card">
                <div class="metric-label">Peer Discovery Success</div>
                <div class="metric-value">` + fmt.Sprintf("%.1f%%", metrics.PeerDiscoverySuccess) + `</div>
            </div>
            <div class="metric-card">
                <div class="metric-label">Route Propagation</div>
                <div class="metric-value">` + formatDuration(metrics.RoutePropagationTime) + `</div>
            </div>
            <div class="metric-card">
                <div class="metric-label">Daemon Restarts</div>
                <div class="metric-value">` + fmt.Sprintf("%d", metrics.DaemonRestarts) + `</div>
            </div>
        </div>
`

	if len(metrics.DiscoveryLayerCounts) > 0 {
		html += `
        <h2>Discovery Layer Distribution</h2>
        <table>
            <tr><th>Layer</th><th>Count</th></tr>
`
		for layer, count := range metrics.DiscoveryLayerCounts {
			html += fmt.Sprintf("            <tr><td>%s</td><td>%d</td></tr>\n", layer, count)
		}
		html += `        </table>
`
	}

	if len(metrics.NATTypes) > 0 {
		html += `
        <h2>NAT Types Detected</h2>
        <table>
            <tr><th>Type</th><th>Nodes</th></tr>
`
		for natType, count := range metrics.NATTypes {
			html += fmt.Sprintf("            <tr><td>%s</td><td>%d</td></tr>\n", natType, count)
		}
		html += `        </table>
`
	}

	html += `
        <h2>Issues / Warnings</h2>
`

	hasWarnings := false
	if metrics.MeshUptimePercent < p.state.Config.MetricsTargets.MeshConnectivity {
		html += fmt.Sprintf(`        <div class="warning">Mesh connectivity (%.2f%%) below target (%.1f%%)</div>`,
			metrics.MeshUptimePercent, p.state.Config.MetricsTargets.MeshConnectivity)
		hasWarnings = true
	}

	if metrics.DaemonRestarts > 0 {
		html += fmt.Sprintf(`        <div class="warning">%d daemon restart(s) detected</div>`, metrics.DaemonRestarts)
		hasWarnings = true
	}

	if metrics.WireGuardRestarts > 0 {
		html += fmt.Sprintf(`        <div class="warning">%d WireGuard restart(s) detected</div>`, metrics.WireGuardRestarts)
		hasWarnings = true
	}

	if metrics.RelayFallbackCount > 0 {
		html += fmt.Sprintf(`        <div class="info">%d relay fallback(s) recorded</div>`, metrics.RelayFallbackCount)
		hasWarnings = true
	}

	if !hasWarnings {
		html += `        <div class="info">No issues detected</div>`
	}

	html += `
        <div class="timestamp">
            Generated: ` + time.Now().Format(time.RFC3339) + `
        </div>
    </div>
</body>
</html>
`

	return html
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// getNextMilestone finds the next uncompleted milestone
func getNextMilestone(milestones map[string]*Milestone, daysElapsed int) *Milestone {
	order := []string{"baseline", "mesh_stability", "production_traffic", "advanced_scenarios"}
	for _, key := range order {
		milestone := milestones[key]
		if milestone != nil && !milestone.Completed {
			return milestone
		}
	}
	return nil
}
