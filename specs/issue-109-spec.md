# Specification: Issue #109

## Classification
feature

## Deliverables
code

## Problem Analysis

The daemon currently only handles shutdown signals (SIGINT, SIGTERM) and lacks a mechanism to reload configuration at runtime. This is evident in two locations:

1. **`pkg/daemon/daemon.go:105-106`** (Run method):
```go
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
// Only shutdown signals — no SIGHUP
```

2. **`pkg/daemon/daemon.go:390-391`** (RunWithDHTDiscovery method):
```go
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
```

When running as a systemd service, changing configuration options like `--advertise-routes` or `--gossip` requires:
1. Editing `/etc/systemd/system/wgmesh.service`
2. Running `systemctl daemon-reload`
3. Running `systemctl restart wgmesh`

This restart causes:
- Brief WireGuard interface downtime
- Loss of established DHT connections
- Temporary loss of peer discovery state (until cache is restored)
- Potential brief connectivity loss for routing traffic

### Current Configuration

The daemon has several runtime-changeable configuration options stored in the `Config` struct (`pkg/daemon/config.go`):
- `AdvertiseRoutes []string` - Networks to advertise to peers (line 25)
- `LogLevel string` - Log verbosity level (line 26)
- `Gossip bool` - Enable/disable in-mesh gossip discovery (line 28)

However, some configuration options should **NOT** be reloadable as they would break the mesh:
- `Secret` - Would change encryption keys and network ID
- `InterfaceName` - Would require recreating WireGuard interface
- `WGListenPort` - Would require rebinding WireGuard socket
- `Privacy` - Would require starting/stopping Dandelion++ epoch manager

### How Config Values Are Used

**AdvertiseRoutes:**
- Set in `LocalNode.RoutableNetworks` during `initLocalNode()` (line 136)
- Included in peer announcements sent to other nodes
- Used by peers to configure routing via this node
- Applied in reconciliation via `syncPeerRoutes()` (routes.go)

**LogLevel:**
- Currently stored but not actively used for filtering log output
- Would require integration with Go's log package or structured logging

**Gossip:**
- Checked once during DHT discovery startup (pkg/discovery/dht.go:98)
- If enabled, creates `MeshGossip` instance for in-mesh peer discovery
- Cannot be dynamically started/stopped after daemon initialization

## Proposed Approach

Add SIGHUP signal handling to allow runtime configuration reload without restarting the daemon or disrupting WireGuard connections.

### High-Level Strategy

1. **Add SIGHUP to signal handlers** in both `Run()` and `RunWithDHTDiscovery()` methods
2. **Re-read configuration source** when SIGHUP is received
3. **Apply only safe configuration changes** that don't break the mesh
4. **Log changes** for observability

### Detailed Implementation Steps

#### Step 1: Determine Configuration Source

The daemon needs a way to reload configuration. Two approaches:

**Option A: Re-parse systemd service file**
- Parse `/etc/systemd/system/wgmesh.service` to extract CLI flags
- Pros: Single source of truth
- Cons: Complex parsing, systemd-specific

**Option B: Configuration file** (RECOMMENDED)
- Store reloadable config in `/etc/wgmesh/config.env` or `/var/lib/wgmesh/<iface>.conf`
- Format: Simple key=value pairs
- Example:
```
ADVERTISE_ROUTES=192.168.0.0/24,10.0.0.0/8
LOG_LEVEL=debug
```
- Pros: Simple to parse, systemd-agnostic, follows Unix conventions
- Cons: Another file to manage

**Recommendation:** Use Option B with `/var/lib/wgmesh/<iface>.conf` since the daemon already uses this directory for state files.

#### Step 2: Add Configuration Reload Logic

In `pkg/daemon/daemon.go`, add a new method:

```go
// ReloadConfig re-reads configuration from disk and applies safe runtime changes
func (d *Daemon) ReloadConfig() error {
    // 1. Read config file from /var/lib/wgmesh/<iface>.conf
    // 2. Parse key=value pairs
    // 3. Validate new values
    // 4. Apply changes to d.config (with mutex protection)
    // 5. Update LocalNode.RoutableNetworks if advertise-routes changed
    // 6. Log what was changed
    // 7. Trigger immediate reconciliation to announce updated routes
}
```

Add a mutex to protect config reads/writes:
```go
type Daemon struct {
    config    *Config
    configMu  sync.RWMutex  // NEW: Protect config access
    // ... rest of fields
}
```

#### Step 3: Modify Signal Handling

In both `Run()` and `RunWithDHTDiscovery()`, change:

**Before:**
```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
```

**After:**
```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
```

Modify the select statement to handle SIGHUP:

**Before:**
```go
select {
case sig := <-sigCh:
    log.Printf("Received signal %v, shutting down...", sig)
case <-d.ctx.Done():
    log.Printf("Context cancelled, shutting down...")
}
```

**After:**
```go
for {
    select {
    case sig := <-sigCh:
        if sig == syscall.SIGHUP {
            log.Printf("Received SIGHUP, reloading configuration...")
            if err := d.ReloadConfig(); err != nil {
                log.Printf("Failed to reload config: %v", err)
            }
            continue
        }
        log.Printf("Received signal %v, shutting down...", sig)
        d.cancel()
        return nil
    case <-d.ctx.Done():
        log.Printf("Context cancelled, shutting down...")
        d.cancel()
        return nil
    }
}
```

#### Step 4: Create Config File During Service Installation

Modify `pkg/daemon/systemd.go` to create a config file alongside the service:

```go
func InstallSystemdService(cfg SystemdServiceConfig) error {
    // ... existing code ...
    
    // Write reloadable config file
    configPath := fmt.Sprintf("/var/lib/wgmesh/%s.conf", cfg.InterfaceName)
    configContent := generateConfigFile(cfg)
    if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
        return fmt.Errorf("failed to write config file: %w", err)
    }
    
    // ... rest of existing code ...
}

func generateConfigFile(cfg SystemdServiceConfig) string {
    var buf strings.Builder
    buf.WriteString("# wgmesh configuration - edit and send SIGHUP to reload\n")
    buf.WriteString(fmt.Sprintf("ADVERTISE_ROUTES=%s\n", strings.Join(cfg.AdvertiseRoutes, ",")))
    buf.WriteString("LOG_LEVEL=info\n")
    return buf.String()
}
```

#### Step 5: Apply Configuration Changes

When `AdvertiseRoutes` changes:
1. Update `d.config.AdvertiseRoutes` (with configMu lock)
2. Update `d.localNode.RoutableNetworks`
3. Trigger immediate reconciliation to update peer announcements
4. Next announcement broadcast will include new routes

When `LogLevel` changes:
1. Update `d.config.LogLevel`
2. If log level filtering is implemented, apply the new level

**Note:** `Gossip` cannot be reloaded because:
- Gossip is initialized once during DHT discovery startup
- Stopping/starting gossip would require stopping/starting the entire discovery layer
- Not safe to reload at runtime

### Scope Limitations

**Reloadable options:**
- ✅ `advertise-routes` - Safe to change, affects routing announcements
- ✅ `log-level` - Safe to change, affects logging verbosity

**Non-reloadable options (ignore during reload):**
- ❌ `secret` - Would break encryption and mesh identity
- ❌ `interface` - Would require recreating WireGuard interface
- ❌ `listen-port` - Would require rebinding sockets
- ❌ `privacy` - Would require epoch manager lifecycle changes
- ❌ `gossip` - Would require discovery layer restart

If a user changes a non-reloadable option, log a warning that it requires a full restart:
```
WARN: 'gossip' changed in config file but requires full daemon restart to take effect
```

### Error Handling

- If config file doesn't exist: Log warning, continue with current config
- If config file is malformed: Log error, keep current config
- If a value is invalid: Log error, keep current value
- If SIGHUP received but reload fails: Continue running with old config

## Affected Files

### Core Implementation

1. **`pkg/daemon/config.go`** (~40 new lines)
   - Add `LoadConfigFile(path string) (map[string]string, error)` function
   - Add `ParseAdvertiseRoutes(value string) ([]string, error)` helper
   - Add config file path helper: `GetConfigPath(interfaceName string) string`

2. **`pkg/daemon/daemon.go`** (~80 new lines, ~10 modified)
   - Line 26: Add `configMu sync.RWMutex` field to Daemon struct
   - Line 105-125: Modify `Run()` signal handling (add SIGHUP, change select to loop)
   - Line 390-410: Modify `RunWithDHTDiscovery()` signal handling (same changes)
   - Add `ReloadConfig()` method (~50 lines)
   - Add `applyConfigChanges(oldCfg, newCfg *Config)` helper (~30 lines)

3. **`pkg/daemon/systemd.go`** (~30 new lines)
   - Line 98-144: Modify `InstallSystemdService()` to create config file
   - Add `generateConfigFile(cfg SystemdServiceConfig) string` function (~20 lines)
   - Update service template to document SIGHUP reload

### Documentation

4. **`README.md`**
   - Add section on configuration reload with SIGHUP
   - Document which options are reloadable
   - Add example: `sudo systemctl reload wgmesh` or `sudo kill -HUP $(pidof wgmesh)`

5. **`CONTRIBUTING.md`** (optional)
   - Document config reload mechanism for contributors

### Testing

6. **`pkg/daemon/config_test.go`** (new file, ~100 lines)
   - Test `LoadConfigFile()` with valid config
   - Test with malformed config
   - Test with missing config file
   - Test `ParseAdvertiseRoutes()` with various inputs

7. **`pkg/daemon/daemon_test.go`** (new tests, ~150 lines)
   - Test `ReloadConfig()` with changed advertise-routes
   - Test `ReloadConfig()` with changed log-level
   - Test `ReloadConfig()` with non-existent config file
   - Test signal handling with SIGHUP (may require process spawning)

## Test Strategy

### Unit Tests

1. **Config parsing tests** (`pkg/daemon/config_test.go`):
   ```go
   func TestLoadConfigFile(t *testing.T) {
       tests := []struct {
           name     string
           content  string
           wantErr  bool
           expected map[string]string
       }{
           {"valid config", "ADVERTISE_ROUTES=10.0.0.0/8\n", false, map[string]string{"ADVERTISE_ROUTES": "10.0.0.0/8"}},
           {"empty file", "", false, map[string]string{}},
           {"comment lines", "# comment\nLOG_LEVEL=debug\n", false, map[string]string{"LOG_LEVEL": "debug"}},
       }
       // ... test implementation
   }
   ```

2. **Reload logic tests** (`pkg/daemon/daemon_test.go`):
   - Mock config file with test values
   - Call `ReloadConfig()`
   - Verify `d.config` updated correctly
   - Verify `d.localNode.RoutableNetworks` updated

### Integration Tests

1. **Manual systemd test**:
   ```bash
   # Install service with initial routes
   sudo ./wgmesh install-service --secret test --advertise-routes 10.0.0.0/8
   
   # Verify service running
   systemctl status wgmesh
   
   # Edit config file
   sudo vi /var/lib/wgmesh/wg0.conf
   # Change: ADVERTISE_ROUTES=192.168.0.0/24
   
   # Reload config
   sudo systemctl reload wgmesh  # or: sudo kill -HUP $(pidof wgmesh)
   
   # Verify in logs that config was reloaded
   journalctl -u wgmesh -f
   # Should see: "Received SIGHUP, reloading configuration..."
   # Should see: "Config reloaded: advertise-routes changed from [10.0.0.0/8] to [192.168.0.0/24]"
   ```

2. **Multi-node mesh test**:
   - Start node A with route 10.0.0.0/8
   - Start node B
   - Verify node B sees route via node A
   - Send SIGHUP to node A with updated routes (192.168.0.0/24)
   - Verify node B receives updated routes without connectivity loss
   - Confirm WireGuard interface stayed up throughout

### Edge Cases

- Send SIGHUP during startup (should be harmless)
- Send multiple SIGHUP signals rapidly (should queue safely)
- Config file deleted after daemon started (log warning, keep current config)
- Config file with very long route list (test parsing limits)
- Send SIGHUP with no actual config changes (should be no-op)

### Backward Compatibility

- Daemon without config file should start normally (use CLI flags)
- Existing daemons without SIGHUP support can be upgraded without restart
- Old service files without config file continue to work

## Estimated Complexity

**Medium** (4-6 hours)

### Breakdown

**Implementation (3-4 hours):**
- Config file parsing: 1 hour
- SIGHUP signal handling: 1 hour
- ReloadConfig() logic and synchronization: 1 hour
- Systemd service integration: 30 minutes
- Testing and debugging: 30-60 minutes

**Testing (1-2 hours):**
- Unit tests: 45 minutes
- Integration tests (manual): 45 minutes
- Edge case testing: 30 minutes

**Documentation:**
- README updates: 15 minutes
- Code comments: 15 minutes

### Risk Factors

**Low Risk:**
- Config file parsing is simple key=value format
- SIGHUP is standard Unix signal handling
- No protocol changes required
- Fails gracefully (keeps old config on error)
- No WireGuard interface changes

**Potential Challenges:**
- Race conditions between config reload and reconciliation loop (mitigated by mutex)
- Testing signal handling (may require subprocess spawning)
- Ensuring systemd service file generation includes config file

### Alternative Approaches Considered

**Alternative 1: Use systemd's `ExecReload=` directive**
- Add `ExecReload=/bin/kill -HUP $MAINPID` to service template
- Pros: Standard systemd pattern
- Cons: Still requires implementing SIGHUP handling
- Decision: Include this as part of the solution

**Alternative 2: Watch config file for changes (inotify)**
- Automatically reload when config file changes
- Pros: No manual signal needed
- Cons: More complex, potential for rapid reload loops
- Decision: Not recommended, SIGHUP is simpler and more predictable

**Alternative 3: Reload all config from CLI flags**
- Re-parse systemd service file or CLI arguments
- Pros: Single source of truth
- Cons: Complex systemd file parsing, harder to edit
- Decision: Config file approach is simpler

## Notes

- This implementation follows the Unix convention of SIGHUP for configuration reload
- The config file approach is used by many daemons (e.g., nginx, haproxy)
- Users can reload with either `systemctl reload wgmesh` or `kill -HUP <pid>`
- Future enhancement: Add more reloadable options as needed
- Future enhancement: Add structured logging with configurable log levels
