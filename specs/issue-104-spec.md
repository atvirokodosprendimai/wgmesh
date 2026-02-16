# Specification: Issue #104

## Classification
fix

## Deliverables
code

## Problem Analysis

The daemon's `Run()` and `RunWithDHTDiscovery()` methods launch background goroutines but do not wait for them to complete during shutdown. When the daemon receives SIGINT/SIGTERM, it calls `d.cancel()` and returns immediately, potentially leaving goroutines mid-operation.

### Current Behavior

**In `Run()` (lines 78-125):**
- Line 108: Launches `go d.reconcileLoop()` 
- Line 111: Launches `go d.statusLoop()`
- Lines 117-121: Wait for shutdown signal
- Line 123: Call `d.cancel()` to cancel context
- Line 124: **Return immediately** without waiting

**In `RunWithDHTDiscovery()` (lines 330-411):**
- Line 353: Launches `go StartCacheSaver(...)` (has stopCh mechanism)
- Line 394: Launches `go d.reconcileLoop()`
- Line 397: Launches `go d.statusLoop()`
- Lines 402-407: Wait for shutdown signal
- Line 409: Call `d.cancel()` to cancel context
- Line 410: **Return immediately** without waiting

### Why This Is Dangerous

The `reconcileLoop()` (lines 221-233) performs critical operations:
- Line 230: Calls `d.reconcile()` which writes WireGuard configuration
- Lines 245-247: Configures peers via `d.configurePeer(peer)` 
- Lines 250-252: Syncs peer routes via `d.syncPeerRoutes(peers)`
- Lines 260-262: Removes stale peers via `d.removePeer(pubKey)`

If the context is cancelled while `reconcile()` is executing, WireGuard configuration could be left in an inconsistent state (e.g., peer partially added, routes not synced).

The `statusLoop()` is less critical but should still complete cleanly.

### Current Goroutine Lifecycle

Both `reconcileLoop()` and `statusLoop()` check `d.ctx.Done()` in their select statements, so they **will** eventually exit after `d.cancel()` is called. However:
1. They may be mid-iteration when cancellation occurs
2. No mechanism waits for them to complete before `Run()` returns
3. The main process may exit before goroutines finish cleanup

### Note on Other Components

- **DHT Discovery**: Already has `defer d.dhtDiscovery.Stop()` which blocks until stopped
- **EpochManager**: Already has `defer d.epochManager.Stop()` which closes stopCh
- **StartCacheSaver**: Already receives `d.cacheStopCh` and handles cleanup via `defer` close

The issue is specifically with `reconcileLoop()` and `statusLoop()`, which have no wait mechanism.

## Proposed Approach

Add `sync.WaitGroup` to the `Daemon` struct to track background goroutines and wait for them to complete during shutdown.

### Implementation Steps

1. **Add WaitGroup to Daemon struct** (around line 25-41 in `pkg/daemon/daemon.go`):
   ```go
   type Daemon struct {
       // ... existing fields ...
       wg     sync.WaitGroup
   }
   ```

2. **Wrap goroutine launches with WaitGroup** in both `Run()` and `RunWithDHTDiscovery()`:

   **In `Run()` (around lines 107-112):**
   ```go
   // Start reconciliation loop
   d.wg.Add(1)
   go func() {
       defer d.wg.Done()
       d.reconcileLoop()
   }()
   
   // Start status printer
   d.wg.Add(1)
   go func() {
       defer d.wg.Done()
       d.statusLoop()
   }()
   ```

   **In `RunWithDHTDiscovery()` (around lines 393-398):**
   ```go
   // Start reconciliation loop
   d.wg.Add(1)
   go func() {
       defer d.wg.Done()
       d.reconcileLoop()
   }()
   
   // Start status printer
   d.wg.Add(1)
   go func() {
       defer d.wg.Done()
       d.statusLoop()
   }()
   ```

3. **Wait for goroutines after cancellation**:

   **In `Run()` (after line 123):**
   ```go
   d.cancel()
   log.Printf("Waiting for background tasks to complete...")
   d.wg.Wait()
   log.Printf("Shutdown complete")
   return nil
   ```

   **In `RunWithDHTDiscovery()` (after line 409):**
   ```go
   d.cancel()
   log.Printf("Waiting for background tasks to complete...")
   d.wg.Wait()
   log.Printf("Shutdown complete")
   return nil
   ```

### Why This Works

1. `wg.Add(1)` increments the counter before launching each goroutine
2. `defer wg.Done()` in the wrapper function decrements when the goroutine exits
3. `wg.Wait()` blocks until all tracked goroutines have called `Done()`
4. The context cancellation (`d.cancel()`) signals goroutines to exit
5. The loops check `ctx.Done()` and return, triggering `defer wg.Done()`
6. Main function waits for all `Done()` calls before returning

### Alternative Considered: Channels

Could use done channels instead of WaitGroup:
```go
reconcileDone := make(chan struct{})
statusDone := make(chan struct{})
// ... launch goroutines that close channels on exit ...
<-reconcileDone
<-statusDone
```

**Rejected because:**
- WaitGroup is more idiomatic for waiting on N goroutines
- Scales better if more goroutines are added later
- Less channel management overhead
- Standard library pattern for this use case

## Affected Files

### Code Changes Required

1. **`pkg/daemon/daemon.go`**:
   - **Line ~40** (in `Daemon` struct): Add `wg sync.WaitGroup` field
   - **Lines 107-112** (in `Run()`): Wrap goroutine launches with `wg.Add(1)` and `defer wg.Done()`
   - **Lines 123-124** (in `Run()`): Add `wg.Wait()` after `d.cancel()` with logging
   - **Lines 393-398** (in `RunWithDHTDiscovery()`): Wrap goroutine launches with `wg.Add(1)` and `defer wg.Done()`
   - **Lines 409-410** (in `RunWithDHTDiscovery()`): Add `wg.Wait()` after `d.cancel()` with logging

**Total changes:** ~20 lines added/modified across 1 file

## Test Strategy

### Manual Testing

1. **Basic shutdown test**:
   ```bash
   # Start daemon
   sudo wgmesh daemon -secret test123 -interface wg0
   
   # Wait for "Daemon running" message
   # Send SIGINT (Ctrl+C)
   # Verify logs show:
   #   - "Received signal interrupt, shutting down..."
   #   - "Waiting for background tasks to complete..."
   #   - "Shutdown complete"
   # Verify process exits cleanly (no goroutine leaks)
   ```

2. **Shutdown during reconciliation**:
   ```bash
   # Modify reconcileLoop to add sleep:
   # case <-ticker.C:
   #     time.Sleep(3 * time.Second)  # Simulate slow reconcile
   #     d.reconcile()
   
   # Start daemon, wait ~2 seconds, send SIGINT
   # Verify it waits up to 3 seconds for reconcile to complete
   # Verify logs show completion messages
   ```

3. **DHT mode shutdown**:
   ```bash
   # Start daemon with DHT discovery
   sudo wgmesh daemon -secret test123 -interface wg0 -dht
   
   # Send SIGTERM
   # Verify all components shut down cleanly:
   #   - DHT discovery stops
   #   - Cache saver completes final save
   #   - Epoch manager stops (if privacy enabled)
   #   - Reconcile/status loops complete
   ```

### Automated Testing

Add a test in `pkg/daemon/daemon_test.go` (create if doesn't exist):

```go
func TestDaemonShutdownWaitsForGoroutines(t *testing.T) {
    // Create daemon with short intervals for testing
    config := &Config{
        InterfaceName: "wg-test",
        Secret:        "test-secret",
        // ... other config ...
    }
    
    daemon, err := NewDaemon(config)
    if err != nil {
        t.Fatalf("NewDaemon failed: %v", err)
    }
    
    // Start daemon in goroutine
    done := make(chan error)
    go func() {
        done <- daemon.Run()
    }()
    
    // Give goroutines time to start
    time.Sleep(100 * time.Millisecond)
    
    // Cancel context to trigger shutdown
    daemon.cancel()
    
    // Should complete without hanging
    select {
    case err := <-done:
        if err != nil {
            t.Errorf("Run() returned error: %v", err)
        }
    case <-time.After(5 * time.Second):
        t.Fatal("Daemon did not shut down within 5 seconds")
    }
}
```

### Race Detection

Run tests with race detector to ensure no data races during shutdown:
```bash
go test -race ./pkg/daemon/...
```

### Edge Cases to Test

1. **Multiple rapid shutdowns**: Ensure second shutdown doesn't cause panic
2. **Shutdown during startup**: Cancel context before goroutines fully start
3. **Context already cancelled**: Verify `wg.Wait()` completes immediately if loops already exited

### Success Criteria

- [ ] Logs show "Waiting for background tasks to complete..."
- [ ] Logs show "Shutdown complete"
- [ ] Process exits cleanly without hanging
- [ ] No goroutine leaks (verify with pprof if available)
- [ ] No race conditions detected with `-race` flag
- [ ] WireGuard configuration is not corrupted after shutdown

## Estimated Complexity

**low** (30-45 minutes)

- Simple addition of `sync.WaitGroup` to existing code
- Well-defined problem with straightforward solution
- Pattern is standard Go practice for goroutine coordination
- Minimal testing required (basic shutdown scenarios)
- No breaking changes to API or behavior
- Low risk of introducing new bugs
