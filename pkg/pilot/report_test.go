package pilot

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestGenerateReportMarkdown(t *testing.T) {
	state := NewPilotState(ModeDecentralized, UseCaseRemoteTeam, 3)
	state.PlatformBreakdown = map[string]int{"linux": 2, "darwin": 1}
	state.CompleteMilestone("mesh-bootstrap")
	state.AddHealthResult(HealthCheckResult{
		Timestamp: time.Now(),
		PassCount: 7,
		FailCount: 1,
		WarnCount: 0,
		Checks: []CheckResult{
			{Name: "Interface exists", Status: HealthPass, Message: "ok"},
			{Name: "Peers connected", Status: HealthFail, Message: "1 peer unreachable"},
		},
	})
	state.AddIssue("NAT timeout", "Adjusted keepalive")

	report, err := GenerateReport(state, ReportMarkdown)
	if err != nil {
		t.Fatalf("GenerateReport() error: %v", err)
	}

	// Verify key sections exist
	if !strings.Contains(report, "# wgmesh Pilot Evaluation Report") {
		t.Error("report missing title")
	}
	if !strings.Contains(report, "decentralized") {
		t.Error("report missing deployment mode")
	}
	if !strings.Contains(report, "remote-team") {
		t.Error("report missing use case")
	}
	if !strings.Contains(report, "Milestone Timeline") {
		t.Error("report missing milestone section")
	}
	if !strings.Contains(report, "[x] mesh-bootstrap") {
		t.Error("report missing completed milestone")
	}
	if !strings.Contains(report, "Health Check Summary") {
		t.Error("report missing health check section")
	}
	if !strings.Contains(report, "Issues Encountered") {
		t.Error("report missing issues section")
	}
	if !strings.Contains(report, "Platform Breakdown") {
		t.Error("report missing platform breakdown")
	}
}

func TestGenerateReportJSON(t *testing.T) {
	state := NewPilotState(ModeCentralized, UseCaseMultiCloud, 5)
	state.CompleteMilestone("mesh-bootstrap")
	state.CompleteMilestone("all-peers-connected")
	state.AddHealthResult(HealthCheckResult{
		Timestamp: time.Now(),
		PassCount: 6,
		FailCount: 2,
		WarnCount: 0,
		Checks: []CheckResult{
			{Name: "test", Status: HealthFail, Message: "failed"},
		},
	})

	report, err := GenerateReport(state, ReportJSON)
	if err != nil {
		t.Fatalf("GenerateReport() error: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(report), &parsed); err != nil {
		t.Fatalf("report is not valid JSON: %v", err)
	}

	// Check key fields
	if _, ok := parsed["pilot_day"]; !ok {
		t.Error("missing pilot_day field")
	}
	if _, ok := parsed["pilot_week"]; !ok {
		t.Error("missing pilot_week field")
	}
	if _, ok := parsed["deployment_mode"]; !ok {
		t.Error("missing deployment_mode field")
	}
	if _, ok := parsed["milestones"]; !ok {
		t.Error("missing milestones field")
	}
	if _, ok := parsed["week_progress"]; !ok {
		t.Error("missing week_progress field")
	}
	if _, ok := parsed["health_summary"]; !ok {
		t.Error("missing health_summary field")
	}
}

func TestGenerateReportUnsupportedFormat(t *testing.T) {
	state := NewPilotState(ModeDecentralized, UseCaseGeneral, 1)
	_, err := GenerateReport(state, ReportFormat("xml"))
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestReportEmptyState(t *testing.T) {
	state := NewPilotState(ModeDecentralized, UseCaseGeneral, 1)

	report, err := GenerateReport(state, ReportMarkdown)
	if err != nil {
		t.Fatalf("GenerateReport() error: %v", err)
	}

	if !strings.Contains(report, "No health checks have been run yet") {
		t.Error("expected message about no health checks for empty state")
	}
}

func TestReportCompletedPilot(t *testing.T) {
	state := NewPilotState(ModeDecentralized, UseCaseGeneral, 1)
	state.StartDate = time.Now().Add(-30 * 24 * time.Hour)

	report, err := GenerateReport(state, ReportMarkdown)
	if err != nil {
		t.Fatalf("GenerateReport() error: %v", err)
	}

	if !strings.Contains(report, "Pilot Complete") {
		t.Error("expected pilot complete section for 30-day pilot")
	}
}

func TestHealthCheckStatuses(t *testing.T) {
	tests := []struct {
		name   string
		status HealthStatus
		want   string
	}{
		{"pass", HealthPass, "pass"},
		{"fail", HealthFail, "fail"},
		{"warn", HealthWarn, "warn"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("got %q, want %q", tt.status, tt.want)
			}
		})
	}
}
