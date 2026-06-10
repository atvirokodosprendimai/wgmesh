package pilot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewPilotState(t *testing.T) {
	state := NewPilotState(ModeDecentralized, UseCaseRemoteTeam, 3)

	if state.Mode != ModeDecentralized {
		t.Errorf("expected mode %s, got %s", ModeDecentralized, state.Mode)
	}
	if state.UseCase != UseCaseRemoteTeam {
		t.Errorf("expected use case %s, got %s", UseCaseRemoteTeam, state.UseCase)
	}
	if state.InitialNodeCount != 3 {
		t.Errorf("expected 3 nodes, got %d", state.InitialNodeCount)
	}
	if state.StartDate.IsZero() {
		t.Error("expected non-zero start date")
	}
	if len(state.Milestones) != 0 {
		t.Errorf("expected empty milestones, got %d", len(state.Milestones))
	}
}

func TestPilotDayWeek(t *testing.T) {
	tests := []struct {
		day  PilotDay
		week int
	}{
		{1, 1},
		{7, 1},
		{8, 2},
		{14, 2},
		{15, 3},
		{21, 3},
		{22, 4},
		{28, 4},
		{29, 4},
		{30, 4},
	}
	for _, tt := range tests {
		if got := tt.day.Week(); got != tt.week {
			t.Errorf("PilotDay(%d).Week() = %d, want %d", tt.day, got, tt.week)
		}
	}
}

func TestCurrentDay(t *testing.T) {
	// Pilot started just now — day should be 1
	state := NewPilotState(ModeDecentralized, UseCaseGeneral, 1)
	if got := state.CurrentDay(); got != 1 {
		t.Errorf("expected day 1, got %d", got)
	}

	// Pilot started 10 days ago
	state.StartDate = time.Now().Add(-10 * 24 * time.Hour)
	if got := state.CurrentDay(); got != 11 {
		t.Errorf("expected day 11, got %d", got)
	}

	// Pilot started far in the future
	state.StartDate = time.Now().Add(-100 * 24 * time.Hour)
	if got := state.CurrentDay(); got != 30 {
		t.Errorf("expected day 30 (capped), got %d", got)
	}

	// Zero start date
	state.StartDate = time.Time{}
	if got := state.CurrentDay(); got != 0 {
		t.Errorf("expected day 0 for zero start date, got %d", got)
	}
}

func TestIsComplete(t *testing.T) {
	state := NewPilotState(ModeDecentralized, UseCaseGeneral, 1)
	if state.IsComplete() {
		t.Error("new pilot should not be complete")
	}

	state.StartDate = time.Now().Add(-30 * 24 * time.Hour)
	if !state.IsComplete() {
		t.Error("30-day-old pilot should be complete")
	}
}

func TestMilestoneCompletion(t *testing.T) {
	state := NewPilotState(ModeDecentralized, UseCaseGeneral, 1)

	if state.MilestoneCompleted("mesh-bootstrap") {
		t.Error("milestone should not be completed initially")
	}

	state.CompleteMilestone("mesh-bootstrap")
	if !state.MilestoneCompleted("mesh-bootstrap") {
		t.Error("milestone should be completed after CompleteMilestone")
	}

	completed := state.CompletedMilestones()
	if len(completed) != 1 {
		t.Fatalf("expected 1 completed milestone, got %d", len(completed))
	}
	if completed[0].Name != "mesh-bootstrap" {
		t.Errorf("expected milestone name 'mesh-bootstrap', got %s", completed[0].Name)
	}
}

func TestMilestonesSorted(t *testing.T) {
	state := NewPilotState(ModeDecentralized, UseCaseGeneral, 1)
	state.CompleteMilestone("second")
	time.Sleep(time.Millisecond)
	state.CompleteMilestone("first")

	completed := state.CompletedMilestones()
	if len(completed) != 2 {
		t.Fatalf("expected 2 milestones, got %d", len(completed))
	}
	if completed[0].Name != "second" {
		t.Errorf("expected 'second' first (earlier timestamp), got %s", completed[0].Name)
	}
}

func TestAddHealthResult(t *testing.T) {
	state := NewPilotState(ModeDecentralized, UseCaseGeneral, 1)

	if state.LastHealthResult() != nil {
		t.Error("expected nil last health result initially")
	}

	result := HealthCheckResult{
		Timestamp: time.Now(),
		PassCount: 5,
		FailCount: 1,
		WarnCount: 2,
		Checks: []CheckResult{
			{Name: "test", Status: HealthPass, Message: "ok"},
		},
	}
	state.AddHealthResult(result)

	last := state.LastHealthResult()
	if last == nil {
		t.Fatal("expected non-nil last health result")
	}
	if last.PassCount != 5 {
		t.Errorf("expected 5 pass, got %d", last.PassCount)
	}
}

func TestHealthHistoryCapped(t *testing.T) {
	state := NewPilotState(ModeDecentralized, UseCaseGeneral, 1)

	for i := 0; i < 15; i++ {
		state.AddHealthResult(HealthCheckResult{
			Timestamp: time.Now(),
			PassCount: i,
		})
	}

	if len(state.HealthHistory) != maxHealthHistory {
		t.Errorf("expected %d history entries, got %d", maxHealthHistory, len(state.HealthHistory))
	}

	// Should keep the latest entries
	if state.HealthHistory[0].PassCount != 5 {
		t.Errorf("expected first entry pass count 5, got %d", state.HealthHistory[0].PassCount)
	}
}

func TestHealthCheckResultStatus(t *testing.T) {
	tests := []struct {
		name       string
		result     HealthCheckResult
		wantAll    bool
		wantStatus HealthStatus
	}{
		{
			"all pass",
			HealthCheckResult{PassCount: 5},
			true,
			HealthPass,
		},
		{
			"has failures",
			HealthCheckResult{PassCount: 3, FailCount: 2},
			false,
			HealthFail,
		},
		{
			"warnings only",
			HealthCheckResult{PassCount: 3, WarnCount: 2},
			true,
			HealthWarn,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.AllPassed(); got != tt.wantAll {
				t.Errorf("AllPassed() = %v, want %v", got, tt.wantAll)
			}
			if got := tt.result.Status(); got != tt.wantStatus {
				t.Errorf("Status() = %v, want %v", got, tt.wantStatus)
			}
		})
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "pilot.json")

	state := NewPilotState(ModeCentralized, UseCaseMultiCloud, 5)
	state.PlatformBreakdown = map[string]int{"linux": 3, "darwin": 2}
	state.CompleteMilestone("mesh-bootstrap")
	state.AddHealthResult(HealthCheckResult{
		Timestamp: time.Now(),
		PassCount: 7,
		FailCount: 1,
		WarnCount: 0,
	})
	state.AddIssue("NAT traversal failed", "Adjusted keepalive interval")

	if err := state.Save(path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}

	if loaded.Mode != ModeCentralized {
		t.Errorf("expected mode %s, got %s", ModeCentralized, loaded.Mode)
	}
	if loaded.UseCase != UseCaseMultiCloud {
		t.Errorf("expected use case %s, got %s", UseCaseMultiCloud, loaded.UseCase)
	}
	if loaded.InitialNodeCount != 5 {
		t.Errorf("expected 5 nodes, got %d", loaded.InitialNodeCount)
	}
	if !loaded.MilestoneCompleted("mesh-bootstrap") {
		t.Error("expected mesh-bootstrap milestone to be completed")
	}
	if len(loaded.HealthHistory) != 1 {
		t.Errorf("expected 1 health result, got %d", len(loaded.HealthHistory))
	}
	if loaded.HealthHistory[0].PassCount != 7 {
		t.Errorf("expected 7 pass, got %d", loaded.HealthHistory[0].PassCount)
	}
	if len(loaded.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(loaded.Issues))
	}
	if loaded.Issues[0].Description != "NAT traversal failed" {
		t.Errorf("unexpected issue description: %s", loaded.Issues[0].Description)
	}
	if loaded.PlatformBreakdown["linux"] != 3 {
		t.Errorf("expected 3 linux, got %d", loaded.PlatformBreakdown["linux"])
	}
}

func TestLoadStateNotExist(t *testing.T) {
	_, err := LoadState("/nonexistent/path/pilot.json")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nested", "dir", "pilot.json")

	state := NewPilotState(ModeDecentralized, UseCaseGeneral, 1)
	if err := state.Save(path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file should exist: %v", err)
	}
}

func TestStateFileIsJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "pilot.json")

	state := NewPilotState(ModeDecentralized, UseCaseGeneral, 1)
	if err := state.Save(path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("state file should be valid JSON: %v", err)
	}

	if _, ok := raw["start_date"]; !ok {
		t.Error("expected start_date field in JSON")
	}
	if _, ok := raw["mode"]; !ok {
		t.Error("expected mode field in JSON")
	}
}

func TestWeekMilestones(t *testing.T) {
	tests := []struct {
		week    int
		wantLen int
	}{
		{1, 3},
		{2, 3},
		{3, 3},
		{4, 3},
		{0, 0},
		{5, 0},
	}
	for _, tt := range tests {
		got := WeekMilestones(tt.week)
		if len(got) != tt.wantLen {
			t.Errorf("WeekMilestones(%d) returned %d milestones, want %d", tt.week, len(got), tt.wantLen)
		}
	}
}

func TestWeekTheme(t *testing.T) {
	tests := []struct {
		week int
		want string
	}{
		{1, "Deployment & Connectivity"},
		{2, "Operational Testing"},
		{3, "Resilience & Recovery"},
		{4, "Production Readiness"},
		{0, ""},
		{5, ""},
	}
	for _, tt := range tests {
		if got := WeekTheme(tt.week); got != tt.want {
			t.Errorf("WeekTheme(%d) = %q, want %q", tt.week, got, tt.want)
		}
	}
}

func TestWeekProgress(t *testing.T) {
	state := NewPilotState(ModeDecentralized, UseCaseGeneral, 1)

	// No milestones completed
	completed, total := WeekProgress(state, 1)
	if completed != 0 {
		t.Errorf("expected 0 completed, got %d", completed)
	}
	if total != 3 {
		t.Errorf("expected 3 total, got %d", total)
	}

	// Complete one milestone
	state.CompleteMilestone("mesh-bootstrap")
	completed, total = WeekProgress(state, 1)
	if completed != 1 {
		t.Errorf("expected 1 completed, got %d", completed)
	}
	if total != 3 {
		t.Errorf("expected 3 total, got %d", total)
	}
}

func TestAddIssue(t *testing.T) {
	state := NewPilotState(ModeDecentralized, UseCaseGeneral, 1)

	if len(state.Issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(state.Issues))
	}

	state.AddIssue("NAT traversal failed", "Increased keepalive")
	if len(state.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(state.Issues))
	}
	if state.Issues[0].Description != "NAT traversal failed" {
		t.Errorf("unexpected description: %s", state.Issues[0].Description)
	}
	if state.Issues[0].Resolution != "Increased keepalive" {
		t.Errorf("unexpected resolution: %s", state.Issues[0].Resolution)
	}
	if state.Issues[0].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestDeploymentModes(t *testing.T) {
	if ModeCentralized != "centralized" {
		t.Errorf("expected 'centralized', got %s", ModeCentralized)
	}
	if ModeDecentralized != "decentralized" {
		t.Errorf("expected 'decentralized', got %s", ModeDecentralized)
	}
}

func TestUseCases(t *testing.T) {
	useCases := []UseCase{
		UseCaseHybridSiteToSite,
		UseCaseMultiCloud,
		UseCaseRemoteTeam,
		UseCaseManagedFleet,
		UseCaseGeneral,
	}
	for _, uc := range useCases {
		if uc == "" {
			t.Error("use case should not be empty")
		}
	}
}

func TestDefaultPilotPath(t *testing.T) {
	path := DefaultPilotPath()
	if path == "" {
		t.Error("expected non-empty default path")
	}
	if filepath.Base(path) != "pilot.json" {
		t.Errorf("expected path to end with pilot.json, got %s", path)
	}
}

func TestLoadStateCorrupt(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "pilot.json")
	if err := os.WriteFile(path, []byte("{invalid json"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadState(path)
	if err == nil {
		t.Error("expected error for corrupt JSON")
	}
}

func TestRoundTripAllFields(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "pilot.json")

	state := NewPilotState(ModeDecentralized, UseCaseRemoteTeam, 4)
	state.PlatformBreakdown = map[string]int{"linux": 2, "darwin": 1, "windows": 1}
	state.CompleteMilestone("mesh-bootstrap")
	state.CompleteMilestone("all-peers-connected")
	state.AddHealthResult(HealthCheckResult{
		Timestamp: time.Now(),
		PassCount: 6,
		FailCount: 1,
		WarnCount: 1,
		Checks: []CheckResult{
			{Name: "Interface exists", Status: HealthPass, Message: "ok"},
			{Name: "Peers connected", Status: HealthFail, Message: "1 peer unreachable"},
		},
	})
	state.AddIssue("Intermittent timeout", "Adjusted MTU to 1280")
	state.AddIssue("DNS resolution slow", "Added local DNS cache")

	if err := state.Save(path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}

	if loaded.UseCase != UseCaseRemoteTeam {
		t.Errorf("use case mismatch: %s", loaded.UseCase)
	}
	if len(loaded.Milestones) != 2 {
		t.Errorf("expected 2 milestones, got %d", len(loaded.Milestones))
	}
	if len(loaded.HealthHistory) != 1 {
		t.Errorf("expected 1 health result, got %d", len(loaded.HealthHistory))
	}
	if len(loaded.HealthHistory[0].Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(loaded.HealthHistory[0].Checks))
	}
	if len(loaded.Issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(loaded.Issues))
	}
	if loaded.PlatformBreakdown["windows"] != 1 {
		t.Errorf("expected 1 windows, got %d", loaded.PlatformBreakdown["windows"])
	}
}
