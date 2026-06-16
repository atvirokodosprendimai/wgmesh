# Specification: Issue #729 - Design referral program to leverage existing customer satisfaction

## Classification
feature

## Problem Analysis
wgmesh has achieved early customer satisfaction based on positive feedback from dogfooding users. Word-of-mouth is already happening organically. However, there is no structured mechanism to:

1. **Track and incentivize referrals** - Satisfied customers cannot be formally rewarded for bringing new users
2. **Measure referral effectiveness** - No visibility into which acquisition channels or users drive growth
3. **Sustain growth momentum** - Early adopter enthusiasm needs to be captured and amplified before it fades

Current state:
- No referral tracking system exists
- No customer account management beyond basic mesh configuration
- CLI-only tooling limits non-technical referral opportunities
- Dogfood users provide positive feedback but no structured NPS collection

This feature must integrate cleanly with wgmesh's two operational modes:
- **Centralized mode** (SSH-managed nodes): Likely referral source (IT/admin users)
- **Decentralized mode** (self-discovering mesh): Likely referral target (technical teams)

## Proposed Approach

### Phase 1: Account and referral identity infrastructure

**Task 1.1: Create account package**
Create new package `pkg/account/` with:

```go
// types.go
type AccountID string
type ReferralCode string

type Account struct {
    ID           AccountID
    Email        string // Optional, for reward delivery
    ReferralCode ReferralCode
    CreatedAt    time.Time
}

type Referral struct {
    ReferrerID  AccountID
    ReferredID  AccountID
    Code        ReferralCode
    ConvertedAt time.Time // When referred account completed first mesh setup
}
```

**Task 1.2: Referral code generation**
Implement cryptographically secure referral code generation in `pkg/account/code.go`:

```go
// GenerateCode creates a unique referral code for an account
// Format: [base32(account_id_hash)][checksum][version]
// Length: 12 characters, URL-safe, case-insensitive
func GenerateCode(accountID AccountID) (ReferralCode, error)

// ValidateCode verifies checksum and extracts account ID
func ValidateCode(code ReferralCode) (AccountID, error)
```

Use HKDF-SHA256 for code derivation (consistent with existing `pkg/crypto/derive.go`). Format:
- First 8 chars: Base32(HKDF(accountID, "referral"))
- Next 3 chars: CRC24 checksum (detect typos)
- Final 1 char: Version byte (currently '1')

**Task 1.3: Local account storage**
Create `pkg/account/store.go` with file-based storage:

```go
type Store struct {
    mu       sync.RWMutex
    path     string // ~/.wgmesh/accounts.json
    accounts map[AccountID]*Account
    referrals []*Referral
}

// CreateAccount generates a new account with referral code
func (s *Store) CreateAccount(email string) (*Account, error)

// GetByCode retrieves account by referral code
func (s *Store) GetByCode(code ReferralCode) (*Account, error)

// RecordReferral records a successful referral conversion
func (s *Store) RecordReferral(referrerID, referredID AccountID, code ReferralCode) error
```

Storage format: JSON file at `~/.wgmesh/accounts.json` with atomic writes (temp file + rename).

### Phase 2: CLI integration for referral flows

**Task 2.1: Add `wgmesh account` subcommand**
Extend `main.go` with new subcommand group:

```go
var cmdAccount = &cli.Command{
    Name:  "account",
    Usage: "Manage your wgmesh account and referrals",
    Subcommands: []*cli.Command{
        cmdAccountCreate,
        cmdAccountStatus,
        cmdAccountReferrals,
    },
}

var cmdAccountCreate = &cli.Command{
    Name:  "create",
    Usage: "Create a new account",
    Flags: []cli.Flag{
        &cli.StringFlag{
            Name:  "email",
            Usage: "Email address for reward delivery (optional)",
        },
        &cli.StringFlag{
            Name:  "referral-code",
            Usage: "Referral code if you were referred (optional)",
        },
    },
    Action: func(ctx *cli.Context) error {
        // Create account, handle referral if present
    },
}
```

**Task 2.2: Implement status command**
```go
var cmdAccountStatus = &cli.Command{
    Name:  "status",
    Usage: "Show account details and referral code",
    Action: func(ctx *cli.Context) error {
        // Display:
        // - Account ID
        // - Referral code (prominent: "Share this code!")
        // - Total referrals
        // - Conversion rate (referrals that completed mesh setup)
    },
}
```

**Task 2.3: Implement referrals list command**
```go
var cmdAccountReferrals = &cli.Command{
    Name:  "referrals",
    Usage: "List all your successful referrals",
    Action: func(ctx *cli.Context) error {
        // Table output:
        // | Referred ID | Code Used | Converted At |
    },
}
```

### Phase 3: Conversion tracking

**Task 3.1: Track first-mesh completion**
Modify `pkg/daemon/daemon.go` to detect first successful mesh setup:

```go
// After first successful reconcile with >=1 peer:
if d.firstMeshComplete && !d.conversionRecorded {
    accountStore := getAccountStore()
    if account := accountStore.GetCurrent(); account != nil {
        if account.ReferredBy != "" {
            accountStore.RecordReferral(account.ReferredBy, account.ID, account.ReferralCode)
        }
    }
    d.conversionRecorded = true
}
```

**Task 3.2: Add telemetry events**
Extend `pkg/pilot/metrics.go` with referral events:

```go
type ReferralEvent struct {
    EventType string // "code_generated", "conversion", "reward_earned"
    AccountID AccountID
    Code      ReferralCode
    Timestamp time.Time
}
```

### Phase 4: Reward structure (foundation for future payments)

**Task 4.1: Define reward tiers**
Create `pkg/account/rewards.go` with reward logic:

```go
type RewardTier struct {
    ReferralsRequired int
    RewardType        string // "credit", "extension", "premium_feature"
    Value             int    // Duration in months or credit amount
}

var RewardTiers = []RewardTier{
    {1, "credit", 1},         // 1 referral = 1 month credit
    {5, "extension", 3},      // 5 referrals = 3 month extension
    {10, "premium_feature", 0}, // 10 referrals = feature unlock
}
```

**Task 4.2: Calculate pending rewards**
```go
func (s *Store) CalculateRewards(accountID AccountID) []Reward {
    // Count successful referrals
    // Match against reward tiers
    // Return unclaimed rewards
}
```

**Task 4.3: Display pending rewards in status**
Add reward summary to `wgmesh account status` output:

```
Your Referral Stats:
  Referral Code: ABC123XYZ4A
  Total Referrals: 7
  Converted: 5 (71%)

Pending Rewards:
  - 5 referrals: 3-month service extension (unclaimed)
```

### Phase 5: Documentation and UX

**Task 5.1: Create referral guide**
Add `docs/referral-program.md` with:
- How to get your referral code
- Sharing guidelines (what to avoid)
- Reward structure explanation
- FAQ ("Can I refer my team?", "When do rewards apply?")

**Task 5.2: In-app messaging**
Add post-install messaging in daemon startup:

```
First mesh setup complete!
Your referral code: ABC123XYZ4A
Share it with your team to earn rewards.
Run 'wgmesh account status' for details.
```

## Acceptance Criteria

- [ ] **Code generation**: Referral codes are 12 characters, URL-safe, typo-resistant (checksum validated)
- [ ] **Account creation**: `wgmesh account create` generates unique account with code in <100ms
- [ ] **Referral tracking**: Referred accounts record referrer ID and code on conversion
- [ ] **Status display**: `wgmesh account status` shows code, referral count, conversion rate
- [ ] **Reward calculation**: System correctly calculates pending rewards based on referral tiers
- [ ] **Data persistence**: Account/referral data survives daemon restarts (atomic file writes)
- [ ] **CLI usability**: All commands work with `--help`, clear error messages for invalid codes
- [ ] **Test coverage**: `pkg/account/` package has >80% test coverage
- [ ] **Documentation**: Referral guide exists with clear examples
- [ ] **No secrets**: No API keys or credentials hardcoded; codes are public identifiers

## Out of Scope

- **Payment processing** - No actual credit card processing or payouts in this phase
- **Email verification** - Email is optional, no verification flow
- **Web dashboard** - All referral management via CLI
- **Multi-level referrals** - No "refer-a-friend-of-friend" tracking
- **Fraud detection** - Basic code validation only; no abuse detection yet
- **API integration** - No external service dependencies (Stripe, SendGrid, etc.)
- **Compliance tools** - No tax reporting or 1099 generation
- **Time-limited campaigns** - Rewards are persistent, no expiration logic

## Affected Files

### New files
- `pkg/account/types.go` - Core account and referral data structures
- `pkg/account/code.go` - Referral code generation/validation
- `pkg/account/store.go` - Persistent account storage
- `pkg/account/rewards.go` - Reward tier calculations
- `pkg/account/store_test.go` - Tests for store persistence
- `pkg/account/code_test.go` - Tests for code generation/validation
- `docs/referral-program.md` - User-facing documentation

### Modified files
- `main.go` - Add `account` subcommand group
- `pkg/daemon/daemon.go` - Track first-mesh completion for conversion
- `go.mod` / `go.sum` - No new external dependencies (stdlib + existing crypto only)

## Test Strategy

### Unit tests
- `TestGenerateCodeUnique` - Generate 1000 codes, verify no collisions
- `TestValidateCodeChecksum` - Test typo detection (single-char errors)
- `TestStoreAtomicWrite` - Verify crash-safe persistence (write/interrupt/read)
- `TestReferralTracking` - Verify referrer ID preserved through conversion
- `TestRewardCalculation` - Test edge cases (0 referrals, exact tier boundaries)

### Integration tests
- `TestCLIReferralFlow` - End-to-end: create account → get code → refer → convert
- `TestConversionTracking` - Simulate daemon reconcile, verify conversion recorded
- `TestConcurrentAccountAccess` - Race detection on store access (`-race` flag)

### Manual verification
1. Create two accounts, verify codes are unique
2. Use referral code during account creation
3. Complete mesh setup with referred account
4. Verify referrer's status shows new referral
5. Check atomic persistence (kill daemon during write, verify data intact)

## Estimated Complexity

**Medium** - ~1,200 lines of new code across 7 files

Breakdown:
- Account/store logic: 400 lines
- Code generation/validation: 150 lines
- CLI integration: 200 lines
- Tests: 350 lines
- Documentation: 100 lines

**Risks:**
- File storage race conditions (mitigated by atomic writes)
- Code collisions at scale (mitigated by HKDF + sufficient entropy)
- Conversion tracking false positives (mitigated by explicit first-mesh check)

**Dependencies:**
- None (uses existing `pkg/crypto/` for HKDF)
- Go stdlib only (encoding, crypto, filesystem)
