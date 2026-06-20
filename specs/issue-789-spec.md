# Specification: Issue #789 - Implement referral tracking system with share codes and reward logic

## Classification
feature

## Problem Analysis

wgmesh currently lacks a user acquisition and growth mechanism. As a networked product, wgmesh benefits from network effects — each new node operator potentially brings multiple peers. However, there is no structured way to:

1. **Track referrals**: When a new user learns about wgmesh from an existing user, we cannot attribute the signup or reward the referrer.
2. **Incentivize growth**: Existing users have no structured motivation to recommend wgmesh to others.
3. **Measure viral loops**: We cannot quantify word-of-mouth effectiveness or identify which users are effective advocates.

The desired solution is a **referral tracking system** with the following characteristics:

- **Share codes**: Each user gets a unique, human-readable referral code (e.g., `wgmesh.io/share/CLOUD-RUN` or `wgmesh join --referral CLOUD-RUN`).
- **Attribution**: When a new user joins with a referral code, the system records the referrer-referee relationship with a timestamp.
- **Reward logic**: Referrers earn rewards when referred users complete qualifying actions (e.g., first mesh deployment, first week of daemon uptime, subscription activation).
- **Public-repo safety**: The implementation must not expose sensitive customer data, PII, or exact revenue figures in the public repository.
- **Minimal infra**: Leverage existing wgmesh infrastructure where possible; avoid building a full customer data platform from scratch.

This system will integrate with both **centralized mode** (operators deploying nodes via SSH) and **decentralized mode** (self-discovering mesh via shared secret). The referral flow should work at the point where a user first runs `wgmesh init` or `wgmesh join`.

## Proposed Approach

### Phase 1: Core referral data structures and storage

Create a new package `pkg/referral` with the following components:

**1. ReferralCode generation (10 alphanumeric characters, 5-character groups separated by dash)**

```go
// pkg/referral/code.go
package referral

import (
    "crypto/rand"
    "encoding/base32"
    "strings"
)

// Generate creates a new share code in format XXXXX-XXXXX
// Uses base32 encoding for human-readability and collision resistance
func Generate() (string, error) {
    // 10 random bytes → 16 base32 chars → use 10 for XXXXX-XXXXX format
    buf := make([]byte, 10)
    if _, err := rand.Read(buf); err != nil {
        return "", err
    }
    
    // Base32 encode without padding
    encoded := base32.StdEncoding.EncodeToString(buf)
    encoded = strings.TrimRight(encoded, "=")
    
    // Take first 10 chars, insert dash at position 5
    if len(encoded) < 10 {
        // Fallback: repeat to reach 10 chars
        encoded = encoded + encoded[:10-len(encoded)]
    }
    
    return encoded[:5] + "-" + encoded[5:10], nil
}

// Validate checks if a code format is valid (XXXXX-XXXXX)
func Validate(code string) bool {
    parts := strings.Split(code, "-")
    if len(parts) != 2 {
        return false
    }
    if len(parts[0]) != 5 || len(parts[1]) != 5 {
        return false
    }
    // Must be alphanumeric (base32 character set)
    for _, part := range parts {
        for _, r := range part {
            if !((r >= 'A' && r <= 'Z') || (r >= '2' && r <= '7')) {
                return false
            }
        }
    }
    return true
}
```

**2. Referral record types**

```go
// pkg/referral/types.go
package referral

import "time"

// Referrer represents a user who can refer others
type Referrer struct {
    ID           string    // Unique identifier (e.g., mesh ID or account email hash)
    Code         string    // Share code (XXXXX-XXXXX)
    CreatedAt    time.Time // When the referrer joined
    ReferralCount int      // Number of successful referrals
}

// Referral represents a referrer-referee relationship
type Referral struct {
    ReferrerCode    string    // The referrer's share code
    RefereeID       string    // The referred user's identifier
    ConvertedAt     time.Time // When the referee completed a qualifying action
    RewardTier      int       // 0=registered, 1=first_deployment, 2=week_active, etc.
}

// Reward represents an earned reward
type Reward struct {
    ReferrerID      string    // Who earned it
    ReferralID      string    // Which referral triggered it
    Tier            int       // Reward tier
    GrantedAt       time.Time // When granted
    // Note: Actual reward value is stored separately (outside public repo)
}
```

**3. Storage interface**

```go
// pkg/referral/store.go
package referral

import "context"

// Store defines the persistence interface for referral data
// Implementation is NOT in this public repo — it's in the private backend
type Store interface {
    // CreateReferrer creates a new referrer with a generated code
    CreateReferrer(ctx context.Context, id string) (*Referrer, error)
    
    // GetByCode retrieves a referrer by their share code
    GetByCode(ctx context.Context, code string) (*Referrer, error)
    
    // GetByID retrieves a referrer by their ID
    GetByID(ctx context.Context, id string) (*Referrer, error)
    
    // RecordReferral records a new referrer-referee relationship
    RecordReferral(ctx context.Context, referrerCode, refereeID string) (*Referral, error)
    
    // UpdateReferralTier updates a referral's reward tier
    UpdateReferralTier(ctx context.Context, referralID string, tier int) error
    
    // ListRewards lists rewards for a referrer (paginated)
    ListRewards(ctx context.Context, referrerID string, limit int) ([]Reward, error)
}
```

### Phase 2: CLI integration

Add referral-related flags and commands to `main.go`:

**New flags for `wgmesh init` and `wgmesh join`:**

```go
// In main.go flag parsing
referralCode := flag.String("referral", "", "Referral share code (format: XXXXX-XXXXX)")
```

**New subcommand: `wgmesh referral`**

```bash
# Show user's referral code
wgmesh referral show

# Show referral stats (count, rewards)
wgmesh referral stats

# Validate a code without using it
wgmesh referral validate XXXXX-XXXXX
```

**Implementation in `main.go`:**

```go
case "referral":
    if len(os.Args) < 3 {
        fmt.Println("Usage: wgmesh referral <show|stats|validate> [code]")
        return
    }
    switch os.Args[2] {
    case "show":
        // Display user's referral code
        code, err := getOrCreateReferralCode()
        if err != nil {
            log.Fatalf("Error: %v", err)
        }
        fmt.Printf("Your referral code: %s\n", code)
        fmt.Printf("Share URL: https://wgmesh.io/?ref=%s\n", code)
        
    case "stats":
        // Display referral statistics
        stats, err := getReferralStats()
        if err != nil {
            log.Fatalf("Error: %v", err)
        }
        fmt.Printf("Referrals: %d\n", stats.Count)
        fmt.Printf("Pending conversions: %d\n", stats.Pending)
        
    case "validate":
        if len(os.Args) < 4 {
            fmt.Println("Usage: wgmesh referral validate XXXXX-XXXXX")
            return
        }
        code := os.Args[3]
        if !referral.Validate(code) {
            fmt.Println("Invalid referral code format")
            return
        }
        // Check if code exists in backend
        exists, err := backend.ReferralCodeExists(ctx, code)
        if err != nil {
            log.Fatalf("Error: %v", err)
        }
        if exists {
            fmt.Printf("Valid referral code: %s\n", code)
        } else {
            fmt.Println("Referral code not found")
        }
    }
```

**Integration with `wgmesh init`:**

When a user runs `wgmesh init` with a `--referral` flag:

1. Validate code format using `referral.Validate()`
2. Call backend to check code exists
3. Record the referral in backend
4. Store referrer code locally in `wgmesh-state.json` for later reward tier updates

**Integration with `wgmesh join`:**

Same flow as `wgmesh init` — record referral when joining an existing mesh.

### Phase 3: Reward tier logic

Define reward tiers that trigger rewards when a referred user completes qualifying actions:

```go
// pkg/referral/tier.go
package referral

// Tier definitions
const (
    TierRegistered   = 0 // User registered with referral code
    TierDeployed    = 1 // User completed first mesh deployment
    TierWeekActive  = 2 // Daemon ran for 7+ days
    TierMonthActive = 3 // Daemon ran for 30+ days
    TierSubscribed  = 4 // User activated a paid subscription (if applicable)
)

// ShouldUpgradeTier checks if a referral should be upgraded to a new tier
func ShouldUpgradeTier(currentTier, newTier int) bool {
    return newTier > currentTier
}
```

**Tier upgrade triggers:**

- **TierRegistered**: Immediate when user runs `wgmesh init --referral XXXXX-XXXXX`
- **TierDeployed**: When `wgmesh up` completes successfully for the first time
- **TierWeekActive**: When daemon uptime reaches 7 days (check via `pkg/daemon/daemon.go` metrics)
- **TierMonthActive**: When daemon uptime reaches 30 days
- **TierSubscribed**: When user activates a subscription (via Lighthouse/Polar.sh integration)

These triggers call `Store.UpdateReferralTier()` in the backend.

### Phase 4: Public repo safety measures

**Data that stays in the public repo:**

- Code generation/validation logic (`pkg/referral/code.go`)
- Type definitions (`pkg/referral/types.go`)
- Storage interface (`pkg/referral/store.go`)
- CLI integration code (`main.go`)
- Tests for all the above

**Data that NEVER goes in the public repo:**

- Actual referrer IDs (if PII)
- Referral timestamps (could identify users)
- Reward counts/amounts (revenue sensitivity)
- Store implementation (lives in private backend)

**Store implementation:**

The actual `Store` implementation lives in a private repository (e.g., `wgmesh-backend`). The public repo only contains the interface. The CLI makes HTTP/gRPC calls to the backend to perform store operations.

**Environment variable for backend endpoint:**

```bash
# In production
export WGMESH_REFERRAL_BACKEND="https://api.wgmesh.io/referral"

# In local dev
export WGMESH_REFERRAL_BACKEND="http://localhost:8080/referral"
```

### Phase 5: Testing strategy

**Unit tests:**

- `pkg/referral/code_test.go`: Test code generation format, collision resistance (Monte Carlo), validation
- `pkg/referral/tier_test.go`: Test tier upgrade logic
- `main_test.go`: Test CLI flag parsing and referral subcommands

**Integration tests:**

- Mock `Store` implementation for testing CLI flow end-to-end
- Test referral code validation and backend calls

**Test structure:**

```go
// pkg/referral/code_test.go
package referral

import (
    "testing"
)

func TestGenerateFormat(t *testing.T) {
    code, err := Generate()
    if err != nil {
        t.Fatalf("Generate() failed: %v", err)
    }
    if !Validate(code) {
        t.Errorf("Generated code failed validation: %s", code)
    }
    // Check format: XXXXX-XXXXX
    if len(code) != 11 { // 5 + 1 + 5
        t.Errorf("Wrong length: got %d, want 11", len(code))
    }
}

func TestValidate(t *testing.T) {
    tests := []struct {
        code string
        want bool
    }{
        {"ABCDE-FGHIJ", true},   // Valid
        {"abcde-fghij", false},  // Lowercase invalid
        {"AB-DEFGHIJ", false},    // Wrong length first part
        {"ABCDE-FGH", false},     // Wrong length second part
        {"ABC DE-FGHIJ", false},  // Space invalid
        {"", false},              // Empty
    }
    for _, tt := range tests {
        t.Run(tt.code, func(t *testing.T) {
            got := Validate(tt.code)
            if got != tt.want {
                t.Errorf("Validate(%q) = %v, want %v", tt.code, got, tt.want)
            }
        })
    }
}

func TestCollisionResistance(t *testing.T) {
    // Generate 10,000 codes and check for duplicates
    // Monte Carlo test for collision resistance
    generated := make(map[string]bool)
    iterations := 10000
    for i := 0; i < iterations; i++ {
        code, err := Generate()
        if err != nil {
            t.Fatalf("Generate() failed: %v", err)
        }
        if generated[code] {
            t.Fatalf("Collision detected at iteration %d: %s", i, code)
        }
        generated[code] = true
    }
    // If we get here, no collisions in 10k iterations (acceptable for 10^16 space)
}
```

**Mock Store for testing:**

```go
// pkg/referral/mock_store.go
package referral

import "context"

// MockStore implements Store for testing
type MockStore struct {
    Referrers map[string]*Referrer  // Key: ID
    Referrals map[string]*Referral  // Key: refereeID
}

func NewMockStore() *MockStore {
    return &MockStore{
        Referrers: make(map[string]*Referrer),
        Referrals: make(map[string]*Referral),
    }
}

func (m *MockStore) CreateReferrer(ctx context.Context, id string) (*Referrer, error) {
    code, _ := Generate()
    ref := &Referrer{
        ID:        id,
        Code:      code,
        CreatedAt: time.Now(),
    }
    m.Referrers[id] = ref
    return ref, nil
}

func (m *MockStore) GetByCode(ctx context.Context, code string) (*Referrer, error) {
    for _, ref := range m.Referrers {
        if ref.Code == code {
            return ref, nil
        }
    }
    return nil, ErrNotFound
}

// ... other methods
```

### Phase 6: CLI user experience

**Initial signup with referral:**

```bash
$ wgmesh init --referral CLOUD-RUN
Initialized mesh with secret: [redacted]
Your mesh ID: mesh-abc123
Referral applied: CLOUD-RUN
$ wgmesh referral show
Your referral code: NIMBUS-GOLF
Share URL: https://wgmesh.io/?ref=NIMBUS-GOLF
```

**Checking stats:**

```bash
$ wgmesh referral stats
Referral code: NIMBUS-GOLF
Total referrals: 5
Converted (first deployment): 3
Pending conversions: 2
```

**Validating a code:**

```bash
$ wgmesh referral validate CLOUD-RUN
Valid referral code: CLOUD-RUN
```

## Acceptance Criteria

1. **Code generation**: `pkg/referral.Generate()` produces codes in format `XXXXX-XXXXX` with alphanumeric characters (base32 set).
2. **Code validation**: `referral.Validate()` correctly validates valid codes and rejects invalid formats.
3. **Collision resistance**: Generating 10,000 codes produces no duplicates (Monte Carlo test passes).
4. **CLI integration**: `wgmesh init --referral XXXXX-XXXXX` validates code format and records referral with backend.
5. **CLI subcommands**: `wgmesh referral show`, `wgmesh referral stats`, and `wgmesh referral validate` work correctly.
6. **Reward tiers**: Tier definitions exist and `ShouldUpgradeTier()` logic works.
7. **Type safety**: `Referrer`, `Referral`, and `Reward` types are defined and used consistently.
8. **Store interface**: `Store` interface is defined with all required methods.
9. **Public repo safety**: No PII, actual reward values, or backend credentials in the public repo.
10. **Tests**: Unit tests for code generation, validation, tier logic, and CLI integration with mock store.

## Out of scope

- Backend implementation of `Store` interface (lives in private repo)
- Actual reward fulfillment (billing/credit logic)
- Referral fraud detection (duplicate prevention, abuse mitigation)
- Referral analytics dashboard
- Email notification system for referrals
- Social media sharing integration
- QR code generation for referral links
- Referral program terms of service/legal compliance
- Geographic or audience-based referral tracking
- Time-limited referral campaigns or bonus multipliers
- Integration with third-party referral platforms
- Referral redemption beyond the initial signup flow
- Public-facing referral leaderboard
