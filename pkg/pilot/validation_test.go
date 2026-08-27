package pilot

import (
	"strings"
	"testing"
	"time"
)

func setupValidatedPilot(t *testing.T) *Pilot {
	t.Helper()
	p := New("")
	if err := p.Initialize("Test Corp", "admin@test.com", 5, "decentralized", 30); err != nil {
		t.Fatalf("failed to initialize pilot: %v", err)
	}
	if err := p.Start(); err != nil {
		t.Fatalf("failed to start pilot: %v", err)
	}
	return p
}

func TestValidate_NotStarted(t *testing.T) {
	p := New("")
	p.Initialize("Test Corp", "admin@test.com", 5, "decentralized", 30)

	_, err := p.Validate()
	if err == nil {
		t.Error("expected error when validating unstarted pilot")
	}
	if !strings.Contains(err.Error(), "not started") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_AllPass(t *testing.T) {
	p := setupValidatedPilot(t)

	// Set good metrics
	p.metrics.RecordMeshConnectivity(99.95)

	result, err := p.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Passed {
		t.Error("validation should pass with good metrics")
	}
	if len(result.Checks) == 0 {
		t.Error("expected validation checks")
	}
	if result.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if result.ValidatedAt.IsZero() {
		t.Error("expected non-zero validated_at")
	}
}

func TestValidate_LowConnectivity(t *testing.T) {
	p := setupValidatedPilot(t)

	p.metrics.RecordMeshConnectivity(90.0) // Below 99.9%

	result, err := p.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Passed {
		t.Error("validation should fail with low connectivity (error severity)")
	}

	// Find the connectivity check
	found := false
	for _, check := range result.Checks {
		if check.Name == "Mesh Connectivity" {
			found = true
			if check.Passed {
				t.Error("mesh connectivity check should fail")
			}
			if check.Severity != "error" {
				t.Errorf("expected error severity, got %s", check.Severity)
			}
		}
	}
	if !found {
		t.Error("mesh connectivity check not found")
	}
}

func TestValidate_DaemonRestarts(t *testing.T) {
	p := setupValidatedPilot(t)

	p.metrics.RecordMeshConnectivity(99.95)
	p.metrics.RecordDaemonRestart()

	result, err := p.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Passed {
		t.Error("validation should fail with daemon restarts (error severity)")
	}

	found := false
	for _, check := range result.Checks {
		if check.Name == "System Stability" {
			found = true
			if check.Passed {
				t.Error("stability check should fail with daemon restarts")
			}
			if check.Severity != "error" {
				t.Errorf("expected error severity, got %s", check.Severity)
			}
		}
	}
	if !found {
		t.Error("system stability check not found")
	}
}

func TestValidate_WireGuardRestarts(t *testing.T) {
	p := setupValidatedPilot(t)

	p.metrics.RecordMeshConnectivity(99.95)
	p.metrics.RecordWireGuardRestart()

	result, err := p.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Passed {
		t.Error("validation should fail with WireGuard restarts (error severity)")
	}
}

func TestValidate_PeerDiscovery(t *testing.T) {
	p := setupValidatedPilot(t)

	p.metrics.RecordMeshConnectivity(99.95)
	// PeerDiscoverySuccess is 0 by default, which is less than expected

	result, err := p.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, check := range result.Checks {
		if check.Name == "Peer Discovery" {
			found = true
			if check.Passed {
				t.Error("peer discovery check should fail with 0 discovery success")
			}
			if check.Severity != "warning" {
				t.Errorf("expected warning severity, got %s", check.Severity)
			}
		}
	}
	if !found {
		t.Error("peer discovery check not found")
	}
}

func TestValidate_RoutePropagation(t *testing.T) {
	p := setupValidatedPilot(t)

	p.metrics.RecordMeshConnectivity(99.95)
	// Route propagation time is 0 by default, which should pass (not measured yet)

	result, err := p.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, check := range result.Checks {
		if check.Name == "Route Propagation" {
			found = true
			if !check.Passed {
				t.Error("route propagation should pass when not measured (0)")
			}
		}
	}
	if !found {
		t.Error("route propagation check not found")
	}
}

func TestValidate_RoutePropagationSlow(t *testing.T) {
	p := setupValidatedPilot(t)

	p.metrics.RecordMeshConnectivity(99.95)
	p.metrics.RecordRoutePropagation(60 * time.Second) // Over 30s target

	result, err := p.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, check := range result.Checks {
		if check.Name == "Route Propagation" {
			found = true
			if check.Passed {
				t.Error("route propagation should fail when over target")
			}
			if check.Severity != "warning" {
				t.Errorf("expected warning severity, got %s", check.Severity)
			}
		}
	}
	if !found {
		t.Error("route propagation check not found")
	}
}

func TestValidate_ConfigInvalid(t *testing.T) {
	p := setupValidatedPilot(t)

	// Manually corrupt config to test validation
	p.mu.Lock()
	p.state.Config.PilotID = ""
	p.mu.Unlock()

	result, err := p.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, check := range result.Checks {
		if check.Name == "Pilot Configuration" {
			found = true
			if check.Passed {
				t.Error("config check should fail with empty pilot ID")
			}
		}
	}
	if !found {
		t.Error("pilot configuration check not found")
	}
}

func TestValidate_MilestoneProgress(t *testing.T) {
	p := setupValidatedPilot(t)

	p.metrics.RecordMeshConnectivity(99.95)

	result, err := p.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, check := range result.Checks {
		if check.Name == "Milestone Progress" {
			found = true
			// Day 0, no milestones overdue, should pass
			if !check.Passed {
				t.Error("milestone progress should pass on day 0")
			}
		}
	}
	if !found {
		t.Error("milestone progress check not found")
	}
}

func TestValidate_SixChecks(t *testing.T) {
	p := setupValidatedPilot(t)

	result, err := p.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedChecks := []string{
		"Pilot Configuration",
		"Mesh Connectivity",
		"Peer Discovery",
		"Route Propagation",
		"System Stability",
		"Milestone Progress",
	}

	if len(result.Checks) != len(expectedChecks) {
		t.Errorf("expected %d checks, got %d", len(expectedChecks), len(result.Checks))
	}

	for _, name := range expectedChecks {
		found := false
		for _, check := range result.Checks {
			if check.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected check %q not found", name)
		}
	}
}

func TestValidationResult_FormatConsole(t *testing.T) {
	p := setupValidatedPilot(t)
	p.metrics.RecordMeshConnectivity(99.95)

	result, err := p.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := result.FormatConsole()

	if !strings.Contains(output, "Pilot Validation Results") {
		t.Error("expected header in console output")
	}
	if !strings.Contains(output, "Summary:") {
		t.Error("expected summary in console output")
	}
	if !strings.Contains(output, "Validated at:") {
		t.Error("expected timestamp in console output")
	}
}

func TestValidationResult_FormatJSON(t *testing.T) {
	p := setupValidatedPilot(t)
	p.metrics.RecordMeshConnectivity(99.95)

	result, err := p.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := result.FormatJSON()

	if !strings.Contains(output, `"passed"`) {
		t.Error("expected 'passed' field in JSON output")
	}
	if !strings.Contains(output, `"summary"`) {
		t.Error("expected 'summary' field in JSON output")
	}
	if !strings.Contains(output, `"validated_at"`) {
		t.Error("expected 'validated_at' field in JSON output")
	}
	if !strings.Contains(output, `"checks"`) {
		t.Error("expected 'checks' field in JSON output")
	}
}

func TestValidate_WarningOnlyPasses(t *testing.T) {
	p := setupValidatedPilot(t)

	p.metrics.RecordMeshConnectivity(99.95)
	p.metrics.RecordRoutePropagation(60 * time.Second) // Warning, not error

	result, err := p.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Warnings should not cause overall failure
	if !result.Passed {
		t.Error("validation should pass with only warnings (no errors)")
	}

	if !strings.Contains(result.Summary, "warnings") {
		t.Errorf("summary should mention warnings: %s", result.Summary)
	}
}

func TestValidate_ErrorFails(t *testing.T) {
	p := setupValidatedPilot(t)

	p.metrics.RecordMeshConnectivity(90.0) // Error severity

	result, err := p.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Passed {
		t.Error("validation should fail with error severity checks")
	}

	if !strings.Contains(result.Summary, "failed") {
		t.Errorf("summary should mention failure: %s", result.Summary)
	}
}

func TestValidate_AllChecksPassed(t *testing.T) {
	p := setupValidatedPilot(t)

	p.metrics.RecordMeshConnectivity(99.99)
	p.metrics.RecordRoutePropagation(10 * time.Second)
	p.metrics.PeerDiscoverySuccess = 1.0 // All peers discovered

	result, err := p.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Passed {
		t.Error("validation should pass with all good metrics")
	}

	// All checks should be passing
	for _, check := range result.Checks {
		if !check.Passed {
			t.Errorf("check %q should pass", check.Name)
		}
	}

	if !strings.Contains(result.Summary, "All checks passed") {
		t.Errorf("summary should say all checks passed: %s", result.Summary)
	}
}
