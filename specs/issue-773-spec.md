   spec_file: specs/issue-773-spec.md
   issue_number: 773
   issue_title: Create 'Time-to-First-Mesh' onboarding checklist: embed in dashboard, track completion rate

    __( O)>  ● new session · anthropic glm-4.6
   \____)    20260619_14 · /opt/wgmesh-checkout
     L L     goose is ready
I'll analyze the repository structure to understand the codebase and then create the implementation specification.
  ────────────────────────────────────────
  ▸ analyze
    path: /opt/wgmesh-checkout
    max_depth: 2

4 files, 2688L, 60F, 1C (depth=2)
(195 files skipped: no parser)
go 100%

main.go [1507L, 31F, 1C]
main_test.go [371L, 11F]
service.go [503L, 12F]
service_test.go [307L, 6F]

  ────────────────────────────────────────
  ▸ analyze
    path: /opt/wgmesh-checkout
    max_depth: 3

113 files, 32959L, 1039F, 115C (depth=3)
(291 files skipped: no parser)
go 90% | javascript 8% | python 0%

cmd/
  eidos-lint/
    main.go [31L, 1F]
  status-gen/
    main.go [168L, 8F]
    render.go [159L, 5F, 3C]
    render_test.go [157L, 4F]
eidos/
  eidosmeta/
    meta.go [276L, 8F, 4C]
    meta_test.go [306L, 4F]
pkg/
  crypto/
    capability_version.go [8L, 0F]
    derive.go [300L, 12F, 1C]
    derive_test.go [448L, 20F]
    encrypt.go [106L, 2F]
    encrypt_test.go [319L, 6F]
    envelope.go [328L, 10F, 3C]
    envelope_compat_test.go [287L, 12F, 1C]
    envelope_test.go [363L, 3F]
    membership.go [53L, 3F]
    membership_test.go [110L, 6F]
    password.go [40L, 2F]
    rotation.go [144L, 8F, 3C]
    rotation_test.go [221L, 6F]
  daemon/
    cache.go [158L, 5F, 2C]
    cache_test.go [106L, 3F]
    collision.go [177L, 7F, 1C]
    collision_test.go [123L, 5F]
    config.go [244L, 7F, 2C]
    config_test.go [179L, 10F]
    daemon.go [1811L, 74F, 10C]
    daemon_test.go [394L, 9F]
    epoch.go [46L, 5F, 1C]
    executor.go [92L, 10F, 4C]
    health_test.go [33L, 1F]
    helpers.go [390L, 16F, 1C]
    helpers_test.go [634L, 19F, 2C]
    metrics.go [144L, 8F]
    metrics_test.go [266L, 11F]
    peerstore_compat.go [24L, 1F]
    peerstore_test.go [479L, 23F]
    relay_test.go [403L, 18F]
    reload_test.go [274L, 16F]
    routes.go [147L, 6F]
    routes_test.go [436L, 10F]
    systemd.go [220L, 5F, 1C]
    systemd_test.go [428L, 16F]
    validate.go [9L, 1F]
  discovery/
    dht.go [1472L, 49F, 5C]
    dht_test.go [370L, 16F]
    exchange.go [1166L, 40F, 5C]
    exchange_test.go [703L, 17F]
    gossip.go [330L, 10F, 1C]
    gossip_test.go [220L, 8F]
    init.go [18L, 2F]
    lan.go [253L, 9F, 1C]
    lan_test.go [32L, 1F]
    registry.go [412L, 8F, 2C]
    stun.go [332L, 8F, 1C]
    stun_integration_test.go [32L, 1F]
    stun_test.go [494L, 14F]
  ifname/
    validate.go [82L, 1F]
    validate_test.go [165L, 3F]
  mesh/
    account.go [63L, 2F, 1C]
    account_test.go [72L, 4F]
    deploy.go [338L, 8F]
    mesh.go [266L, 9F]
    mesh_test.go [268L, 5F]
    policy.go [262L, 10F]
    policy_test.go [717L, 13F]
    services.go [73L, 2F, 2C]
    services_test.go [107L, 4F]
    types.go [60L, 0F, 5C]
  node/
    store.go [374L, 22F, 3C]
    types.go [126L, 2F, 7C]
  pilot/
    complete.go [244L, 5F, 2C]
    complete_test.go [329L, 14F]
    config.go [59L, 4F]
    metrics.go [255L, 17F, 1C]
    metrics_test.go [277L, 18F]
    pilot.go [318L, 13F, 6C]
    pilot_test.go [435L, 14F]
    report.go [446L, 6F, 1C]
    report_test.go [424L, 25F]
    validation.go [267L, 11F, 2C]
    validation_test.go [420L, 17F]
  privacy/
    dandelion.go [301L, 14F, 4C]
    dandelion_test.go [154L, 9F]
  ratelimit/
    limiter.go [153L, 8F, 3C]
    limiter_test.go [197L, 11F]
  routes/
    routes.go [103L, 3F, 1C]
    routes_test.go [177L, 12F]
  rpc/
    client.go [82L, 3F, 1C]
    integration_test.go [233L, 1F]
    protocol.go [77L, 0F, 8C]
    protocol_test.go [138L, 5F]
    server.go [428L, 15F, 4C]
    server_test.go [111L, 4F]
  ssh/
    client.go [149L, 6F, 1C]
    routes.go [121L, 3F]
    wireguard.go [111L, 6F]
  wireguard/
    apply.go [252L, 9F, 4C]
    config.go [236L, 6F, 4C]
    config_test.go [630L, 3F]
    convert.go [23L, 1F]
    keys.go [35L, 1F]
    persist.go [134L, 5F]
scripts/
  workflows/
    e2e-stalled-watcher.js [123L, 3F]
    e2e-stalled-watcher.test.js [250L, 4F]
    e2e-verify-close-handler.js [339L, 6F]
    e2e-verify-close-handler.test.js [690L, 3F]
    impl-merged-close-handler.js [434L, 9F]
    impl-merged-close-handler.test.js [951L, 3F]
testlab/
  cloud/
    gen-report.py [317L, 8F]
main.go [1507L, 31F, 1C]
main_test.go [371L, 11F]
service.go [503L, 12F]
service_test.go [307L, 6F]

  ────────────────────────────────────────
  ▸ shell
    command: ls -la /opt/wgmesh-checkout/

total 20920
drwxr-xr-x 24 wgmesh wgmesh     4096 Jun 19 09:22 .
drwxr-xr-x  5 root   root       4096 Jun 11 05:32 ..
-rw-r--r--  1 wgmesh wgmesh     6003 Jun 11 05:32 AGENTS.md
drwxr-xr-x  2 wgmesh wgmesh     4096 Jun 11 05:32 .axon
-rw-r--r--  1 wgmesh wgmesh     2430 Jun 11 05:32 BUGFIX-network-format.md
-rw-r--r--  1 wgmesh wgmesh     5891 Jun 11 05:32 CLAUDE.md
drwxr-xr-x  4 wgmesh wgmesh     4096 Jun 18 12:06 cmd
drwxr-xr-x  4 wgmesh wgmesh     4096 Jun 11 05:32 company
drwxr-xr-x  2 wgmesh wgmesh     4096 Jun 11 05:32 .compound-engineering
-rw-r--r--  1 wgmesh wgmesh     5538 Jun 11 05:32 CONTRIBUTING.md
drwxr-xr-x  4 wgmesh wgmesh     4096 Jun 17 17:32 deploy
-rw-r--r--  1 wgmesh wgmesh     9993 Jun 11 05:32 DOCKER-COMPOSE.md
-rw-r--r--  1 wgmesh wgmesh     2673 Jun 11 05:32 docker-compose.yml
-rw-r--r--  1 wgmesh wgmesh     1539 Jun 11 05:32 Dockerfile
-rw-r--r--  1 wgmesh wgmesh      253 Jun 11 05:32 .dockerignore
-rw-r--r--  1 wgmesh wgmesh     4320 Jun 11 05:32 DOCKER.md
drwxr-xr-x 11 wgmesh wgmesh     4096 Jun 19 09:39 docs
drwxr-xr-x  3 wgmesh wgmesh     4096 Jun 11 05:32 eidos
-rw-r--r--  1 wgmesh wgmesh      294 Jun 11 05:32 .eidos-config.yaml
-rw-r--r--  1 wgmesh wgmesh     8003 Jun 11 05:32 ENCRYPTION.md
-rw-r--r--  1 wgmesh wgmesh      265 Jun 11 05:32 .env.example
drwxr-xr-x  2 wgmesh wgmesh     4096 Jun 11 05:32 evolution
-rwxr-xr-x  1 wgmesh wgmesh      699 Jun 11 05:32 example.sh
-rw-r--r--  1 wgmesh wgmesh    10153 Jun 11 05:32 FEATURE_MATRIX.md
drwxr-xr-x  3 wgmesh wgmesh     4096 Jun 11 05:32 features
-rw-r--r--  1 wgmesh wgmesh     1497 Jun 11 05:32 flake.lock
-rw-r--r--  1 wgmesh wgmesh     2744 Jun 11 05:32 flake.nix
drwxr-xr-x  2 wgmesh wgmesh     4096 Jun 11 05:32 formal
drwxr-xr-x  8 wgmesh wgmesh     4096 Jun 19 09:41 .git
drwxr-xr-x  8 wgmesh wgmesh     4096 Jun 11 05:32 .github
-rw-r--r--  1 wgmesh wgmesh      955 Jun 17 17:32 .gitignore
-rw-r--r--  1 wgmesh wgmesh     2044 Jun 17 09:01 go.mod
-rw-r--r--  1 wgmesh wgmesh     1508 Jun 11 05:32 .goosehints
-rw-r--r--  1 wgmesh wgmesh     2365 Jun 11 05:32 .goreleaser.yml
-rw-r--r--  1 wgmesh wgmesh     1651 Jun 11 05:32 GOSSIP_TESTING.md
-rw-r--r--  1 wgmesh wgmesh    64246 Jun 17 09:01 go.sum
-rw-r--r--  1 wgmesh wgmesh    11614 Jun 19 07:41 index.html
-rw-r--r--  1 wgmesh wgmesh    45657 Jun 19 09:22 main.go
-rw-r--r--  1 wgmesh wgmesh    11093 Jun 11 05:32 main_test.go
-rw-r--r--  1 wgmesh wgmesh      753 Jun 11 05:32 Makefile
drwxr-xr-x  2 wgmesh wgmesh     4096 Jun 19 07:05 memory
drwxr-xr-x  2 wgmesh wgmesh     4096 Jun 11 05:32 packaging
drwxr-xr-x  2 wgmesh wgmesh     4096 Jun 19 09:28 pipeline-output
drwxr-xr-x 15 wgmesh wgmesh     4096 Jun 19 09:22 pkg
-rw-r--r--  1 wgmesh wgmesh     1825 Jun 19 07:41 pp.html
drwxr-xr-x  2 wgmesh wgmesh     4096 Jun 19 07:41 public
-rw-r--r--  1 wgmesh wgmesh    13030 Jun 17 17:32 README.md
-rw-r--r--  1 wgmesh wgmesh    22310 Jun 11 05:32 rnd.txt
-rw-r--r--  1 wgmesh wgmesh     8932 Jun 11 05:32 ROADMAP.md
drwxr-xr-x  3 wgmesh wgmesh     4096 Jun 19 07:05 scripts
-rw-r--r--  1 wgmesh wgmesh    15786 Jun 11 05:32 service.go
-rw-r--r--  1 wgmesh wgmesh     8282 Jun 11 05:32 service_test.go
-rw-r--r--  1 wgmesh wgmesh      259 Jun 11 05:32 sonar-project.properties
drwxr-xr-x  5 wgmesh wgmesh     4096 Jun 19 09:27 specs
-rw-r--r--  1 wgmesh wgmesh     4868 Jun 11 05:32 STATUS.md
-rw-r--r--  1 wgmesh wgmesh     3729 Jun 11 05:32 STRATEGY.md
drwxr-xr-x  4 wgmesh wgmesh     4096 Jun 11 05:32 testdata
-rwxr-xr-x  1 wgmesh wgmesh     3115 Jun 11 05:32 test-encryption.sh
drwxr-xr-x  4 wgmesh wgmesh     4096 Jun 11 05:32 testlab
-rwxr-xr-x  1 wgmesh wgmesh     2466 Jun 11 05:32 test-route-cleanup.sh
-rw-r--r--  1 wgmesh wgmesh     3676 Jun 11 05:32 test-route-diff-scenarios.md
-rw-r--r--  1 wgmesh wgmesh    16418 Jun 11 05:32 TODO.md
-rwxr-xr-x  1 wgmesh wgmesh 20921614 Jun 17 02:46 wgmesh
drwxr-xr-x  2 wgmesh wgmesh     4096 Jun 19 07:41 wgmesh.dev
  ────────────────────────────────────────
  ▸ analyze
    path: /opt/wgmesh-checkout/pkg/pilot
    max_depth: 3

11 files, 3474L, 144F, 12C (depth=3)
go 100%

complete.go [244L, 5F, 2C]
complete_test.go [329L, 14F]
config.go [59L, 4F]
metrics.go [255L, 17F, 1C]
metrics_test.go [277L, 18F]
pilot.go [318L, 13F, 6C]
pilot_test.go [435L, 14F]
report.go [446L, 6F, 1C]
report_test.go [424L, 25F]
validation.go [267L, 11F, 2C]
validation_test.go [420L, 17F]

  ────────────────────────────────────────
  ▸ shell
    command: cat /opt/wgmesh-checkout/pkg/pilot/pilot.go

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
  ────────────────────────────────────────
  ▸ shell
    command: cat /opt/wgmesh-checkout/pkg/pilot/metrics.go

package pilot

import (
	"sync"
	"time"
)

// Metrics tracks evaluation-specific measurements
type Metrics struct {
	PilotID     string
	Phase       string
	DaysElapsed int

	// Connectivity metrics
	MeshUptimePercent    float64
	PeerDiscoverySuccess float64
	RoutePropagationTime time.Duration

	// Performance metrics
	ThroughputMbps       float64
	LatencyMs            float64
	ConnectionOverheadMs float64

	// Reliability metrics
	DaemonRestarts    int
	WireGuardRestarts int
	NetworkPartitions int
	RecoveryTimeSec   int

	// Discovery layer usage
	DiscoveryLayerCounts map[string]int

	// NAT traversal
	NATTypes           map[string]int
	HolePunchSuccess   float64
	RelayFallbackCount int

	mu      sync.RWMutex
	started bool
}

// NewMetrics creates a new Metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		DiscoveryLayerCounts: make(map[string]int),
		NATTypes:             make(map[string]int),
	}
}

// Start initializes metrics collection for a pilot
func (m *Metrics) Start(pilotID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PilotID = pilotID
	m.started = true
}

// RecordPeerDiscovery records a successful peer discovery
func (m *Metrics) RecordPeerDiscovery(layer string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.DiscoveryLayerCounts[layer]++
}

// RecordMeshConnectivity updates the mesh uptime percentage
func (m *Metrics) RecordMeshConnectivity(percent float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.MeshUptimePercent = percent
}

// RecordRoutePropagation records a route propagation time
func (m *Metrics) RecordRoutePropagation(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.RoutePropagationTime = duration
}

// RecordThroughput records the throughput measurement
func (m *Metrics) RecordThroughput(mbps float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.ThroughputMbps = mbps
}

// RecordLatency records the latency measurement
func (m *Metrics) RecordLatency(latencyMs, overheadMs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.LatencyMs = latencyMs
	m.ConnectionOverheadMs = overheadMs
}

// RecordDaemonRestart records a daemon restart
func (m *Metrics) RecordDaemonRestart() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.DaemonRestarts++
}

// RecordWireGuardRestart records a WireGuard restart
func (m *Metrics) RecordWireGuardRestart() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.WireGuardRestarts++
}

// RecordNetworkPartition records a network partition event
func (m *Metrics) RecordNetworkPartition(recoveryTimeSec int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.NetworkPartitions++
	m.RecoveryTimeSec = recoveryTimeSec
}

// RecordNATType records the detected NAT type
func (m *Metrics) RecordNATType(natType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.NATTypes[natType]++
}

// RecordHolePunchSuccess records a successful hole punch
func (m *Metrics) RecordHolePunchSuccess(success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	// Update success rate as running average
	if success {
		m.HolePunchSuccess = (m.HolePunchSuccess + 1.0) / 2.0
	} else {
		m.HolePunchSuccess = m.HolePunchSuccess / 2.0
	}
}

// RecordRelayFallback records a relay fallback event
func (m *Metrics) RecordRelayFallback() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	m.RelayFallbackCount++
}

// Snapshot returns a copy of current metrics
func (m *Metrics) Snapshot() *Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := &Metrics{
		PilotID:              m.PilotID,
		Phase:                m.Phase,
		DaysElapsed:          m.DaysElapsed,
		MeshUptimePercent:    m.MeshUptimePercent,
		PeerDiscoverySuccess: m.PeerDiscoverySuccess,
		RoutePropagationTime: m.RoutePropagationTime,
		ThroughputMbps:       m.ThroughputMbps,
		LatencyMs:            m.LatencyMs,
		ConnectionOverheadMs: m.ConnectionOverheadMs,
		DaemonRestarts:       m.DaemonRestarts,
		WireGuardRestarts:    m.WireGuardRestarts,
		NetworkPartitions:    m.NetworkPartitions,
		RecoveryTimeSec:      m.RecoveryTimeSec,
		HolePunchSuccess:     m.HolePunchSuccess,
		RelayFallbackCount:   m.RelayFallbackCount,
		DiscoveryLayerCounts: make(map[string]int),
		NATTypes:             make(map[string]int),
	}

	// Deep copy maps
	for k, v := range m.DiscoveryLayerCounts {
		snapshot.DiscoveryLayerCounts[k] = v
	}
	for k, v := range m.NATTypes {
		snapshot.NATTypes[k] = v
	}

	return snapshot
}

// SetPhase updates the current phase
func (m *Metrics) SetPhase(phase string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Phase = phase
}

// SetDaysElapsed updates the days elapsed
func (m *Metrics) SetDaysElapsed(days int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.DaysElapsed = days
}

// IsStarted returns whether metrics collection has started
func (m *Metrics) IsStarted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}
  ────────────────────────────────────────
  ▸ shell
    command: cat /opt/wgmesh-checkout/pkg/pilot/complete.go

package pilot

import (
	"fmt"
	"strings"
	"time"
)

// FinalReport represents the final pilot summary
type FinalReport struct {
	PilotID      string
	Organization string
	ContactEmail string
	StartDate    time.Time
	EndDate      time.Time
	DurationDays int
	Mode         string
	NodeCount    int
	Milestones   map[string]*Milestone
	Summary      *ReportSummary
	CompletedAt  time.Time
}

// ReportSummary provides a summary of pilot results
type ReportSummary struct {
	AllMilestonesCompleted bool
	MeshConnectivityAvg    float64
	PeerDiscoverySuccess   float64
	TotalDaemonRestarts    int
	TotalWGRestarts        int
	Recommendation         string
	OverallRating          string // "excellent", "good", "fair", "poor"
}

// Complete finalizes the pilot and generates final summary
func (p *Pilot) Complete() (*FinalReport, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.state.Started {
		return nil, fmt.Errorf("pilot not started")
	}

	if p.state.Completed {
		return nil, fmt.Errorf("pilot already completed")
	}

	// Get final metrics
	metricsSnapshot := p.metrics.Snapshot()

	// Generate final report
	report := &FinalReport{
		PilotID:      p.state.Config.PilotID,
		Organization: p.state.Config.Organization,
		ContactEmail: p.state.Config.ContactEmail,
		StartDate:    p.state.Config.StartDate,
		EndDate:      p.state.Config.EndDate,
		DurationDays: int(p.state.Config.EndDate.Sub(p.state.Config.StartDate).Hours() / 24),
		Mode:         p.state.Config.Mode,
		NodeCount:    p.state.Config.NodeCount,
		Milestones:   p.state.Config.Milestones,
		Summary:      p.generateReportSummary(metricsSnapshot),
		CompletedAt:  time.Now(),
	}

	// Mark pilot as completed
	p.state.Completed = true

	return report, nil
}

// generateReportSummary creates a summary of pilot results
func (p *Pilot) generateReportSummary(metrics *Metrics) *ReportSummary {
	summary := &ReportSummary{
		MeshConnectivityAvg:  metrics.MeshUptimePercent,
		PeerDiscoverySuccess: metrics.PeerDiscoverySuccess,
		TotalDaemonRestarts:  metrics.DaemonRestarts,
		TotalWGRestarts:      metrics.WireGuardRestarts,
	}

	// Check if all milestones completed
	allCompleted := true
	for _, milestone := range p.state.Config.Milestones {
		if !milestone.Completed {
			allCompleted = false
			break
		}
	}
	summary.AllMilestonesCompleted = allCompleted

	// Determine overall rating and recommendation
	summary.OverallRating, summary.Recommendation = p.evaluatePilot(summary, metrics)

	return summary
}

// evaluatePilot determines the overall rating and recommendation
func (p *Pilot) evaluatePilot(summary *ReportSummary, metrics *Metrics) (string, string) {
	score := 0

	// Mesh connectivity score (30 points)
	if summary.MeshConnectivityAvg >= 99.9 {
		score += 30
	} else if summary.MeshConnectivityAvg >= 99.0 {
		score += 25
	} else if summary.MeshConnectivityAvg >= 95.0 {
		score += 20
	}

	// Peer discovery score (20 points)
	if summary.PeerDiscoverySuccess >= 0.95 {
		score += 20
	} else if summary.PeerDiscoverySuccess >= 0.80 {
		score += 15
	} else if summary.PeerDiscoverySuccess >= 0.60 {
		score += 10
	}

	// Stability score (30 points)
	if summary.TotalDaemonRestarts == 0 && summary.TotalWGRestarts == 0 {
		score += 30
	} else if summary.TotalDaemonRestarts <= 2 && summary.TotalWGRestarts <= 2 {
		score += 20
	} else if summary.TotalDaemonRestarts <= 5 && summary.TotalWGRestarts <= 5 {
		score += 10
	}

	// Milestone completion score (20 points)
	if summary.AllMilestonesCompleted {
		score += 20
	} else {
		completedCount := 0
		for _, m := range p.state.Config.Milestones {
			if m.Completed {
				completedCount++
			}
		}
		completionRate := float64(completedCount) / float64(len(p.state.Config.Milestones))
		score += int(completionRate * 20)
	}

	// Determine rating
	var rating, recommendation string
	switch {
	case score >= 90:
		rating = "excellent"
		recommendation = "Ready for production deployment"
	case score >= 70:
		rating = "good"
		recommendation = "Suitable for production with monitoring"
	case score >= 50:
		rating = "fair"
		recommendation = "Requires investigation before production"
	default:
		rating = "poor"
		recommendation = "Not recommended for production"
	}

	return rating, recommendation
}

// FormatConsole formats the final report for console output
func (r *FinalReport) FormatConsole() string {
	output := fmt.Sprintf("wgmesh Pilot Final Report: %s\n", r.PilotID)
	output += fmt.Sprintf("%s\n\n", strings.Repeat("=", 70))

	output += fmt.Sprintf("Organization: %s\n", r.Organization)
	output += fmt.Sprintf("Contact: %s\n\n", r.ContactEmail)

	output += fmt.Sprintf("PILOT DURATION\n")
	output += fmt.Sprintf("Start: %s\n", r.StartDate.Format("2006-01-02"))
	output += fmt.Sprintf("End: %s\n", r.EndDate.Format("2006-01-02"))
	output += fmt.Sprintf("Duration: %d days\n\n", r.DurationDays)

	output += fmt.Sprintf("CONFIGURATION\n")
	output += fmt.Sprintf("Mode: %s\n", r.Mode)
	output += fmt.Sprintf("Node Count: %d\n\n", r.NodeCount)

	output += fmt.Sprintf("MILESTONE COMPLETION\n")
	for _, milestone := range r.Milestones {
		status := "✗ Not completed"
		if milestone.Completed {
			status = fmt.Sprintf("✓ Completed (Day %d)",
				int(milestone.CompletedAt.Sub(r.StartDate).Hours()/24))
		}
		output += fmt.Sprintf("  %s: %s\n", milestone.Name, status)
	}
	output += "\n"

	output += fmt.Sprintf("SUMMARY METRICS\n")
	output += fmt.Sprintf("  Mesh Connectivity Avg: %.2f%%\n", r.Summary.MeshConnectivityAvg)
	output += fmt.Sprintf("  Peer Discovery Success: %.1f%%\n", r.Summary.PeerDiscoverySuccess*100)
	output += fmt.Sprintf("  Total Daemon Restarts: %d\n", r.Summary.TotalDaemonRestarts)
	output += fmt.Sprintf("  Total WireGuard Restarts: %d\n\n", r.Summary.TotalWGRestarts)

	output += fmt.Sprintf("EVALUATION\n")
	output += fmt.Sprintf("  Overall Rating: %s\n", strings.ToUpper(r.Summary.OverallRating))
	output += fmt.Sprintf("  Recommendation: %s\n\n", r.Summary.Recommendation)

	output += fmt.Sprintf("Completed at: %s\n", r.CompletedAt.Format(time.RFC3339))

	return output
}

// FormatJSON formats the final report as JSON
func (r *FinalReport) FormatJSON() string {
	return fmt.Sprintf(`{
  "pilot_id": "%s",
  "organization": "%s",
  "contact_email": "%s",
  "start_date": "%s",
  "end_date": "%s",
  "duration_days": %d,
  "mode": "%s",
  "node_count": %d,
  "summary": {
    "all_milestones_completed": %t,
    "mesh_connectivity_avg": %.2f,
    "peer_discovery_success": %.2f,
    "total_daemon_restarts": %d,
    "total_wg_restarts": %d,
    "overall_rating": "%s",
    "recommendation": "%s"
  },
  "completed_at": "%s"
}`,
		r.PilotID,
		r.Organization,
		r.ContactEmail,
		r.StartDate.Format(time.RFC3339),
		r.EndDate.Format(time.RFC3339),
		r.DurationDays,
		r.Mode,
		r.NodeCount,
		r.Summary.AllMilestonesCompleted,
		r.Summary.MeshConnectivityAvg,
		r.Summary.PeerDiscoverySuccess,
		r.Summary.TotalDaemonRestarts,
		r.Summary.TotalWGRestarts,
		r.Summary.OverallRating,
		r.Summary.Recommendation,
		r.CompletedAt.Format(time.RFC3339),
	)
}
  ────────────────────────────────────────
  ▸ shell
    command: ls -la /opt/wgmesh-checkout/wgmesh.dev/ 2>/dev/null | head -20

total 20
drwxr-xr-x  2 wgmesh wgmesh  4096 Jun 19 07:41 .
drwxr-xr-x 24 wgmesh wgmesh  4096 Jun 19 09:22 ..
-rw-r--r--  1 wgmesh wgmesh 11715 Jun 19 07:41 index.html
  ────────────────────────────────────────
  ▸ shell
    command: head -100 /opt/wgmesh-checkout/wgmesh.dev/index.html

<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>wgmesh.dev - Revolutionizing Your Network</title>
    <style>
        body { font-family: sans-serif; margin: 0; padding: 2em; line-height: 1.6; color: #333; background-color: #f8f9fa; }
        header { text-align: center; margin-bottom: 3em; background-color: #ffffff; padding: 2em; border-radius: 10px; box-shadow: 0 4px 8px rgba(0,0,0,0.05); }
        h1 { color: #007bff; font-size: 2.5em; margin-bottom: 0.5em; }
        nav { margin-top: 1.5em; }
        nav a { margin: 0 1em; text-decoration: none; color: #007bff; font-weight: bold; font-size: 1.1em; transition: color 0.3s ease; }
        nav a:hover { color: #0056b3; text-decoration: underline; }
        main { max-width: 960px; margin: 0 auto; }
        section { margin-bottom: 3em; padding: 2em; border-radius: 10px; background-color: #ffffff; box-shadow: 0 2px 5px rgba(0,0,0,0.05); }
        section h2 { color: #007bff; border-bottom: 2px solid #e9ecef; padding-bottom: 0.8em; margin-bottom: 1.5em; font-size: 2em; }
        footer { text-align: center; margin-top: 4em; padding-top: 2em; border-top: 1px solid #e9ecef; color: #777; font-size: 0.9em; background-color: #ffffff; }

        /* CTA button style */
        .cta-button {
            display: inline-block;
            background-color: #007bff; /* Primary brand color */
            color: white;
            padding: 15px 30px;
            text-decoration: none;
            border-radius: 8px;
            margin-top: 1.5em;
            font-size: 1.2em;
            font-weight: bold;
            transition: background-color 0.3s ease, transform 0.2s ease;
            box-shadow: 0 4px 10px rgba(0,0,0,0.1);
        }
        .cta-button:hover {
            background-color: #0056b3; /* Darker shade on hover */
            transform: translateY(-2px);
            box-shadow: 0 6px 12px rgba(0,0,0,0.15);
        }

        /* Pricing section styles */
        #pricing {
            text-align: center;
            padding: 2em;
            background-color: #f0f8ff; /* Light background for emphasis */
            border-radius: 12px;
            margin-bottom: 3em;
            box-shadow: 0 5px 15px rgba(0,0,0,0.08);
        }
        #pricing h2 {
            color: #007bff;
            font-size: 2.8em;
            margin-bottom: 0.6em;
            border-bottom: none;
            padding-bottom: 0;
        }
        #pricing > p { /* Description under the main pricing title */
            font-size: 1.3em;
            margin-bottom: 2em;
            color: #555;
            max-width: 700px;
            margin-left: auto;
            margin-right: auto;
        }
        .pricing-tiers-container {
            display: flex;
            justify-content: center;
            flex-wrap: wrap; /* Allow wrapping on smaller screens */
            gap: 2em; /* Space between cards */
            margin-top: 2em;
        }
        .pricing-card {
            background-color: #ffffff;
            border: 1px solid #e9ecef;
            border-radius: 10px;
            padding: 2em;
            width: 300px; /* Fixed width for consistency */
            box-shadow: 0 4px 15px rgba(0,0,0,0.08);
            transition: transform 0.3s ease, box-shadow 0.3s ease;
            text-align: left;
            display: flex; /* Flexbox for internal content alignment */
            flex-direction: column;
            justify-content: space-between;
        }
        .pricing-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 8px 20px rgba(0,0,0,0.12);
        }
        .pricing-card h3 {
            color: #007bff;
            font-size: 1.8em;
            margin-top: 0;
            margin-bottom: 0.5em;
        }
        .pricing-card .price {
            font-size: 2.5em;
            font-weight: bold;
            color: #28a745; /* Green for price */
            margin-bottom: 1em;
        }
        .pricing-card p {
            font-size: 1em;
  ────────────────────────────────────────
  ▸ shell
    command: grep -r "dashboard\|onboarding" /opt/wgmesh-checkout/pkg/ --include="*.go" | head -20

(no output)
  ────────────────────────────────────────
  ▸ shell
    command: grep -r "Time-to-First-Mesh\|TTFM" /opt/wgmesh-checkout/ --include="*.go" --include="*.md" | head -10

(no output)
  ────────────────────────────────────────
  ▸ shell
    command: cat /opt/wgmesh-checkout/README.md | head -150

<a href="https://viberank.dev/apps/wgmesh" target="_blank" rel="noopener noreferrer"><img src="https://viberank.dev/badge?app=wgmesh&theme=dark" alt="wgmesh on VibeRank" /></a>
<a href="https://www.producthunt.com/products/wgmesh?embed=true&amp;utm_source=badge-featured&amp;utm_medium=badge&amp;utm_campaign=badge-wgmesh" target="_blank" rel="noopener noreferrer"><img alt="wgmesh - Decentralized WireGuard mesh builder with DHT discovery | Product Hunt" width="250" height="54" src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=1081094&amp;theme=light&amp;t=1771444856938"></a>
[![Chimney Deploy](https://github.com/atvirokodosprendimai/wgmesh/actions/workflows/chimney-deploy.yml/badge.svg)](https://github.com/atvirokodosprendimai/wgmesh/actions/workflows/chimney-deploy.yml)
# wgmesh — Share a Secret, Build a Mesh

**Build encrypted mesh networks in minutes, not hours.** Generate a shared secret, run `wgmesh join` on each node, and let DHT discovery wire everything together — NAT traversal, endpoint detection, and route management included.

## Motivation

Setting up WireGuard between two machines is simple. Setting it up between *ten* is a nightmare of key exchanges, endpoint tracking, and config file juggling. Every time you add or remove a node, every other node's config needs updating.

Existing tools either require a coordination server you have to host and trust (Tailscale, Netmaker), or need manual key distribution (innernet). wgmesh takes a different approach:

- **Decentralized mode** — nodes discover each other automatically via DHT using a shared secret. No coordination server, no manual config. Just `wgmesh join` and you're in the mesh.
- **Centralized mode** — SSH into your fleet, deploy WireGuard configs, and manage the topology from a single state file. Diff-based updates mean minimal disruption.

Both modes handle NAT traversal, route propagation, and persistence across reboots out of the box.

## Quick Start

```bash
# Generate a secret (once)
wgmesh init --secret

# On every node — same secret, automatic discovery
wgmesh join --secret "wgmesh://v1/<your-secret>"

# Check status
wgmesh status --secret "wgmesh://v1/<your-secret>"
```

That's it. Nodes find each other via DHT, exchange keys, and build the mesh.

For a step-by-step walkthrough with verification steps, troubleshooting, and all install methods,
see [docs/quickstart.md](docs/quickstart.md).

## Common Use Cases

### Home lab / self-hosted services

Connect your home server, a VPS, and a laptop into a single private network without opening
firewall ports. Every node gets a stable mesh IP regardless of its real IP or NAT situation.

```bash
# On each machine — same secret, automatic discovery
sudo wgmesh join --secret "wgmesh://v1/<your-secret>"
```

### Remote development / team VPN

Give every developer a persistent mesh IP. Expose internal services (databases, staging
environments) without a VPN server or static IP. New team member joins by receiving the secret.

```bash
# Developer laptop joins the team mesh
sudo wgmesh join --secret "wgmesh://v1/<team-secret>" --interface wg1
```

### Advertising subnets (site-to-site)

A node can advertise a local subnet into the mesh so all peers route traffic through it — useful
for connecting office networks or exposing a Kubernetes pod CIDR.

```bash
sudo wgmesh join \
  --secret "wgmesh://v1/<your-secret>" \
  --advertise-routes "192.168.10.0/24"
```

### Fleet management (centralized mode)

Manage WireGuard across a large fleet from a single control node. Topology lives in a state file;
changes are deployed via SSH with zero interface restarts.

```bash
wgmesh -init
wgmesh -add node1:10.99.0.1:192.168.1.10
wgmesh -add node2:10.99.0.2:203.0.113.50
wgmesh -deploy
```

See [docs/centralized-mode.md](docs/centralized-mode.md) for the full reference.

For end-to-end walkthroughs of the most common deployment patterns, see the
[use-case guides](docs/use-cases/README.md).

Evaluating whether wgmesh fits your infrastructure? Use the
[15-minute evaluation checklist](docs/evaluation-checklist.md) to reach a go/no-go decision.

## How It Works

### Mesh Topology

Every node becomes a peer to every other node:

```
node1 <----> node2
  ^            ^
  |            |
  v            v
node3 <----> node4
```

### NAT Traversal

Nodes with public IPs are configured as endpoints for other nodes. Nodes behind NAT use persistent keepalive to maintain connections. NAT status is detected automatically by comparing the SSH host with the detected public IP.

### Online Updates

Deploying changes reads the current WireGuard state via `wg show dump`, calculates a diff against the desired state, and applies changes with `wg set` — no interface restart needed. Routes are managed the same way: stale routes are removed and new ones added in-place.

### State Persistence

Mesh state is persisted in `/var/lib/wgmesh/`. In centralized mode, the state file (`mesh-state.json`) holds the full topology including keys and node metadata. In decentralized mode, each node stores its keypair in `/var/lib/wgmesh/{interface}.json`. WireGuard configuration persists across reboots via systemd (`wg-quick@wg0.service`).

## Usage

### Decentralized Mode (Secret-Based Discovery)

Nodes self-discover and peer automatically via DHT.

```bash
# 1) Generate a mesh secret (run once)
wgmesh init --secret

# 2) Join on each node using the same secret
wgmesh join --secret "wgmesh://v1/<your-secret>"

# 3) Check local derived mesh parameters
wgmesh status --secret "wgmesh://v1/<your-secret>"
```

Common `join` options:

```bash
wgmesh join \
  --secret "wgmesh://v1/<your-secret>" \
  --advertise-routes "192.168.10.0/24,10.0.0.0/8" \
  --listen-port 51820 \
  --interface wg0 \
  --log-level debug \
  --gossip
```

### Centralized Mode (SSH Deployment)

Manage WireGuard across your fleet from a single control node via SSH:

```bash
wgmesh -init                                        # Create mesh state
  ────────────────────────────────────────
  ▸ shell
    command: ls -la /opt/wgmesh-checkout/docs/

total 1852
drwxr-xr-x 11 wgmesh wgmesh    4096 Jun 19 09:39 .
drwxr-xr-x 24 wgmesh wgmesh    4096 Jun 19 09:22 ..
-rw-r--r--  1 wgmesh wgmesh    8880 Jun 11 05:32 access-control.md
drwxr-xr-x  2 wgmesh wgmesh    4096 Jun 11 05:32 brainstorms
-rw-r--r--  1 wgmesh wgmesh    5152 Jun 11 05:32 centralized-mode.md
-rw-r--r--  1 wgmesh wgmesh    1135 Jun 11 05:32 compat-tracking.md
drwxr-xr-x  2 wgmesh wgmesh    4096 Jun 11 05:32 dogfooding
-rw-r--r--  1 wgmesh wgmesh   12616 Jun 16 08:43 evaluation-checklist.md
-rw-r--r--  1 wgmesh wgmesh    4596 Jun 11 05:32 FAQ.md
drwxr-xr-x  2 wgmesh wgmesh    4096 Jun 11 05:32 ideation
-rw-r--r--  1 wgmesh wgmesh   78981 Jun 19 07:05 index.html
-rw-r--r--  1 wgmesh wgmesh    2157 Jun 11 05:32 install-verification.md
-rw-r--r--  1 wgmesh wgmesh   11273 Jun 16 08:43 pilot-evaluation-guide.md
-rw-r--r--  1 wgmesh wgmesh    9490 Jun 11 05:32 pipeline-flow.d2
-rw-r--r--  1 wgmesh wgmesh 1578777 Jun 11 05:32 pipeline-flow.png
-rw-r--r--  1 wgmesh wgmesh   93545 Jun 11 05:32 pipeline-flow.svg
drwxr-xr-x  2 wgmesh wgmesh    4096 Jun 11 05:32 plans
drwxr-xr-x  2 wgmesh wgmesh    4096 Jun 11 05:32 pulse-reports
-rw-r--r--  1 wgmesh wgmesh   10212 Jun 11 05:32 quickstart.md
drwxr-xr-x  2 wgmesh wgmesh    4096 Jun 11 05:32 research
drwxr-xr-x  2 wgmesh wgmesh    4096 Jun 19 09:39 seo
drwxr-xr-x  5 wgmesh wgmesh    4096 Jun 11 05:32 solutions
-rw-r--r--  1 wgmesh wgmesh    1603 Jun 11 05:32 troubleshooting.md
drwxr-xr-x  2 wgmesh wgmesh    4096 Jun 11 05:32 use-cases
  ────────────────────────────────────────
  ▸ shell
    command: cat /opt/wgmesh-checkout/docs/evaluation-checklist.md | head -200

# Evaluation Checklist for Network Administrators

Use this checklist to assess whether wgmesh fits your use case in roughly 15 minutes.
Work through each section top to bottom; the final section gives you a clear go/no-go recommendation.

---

## Section 1 — Infrastructure Requirements (5 minutes)

Mark each item ✅ (met), ❌ (not met), or ⚠️ (needs investigation).

### 1.1 — Kernel / OS

| Requirement | Notes | Status |
|-------------|-------|--------|
| Linux kernel ≥ 5.6, **or** macOS with `wireguard-go` installed | WireGuard is built into Linux 5.6+; older kernels need the DKMS module | |
| `wireguard-tools` package installed (`wg` command available) | `apt install wireguard-tools` / `yum install wireguard-tools` | |
| Root access or `CAP_NET_ADMIN` capability on each node | Required to create and configure WireGuard interfaces | |

### 1.2 — Network

| Requirement | Notes | Status |
|-------------|-------|--------|
| Outbound UDP is not completely blocked on every node | At minimum, one node must be reachable over UDP for direct connections | |
| At least one node has a public IP **or** NAT traversal (UDP hole-punching) is acceptable | Nodes behind symmetric NAT may require a relay path | |
| Nodes can reach the public internet for initial DHT discovery | Outbound UDP to DHT bootstrap nodes, or HTTPS to GitHub API for the registry fallback | |

### 1.3 — Storage and State

| Requirement | Notes | Status |
|-------------|-------|--------|
| `/var/lib/wgmesh/` is writable on each node | Stores WireGuard keypair and peer cache; ~few KB per node | |
| Systemd available (if persistent service is required) | `wgmesh install-service` creates a `wgmesh.service` unit | |

**Section 1 verdict:** If any item is ❌, resolve it before proceeding. Items marked ⚠️ are addressed in Section 3.

---

## Section 2 — Use Case Fit (5 minutes)

Answer each question and tally the mode recommendations.

### 2.1 — Topology Questions

**Q1: How many nodes will the mesh contain at steady state?**
- ≤ 50 nodes → **decentralized mode preferred**
- 50–200 nodes → **either mode works; decentralized scales to this range**
- > 200 nodes → ⚠️ evaluate centralized mode or contact the project for guidance

**Q2: Who manages node additions and removals?**
- Nodes join/leave autonomously (e.g., auto-scaling, developer laptops) → **decentralized mode**
- An operator controls all changes via SSH from a central host → **centralized mode**

**Q3: Are all nodes behind NAT, or do some have public IPs?**
- All behind NAT → decentralized mode with UDP hole-punching; check that at least one bootstrap peer is reachable
- At least one node has a public IP → direct endpoint configuration; both modes handle this

**Q4: Do you require site-to-site routing (advertising subnets)?**
- Yes → both modes support `--advertise-routes`; decentralized mode propagates routes automatically
- No → no impact on mode choice

### 2.2 — Operational Questions

**Q5: Is there a central operations team that manages WireGuard state?**
- Yes → **centralized mode** (state file in `mesh-state.json`, SSH-based deployment)
- No / decentralized DevOps → **decentralized mode**

**Q6: Do nodes need to discover each other without pre-sharing IP addresses?**
- Yes (dynamic IPs, cloud auto-scaling, remote workers) → **decentralized mode required**
- No (static IP fleet) → either mode works

**Q7: Is an encrypted state file at rest required?**
- Yes → centralized mode supports AES-256-GCM + PBKDF2 encryption of `mesh-state.json`
- No preference → either mode

### 2.3 — Mode Recommendation

Count your answers:
- Mostly decentralized → proceed with **decentralized mode** (`wgmesh join`)
- Mostly centralized → proceed with **centralized mode** (`wgmesh -deploy`)
- Mixed → decentralized mode is the default; use centralized for operator-controlled fleets

---

## Section 3 — Decision Framework: wgmesh vs Alternatives (2 minutes)

Use this table to confirm wgmesh is the right tool. If a competing tool better fits your profile, the ❌ cells explain why.

| Scenario | wgmesh | Tailscale | Netmaker | innernet |
|----------|--------|-----------|----------|----------|
| No coordination server to host or trust | ✅ (DHT-based, no server) | ❌ (requires Tailscale control plane) | ❌ (requires Netmaker server) | ❌ (requires innernet server) |
| Self-hosted, open-source, auditable | ✅ | ❌ (SaaS) | ✅ (self-hosted) | ✅ (self-hosted) |
| Serverless peer discovery (NAT traversal included) | ✅ | ✅ | ❌ (server required) | ❌ (server required) |
| Share one secret to add any node | ✅ | ❌ (ACL/invite required) | ❌ (token per node) | ❌ (certificate per node) |
| macOS and Linux support | ✅ | ✅ | ✅ | ✅ |
| Windows support | ❌ (not yet) | ✅ | ✅ | ❌ |
| Web UI / dashboard | ❌ | ✅ | ✅ | ❌ |
| Per-node access control lists | ✅ (centralized mode policy engine) | ✅ | ✅ | ✅ |
| No dedicated coordination server to operate | ✅ (DHT + GitHub Issues registry) | ❌ (requires Tailscale control plane) | ❌ (requires Netmaker server) | ❌ (requires innernet server) |

**Choose wgmesh when:** you want a coordination-server-free, self-hosted mesh where any node can join with a shared secret, and you do not need Windows support or a web dashboard.

**Choose an alternative when:** you need Windows clients, a web UI, or an enterprise-grade access-control system with per-user identity.

---

## Section 4 — Evaluation Test Scenarios (3 minutes to read; 15–30 minutes to run)

Run these tests during your pilot to confirm wgmesh behaves as expected.

### Test A — Two-node basic mesh (decentralized mode)

**Setup:** Two hosts (can be VMs or VPS). Both must have outbound internet access.

**Steps:**
1. On **host-1**: `wgmesh init --secret` → copy the printed secret.
2. On **host-1**: `sudo wgmesh join --secret "<secret>" --log-level debug`
3. On **host-2**: `sudo wgmesh join --secret "<secret>" --log-level debug`
4. Wait up to 30 seconds for DHT discovery.
5. On **host-1**: `wgmesh peers list`

**Pass criteria:**
- `wgmesh peers list` shows **host-2** with a mesh IP and a non-stale `LAST SEEN` timestamp.
- `ping <host-2-mesh-ip>` from **host-1** succeeds.
- `sudo wg show` on **host-1** shows **host-2** with a recent `latest-handshake`.

**Fail indicator:** No peers appear after 2 minutes → check UDP outbound; run the daemon in foreground with `--log-level debug` and look for `[dht]` lines in the output.

---

### Test B — NAT traversal (both hosts behind NAT)

**Setup:** Two hosts behind different NAT gateways (e.g., two cloud VMs in different VPCs without public IPs, or two developer laptops on different home networks).

**Steps:**
1. Repeat Test A steps with both hosts behind NAT.
2. Observe `ENDPOINT` column in `wgmesh peers list`.

**Pass criteria:**
- Peer appears with an `ENDPOINT` of the form `<public-ip>:<port>` (hole-punched) **or** `(relayed)`.
- `ping <mesh-ip>` succeeds in either case.

**Note:** Symmetric NAT on both ends may result in `(relayed)` — this is expected and functional.

---

### Test C — Subnet advertisement (site-to-site routing)

**Setup:** **host-1** has a private subnet `192.168.10.0/24` behind it (or a loopback alias for testing).

**Steps:**
1. On **host-1**:
   ```bash
   sudo wgmesh join --secret "<secret>" --advertise-routes "192.168.10.0/24"
   ```
2. On **host-2**: `wgmesh peers list` → confirm **host-1** shows `ROUTES: 192.168.10.0/24`.
3. On **host-2**: `ip route get 192.168.10.1` → output should route through `wg0`.
4. On **host-2**: `ping 192.168.10.1` (or a host in that subnet).

**Pass criteria:**
- Route `192.168.10.0/24 via <host-1-mesh-ip> dev wg0` is present on **host-2**.
- Traffic to the subnet is forwarded correctly.

---

### Test D — Node restart / persistence

**Steps:**
1. After Test A is passing, stop the daemon on **host-1**: `sudo systemctl stop wgmesh` (or Ctrl-C).
2. Restart: `sudo systemctl start wgmesh` (or re-run `sudo wgmesh join ...`).
3. On **host-1**: wait 30 seconds, then `wgmesh peers list`.

**Pass criteria:**
- **host-2** reappears in the peer list within 60 seconds without manual intervention.
- Same mesh IPs are used as before (deterministic from secret).

---

### Test E — Adding a third node

**Steps:**
1. While Test A is passing, bring up **host-3** with the same secret.
2. On **host-1** and **host-2**: `wgmesh peers list` after 30 seconds.

**Pass criteria:**
- All three nodes see each other.
- `wgmesh peers count` returns `2` on each node (two remote peers).

---

## Section 5 — Pilot Setup Recommendations

### Minimal pilot topology

- **2–3 nodes**: one with a public IP (or Hetzner/DigitalOcean cheapest VPS), one or two behind NAT.
- **Duration**: 48–72 hours continuous operation to confirm stability and peer reconnection after restarts.
- **Monitoring**: tail daemon logs (`journalctl -u wgmesh -f`) and periodically run `wgmesh peers list`.

### Recommended pilot sequence

  ────────────────────────────────────────
  ▸ shell
    command: cat /opt/wgmesh-checkout/docs/pilot-evaluation-guide.md

# wgmesh Pilot Evaluation Guide

This guide walks network administrators through a structured 30-day evaluation of wgmesh
using the built-in pilot framework. By the end of this evaluation you will have measurable
data on mesh stability, peer discovery, NAT traversal, and operational readiness.

---

## Prerequisites

Before starting the pilot, ensure your environment meets these requirements:

| Requirement | Details |
|-------------|---------|
| Linux kernel ≥ 5.6 | WireGuard is built into Linux 5.6+ |
| `wireguard-tools` installed | `apt install wireguard-tools` or equivalent |
| Root access or `CAP_NET_ADMIN` | Required to create WireGuard interfaces |
| Outbound UDP not blocked | At least one node must be reachable over UDP |
| 2–5 test nodes | Mix of public IP and NAT-behind nodes recommended |
| `/etc/wgmesh/` writable | Stores pilot configuration and peer cache |
| Systemd (optional) | For persistent service via `install-service` |

See [evaluation-checklist.md](evaluation-checklist.md) for a detailed pre-evaluation checklist.

---

## Quick Start

### 1. Initialize the Pilot (Day 0)

On your primary evaluation node:

```bash
wgmesh pilot init \
  --org "Your Organization" \
  --contact admin@yourorg.com \
  --nodes 5 \
  --mode decentralized \
  --duration 30
```

This creates `/etc/wgmesh/pilot.yaml` with default milestones and metrics targets.

### 2. Generate a Mesh Secret

```bash
wgmesh init --secret
```

Copy the printed `wgmesh://v1/...` URI.

### 3. Start the Pilot

```bash
wgmesh pilot start
```

This starts the 30-day evaluation clock and begins metrics collection.

### 4. Deploy to Pilot Nodes

On each node, join the mesh:

```bash
wgmesh join --secret "wgmesh://v1/..."
```

For persistent operation:

```bash
wgmesh install-service --secret "wgmesh://v1/..."
```

### 5. Monitor Progress

```bash
wgmesh pilot status       # Current phase, milestones, days elapsed
wgmesh pilot validate     # Run health checks
wgmesh pilot report       # Generate evaluation report
```

---

## Four-Phase Milestone Structure

The pilot progresses through four phases, each with specific validation criteria.

### Phase 1: Baseline Setup (Days 1–3)

**Goal:** Successful deployment and basic connectivity.

**Tasks:**
- Install wgmesh on all pilot nodes
- Configure mesh secret and join mesh
- Verify peer discovery across all nodes
- Run `wgmesh pilot status` to confirm Phase 1 progress

**Validation:**
- [ ] All peers visible in `wgmesh peers list`
- [ ] `ping <mesh-ip>` succeeds between all peer pairs
- [ ] No interface churn (WireGuard restart loops)
- [ ] `wgmesh pilot validate` reports no errors

**Commands to verify:**
```bash
wgmesh peers list                    # Should show all pilot nodes
wg show                              # Check latest handshake times
ping <mesh-ip-of-another-node>       # Verify connectivity
wgmesh pilot validate                # Run health checks
```

**Mark milestone complete:**
```bash
# Milestones are tracked automatically; you can also manually mark:
# (via the pilot status reporting — all milestones appear in reports)
```

---

### Phase 2: Mesh Stability (Days 4–7)

**Goal:** Verify mesh stability under normal operations.

**Tasks:**
- Run continuous connectivity tests (24h soak)
- Verify route propagation after network changes
- Test graceful node restart and reconnection
- Log key metrics: connection uptime, discovery success rate

**Validation:**
- [ ] ≥99.9% connectivity uptime between all nodes
- [ ] All routes propagate within 30 seconds of topology change
- [ ] Zero daemon crashes or WireGuard interface crashes
- [ ] NAT type detection completed for all nodes

**Commands to verify:**
```bash
wgmesh pilot status                  # Check milestone progress
wgmesh peers count                   # Verify all peers still active
journalctl -u wgmesh --since "24 hours ago" | grep -i error
wgmesh pilot report                  # Generate progress report
```

**Simulating a node restart:**
```bash
# On the test node:
sudo systemctl restart wgmesh        # Or: Ctrl-C and re-run join

# Wait 60 seconds, then from another node:
wgmesh peers list                    # Node should reappear
```

---

### Phase 3: Production Traffic Simulation (Days 8–14)

**Goal:** Validate under realistic workload.

**Tasks:**
- Route application traffic through mesh
- Measure throughput and latency
- Test with intermittent network failures (simulated outages)
- Exercise all discovery layers (registry, LAN, DHT, gossip)

**Validation:**
- [ ] Throughput ≥80% of native WireGuard baseline
- [ ] Latency overhead <20ms compared to native WireGuard
- [ ] Successful recovery from simulated network partitions
- [ ] All discovery layers successfully used

**Throughput testing:**
```bash
# On node A (server):
iperf3 -s -B <mesh-ip-a>

# On node B (client):
iperf3 -c <mesh-ip-a> -t 30         # 30-second throughput test
```

**Latency testing:**
```bash
# Native WireGuard latency:
ping -c 100 <mesh-ip> | tail -1

# Compare with direct IP latency to assess overhead
```

**Simulating network failure:**
```bash
# Temporarily block traffic on one node:
sudo iptables -A OUTPUT -d <other-node-ip> -j DROP

# Wait 30 seconds, then restore:
sudo iptables -D OUTPUT -d <other-node-ip> -j DROP

# Verify reconnection:
wgmesh peers list
```

---

### Phase 4: Advanced Scenarios & NAT Traversal (Days 15–30)

**Goal:** Stress-test edge cases and operational workflows.

**Tasks:**
- Deploy nodes behind diverse NAT types (Full Cone, Symmetric, etc.)
- Test relay fallback when direct connection fails
- Verify secret rotation workflow
- Exercise operational procedures: daemon restart, config reload, node add/remove

**Validation:**
- [ ] Successful hole-punching across all NAT type combinations
- [ ] Relay fallback engages within 60 seconds of direct path failure
- [ ] Zero secret leaks or key derivation failures
- [ ] Clean node addition/removal with no orphaned WireGuard configs

**Adding a new node mid-pilot:**
```bash
# On the new node:
wgmesh join --secret "wgmesh://v1/..."

# From existing nodes:
wgmesh peers list                    # New node should appear within 60s
```

**Removing a node:**
```bash
# On the node being removed:
sudo wgmesh uninstall-service        # If running as service
sudo systemctl stop wgmesh           # Stop the daemon

# Other nodes will mark it as stale after timeout
```

**Secret rotation:**
```bash
wgmesh rotate-secret --current "wgmesh://v1/old-secret"
# Follow printed instructions to deploy new secret
```

---

## Reports and Metrics

### Generating Reports

```bash
# Console report (default)
wgmesh pilot report

# JSON export for automated analysis
wgmesh pilot report --format json --output pilot-report.json

# HTML report for executive summary
wgmesh pilot report --format html --output pilot-report.html
```

### Default Metrics Targets

| Metric | Target | Description |
|--------|--------|-------------|
| Mesh Connectivity | ≥99.9% | Uptime between all peer pairs |
| Peer Discovery Time | ≤60s | Time for all nodes to discover each other |
| Route Propagation | ≤30s | Time for route changes to propagate |
| Throughput | ≥80 Mbps | Compared to native WireGuard |
| Latency Overhead | ≤20ms | Additional latency vs native WireGuard |

### Report Sections

Each report contains:

1. **Milestone Status** — Progress through the four phases
2. **Key Metrics** — Connectivity, discovery, route propagation, restart counts
3. **Discovery Layer Distribution** — Usage of Registry, LAN, DHT, and Gossip layers
4. **NAT Types Detected** — Breakdown of NAT types across pilot nodes
5. **Issues / Warnings** — Any metrics below targets or operational issues
6. **Next Steps** — Guidance on what to do next

---

## Completing the Pilot

### Final Evaluation

```bash
wgmesh pilot complete
```

This produces a final report with:

- **Overall Rating**: Excellent / Good / Fair / Poor
- **Recommendation**: Production readiness assessment
- **Milestone Summary**: Which milestones were completed
- **Metrics Summary**: Final metrics across the evaluation period

### Rating Criteria

| Rating | Score | Meaning |
|--------|-------|---------|
| Excellent | ≥90 | Ready for production deployment |
| Good | 70–89 | Suitable for production with monitoring |
| Fair | 50–69 | Requires investigation before production |
| Poor | <50 | Not recommended for production |

### Saving Results

```bash
# Save final report as JSON
wgmesh pilot complete --output pilot-final.json

# The console output shows the human-readable summary
```

---

## Troubleshooting

### Peers Not Discovered

**Symptoms:** `wgmesh peers list` shows no peers after 5+ minutes.

**Check:**
```bash
# Verify both nodes use the same secret
wgmesh status --secret "wgmesh://v1/..."   # Compare Network ID

# Check UDP connectivity
wgmesh test-peer --secret "wgmesh://v1/..." --peer <ip:port>

# Check firewall rules
sudo iptables -L -n | grep -i drop
```

**Common causes:**
- Different secrets on different nodes (Network IDs won't match)
- Firewall blocking outbound UDP
- No internet access for DHT bootstrap

### High Latency

**Symptoms:** Latency overhead exceeds 20ms target.

**Check:**
```bash
# Compare native vs mesh latency
ping -c 10 <direct-ip>     # Direct latency
ping -c 10 <mesh-ip>       # Mesh latency

# Check if relay is being used
wgmesh peers list          # Look for "(relayed)" in endpoint
```

**Common causes:**
- Relay path instead of direct (Symmetric NAT)
- High baseline latency between regions

### Daemon Crashes

**Symptoms:** Daemon exits unexpectedly, shown in pilot report as restart count.

**Check:**
```bash
journalctl -u wgmesh --since "1 hour ago" | grep -i "panic\|fatal\|error"
```

**Action:** File an issue at https://github.com/atvirokodosprendimai/wgmesh/issues
with the pilot report and relevant log output.

### Connectivity Drops

**Symptoms:** Pilot report shows mesh connectivity below 99.9%.

**Check:**
```bash
# Monitor connectivity in real-time
watch -n 5 'wg show wg0 | grep latest-handshake'

# Check for WireGuard interface issues
sudo wg show wg0
```

---

## FAQ

**Q: Can I run the pilot with fewer than 5 nodes?**
A: Yes. Set `--nodes` to your actual count. The minimum is 1, but 2–3 is recommended
   for meaningful evaluation.

**Q: Can I shorten the pilot duration?**
A: Yes. The minimum is 7 days (`--duration 7`), but the full 30-day evaluation
   provides the most comprehensive results.

**Q: Do I need a separate node for pilot management?**
A: No. The pilot commands run on any node with the pilot configuration. Typically
   you run them on your primary evaluation node.

**Q: Can I run multiple pilots simultaneously?**
A: No. The current implementation supports one pilot at a time per configuration
   file. Use separate configuration paths if needed.

**Q: What happens if I restart the daemon?**
A: Daemon restarts are tracked in pilot metrics. A few restarts during evaluation
   are acceptable; excessive restarts will lower the overall rating.

**Q: How do I reset and start over?**
A: Remove the pilot configuration and reinitialize:
```bash
sudo rm /etc/wgmesh/pilot.yaml
wgmesh pilot init --org "..." --contact "..."
wgmesh pilot start
```
  ────────────────────────────────────────
  ▸ shell
    command: grep -n "func.*HTTP\|http\|server\|ListenAndServe" /opt/wgmesh-checkout/main.go | head -20

9:	"net/http"
10:	_ "net/http/pprof"
22:	"github.com/prometheus/client_golang/prometheus/promhttp"
302:	pprofAddr := fs.String("pprof", "", "Enable pprof HTTP server (e.g. localhost:6060)")
303:	metricsAddr := fs.String("metrics", "", "Enable Prometheus metrics server (e.g. :9090)")
372:	// Start pprof HTTP server if requested (for profiling/flame graphs)
375:			log.Printf("pprof server listening on %s", *pprofAddr)
376:			if err := http.ListenAndServe(*pprofAddr, nil); err != nil {
377:				log.Printf("pprof server error: %v", err)
382:	// Start Prometheus metrics server if requested
385:		metricsMux := http.NewServeMux()
386:		metricsMux.Handle("/metrics", promhttp.Handler())
388:			log.Printf("metrics server listening on %s", *metricsAddr)
389:			if err := http.ListenAndServe(*metricsAddr, metricsMux); err != nil {
390:				log.Printf("metrics server error: %v", err)
395:	// Setup RPC server
402:	// Create RPC server with callback functions
405:		fmt.Fprintf(os.Stderr, "Warning: failed to create RPC server: %v\n", err)
898:// createRPCServer creates an RPC server for the daemon
  ────────────────────────────────────────
  ▸ shell
    command: ls -la /opt/wgmesh-checkout/specs/

total 500
drwxr-xr-x  5 wgmesh wgmesh  4096 Jun 19 09:27 .
drwxr-xr-x 24 wgmesh wgmesh  4096 Jun 19 09:22 ..
-rw-r--r--  1 wgmesh wgmesh     0 Jun 11 05:32 .gitkeep
drwxr-xr-x  2 wgmesh wgmesh  4096 Jun 11 05:32 implemented
-rw-r--r--  1 wgmesh wgmesh 38865 Jun 11 05:32 issue-176-spec.md
-rw-r--r--  1 wgmesh wgmesh  6359 Jun 11 05:32 issue-178-spec.md
-rw-r--r--  1 wgmesh wgmesh  9896 Jun 11 05:32 issue-181-spec.md
-rw-r--r--  1 wgmesh wgmesh  4002 Jun 11 05:32 issue-470-spec.md
-rw-r--r--  1 wgmesh wgmesh 10914 Jun 11 05:32 issue-475-spec.md
-rw-r--r--  1 wgmesh wgmesh 13898 Jun 11 05:32 issue-492-spec.md
-rw-r--r--  1 wgmesh wgmesh 27211 Jun 11 05:32 issue-496-spec.md
-rw-r--r--  1 wgmesh wgmesh  8660 Jun 11 05:32 issue-497-spec.md
-rw-r--r--  1 wgmesh wgmesh 12771 Jun 11 05:32 issue-499-spec.md
-rw-r--r--  1 wgmesh wgmesh 14317 Jun 11 05:32 issue-505-spec.md
-rw-r--r--  1 wgmesh wgmesh 10537 Jun 11 05:32 issue-508-spec.md
-rw-r--r--  1 wgmesh wgmesh 17368 Jun 11 05:32 issue-510-spec.md
-rw-r--r--  1 wgmesh wgmesh 16452 Jun 11 05:32 issue-512-spec.md
-rw-r--r--  1 wgmesh wgmesh  6841 Jun 11 05:32 issue-520-spec.md
-rw-r--r--  1 wgmesh wgmesh  4206 Jun 11 05:32 issue-523-spec.md
-rw-r--r--  1 wgmesh wgmesh 15581 Jun 11 05:32 issue-532-spec.md
-rw-r--r--  1 wgmesh wgmesh 28323 Jun 11 05:32 issue-536-spec.md
-rw-r--r--  1 wgmesh wgmesh 15797 Jun 11 05:49 issue-539-spec.md
-rw-r--r--  1 wgmesh wgmesh 11823 Jun 11 05:32 issue-545-spec.md
-rw-r--r--  1 wgmesh wgmesh 22986 Jun 11 05:32 issue-547-spec.md
-rw-r--r--  1 wgmesh wgmesh 32720 Jun 11 05:32 issue-550-spec.md
-rw-r--r--  1 wgmesh wgmesh 23780 Jun 11 05:32 issue-551-spec.md
-rw-r--r--  1 wgmesh wgmesh 11202 Jun 11 05:32 issue-556-spec.md
-rw-r--r--  1 wgmesh wgmesh  5225 Jun 11 05:32 issue-561-spec.md
-rw-r--r--  1 wgmesh wgmesh 18062 Jun 11 05:32 issue-571-spec.md
-rw-r--r--  1 wgmesh wgmesh  4747 Jun 11 05:32 issue-573-spec.md
-rw-r--r--  1 wgmesh wgmesh  6974 Jun 11 05:32 issue-587-spec.md
-rw-r--r--  1 wgmesh wgmesh  2697 Jun 11 05:32 issue-591-spec.md
-rw-r--r--  1 wgmesh wgmesh  6151 Jun 11 05:32 issue-609-spec.md
-rw-r--r--  1 wgmesh wgmesh  7873 Jun 11 05:32 issue-641-spec.md
-rw-r--r--  1 wgmesh wgmesh 10744 Jun 11 05:32 issue-714-spec.md
drwxr-xr-x  2 wgmesh wgmesh  4096 Jun 11 05:32 not-implemented
drwxr-xr-x  2 wgmesh wgmesh  4096 Jun 11 05:32 partial
-rw-r--r--  1 wgmesh wgmesh  4246 Jun 11 05:32 STATUS.md
  ────────────────────────────────────────
  ▸ shell
    command: head -100 /opt/wgmesh-checkout/specs/issue-176-spec.md

# Specification: Issue #176

## Classification
feature

## Deliverables
code + documentation

## Problem Analysis

Currently, wgmesh operates as a "flat network" in centralized mode where all nodes can reach all other nodes through the WireGuard mesh. Every node is configured as a peer to every other node, and the `AllowedIPs` configuration permits traffic from all mesh IPs and routable networks.

This creates a security and operational challenge:
- No ability to segment the network into groups (e.g., production vs staging)
- No way to restrict which nodes can communicate with which resources
- All nodes have full mesh connectivity regardless of their purpose or trust level
- Networks behind nodes (via `routable_networks`) are accessible to all mesh members

### Current Architecture (Centralized Mode)

From `pkg/mesh/deploy.go`, the `generateConfigForNode()` function creates a full mesh:
- Each node gets **all other nodes** as WireGuard peers
- `AllowedIPs` includes the mesh IP (`/32`) of every peer plus all their `routable_networks`
- There's no filtering mechanism to limit which peers a node should connect to

Example: If we have 5 nodes (A, B, C, D, E), every node gets 4 peers configured with full access to all networks.

### Business Use Cases

Organizations need network segmentation for:
1. **Environment isolation**: Dev/staging/prod nodes shouldn't all interconnect
2. **Security boundaries**: Database nodes shouldn't be reachable from all nodes
3. **Compliance**: PCI/HIPAA networks require restricted access
4. **Multi-tenant**: Different customers/projects on same mesh infrastructure
5. **Least privilege**: Nodes should only access resources they need

## Proposed Approach

Implement a **group-based access control** system where nodes can be assigned to one or more groups, and access policies define which groups can communicate with which other groups.

### Design Principles

1. **Backward compatible**: Existing meshes without groups continue to work as full mesh
2. **Declarative**: Groups and policies defined in mesh state file
3. **WireGuard native**: Uses AllowedIPs filtering (no external firewall needed)
4. **Simple first**: Start with basic group membership and access rules
5. **Centralized control**: Operator defines policies, deployment enforces them

### Data Model

Extend the mesh state file (`/var/lib/wgmesh/mesh-state.json`) with:

```json
{
  "interface_name": "wg0",
  "network": "10.99.0.0/16",
  "listen_port": 51820,
  "local_hostname": "control-node",
  "groups": {
    "production": {
      "description": "Production environment nodes",
      "members": ["node1", "node2"]
    },
    "staging": {
      "description": "Staging environment",
      "members": ["node3", "node4"]
    },
    "database": {
      "description": "Database servers",
      "members": ["node5"]
    }
  },
  "access_policies": [
    {
      "name": "prod-to-db",
      "description": "Allow production nodes to access database",
      "from_groups": ["production"],
      "to_groups": ["database"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    },
    {
      "name": "staging-isolated",
      "description": "Staging can only talk within staging",
      "from_groups": ["staging"],
      "to_groups": ["staging"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    }
  ],
  "nodes": {
    "node1": {
      "hostname": "node1",
      "mesh_ip": "10.99.0.1",
      "routable_networks": ["192.168.10.0/24"],
      ...
    },
    ...
  }
}
  ────────────────────────────────────────
  ▸ shell
    command: cat /opt/wgmesh-checkout/specs/issue-714-spec.md

# Specification: Issue #714

## Classification
feature

## Deliverables
code + documentation

## Problem Analysis

The issue reports 7 PRs stalled with `copilot-triaging` labels for weeks, indicating a systematic blockage in the spec generation pipeline. After analyzing the codebase, the root cause is architectural, not transient:

1. **No retry or fallback when `goose-triage.yml` fails.** The workflow (`.github/workflows/goose-triage.yml`) fires once on the `needs-triage` label event, runs Goose, and if it fails (API error, timeout, sanitise gate), the issue is left in `copilot-triaging` with no retry mechanism. There is no scheduled scan analogous to the one `spec-auto-approve.yml` provides for spec PRs (line 119: `cron: '*/5 * * * *'`).

2. **No staleness detection for issues stuck in `copilot-triaging`.** The existing `e2e-stalled-watcher.js` only monitors `awaiting-verification` issues. Nothing monitors issues carrying `copilot-triaging` for too long. The `board-sync.yml` maps `copilot-triaging` to the "Spec in Progress" column (line 51), but never alerts on dwell time.

3. **No pipeline-health dashboard surface.** The `agent-metrics-report.yml` produces weekly DORA metrics from uploaded artifacts, but has no concept of pipeline stage latency per issue. The `pulse.sh` script queries external signals (Polar, GitHub stars) but never queries internal pipeline health.

4. **The `spec-auto-approve.yml` scheduled scan (line 119) already demonstrates the pattern** — it catches spec PRs that `pull_request_target` missed. The same pattern should apply to the triage stage.

The fix requires three new components:

- A **spec-triage-watcher** workflow (cron + workflow_dispatch) that detects issues with `copilot-triaging` label for longer than a configurable SLA and either re-triggers `goose-triage.yml` or alerts.
- A **pipeline health** section in the pulse report showing triage latency and stall counts.
- A new **pipeline-health label** (`spec-stalled`) for surfacing stuck issues.

## Proposed Approach

Add a scheduled triage watcher workflow (analogous to `e2e-stalled-watcher.yml`) that detects issues carrying `copilot-triaging` beyond a configurable SLA (default 6 hours), labels them `spec-stalled`, and re-dispatches `goose-triage.yml` for automatic recovery. Extend `scripts/pulse.sh` with a new "Pipeline Health" section querying triage-stage latency and stall counts via `gh issue list`. Create a testable JavaScript handler module following the `e2e-stalled-watcher.js` pattern.

## Implementation Tasks

### Task 1: Create label `spec-stalled` in `.github/labels.yml`

- **File:** `.github/labels.yml` (modify)
- **What:** Add a new label entry for `spec-stalled` after the existing resolution labels block.
- **Detail:** Append the following entry to the YAML array in `.github/labels.yml`, after the `needs-info` label block (around line 37):
  ```yaml
  - name: spec-stalled
    color: "B60205"
    description: "Issue stuck in copilot-triaging beyond SLA — spec generation did not complete"
  ```

### Task 2: Create testable handler module `scripts/workflows/spec-triage-watcher.js`

- **File:** `scripts/workflows/spec-triage-watcher.js` (create)
- **What:** Export an async `handler({github, context, core, nowMs})` function that detects issues stuck in `copilot-triaging` beyond an SLA budget, labels them `spec-stalled`, and returns the stalled issue numbers.
- **Detail:** Follow the exact pattern from `scripts/workflows/e2e-stalled-watcher.js`. The module must:
  1. Export a `handler` function and helper functions for testing (`shouldFlag`, `labelNamesOf`, `STALL_BUDGET_MS`, `TERMINAL_LABELS`).
  2. Define `STALL_BUDGET_MS = 6 * 60 * 60 * 1000` (6 hours).
  3. Define `TERMINAL_LABELS = new Set(['spec-stalled', 'wont-do', 'needs-info'])` — if an issue already carries any of these, skip it.
  4. `shouldFlag({labels, updatedAt, now, budgetMs})` returns `true` when: `labels` includes `copilot-triaging`, `now - updatedMs > budgetMs`, and no terminal label is present.
  5. `handler` paginates open issues with label `copilot-triaging` via `github.rest.issues.listForRepo`. For each issue where `shouldFlag` returns true, call `github.rest.issues.addLabels` with `['spec-stalled']` and log via `core.info`.
  6. Append a step summary to `GITHUB_STEP_SUMMARY` (if set) showing count and issue numbers.
  7. Return `{ stalledCount, stalledNumbers }`.
  8. Filter out PRs (issues where `pull_request` key is truthy).

### Task 3: Create unit tests `scripts/workflows/spec-triage-watcher.test.js`

- **File:** `scripts/workflows/spec-triage-watcher.test.js` (create)
- **What:** Write unit tests for `spec-triage-watcher.js` following the pattern in `scripts/workflows/e2e-stalled-watcher.test.js`.
- **Detail:** Use Node.js `node:assert` (no test framework). Test `shouldFlag` with these cases:
  - Issue with `copilot-triaging`, updated 7h ago → `true`
  - Issue with `copilot-triaging`, updated 3h ago → `false` (within SLA)
  - Issue with `copilot-triaging` + `spec-stalled`, updated 7h ago → `false` (terminal label)
  - Issue with `copilot-triaging` + `wont-do`, updated 7h ago → `false` (terminal label)
  - Issue without `copilot-triaging`, updated 7h ago → `false`
  - Issue with missing `updatedAt` → `false`
  Test `handler` with a mock `github` object that returns a controlled issue list. Verify it calls `addLabels` only for qualifying issues and returns correct counts. Run tests via `node scripts/workflows/spec-triage-watcher.test.js`.

### Task 4: Create workflow `.github/workflows/spec-triage-watcher.yml`

- **File:** `.github/workflows/spec-triage-watcher.yml` (create)
- **What:** Create a scheduled workflow that runs the watcher handler every 30 minutes and optionally re-triggers failed triage runs.
- **Detail:** Follow the exact structure of `.github/workflows/e2e-stalled-watcher.yml`. The workflow must:
  1. Trigger on `schedule: cron: '*/30 * * * *'` and `workflow_dispatch`.
  2. Permissions: `contents: write`, `pull-requests: write`, `issues: write`, `actions: write`.
  3. Steps:
     a. Generate app token via `actions/create-github-app-token@v1` (same as other workflows).
     b. Checkout repository.
     c. Ensure `spec-stalled` label exists via `gh label create spec-stalled --description "Issue stuck in copilot-triaging beyond SLA" --color B60205 --force`.
     d. Run the handler via `actions/github-script@v8`:
        ```javascript
        const handler = require('./scripts/workflows/spec-triage-watcher.js');
        const result = await handler({github, context, core});
        ```
     e. Re-trigger stalled issues: if `result.stalledCount > 0`, for each stalled issue number, remove `copilot-triaging` label, add `needs-triage` label (which re-triggers `goose-triage.yml`), and comment on the issue: `"Spec generation stalled for over 6h. Re-triggering triage automatically."`
        Use `github.rest.issues.removeLabel` to remove `copilot-triaging`, `github.rest.issues.addLabels` to add `needs-triage`, and `github.rest.issues.createComment` to post the comment.
     f. Log re-triggered count to step summary.
  4. Use `GITHUB_TOKEN` (from secrets) for issue operations and `app-token` outputs for checkout.

### Task 5: Add pipeline health section to `scripts/pulse.sh`

- **File:** `scripts/pulse.sh` (modify)
- **What:** Add a "Pipeline Health" section to the pulse report output that shows triage-stage stall counts and latency.
- **Detail:** Add a new function `query_pipeline_health()` after the existing `query_github_external_issues()` function (around line 200 in `scripts/pulse.sh`). The function must:
  1. Call `gh_ready` to check `gh` CLI availability.
  2. Query open issues with label `copilot-triaging` via `gh issue list -R "$GH_REPO" --state open --label copilot-triaging --limit 100 --json number,updatedAt --jq '.'`. Count them as `TRIAGING_OPEN`.
  3. Query open issues with label `spec-stalled` via `gh issue list -R "$GH_REPO" --state open --label spec-stalled --limit 100 --json number --jq 'length'`. Store as `SPEC_STALLED_COUNT`.
  4. Set `PIPELINE_HEALTH_RENDER` to a human-readable string like `"${TRIAGING_OPEN} issues in triage, ${SPEC_STALLED_COUNT} spec-stalled"`.
  5. If `gh` is not available, set `PIPELINE_HEALTH_RENDER="no data (gh CLI unavailable)"` and append to `QUERY_FAILURES`.
  6. Call `query_pipeline_health` after `query_github_external_issues` in the main body (around line 260).
  7. Add to the report output: in the `## Headlines` section, add a line `- Pipeline: ${PIPELINE_HEALTH_RENDER}.`. In the `## Followups` section, add `- If SPEC_STALLED_COUNT > 0, review spec-stalled issues for manual triage.` using the existing `render_followups` pattern.

### Task 6: Update `docs/pipeline-flow.d2` with spec-triage-watcher node

- **File:** `docs/pipeline-flow.d2` (modify)
- **What:** Add a `spec-triage-watcher` node to the pipeline flow diagram showing the feedback loop from stalled issues back to `goose-triage`.
- **Detail:** In `docs/pipeline-flow.d2`, add after the existing `e2e-stalled-watcher` definition (if present) or after the `goose-triage` node:
  ```
  spec-triage-watcher: {
    shape: rectangle
    label: "spec-triage-watcher (cron */30m)"
  }
  spec-triage-watcher -> goose-triage: "re-trigger needs-triage"
  copilot-triaging -> spec-triage-watcher: "detects stall >6h"
  spec-triage-watcher -> spec-stalled: "labels spec-stalled"
  ```

## Affected Files

```
.github/labels.yml                                         (modify: add spec-stalled label)
.github/workflows/spec-triage-watcher.yml                  (new)
scripts/workflows/spec-triage-watcher.js                   (new)
scripts/workflows/spec-triage-watcher.test.js              (new)
scripts/pulse.sh                                           (modify: add pipeline health query)
docs/pipeline-flow.d2                                      (modify: add watcher node)
```

## Acceptance Criteria

- `node scripts/workflows/spec-triage-watcher.test.js` passes all test cases.
- `spec-triage-watcher.yml` is valid YAML (`python3 -c "import yaml; yaml.safe_load(open('.github/workflows/spec-triage-watcher.yml'))"` exits 0).
- The `spec-stalled` label definition exists in `.github/labels.yml`.
- `scripts/pulse.sh` includes a `query_pipeline_health` function and emits a "Pipeline:" headline line.
- The watcher workflow triggers on schedule (`*/30 * * * *`) and `workflow_dispatch`.
- Stalled issues (>6h with `copilot-triaging` and no spec PR) are labeled `spec-stalled` and re-triggered with `needs-triage`.
- `go build ./...` passes (no Go code changes, but verify no breakage).
- `make lint` passes (verify with `gofmt -l .` showing no Go changes).

## Estimated Complexity
mediumNow I'll create the implementation specification for issue #773:-32602: Could not parse tool arguments: {"path":"/opt/wgmesh-checkout/specs/issue-773-spec.md","content":"# Specification: Issue #773\n\n## Classification\nfeature\n\n## Problem Analysis\n\n**Issue:** Users lack a standardized, visible onboarding checklist for Time-to-First-Mesh (TTFM), making it difficult to track progress from \"zero to working mesh\" and identify where users drop off in the onboarding flow.\n\n**Context from codebase analysis:**\n\n1. **Existing pilot framework** (`pkg/pilot/`) provides a 30-day evaluation structure with milestones (baseline, mesh stability, production traffic, advanced scenarios), but this is for structured evaluation, not initial onboarding. The pilot framework tracks long-term metrics like mesh connectivity, peer discovery time, and throughput.\n\n2. **Documentation gap** — The evaluation checklist (`docs/evaluation-checklist.md`) and pilot guide (`docs/pilot-evaluation-guide.md`) exist as static documentation, but there is no interactive, tracked onboarding experience embedded in any UI or dashboard.\n\n3. **No TTFM tracking** — There is no mechanism to measure or display Time-to-First-Mesh: the elapsed time from running `wgmesh join` to achieving the first successful peer connection and bidirectional connectivity.\n\n4. **Dashboard analysis** — The project has `wgmesh.dev/index.html` (marketing site with pricing tiers) but no operational dashboard for users to track their onboarding progress or mesh health.\n\n5. **Missing infrastructure** — There is no:\n   - HTTP server serving a web dashboard\n   - JSON API exposing onboarding state\n   - Frontend rendering checklist progress\n   - Analytics tracking completion rates\n\n**User impact:**\n\n- New users cannot quickly see \"what do I do next?\" after running `wgmesh join`\n- Operators have no visibility into onboarding completion rates across their fleet\n- Support teams cannot identify where users struggle (e.g., NAT traversal failures, discovery timeouts)\n- No built-in validation that the mesh is actually working end-to-end\n\n## Proposed Approach\n\nCreate a **Time-to-First-Mesh onboarding checklist** embedded in a new web dashboard, tracking completion rate from initial deployment to first successful mesh connectivity. The approach extends the existing pilot framework with an \"onboarding mode\" that focuses on the critical path to first mesh, rather than long-term evaluation.\n\n### Architecture Overview\n\n```\n┌─────────────────────────────────────────────────────────────────────┐\n│                        wgmesh daemon                                │\n│  ┌──────────────────────────────────────────────────────────────┐  │\n│  │              Onboarding Tracker (pkg/onboarding/)            │  │\n│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │  │\n│  │  │ Checklist    │  │ TTFM Timer   │  │ Validator    │      │  │\n│  │  │ State Machine│  │ (first mesh) │  │ (healthchecks)│     │  │\n│  │  └──────────────┘  └──────────────┘  └──────────────┘      │  │\n│  └──────────────────────────────────────────────────────────────┘  │\n│                              │                                       │\n│                         HTTP API                                     │\n│                              │                                       │\n└──────────────────────────────┼───────────────────────────────────────┘\n                               │\n                    ┌──────────▼──────────┐\n                    │   HTTP Dashboard    │\n                    │  (embedded in       │\n                    │   wgmesh daemon)   │\n                    └────────────────────┘\n                               │\n                    ┌──────────▼──────────┐\n                    │  Web UI (static)    │\n                    │  /onboarding HTML/JS │\n                    └─────────────────────┘\n```\n\n### Design Principles\n\n1. **Embedded in daemon** — No separate dashboard service. The HTTP server runs alongside the daemon (similar to existing pprof/metrics servers).\n2. **Zero external deps** — No database. State stored in `/var/lib/wgmesh/onboarding-state.json` (similar to pilot state).\n3. **Backward compatible** — Existing workflows unchanged. Onboarding tracking is opt-in via `--dashboard` flag.\n4. **Privacy-first** — No data sent externally. Completion rate is local-only. No telemetry.\n5. **Reusability** — Validator functions shared between CLI (`wgmesh validate`) and dashboard API.\n\n### Data Model\n\n**Onboarding checklist tasks** (5 tasks total):\n\n```go\ntype ChecklistTask struct {\n    ID          string    `json:\"id\"`           // \"install\", \"secret\", \"join\", \"peers\", \"connectivity\"\n    Name        string    `json:\"name\"`         // Display name\n    Description string    `json:\"description\"`  // Help text\n    Status      string    `json:\"status\"`       // \"pending\", \"in_progress\", \"complete\", \"failed\"\n    StartedAt   time.Time `json:\"started_at,omitempty\"`\n    CompletedAt time.Time `json:\"completed_at,omitempty\"`\n    ErrorMsg    string    `json:\"error_msg,omitempty\"`\n}\n```\n\n**Onboarding state:**\n\n```go\ntype OnboardingState struct {\n    StartedAt        time.Time        `json:\"started_at\"`\n    CompletedAt      time.Time        `json:\"completed_at,omitempty\"`\n    TTFMSeconds      int              `json:\"ttfm_seconds,omitempty\"` // Time to first mesh\n    CompletionRate   float64          `json:\"completion_rate\"`         // 0.0 to 1.0\n    Tasks            []*ChecklistTask `json:\"tasks\"`\n    CurrentPhase     string           `json:\"current_phase\"`          // \"setup\", \"discovery\", \"validation\"\n    MeshID           string           `json:\"mesh_id\"`                 // Derived from secret\n}\n```\n\n### Five-Task Onboarding Checklist\n\n| Task ID | Name | Validation | Success Criteria |\n|---------|------|------------|------------------|\n| `install` | Verify installation | `wg` command available, kernel ≥5.6 | WireGuard tools installed |\n| `secret` | Generate or receive secret | Secret parsed, network ID derived | Valid `wgmesh://v1/...` URI |\n| `join` | Join mesh | Daemon running, interface created | `wg0` interface exists |\n| `peers` | Discover first peer | Peer appears in peer store | `wgmesh peers count ≥ 1` |\n| `connectivity` | Verify connectivity | Ping mesh IP succeeds | `ping <mesh-ip>` succeeds |\n\n### HTTP API Endpoints\n\n**Dashboard server** runs on port 8080 (configurable via `--dashboard-port`).\n\n```\nGET  /api/onboarding/state      → Returns OnboardingState as JSON\nPOST /api/onboarding/validate   → Runs validation, returns updated state\nGET  /api/onboarding/ttfm       → Returns current TTFM (seconds)\nGET  /                          → Serves static onboarding dashboard HTML\n```\n\n**API response example:**\n\n```json\n{\n  \"started_at\": \"2026-06-19T09:00:00Z\",\n  \"completed_at\": \"2026-06-19T09:05:23Z\",\n  \"ttfm_seconds\": 323,\n  \"completion_rate\": 1.0,\n  \"current_phase\": \"complete\",\n  \"mesh_id\": \"orgname-20260619\",\n  \"tasks\": [\n    {\n      \"id\": \"install\",\n      \"name\": \"Verify installation\",\n      \"status\": \"complete\",\n      \"completed_at\": \"2026-06-19T09:00:15Z\"\n    },\n    {\n      \"id\": \"secret\",\n      \"name\": \"Generate or receive secret\",\n      \"status\": \"complete\",\n      \"completed_at\": \"2026-06-19T09:01:00Z\"\n    },\n    {\n      \"id\": \"join\",\n      \"name\": \"Join mesh\",\n      \"status\": \"complete\",\n      \"completed_at\": \"2026-06-19T09:02:30Z\"\n    },\n    {\n      \"id\": \"peers\",\n      \"name\": \"Discover first peer\",\n      \"status\": \"complete\",\n      \"completed_at\": \"2026-06-19T09:04:00Z\"\n    },\n    {\n      \"id\": \"connectivity\",\n      \"name\": \"Verify connectivity\",\n      \"status\": \"complete\",\n      \"completed_at\": \"2026-06-19T09:05:23Z\"\n    }\n  ]\n}\n```\n\n### Static Dashboard UI\n\n**File:** `pkg/onboarding/dashboard.html` (embedded static HTML served at root)\n\nThe dashboard provides:\n\n- **Progress bar** showing completion rate (0-100%)\n- **Checklist visualization** with task status icons (✓ complete, ⟳ in progress, ✗ failed, ⏸ pending)\n- **TTFM timer** showing elapsed time since start\n- **\"Validate Now\" button** that POSTs to `/api/onboarding/validate`\n- **Phase indicator** (Setup → Discovery → Validation → Complete)\n- **Error messages** for failed tasks with troubleshooting guidance\n\n**Design constraints:**\n\n- Single HTML file with embedded CSS and vanilla JavaScript\n- No external dependencies (no CDNs, no frameworks)\n- Responsive design (mobile-friendly)\n- Auto-refresh every 5 seconds when in \"in_progress\" state\n- Dark mode support (prefers-color-scheme)\n\n### Integration with Existing Code\n\n**Reuses existing components:**\n\n- `pkg/daemon/peerstore.go` — Check peer count for \"peers\" task\n- `pkg/crypto/derive.go` — Derive mesh ID from secret for \"secret\" task\n- `pkg/wireguard/config.go` — Verify interface exists for \"join\" task\n- `pkg/daemon/health_test.go` pattern — Validation logic similar to health checks\n\n**New package structure:**\n\n```\npkg/onboarding/\n├── onboarding.go          // OnboardingState struct, state machine\n├── onboarding_test.go     // Tests\n├── validator.go           // Task validation functions\n├── validator_test.go      // Tests\n├── dashboard.go           // HTTP server, API handlers\n├── dashboard_test.go      // Tests\n├── dashboard.html         // Static UI (embedded in binary)\n└── state.go               // Persistence to /var/lib/wgmesh/onboarding-state.json\n```\n\n### CLI Integration\n\nAdd new subcommands:\n\n```bash\nwgmesh onboarding start          # Initialize onboarding tracker\nwgmesh onboarding status         # Show checklist progress (console)\nwgmesh onboarding validate       # Run validation tasks\nwgmesh onboarding complete       # Mark onboarding complete, show TTFM\n```\n\n**Optional flags for daemon:**\n\n```bash\nwgmesh join --secret \"...\" --dashboard        # Enable dashboard on :8080\nwgmesh join --secret \"...\" --dashboard-port 9090\n```\n\n## Implementation Tasks\n\n### Task 1: Create onboarding state machine and data model\n\n**File:** `pkg/onboarding/onboarding.go` (create)\n\n**What:** Define `OnboardingState` struct and state transition logic.\n\n**Detail:**\n- Export `OnboardingState`, `ChecklistTask` structs with JSON tags\n- Implement `NewOnboardingState(meshID string) *OnboardingState`\n- Implement `Start()` to set `StartedAt = time.Now()` and initialize tasks\n- Implement `Complete()` to set `CompletedAt` and calculate final `TTFMSeconds`\n- Implement `CompletionRate() float64` to return fraction of tasks with status \"complete\"\n- Implement `CurrentPhase() string` based on task statuses:\n  - \"setup\": tasks 0-1 in progress\n  - \"discovery\": tasks 2-3 in progress\n  - \"validation\": task 4 in progress\n  - \"complete\": all tasks complete\n- Implement mutex-protected setters/getters for thread safety\n\n**Tests:** `pkg/onboarding/onboarding_test.go`\n- Test `NewOnboardingState` initializes all 5 tasks with \"pending\" status\n- Test `Start()` sets `StartedAt` and moves first task to \"in_progress\"\n- Test `CompletionRate()` returns 0.0 for no tasks, 0.2 for 1/5 tasks, 1.0 for all tasks\n- Test `CurrentPhase()` returns correct phase for each task progression\n\n### Task 2: Create task validation functions\n\n**File:** `pkg/onboarding/validator.go` (create)\n\n**What:** Implement validation logic for each checklist task.\n\n**Detail:**\n- Export `ValidateTask(taskID string, meshID string) error` function\n- Task-specific validation:\n  - `install`: Check `wg` command exists via `exec.LookPath`, check kernel version via `uname -r`\n  - `secret`: Validate `meshID` is non-empty (derivation already happened by the time validator runs)\n  - `join`: Check `wg0` interface exists via `net.InterfaceByName(\"wg0\")`\n  - `peers`: Query peer store via daemon RPC, check peer count ≥ 1\n  - `connectivity`: Ping first peer's mesh IP via `exec.Command(\"ping\", \"-c\", \"1\", \"-W\", \"2\", meshIP)`\n- Return descriptive error messages for each failure case (e.g., \"WireGuard tools not installed: exec: wg: not found\")\n- Use context with timeout for ping (2 second timeout)\n\n**Tests:** `pkg/onboarding/validator_test.go`\n- Mock `exec.Command` for `wg` and `ping` commands\n- Test successful validation for each task\n- Test failure paths with mocked errors\n- Test timeout handling in ping validator\n\n### Task 3: Create state persistence\n\n**File:** `pkg/onboarding/state.go` (create)\n\n**What:** Save/load onboarding state to/from disk.\n\n**Detail:**\n- Implement `Save(state *OnboardingState, path string) error` using `os.WriteFile` with JSON encoding\n- Implement `Load(path string) (*OnboardingState, error)` with proper error handling\n- Use path `/var/lib/wgmesh/onboarding-state.json` by default\n- Ensure atomic writes via temp file + rename pattern\n- Handle missing file gracefully (return \"not initialized\" error, not panic)\n\n**Tests:** `pkg/onboarding/state_test.go`\n- Test `Save` creates file with valid JSON\n- Test `Load` parses saved state correctly\n- Test `Load` returns error for non-existent file\n- Test atomic write pattern (temp file created then renamed)\n\n### Task 4: Create HTTP dashboard server and API\n\n**File:** `pkg/onboarding/dashboard.go` (create)\n\n**What:** Implement HTTP server with API endpoints and static UI serving.\n\n**Detail:**\n- Export `Dashboard` struct with fields: `state *OnboardingState`, `statePath string`, `port int`\n- Implement `NewDashboard(statePath string, port int) *Dashboard`\n- Implement `Start() error` to start HTTP server on specified port\n- HTTP handlers:\n  - `handleState()`: GET `/api/onboarding/state` → return JSON of current state\n  - `handleValidate()`: POST `/api/onboarding/validate` → run all pending/in_progress task validations, update state, save to disk, return updated state\n  - `handleTTFM()`: GET `/api/onboarding/ttfm` → return JSON `{\"ttfm_seconds\": int}`\n  - `handleIndex()`: GET `/` → serve `dashboard.html` with content-type `text/html`\n- CORS headers for local development (optional, allow origin `*`)\n- Graceful shutdown on SIGTERM\n- Access logging to stdout\n\n**Tests:** `pkg/onboarding/dashboard_test.go`\n- Test `NewDashboard` initializes with correct port\n- Test HTTP handlers using `httptest.NewServer`\n- Test `handleState` returns valid JSON with all fields\n- Test `handleValidate` updates task statuses and saves state\n- Test `handleTTFM` returns correct seconds\n- Test `handleIndex` serves HTML with correct content-type\n\n### Task 5: Create static dashboard UI\n\n**File:** `pkg/onboarding/dashboard.html` (create)\n\n**What:** Create embedded static HTML/JS dashboard UI.\n\n**Detail:**\n- Single HTML5 document with embedded CSS in `<style>` and JS in `<script>`\n- CSS: Dark mode via `@media (prefers-color-scheme: dark)`, responsive flexbox layout\n- JavaScript: `fetch('/api/onboarding/state')` on load, render checklist, auto-refresh every 5s if any task is \"in_progress\"\n- Progress bar: `<progress value=\"{completion_rate} max=\"1\">`\n- Task icons: Use Unicode characters (✓, ⟳, ✗, ⏸) based on status\n- Validate button: `<button onclick=\"validate()\">Validate Now</button>` that POSTs to `/api/onboarding/validate`\n- Phase indicator: Display `current_phase` with color coding (setup=blue, discovery=yellow, validation=orange, complete=green)\n- TTFM display: Show elapsed time in MM:SS format if not complete, show final TTFM if complete\n- Error display: Show `error_msg` for failed tasks in red with troubleshooting suggestions\n- No external dependencies (no CDNs, no fonts other than system fonts)\n\n**Tests:** Manual testing via browser (no automated tests for HTML)\n\n### Task 6: Integrate dashboard into daemon CLI\n\n**File:** `main.go` (modify)\n\n**What:** Add flags to enable dashboard and start HTTP server alongside daemon.\n\n**Detail:**\n- Add flags to `joinCmd`:\n  - `--dashboard`: bool flag to enable onboarding dashboard\n  - `--dashboard-port`: int flag (default 8080) for dashboard port\n- When `--dashboard` is true:\n  - Create onboarding state before joining mesh\n  - Start dashboard HTTP server in goroutine after daemon starts\n  - Log \"Onboarding dashboard available at http://localhost:{port}\"\n- Ensure dashboard goroutine exits cleanly on daemon shutdown\n\n**Tests:** Integration test with `wgmesh join --dashboard --help` to verify flags are present\n\n### Task 7: Add onboarding CLI subcommands\n\n**File:** `main.go` (modify)\n\n**What:** Implement `onboarding` subcommand group.\n\n**Detail:**\n- Add `onboardingCmd` with subcommands: `start`, `status`, `validate`, `complete`\n- `wgmesh onboarding start`: Initialize onboarding state, save to disk, print \"Started onboarding tracker\"\n- `wgmesh onboarding status`: Load state, print table of tasks with status, show completion rate and TTFM\n- `wgmesh onboarding validate`: Run validator for all pending/in_progress tasks, update state, save, print results\n- `wgmesh onboarding complete`: Mark onboarding complete, calculate TTFM, print summary\n- Use `tabwriter` for formatted console output (similar to `wgmesh peers list`)\n\n**Tests:** Manual testing of CLI commands\n\n### Task 8: Add metrics for completion rate tracking\n\n**A tool call could not be parsed — the response may have been truncated. Try breaking the task into smaller steps or resending your message.
