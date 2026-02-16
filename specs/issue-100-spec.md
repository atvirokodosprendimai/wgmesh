# Specification: Issue #100

## Classification
fix

## Deliverables
code + documentation

## Problem Analysis

When receiving `PeerAnnouncement` messages via `OpenEnvelope` (in `pkg/crypto/envelope.go`), only protocol version and timestamp freshness are validated. The following critical fields are accepted without any validation and stored directly in the PeerStore:

1. **`WGPubKey`** — Not checked for valid base64 encoding or correct length (44 characters expected)
2. **`MeshIP`** — Not checked for valid IPv4/IPv6 address format
3. **`WGEndpoint`** — Not checked for valid `host:port` format
4. **`RoutableNetworks`** — Not checked for valid CIDR notation
5. **`Hostname`** — Issue mentions this field but it doesn't currently exist in `PeerAnnouncement` struct. If added in the future, it would need length validation (≤ 253 chars per DNS standards)

### Security Impact

Accepting unvalidated data from the network creates several security risks:

- **Malformed data injection**: Invalid IP addresses or malformed CIDRs could cause crashes or unexpected behavior
- **Resource exhaustion**: Unbounded strings could consume excessive memory
- **Configuration corruption**: Invalid WireGuard keys could break mesh connectivity
- **Code injection**: Improperly validated endpoint strings could be exploited in system calls

### Evidence from Code Analysis

**`OpenEnvelope` validation** (`pkg/crypto/envelope.go:88-138`):
- Line 95: Validates nonce size only
- Lines 124-126: Validates protocol version
- Lines 129-135: Validates timestamp freshness
- **Missing**: No validation of announcement field contents

**Direct usage of unvalidated fields** — All discovery handlers create `PeerInfo` from raw announcement fields:

| Handler | File | Lines | Unvalidated Fields Used |
|---------|------|-------|------------------------|
| `handleHello` | `pkg/discovery/exchange.go` | 180-186 | WGPubKey, MeshIP, WGEndpoint, RoutableNetworks |
| `handleReply` | `pkg/discovery/exchange.go` | 200-206 | WGPubKey, MeshIP, WGEndpoint, RoutableNetworks |
| `updateTransitivePeers` | `pkg/discovery/exchange.go` | 291-297 | WGPubKey, MeshIP, WGEndpoint |
| `handleAnnouncement` (gossip) | `pkg/discovery/gossip.go` | 277-295 | WGPubKey, MeshIP, WGEndpoint, RoutableNetworks |
| LAN `listenLoop` | `pkg/discovery/lan.go` | 199-207 | WGPubKey, MeshIP, WGEndpoint, RoutableNetworks |

**Additional unvalidated `KnownPeer` fields** — The `KnownPeer` struct used in transitive discovery also lacks validation:
- `WGPubKey`, `MeshIP`, `WGEndpoint` are used without validation in transitive peer updates

## Proposed Approach

### 1. Add Validation Method to `PeerAnnouncement`

Add a `Validate() error` method to `pkg/crypto/envelope.go` that checks:

```go
func (pa *PeerAnnouncement) Validate() error
```

**Validation rules:**

1. **`WGPubKey`**: 
   - Must be valid base64 encoding
   - Must be exactly 44 characters (standard WireGuard public key length)
   - Decoded value must be 32 bytes

2. **`MeshIP`**: 
   - Must parse as a valid IPv4 or IPv6 address using `net.ParseIP()`
   - Must not be empty

3. **`WGEndpoint`**: 
   - If non-empty, must be valid `host:port` format
   - Use `net.SplitHostPort()` for validation
   - Port must be numeric and in valid range (1-65535)
   - Empty string is acceptable (peer may not have endpoint yet)

4. **`RoutableNetworks`**: 
   - Each entry must be valid CIDR notation
   - Use `net.ParseCIDR()` for validation
   - Empty slice is acceptable

5. **`Hostname`** (if added in future):
   - Length must be ≤ 253 characters (DNS maximum)
   - Empty string is acceptable

### 2. Add Validation Method to `KnownPeer`

Add a `Validate() error` method to validate transitive peer data:

```go
func (kp *KnownPeer) Validate() error
```

Apply same validation rules for `WGPubKey`, `MeshIP`, and `WGEndpoint`.

### 3. Call Validation in `OpenEnvelope`

After deserializing the `PeerAnnouncement` in `OpenEnvelope` (line ~121), call:

```go
if err := announcement.Validate(); err != nil {
    return nil, nil, fmt.Errorf("invalid announcement: %w", err)
}
```

This provides a single validation point for all discovery methods.

### 4. Validate `KnownPeers` Array

In `PeerAnnouncement.Validate()`, also validate each entry in `KnownPeers`:

```go
for i, kp := range pa.KnownPeers {
    if err := kp.Validate(); err != nil {
        return fmt.Errorf("invalid known peer at index %d: %w", i, err)
    }
}
```

### 5. Defense-in-Depth: Optional Handler-Level Validation

While `OpenEnvelope` validation is sufficient, consider adding defensive checks in discovery handlers as a second layer of protection. This is optional but recommended for defense-in-depth.

## Affected Files

### Code Changes Required

1. **`pkg/crypto/envelope.go`**:
   - Add `Validate()` method to `PeerAnnouncement` struct (~30-40 lines)
   - Add `Validate()` method to `KnownPeer` struct (~15-20 lines)
   - Call `announcement.Validate()` in `OpenEnvelope` after deserialization (line ~121)

### Test Files to Create

2. **`pkg/crypto/envelope_test.go`** (new file):
   - Test `PeerAnnouncement.Validate()` with valid data
   - Test rejection of invalid `WGPubKey` (wrong length, invalid base64, etc.)
   - Test rejection of invalid `MeshIP` (malformed IP, empty string)
   - Test rejection of invalid `WGEndpoint` (malformed host:port, invalid port)
   - Test rejection of invalid `RoutableNetworks` (malformed CIDR)
   - Test validation of `KnownPeers` array
   - Test `KnownPeer.Validate()` edge cases
   - Table-driven tests for comprehensive coverage

### Documentation Updates (Optional)

3. **`ENCRYPTION.md`** or **`README.md`**:
   - Document the validation rules for peer announcements
   - Note that announcements with invalid fields are rejected

## Test Strategy

### Unit Tests

Create comprehensive table-driven tests in `pkg/crypto/envelope_test.go`:

1. **Valid Announcement Tests**:
   - Valid WireGuard key (44 chars base64)
   - Valid IPv4 and IPv6 addresses
   - Valid endpoint formats (with and without DNS names)
   - Valid CIDR notations
   - Empty optional fields

2. **Invalid WGPubKey Tests**:
   - Too short (< 44 chars)
   - Too long (> 44 chars)
   - Invalid base64 characters
   - Empty string

3. **Invalid MeshIP Tests**:
   - Malformed IP address
   - Empty string
   - Invalid characters

4. **Invalid WGEndpoint Tests**:
   - Missing port
   - Invalid port (non-numeric, out of range)
   - Malformed host:port

5. **Invalid RoutableNetworks Tests**:
   - Malformed CIDR notation
   - Missing network bits (/XX)
   - Invalid IP in CIDR

6. **KnownPeer Validation Tests**:
   - Valid known peers
   - Invalid known peers in array
   - Empty known peers array

### Integration Testing

1. **Test with modified messages**:
   - Use `test-encryption.sh` as reference
   - Create test that sends malformed announcements
   - Verify they are rejected at `OpenEnvelope` level

2. **Test discovery handlers**:
   - Verify handlers don't crash with validated input
   - Verify invalid announcements are rejected before reaching handlers

### Manual Testing

1. Build and run basic mesh network
2. Verify normal operation is unaffected
3. Test that legitimate peers still connect successfully
4. Monitor logs for validation errors (should be none in normal operation)

## Estimated Complexity

**medium** (2-4 hours)

### Breakdown:
- **Validation method implementation**: 1-1.5 hours
  - `PeerAnnouncement.Validate()`: ~30-40 lines
  - `KnownPeer.Validate()`: ~15-20 lines
  - Integration into `OpenEnvelope`: ~2 lines
  
- **Test implementation**: 1-2 hours
  - Comprehensive table-driven tests
  - Edge case coverage
  - Integration test scenarios
  
- **Documentation**: 15-30 minutes
  - Update ENCRYPTION.md or README.md with validation rules
  
- **Testing and verification**: 30 minutes
  - Build and run tests
  - Manual verification
  - Review edge cases

### Risk Assessment:
- **Low risk**: Changes are additive and fail-safe
- **No breaking changes**: Only rejects invalid data that should never exist
- **Single validation point**: `OpenEnvelope` catches all malformed announcements
- **Easy to test**: Unit tests provide comprehensive coverage
