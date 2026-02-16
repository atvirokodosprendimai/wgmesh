# Specification: Issue #102

## Classification
fix

## Deliverables
code

## Problem Analysis

The `Hostname` field in the `PeerAnnouncement` and `KnownPeer` structs (added in issue #87) currently has no length or character validation. This creates a security vulnerability where:

1. **Memory exhaustion**: A malicious peer could send arbitrarily long hostname strings, consuming excessive memory in the `PeerStore`
2. **Display issues**: Unbounded strings could cause problems in logging, terminal display, or UI rendering
3. **Storage bloat**: Large hostnames would unnecessarily inflate the mesh state file and network messages
4. **No character validation**: Control characters or non-printable characters could cause terminal issues or security problems

### Current State

From the codebase analysis:
- **PeerAnnouncement struct** (`pkg/crypto/envelope.go`): Contains a `Hostname` field with no validation
- **KnownPeer struct** (`pkg/crypto/envelope.go`): Also contains a `Hostname` field with no validation  
- **PeerStore.Update()** (`pkg/daemon/peerstore.go:64-65`): Stores hostname without length checks
- **No Validate() method**: `PeerAnnouncement` currently has no `Validate()` method for input validation

### Security Impact

**Severity**: P1 (High Priority)
- Allows resource exhaustion attacks
- No authentication required (any peer with the shared secret can exploit)
- Affects all nodes in the mesh
- Could be used for DoS attacks

### Standards Reference

According to **RFC 1035** (Domain Names):
- Maximum hostname length: **253 characters** (excluding null terminator)
- Valid characters: alphanumeric, hyphens, and dots
- However, for display purposes, printable ASCII is more lenient and appropriate

## Proposed Approach

Add validation to `PeerAnnouncement` following the existing validation patterns in the codebase (similar to `ValidateMembershipToken` and `ValidateRotationAnnouncement`).

### Implementation Strategy

1. **Add a `Validate()` method** to `PeerAnnouncement` in `pkg/crypto/envelope.go`:
   - Check hostname length ≤ 253 characters (RFC 1035 limit)
   - Validate only printable ASCII characters (byte range 32-126)
   - Reject control characters and non-printable characters
   - Return descriptive errors for validation failures

2. **Call validation in `OpenEnvelope()`** after unmarshaling the announcement:
   - Add validation check after protocol version verification
   - This ensures all received announcements are validated before processing
   - Prevents invalid data from entering the PeerStore

3. **Consistent with existing patterns**:
   - Follow the validation pattern used in `ValidateMembershipToken()` and `ValidateRotationAnnouncement()`
   - Use clear error messages
   - Keep validation logic in the crypto package where the structs are defined

### Alternative Approaches Considered

**Option 1: Validate in PeerAnnouncement.Validate()** (Recommended)
- ✅ Validates at deserialization, before storage
- ✅ Fails fast with clear errors
- ✅ Consistent with security-first design
- ✅ No invalid data enters the system

**Option 2: Truncate in PeerStore.Update()**
- ❌ Allows invalid data into the system
- ❌ Silent data modification (unexpected behavior)
- ❌ Doesn't protect against control characters
- ❌ Validation should happen at the security boundary (crypto layer), not storage layer

**Decision**: Use Option 1 (validation in `PeerAnnouncement.Validate()`)

### Validation Rules

```go
// Hostname validation rules:
// 1. Length: 0-253 characters (empty is allowed, represents no hostname)
// 2. Characters: Only printable ASCII (bytes 32-126)
// 3. No control characters (bytes 0-31, 127+)
```

### Pseudocode

```go
// In pkg/crypto/envelope.go

// Validate validates the PeerAnnouncement fields
func (pa *PeerAnnouncement) Validate() error {
    // Validate hostname length
    if len(pa.Hostname) > 253 {
        return fmt.Errorf("hostname too long: %d characters (max 253)", len(pa.Hostname))
    }
    
    // Validate printable ASCII characters
    for i, b := range []byte(pa.Hostname) {
        if b < 32 || b > 126 {
            return fmt.Errorf("hostname contains invalid character at position %d: byte 0x%02x", i, b)
        }
    }
    
    // Additional validations can be added here for other fields
    
    return nil
}

// In OpenEnvelope(), after line 125 (protocol version check):
if err := announcement.Validate(); err != nil {
    return nil, nil, fmt.Errorf("invalid announcement: %w", err)
}
```

## Affected Files

### Code Changes

1. **`pkg/crypto/envelope.go`** (~25 lines added):
   - Add `Validate()` method to `PeerAnnouncement` struct
   - Modify `OpenEnvelope()` to call `Validate()` after unmarshaling
   - Add validation constants (e.g., `MaxHostnameLength = 253`)

### Test Files

2. **`pkg/crypto/envelope_test.go`** (NEW FILE, ~150 lines):
   - Test valid hostnames (empty, short, max length 253)
   - Test invalid hostnames (too long, control characters, non-ASCII)
   - Test edge cases (exactly 253 chars, boundary characters)
   - Test that `OpenEnvelope()` rejects invalid announcements
   - Use table-driven tests following the pattern in `membership_test.go`

## Test Strategy

### Unit Tests

**Required test cases** for `PeerAnnouncement.Validate()`:

1. **Valid hostnames**:
   - Empty string (should pass)
   - Short hostname: "server01" (should pass)
   - Maximum length: 253 character hostname (should pass)
   - Printable ASCII: "test-server.example.com" (should pass)
   - Numbers and symbols: "server-123_test" (should pass)

2. **Invalid hostnames**:
   - Too long: 254 character hostname (should fail)
   - Control character: "test\x00server" (should fail with descriptive error)
   - Newline: "test\nserver" (should fail)
   - Tab: "test\tserver" (should fail)
   - Non-ASCII: "test-über-server" (should fail)
   - High ASCII: byte 200 (should fail)

3. **Integration with OpenEnvelope()**:
   - Create envelope with invalid hostname (should be rejected)
   - Create envelope with valid hostname (should be accepted)
   - Verify error messages are descriptive

### Manual Testing

Since this is a validation-only change:
- No manual testing required beyond running unit tests
- All behavior is captured in automated tests

### Security Testing

1. **Fuzzing consideration**: Could add fuzz tests for hostname validation (future work)
2. **Attack simulation**: Unit tests should include attack scenarios (1000+ char strings)
3. **Character set testing**: Comprehensive tests for all byte values 0-255

### Test Implementation Pattern

Follow existing test patterns in the codebase:
```go
func TestPeerAnnouncementValidate(t *testing.T) {
    tests := []struct {
        name      string
        hostname  string
        wantError bool
        errContains string
    }{
        {"empty hostname", "", false, ""},
        {"valid short hostname", "server01", false, ""},
        {"max length hostname", strings.Repeat("a", 253), false, ""},
        {"too long hostname", strings.Repeat("a", 254), true, "hostname too long"},
        {"control character", "test\x00server", true, "invalid character"},
        {"newline character", "test\nserver", true, "invalid character"},
        // ... more cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            pa := &PeerAnnouncement{
                Protocol: ProtocolVersion,
                Hostname: tt.hostname,
                // ... other required fields
            }
            
            err := pa.Validate()
            if tt.wantError && err == nil {
                t.Errorf("expected error containing %q, got nil", tt.errContains)
            }
            if !tt.wantError && err != nil {
                t.Errorf("expected no error, got: %v", err)
            }
            if tt.wantError && err != nil && !strings.Contains(err.Error(), tt.errContains) {
                t.Errorf("expected error containing %q, got: %v", tt.errContains, err)
            }
        })
    }
}
```

## Estimated Complexity

**low** (30-45 minutes)

### Rationale

- **Simple validation logic**: Straightforward length and character checks
- **Clear requirements**: Well-defined limits (253 chars, printable ASCII)
- **Existing patterns**: Can follow `ValidateMembershipToken()` structure
- **Single file change**: Only `pkg/crypto/envelope.go` needs modification
- **Standard testing**: Table-driven tests are straightforward
- **No architectural changes**: Adding validation doesn't affect existing code paths
- **Low risk**: Validation is additive, doesn't modify existing behavior

### Time Breakdown

- Implementation: 10 minutes
  - Add `Validate()` method: 5 minutes
  - Integrate into `OpenEnvelope()`: 5 minutes
- Testing: 20 minutes
  - Write comprehensive unit tests: 15 minutes
  - Run tests and verify: 5 minutes  
- Documentation: 5 minutes
  - Add code comments explaining validation rules
- Verification: 10 minutes
  - Review code for edge cases
  - Ensure error messages are clear

**Total**: ~45 minutes
