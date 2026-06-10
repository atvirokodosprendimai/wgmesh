# Issue #651: Build 30-day wgmesh pilot evaluation framework for network administrators

## Classification
feature

## Problem Analysis

Network administrators evaluating wgmesh lack a structured framework to assess the product's capabilities within their environment. Current evaluation approaches are ad-hoc, leading to:

1. **Incomplete testing**: Administrators may not exercise all discovery layers, NAT traversal scenarios, or operational modes (centralized vs decentralized)
2. **Unclear success criteria**: No measurable benchmarks for mesh stability, throughput, or operational readiness
3. **Poor documentation of findings**: Evaluation results are not systematically captured for decision-making
4. **Lengthy time-to-value**: Without guided milestones, evaluations drag on inconclusively

A 30-day pilot framework provides structure for both the evaluator (network administrator) and the wgmesh team (to gather feedback, improve product-market fit, and convert evaluators to customers).

## Proposed Approach

### 1. Pilot Evaluation Package (Day 0)

Create a reusable pilot package that administrators can deploy:

- **`pkg/pilot/` package**: Core pilot framework logic
  - `pilot.go`: Pilot state machine, configuration, milestone tracking
  - `metrics.go`: Collection and export of pilot-specific metrics
  - `report.go`: Report generation (plaintext, JSON, HTML)
  - `validation.go`: Health checks and validation tests
  
- **CLI commands** (extend `main.go`):
  ```bash
  wgmesh pilot init --org "Acme Corp" --contact admin@acme.com
  wgmesh pilot start              # Start 30-day pilot clock
  wgmesh pilot status            # Show current phase, milestones, days elapsed
  wgmesh pilot report            # Generate evaluation report
  wgmesh pilot complete          # Finalize pilot, generate summary
  ```

- **Pilot configuration file** (`/etc/wgmesh/pilot.yaml`):
  ```yaml
  pilot_id: "acme-corp-20250610"
  organization: "Acme Corp"
  contact_email: "admin@acme.com"
  start_date: "2025-06-10T00:00:00Z"
  end_date: "2025-07-10T00:00:00Z"
  mode: "decentralized"  # or "centralized"
  node_count: 5
  milestones:
    baseline:
      completed: true
      completed_at: "2025-06-12T15:30:00Z"
    mesh_stability:
      completed: false
      target_date: "2025-06-17T00:00:00Z"
    production_traffic:
      completed: false
      target_date: "2025-06-24T00:00:00Z"
    nat_traversal:
      completed: false
      target_date: "2025-07-01T00:00:00Z"
  metrics_targets:
    mesh_connectivity: 99.9   # percent
    peer_discovery_time: 60  # seconds
    route_propagation: 30    # seconds
  ```

### 2. Four-Phase Milestone Structure

Design the pilot around progressive milestones that exercise wgmesh capabilities:

#### Phase 1: Baseline Setup (Days 1-3)
- **Goal**: Successful deployment and basic connectivity
- **Tasks**:
  - Install wgmesh on pilot nodes
  - Configure mesh secret and join mesh
  - Verify peer discovery across all nodes
  - Execute `wgmesh pilot status` to confirm Phase 1 completion
- **Validation**:
  - All peers visible in `wgmesh status`
  - Ping/HTTP test between all peer pairs
  - No interface churn (WireGuard restart loops)

#### Phase 2: Mesh Stability (Days 4-7)
- **Goal**: Verify mesh stability under normal operations
- **Tasks**:
  - Run continuous connectivity tests (24h soak)
  - Verify route propagation after network changes
  - Test graceful node restart and reconnection
  - Log key metrics: connection uptime, discovery success rate
- **Validation**:
  - ≥99.9% connectivity uptime between all nodes
  - All routes propagate within 30 seconds of topology change
  - Zero daemon crashes or WireGuard interface crashes
  - NAT type detection completed for all nodes

#### Phase 3: Production Traffic Simulation (Days 8-14)
- **Goal**: Validate under realistic workload
- **Tasks**:
  - Route application traffic through mesh
  - Measure throughput and latency
  - Test with intermittent network failures (simulated outages)
  - Exercise all discovery layers (registry, LAN, DHT, gossip)
- **Validation**:
  - Throughput ≥80% of native WireGuard baseline
  - Latency增加 <20ms compared to native WireGuard
  - Successful recovery from simulated network partitions
  - All discovery layers successfully used

#### Phase 4: Advanced Scenarios & NAT Traversal (Days 15-30)
- **Goal**: Stress-test edge cases and operational workflows
- **Tasks**:
  - Deploy nodes behind diverse NAT types (Full Cone, Symmetric, etc.)
  - Test relay fallback when direct connection fails
  - Verify peer rotation and secret rotation workflows
  - Exercise centralized mode features (if applicable)
  - Test operational procedures: daemon restart, config reload, node add/remove
- **Validation**:
  - Successful hole-punching across all NAT type combinations
  - Relay fallback engages within 60 seconds of direct path failure
  - Zero secret leaks or key derivation failures
  - Clean node addition/removal with no orphaned WireGuard configs

### 3. Metrics Collection and Reporting

Extend `pkg/daemon/metrics.go` to collect pilot-specific metrics:

```go
// PilotMetrics tracks evaluation-specific measurements
type PilotMetrics struct {
    PilotID              string
    Phase                string
    DaysElapsed          int
    
    // Connectivity metrics
    MeshUptimePercent    float64
    PeerDiscoverySuccess float64
    RoutePropagationTime time.Duration
    
    // Performance metrics
    ThroughputMbps       float64
    LatencyMs            float64
    ConnectionOverheadMs float64
    
    // Reliability metrics
    DaemonRestarts       int
    WireGuardRestarts    int
    NetworkPartitions    int
    RecoveryTimeSec      int
    
    // Discovery layer usage
    DiscoveryLayerCounts map[string]int
    
    // NAT traversal
    NATTypes             map[string]int
    HolePunchSuccess    float64
    RelayFallbackCount  int
}
```

Generate reports in multiple formats:

- **Console output** (default for `wgmesh pilot report`):
  ```
  wgmesh Pilot Report: acme-corp-20250610
  ==================================================
  
  Phase: Mesh Stability (Day 5 of 30)
  Organization: Acme Corp | Contact: admin@acme.com
  
  MILESTONE STATUS
  ✓ Baseline Setup (completed Day 3)
  ○ Mesh Stability (target: Day 7) — IN PROGRESS
  ⬜ Production Traffic (target: Day 14)
  ⬜ Advanced Scenarios (target: Day 30)
  
  KEY METRICS (last 24h)
  ----------------------------
  Mesh Connectivity: 99.97% (target: ≥99.9%)
  Peer Discovery: 100% (5/5 peers discovered)
  Route Propagation: 12s avg (target: ≤30s)
  Daemon Restarts: 0
  WireGuard Restarts: 0
  
  ISSUES / WARNINGS
  ----------------------------
  [WARN] Node node-03 detected behind Symmetric NAT — expect relay fallback
  [INFO] Discovery layer distribution: Registry=5, LAN=3, DHT=2, Gossip=0
  
  NEXT STEPS
  ----------------------------
  → Continue 24h soak test through Day 7
  → If no connectivity drops >0.1%, proceed to Phase 3
  → Document NAT traversal results for node-03
  ```

- **JSON export** (for automated analysis):
  ```bash
  wgmesh pilot report --format json > pilot-report-day05.json
  ```

- **HTML report** (for executive summary):
  ```bash
  wgmesh pilot report --format html > pilot-report-final.html
  ```

### 4. Pilot CLI Implementation

#### `main.go` additions

Add pilot subcommand:

```go
var cmdPilot = &cli.Command{
    Name:  "pilot",
    Usage: "Pilot evaluation management",
    Subcommands: []*cli.Command{
        {
            Name:  "init",
            Usage: "Initialize a new pilot evaluation",
            Flags: []cli.Flag{
                &cli.StringFlag{
                    Name:     "org",
                    Usage:    "Organization name",
                    Required: true,
                },
                &cli.StringFlag{
                    Name:     "contact",
                    Usage:    "Contact email",
                    Required: true,
                },
                &cli.IntFlag{
                    Name:  "nodes",
                    Usage: "Expected number of nodes",
                    Value: 3,
                },
                &cli.StringFlag{
                    Name:  "mode",
                    Usage: "Operational mode",
                    Value: "decentralized",
                },
                &cli.IntFlag{
                    Name:  "duration",
                    Usage: "Pilot duration in days",
                    Value: 30,
                },
            },
            Action: runPilotInit,
        },
        {
            Name:   "start",
            Usage:  "Start the pilot evaluation",
            Action: runPilotStart,
        },
        {
            Name:   "status",
            Usage:  "Show pilot status and progress",
            Action: runPilotStatus,
        },
        {
            Name:  "report",
            Usage: "Generate pilot evaluation report",
            Flags: []cli.Flag{
                &cli.StringFlag{
                    Name:  "format",
                    Usage: "Report format: console, json, html",
                    Value: "console",
                },
                &cli.StringFlag{
                    Name:  "output",
                    Usage: "Output file path",
                },
            },
            Action: runPilotReport,
        },
        {
            Name:   "complete",
            Usage:  "Finalize pilot and generate summary",
            Action: runPilotComplete,
        },
        {
            Name:   "validate",
            Usage:  "Run pilot validation checks",
            Action: runPilotValidate,
        },
    },
}
```

#### Implementation functions (`pkg/pilot/`)

```go
// Initialize creates a new pilot configuration
func Initialize(org, contact string, nodeCount int, mode string, durationDays int) error

// Start begins the pilot evaluation
func Start() error

// Status returns the current pilot state
func Status() (*PilotState, error)

// GenerateReport produces evaluation reports in specified format
func GenerateReport(format, outputPath string) error

// Complete finalizes the pilot and generates final summary
func Complete() (*FinalReport, error)

// Validate runs health checks and returns validation results
func Validate() (*ValidationResult, error)
```

### 5. Integration with Existing Components

#### RPC server extension (`pkg/rpc/server.go`)

Add RPC methods for remote pilot monitoring:

```go
type PilotService struct{}

func (s *PilotService) Status(ctx context.Context, req *Empty) (*PilotStatus, error)
func (s *PilotService) Report(ctx context.Context, req *ReportRequest) (*Report, error)
func (s *PilotService) Validate(ctx context.Context, req *Empty) (*ValidationResult, error)
```

#### Daemon integration (`pkg/daemon/daemon.go`)

Pilot metrics collection should run alongside existing daemon metrics:

```go
// In daemon.go main loop
select {
case <-tick.C:
    d.collectMetrics()
    if d.pilot != nil {
        d.pilot.CollectMetrics(d.peerStore, d.relay)
    }
case <-d.done:
    return
}
```

### 6. Documentation

Create pilot evaluation guide (`docs/pilot-evaluation-guide.md`):

- Prerequisites and system requirements
- Step-by-step deployment walkthrough
- Milestone validation checklist
- Troubleshooting common issues
- FAQ for pilot evaluators

Add evaluation checklist to existing docs (`docs/evaluation-checklist.md` enhancement):

- Pre-evaluation readiness checklist
- Environment validation script
- Success criteria template

## Acceptance Criteria

### Core Functionality
- [ ] `wgmesh pilot init` creates valid pilot configuration
- [ ] `wgmesh pilot start` initializes pilot state and begins day tracking
- [ ] `wgmesh pilot status` displays current phase, milestones, days elapsed
- [ ] `wgmesh pilot report` generates console, JSON, and HTML reports
- [ ] `wgmesh pilot complete` produces final summary with all metrics
- [ ] `wgmesh pilot validate` executes health checks and returns results

### Milestone Tracking
- [ ] Baseline Setup milestone auto-validates on successful peer discovery
- [ ] Mesh Stability milestone requires 24h of ≥99.9% connectivity
- [ ] Production Traffic milestone records throughput/latency metrics
- [ ] Advanced Scenarios milestone validates NAT traversal success

### Metrics and Reporting
- [ ] Pilot metrics collected alongside daemon metrics without interference
- [ ] Console report shows milestone status, key metrics, issues, next steps
- [ ] JSON export contains complete metrics dataset for analysis
- [ ] HTML report renders charts and executive summary
- [ ] Reports include comparison against targets (connectivity, latency, etc.)

### Integration
- [ ] Pilot state persisted to `/etc/wgmesh/pilot.yaml`
- [ ] Pilot CLI commands work in both centralized and decentralized modes
- [ ] RPC service exposes pilot status and report generation
- [ ] Pilot metrics exported via existing Prometheus endpoint

### Documentation
- [ ] Pilot evaluation guide covers all four phases and validation steps
- [ ] Pre-evaluation checklist prevents environment issues
- [ ] Troubleshooting section addresses common failure modes
- [ ] Success criteria template provided for customization

### Testing
- [ ] Unit tests for pilot state machine and milestone logic
- [ ] Integration test for full 30-day pilot simulation (accelerated time)
- [ ] Validation that pilot metrics match daemon metrics where overlapping
- [ ] Report generation tests for all three formats

## Out of Scope

- **Automated remediation**: The pilot framework detects and reports issues but does not automatically fix them
- **Custom milestone definitions**: Initial version supports only the four predefined phases
- **Multi-org dashboards**: No centralized dashboard for monitoring multiple simultaneous pilots
- **SLA guarantee**: Pilot framework provides evaluation tools, not production SLAs
- **Billing/integration**: No payment processing or customer portal integration
- **Advanced analytics**: Statistical analysis, anomaly detection, or ML-based insights
- **Mobile apps**: CLI-based only, no iOS/Android pilot monitoring apps
- **Secret management**: Does not replace existing wgmesh secret handling, only tracks usage

### Future Enhancements (Post-v1)
- Web-based pilot dashboard
- Custom milestone templates
- Integration with customer support ticketing
- Automated success/failure classification
- Comparative analysis across pilot cohorts
