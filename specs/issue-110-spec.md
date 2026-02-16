# Specification: Issue #110

## Classification
fix

## Deliverables
code

## Problem Analysis

The `Config.LogLevel` field is set during daemon initialization (in `pkg/daemon/config.go:64-67`) but is never read or used anywhere in the codebase. All 105+ `log.Printf` calls across the entire project print unconditionally, regardless of the configured log level.

This makes the `--log-level` CLI flag completely non-functional:
- Users cannot reduce log verbosity by setting a higher level (e.g., `--log-level=error`)
- Users cannot increase log verbosity for debugging (e.g., `--log-level=debug`)
- All log output is unconditionally printed, creating noise in production environments

**Evidence from code:**
```go
// pkg/daemon/config.go:64-67
logLevel := opts.LogLevel
if logLevel == "" {
    logLevel = "info"
}
// ... assigned to config.LogLevel but never checked
```

A search for `LogLevel` usage shows it is only written to `config.LogLevel`, never read. All `log.Printf` calls throughout the codebase are unconditional.

## Proposed Approach

**Scope limitation (Phase 1):** This specification addresses the daemon package only (~38 log.Printf calls), not the entire codebase (~105 calls). A full migration to `log/slog` or refactoring all packages is out of scope for this P2 issue.

**Phase 2 consideration:** If a project-wide slog migration is desired for the remaining ~75 calls in discovery/crypto/wireguard packages, that should be filed as a separate issue with its own discussion and planning.

### Implementation Strategy

Implement a simple level-filtering wrapper using the existing stdlib `log.Printf` infrastructure:

1. **Create `pkg/daemon/logger.go`** with helper functions that check `Config.LogLevel` before calling `log.Printf`:
   - `logDebug(config *Config, format string, v ...interface{})`
   - `logInfo(config *Config, format string, v ...interface{})`
   - `logWarn(config *Config, format string, v ...interface{})`
   - `logError(config *Config, format string, v ...interface{})`

2. **Define log level hierarchy:**
   - `debug` (level 0): Show all messages (debug, info, warn, error)
   - `info` (level 1): Show info, warn, error (default)
   - `warn` (level 2): Show warn, error
   - `error` (level 3): Show error only

3. **Preserve existing logging convention:**
   - Keep stdlib `log.Printf` as the underlying mechanism
   - Keep existing `[Component]` prefix tags (e.g., `[Cache]`, `[Collision]`, `[Daemon]`)
   - Maintain current output format

4. **Migrate daemon package calls:**
   - Replace ~38 `log.Printf` calls in `pkg/daemon/*.go` with appropriate level helpers
   - Categorize each call by analyzing its content and context:
     - **Debug**: Verbose operational details, state transitions
     - **Info**: Normal operational messages, startup/shutdown
     - **Warn**: Recoverable errors, degraded functionality
     - **Error**: Critical failures, unrecoverable errors

### Implementation Details

**Logger helper functions** (`pkg/daemon/logger.go`):
```go
package daemon

import "log"

// Log level constants
const (
    LogLevelDebug = "debug"
    LogLevelInfo  = "info"
    LogLevelWarn  = "warn"
    LogLevelError = "error"
)

// shouldLog returns true if the message level should be printed
func shouldLog(configLevel, msgLevel string) bool {
    levels := map[string]int{
        LogLevelDebug: 0,
        LogLevelInfo:  1,
        LogLevelWarn:  2,
        LogLevelError: 3,
    }
    
    configLvl, ok := levels[configLevel]
    if !ok {
        configLvl = levels[LogLevelInfo] // default to info
    }
    
    msgLvl := levels[msgLevel]
    return msgLvl >= configLvl
}

func logDebug(cfg *Config, format string, v ...interface{}) {
    if shouldLog(cfg.LogLevel, LogLevelDebug) {
        log.Printf(format, v...)
    }
}

func logInfo(cfg *Config, format string, v ...interface{}) {
    if shouldLog(cfg.LogLevel, LogLevelInfo) {
        log.Printf(format, v...)
    }
}

func logWarn(cfg *Config, format string, v ...interface{}) {
    if shouldLog(cfg.LogLevel, LogLevelWarn) {
        log.Printf(format, v...)
    }
}

func logError(cfg *Config, format string, v ...interface{}) {
    if shouldLog(cfg.LogLevel, LogLevelError) {
        log.Printf(format, v...)
    }
}
```

**Config access pattern:**
- Daemon struct already has `config *Config` field
- Pass `d.config` to log helper functions
- For standalone functions, pass config as parameter

**Migration examples:**

Current code:
```go
log.Printf("[Cache] Restored %d peers from cache", restored)
```

Migrated (info level):
```go
logInfo(d.config, "[Cache] Restored %d peers from cache", restored)
```

Current code:
```go
log.Printf("[Collision] Failed to update interface address: %v", err)
```

Migrated (error level):
```go
logError(d.config, "[Collision] Failed to update interface address: %v", err)
```

## Affected Files

### Code Changes Required

1. **`pkg/daemon/logger.go`** (new file):
   - Create log helper functions with level filtering
   - Define log level constants and hierarchy

2. **`pkg/daemon/cache.go`** (~5 log.Printf calls):
   - Migrate to logInfo/logError helpers

3. **`pkg/daemon/collision.go`** (~6 log.Printf calls):
   - Migrate to logDebug/logInfo/logError helpers

4. **`pkg/daemon/daemon.go`** (~15 log.Printf calls):
   - Migrate to logDebug/logInfo/logWarn/logError helpers

5. **`pkg/daemon/routes.go`** (estimated ~5 log.Printf calls):
   - Migrate to appropriate level helpers

6. **`pkg/daemon/epoch.go`** (estimated ~3 log.Printf calls):
   - Migrate to appropriate level helpers

7. **`pkg/daemon/executor.go`** (estimated ~4 log.Printf calls):
   - Migrate to appropriate level helpers

**Note:** Exact counts will be verified during implementation by searching `grep -n "log\.Printf" pkg/daemon/*.go`.

## Test Strategy

### Unit Tests

1. **Test `pkg/daemon/logger.go`** (`pkg/daemon/logger_test.go`):
   - Test `shouldLog()` function for all level combinations
   - Verify debug messages are filtered at info/warn/error levels
   - Verify info messages are shown at debug/info levels
   - Verify warn messages are shown at debug/info/warn levels
   - Verify error messages are always shown
   - Test invalid log level defaults to "info"

2. **Existing daemon tests**:
   - Run existing `pkg/daemon/*_test.go` tests to ensure no regressions
   - Verify daemon functionality is unchanged

### Integration Testing

1. **Manual CLI testing:**
   ```bash
   # Test debug level (all messages)
   wgmesh daemon --log-level=debug --secret=test
   
   # Test info level (no debug messages)
   wgmesh daemon --log-level=info --secret=test
   
   # Test warn level (only warnings and errors)
   wgmesh daemon --log-level=warn --secret=test
   
   # Test error level (only errors)
   wgmesh daemon --log-level=error --secret=test
   ```

2. **Verify output filtering:**
   - At `debug` level: Should see debug, info, warn, error messages
   - At `info` level: Should NOT see debug messages
   - At `warn` level: Should only see warn and error messages
   - At `error` level: Should only see error messages

3. **Verify default behavior:**
   - Without `--log-level` flag: Should default to "info"
   - Should match previous behavior (show info and above)

### Categorization Verification

Review categorization of each log call to ensure:
- **Debug** is used for verbose operational details users don't need in normal operation
- **Info** is used for normal operational messages (startup, peer discovery)
- **Warn** is used for recoverable errors or degraded states
- **Error** is used for critical failures that indicate problems

### Regression Testing

1. Run full test suite: `make test`
2. Verify no existing tests are broken by the changes
3. Verify daemon startup/shutdown behavior is unchanged
4. Verify peer discovery and mesh operations still log appropriately

## Estimated Complexity

**medium** (2-3 hours)

- Creating logger.go with helper functions: 30 minutes
- Writing unit tests for logger.go: 30 minutes
- Categorizing and migrating ~38 log.Printf calls: 60-90 minutes
- Manual testing with different log levels: 20-30 minutes
- Documentation review and updates: 10-20 minutes

The complexity is medium because:
- Need to analyze each log call to determine appropriate level
- Need to ensure config is accessible in all contexts
- Need to verify no behavioral changes beyond filtering
- Relatively straightforward implementation (no complex refactoring)
