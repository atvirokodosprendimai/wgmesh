# Specification: Issue #112

## Classification
refactor

## Deliverables
code

## Problem Analysis

The codebase has significant code duplication between `pkg/ssh/routes.go` and `pkg/daemon/routes.go`. These files contain nearly identical implementations of route management logic:

### Duplicated Components

1. **Route Data Structures** (virtually identical):
   - `pkg/ssh/routes.go` line 8: `RouteEntry` struct (exported)
   - `pkg/daemon/routes.go` line 10: `routeEntry` struct (unexported)
   - Both have `Network` and `Gateway` string fields

2. **Helper Functions** (100% identical logic):
   - `normalizeNetwork`: Lines 116-131 (ssh) vs 125-135 (daemon)
     - Adds `/32` suffix for IPv4 host routes
     - Adds `/128` suffix for IPv6 host routes
     - Identical implementation
   
   - `makeRouteKey`: Lines 110-114 (ssh) vs 121-123 (daemon)
     - Creates composite key: `"network|gateway"`
     - Identical implementation

3. **Core Route Logic** (structurally identical with minor variations):
   - `CalculateRouteDiff` (ssh) vs `calculateRouteDiff` (daemon): Lines 54-108 (ssh) vs 81-119 (daemon)
     - Both use dual-map approach (exact match + network-only)
     - Same logic for detecting gateway changes
     - Minor difference: ssh checks `route.Gateway != ""` in line 99; daemon doesn't (line 113)
     - Same algorithm overall

4. **Route Retrieval** (different implementations due to execution context):
   - `GetCurrentRoutes` (ssh): Lines 13-52 - uses SSH client to run `ip route` remotely
   - `getCurrentRoutes` (daemon): Lines 43-79 - uses `exec.Command` to run `ip route` locally
   - Different execution mechanism but same parsing logic

### Maintenance Issues

- Changes to route logic require updating both files
- Risk of divergence when one is updated without the other
- Harder to write comprehensive tests (must duplicate test cases)
- Violates DRY (Don't Repeat Yourself) principle

## Proposed Approach

Create a new shared package `pkg/routes` to extract common route management logic. Both `ssh` and `daemon` packages will import this shared package.

### Package Structure

Create `pkg/routes/routes.go` containing:

1. **Exported Route Type**:
   ```go
   type Entry struct {
       Network string
       Gateway string
   }
   ```

2. **Exported Core Functions**:
   - `CalculateDiff(current, desired []Entry) (toAdd, toRemove []Entry)` - route diffing algorithm
   - `NormalizeNetwork(network string) string` - network CIDR normalization
   - `MakeKey(network, gateway string) string` - composite key generation

3. **Implementation Strategy**:
   - Extract identical functions (`normalizeNetwork`, `makeRouteKey`) as-is
   - Extract `CalculateRouteDiff` with the more defensive ssh version (includes gateway empty check)
   - Keep route retrieval functions (`GetCurrentRoutes`, `getCurrentRoutes`) in their respective packages since they have different execution contexts

### Migration Steps

1. **Create new package** `pkg/routes`:
   - Create `pkg/routes/routes.go` with exported types and functions
   - Use the more defensive implementation from ssh for `CalculateDiff`

2. **Update `pkg/ssh/routes.go`**:
   - Import `"github.com/atvirokodosprendimai/wgmesh/pkg/routes"`
   - Change `RouteEntry` to `routes.Entry` throughout the file
   - Replace `CalculateRouteDiff` with call to `routes.CalculateDiff`
   - Remove `makeRouteKey`, `normalizeNetwork` functions
   - Keep `GetCurrentRoutes` and `ApplyRouteDiff` (SSH-specific)

3. **Update `pkg/daemon/routes.go`**:
   - Import `"github.com/atvirokodosprendimai/wgmesh/pkg/routes"`
   - Change `routeEntry` to `routes.Entry` throughout the file
   - Replace `calculateRouteDiff` with call to `routes.CalculateDiff`
   - Remove `makeRouteKey`, `normalizeNetwork` functions
   - Keep `getCurrentRoutes` and `applyRouteDiff` (exec-specific)
   - Update `syncPeerRoutes` to use `routes.Entry`

4. **Update `pkg/daemon/daemon.go`**:
   - Update any references to `routeEntry` type if needed

### Why `pkg/routes` instead of `pkg/wireguard`?

While the issue suggests `pkg/wireguard` as an option, creating `pkg/routes` is better because:
- Route management is independent of WireGuard configuration
- Routes can apply to any network interface, not just WireGuard
- Clear separation of concerns: `pkg/wireguard` handles WireGuard configs; `pkg/routes` handles OS-level routing
- Follows the existing package structure (crypto, privacy, discovery are all separate)

## Affected Files

### New Files
1. **`pkg/routes/routes.go`** (create)
   - New package with shared route logic
   - ~80 lines of code

### Modified Files
1. **`pkg/ssh/routes.go`**
   - Remove lines 54-131 (duplicated functions)
   - Replace with imports and calls to `pkg/routes`
   - Update type references from `RouteEntry` to `routes.Entry`
   - Keep SSH-specific functions: `GetCurrentRoutes`, `ApplyRouteDiff`

2. **`pkg/daemon/routes.go`**
   - Remove lines 81-135 (duplicated functions)
   - Replace with imports and calls to `pkg/routes`
   - Update type references from `routeEntry` to `routes.Entry`
   - Keep daemon-specific functions: `getCurrentRoutes`, `applyRouteDiff`
   - Update `syncPeerRoutes` signature

3. **`pkg/daemon/daemon.go`** (potentially)
   - Update references if `routeEntry` type is used elsewhere

## Test Strategy

### Unit Tests

1. **Create `pkg/routes/routes_test.go`**:
   - Test `NormalizeNetwork`:
     - IPv4 host addresses (add `/32`)
     - IPv6 host addresses (add `/128`)
     - Already-normalized networks (no change)
   - Test `MakeKey`:
     - Various network/gateway combinations
     - Empty gateway handling
   - Test `CalculateDiff`:
     - No changes (identical current/desired)
     - Add new routes
     - Remove old routes
     - Gateway changes (remove old, add new)
     - Mixed scenarios

2. **Update existing tests** (if they exist):
   - Check for `ssh` package tests that use `RouteEntry`
   - Check for `daemon` package tests that use `routeEntry`
   - Update type references to `routes.Entry`

### Integration Tests

1. **Verify SSH route operations**:
   - Run existing SSH-based route tests
   - Ensure route diffing works correctly in centralized mode

2. **Verify daemon route operations**:
   - Run existing daemon route tests
   - Ensure route diffing works correctly in decentralized mode

### Manual Verification

1. Test centralized mode route management:
   ```bash
   wgmesh -init
   wgmesh -add node1:10.99.0.1:192.168.1.10
   # Verify routes are correctly calculated and applied
   ```

2. Test decentralized mode route management:
   ```bash
   wgmesh join --secret test-secret
   # Verify daemon correctly syncs peer routes
   ```

3. Compare route diff output before/after refactor to ensure behavior is identical

## Estimated Complexity

**medium** (4-6 hours)

### Breakdown
- Creating new `pkg/routes` package: 1 hour
- Refactoring `pkg/ssh/routes.go`: 1 hour
- Refactoring `pkg/daemon/routes.go`: 1 hour
- Writing comprehensive unit tests: 2 hours
- Integration testing and verification: 1 hour

### Complexity Justification
- Medium complexity due to:
  - Need to carefully extract and test shared code
  - Must ensure no behavior changes in either ssh or daemon modes
  - Requires updating multiple files and ensuring type consistency
  - Need comprehensive tests to verify no regressions
- Not "high" because:
  - Logic is already well-defined and tested in both locations
  - Clear separation between shared and package-specific code
  - No new features, just code reorganization
