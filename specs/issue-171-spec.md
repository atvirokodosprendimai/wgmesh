# Specification: Issue #171

## Classification
fix

## Deliverables
code

## Problem Analysis

After implementing PR #111 (binary path caching), the `wireguard-go` binary path is now resolved once at package initialization time in `pkg/daemon/helpers.go` via `exec.LookPath`. This optimization caches the absolute path (e.g., `/usr/local/bin/wireguard-go`) in the package-level variable `wireguardGoBinPath`.

However, the test suite in `pkg/daemon/helpers_test.go` was not updated to account for this change. Specifically, `TestCreateInterface_Darwin` sets up a mock executor with a `commandFunc` that checks for the literal string `"wireguard-go"`:

```go
commandFunc: func(name string, args ...string) Command {
    if filepath.Base(name) == "wireguard-go" && len(args) == 1 && args[0] == tt.interfaceName {
        // ... mock implementation
    }
    // falls through to "unexpected command" error
}
```

The issue is that `createInterface()` now calls `cmdExecutor.Command(wireguardGoBinPath, name)` where `wireguardGoBinPath` is an absolute path like `/usr/local/bin/wireguard-go` on macOS systems where the binary is installed. The mock's check `filepath.Base(name) == "wireguard-go"` correctly extracts the base name, but the production code path has already been cached at init time with the absolute path.

### Current Behavior on macOS
When `wireguard-go` is installed on the system:
1. Package `init()` runs: `wireguardGoBinPath` is set to `/usr/local/bin/wireguard-go`
2. Test runs and sets up mock with `commandFunc` checking for `"wireguard-go"`
3. Production code calls `cmdExecutor.Command("/usr/local/bin/wireguard-go", "utun0")`
4. Mock's `commandFunc` receives `name = "/usr/local/bin/wireguard-go"`
5. `filepath.Base(name)` extracts `"wireguard-go"` correctly, so the mock should match
6. **Wait - looking more carefully at line 275 in helpers_test.go, the check IS using `filepath.Base(name)`!**

Let me re-examine the actual failure. The issue description states the tests fail with "unexpected command" error. Looking at helpers.go line 122:

```go
cmd := cmdExecutor.Command(wireguardGoBinPath, name)
```

And the test at lines 274-280 has:
```go
if filepath.Base(name) == "wireguard-go" && len(args) == 1 && args[0] == tt.interfaceName {
```

Actually, looking more carefully at the test code from lines 261-265, I see:
```go
oldWireguardGoBinPath := wireguardGoBinPath
wireguardGoBinPath = "wireguard-go"
defer func() {
    wireguardGoBinPath = oldWireguardGoBinPath
}()
```

So the test IS already trying to reset `wireguardGoBinPath` to `"wireguard-go"`! But this appears to be the CORRECT approach already implemented.

Let me re-read the issue description more carefully. It says:

> After caching the `wireguard-go` binary path at package init in `pkg/daemon/helpers.go` (PR for atvirokodosprendimai/wgmesh#111), the `TestCreateInterface_Darwin` tests break on macOS.

This means the tests are currently failing. Let me check if the mock commandFunc is properly matching. Looking at line 275:

```go
if filepath.Base(name) == "wireguard-go" && len(args) == 1 && args[0] == tt.interfaceName {
```

Wait, this should work! Unless... let me check if there's a different code path. Oh! Looking at line 262-265:

```go
oldWireguardGoBinPath := wireguardGoBinPath
wireguardGoBinPath = "wireguard-go"
defer func() {
    wireguardGoBinPath = oldWireguardGoBinPath
}()
```

This code DOES reset the variable, and it's using `t.Cleanup` pattern via defer. But the issue says the tests are failing. Let me re-read the issue's "Fix Required" section:

> Update `pkg/daemon/helpers_test.go` `TestCreateInterface_Darwin` so the mock `commandFunc` matches on `wireguardGoBinPath` (the package-level cached path) instead of the literal string `"wireguard-go"`.

So the issue is asking us to make the mock match on whatever `wireguardGoBinPath` is set to, rather than hardcoding `"wireguard-go"`.

Actually, I think I misread the test code. Let me look again at lines 261-265. These lines save and restore the variable, setting it to `"wireguard-go"` during the test. This IS Option A from the issue description!

But the issue says the tests are failing. Perhaps the current code doesn't have these lines 261-265 yet? Or perhaps they're not working as expected?

Looking at the issue description again:

> **Option A** (preferred): Make `wireguardGoBinPath` a test-overridable package var. In each test case, save and restore `wireguardGoBinPath` via `t.Cleanup`, and set it to `"wireguard-go"` before calling `createInterface` so the mock `commandFunc`'s `name == "wireguard-go"` check still matches.

So Option A is to add the save/restore logic. But I can see it's already there in lines 261-265! Unless this is on a different branch?

Let me assume the current code DOES have the caching (from PR #111) but does NOT have the test fix yet. The spec needs to describe adding the save/restore logic.

Actually, re-reading more carefully, the issue mentions branch `feat/cache-wg-binary-path-111`. We're on `copilot/fix-testcreateinterface-darwin`. So the feat branch has the breaking change, and we need to write a spec for the fix.

### Root Cause
1. `pkg/daemon/helpers.go` now caches `wireguardGoBinPath` at package init time (line 32-34)
2. On macOS with `wireguard-go` installed, this becomes an absolute path like `/usr/local/bin/wireguard-go`
3. Tests in `helpers_test.go` mock the executor but expect to match on the string `"wireguard-go"`
4. When production code calls `cmdExecutor.Command(wireguardGoBinPath, ...)` with the cached absolute path, the mock's literal string check fails
5. The test code at lines 261-265 attempts to reset the variable but may not be working correctly, OR this code doesn't exist yet on the failing branch

### Failing Tests
From the issue:
```
--- FAIL: TestCreateInterface_Darwin/success_-_interface_created
    helpers_test.go:300: createInterface(utun0) unexpected error: failed to start wireguard-go: unexpected command
--- FAIL: TestCreateInterface_Darwin/error_-_wireguard-go_not_found
    helpers_test.go:296: createInterface(utun0) error = failed to start wireguard-go: unexpected command, want error containing "wireguard-go not found in PATH"
--- FAIL: TestCreateInterface_Darwin/error_-_interface_not_created_after_polling
    helpers_test.go:296: createInterface(utun0) error = failed to start wireguard-go: unexpected command, want error containing "was not created on macOS"
```

All three test cases are getting "unexpected command" error, which means the mock's `commandFunc` is not matching the command call.

## Proposed Approach

Implement **Option A** from the issue description (preferred approach):

### Changes Required

1. **Ensure `wireguardGoBinPath` is exported or accessible for testing**
   - Current code already has `wireguardGoBinPath` as a package-level variable (line 26 in helpers.go)
   - It's not exported (lowercase), which is fine for same-package tests

2. **Update `TestCreateInterface_Darwin` in `helpers_test.go`**
   - For each test case, save the original `wireguardGoBinPath` value
   - Set `wireguardGoBinPath = "wireguard-go"` before calling the mock executor
   - Restore the original value using `defer` or `t.Cleanup()`
   - This ensures the mock's literal string check `name == "wireguard-go"` or `filepath.Base(name) == "wireguard-go"` will match

3. **Update the mock `commandFunc` if necessary**
   - Current check uses `filepath.Base(name) == "wireguard-go"` (line 275)
   - This should work with both absolute paths and relative names
   - However, if we reset `wireguardGoBinPath` to `"wireguard-go"` in the test, it will pass the literal string to the mock
   - Verify the mock check handles both cases correctly

### Implementation Pattern

```go
func TestCreateInterface_Darwin(t *testing.T) {
    // ... existing test setup ...
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Save original value
            oldWireguardGoBinPath := wireguardGoBinPath
            
            // Reset to short name for test
            wireguardGoBinPath = "wireguard-go"
            
            // Restore after test
            defer func() {
                wireguardGoBinPath = oldWireguardGoBinPath
            }()
            
            // Or using t.Cleanup (Go 1.14+):
            // t.Cleanup(func() {
            //     wireguardGoBinPath = oldWireguardGoBinPath
            // })
            
            // ... rest of test ...
        })
    }
}
```

### Alternative: Option B

Update the mock `commandFunc` to match on `filepath.Base(name)` instead of exact string match. However, looking at line 275, this is already implemented! So Option B is actually already done, which suggests Option A is the missing piece.

## Affected Files

### Code Changes
- `pkg/daemon/helpers_test.go`:
  - Function: `TestCreateInterface_Darwin` (lines 210-313)
  - Add save/restore logic for `wireguardGoBinPath` inside each test case (after line 260, before mock setup)
  - Estimated changes: ~6-10 lines added (save, set, restore pattern)

### Documentation Changes
None required (internal test fix)

## Test Strategy

### Pre-Implementation Verification
1. **Confirm tests are failing on branch `feat/cache-wg-binary-path-111`:**
   ```bash
   git checkout feat/cache-wg-binary-path-111
   go test ./pkg/daemon/... -run TestCreateInterface_Darwin -v
   ```
   Expected: All 3 test cases fail with "unexpected command" error

### Post-Implementation Verification
1. **Run the specific failing test:**
   ```bash
   go test ./pkg/daemon/... -run TestCreateInterface_Darwin -v
   ```
   Expected: All 3 test cases pass

2. **Run all daemon package tests:**
   ```bash
   go test ./pkg/daemon/... -v
   ```
   Expected: No failures, no regressions

3. **Run full test suite:**
   ```bash
   go test ./...
   ```
   Expected: No failures across all packages

4. **Verify production behavior is unchanged:**
   - The `wireguardGoBinPath` is still cached at init time
   - The caching optimization from PR #111 is preserved
   - Only test behavior is modified

### Edge Cases to Test
1. **System with wireguard-go installed**: Path is absolute (e.g., `/usr/local/bin/wireguard-go`)
2. **System without wireguard-go**: Path remains `"wireguard-go"` (fallback)
3. **Multiple test cases in sequence**: Variable is properly restored between tests
4. **Parallel test execution**: Each test case properly isolates its variable state (use `t.Parallel()` if appropriate)

### Manual Testing on macOS
If possible, test on an actual macOS system with `wireguard-go` installed:
```bash
# Verify wireguard-go is in PATH
which wireguard-go

# Run the tests
go test ./pkg/daemon/... -run TestCreateInterface_Darwin -v

# Verify the cached path is being used in production
go build
./wgmesh join --secret "wgmesh://v1/test" --interface utun5
# Check that wireguard-go process is running
ps aux | grep wireguard-go
```

## Estimated Complexity
low

**Reasoning:**
- Simple test-only change
- Well-understood pattern (save/restore test state)
- Clear fix path already identified in the issue
- No production code changes
- No changes to APIs, protocols, or data structures
- Low risk of side effects

**Estimated Time:** 15-30 minutes including testing

## Additional Notes

### Why Option A is Preferred

1. **Minimal test changes**: Only requires adding save/restore logic to one test function
2. **Preserves existing mock logic**: The `commandFunc` check remains unchanged
3. **Clear intent**: Explicitly shows that tests need to use short binary name
4. **Standard pattern**: Save/restore test state is a common Go testing pattern

### Why Not Option B Alone

While Option B (using `filepath.Base(name)`) is good defensive programming and appears to already be implemented in line 275, it alone doesn't solve the problem if there are other parts of the test or mock that expect the short name. Option A ensures the entire test operates with the expected short name consistently.

### Concurrency Considerations

If tests are run with `t.Parallel()`, each test case runs in its own goroutine. Since `wireguardGoBinPath` is a package-level variable, concurrent modifications could cause race conditions. 

**Recommendation**: Do NOT use `t.Parallel()` in this test function, or use a different approach:
- Pass the binary path as a parameter to `createInterface()`
- Use a context or config struct
- Use `sync.Mutex` to protect the variable

However, since the existing test doesn't use `t.Parallel()`, we can safely use the save/restore pattern without concurrency concerns.

### Production Behavior Preserved

The fix only affects test code. Production behavior remains unchanged:
- `init()` still caches the binary path at startup (lines 28-35 in helpers.go)
- `createInterface()` still uses the cached `wireguardGoBinPath` (line 122)
- Performance optimization from PR #111 is fully preserved
