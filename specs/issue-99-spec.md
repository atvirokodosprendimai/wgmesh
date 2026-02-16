# Specification: Issue #99

## Classification
fix

## Deliverables
code + documentation

## Problem Analysis

The daemon package has a well-designed `CommandExecutor` interface (defined in `pkg/daemon/executor.go`) that abstracts command execution for testability. This interface is already successfully used throughout `pkg/daemon/helpers.go` via the global `cmdExecutor` variable, which can be replaced with mock implementations during testing (as demonstrated in `pkg/daemon/helpers_test.go`).

However, two daemon package files bypass this abstraction and use `exec.Command` directly:

### Routes.go (4 direct calls)
1. **Line 44**: `exec.Command("ip", "route", "show", "dev", iface)` in `getCurrentRoutes()`
2. **Line 139**: `exec.Command("ip", "route", "del", ...)` in `applyRouteDiff()`
3. **Line 144**: `exec.Command("ip", "route", "replace", ...)` in `applyRouteDiff()`
4. **Line 150**: `exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1")` in `applyRouteDiff()`

### Systemd.go (7 direct calls)
1. **Line 127**: `exec.Command("systemctl", "daemon-reload")` in `InstallSystemdService()`
2. **Line 132**: `exec.Command("systemctl", "enable", "wgmesh.service")` in `InstallSystemdService()`
3. **Line 137**: `exec.Command("systemctl", "start", "wgmesh.service")` in `InstallSystemdService()`
4. **Line 147**: `exec.Command("systemctl", "stop", "wgmesh.service")` in `UninstallSystemdService()`
5. **Line 150**: `exec.Command("systemctl", "disable", "wgmesh.service")` in `UninstallSystemdService()`
6. **Line 169**: `exec.Command("systemctl", "daemon-reload")` in `UninstallSystemdService()`
7. **Line 176**: `exec.Command("systemctl", "is-active", "wgmesh.service")` in `ServiceStatus()`

### Impact

This inconsistency makes route management and systemd service management code untestable without:
- Running actual shell commands
- Requiring root/sudo permissions
- Requiring systemctl to be available
- Requiring a Linux environment with proper networking tools

The lack of tests for these functions increases the risk of bugs and makes refactoring difficult.

## Proposed Approach

### Phase 1: Refactor to use cmdExecutor

Replace all `exec.Command()` calls with `cmdExecutor.Command()` in both files. This is a mechanical refactoring that maintains identical runtime behavior while enabling testability.

#### routes.go changes:
```go
// Line 44 - getCurrentRoutes()
- cmd := exec.Command("ip", "route", "show", "dev", iface)
+ cmd := cmdExecutor.Command("ip", "route", "show", "dev", iface)

// Line 139 - applyRouteDiff()
- cmd := exec.Command("ip", "route", "del", route.Network, "via", route.Gateway, "dev", iface)
+ cmd := cmdExecutor.Command("ip", "route", "del", route.Network, "via", route.Gateway, "dev", iface)

// Line 144 - applyRouteDiff()
- cmd := exec.Command("ip", "route", "replace", route.Network, "via", route.Gateway, "dev", iface)
+ cmd := cmdExecutor.Command("ip", "route", "replace", route.Network, "via", route.Gateway, "dev", iface)

// Line 150 - applyRouteDiff()
- cmd := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1")
+ cmd := cmdExecutor.Command("sysctl", "-w", "net.ipv4.ip_forward=1")
```

#### systemd.go changes:
```go
// Line 127 - InstallSystemdService()
- if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
+ if err := cmdExecutor.Command("systemctl", "daemon-reload").Run(); err != nil {

// Line 132 - InstallSystemdService()
- if err := exec.Command("systemctl", "enable", "wgmesh.service").Run(); err != nil {
+ if err := cmdExecutor.Command("systemctl", "enable", "wgmesh.service").Run(); err != nil {

// Line 137 - InstallSystemdService()
- if err := exec.Command("systemctl", "start", "wgmesh.service").Run(); err != nil {
+ if err := cmdExecutor.Command("systemctl", "start", "wgmesh.service").Run(); err != nil {

// Line 147 - UninstallSystemdService()
- exec.Command("systemctl", "stop", "wgmesh.service").Run()
+ cmdExecutor.Command("systemctl", "stop", "wgmesh.service").Run()

// Line 150 - UninstallSystemdService()
- exec.Command("systemctl", "disable", "wgmesh.service").Run()
+ cmdExecutor.Command("systemctl", "disable", "wgmesh.service").Run()

// Line 169 - UninstallSystemdService()
- exec.Command("systemctl", "daemon-reload").Run()
+ cmdExecutor.Command("systemctl", "daemon-reload").Run()

// Line 176 - ServiceStatus()
- cmd := exec.Command("systemctl", "is-active", "wgmesh.service")
+ cmd := cmdExecutor.Command("systemctl", "is-active", "wgmesh.service")
```

### Phase 2: Add Unit Tests

Add comprehensive unit tests using mock executors following the pattern established in `helpers_test.go`:

#### routes_test.go (new file)
Create unit tests for:
1. `getCurrentRoutes()` - test parsing of `ip route show` output
2. `calculateRouteDiff()` - test route diffing logic (already pure, but needs tests)
3. `applyRouteDiff()` - test that correct commands are executed
4. `normalizeNetwork()` - test network CIDR normalization
5. Integration test for `syncPeerRoutes()` using mock executor

#### systemd_test.go (extend existing)
Add unit tests for:
1. `InstallSystemdService()` - mock systemctl commands, verify error handling
2. `UninstallSystemdService()` - mock systemctl commands, verify cleanup
3. `ServiceStatus()` - mock systemctl output, test various states

### Phase 3: Documentation

Update inline documentation to note that these functions use `cmdExecutor` for testability.

## Affected Files

### Code Changes
1. **`pkg/daemon/routes.go`** - Replace 4 `exec.Command()` calls with `cmdExecutor.Command()`
   - No import changes needed (exec import can remain for compatibility)
2. **`pkg/daemon/systemd.go`** - Replace 7 `exec.Command()` calls with `cmdExecutor.Command()`
   - No import changes needed

### New Test Files
1. **`pkg/daemon/routes_test.go`** (new) - Comprehensive tests for route management
   - Test `getCurrentRoutes()` with various `ip route show` outputs
   - Test `calculateRouteDiff()` with various current/desired states
   - Test `applyRouteDiff()` command execution
   - Test `normalizeNetwork()` edge cases
   - Integration test for `syncPeerRoutes()`

### Extended Test Files
1. **`pkg/daemon/systemd_test.go`** (extend) - Add command execution tests
   - Currently only tests `GenerateSystemdUnit()` (template generation)
   - Add tests for `InstallSystemdService()`, `UninstallSystemdService()`, `ServiceStatus()`

### Documentation (Optional)
1. **`pkg/daemon/CLAUDE.md`** - If exists, update to mention testing patterns
2. Inline godoc comments if needed

## Test Strategy

### Unit Tests

#### routes_test.go
```go
// Test getCurrentRoutes with mock output
func TestGetCurrentRoutes(t *testing.T) {
    tests := []struct {
        name           string
        mockOutput     string
        mockErr        error
        expectedRoutes []routeEntry
        expectError    bool
    }{
        {
            name: "parse multiple routes",
            mockOutput: "10.0.1.0/24 via 10.99.0.2\n10.0.2.0/24 via 10.99.0.3\n",
            expectedRoutes: []routeEntry{
                {Network: "10.0.1.0/24", Gateway: "10.99.0.2"},
                {Network: "10.0.2.0/24", Gateway: "10.99.0.3"},
            },
        },
        // ... more test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mock := &MockCommandExecutor{
                commandFunc: func(name string, args ...string) Command {
                    return &MockCommand{
                        outputFunc: func() ([]byte, error) {
                            return []byte(tt.mockOutput), tt.mockErr
                        },
                    }
                },
            }
            withMockExecutor(t, mock, func() {
                routes, err := getCurrentRoutes("wg0")
                // ... assertions
            })
        })
    }
}

// Test applyRouteDiff command execution
func TestApplyRouteDiff(t *testing.T) {
    var executedCommands []string
    
    mock := &MockCommandExecutor{
        commandFunc: func(name string, args ...string) Command {
            executedCommands = append(executedCommands, name+" "+strings.Join(args, " "))
            return &MockCommand{
                runFunc: func() error { return nil },
                combinedOutputFunc: func() ([]byte, error) { return []byte{}, nil },
            }
        },
    }
    
    withMockExecutor(t, mock, func() {
        toAdd := []routeEntry{{Network: "10.0.1.0/24", Gateway: "10.99.0.2"}}
        toRemove := []routeEntry{{Network: "10.0.2.0/24", Gateway: "10.99.0.3"}}
        err := applyRouteDiff("wg0", toAdd, toRemove)
        
        // Verify correct commands were executed
        // Should have: ip route del, ip route replace, sysctl
    })
}
```

#### systemd_test.go extensions
```go
func TestInstallSystemdService(t *testing.T) {
    var executedCommands []string
    
    mock := &MockCommandExecutor{
        commandFunc: func(name string, args ...string) Command {
            executedCommands = append(executedCommands, name+" "+strings.Join(args, " "))
            return &MockCommand{
                runFunc: func() error { return nil },
            }
        },
    }
    
    withMockExecutor(t, mock, func() {
        cfg := SystemdServiceConfig{
            Secret:     "test-secret",
            BinaryPath: "/usr/local/bin/wgmesh",
        }
        // Note: Will fail on file operations, but we can test command execution
        // May need to skip or mock file operations separately
    })
}
```

### Manual Testing

1. **Verify no behavior change**:
   - Run daemon in decentralized mode with route advertising
   - Verify routes are still added/removed correctly
   - Verify `wgmesh install-service` still works

2. **Run existing tests**:
   - `go test ./pkg/daemon/...` should pass
   - No regressions in existing helper tests

### Integration Testing

1. Test route synchronization with mock peers
2. Test systemd service installation on a test VM (manual)
3. Verify existing daemon functionality unchanged

### Code Coverage

- Target >80% coverage for new test code
- Use `go test -cover ./pkg/daemon/` to verify

## Estimated Complexity

**medium** (4-6 hours)

### Breakdown:
- **Phase 1 (Refactoring)**: 0.5 hours
  - Mechanical find-replace in 2 files
  - 11 one-line changes total
  - Low risk, high confidence
  
- **Phase 2 (Unit Tests)**: 3-4 hours
  - Create routes_test.go with comprehensive tests (~200-300 lines)
  - Extend systemd_test.go with command execution tests (~100-150 lines)
  - Handle edge cases (empty output, errors, permission issues)
  - Mock command executors for various scenarios
  
- **Phase 3 (Validation)**: 1-2 hours
  - Run tests and fix any issues
  - Manual testing of daemon with routes
  - Verify no regressions
  - Code review and cleanup

### Complexity Factors:
- **Low**: The refactoring itself is trivial (mechanical search-replace)
- **Medium**: Writing comprehensive tests requires understanding route parsing, systemd behavior
- **Medium**: systemd tests may be challenging due to file I/O dependencies in Install/Uninstall functions
- **Low**: No algorithm changes, pure refactoring for testability

### Dependencies:
- None - self-contained work in daemon package
- Existing `CommandExecutor` infrastructure is already in place
- Mock patterns are well-established in helpers_test.go

### Risk:
- **Very Low**: Changes are mechanical and don't alter behavior
- All existing functionality preserved
- Can be validated with simple before/after comparison
