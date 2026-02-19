# Specification: Issue #217

## Classification
feature

## Deliverables
code

## Problem Analysis

The `pkg/wireguard/` package currently has zero test coverage. This is a critical gap because the package contains core logic used by both centralized and decentralized modes for managing WireGuard configurations.

The most important untested code is the config diffing logic in `pkg/wireguard/config.go`:

1. **`CalculateDiff(current, desired *Config) *ConfigDiff`** (lines 105-132)
   - Computes the set of changes required to transition from current to desired configuration
   - Identifies added peers, removed peers, modified peers, and interface changes
   - Returns a `ConfigDiff` struct describing all necessary changes

2. **`peersEqual(a, b Peer) bool`** (lines 139-168)
   - Compares two peers for equality
   - Checks PresharedKey, Endpoint, PersistentKeepalive, and AllowedIPs
   - Uses order-independent comparison for AllowedIPs slice
   - Used by `CalculateDiff` to detect modified peers

3. **`HasChanges() bool`** (lines 135-137)
   - Method on `ConfigDiff` to determine if any changes exist
   - Returns true if there are interface changes, added peers, removed peers, or modified peers
   - Used to avoid unnecessary WireGuard reconfigurations

These are pure functions with no I/O dependencies (no SSH, no filesystem, no network), making them ideal candidates for comprehensive unit testing.

### Why This Matters

- **Reliability**: Config diffing errors can cause peers to be incorrectly added, removed, or misconfigured
- **Both modes depend on it**: Centralized mode (SSH deploy) and decentralized mode (daemon) both use this logic
- **Complex logic**: The peer comparison includes AllowedIPs order-independent matching which needs validation
- **Easy to test**: Pure functions with no external dependencies

## Proposed Approach

Create a new test file `pkg/wireguard/config_test.go` with table-driven tests for the three functions.

### Test 1: `TestCalculateDiff`

Table-driven test covering multiple scenarios:

1. **Identical configs** → Empty diff (no changes)
2. **Added peer** → Peer in `AddedPeers` map
3. **Removed peer** → Peer public key in `RemovedPeers` slice
4. **Modified peer - endpoint changed** → Peer in `ModifiedPeers` map
5. **Modified peer - AllowedIPs changed** → Peer in `ModifiedPeers` map
6. **Interface port changed** → `InterfaceChanged = true`
7. **Multiple changes at once** → All applicable diff fields populated
8. **AllowedIPs order changed (but same IPs)** → No modification detected (order-independent)

Each test case will:
- Define a `current` and `desired` Config
- Call `CalculateDiff(current, desired)`
- Assert the expected diff structure
- Use `t.Parallel()` for concurrent execution

### Test 2: `TestPeersEqual`

Table-driven test covering equality comparisons:

1. **Identical peers** → true
2. **Different PresharedKey** → false
3. **Different Endpoint** → false
4. **Different PersistentKeepalive** → false
5. **Different AllowedIPs (different IPs)** → false
6. **Different AllowedIPs (different count)** → false
7. **Same AllowedIPs in different order** → true (order-independent)
8. **Empty vs populated AllowedIPs** → false

Each test case will:
- Define two `Peer` structs
- Call `peersEqual(a, b)`
- Assert expected boolean result
- Use `t.Parallel()` for concurrent execution

### Test 3: `TestHasChanges`

Table-driven test covering change detection:

1. **Empty diff** → false
2. **InterfaceChanged only** → true
3. **AddedPeers only** → true
4. **RemovedPeers only** → true
5. **ModifiedPeers only** → true
6. **Multiple change types** → true

Each test case will:
- Define a `ConfigDiff` struct
- Call `diff.HasChanges()`
- Assert expected boolean result
- Use `t.Parallel()` for concurrent execution

### Testing Pattern

Following existing project conventions (seen in `pkg/ratelimit/limiter_test.go`, `pkg/mesh/mesh_test.go`, `pkg/daemon/peerstore_test.go`):

```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name     string
        input    InputType
        expected ExpectedType
    }{
        {"scenario 1", input1, expected1},
        {"scenario 2", input2, expected2},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            // test logic
        })
    }
}
```

## Affected Files

### New Files
- `pkg/wireguard/config_test.go` (new test file)

### Existing Files
- `pkg/wireguard/config.go` (NO CHANGES - only read for testing)

## Test Strategy

### Coverage Goals
- `CalculateDiff`: minimum 4 scenarios (requirement), targeting 8+ for comprehensive coverage
- `peersEqual`: minimum 3 scenarios (requirement), targeting 8+ for comprehensive coverage
- `HasChanges`: complete coverage of all boolean conditions (6 scenarios)

### Execution Strategy
1. Use `t.Parallel()` in all test cases for concurrent execution
2. Table-driven tests with descriptive test names
3. Clear assertions with helpful error messages
4. No test mocking needed (pure functions)

### Validation
Run tests with:
```bash
go test ./pkg/wireguard/
go test -v ./pkg/wireguard/
go test -race ./pkg/wireguard/  # verify no race conditions
go test -cover ./pkg/wireguard/ # measure coverage
```

### Success Criteria
- All tests pass
- No production code changes
- Coverage ≥80% for `CalculateDiff`, `peersEqual`, `HasChanges`
- Tests follow project conventions (table-driven, t.Parallel())
- Tests are readable and maintainable

## Estimated Complexity

**low**

### Rationale
- Pure function testing with no I/O or mocking required
- Well-defined inputs and outputs
- Clear test patterns already established in the project
- No production code changes needed
- No new dependencies required
- Straightforward table-driven test structure

### Time Estimate
- Writing `TestCalculateDiff`: 25-30 minutes
- Writing `TestPeersEqual`: 15-20 minutes
- Writing `TestHasChanges`: 10-15 minutes
- Running tests and verifying coverage: 10 minutes
- Total: ~60-75 minutes

## Additional Notes

### Test Data Construction

Test cases will use realistic WireGuard data:
- **Public keys**: Base64-encoded 44-character strings (e.g., `"abc123...XYZ="`)
- **Endpoints**: Standard IP:port format (e.g., `"192.168.1.5:51820"`)
- **AllowedIPs**: CIDR notation (e.g., `[]string{"10.99.0.2/32", "fd00::2/128"}`)
- **PersistentKeepalive**: Integer seconds (e.g., `25`)

### Edge Cases to Test

1. **Empty configs**: Both current and desired are empty
2. **Nil vs empty slices**: `AllowedIPs` as nil vs `[]string{}`
3. **Special values**: Endpoint as `"(none)"` per WireGuard convention
4. **AllowedIPs ordering**: Same IPs in different order should be equal
5. **Multiple peer changes**: Mix of add, remove, modify in single diff

### Alignment with Project Standards

- Follows Go 1.23 and standard `gofmt` formatting
- Matches existing test patterns in `pkg/daemon/peerstore_test.go` and `pkg/mesh/mesh_test.go`
- Uses standard `testing` package (no external test frameworks)
- Error messages use consistent format: `t.Errorf("expected X, got Y")`
- Test function names: `TestFunctionName` or `TestFunctionName_Scenario`

### No Production Code Changes

Per acceptance criteria:
- **Zero changes** to `pkg/wireguard/config.go`
- **Zero changes** to any production code
- **Only file created**: `pkg/wireguard/config_test.go`
