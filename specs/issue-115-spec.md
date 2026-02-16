# Specification: Issue #115

## Classification
refactor

## Deliverables
code

## Problem Analysis

The codebase follows a consistent logging pattern where each component uses a `[Component]` prefix in its log messages to facilitate filtering and tracing. This pattern is used throughout the discovery and daemon packages:

| Package | Prefix | Consistent? | Evidence |
|---|---|---|---|
| `pkg/discovery/dht.go` | `[DHT]` | Yes | All log.Printf calls use `[DHT]` prefix |
| `pkg/discovery/gossip.go` | `[Gossip]` | Yes | Line 75, 103, etc. use `[Gossip]` prefix |
| `pkg/discovery/lan.go` | `[LAN]` | Yes | Line 86, 103, etc. use `[LAN]` prefix |
| `pkg/discovery/exchange.go` | `[Exchange]` | Yes | Lines 78, 131, 149, 153, etc. use `[Exchange]` prefix |
| `pkg/daemon/cache.go` | `[Cache]` | Yes | Line 89, 103, 105, etc. use `[Cache]` prefix |
| `pkg/daemon/collision.go` | `[Collision]` | Yes | All log.Printf calls use `[Collision]` prefix |
| **`pkg/daemon/daemon.go`** | **None** | **No** | 24 out of 25 log.Printf calls lack prefix |

In `pkg/daemon/daemon.go`, there are 25 total log.Printf calls:
- **24 calls** have no prefix (lines 79, 86, 87, 113, 118, 120, 158, 166, 174, 176, 196, 216, 246, 251, 261, 331, 338, 339, 340, 378, 386, 399, 404, 406)
- **1 call** uses `[Status]` prefix (line 307) — this should remain as is, since it's a status-specific message and follows the pattern in `printStatus()` method

This inconsistency makes it difficult to:
1. Filter daemon-specific log messages from other components
2. Trace daemon lifecycle events (startup, shutdown, configuration)
3. Debug daemon-related issues in production environments
4. Follow log output when multiple components are logging simultaneously

## Proposed Approach

Add the `[Daemon]` prefix to all log.Printf calls in `pkg/daemon/daemon.go` that currently lack a prefix, following the established pattern used in other packages.

### Implementation Steps

1. **Identify all unprefixed log.Printf calls** in `pkg/daemon/daemon.go`:
   - Lines: 79, 86, 87, 113, 118, 120, 158, 166, 174, 176, 196, 216, 246, 251, 261, 331, 338, 339, 340, 378, 386, 399, 404, 406

2. **Add `[Daemon]` prefix** to each call:
   - Change `log.Printf("message", ...)` to `log.Printf("[Daemon] message", ...)`
   - Maintain exact same formatting and parameters otherwise

3. **Preserve special cases**:
   - Line 307: Keep `[Status]` prefix as it's a specific status message from `printStatus()` method
   - Line 309: Keep existing format (it's within the status printer loop and shows peer details)

4. **Verify consistency**:
   - Ensure all daemon lifecycle events are prefixed: startup, initialization, configuration, shutdown
   - Ensure all daemon operations are prefixed: WireGuard setup, peer management, warnings

### Pattern Reference

Other packages show the consistent pattern to follow:

```go
// From pkg/discovery/exchange.go
log.Printf("[Exchange] Listening on UDP port %d", port)
log.Printf("[Exchange] SUCCESS! Received valid %s from wgmesh peer at %s", ...)

// From pkg/discovery/lan.go
log.Printf("[LAN] Multicast discovery started on %s", l.multicastAddr.String())

// From pkg/daemon/cache.go
log.Printf("[Cache] Failed to load peer cache: %v", err)
log.Printf("[Cache] Restored %d peers from cache", restored)
```

The pattern should be applied to daemon.go as:

```go
// Before
log.Printf("Starting wgmesh daemon...")

// After
log.Printf("[Daemon] Starting wgmesh daemon...")
```

## Affected Files

### Code Changes Required

1. **`pkg/daemon/daemon.go`**:
   - Lines to modify: 79, 86, 87, 113, 118, 120, 158, 166, 174, 176, 196, 216, 246, 251, 261, 331, 338, 339, 340, 378, 386, 399, 404, 406
   - Action: Add `[Daemon]` prefix to each log.Printf format string
   - Lines to preserve: 307, 309 (already have appropriate prefixes or formatting)

### Example Changes

```go
// Line 79
- log.Printf("Starting wgmesh daemon...")
+ log.Printf("[Daemon] Starting wgmesh daemon...")

// Line 86
- log.Printf("Local node: %s", d.localNode.WGPubKey[:16]+"...")
+ log.Printf("[Daemon] Local node: %s", d.localNode.WGPubKey[:16]+"...")

// Line 158
- log.Printf("Warning: failed to save local node state: %v", err)
+ log.Printf("[Daemon] Warning: failed to save local node state: %v", err)

// Line 166
- log.Printf("Setting up WireGuard interface %s...", d.config.InterfaceName)
+ log.Printf("[Daemon] Setting up WireGuard interface %s...", d.config.InterfaceName)

// Line 246
- log.Printf("Failed to configure peer %s: %v", peer.WGPubKey[:8]+"...", err)
+ log.Printf("[Daemon] Failed to configure peer %s: %v", peer.WGPubKey[:8]+"...", err)
```

## Test Strategy

### Manual Testing

1. **Daemon startup test**:
   - Run `wgmesh -join <secret>`
   - Verify all startup messages show `[Daemon]` prefix
   - Expected output:
     ```
     [Daemon] Starting wgmesh daemon with DHT discovery...
     [Daemon] Local node: abcd1234...
     [Daemon] Mesh IP: 10.99.x.x
     [Daemon] Network ID: ...
     ```

2. **WireGuard setup test**:
   - Run daemon and observe WireGuard configuration logs
   - Verify interface setup messages have `[Daemon]` prefix
   - Expected output:
     ```
     [Daemon] Setting up WireGuard interface wgmesh...
     [Daemon] Interface wgmesh exists, resetting...
     [Daemon] WireGuard interface wgmesh ready on port 51820
     ```

3. **Peer operations test**:
   - Let daemon discover peers
   - Verify peer configuration/removal messages have `[Daemon]` prefix
   - Expected output:
     ```
     [Daemon] Failed to configure peer abc123: <error>
     [Daemon] Failed to sync peer routes: <error>
     ```

4. **Status output test**:
   - Wait for status interval (30 seconds)
   - Verify status messages use `[Status]` prefix (unchanged)
   - Expected output:
     ```
     [Status] Active peers: 2
       - abc123... (10.99.x.x) via [dht]
     ```

5. **Shutdown test**:
   - Send SIGINT (Ctrl+C) to daemon
   - Verify shutdown messages have `[Daemon]` prefix
   - Expected output:
     ```
     [Daemon] Received signal interrupt, shutting down...
     ```

6. **Multi-component log filtering test**:
   - Run daemon with DHT, Gossip, and LAN discovery enabled
   - Use `grep "[Daemon]"` to filter only daemon messages
   - Verify clean separation from `[DHT]`, `[Gossip]`, `[LAN]`, `[Exchange]` messages

### Automated Testing

No new automated tests required for this refactor since:
- This is purely a logging format change
- No functional behavior changes
- No API changes
- Existing tests verify functionality, not log output format

### Verification Checklist

- [ ] All 24 unprefixed log.Printf calls in daemon.go now have `[Daemon]` prefix
- [ ] Existing `[Status]` prefix on line 307 is preserved
- [ ] No changes to log.Printf parameters beyond adding prefix
- [ ] Log output is easily filterable with `grep "[Daemon]"`
- [ ] Consistency with other packages' logging patterns (`[DHT]`, `[Gossip]`, `[LAN]`, `[Cache]`, etc.)

### Risk Assessment

- **Very low risk**: This is a pure cosmetic change to logging output
- **No functional changes**: Only modifying format strings, not logic
- **No API changes**: No changes to function signatures or behavior
- **Backward compatible**: Log output format is for human consumption, not programmatic parsing
- **Easy to review**: Each change is a simple string modification
- **Easy to revert**: If any issues arise, changes can be easily reverted

## Estimated Complexity

**low** (30 minutes - 1 hour)

- Simple string modification across 24 lines
- No logic changes required
- No test updates required
- No documentation updates required (logging format is internal)
- Straightforward verification through manual testing
- Follow established pattern used throughout the codebase
