# Specification: Issue #114

## Classification
refactor

## Deliverables
code

## Problem Analysis

The wgmesh codebase has established a consistent error wrapping pattern using `fmt.Errorf("context: %w", err)` to preserve error context throughout the stack trace. However, `pkg/discovery/registry.go` contains inconsistencies where some errors are returned without wrapping, losing valuable debugging context.

**Specific instances in `pkg/discovery/registry.go`:**

1. **Line 371** in `UpdatePeerListWithAll()`:
   ```go
   jsonData, err := json.Marshal(update)
   if err != nil {
       return err  // No context - should wrap
   }
   ```

2. **Line 377** in `UpdatePeerListWithAll()`:
   ```go
   req, err := http.NewRequest("PATCH", url, bytes.NewReader(jsonData))
   if err != nil {
       return err  // No context - should wrap
   }
   ```

3. **Line 386** in `UpdatePeerListWithAll()`:
   ```go
   resp, err := r.client.Do(req)
   if err != nil {
       return err  // No context - should wrap
   }
   ```

**Contrast with adjacent properly-wrapped errors:**

- Line 362: `return fmt.Errorf("failed to build issue body: %w", err)` ✓
- Line 392: `return fmt.Errorf("update returned status %d: %s", resp.StatusCode, string(respBody))` ✓

The same file has 13 other error returns that properly wrap errors with context, making these 3 instances stand out as inconsistent.

**Broader scope:**

A codebase-wide search reveals similar bare `return err` patterns in other packages:
- `pkg/discovery/exchange.go` (2 instances)
- `pkg/crypto/derive.go` (1 instance)
- `pkg/daemon/routes.go` (1 instance)
- `pkg/daemon/helpers.go` (1 instance)
- `pkg/wireguard/config.go` (instances found)

While the issue focuses on `registry.go`, the audit should identify these other cases for consistency.

## Proposed Approach

### Primary Fix: pkg/discovery/registry.go

Wrap all three bare error returns in `UpdatePeerListWithAll()` function with descriptive context:

1. **Line 371** - JSON marshaling error:
   ```go
   if err != nil {
       return fmt.Errorf("failed to marshal update: %w", err)
   }
   ```

2. **Line 377** - HTTP request creation error:
   ```go
   if err != nil {
       return fmt.Errorf("failed to create PATCH request: %w", err)
   }
   ```

3. **Line 386** - HTTP request execution error:
   ```go
   if err != nil {
       return fmt.Errorf("update request failed: %w", err)
   }
   ```

### Audit Other Packages

Review and document bare `return err` instances in other `pkg/` files:

1. **pkg/discovery/exchange.go**:
   - Line 239: `WriteToUDP` error in send function
   - Line 387: `WriteToUDP` error in send function

2. **pkg/crypto/derive.go**:
   - Line 148: `io.ReadFull` error in HKDF derivation

3. **pkg/daemon/routes.go**:
   - Line 36: `getCurrentRoutes` error

4. **pkg/daemon/helpers.go**:
   - Line 58: `json.MarshalIndent` error

5. **pkg/wireguard/config.go**:
   - To be identified during implementation

For each instance, determine if wrapping is appropriate based on:
- Whether the calling function already has sufficient context
- Whether the error is being passed through from a well-named function
- Whether additional context would aid debugging

**Decision criteria:**
- If the function is a low-level utility returning an error from stdlib, wrapping is beneficial
- If the error is already well-contextualized by the calling function name, wrapping may be optional
- Consistency with surrounding code in the same file is paramount

## Affected Files

### Code Changes Required

1. **`pkg/discovery/registry.go`** (lines 371, 377, 386):
   - Wrap 3 bare error returns in `UpdatePeerListWithAll()` function
   - Add descriptive context matching the style of other error returns in the file

### Potential Changes (Based on Audit)

2. **`pkg/discovery/exchange.go`** (lines 239, 387):
   - Review and potentially wrap UDP write errors

3. **`pkg/crypto/derive.go`** (line 148):
   - Review and potentially wrap HKDF read error

4. **`pkg/daemon/routes.go`** (line 36):
   - Review and potentially wrap route retrieval error

5. **`pkg/daemon/helpers.go`** (line 58):
   - Review and potentially wrap JSON marshal error

6. **`pkg/wireguard/config.go`**:
   - Identify and review bare error returns

## Test Strategy

### Code Review Verification

1. **Manual inspection**:
   - Read through each changed function
   - Verify error messages provide useful debugging context
   - Ensure error wrapping uses `%w` verb to preserve error chains
   - Confirm consistency with adjacent error handling in the same file

2. **Pattern search**:
   - Re-run `grep -rn "^\s+return err\s*$" pkg/` to verify all intentional bare returns
   - Use `grep -rn "fmt.Errorf.*%w" pkg/` to verify proper error wrapping syntax

### Existing Tests

1. **Run existing unit tests**:
   ```bash
   go test ./pkg/discovery/... -v
   go test ./pkg/crypto/... -v
   go test ./pkg/daemon/... -v
   go test ./pkg/wireguard/... -v
   ```
   - Verify no tests break due to error message changes
   - Existing tests should pass unchanged (error wrapping is transparent to behavior)

2. **Integration tests**:
   - Run integration tests if available to ensure end-to-end functionality is preserved
   - Error wrapping should not change program behavior, only improve debugging

### Manual Testing

1. **Trigger error conditions** (optional, if test environment permits):
   - Attempt registry operations with invalid GitHub token to trigger HTTP errors
   - Verify error messages now include helpful context
   - Example: Instead of generic `json: unsupported type`, should see `failed to marshal update: json: unsupported type`

2. **Error chain verification**:
   - Use `errors.Is()` and `errors.As()` if applicable to verify error unwrapping works correctly
   - Confirm that wrapped errors can still be compared with their underlying types

### Risk Assessment

- **Very low risk**: Error wrapping only adds context, doesn't change program logic
- **No behavioral changes**: Functions return the same error types, just with more context
- **Backward compatible**: Error wrapping with `%w` preserves error identity for `errors.Is()` checks
- **Testing impact**: Minimal - only affects error message content in test assertions (if any)

## Estimated Complexity

**low** (30-60 minutes)

- Primary fix in `registry.go` is straightforward (3 lines to change)
- Audit of other packages requires review but not necessarily changes
- No new logic or complex refactoring required
- Existing tests should pass without modification
- Main effort is in thorough audit and documentation of findings
