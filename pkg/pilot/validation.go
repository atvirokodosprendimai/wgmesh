package pilot

import (
	"fmt"
	"strings"
	"time"
)

// ValidationResult represents the result of validation checks
type ValidationResult struct {
	Passed      bool
	Checks      []*CheckResult
	Summary     string
	ValidatedAt time.Time
}

// CheckResult represents a single validation check
type CheckResult struct {
	Name     string
	Passed   bool
	Message  string
	Severity string // "info", "warning", "error"
}

// Validate runs health checks and returns validation results
func (p *Pilot) Validate() (*ValidationResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.state.Started {
		return nil, fmt.Errorf("pilot not started")
	}

	result := &ValidationResult{
		Checks:      make([]*CheckResult, 0),
		ValidatedAt: time.Now(),
	}

	// Update state
	p.state.DaysElapsed = int(time.Since(p.state.Config.StartDate).Hours() / 24)
	metricsSnapshot := p.metrics.Snapshot()

	// Check 1: Pilot configuration is valid
	result.Checks = append(result.Checks, &CheckResult{
		Name:     "Pilot Configuration",
		Passed:   p.validateConfig(),
		Message:  "Pilot configuration is valid",
		Severity: "info",
	})

	// Check 2: Mesh connectivity
	connCheck := p.validateMeshConnectivity(metricsSnapshot)
	result.Checks = append(result.Checks, connCheck)

	// Check 3: Peer discovery
	discoveryCheck := p.validatePeerDiscovery(metricsSnapshot)
	result.Checks = append(result.Checks, discoveryCheck)

	// Check 4: Route propagation
	routeCheck := p.validateRoutePropagation(metricsSnapshot)
	result.Checks = append(result.Checks, routeCheck)

	// Check 5: Stability (no excessive restarts)
	stabilityCheck := p.validateStability(metricsSnapshot)
	result.Checks = append(result.Checks, stabilityCheck)

	// Check 6: Milestone progress
	progressCheck := p.validateMilestoneProgress()
	result.Checks = append(result.Checks, progressCheck)

	// Determine overall pass status
	result.Passed = p.computeOverallPass(result.Checks)
	result.Summary = p.generateValidationSummary(result.Checks)

	return result, nil
}

// validateConfig checks if the pilot configuration is valid
func (p *Pilot) validateConfig() bool {
	return p.state.Config.PilotID != "" &&
		p.state.Config.Organization != "" &&
		p.state.Config.ContactEmail != "" &&
		p.state.Config.NodeCount > 0
}

// validateMeshConnectivity checks mesh connectivity against target
func (p *Pilot) validateMeshConnectivity(metrics *Metrics) *CheckResult {
	target := p.state.Config.MetricsTargets.MeshConnectivity
	actual := metrics.MeshUptimePercent

	passed := actual >= target
	message := fmt.Sprintf("Mesh connectivity: %.2f%% (target: ≥%.1f%%)", actual, target)
	severity := "info"
	if !passed {
		severity = "error"
	}

	return &CheckResult{
		Name:     "Mesh Connectivity",
		Passed:   passed,
		Message:  message,
		Severity: severity,
	}
}

// validatePeerDiscovery checks peer discovery success rate
func (p *Pilot) validatePeerDiscovery(metrics *Metrics) *CheckResult {
	// Expect all nodes discovered
	expectedPeers := p.state.Config.NodeCount
	actualPeers := int(metrics.PeerDiscoverySuccess * float64(expectedPeers))

	passed := actualPeers >= expectedPeers
	message := fmt.Sprintf("Peer discovery: %d/%d nodes discovered", actualPeers, expectedPeers)
	severity := "info"
	if !passed {
		severity = "warning"
	}

	return &CheckResult{
		Name:     "Peer Discovery",
		Passed:   passed,
		Message:  message,
		Severity: severity,
	}
}

// validateRoutePropagation checks route propagation time
func (p *Pilot) validateRoutePropagation(metrics *Metrics) *CheckResult {
	target := p.state.Config.MetricsTargets.RoutePropagation
	actual := int(metrics.RoutePropagationTime.Seconds())

	passed := actual <= target || actual == 0 // 0 means not measured yet
	message := fmt.Sprintf("Route propagation: %ds (target: ≤%ds)", actual, target)
	severity := "info"
	if !passed && actual > 0 {
		severity = "warning"
	}

	return &CheckResult{
		Name:     "Route Propagation",
		Passed:   passed,
		Message:  message,
		Severity: severity,
	}
}

// validateStability checks for excessive restarts
func (p *Pilot) validateStability(metrics *Metrics) *CheckResult {
	daemonRestarts := metrics.DaemonRestarts
	wgRestarts := metrics.WireGuardRestarts

	passed := daemonRestarts == 0 && wgRestarts == 0
	message := fmt.Sprintf("Daemon restarts: %d, WireGuard restarts: %d", daemonRestarts, wgRestarts)
	severity := "info"
	if !passed {
		severity = "error"
	}

	return &CheckResult{
		Name:     "System Stability",
		Passed:   passed,
		Message:  message,
		Severity: severity,
	}
}

// validateMilestoneProgress checks if milestones are on track
func (p *Pilot) validateMilestoneProgress() *CheckResult {
	overdueCount := 0
	completedCount := 0

	for _, milestone := range p.state.Config.Milestones {
		if milestone.Completed {
			completedCount++
		} else if p.state.DaysElapsed > int(milestone.TargetDate.Sub(p.state.Config.StartDate).Hours()/24) {
			overdueCount++
		}
	}

	passed := overdueCount == 0
	message := fmt.Sprintf("Milestones: %d completed, %d overdue", completedCount, overdueCount)
	severity := "info"
	if !passed {
		severity = "warning"
	}

	return &CheckResult{
		Name:     "Milestone Progress",
		Passed:   passed,
		Message:  message,
		Severity: severity,
	}
}

// computeOverallPass determines if validation passed overall
func (p *Pilot) computeOverallPass(checks []*CheckResult) bool {
	// Fail if any error severity check failed
	for _, check := range checks {
		if check.Severity == "error" && !check.Passed {
			return false
		}
	}
	return true
}

// generateValidationSummary creates a summary of validation results
func (p *Pilot) generateValidationSummary(checks []*CheckResult) string {
	passedCount := 0
	failedCount := 0
	warningCount := 0

	for _, check := range checks {
		if check.Passed {
			passedCount++
		} else {
			if check.Severity == "error" {
				failedCount++
			} else {
				warningCount++
			}
		}
	}

	if failedCount > 0 {
		return fmt.Sprintf("Validation failed: %d passed, %d failed, %d warnings",
			passedCount, failedCount, warningCount)
	}
	if warningCount > 0 {
		return fmt.Sprintf("Validation passed with warnings: %d passed, %d warnings",
			passedCount, warningCount)
	}
	return fmt.Sprintf("All checks passed: %d/%d", passedCount, len(checks))
}

// FormatConsole formats validation results for console output
func (v *ValidationResult) FormatConsole() string {
	var output string

	output += fmt.Sprintf("Pilot Validation Results\n")
	output += fmt.Sprintf("%s\n\n", strings.Repeat("=", 70))

	for _, check := range v.Checks {
		status := "✓"
		if !check.Passed {
			status = "✗"
		}
		severity := strings.ToUpper(check.Severity)
		output += fmt.Sprintf("%s [%s] %s: %s\n", status, severity, check.Name, check.Message)
	}

	output += fmt.Sprintf("\n%s\n", strings.Repeat("=", 70))
	output += fmt.Sprintf("Summary: %s\n", v.Summary)
	output += fmt.Sprintf("Validated at: %s\n", v.ValidatedAt.Format(time.RFC3339))

	return output
}

// FormatJSON formats validation results as JSON
func (v *ValidationResult) FormatJSON() string {
	// Simple JSON representation
	return fmt.Sprintf(`{
  "passed": %t,
  "summary": "%s",
  "validated_at": "%s",
  "checks": %d
}`, v.Passed, v.Summary, v.ValidatedAt.Format(time.RFC3339), len(v.Checks))
}
