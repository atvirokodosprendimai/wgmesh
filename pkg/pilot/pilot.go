package pilot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// PilotConfigPath is the default path for pilot configuration
	PilotConfigPath = "/etc/wgmesh/pilot.yaml"
	// DefaultPilotDuration is the default duration in days
	DefaultPilotDuration = 30
)

// Milestone represents a pilot evaluation milestone
type Milestone struct {
	Name        string    `yaml:"name"`
	Completed   bool      `yaml:"completed"`
	CompletedAt time.Time `yaml:"completed_at,omitempty"`
	TargetDate  time.Time `yaml:"target_date,omitempty"`
}

// Phase represents a pilot phase with its milestones
type Phase struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	StartDay    int          `yaml:"start_day"`
	EndDay      int          `yaml:"end_day"`
	Milestones  []*Milestone `yaml:"milestones"`
}

// MetricsTarget defines evaluation targets
type MetricsTarget struct {
	MeshConnectivity  float64 `yaml:"mesh_connectivity"`   // percent
	PeerDiscoveryTime int     `yaml:"peer_discovery_time"` // seconds
	RoutePropagation  int     `yaml:"route_propagation"`   // seconds
	ThroughputMbps    float64 `yaml:"throughput_mbps"`
	LatencyOverheadMs int     `yaml:"latency_overhead_ms"`
}

// Config represents the pilot configuration
type Config struct {
	PilotID        string                `yaml:"pilot_id"`
	Organization   string                `yaml:"organization"`
	ContactEmail   string                `yaml:"contact_email"`
	StartDate      time.Time             `yaml:"start_date"`
	EndDate        time.Time             `yaml:"end_date"`
	Mode           string                `yaml:"mode"` // centralized or decentralized
	NodeCount      int                   `yaml:"node_count"`
	Milestones     map[string]*Milestone `yaml:"milestones"`
	MetricsTargets MetricsTarget         `yaml:"metrics_targets"`
}

// State represents the current pilot state
type State struct {
	Config       *Config
	CurrentPhase string
	DaysElapsed  int
	Started      bool
	Completed    bool
	mu           sync.RWMutex
}

// Pilot manages the pilot evaluation lifecycle
type Pilot struct {
	configPath string
	state      *State
	metrics    *Metrics
	mu         sync.RWMutex
}

// New creates a new Pilot instance
func New(configPath string) *Pilot {
	if configPath == "" {
		configPath = PilotConfigPath
	}
	return &Pilot{
		configPath: configPath,
		state: &State{
			Config: &Config{
				Milestones: make(map[string]*Milestone),
			},
		},
		metrics: NewMetrics(),
	}
}

// Initialize creates a new pilot configuration
func (p *Pilot) Initialize(org, contact string, nodeCount int, mode string, durationDays int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Validate inputs
	if org == "" {
		return fmt.Errorf("organization name is required")
	}
	if contact == "" {
		return fmt.Errorf("contact email is required")
	}
	if nodeCount < 1 {
		return fmt.Errorf("node count must be at least 1")
	}
	if mode != "centralized" && mode != "decentralized" {
		return fmt.Errorf("mode must be 'centralized' or 'decentralized'")
	}
	if durationDays < 7 {
		return fmt.Errorf("duration must be at least 7 days")
	}

	now := time.Now()
	pilotID := generatePilotID(org, now)

	p.state.Config = &Config{
		PilotID:      pilotID,
		Organization: org,
		ContactEmail: contact,
		StartDate:    now,
		EndDate:      now.AddDate(0, 0, durationDays),
		Mode:         mode,
		NodeCount:    nodeCount,
		Milestones:   initializeMilestones(now, durationDays),
		MetricsTargets: MetricsTarget{
			MeshConnectivity:  99.9,
			PeerDiscoveryTime: 60,
			RoutePropagation:  30,
			ThroughputMbps:    80.0,
			LatencyOverheadMs: 20,
		},
	}

	p.state.Started = false
	p.state.Completed = false

	return nil
}

// Start begins the pilot evaluation
func (p *Pilot) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state.Started {
		return fmt.Errorf("pilot already started")
	}

	if p.state.Config.PilotID == "" {
		return fmt.Errorf("pilot not initialized")
	}

	p.state.Started = true

	// Start metrics collection
	p.metrics.Start(p.state.Config.PilotID)

	return nil
}

// Status returns the current pilot state
func (p *Pilot) Status() (*State, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.state.Started {
		return nil, fmt.Errorf("pilot not started")
	}

	// Update days elapsed
	p.state.DaysElapsed = int(time.Since(p.state.Config.StartDate).Hours() / 24)

	// Update current phase
	p.state.CurrentPhase = getCurrentPhase(p.state.DaysElapsed)

	return p.state, nil
}

// Config returns the pilot configuration
func (p *Pilot) Config() *Config {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state.Config
}

// IsStarted returns whether the pilot has been started
func (p *Pilot) IsStarted() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state.Started
}

// MarkMilestoneComplete marks a milestone as completed
func (p *Pilot) MarkMilestoneComplete(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	milestone, exists := p.state.Config.Milestones[name]
	if !exists {
		return fmt.Errorf("milestone not found: %s", name)
	}

	milestone.Completed = true
	milestone.CompletedAt = time.Now()

	return nil
}

// IsMilestoneComplete checks if a milestone is completed
func (p *Pilot) IsMilestoneComplete(name string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	milestone, exists := p.state.Config.Milestones[name]
	if !exists {
		return false
	}
	return milestone.Completed
}

// Save persists the pilot state to disk
func (p *Pilot) Save() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(p.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return saveConfig(p.state.Config, p.configPath)
}

// Load loads the pilot state from disk
func (p *Pilot) Load() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	config, err := loadConfig(p.configPath)
	if err != nil {
		return err
	}

	p.state.Config = config
	p.state.Started = !config.StartDate.IsZero()
	p.state.Completed = false

	return nil
}

// generatePilotID creates a unique pilot ID from organization and date
func generatePilotID(org string, t time.Time) string {
	// Normalize organization name: lowercase, replace spaces with hyphens
	orgNormalized := strings.ToLower(org)
	orgNormalized = strings.ReplaceAll(orgNormalized, " ", "-")
	orgNormalized = strings.ReplaceAll(orgNormalized, "/", "-")

	// Remove any non-alphanumeric characters except hyphens
	cleaned := ""
	for _, r := range orgNormalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			cleaned += string(r)
		}
	}

	return fmt.Sprintf("%s-%s", cleaned, t.Format("20060102"))
}

// initializeMilestones creates the standard four-phase milestones
func initializeMilestones(startDate time.Time, durationDays int) map[string]*Milestone {
	milestones := make(map[string]*Milestone)

	// Phase 1: Baseline Setup (Days 1-3)
	milestones["baseline"] = &Milestone{
		Name:       "Baseline Setup",
		TargetDate: startDate.AddDate(0, 0, 3),
		Completed:  false,
	}

	// Phase 2: Mesh Stability (Days 4-7)
	milestones["mesh_stability"] = &Milestone{
		Name:       "Mesh Stability",
		TargetDate: startDate.AddDate(0, 0, 7),
		Completed:  false,
	}

	// Phase 3: Production Traffic (Days 8-14)
	milestones["production_traffic"] = &Milestone{
		Name:       "Production Traffic",
		TargetDate: startDate.AddDate(0, 0, 14),
		Completed:  false,
	}

	// Phase 4: Advanced Scenarios (Days 15-30)
	milestones["advanced_scenarios"] = &Milestone{
		Name:       "Advanced Scenarios",
		TargetDate: startDate.AddDate(0, 0, durationDays),
		Completed:  false,
	}

	return milestones
}

// getCurrentPhase determines the current phase based on days elapsed
func getCurrentPhase(daysElapsed int) string {
	switch {
	case daysElapsed <= 3:
		return "Baseline Setup"
	case daysElapsed <= 7:
		return "Mesh Stability"
	case daysElapsed <= 14:
		return "Production Traffic"
	default:
		return "Advanced Scenarios"
	}
}
