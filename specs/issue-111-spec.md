# Specification: Issue #111

## Classification
feature

## Deliverables
code

## Problem Analysis

Currently, every call to `exec.Command("wg", ...)` triggers a PATH lookup to find the `wg` binary. This is inefficient, especially in the reconcile loop which runs every 5 seconds and makes multiple WireGuard operations per cycle.

### Current Implementation Issues

1. **Direct `exec.Command("wg")` calls without caching** in:
   - `pkg/wireguard/apply.go:112` - SetPeer (called for each peer during reconcile)
   - `pkg/wireguard/apply.go:125` - RemovePeer (called for stale peers)
   - `pkg/wireguard/apply.go:134` - GetPeers (called during sync operations)
   - `pkg/wireguard/keys.go:11,21,36` - Key generation/validation (less frequent but still uncached)

2. **Via cmdExecutor in daemon package** at:
   - `pkg/daemon/helpers.go:140` - configureInterface
   - `pkg/daemon/helpers.go:251` - resetInterface peer cleanup
   - `pkg/daemon/helpers.go:283` - getWGInterfacePort

3. **wireguard-go lookup** at:
   - `pkg/daemon/helpers.go:89` - Uses `exec.LookPath` but doesn't cache result

### Performance Impact

- Reconcile loop runs every 5 seconds (`ReconcileInterval = 5 * time.Second`)
- Each cycle can call `wg` multiple times:
  - SetPeer for each active peer
  - GetPeers for peer list
  - RemovePeer for stale peers
  - getWGInterfacePort for port checks
- PATH lookup is performed for each call, adding unnecessary overhead

## Proposed Approach

Cache the WireGuard binary path at initialization time and reuse it for all subsequent command executions.

### Implementation Strategy

1. **For `pkg/wireguard` package**:
   - Add package-level variable to store cached `wg` binary path
   - Add init() function or explicit Initialize() function to call `exec.LookPath("wg")`
   - Fall back to `"wg"` if LookPath fails (maintaining current behavior)
   - Update all `exec.Command("wg", ...)` calls to use the cached path

2. **For `pkg/daemon` package** (via cmdExecutor):
   - cmdExecutor already has LookPath capability via the CommandExecutor interface
   - Add caching mechanism in RealCommandExecutor or at daemon init
   - Option A: Modify RealCommandExecutor to cache paths on first lookup
   - Option B: Pre-lookup paths at daemon initialization and store in Daemon struct
   - Update cmdExecutor.Command calls to use cached paths

3. **For wireguard-go**:
   - Cache the wireguard-go path similarly (macOS-specific)
   - Use cached path for createInterface calls

### Backward Compatibility

- If `exec.LookPath` fails, fall back to using `"wg"` directly (current behavior)
- This maintains compatibility with systems where `wg` is in PATH but LookPath might fail
- No API changes to public functions

### Design Decisions

**Decision 1: Where to cache?**
- **Option A**: Package-level variables with init() function
  - Pros: Simple, automatic initialization
  - Cons: Harder to test, global state
- **Option B**: Explicit Initialize() function
  - Pros: More testable, explicit control
  - Cons: Requires caller to remember to initialize
- **Recommended**: Option A for `pkg/wireguard`, integrate with existing CommandExecutor pattern for `pkg/daemon`

**Decision 2: Error handling on lookup failure?**
- Fall back to `"wg"` to maintain current behavior
- Log a warning that PATH lookup failed but continuing with command name
- This ensures no breaking changes

**Decision 3: Thread safety?**
- Package-level variables with init() are initialized once, no concurrent writes
- No mutex needed for read-only cached paths

## Affected Files

### Code Changes Required

1. **`pkg/wireguard/keys.go`**:
   - Add package variable: `var wgBinaryPath string`
   - Add init() function to cache path
   - Update lines 11, 21, 36 to use cached path

2. **`pkg/wireguard/apply.go`**:
   - Use same cached `wgBinaryPath` from keys.go (shared package variable)
   - Update lines 112, 125, 134 to use cached path

3. **`pkg/daemon/executor.go`**:
   - Add caching to RealCommandExecutor
   - Option: Add cache map for commonly used binaries
   - Cache "wg" and "wireguard-go" paths

4. **`pkg/daemon/helpers.go`**:
   - Potentially add daemon initialization code to pre-lookup paths
   - Or rely on executor-level caching (preferred)

### Testing Considerations

- Mock testing already uses MockCommandExecutor which doesn't perform real PATH lookups
- Add unit tests to verify caching works correctly
- Test fallback behavior when LookPath fails
- Ensure cached path is used for all subsequent calls

## Test Strategy

### Unit Tests

1. **Test path caching in wireguard package**:
   - Create test that verifies `wgBinaryPath` is set after init
   - Test that failed LookPath falls back to "wg"
   - Mock exec.Command to verify cached path is used

2. **Test CommandExecutor caching**:
   - Test RealCommandExecutor caches paths correctly
   - Test that multiple Command() calls use cached path
   - Test fallback behavior

### Integration Tests

1. **Daemon reconcile loop test**:
   - Start daemon, let it run through several reconcile cycles
   - Verify `wg` commands execute successfully
   - Performance test (optional): Measure improvement in reconcile time

2. **Key generation test**:
   - Test GenerateKeyPair uses cached path
   - Test ValidatePrivateKey uses cached path

### Manual Testing

1. Run daemon in normal mode, verify reconciliation works
2. Test on system where `wg` is in non-standard location
3. Test fallback behavior by temporarily making `wg` not findable

### Backward Compatibility Testing

1. Ensure existing tests continue to pass
2. Verify behavior is unchanged from user perspective
3. Test on systems with different PATH configurations

## Estimated Complexity

**low** (1-2 hours)

- Straightforward caching implementation
- No API changes required
- Existing test infrastructure (MockCommandExecutor) already in place
- Main effort is ensuring all call sites are updated consistently
- Simple verification via unit tests
