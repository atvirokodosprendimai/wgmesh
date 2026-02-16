# Specification: Issue #105

## Classification
refactor

## Deliverables
code

## Problem Analysis

Currently, `pkg/crypto/derive.go` uses inline string literals for HKDF info/salt parameters and implicit key sizes derived from array slice operations. This creates several maintainability and security audit concerns:

### Current Issues

1. **Magic String Literals**: HKDF info strings are scattered as inline literals throughout the `DeriveKeys()` function:
   - Line 43: `"wgmesh-gossip-v1"` for gossip key derivation
   - Line 48: `"wgmesh-subnet-v1"` for mesh subnet derivation
   - Line 53: `"wgmesh-mcast-v1"` for multicast ID derivation
   - Line 58: `"wgmesh-wg-psk-v1"` for WireGuard PSK derivation
   - Line 64: `"wgmesh-gossip-port-v1"` for gossip port derivation
   - Line 70: `"rv"` for rendezvous ID (used in concat, not HKDF)
   - Line 74: `"wgmesh-membership-v1"` for membership key derivation
   - Line 79: `"wgmesh-epoch-v1"` for epoch seed derivation

2. **Implicit Key Sizes**: Key sizes are defined by the target array/slice size rather than explicit constants:
   - Network ID: 20 bytes (DHT infohash requirement) - defined in struct but not as standalone constant
   - Gossip Key: 32 bytes (AES-256 requirement) - defined in struct
   - Mesh Subnet: 2 bytes - defined in struct
   - Multicast ID: 4 bytes - defined in struct
   - PSK: 32 bytes (WireGuard requirement) - defined in struct
   - Rendezvous ID: 8 bytes - defined in struct
   - Membership Key: 32 bytes - defined in struct
   - Epoch Seed: 32 bytes - defined in struct

3. **Security Audit Concerns**: 
   - Typos in info strings would silently produce wrong keys
   - Difficult to verify all derivation contexts at a glance
   - No single source of truth for cryptographic parameters
   - Version strings (v1) are not easily searchable or auditable

4. **Reusability Risk**: If any of these info strings need to be referenced elsewhere in the codebase (e.g., for key rotation, compatibility checks, or documentation generation), developers would need to duplicate the string literals, creating maintenance burden and typo risk.

### Why This Matters

HKDF info/salt parameters are cryptographically significant - they provide domain separation between different key derivations from the same master secret. Using the wrong info string would produce completely different keys, breaking the system. Having these as named constants:
- Makes them easier to audit in security reviews
- Prevents typos through IDE autocomplete and compiler checks
- Centralizes cryptographic parameters for easier review
- Improves code documentation and maintainability

## Proposed Approach

### Step 1: Define HKDF Info Constants

Add a new constant block near the top of `pkg/crypto/derive.go` (after existing constants) with all HKDF info strings:

```go
const (
    // HKDF info strings for domain separation
    hkdfInfoGossipKey    = "wgmesh-gossip-v1"
    hkdfInfoSubnet       = "wgmesh-subnet-v1"
    hkdfInfoMulticastID  = "wgmesh-mcast-v1"
    hkdfInfoPSK          = "wgmesh-wg-psk-v1"
    hkdfInfoGossipPort   = "wgmesh-gossip-port-v1"
    hkdfInfoMembership   = "wgmesh-membership-v1"
    hkdfInfoEpoch        = "wgmesh-epoch-v1"
    
    // Other derivation-related strings
    rendezvousSuffix     = "rv"
)
```

**Naming Convention**: Use `hkdfInfo` prefix for HKDF salt/info parameters to clearly indicate their purpose. Use camelCase for Go constant naming.

### Step 2: Define Key Size Constants

Add constants for key sizes that have specific cryptographic or protocol requirements:

```go
const (
    // Key and ID sizes (in bytes)
    networkIDSize        = 20  // DHT infohash requirement (BEP 5)
    gossipKeySize        = 32  // AES-256 key size
    meshSubnetSize       = 2   // /16 subnet prefix
    multicastIDSize      = 4   // IPv4 multicast group suffix
    pskSize              = 32  // WireGuard preshared key size
    gossipPortBytesSize  = 2   // uint16 for port derivation
    rendezvousIDSize     = 8   // GitHub Issue search term
    membershipKeySize    = 32  // HMAC-SHA256 key size
    epochSeedSize        = 32  // Relay rotation seed
)
```

**Note**: These sizes are already implicitly defined by the `DerivedKeys` struct field types. The constants serve as documentation and can be used in documentation comments but are NOT used to replace struct field definitions (arrays are sized explicitly in the struct).

### Step 3: Update Function Calls

Replace all string literals with the named constants in `DeriveKeys()` function:
- Line 43: `deriveHKDF(secret, hkdfInfoGossipKey, keys.GossipKey[:])`
- Line 48: `deriveHKDF(secret, hkdfInfoSubnet, keys.MeshSubnet[:])`
- Line 53: `deriveHKDF(secret, hkdfInfoMulticastID, keys.MulticastID[:])`
- Line 58: `deriveHKDF(secret, hkdfInfoPSK, keys.PSK[:])`
- Line 64: `deriveHKDF(secret, hkdfInfoGossipPort, portBytes[:])`
- Line 70: `secret + rendezvousSuffix` (rendezvous ID computation)
- Line 74: `deriveHKDF(secret, hkdfInfoMembership, keys.MembershipKey[:])`
- Line 79: `deriveHKDF(secret, hkdfInfoEpoch, keys.EpochSeed[:])`

### Step 4: Update Documentation

Add a comment block above the HKDF info constants explaining their purpose:

```go
// HKDF info/salt strings provide domain separation for key derivation.
// These ensure that different keys derived from the same secret are
// cryptographically independent. Changing these values will break
// compatibility with existing meshes.
```

### Step 5: Update `deriveHKDF` Function Signature

The `deriveHKDF` helper function currently takes `salt string` parameter. According to HKDF specification and the `hkdf.New()` call pattern, this is actually the "info" parameter (the third argument), not salt (second argument, which is `nil` in all cases).

Consider renaming the parameter for clarity:
```go
func deriveHKDF(secret, info string, output []byte) error {
    reader := hkdf.New(sha256.New, []byte(secret), nil, []byte(info))
    // ...
}
```

This is a minor documentation fix to use correct HKDF terminology.

## Affected Files

### Code Changes Required

1. **`pkg/crypto/derive.go`**:
   - **Lines 13-15** (after existing constants): Add new constant block for HKDF info strings and key sizes
   - **Lines 43, 48, 53, 58, 64, 70, 74, 79**: Replace string literals with named constants
   - **Line 145** (function signature): Optional parameter rename from `salt` to `info` for clarity
   - **Line 146** (HKDF call): Update comment if parameter renamed

### No Changes Required

- **`pkg/crypto/derive_test.go`**: Tests use the public API and don't reference these internal constants directly. Tests should continue passing without modification.
- **Other crypto files**: No other files in `pkg/crypto/` reference these strings directly.
- **Documentation**: No external documentation references these internal implementation details.

## Test Strategy

### Existing Tests Verification

The existing test suite in `pkg/crypto/derive_test.go` should pass without modification:

1. **`TestDeriveKeys`**: Verifies deterministic derivation - should still produce identical keys
2. **`TestDeriveKeysDifferentSecrets`**: Verifies different secrets produce different keys
3. **`TestGossipPortRange`**: Verifies gossip port is in expected range
4. **`TestRendezvousIDLength`**: Verifies rendezvous ID is correct length

Run the full crypto package test suite:
```bash
go test -v ./pkg/crypto/
```

### Key Derivation Consistency Check

Since this is a pure refactoring with no functional changes, the derived keys must remain identical. Create a simple validation:

```bash
# Before changes
go test -run TestDeriveKeys -v ./pkg/crypto/ > /tmp/before.txt

# After changes
go test -run TestDeriveKeys -v ./pkg/crypto/ > /tmp/after.txt

# Compare (should be identical)
diff /tmp/before.txt /tmp/after.txt
```

Alternatively, add a temporary test that compares the actual key values against known-good values derived from a test secret, then remove it after validation.

### Code Review Checklist

- [ ] All HKDF info string literals replaced with constants
- [ ] Constants follow Go naming conventions (camelCase)
- [ ] Constants grouped logically with clear comments
- [ ] No functional changes to key derivation logic
- [ ] All existing tests pass without modification
- [ ] No changes to the `DerivedKeys` struct field sizes
- [ ] `deriveHKDF` parameter naming is clear (optional improvement)

### Manual Verification

Visually inspect that:
1. No string literal appears in HKDF calls
2. All HKDF info constants are used exactly once in `DeriveKeys()`
3. Constant names clearly indicate their purpose
4. Comments explain cryptographic significance

### Build and Lint

```bash
make build  # Verify clean compilation
make test   # Verify all tests pass
make lint   # Verify no style issues (if golangci-lint available)
```

## Estimated Complexity

**low** (30-45 minutes)

**Justification**:
- Pure refactoring with no logic changes
- Small, well-defined scope (single file)
- No new functionality or tests required
- Low risk: existing tests verify correctness
- Mechanical transformation of literals to constants
- No cross-package dependencies to update

**Breakdown**:
- Define constants: 10 minutes
- Replace string literals: 10 minutes  
- Update documentation/comments: 5 minutes
- Run tests and verify: 10 minutes
- Code review and final checks: 5-10 minutes
