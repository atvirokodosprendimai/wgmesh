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

### Option B (Recommended): Use `log/slog`

**Rationale:**
- Go 1.23 is already in use (per `go.mod`)
- `log/slog` is stdlib (no new dependencies)
- Built-in level filtering (Debug, Info, Warn, Error)
- Structured logging with context fields
- Consistent with modern Go practices
- Zero-allocation in critical paths

**Implementation Steps:**

1. **Create logging wrapper in `pkg/daemon/logger.go`**
   - Initialize `slog.Logger` with configured level
   - Provide helper methods that map to current usage patterns
   - Support the existing tag-based format (e.g., `[DHT]`, `[Gossip]`)

2. **Update `pkg/daemon/config.go`**
   - Convert LogLevel string to `slog.Level`
   - Validate log level (debug, info, warn, error)
   - Initialize logger in `NewConfig()`

3. **Replace `log.Printf` calls systematically**
   - Map current patterns to appropriate levels:
     - `[DHT] SUCCESS!...` → Info
     - `[Exchange] Received...` → Debug
     - `Failed to...` patterns → Warn or Error
     - Status messages → Info
   - Preserve existing message formats
   - Keep tags for categorization

4. **Update daemon startup**
   - Pass logger to components that need it
   - Use `slog.SetDefault()` for global access (minimal invasiveness)

5. **Update documentation**
   - Document valid log levels in `README.md`
   - Update `--log-level` flag help text
   - Add example in quick start guide

### Alternative: Option A (Helper Functions)

If avoiding `slog` is desired:
- Define `logDebug()`, `logInfo()`, `logWarn()`, `logError()` helpers
- Check `Config.LogLevel` before calling `log.Printf`
- More invasive (requires passing config everywhere)
- Less performant (string comparison per log call)

## Affected Files

### Code Changes
- **`pkg/daemon/logger.go`** (new): slog wrapper with level filtering
- **`pkg/daemon/config.go`**: Parse LogLevel string to slog.Level
- **`pkg/daemon/daemon.go`**: Initialize and use structured logger
- **`pkg/daemon/cache.go`**: Replace 4 log.Printf calls
- **`pkg/daemon/collision.go`**: Replace 4 log.Printf calls
- **`pkg/daemon/epoch.go`**: Replace 1 log.Printf call
- **`pkg/daemon/helpers.go`**: Replace 3 log.Printf calls
- **`pkg/daemon/executor.go`**: Replace log.Printf calls
- **`pkg/discovery/dht.go`**: Replace ~24 log.Printf calls
- **`pkg/discovery/lan.go`**: Replace ~9 log.Printf calls
- **`pkg/discovery/registry.go`**: Replace ~9 log.Printf calls
- **`pkg/discovery/gossip.go`**: Replace ~8 log.Printf calls
- **`pkg/discovery/exchange.go`**: Replace ~8 log.Printf calls
- **`pkg/privacy/dandelion.go`**: Replace ~5 log.Printf calls

Total: ~11 files, ~105 log call sites

### Documentation Changes
- **`README.md`**: Update `--log-level` flag documentation, add examples
- **`main.go`**: Update flag help text to list valid levels

### Test Changes
- **`pkg/daemon/logger_test.go`** (new): Test log level filtering
- **`pkg/daemon/config_test.go`**: Test LogLevel parsing and validation

## Test Strategy

### Unit Tests
1. **Log level parsing:**
   - Valid levels: "debug", "info", "warn", "error"
   - Case insensitivity: "DEBUG", "Info", "WaRn"
   - Invalid levels: return error or default to "info"
   - Empty string: defaults to "info"

2. **Log filtering:**
   - Set level to "error", verify debug/info/warn don't print
   - Set level to "debug", verify all messages print
   - Set level to "info", verify debug doesn't print but info/warn/error do

### Integration Tests
1. **CLI flag:**
   - `wgmesh join --secret <SECRET> --log-level debug`: Verify verbose output
   - `wgmesh join --secret <SECRET> --log-level error`: Verify minimal output
   - Default (no flag): Verify "info" level output

2. **Log output verification:**
   - Run with `--log-level error`, confirm no informational messages
   - Run with `--log-level debug`, confirm debug messages appear

### Manual Testing
1. Start daemon with different log levels
2. Verify appropriate filtering occurs
3. Ensure critical errors always appear regardless of level
4. Check that log format remains consistent with existing output

## Estimated Complexity

**Medium** (6-8 hours of work)

**Breakdown:**
- Create logger wrapper: 1 hour
- Update config parsing: 30 minutes  
- Replace log.Printf calls: 3-4 hours (105 call sites across 11 files)
- Write tests: 1 hour
- Update documentation: 30 minutes
- Testing and validation: 1 hour

**Challenges:**
- Large number of log call sites (105) requires careful, systematic replacement
- Need to map informal log patterns to appropriate levels
- Must preserve existing log message formats for compatibility
- Testing across all discovery layers (DHT, LAN, Registry, Gossip)

**Risk Mitigation:**
- Use `slog.SetDefault()` to minimize invasiveness
- Replace calls incrementally, package by package
- Maintain existing message formats
- Add tests before making changes
- Verify no behavioral changes beyond log filtering
