package pilot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupStartedPilot(t *testing.T) *Pilot {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pilot.yaml")
	p := New(configPath)
	if err := p.Initialize("Test Corp", "admin@test.com", 5, "decentralized", 30); err != nil {
		t.Fatalf("failed to initialize pilot: %v", err)
	}
	if err := p.Start(); err != nil {
		t.Fatalf("failed to start pilot: %v", err)
	}
	return p
}

func readReportOutput(t *testing.T, p *Pilot, format ReportFormat) string {
	t.Helper()
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "report.out")

	if err := p.GenerateReport(format, outputPath); err != nil {
		t.Fatalf("GenerateReport(%s) error: %v", format, err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read report output: %v", err)
	}
	return string(data)
}

func TestGenerateReport_Console(t *testing.T) {
	p := setupStartedPilot(t)

	p.metrics.RecordMeshConnectivity(99.95)
	p.metrics.RecordPeerDiscovery("dht")
	p.metrics.RecordPeerDiscovery("lan")
	p.metrics.RecordDaemonRestart()
	p.metrics.RecordNATType("cone")

	output := readReportOutput(t, p, FormatConsole)

	assertContains(t, output, "wgmesh Pilot Report")
	assertContains(t, output, "Test Corp")
	assertContains(t, output, "admin@test.com")
	assertContains(t, output, "MILESTONE STATUS")
	assertContains(t, output, "KEY METRICS")
	assertContains(t, output, "ISSUES / WARNINGS")
	assertContains(t, output, "NEXT STEPS")
	assertContains(t, output, "99.95%")
	assertContains(t, output, "Baseline Setup")
	assertContains(t, output, "Mesh Stability")
}

func TestGenerateReport_ConsoleWithWarnings(t *testing.T) {
	p := setupStartedPilot(t)

	p.metrics.RecordMeshConnectivity(95.0) // Below target
	p.metrics.RecordDaemonRestart()
	p.metrics.RecordWireGuardRestart()

	output := readReportOutput(t, p, FormatConsole)

	assertContains(t, output, "[WARN]")
	assertContains(t, output, "daemon restart")
	assertContains(t, output, "WireGuard restart")
}

func TestGenerateReport_ConsoleNoWarnings(t *testing.T) {
	p := setupStartedPilot(t)

	p.metrics.RecordMeshConnectivity(99.99)

	output := readReportOutput(t, p, FormatConsole)

	assertContains(t, output, "No issues detected")
}

func TestGenerateReport_ConsoleDiscoveryDistribution(t *testing.T) {
	p := setupStartedPilot(t)

	p.metrics.RecordPeerDiscovery("registry")
	p.metrics.RecordPeerDiscovery("lan")
	p.metrics.RecordPeerDiscovery("dht")

	output := readReportOutput(t, p, FormatConsole)

	assertContains(t, output, "DISCOVERY LAYER DISTRIBUTION")
	assertContains(t, output, "registry")
	assertContains(t, output, "lan")
	assertContains(t, output, "dht")
}

func TestGenerateReport_ConsoleNATTypes(t *testing.T) {
	p := setupStartedPilot(t)

	p.metrics.RecordNATType("Full Cone")
	p.metrics.RecordNATType("Symmetric")

	output := readReportOutput(t, p, FormatConsole)

	assertContains(t, output, "NAT TYPES DETECTED")
	assertContains(t, output, "Full Cone")
	assertContains(t, output, "Symmetric")
}

func TestGenerateReport_ConsoleRelayFallback(t *testing.T) {
	p := setupStartedPilot(t)

	p.metrics.RecordRelayFallback()

	output := readReportOutput(t, p, FormatConsole)

	assertContains(t, output, "relay fallback")
}

func TestGenerateReport_ConsoleCompletedMilestone(t *testing.T) {
	p := setupStartedPilot(t)

	p.MarkMilestoneComplete("baseline")

	output := readReportOutput(t, p, FormatConsole)

	assertContains(t, output, "✓ Baseline Setup")
	assertContains(t, output, "completed Day")
}

func TestGenerateReport_JSON(t *testing.T) {
	p := setupStartedPilot(t)

	p.metrics.RecordMeshConnectivity(99.5)
	p.metrics.RecordPeerDiscovery("dht")
	p.metrics.RecordNATType("cone")

	output := readReportOutput(t, p, FormatJSON)

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	// Verify key fields
	assertJSONField(t, result, "pilot_id")
	assertJSONField(t, result, "organization")
	assertJSONField(t, result, "contact_email")
	assertJSONField(t, result, "current_phase")
	assertJSONField(t, result, "milestones")
	assertJSONField(t, result, "metrics")
	assertJSONField(t, result, "targets")
	assertJSONField(t, result, "generated_at")

	// Verify values
	if result["organization"] != "Test Corp" {
		t.Errorf("organization: got %v, want Test Corp", result["organization"])
	}
	if result["contact_email"] != "admin@test.com" {
		t.Errorf("contact_email: got %v, want admin@test.com", result["contact_email"])
	}

	// Verify metrics sub-object
	metrics, ok := result["metrics"].(map[string]interface{})
	if !ok {
		t.Fatal("metrics field is not a map")
	}
	assertJSONField(t, metrics, "mesh_uptime_percent")
	assertJSONField(t, metrics, "daemon_restarts")
	assertJSONField(t, metrics, "discovery_layer_counts")
	assertJSONField(t, metrics, "nat_types")

	// Verify metrics values
	if meshUp, ok := metrics["mesh_uptime_percent"].(float64); !ok || meshUp != 99.5 {
		t.Errorf("mesh_uptime_percent: got %v, want 99.5", metrics["mesh_uptime_percent"])
	}
}

func TestGenerateReport_JSONCompletedMilestone(t *testing.T) {
	p := setupStartedPilot(t)
	p.MarkMilestoneComplete("baseline")

	output := readReportOutput(t, p, FormatJSON)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	milestones, ok := result["milestones"].(map[string]interface{})
	if !ok {
		t.Fatal("milestones is not a map")
	}
	baseline, ok := milestones["baseline"].(map[string]interface{})
	if !ok {
		t.Fatal("baseline milestone is not a map")
	}
	// JSON marshaling uses Go field names (PascalCase) since Milestone has yaml but no json tags
	completed, ok := baseline["Completed"].(bool)
	if !ok || !completed {
		t.Errorf("baseline should be completed, got: %v", baseline)
	}
}

func TestGenerateReport_HTML(t *testing.T) {
	p := setupStartedPilot(t)

	p.metrics.RecordMeshConnectivity(99.5)
	p.metrics.RecordPeerDiscovery("dht")
	p.metrics.RecordNATType("cone")
	p.metrics.RecordDaemonRestart()

	output := readReportOutput(t, p, FormatHTML)

	// Verify HTML structure
	assertContains(t, output, "<!DOCTYPE html>")
	assertContains(t, output, "<html>")
	assertContains(t, output, "</html>")
	assertContains(t, output, "Test Corp")
	assertContains(t, output, "admin@test.com")
	assertContains(t, output, "Milestone Status")
	assertContains(t, output, "Key Metrics")
}

func TestGenerateReport_HTMLWithWarnings(t *testing.T) {
	p := setupStartedPilot(t)

	p.metrics.RecordMeshConnectivity(95.0)
	p.metrics.RecordDaemonRestart()
	p.metrics.RecordWireGuardRestart()

	output := readReportOutput(t, p, FormatHTML)

	assertContains(t, output, "warning")
	assertContains(t, output, "Issues / Warnings")
}

func TestGenerateReport_HTMLWithDiscoveryAndNAT(t *testing.T) {
	p := setupStartedPilot(t)

	p.metrics.RecordPeerDiscovery("registry")
	p.metrics.RecordPeerDiscovery("lan")
	p.metrics.RecordNATType("Full Cone")

	output := readReportOutput(t, p, FormatHTML)

	assertContains(t, output, "Discovery Layer Distribution")
	assertContains(t, output, "registry")
	assertContains(t, output, "NAT Types Detected")
	assertContains(t, output, "Full Cone")
}

func TestGenerateReport_HTMLNoWarnings(t *testing.T) {
	p := setupStartedPilot(t)

	p.metrics.RecordMeshConnectivity(99.99)

	output := readReportOutput(t, p, FormatHTML)

	assertContains(t, output, "No issues detected")
}

func TestGenerateReport_NotStarted(t *testing.T) {
	p := New("")
	p.Initialize("Test Corp", "admin@test.com", 5, "decentralized", 30)

	err := p.GenerateReport(FormatConsole, "")
	if err == nil {
		t.Error("expected error when generating report for unstarted pilot")
	}
	if !strings.Contains(err.Error(), "not started") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGenerateReport_InvalidFormat(t *testing.T) {
	p := setupStartedPilot(t)

	err := p.GenerateReport(ReportFormat("invalid"), "")
	if err == nil {
		t.Error("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGenerateReport_CompareAllFormats(t *testing.T) {
	p := setupStartedPilot(t)

	p.metrics.RecordMeshConnectivity(99.9)
	p.metrics.RecordPeerDiscovery("dht")
	p.metrics.RecordPeerDiscovery("lan")
	p.metrics.RecordNATType("cone")

	consoleOutput := readReportOutput(t, p, FormatConsole)
	jsonOutput := readReportOutput(t, p, FormatJSON)
	htmlOutput := readReportOutput(t, p, FormatHTML)

	// All formats should contain the pilot ID
	pilotID := p.Config().PilotID
	assertContains(t, consoleOutput, pilotID)
	assertContains(t, jsonOutput, pilotID)
	assertContains(t, htmlOutput, pilotID)

	// All formats should reference the org
	assertContains(t, consoleOutput, "Test Corp")
	assertContains(t, jsonOutput, "Test Corp")
	assertContains(t, htmlOutput, "Test Corp")

	// Verify each format has distinct markers
	assertContains(t, consoleOutput, "MILESTONE STATUS")
	assertContains(t, jsonOutput, `"pilot_id"`)
	assertContains(t, htmlOutput, "<!DOCTYPE html>")
}

// formatDuration tests

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "-"},
		{"milliseconds", 500 * time.Millisecond, "500ms"},
		{"seconds", 5 * time.Second, "5.0s"},
		{"minutes", 3 * time.Minute, "3.0m"},
		{"hours", 2 * time.Hour, "2.0h"},
		{"mixed", 90 * time.Second, "1.5m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.d)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

// getNextMilestone tests

func TestGetNextMilestone(t *testing.T) {
	milestones := initializeMilestones(time.Now(), 30)

	// No milestones completed, first should be baseline
	next := getNextMilestone(milestones, 0)
	if next == nil {
		t.Fatal("expected milestone")
	}
	if next.Name != "Baseline Setup" {
		t.Errorf("expected Baseline Setup, got %s", next.Name)
	}

	// Complete baseline
	milestones["baseline"].Completed = true
	next = getNextMilestone(milestones, 0)
	if next == nil {
		t.Fatal("expected milestone")
	}
	if next.Name != "Mesh Stability" {
		t.Errorf("expected Mesh Stability, got %s", next.Name)
	}

	// Complete all
	milestones["mesh_stability"].Completed = true
	milestones["production_traffic"].Completed = true
	milestones["advanced_scenarios"].Completed = true
	next = getNextMilestone(milestones, 0)
	if next != nil {
		t.Errorf("expected nil, got %v", next)
	}
}

func TestGetNextMilestone_NilMilestones(t *testing.T) {
	milestones := map[string]*Milestone{
		"baseline": nil,
	}
	next := getNextMilestone(milestones, 0)
	if next != nil {
		t.Errorf("expected nil for nil milestone, got %v", next)
	}
}

// Helper functions for tests

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q\nFull output:\n%s", needle, truncate(haystack, 500))
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func assertJSONField(t *testing.T, m map[string]interface{}, field string) {
	t.Helper()
	if _, ok := m[field]; !ok {
		t.Errorf("expected JSON field %q, available fields: %v", field, stringKeys(m))
	}
}

func stringKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
