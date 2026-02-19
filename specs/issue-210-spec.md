# Specification: Issue #210

## Classification
fix

## Deliverables
code

## Problem Analysis

In `pkg/mesh/deploy.go`, the `Deploy()` function (lines 15-94) contains a resource leak. Specifically, at line 51, `defer client.Close()` is called inside a `for hostname, node := range m.Nodes` loop.

### Root Cause

In Go, `defer` statements execute when the **enclosing function returns**, not when the loop iteration ends. This means:

1. For a mesh with N nodes, the `Deploy()` function creates N SSH connections
2. Only the **first** connection gets closed at the end of each iteration (implicitly, when the `client` variable is reassigned)
3. The remaining N-1 connections stay open until `Deploy()` returns
4. This holds multiple SSH connections open simultaneously, consuming system resources unnecessarily

### Current Code (lines 44-91)

```go
for hostname, node := range m.Nodes {
    fmt.Printf("Deploying to %s...\n", hostname)

    client, err := ssh.NewClient(node.SSHHost, node.SSHPort)
    if err != nil {
        return fmt.Errorf("failed to connect to %s: %w", hostname, err)
    }
    defer client.Close()  // ❌ WRONG: defers until Deploy() returns

    // ... deployment logic ...
    
    fmt.Printf("  ✓ Deployed successfully\n\n")
}
```

### Impact

- **Resource exhaustion**: Large meshes (e.g., 50+ nodes) may exhaust available SSH connections or file descriptors
- **Delayed cleanup**: Connections remain open longer than necessary
- **Server-side limits**: Remote SSH servers may have connection limits that this pattern violates
- **Debugging confusion**: `netstat` or `ss` will show multiple ESTABLISHED SSH connections during deployment

### Evidence from Codebase

The same file contains a **correct** example in the `detectEndpoints()` function (lines 96-147):

```go
func (m *Mesh) detectEndpoints() error {
    // ...
    for hostname, node := range m.Nodes {
        // ...
        client, err := ssh.NewClient(node.SSHHost, node.SSHPort)
        if err != nil {
            return fmt.Errorf("failed to connect to %s: %w", hostname, err)
        }

        // ... work with client ...
        
        publicIP, err := ssh.DetectPublicIP(client)
        client.Close()  // ✅ CORRECT: explicit close in loop

        // ... rest of iteration ...
    }
    return nil
}
```

At line 129, `client.Close()` is called **explicitly** within the loop, not using `defer`. This is the correct pattern.

## Proposed Approach

### Option 1: Explicit Close (Recommended)

Remove the `defer client.Close()` and add an explicit `client.Close()` call after all operations on that node are complete, **before** moving to the next iteration.

**Pros:**
- Matches the existing `detectEndpoints()` pattern in the same file
- Minimal code changes
- Clear and explicit
- Same resource management pattern across the file

**Cons:**
- Need to ensure all error paths also close the client
- Less idiomatic Go (defer is typically preferred)

### Option 2: Anonymous Function Wrapper

Wrap each loop iteration in an anonymous function that uses `defer client.Close()` correctly:

```go
for hostname, node := range m.Nodes {
    err := func() error {
        fmt.Printf("Deploying to %s...\n", hostname)

        client, err := ssh.NewClient(node.SSHHost, node.SSHPort)
        if err != nil {
            return fmt.Errorf("failed to connect to %s: %w", hostname, err)
        }
        defer client.Close()  // ✅ Now closes when anonymous function returns

        // ... deployment logic ...
        
        fmt.Printf("  ✓ Deployed successfully\n\n")
        return nil
    }()
    if err != nil {
        return err
    }
}
```

**Pros:**
- Uses `defer` idiomatically
- Ensures cleanup even if new code paths are added

**Cons:**
- More code changes (wrapping function, error handling)
- Indentation changes affect many lines
- Less common pattern for this use case

### Recommendation

**Option 1** (explicit close) is recommended because:
1. It matches the existing pattern in `detectEndpoints()` (same file, lines 111-129)
2. Minimal code changes
3. Easier to review and verify
4. Consistent style within the file
5. The deployment logic is straightforward with clear success/error paths

## Affected Files

### Code Changes

- `pkg/mesh/deploy.go`:
  - Line 51: Remove `defer client.Close()`
  - After line 88 (end of deployment logic, before "✓ Deployed successfully"): Add `client.Close()`
  
**Note**: We need to ensure the close happens after all client operations (lines 53-88) but before the success message.

### No Documentation Changes Required

This is an internal implementation fix that doesn't change the public API or behavior.

## Test Strategy

### Existing Tests

The issue states "Existing tests still pass" as an acceptance criterion. Check if tests exist:

```bash
# Find existing tests
find pkg/mesh -name "*_test.go"

# Run existing mesh tests
go test ./pkg/mesh/... -v
```

Expected: `pkg/mesh/mesh_test.go` and `pkg/mesh/policy_test.go` exist

### Manual Testing

Since this is a resource management fix, manual verification is important:

1. **Test with small mesh (2-3 nodes):**
   ```bash
   # Monitor SSH connections before deploy
   ss -tn | grep :22
   
   # Run deploy
   wgmesh deploy
   
   # Verify connections are closed progressively (not all at once at the end)
   watch -n 0.5 'ss -tn | grep :22'
   ```

2. **Test with larger mesh (10+ nodes if available):**
   - Verify no "too many open files" or connection limit errors
   - Verify SSH connections are closed between nodes

3. **Test error paths:**
   - Introduce a connection failure (wrong credentials, unreachable host)
   - Verify no connection leaks occur before the error

### Verifying the Fix

The fix can be verified by:
- Adding debug logging to track when connections are created/closed
- Using `lsof` or `ss` to count active SSH connections during deployment
- Checking that connections are closed **during** the loop, not all at the end

### Regression Testing

Ensure the fix doesn't break existing functionality:
- Deploy still works with 1, 2, and N nodes
- Error handling still works correctly (connection failures, deployment failures)
- Access control features still work (if enabled)
- Route synchronization still works

## Estimated Complexity

**low**

**Reasoning:**
- Single function modification
- 2 lines of code changes (remove defer, add explicit close)
- No changes to public API
- No changes to data structures or logic
- Clear test path using existing infrastructure
- Similar fix pattern already exists in `detectEndpoints()` for reference
- Low risk of introducing bugs

**Estimated Time:** 15-30 minutes including testing

## Additional Notes

### Similar Patterns in the Codebase

This issue highlights a common Go pitfall with `defer` in loops. The codebase should be audited for similar patterns:

```bash
# Find other potential defer-in-loop issues
grep -n "defer.*Close()" pkg/**/*.go | grep -B 5 "for.*range"
```

### Go Best Practices

From the Go FAQ and common guidelines:
- `defer` executes when the surrounding **function** returns, not block/loop scope
- For resources in loops, either:
  1. Call cleanup explicitly at end of iteration (if error handling is simple)
  2. Wrap in anonymous function (if you want defer semantics)
  3. Extract to a separate function with defer

### SSH Connection Lifecycle

Understanding the SSH client lifecycle:
- `ssh.NewClient()` establishes a new TCP connection and SSH handshake
- Each connection consumes: file descriptor, memory, SSH server slot
- Proper cleanup releases these resources immediately
- Delayed cleanup risks exhausting limits on client or server

### No Breaking Changes

This fix:
- ✅ Maintains the same public API (`Deploy() error`)
- ✅ Maintains the same behavior (sequential deployment)
- ✅ Maintains the same error handling (fail-fast on first error)
- ✅ Only changes **when** resources are cleaned up (sooner, not later)
