package pilot

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	p := New("/tmp/test-pilot.yaml")
	if p == nil {
		t.Fatal("New() returned nil")
	}
	if p.configPath != "/tmp/test-pilot.yaml" {
		t.Errorf("config path not set correctly, got %s", p.configPath)
	}
	if p.state == nil {
		t.Error("state not initialized")
	}
	if p.metrics == nil {
		t.Error("metrics not initialized")
	}
}

func TestNewDefaultPath(t *testing.T) {
	p := New("")
	if p.configPath != PilotConfigPath {
		t.Errorf("default config path not used, got %s", p.configPath)
	}
}

func TestInitialize(t *testing.T) {
	tests := []struct {
		name        string
		org         string
		contact     string
		nodeCount   int
		mode        string
		duration    int
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid initialization",
			org:       "Test Corp",
			contact:   "admin@test.com",
			nodeCount: 5,
			mode:      "decentralized",
			duration:  30,
			wantErr:   false,
		},
		{
			name:        "missing organization",
			org:         "",
			contact:     "admin@test.com",
			nodeCount:   5,
			mode:        "decentralized",
			duration:    30,
			wantErr:     true,
			errContains: "organization name is required",
		},
		{
			name:        "missing contact",
			org:         "Test Corp",
			contact:     "",
			nodeCount:   5,
			mode:        "decentralized",
			duration:    30,
			wantErr:     true,
			errContains: "contact email is required",
		},
		{
			name:        "invalid node count",
			org:         "Test Corp",
			contact:     "admin@test.com",
			nodeCount:   0,
			mode:        "decentralized",
			duration:    30,
			wantErr:     true,
			errContains: "node count must be at least 1",
		},
		{
			name:        "invalid mode",
			org:         "Test Corp",
			contact:     "admin@test.com",
			nodeCount:   5,
			mode:        "invalid",
			duration:    30,
			wantErr:     true,
			errContains: "mode must be 'centralized' or 'decentralized'",
		},
		{
			name:        "duration too short",
			org:         "Test Corp",
			contact:     "admin@test.com",
			nodeCount:   5,
			mode:        "decentralized",
			duration:    5,
			wantErr:     true,
			errContains: "duration must be at least 7 days",
		},
		{
			name:      "centralized mode",
			org:       "Test Corp",
			contact:   "admin@test.com",
			nodeCount: 3,
			mode:      "centralized",
			duration:  14,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New("")
			err := p.Initialize(tt.org, tt.contact, tt.nodeCount, tt.mode, tt.duration)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errContains != "" {
					if err.Error() != tt.errContains && err.Error()[:len(tt.errContains)] != tt.errContains {
						t.Errorf("error message mismatch: got %v, want to contain %s", err, tt.errContains)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			config := p.Config()
			if config.Organization != tt.org {
				t.Errorf("organization not set: got %s, want %s", config.Organization, tt.org)
			}
			if config.ContactEmail != tt.contact {
				t.Errorf("contact not set: got %s, want %s", config.ContactEmail, tt.contact)
			}
			if config.NodeCount != tt.nodeCount {
				t.Errorf("node count not set: got %d, want %d", config.NodeCount, tt.nodeCount)
			}
			if config.Mode != tt.mode {
				t.Errorf("mode not set: got %s, want %s", config.Mode, tt.mode)
			}
			if config.PilotID == "" {
				t.Error("pilot ID not generated")
			}
			if len(config.Milestones) != 4 {
				t.Errorf("expected 4 milestones, got %d", len(config.Milestones))
			}
			if p.IsStarted() {
				t.Error("pilot should not be started after initialization")
			}
		})
	}
}

func TestStart(t *testing.T) {
	p := New("")
	p.Initialize("Test Corp", "admin@test.com", 5, "decentralized", 30)

	err := p.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !p.IsStarted() {
		t.Error("pilot should be started after Start()")
	}

	// Test double start
	err = p.Start()
	if err == nil {
		t.Error("expected error on double start")
	}
}

func TestStartNotInitialized(t *testing.T) {
	p := New("")

	err := p.Start()
	if err == nil {
		t.Error("expected error when starting uninitialized pilot")
	}
}

func TestStatus(t *testing.T) {
	p := New("")
	p.Initialize("Test Corp", "admin@test.com", 5, "decentralized", 30)
	p.Start()

	state, err := p.Status()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state == nil {
		t.Fatal("state is nil")
	}
	if state.Config.PilotID == "" {
		t.Error("pilot ID not set in state")
	}
	if state.CurrentPhase == "" {
		t.Error("current phase not set")
	}
	if state.DaysElapsed < 0 {
		t.Error("days elapsed should be non-negative")
	}
}

func TestStatusNotStarted(t *testing.T) {
	p := New("")
	p.Initialize("Test Corp", "admin@test.com", 5, "decentralized", 30)

	_, err := p.Status()
	if err == nil {
		t.Error("expected error when getting status of unstarted pilot")
	}
}

func TestMarkMilestoneComplete(t *testing.T) {
	p := New("")
	p.Initialize("Test Corp", "admin@test.com", 5, "decentralized", 30)
	p.Start()

	err := p.MarkMilestoneComplete("baseline")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !p.IsMilestoneComplete("baseline") {
		t.Error("baseline milestone should be complete")
	}
}

func TestMarkMilestoneCompleteInvalid(t *testing.T) {
	p := New("")
	p.Initialize("Test Corp", "admin@test.com", 5, "decentralized", 30)
	p.Start()

	err := p.MarkMilestoneComplete("invalid_milestone")
	if err == nil {
		t.Error("expected error for invalid milestone")
	}
}

func TestMilestoneTargetDates(t *testing.T) {
	p := New("")
	p.Initialize("Test Corp", "admin@test.com", 5, "decentralized", 30)

	config := p.Config()

	// Check baseline milestone (Day 3)
	baseline := config.Milestones["baseline"]
	if baseline == nil {
		t.Fatal("baseline milestone not found")
	}
	expectedDay := 3
	targetDay := int(baseline.TargetDate.Sub(config.StartDate).Hours() / 24)
	if targetDay < expectedDay || targetDay > expectedDay+1 {
		t.Errorf("baseline target date incorrect: got day %d, want day %d", targetDay, expectedDay)
	}

	// Check mesh_stability milestone (Day 7)
	stability := config.Milestones["mesh_stability"]
	if stability == nil {
		t.Fatal("mesh_stability milestone not found")
	}
	expectedDay = 7
	targetDay = int(stability.TargetDate.Sub(config.StartDate).Hours() / 24)
	if targetDay < expectedDay || targetDay > expectedDay+1 {
		t.Errorf("mesh_stability target date incorrect: got day %d, want day %d", targetDay, expectedDay)
	}

	// Check production_traffic milestone (Day 14)
	production := config.Milestones["production_traffic"]
	if production == nil {
		t.Fatal("production_traffic milestone not found")
	}
	expectedDay = 14
	targetDay = int(production.TargetDate.Sub(config.StartDate).Hours() / 24)
	if targetDay < expectedDay || targetDay > expectedDay+1 {
		t.Errorf("production_traffic target date incorrect: got day %d, want day %d", targetDay, expectedDay)
	}

	// Check advanced_scenarios milestone (Day 30)
	advanced := config.Milestones["advanced_scenarios"]
	if advanced == nil {
		t.Fatal("advanced_scenarios milestone not found")
	}
	expectedDay = 30
	targetDay = int(advanced.TargetDate.Sub(config.StartDate).Hours() / 24)
	if targetDay < expectedDay || targetDay > expectedDay+1 {
		t.Errorf("advanced_scenarios target date incorrect: got day %d, want day %d", targetDay, expectedDay)
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pilot-test.yaml")

	p := New(configPath)
	p.Initialize("Test Corp", "admin@test.com", 5, "decentralized", 30)
	p.Start()
	p.MarkMilestoneComplete("baseline")

	// Save
	err := p.Save()
	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Check file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file not created")
	}

	// Load into new pilot instance
	p2 := New(configPath)
	err = p2.Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	// Verify loaded state
	config := p2.Config()
	if config.PilotID != p.Config().PilotID {
		t.Error("pilot ID mismatch after load")
	}
	if config.Organization != p.Config().Organization {
		t.Error("organization mismatch after load")
	}
	if !p2.IsStarted() {
		t.Error("pilot should be started after load")
	}
	if !p2.IsMilestoneComplete("baseline") {
		t.Error("baseline milestone should be complete after load")
	}
}

func TestGeneratePilotID(t *testing.T) {
	tests := []struct {
		name     string
		org      string
		testTime time.Time
		want     string
	}{
		{
			name:     "simple org",
			org:      "Test Corp",
			testTime: time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC),
			want:     "test-corp-20250610",
		},
		{
			name:     "org with spaces",
			org:      "Acme Corporation Inc",
			testTime: time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC),
			want:     "acme-corporation-inc-20250610",
		},
		{
			name:     "org with slashes",
			org:      "Test/Corp",
			testTime: time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC),
			want:     "test-corp-20250610",
		},
		{
			name:     "mixed case",
			org:      "TESTCORP",
			testTime: time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC),
			want:     "testcorp-20250610",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generatePilotID(tt.org, tt.testTime)
			if got != tt.want {
				t.Errorf("generatePilotID() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestGetCurrentPhase(t *testing.T) {
	tests := []struct {
		name        string
		daysElapsed int
		wantPhase   string
	}{
		{"day 1", 1, "Baseline Setup"},
		{"day 3", 3, "Baseline Setup"},
		{"day 4", 4, "Mesh Stability"},
		{"day 7", 7, "Mesh Stability"},
		{"day 8", 8, "Production Traffic"},
		{"day 14", 14, "Production Traffic"},
		{"day 15", 15, "Advanced Scenarios"},
		{"day 30", 30, "Advanced Scenarios"},
		{"day 60", 60, "Advanced Scenarios"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getCurrentPhase(tt.daysElapsed)
			if got != tt.wantPhase {
				t.Errorf("getCurrentPhase(%d) = %s, want %s", tt.daysElapsed, got, tt.wantPhase)
			}
		})
	}
}

func TestMetricsTargetsDefaults(t *testing.T) {
	p := New("")
	p.Initialize("Test Corp", "admin@test.com", 5, "decentralized", 30)

	targets := p.Config().MetricsTargets

	if targets.MeshConnectivity != 99.9 {
		t.Errorf("default mesh connectivity target: got %.1f, want 99.9", targets.MeshConnectivity)
	}
	if targets.PeerDiscoveryTime != 60 {
		t.Errorf("default peer discovery time: got %d, want 60", targets.PeerDiscoveryTime)
	}
	if targets.RoutePropagation != 30 {
		t.Errorf("default route propagation: got %d, want 30", targets.RoutePropagation)
	}
	if targets.ThroughputMbps != 80.0 {
		t.Errorf("default throughput target: got %.1f, want 80.0", targets.ThroughputMbps)
	}
	if targets.LatencyOverheadMs != 20 {
		t.Errorf("default latency overhead: got %d, want 20", targets.LatencyOverheadMs)
	}
}
