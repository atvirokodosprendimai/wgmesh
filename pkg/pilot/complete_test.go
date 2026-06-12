package pilot

import (
	"strings"
	"testing"
)

func setupCompletablePilot(t *testing.T) *Pilot {
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

func TestComplete_NotStarted(t *testing.T) {
	p := New("")
	p.Initialize("Test Corp", "admin@test.com", 5, "decentralized", 30)

	_, err := p.Complete()
	if err == nil {
		t.Error("expected error when completing unstarted pilot")
	}
	if !strings.Contains(err.Error(), "not started") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestComplete_Success(t *testing.T) {
	p := setupCompletablePilot(t)

	p.metrics.RecordMeshConnectivity(99.95)
	p.metrics.RecordPeerDiscovery("dht")

	report, err := p.Complete()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report == nil {
		t.Fatal("report is nil")
	}
	if report.PilotID == "" {
		t.Error("pilot ID should be set")
	}
	if report.Organization != "Test Corp" {
		t.Errorf("organization: got %s, want Test Corp", report.Organization)
	}
	if report.ContactEmail != "admin@test.com" {
		t.Errorf("contact email: got %s, want admin@test.com", report.ContactEmail)
	}
	if report.Mode != "decentralized" {
		t.Errorf("mode: got %s, want decentralized", report.Mode)
	}
	if report.NodeCount != 5 {
		t.Errorf("node count: got %d, want 5", report.NodeCount)
	}
	if report.DurationDays != 30 {
		t.Errorf("duration days: got %d, want 30", report.DurationDays)
	}
	if report.Summary == nil {
		t.Error("summary should not be nil")
	}
	if report.CompletedAt.IsZero() {
		t.Error("completed at should not be zero")
	}
}

func TestComplete_DoubleComplete(t *testing.T) {
	p := setupCompletablePilot(t)

	_, err := p.Complete()
	if err != nil {
		t.Fatalf("first complete failed: %v", err)
	}

	_, err = p.Complete()
	if err == nil {
		t.Error("expected error on double complete")
	}
	if !strings.Contains(err.Error(), "already completed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestComplete_SummaryAllMilestonesCompleted(t *testing.T) {
	p := setupCompletablePilot(t)

	p.metrics.RecordMeshConnectivity(99.95)
	p.MarkMilestoneComplete("baseline")
	p.MarkMilestoneComplete("mesh_stability")
	p.MarkMilestoneComplete("production_traffic")
	p.MarkMilestoneComplete("advanced_scenarios")

	report, err := p.Complete()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.Summary.AllMilestonesCompleted {
		t.Error("all milestones should be completed")
	}
}

func TestComplete_SummaryPartialMilestones(t *testing.T) {
	p := setupCompletablePilot(t)

	p.metrics.RecordMeshConnectivity(99.95)
	p.MarkMilestoneComplete("baseline")
	p.MarkMilestoneComplete("mesh_stability")

	report, err := p.Complete()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Summary.AllMilestonesCompleted {
		t.Error("not all milestones should be completed")
	}
}

func TestComplete_RatingExcellent(t *testing.T) {
	p := setupCompletablePilot(t)

	p.metrics.RecordMeshConnectivity(99.95)
	p.metrics.PeerDiscoverySuccess = 1.0
	p.MarkMilestoneComplete("baseline")
	p.MarkMilestoneComplete("mesh_stability")
	p.MarkMilestoneComplete("production_traffic")
	p.MarkMilestoneComplete("advanced_scenarios")

	report, err := p.Complete()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Summary.OverallRating != "excellent" {
		t.Errorf("expected excellent rating, got %s", report.Summary.OverallRating)
	}
	if !strings.Contains(report.Summary.Recommendation, "production") {
		t.Errorf("recommendation should mention production: %s", report.Summary.Recommendation)
	}
}

func TestComplete_RatingGood(t *testing.T) {
	p := setupCompletablePilot(t)

	p.metrics.RecordMeshConnectivity(99.5)
	p.MarkMilestoneComplete("baseline")
	p.MarkMilestoneComplete("mesh_stability")
	p.MarkMilestoneComplete("production_traffic")

	report, err := p.Complete()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Summary.OverallRating != "good" {
		t.Errorf("expected good rating, got %s", report.Summary.OverallRating)
	}
	if !strings.Contains(report.Summary.Recommendation, "monitoring") {
		t.Errorf("recommendation should mention monitoring: %s", report.Summary.Recommendation)
	}
}

func TestComplete_RatingFair(t *testing.T) {
	p := setupCompletablePilot(t)

	p.metrics.RecordMeshConnectivity(95.0)
	p.MarkMilestoneComplete("baseline")

	report, err := p.Complete()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Summary.OverallRating != "fair" {
		t.Errorf("expected fair rating, got %s", report.Summary.OverallRating)
	}
	if !strings.Contains(report.Summary.Recommendation, "investigation") {
		t.Errorf("recommendation should mention investigation: %s", report.Summary.Recommendation)
	}
}

func TestComplete_RatingPoor(t *testing.T) {
	p := setupCompletablePilot(t)

	p.metrics.RecordMeshConnectivity(50.0)
	p.metrics.RecordDaemonRestart()
	p.metrics.RecordDaemonRestart()
	p.metrics.RecordDaemonRestart()

	report, err := p.Complete()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Summary.OverallRating != "poor" {
		t.Errorf("expected poor rating, got %s", report.Summary.OverallRating)
	}
	if !strings.Contains(report.Summary.Recommendation, "Not recommended") {
		t.Errorf("recommendation should say not recommended: %s", report.Summary.Recommendation)
	}
}

func TestComplete_MetricsCopiedToReport(t *testing.T) {
	p := setupCompletablePilot(t)

	p.metrics.RecordMeshConnectivity(99.97)
	p.metrics.RecordDaemonRestart()

	report, err := p.Complete()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Summary.MeshConnectivityAvg != 99.97 {
		t.Errorf("mesh connectivity avg: got %.2f, want 99.97", report.Summary.MeshConnectivityAvg)
	}
	if report.Summary.TotalDaemonRestarts != 1 {
		t.Errorf("total daemon restarts: got %d, want 1", report.Summary.TotalDaemonRestarts)
	}
}

func TestComplete_MilestonesInReport(t *testing.T) {
	p := setupCompletablePilot(t)

	p.metrics.RecordMeshConnectivity(99.95)
	p.MarkMilestoneComplete("baseline")

	report, err := p.Complete()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Milestones) != 4 {
		t.Errorf("expected 4 milestones, got %d", len(report.Milestones))
	}

	baseline, exists := report.Milestones["baseline"]
	if !exists {
		t.Fatal("baseline milestone missing")
	}
	if !baseline.Completed {
		t.Error("baseline should be completed")
	}
}

func TestComplete_FormatConsole(t *testing.T) {
	p := setupCompletablePilot(t)

	p.metrics.RecordMeshConnectivity(99.95)
	p.MarkMilestoneComplete("baseline")

	report, err := p.Complete()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := report.FormatConsole()

	if !strings.Contains(output, "wgmesh Pilot Final Report") {
		t.Error("expected header in console output")
	}
	if !strings.Contains(output, "Test Corp") {
		t.Error("expected organization in output")
	}
	if !strings.Contains(output, "admin@test.com") {
		t.Error("expected contact in output")
	}
	if !strings.Contains(output, "PILOT DURATION") {
		t.Error("expected pilot duration section")
	}
	if !strings.Contains(output, "CONFIGURATION") {
		t.Error("expected configuration section")
	}
	if !strings.Contains(output, "MILESTONE COMPLETION") {
		t.Error("expected milestone completion section")
	}
	if !strings.Contains(output, "SUMMARY METRICS") {
		t.Error("expected summary metrics section")
	}
	if !strings.Contains(output, "EVALUATION") {
		t.Error("expected evaluation section")
	}
	if !strings.Contains(output, "Overall Rating") {
		t.Error("expected overall rating")
	}
	if !strings.Contains(output, "Recommendation") {
		t.Error("expected recommendation")
	}
}

func TestComplete_FormatJSON(t *testing.T) {
	p := setupCompletablePilot(t)

	p.metrics.RecordMeshConnectivity(99.95)
	p.MarkMilestoneComplete("baseline")

	report, err := p.Complete()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := report.FormatJSON()

	if !strings.Contains(output, `"pilot_id"`) {
		t.Error("expected pilot_id in JSON output")
	}
	if !strings.Contains(output, `"organization"`) {
		t.Error("expected organization in JSON output")
	}
	if !strings.Contains(output, `"summary"`) {
		t.Error("expected summary in JSON output")
	}
	if !strings.Contains(output, `"overall_rating"`) {
		t.Error("expected overall_rating in JSON output")
	}
	if !strings.Contains(output, `"recommendation"`) {
		t.Error("expected recommendation in JSON output")
	}
	if !strings.Contains(output, `"completed_at"`) {
		t.Error("expected completed_at in JSON output")
	}
}
