// Package pilot provides a 30-day evaluation framework for wgmesh deployments.
// It tracks pilot state, milestones, and health checks to help network
// administrators assess wgmesh in their environment.
package pilot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// DeploymentMode represents how the mesh was deployed.
type DeploymentMode string

const (
	ModeCentralized   DeploymentMode = "centralized"
	ModeDecentralized DeploymentMode = "decentralized"
)

// UseCase describes the pilot's target use case.
type UseCase string

const (
	UseCaseHybridSiteToSite UseCase = "hybrid-site-to-site"
	UseCaseMultiCloud       UseCase = "multi-cloud"
	UseCaseRemoteTeam       UseCase = "remote-team"
	UseCaseManagedFleet     UseCase = "managed-fleet"
	UseCaseGeneral          UseCase = "general"
)

// PilotDay is a day number in the 30-day pilot (1-30).
type PilotDay int

// Week returns the week number (1-4) for the pilot day.
func (d PilotDay) Week() int {
	w := int(d-1)/7 + 1
	if w > 4 {
		w = 4
	}
	return w
}

// PilotState holds all state for a pilot evaluation.
type PilotState struct {
	// StartDate is when the pilot was initialized.
	StartDate time.Time `json:"start_date"`
	// Mode is the deployment mode.
	Mode DeploymentMode `json:"mode"`
	// UseCase is the target use case category.
	UseCase UseCase `json:"use_case"`
	// InitialNodeCount is the number of nodes at pilot start.
	InitialNodeCount int `json:"initial_node_count"`
	// PlatformBreakdown maps platform name to node count.
	PlatformBreakdown map[string]int `json:"platform_breakdown,omitempty"`
	// Milestones tracks completion of named milestones.
	Milestones map[string]time.Time `json:"milestones"`
	// HealthHistory stores the last N health check results.
	HealthHistory []HealthCheckResult `json:"health_history"`
	// Issues tracks problems encountered during the pilot.
	Issues []Issue `json:"issues,omitempty"`
}

// CurrentDay returns the current pilot day (1-based). Returns 0 if not started.
func (s *PilotState) CurrentDay() PilotDay {
	if s.StartDate.IsZero() {
		return 0
	}
	days := int(time.Since(s.StartDate).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	if days > 30 {
		days = 30
	}
	return PilotDay(days)
}

// CurrentWeek returns the current pilot week (1-4).
func (s *PilotState) CurrentWeek() int {
	return s.CurrentDay().Week()
}

// IsComplete returns true if the pilot has reached day 30.
func (s *PilotState) IsComplete() bool {
	return s.CurrentDay() >= 30
}

// CompleteMilestone marks a milestone as completed at the current time.
func (s *PilotState) CompleteMilestone(name string) {
	if s.Milestones == nil {
		s.Milestones = make(map[string]time.Time)
	}
	s.Milestones[name] = time.Now()
}

// MilestoneCompleted returns whether the named milestone has been completed.
func (s *PilotState) MilestoneCompleted(name string) bool {
	t, ok := s.Milestones[name]
	return ok && !t.IsZero()
}

// AddHealthResult appends a health check result, keeping at most maxHealthHistory entries.
func (s *PilotState) AddHealthResult(result HealthCheckResult) {
	s.HealthHistory = append(s.HealthHistory, result)
	if len(s.HealthHistory) > maxHealthHistory {
		s.HealthHistory = s.HealthHistory[len(s.HealthHistory)-maxHealthHistory:]
	}
}

// LastHealthResult returns the most recent health check result, or nil.
func (s *PilotState) LastHealthResult() *HealthCheckResult {
	if len(s.HealthHistory) == 0 {
		return nil
	}
	return &s.HealthHistory[len(s.HealthHistory)-1]
}

// CompletedMilestones returns milestone names sorted by completion time.
func (s *PilotState) CompletedMilestones() []MilestoneEntry {
	var entries []MilestoneEntry
	for name, t := range s.Milestones {
		if !t.IsZero() {
			entries = append(entries, MilestoneEntry{Name: name, CompletedAt: t})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CompletedAt.Before(entries[j].CompletedAt)
	})
	return entries
}

// AddIssue records a problem encountered during the pilot.
func (s *PilotState) AddIssue(description, resolution string) {
	s.Issues = append(s.Issues, Issue{
		Description: description,
		Resolution:  resolution,
		Timestamp:   time.Now(),
	})
}

// MilestoneEntry is a completed milestone with its timestamp.
type MilestoneEntry struct {
	Name        string    `json:"name"`
	CompletedAt time.Time `json:"completed_at"`
}

// Issue represents a problem encountered during the pilot.
type Issue struct {
	Description string    `json:"description"`
	Resolution  string    `json:"resolution,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

const maxHealthHistory = 10

// NewPilotState creates a new pilot state with the given parameters.
func NewPilotState(mode DeploymentMode, useCase UseCase, nodeCount int) *PilotState {
	return &PilotState{
		StartDate:         time.Now(),
		Mode:              mode,
		UseCase:           useCase,
		InitialNodeCount:  nodeCount,
		PlatformBreakdown: make(map[string]int),
		Milestones:        make(map[string]time.Time),
		HealthHistory:     nil,
	}
}

// DefaultPilotPath returns the default path for pilot state storage.
func DefaultPilotPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "pilot.json"
	}
	return filepath.Join(home, ".wgmesh", "pilot.json")
}

// LoadState loads pilot state from a JSON file.
func LoadState(path string) (*PilotState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading pilot state: %w", err)
	}
	var state PilotState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing pilot state: %w", err)
	}
	return &state, nil
}

// Save persists pilot state to a JSON file.
func (s *PilotState) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating pilot state directory: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling pilot state: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing pilot state: %w", err)
	}
	return nil
}

// WeekMilestones returns the expected milestones for a given pilot week.
func WeekMilestones(week int) []string {
	switch week {
	case 1:
		return []string{
			"mesh-bootstrap",
			"all-peers-connected",
			"basic-throughput-test",
		}
	case 2:
		return []string{
			"nat-traversal-verified",
			"relay-fallback-tested",
			"node-addition-removal",
		}
	case 3:
		return []string{
			"daemon-restart-recovery",
			"network-interruption-recovery",
			"key-rotation-tested",
		}
	case 4:
		return []string{
			"systemd-integration",
			"metrics-collection",
			"policy-configuration",
		}
	default:
		return nil
	}
}

// WeekTheme returns a human-readable theme for the given pilot week.
func WeekTheme(week int) string {
	switch week {
	case 1:
		return "Deployment & Connectivity"
	case 2:
		return "Operational Testing"
	case 3:
		return "Resilience & Recovery"
	case 4:
		return "Production Readiness"
	default:
		return ""
	}
}

// WeekProgress returns completed and total milestone counts for a given week.
func WeekProgress(state *PilotState, week int) (completed, total int) {
	expected := WeekMilestones(week)
	total = len(expected)
	for _, name := range expected {
		if state.MilestoneCompleted(name) {
			completed++
		}
	}
	return
}
