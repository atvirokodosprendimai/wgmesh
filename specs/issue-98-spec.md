# Specification: Issue #98

## Classification
fix

## Deliverables
code

## Problem Analysis

The `TestSetInterfaceAddress_Darwin` test in `pkg/daemon/helpers_test.go` has a bug in its mock executor implementation. The mock checks `args[2]` to determine whether the route command is "add" or "change", but this is incorrect.

**The Issue:**

When `setInterfaceAddress` calls the route command (line 186 in `pkg/daemon/helpers.go`), it passes:
```go
cmdExecutor.Command("route", "-n", "add", "-net", networkCIDR, "-interface", name)
```

This creates an args slice of: `["-n", "add", "-net", networkCIDR, "-interface", name]`

Therefore:
- `args[0]` = `"-n"`
- `args[1]` = `"add"` (or `"change"` for the alternate path)
- `args[2]` = `"-net"`

The mock executor in the test (lines 410 and 417) incorrectly checks `args[2]` for the command verb ("add" or "change"), but it should check `args[1]`.

**Why This Is a Bug:**

The test currently checks:
```go
if args[2] == "add" {  // Bug: args[2] is "-net", not "add"
```

This means the mock never matches the "add" or "change" commands correctly, causing the test to potentially fail or behave unexpectedly. The mock would fall through to the default case which returns an "unexpected command" error.

## Proposed Approach

The fix is straightforward: change the argument index from `args[2]` to `args[1]` in both the "add" and "change" conditional checks within the mock executor.

### Implementation Steps

1. **Update the mock executor** in `pkg/daemon/helpers_test.go`:
   - Change line 410 from: `if args[2] == "add" {`
   - To: `if args[1] == "add" {`
   - Change line 417 from: `if args[2] == "change" {`
   - To: `if args[1] == "change" {`

2. **No other changes needed**:
   - The actual implementation in `helpers.go` is correct
   - No documentation changes required (this is an internal test)
   - No API changes

## Affected Files

### Code Changes Required

1. **`pkg/daemon/helpers_test.go`** (lines 410 and 417):
   - Line 410: Change `if args[2] == "add" {` to `if args[1] == "add" {`
   - Line 417: Change `if args[2] == "change" {` to `if args[1] == "change" {`

## Test Strategy

### Verification Steps

1. **Run the specific test**:
   ```bash
   go test -v ./pkg/daemon -run TestSetInterfaceAddress_Darwin
   ```
   - The test should pass after the fix
   - All test cases in the table-driven test should succeed

2. **Run full daemon package tests**:
   ```bash
   go test -v ./pkg/daemon
   ```
   - Ensure no regressions in other tests

3. **Verify test coverage**:
   - The existing test cases already cover the scenarios we need:
     - "success - address and route added" (tests the `add` path)
     - "success - address exists, route exists" (tests the `change` path)
     - "error - route add fails" (tests `add` failure)
     - "error - route change fails" (tests `change` failure)

### Expected Results

After the fix:
- The mock should correctly match `"add"` commands and return `routeAddOutput`/`routeAddErr`
- The mock should correctly match `"change"` commands and return `routeChangeOutput`/`routeChangeErr`
- All four test scenarios involving route commands should execute their intended code paths
- Tests should pass on Darwin/macOS systems (note: may be skipped on other platforms)

### No New Tests Required

The existing test suite is comprehensive. This is purely a bug fix in the test mock, not in the actual implementation. The test cases already validate:
- Successful route addition
- Route already exists (triggering change path)
- Route add failures
- Route change failures

## Estimated Complexity

**low** (< 30 minutes)

- Two-line change (changing index from 2 to 1)
- No implementation logic changes
- No documentation updates needed
- Existing test suite validates the fix
- Risk: Very low - this only affects test code, not production code
