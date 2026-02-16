# Specification: Issue #91

## Classification
fix

## Deliverables
code + documentation

## Problem Analysis

On macOS, when users run `wgmesh join --secret <SECRET>`, they encounter an error:

```
ERROR: (wg0) 2026/02/16 19:45:03 Failed to create TUN device: Interface name must be utun[0-9]*
Daemon error: failed to setup WireGuard: failed to create interface: wireguard interface wg0 was not created on macOS
```

### Root Cause

The issue stems from a fundamental difference between Linux and macOS WireGuard implementations:

1. **Linux**: WireGuard is integrated into the kernel. Interface names can be arbitrary (e.g., `wg0`, `wg1`, `wireguard-mesh`)
   
2. **macOS**: WireGuard uses the userspace `wireguard-go` implementation, which creates TUN devices. macOS enforces that TUN device names **must** follow the pattern `utun[0-9]+` (e.g., `utun0`, `utun20`, `utun99`)

Currently, wgmesh uses a hardcoded default interface name of `"wg0"` across all platforms:

- **Code location**: `pkg/daemon/config.go:16` defines `DefaultInterface = "wg0"`
- **Join command**: `main.go:238` sets flag default: `iface := fs.String("interface", "wg0", "WireGuard interface name")`
- **Install-service command**: `main.go:489` sets flag default: `iface := fs.String("interface", "wg0", "WireGuard interface name")`

When `createInterface()` in `pkg/daemon/helpers.go:88-130` calls `wireguard-go wg0` on macOS, the `wireguard-go` binary itself rejects the interface name and returns the error.

### Why This Wasn't Caught in Testing

The test suite in `pkg/daemon/helpers_test.go` correctly uses `utun0` and `utun99` for macOS test cases, but:
- Tests mock the `wireguard-go` command execution
- Real-world users hit the actual `wireguard-go` binary which enforces the naming constraint
- The default value in production code (`wg0`) was never tested on real macOS hardware

### Impact

- macOS users cannot use the default `join` command without explicitly specifying `--interface utunX`
- Poor user experience: fails immediately with a cryptic error
- Documentation doesn't mention the macOS interface naming requirement
- The error message from `wireguard-go` is not user-friendly

## Proposed Approach

Implement platform-specific default interface names that align with each operating system's requirements and conventions.

### Implementation Strategy

1. **Make default interface OS-aware**:
   - Linux: Keep `wg0` (works universally)
   - macOS: Use `utun20` (high enough to avoid conflicts with system-assigned utun0-utun9)
   
2. **Update default in multiple locations**:
   - `pkg/daemon/config.go`: Make `DefaultInterface` a function that returns OS-specific values
   - `main.go` join command: Call the function for the default flag value
   - `main.go` install-service command: Call the function for the default flag value

3. **Add input validation** (optional but recommended):
   - Validate that on macOS, user-provided interface names match `utun[0-9]+` pattern
   - Provide helpful error message if validation fails: "On macOS, interface name must be utun[0-9]+ (e.g., utun20)"

4. **Update documentation**:
   - README.md: Add note about OS-specific defaults
   - Add macOS-specific examples showing `--interface` flag usage
   - Document the `utun[0-9]+` naming requirement for macOS users

5. **Update tests**:
   - Ensure test coverage for OS-specific defaults
   - Add tests for macOS interface name validation

### Alternative Approaches Considered

**Alternative 1**: Always require `--interface` flag
- ❌ Worse user experience
- ❌ Breaks existing Linux users who rely on `wg0` default

**Alternative 2**: Auto-detect next available utun interface on macOS
- ❌ More complex implementation
- ❌ Unpredictable interface names make debugging harder
- ❌ Requires parsing `ifconfig` output to find free interfaces

**Alternative 3**: Use `utun0` as macOS default
- ❌ May conflict with system-assigned interfaces
- ❌ `utun0` is often already in use by VPNs or system services

**Why `utun20` is the best choice**:
- ✅ High enough to avoid system interface conflicts (macOS typically uses utun0-utun9)
- ✅ Low enough to be memorable and easy to type
- ✅ Consistent with wireguard-go conventions seen in the wild
- ✅ Explicit and predictable

## Affected Files

### Code Changes Required

1. **`pkg/daemon/config.go`** (lines 12-17):
   - Change `DefaultInterface` from a constant to a function
   - Implement OS-specific logic using `runtime.GOOS`
   - Return `"wg0"` for Linux, `"utun20"` for Darwin

2. **`main.go`** (line 238):
   - Change join command default: `iface := fs.String("interface", daemon.DefaultInterface(), "WireGuard interface name")`
   - Or inline the OS check if not using a function

3. **`main.go`** (line 489):
   - Change install-service command default: `iface := fs.String("interface", daemon.DefaultInterface(), "WireGuard interface name")`

4. **`pkg/daemon/helpers.go`** (optional - validation):
   - Add `validateInterfaceName(name string) error` function (lines ~75-78)
   - Call from `createInterface()` before attempting creation
   - On Darwin, check name matches `^utun[0-9]+$` regex

### Documentation Changes Required

1. **`README.md`**:
   - Add section about platform-specific defaults
   - Update join command examples to show both Linux and macOS usage
   - Document the macOS `utun[0-9]+` requirement

2. **`docs/`** (if any daemon/join documentation exists):
   - Update with macOS-specific guidance

### Test Changes Required

1. **`pkg/daemon/config_test.go`**:
   - Add test for `DefaultInterface()` function
   - Test both Linux and macOS code paths (may need build tags or runtime.GOOS mocking)

2. **`pkg/daemon/helpers_test.go`**:
   - Add tests for interface name validation (if implemented)
   - Ensure existing macOS tests continue to pass

3. **`main_test.go`** (if command-line parsing tests exist):
   - Add test verifying OS-specific defaults are used

## Test Strategy

### Manual Testing

1. **Linux test** (existing behavior preserved):
   ```bash
   # Should still default to wg0
   ./wgmesh join --secret test-secret
   ip link show wg0  # Should exist
   ```

2. **macOS test** (new behavior):
   ```bash
   # Should default to utun20
   ./wgmesh join --secret test-secret
   ifconfig utun20  # Should exist
   
   # Explicit interface should still work
   ./wgmesh join --secret test-secret --interface utun99
   ifconfig utun99  # Should exist
   ```

3. **macOS validation test** (if validation added):
   ```bash
   # Should fail with helpful error
   ./wgmesh join --secret test-secret --interface wg0
   # Expected: "Error: On macOS, interface name must be utun[0-9]+ (e.g., utun20)"
   ```

### Automated Testing

1. **Unit tests**:
   - Test `DefaultInterface()` returns correct value per OS
   - Test interface name validation (if implemented)

2. **Integration tests**:
   - Mock `runtime.GOOS` if possible to test cross-platform behavior
   - Verify default values propagate through to daemon configuration

### Risk Assessment

- **Low risk**: Changes only affect default values, not core functionality
- **Backward compatibility**: 
  - Linux users: No change in behavior (still defaults to `wg0`)
  - macOS users: Currently broken, this fixes their experience
  - Users who explicitly specify `--interface`: No change
- **Migration**: None required - this fixes a bug, doesn't change existing working setups

## Estimated Complexity

**low** (2-3 hours)

- Simple conditional logic for OS-specific defaults
- Validation is optional regex check
- Main effort is in testing across platforms and updating documentation
- No architectural changes required
- Clear success criteria: macOS users can join without `--interface` flag
