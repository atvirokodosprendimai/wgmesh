# Issue #734 - Implement time-to-first-mesh onboarding checklist to reduce trial drop-off

## Classification
feature

## Problem Analysis

**Current State**
Users deploying wgmesh for the first time face a complex sequence of manual steps across both centralized and decentralized modes. New users must:
- Understand discovery layer requirements (GitHub Issues, LAN multicast, DHT)
- Generate and distribute shared secrets properly
- Configure WireGuard interfaces manually
- Verify peer discovery is working
- Debug connectivity issues without clear guidance

**Pain Points**
- No guided workflow for initial deployment
- Users don't know when setup is "complete"
- Difficult to validate that each component is working
- No clear next steps when something fails
- Trial users likely abandon before seeing successful mesh formation

**User Impact**
- Increased trial drop-off due to setup complexity
- Longer time-to-value (TTV) for evaluating wgmesh
- Higher support burden for basic onboarding questions
- Poor first-run experience (FRE) reducing conversion

**Technical Gap**
The CLI currently provides raw commands (`join`, `init`, `status`) but no orchestrating wizard to guide users through validation steps. No checkpoint system exists to track progress.

## Proposed Approach

### Phase 1: Onboarding Checklist State Machine
Implement a progress tracking system that captures completion state in `~/.wgmesh/onboarding.json`:

**Checklist Items (ordered)**
1. **Secret Generation**: Shared secret created and meets entropy requirements
2. **Interface Config**: WireGuard interface configured with valid keys
3. **Discovery Layer 0**: GitHub Issues registry connectivity verified
4. **Discovery Layer 1**: LAN multicast listener bound and receiving
5. **Discovery Layer 2**: DHT bootstrap node reachable
6. **First Peer Contact**: At least one peer discovered via any layer
7. **Bidirectional Ping**: Successful WireGuard handshake with peer

### Phase 2: Interactive Onboarding Wizard
Add new `wgmesh onboard` subcommand that:

1. **Prompts for deployment mode**: centralized (SSH) vs decentralized (shared secret)
2. **Generates secrets securely**: Uses existing `pkg/crypto/derive.go` with entropy validation
3. **Guides WireGuard setup**: Calls `wg-genpair` and writes config to `/etc/wireguard/wg0.conf`
4. **Tests discovery layers sequentially**:
   - Pings GitHub API (L0)
   - Binds UDP multicast socket (L1)
   - Contacts DHT bootstrap (L2)
5. **Waits for first peer discovery**: Listens on discovery exchange with 2-minute timeout
6. **Validates handshake**: Uses `wg show wg0 latest-handshakes` to confirm
7. **Writes checkpoint to onboarding.json** after each successful step

### Phase 3: Status Dashboard Integration
Extend `wgmesh status` to show onboarding progress when incomplete:
```
Onboarding Progress: [===--] 3/7 steps complete
  ✓ Secret generated
  ✓ Interface configured
  ✗ GitHub registry unreachable (check network/firewall)
  ⏳ LAN multicast binding
```

### Phase 4: Self-Healing Suggestions
When steps fail, provide contextual remediation:
- L0 fail: "GitHub API unreachable. Check proxy settings or try --skip-registry"
- L1 fail: "Multicast bind failed. Verify interface permissions or use --discovery=dht-only"
- L2 fail: "DHT bootstrap timeout. Check outbound UDP to router.bittorrent.com:6881"

### Implementation Files
- `cmd/onboarding.go`: New wizard subcommand and state machine
- `pkg/onboarding/checklist.go`: Checklist item definitions and validation
- `pkg/onboarding/store.go`: JSON persistence of progress state
- `pkg/onboarding/validation.go`: Health checks for each layer
- `main.go`: Add `onboard` subcommand registration

### Code Structure
```go
// pkg/onboarding/checklist.go
type ChecklistItem struct {
    ID          string
    Name        string
    Description string
    Validate    func() error
    Remediate   string // Fallback guidance
}

// pkg/onboarding/store.go
type OnboardingState struct {
    CompletedItems []string
    CurrentStep    string
    StartedAt      time.Time
    LastUpdated    time.Time
}
```

### Existing Dependencies to Reuse
- `pkg/crypto/derive.go`: Secret generation and validation
- `pkg/discovery/*`: Layer-specific health checks
- `pkg/wireguard/`: Interface configuration parsing
- `golang.org/x/term`: Secure password input for secrets

## Acceptance Criteria

### Functional Requirements
1. **Onboarding wizard executes sequentially**: Each step must complete successfully before proceeding
2. **Progress persists across runs**: State survives process restart via `onboarding.json`
3. **Discovery layer validation**: Each layer (L0-L2) has a specific connectivity test with clear pass/fail
4. **Remediation guidance**: Every failure mode outputs actionable next steps
5. **Status integration**: `wgmesh status` shows incomplete checklist with visual progress bar
6. **Manual override**: Flag to skip completed items (`--skip-to=step-id`) for debugging
7. **Deletion capability**: `wgmesh onboard --reset` clears progress state

### Validation Tests
```go
func TestOnboardingChecklist_SequentialCompletion(t *testing.T)
func TestOnboardingStore_PersistenceAcrossRuns(t *testing.T)
func TestOnboardingValidation_GitHubRegistry(t *testing.T)
func TestOnboardingValidation_LANMulticast(t *testing.T)
func TestOnboardingValidation_DHTBootstrap(t *testing.T)
func TestOnboardingValidation_FirstPeerDiscovery(t *testing.T)
func TestOnboardingWizard_AbortPartialProgress(t *testing.T)
func TestStatusIntegration_ShowsIncompleteSteps(t *testing.T)
```

### Success Metrics
- Onboarding checklist script completes end-to-end in <3 minutes on standard network
- First peer handshake achieved within 5 minutes of starting wizard
- Zero manual file editing required for basic decentralized setup

## Out of Scope

- **Centralized mode enhancements**: Onboarding wizard focuses on decentralized mode; centralized SSH-based deployment already has clearer workflow via `pkg/mesh`
- **Production hardening**: Wizard configures basic connectivity, not security lockdown (firewall rules, key rotation policies)
- **Multi-interface setups**: Initial wizard supports single `wg0` interface; complex topologies left to manual config
- **Windows-specific discovery**: LAN multicast behavior on Windows not addressed in initial implementation
- **Persistent metrics**: No analytics or telemetry collection on onboarding completion rates
- **Automated peer provisioning**: Wizard validates peer discovery but doesn't auto-provision peer configs
- **Recovery from corruption**: If `onboarding.json` is malformed, user must manually delete; wizard only writes, doesn't repair
- **Custom DHT bootstrap nodes**: Wizard uses hardcoded bootstrap from `pkg/discovery/dht.go`
- **Network namespace support**: No `netns` awareness in wizard validation steps
