# Specification: Issue #110

## Classification
fix

## Deliverables
code + documentation

## Problem Analysis

The `Config.LogLevel` field is set during daemon initialization but never used anywhere in the codebase, making the `--log-level` flag a no-op. The issue affects both centralized and decentralized modes.

### Current Behavior

**Configuration Flow:**
1. User specifies `--log-level` flag (e.g., `wgmesh join --secret <SECRET> --log-level debug`)
2. Flag is parsed in `main.go:239` and passed to `DaemonOpts.LogLevel`
3. Value is normalized to "info" if empty in `pkg/daemon/config.go:64-67`
4. Stored in `Config.LogLevel` but **never read or checked**

**Evidence:**
- `grep -r "LogLevel"` shows only 5 occurrences: all are assignments, no reads
- All 105 `log.Printf` calls across the codebase are unconditional
- No filtering logic exists based on log level

**Current Logging Pattern:**
```go
log.Printf("[DHT] Announcing to network ID...")        // Always prints
log.Printf("[Gossip] Received peer update...")         // Always prints
log.Printf("[Cache] Restored %d peers...", count)      // Always prints
```

**Tags Used:** `[DHT]`, `[Registry]`, `[LAN]`, `[Gossip]`, `[Exchange]`, `[Dandelion]`, `[Collision]`, `[Cache]`, `[Status]`, `[Epoch]`

### Impact

- Users cannot reduce log noise in production (all 105 log statements always print)
- Debugging mode is not achievable (no way to enable verbose logging)
- The `--log-level` flag creates false expectations
- Log output can be overwhelming during normal operation

## Proposed Approach

### Phase 1: Simple Level-Filtering Wrapper (This Issue)

**Scope:** Implement log level filtering for daemon package only (~38 calls)

**Rationale:**
- Maintains existing project convention: stdlib `log.Printf` with `[Component]` prefix tags
- Minimal scope for a P2 issue - no project-wide changes
- Quick implementation and testing
- No dependencies or architectural changes

**Implementation Steps:**

1. **Create `pkg/daemon/logger.go`** with helper functions:
   ```go
   // logDebug logs at DEBUG level
   func logDebug(config *Config, format string, v ...interface{})
   
   // logInfo logs at INFO level
   func logInfo(config *Config, format string, v ...interface{})
   
   // logWarn logs at WARN level
   func logWarn(config *Config, format string, v ...interface{})
   
   // logError logs at ERROR level
   func logError(config *Config, format string, v ...interface{})
   ```
   
   Each helper:
   - Checks `config.LogLevel` against the call level
   - Only calls `log.Printf` if level is enabled
   - Preserves existing message format and `[Component]` tags

2. **Add level comparison logic**
   - Parse `config.LogLevel` string (debug, info, warn, error)
   - Implement simple level hierarchy: debug < info < warn < error
   - Case-insensitive comparison

3. **Replace ~38 `log.Printf` calls in daemon package**
   - Map patterns to appropriate levels:
     - `[Cache] Restored...`, `[Collision] Mesh IP collision detected...` → Info
     - `wireguard-go stderr/stdout` → Debug
     - `Failed to...`, `Warning:...` → Warn or Error
     - Status and startup messages → Info
   - Keep existing message formats unchanged
   - Preserve all `[Component]` tags

4. **Update documentation**
   - Document valid log levels in `README.md`
   - Update `--log-level` flag help text
   - Add example showing debug vs info output

### Phase 2: Remaining Packages (Future Issue)

**Scope:** ~67 remaining calls in discovery, privacy, crypto, wireguard packages

This can be addressed in a separate issue if desired. At that point, consider:
- Whether to continue with helper functions
- Whether to migrate to `log/slog` for structured logging
- Needs separate discussion for project-wide logging convention changes

## Affected Files

### Code Changes (Phase 1 - Daemon Package Only)
- **`pkg/daemon/logger.go`** (new): Helper functions for level filtering
- **`pkg/daemon/config.go`**: Add level parsing/validation helper
- **`pkg/daemon/daemon.go`**: Replace ~18 log.Printf calls
- **`pkg/daemon/cache.go`**: Replace 4 log.Printf calls  
- **`pkg/daemon/collision.go`**: Replace 4 log.Printf calls
- **`pkg/daemon/epoch.go`**: Replace 1 log.Printf call
- **`pkg/daemon/helpers.go`**: Replace 3 log.Printf calls
- **`pkg/daemon/executor.go`**: Replace log.Printf calls (if any)
- **`pkg/daemon/routes.go`**: Replace log.Printf calls (if any)
- **`pkg/daemon/systemd.go`**: Replace log.Printf calls (if any)

**Total for Phase 1:** ~10 files, ~38 log call sites in daemon package

### Documentation Changes
- **`README.md`**: Update `--log-level` flag documentation, add examples
- **`main.go`**: Update flag help text to list valid levels (debug, info, warn, error)

### Test Changes
- **`pkg/daemon/logger_test.go`** (new): Test log level filtering and helper functions
- **`pkg/daemon/config_test.go`**: Test LogLevel parsing and validation (if needed)

### Files NOT Changed (Phase 2 - Future)
The following packages contain ~67 additional log.Printf calls but are **out of scope** for this issue:
- `pkg/discovery/*.go` (~53 calls: DHT, LAN, Registry, Gossip, Exchange)
- `pkg/privacy/*.go` (~5 calls: Dandelion)
- `pkg/crypto/*.go` (if any)
- `pkg/wireguard/*.go` (if any)
- `pkg/ssh/*.go` (if any)
- `pkg/mesh/*.go` (if any)

## Test Strategy

### Unit Tests

1. **Log level parsing (`pkg/daemon/config_test.go` or `logger_test.go`):**
   - Valid levels: "debug", "info", "warn", "error"
   - Case insensitivity: "DEBUG", "Info", "WaRn"
   - Invalid levels: return error or default to "info"
   - Empty string: defaults to "info"

2. **Log filtering (`pkg/daemon/logger_test.go`):**
   - Set level to "error", call logDebug/logInfo/logWarn → verify no output
   - Set level to "debug", call logDebug/logInfo/logWarn/logError → verify all print
   - Set level to "info", call logDebug → verify no output, others print
   - Set level to "warn", call logDebug/logInfo → verify no output, warn/error print

3. **Helper function behavior:**
   - Verify message format matches `log.Printf` exactly
   - Verify `[Component]` tags are preserved
   - Verify formatting with arguments works correctly

### Integration Tests

1. **CLI flag:**
   - `wgmesh join --secret <SECRET> --log-level debug`: Verify verbose output from daemon
   - `wgmesh join --secret <SECRET> --log-level error`: Verify minimal output from daemon
   - `wgmesh join --secret <SECRET>`: Verify "info" level output (default)

2. **Daemon package log output:**
   - Run daemon with `--log-level error`, confirm only error messages appear
   - Run daemon with `--log-level debug`, confirm debug messages appear
   - Verify non-daemon packages (discovery, etc.) still log unconditionally (Phase 2)

### Manual Testing

1. Start daemon with different log levels
2. Verify appropriate filtering for daemon package logs
3. Ensure critical errors always appear regardless of level
4. Check that log format remains consistent with existing output
5. Verify discovery/privacy package logs still appear (out of scope for Phase 1)

## Estimated Complexity

**Low-Medium** (3-4 hours of work for Phase 1)

**Breakdown:**
- Create logger helper functions: 45 minutes
- Add level parsing/validation: 30 minutes
- Replace ~38 log.Printf calls in daemon package: 1.5-2 hours
- Write unit tests: 45 minutes
- Update documentation: 15 minutes
- Testing and validation: 30 minutes

**Scope Reduction from Original:**
- Original estimate: 6-8 hours for all 105 calls across 11 files
- Phase 1 estimate: 3-4 hours for 38 calls in daemon package only
- Phase 2 (future): 3-4 hours for remaining 67 calls if pursued

**Challenges:**
- Need to carefully map existing log patterns to appropriate levels
- Must pass Config reference to helper functions (not globally accessible)
- Preserve existing log message formats for consistency
- Ensure level filtering logic is efficient (simple string comparison)

**Benefits of Phased Approach:**
- Smaller scope appropriate for P2 priority
- Maintains existing project logging convention
- Can evaluate effectiveness before expanding to other packages
- No architectural changes or dependency additions
- Easier to review and test

**Risk Mitigation:**
- Replace calls incrementally, file by file
- Maintain existing message formats exactly
- Add comprehensive tests before making changes
- Document which packages are in/out of scope
- Verify no behavioral changes beyond log filtering in daemon package
