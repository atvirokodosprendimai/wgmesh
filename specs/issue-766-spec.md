# Specification: Issue #766 - Build trial expiration paywall: upgrade modal + mesh pause on day 14

## Classification
feature

## Problem Analysis

The wgmesh product currently has no trial expiration enforcement mechanism. New users can run the mesh indefinitely without upgrade prompts or functional limitations. This specification addresses the business requirement to implement a 14-day trial period with:

1. **Upgrade prompt modal**: Display an interactive upgrade prompt when the trial expires (day 14+), with a clear call-to-action to upgrade.
2. **Mesh pause on expiration**: After day 14, the mesh daemon should pause operations (stop peer discovery, stop health probes, stop WireGuard configuration updates) until the user upgrades or dismisses the prompt.
3. **Graceful UX**: Users should be able to dismiss the prompt temporarily to continue using the mesh for a short grace period (e.g., 24 hours) before the pause takes effect.

### Current State

- No trial tracking exists in the codebase.
- No upgrade prompt UI or modal mechanism.
- No pause/resume functionality in the daemon.
- Metrics exist in `pkg/daemon/metrics.go` but no trial-related metrics.
- Account configuration exists in `pkg/mesh/account.go` for Lighthouse API integration.
- Daemon has `startTime` field for uptime tracking (`pkg/daemon/daemon.go` line 84).
- Config system supports hot-reload via `LoadReloadFile` (`pkg/daemon/config.go`).

### Technical Considerations

- **Trial start time**: Should be stored persistently in the state directory (`/var/lib/wgmesh/trial.json` by default).
- **Account linkage**: Trial status should be tied to the Lighthouse account (if configured) or the mesh ID for anonymous trials.
- **Daemon pause**: Must be graceful—existing WireGuard connections should remain active, but new peer discovery and health checks should stop.
- **Upgrade detection**: Need a mechanism to check if the user has upgraded (via Lighthouse API or local license file).
- **Metrics**: Add Prometheus metrics for trial status, days remaining, and upgrade prompt displays.
- **CLI integration**: The `status` command should show trial status and days remaining.

## Proposed Approach

### Phase 1: Trial State Management

Create a new package `pkg/trial` to handle trial lifecycle, state persistence, and upgrade checks.

**New files:**
- `pkg/trial/trial.go` - Core trial logic, state management, expiration checking
- `pkg/trial/trial_test.go` - Unit tests for trial calculations
- `pkg/trial/store.go` - Persistent storage of trial start time and status
- `pkg/trial/store_test.go` - Unit tests for storage operations

**Trial state structure:**
```go
type TrialState struct {
    MeshID       string    `json:"mesh_id"`
    StartedAt    time.Time `json:"started_at"`
    ExpiresAt    time.Time `json:"expires_at"`
    Status       string    `json:"status"` // "active", "expired", "upgraded", "dismissed"
    DismissedAt  *time.Time `json:"dismissed_at,omitempty"`
    UpgradedAt   *time.Time `json:"upgraded_at,omitempty"`
    GraceUntil   *time.Time `json:"grace_until,omitempty"` // Temporary grace period after dismissal
}
```

**Key functions:**
- `StartTrial(meshID string) (*TrialState, error)` - Initialize new trial (14 days from now)
- `LoadTrial(meshID string) (*TrialState, error)` - Load existing trial state
- `CheckExpired(state *TrialState) bool` - Check if trial has expired
- `DaysRemaining(state *TrialState) int` - Calculate days until expiration
- `Dismiss(state *TrialState, graceDuration time.Duration) error` - Dismiss upgrade prompt with grace period
- `Upgrade(state *TrialState) error` - Mark trial as upgraded

### Phase 2: Daemon Integration

Integrate trial checking into the daemon's main reconciliation loop.

**Modified files:**
- `pkg/daemon/daemon.go` - Add trial checking to reconcile loop, implement pause logic
- `pkg/daemon/config.go` - Add trial-related options to `DaemonOpts`
- `pkg/daemon/metrics.go` - Add trial metrics

**New daemon behavior:**

1. **On startup**: Load trial state, check if expired, log status
2. **During reconcile**: 
   - Check trial status every cycle
   - If expired and not dismissed: pause mesh operations
   - If in grace period: continue operations but log warning
3. **Pause mode**:
   - Stop DHT discovery queries
   - Stop LAN multicast discovery
   - Stop gossip propagation
   - Stop health probes
   - Stop WireGuard config updates
   - Keep existing WireGuard interface up (don't call `wg-quick down`)
4. **Resume mode**:
   - Resume all paused operations after upgrade or grace period

**New metrics:**
```go
var (
    trialStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "wgmesh_trial_status",
        Help: "Trial status (1=active, 2=expired, 3=upgraded, 4=grace)",
    }, []string{"status"})
    trialDaysRemaining = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "wgmesh_trial_days_remaining",
        Help: "Days remaining until trial expiration (negative if expired)",
    })
    trialUpgradePromptShown = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "wgmesh_trial_upgrade_prompt_shown_total",
        Help: "Number of times upgrade prompt was displayed",
    })
)
```

### Phase 3: CLI Integration

Update CLI commands to show trial status and handle upgrade prompts.

**Modified files:**
- `main.go` - Add trial status to `status` command, add `upgrade` command
- `service.go` - Integrate trial checks into service operations

**New CLI command: `wgmesh upgrade`**
```bash
wgmesh upgrade [--account <cr_...>] [--secret <SECRET>]
```
- Check trial status via Lighthouse API
- If upgraded: update local state, unpause daemon
- If not upgraded: show upgrade URL and pricing information

**Modified `status` command output:**
```
Mesh ID: abc123xyz
Status: Active (3 peers)
Trial: 8 days remaining
Upgrade: https://wgmesh.dev/upgrade?mesh=abc123xyz
```

**Expired trial output:**
```
Mesh ID: abc123xyz
Status: PAUSED - Trial expired 2 days ago
⚠️  Your trial has expired. Upgrade to continue using wgmesh.
🔗 https://wgmesh.dev/upgrade?mesh=abc123xyz
Dismiss (24h grace): wgmesh trial dismiss
```

**New CLI command: `wgmesh trial`**
```bash
wgmesh trial status         # Show trial status and days remaining
wgmesh trial dismiss        # Dismiss upgrade prompt (24h grace)
wgmesh trial reset          # Reset trial (for testing only)
```

### Phase 4: Upgrade Modal/Prompt

Create an interactive upgrade prompt that displays when the trial expires.

**New files:**
- `pkg/trial/prompt.go` - Upgrade prompt UI logic
- `pkg/trial/prompt_test.go` - Unit tests for prompt logic

**Prompt content:**
```
╔══════════════════════════════════════════════════════════════════╗
║                    ⚠️  TRIAL EXPIRED ⚠️                           ║
╠══════════════════════════════════════════════════════════════════╣
║ Your 14-day trial has expired. Mesh operations are paused.        ║
║                                                                  ║
║ Upgrade now to continue using wgmesh with unlimited peers:      ║
║   → https://wgmesh.dev/upgrade?mesh=abc123xyz                    ║
║                                                                  ║
║ [U]pgrade  [D]ismiss (24h)  [Q]uit                                ║
╚══════════════════════════════════════════════════════════════════╝
```

**Implementation:**
- Prompt appears on daemon startup if trial is expired
- Prompt appears on `status` command if trial is expired
- User can press U/D/Q to upgrade/dismiss/quit
- Dismiss option grants 24-hour grace period before next pause

### Phase 5: Lighthouse Integration

Add Lighthouse API calls to check account subscription status.

**Modified files:**
- `pkg/mesh/account.go` - Add subscription status check
- New function: `CheckSubscriptionStatus(apiKey, meshID string) (bool, error)`

**API contract:**
- Lighthouse API endpoint: `GET /api/v1/subscription`
- Response: `{"active": true, "plan": "pro"}`
- If active: mark trial as upgraded, unpause daemon

## Acceptance Criteria

1. **Trial initialization**: When a new mesh is created (`wgmesh init` or `wgmesh join`), a trial state file is created with start time and 14-day expiration.
2. **Days remaining calculation**: `DaysRemaining()` correctly calculates days until expiration, returning negative values if expired.
3. **Expiration detection**: `CheckExpired()` returns true when `time.Now() > ExpiresAt`.
4. **Daemon pause**: When trial expires and not dismissed:
   - DHT discovery stops (no more DHT queries)
   - LAN discovery stops (no more multicast)
   - Gossip stops (no more gossip propagation)
   - Health probes stop (no more mesh probes)
   - WireGuard config updates stop (existing connections remain)
6. **Metrics**: Trial status, days remaining, and upgrade prompt shown are exposed via Prometheus metrics.
7. **CLI status output**: `wgmesh status` shows trial status and days remaining.
8. **Upgrade prompt modal**: Interactive prompt appears on daemon startup when trial is expired.
9. **Dismiss grace period**: Dismissing the prompt grants a 24-hour grace period during which mesh continues operating.
10. **Upgrade detection**: Daemon checks Lighthouse API for subscription status; if upgraded, trial state is updated and mesh resumes.
11. **Persistence**: Trial state survives daemon restarts and system reboots.
12. **Test coverage**: All trial logic has unit tests with >80% coverage.

## Out of scope

- Payment processing (handled by external payment provider)
- License key generation/validation (assumes Lighthouse API handles this)
- Web UI or dashboard (CLI-only for this phase)
- Trial length customization (fixed at 14 days)
- Multi-node trial coordination (each node tracks its own trial)
- Trial extension mechanisms (no trial extensions in this phase)
- Analytics/telemetry for trial funnel optimization
- A/B testing of upgrade prompt copy or timing
