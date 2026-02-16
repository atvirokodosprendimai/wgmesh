# Specification: Issue #108

## Classification
refactor

## Deliverables
code

## Problem Analysis

The current implementation has three independent context/cancellation issues that break proper context propagation:

### 1. DHTDiscovery Independent Context

**Location**: `pkg/discovery/dht.go:69`

```go
ctx, cancel := context.WithCancel(context.Background())
```

`DHTDiscovery` creates its own independent context from `context.Background()` rather than deriving from the daemon's context. This means:
- The DHT discovery layer won't automatically stop when the daemon's context is cancelled
- Cancellation depends on explicit `Stop()` calls via `defer` statements
- The context hierarchy is broken - DHT should be a child of daemon

**Impact**: The daemon has a properly initialized context at line 60 (`pkg/daemon/daemon.go`):
```go
ctx, cancel := context.WithCancel(context.Background())
```

This daemon context is used for:
- Reconciliation loop (line 227: `case <-d.ctx.Done()`)  
- Status loop (similar pattern)
- Graceful shutdown coordination (line 119: `case <-d.ctx.Done()`)

But DHTDiscovery doesn't participate in this hierarchy, relying instead on explicit cleanup.

### 2. StartCacheSaver Using Stop Channel

**Location**: `pkg/daemon/daemon.go:352-353`

```go
d.cacheStopCh = make(chan struct{})
go StartCacheSaver(d.config.InterfaceName, d.peerStore, d.cacheStopCh)
```

**Location**: `pkg/daemon/cache.go:125`

```go
func StartCacheSaver(interfaceName string, peerStore *PeerStore, stopCh <-chan struct{})
```

The cache saver uses its own dedicated stop channel instead of the daemon's context. This creates:
- Inconsistent cancellation patterns (some goroutines use context, some use channels)
- Extra cleanup code with defer blocks (lines 354-361)
- Manual channel management complexity

### 3. EpochManager Using Stop Channel

**Location**: `pkg/daemon/epoch.go:12,19`

```go
type EpochManager struct {
    router *privacy.DandelionRouter
    stopCh chan struct{}
}
```

```go
stopCh: make(chan struct{})
```

**Location**: `pkg/daemon/epoch.go:25`

```go
func (em *EpochManager) Start(getPeers func() []privacy.PeerInfo) {
    go em.router.EpochRotationLoop(em.stopCh, getPeers)
}
```

The epoch manager creates its own stop channel and passes it to `EpochRotationLoop`. Like the cache saver, this:
- Uses channels instead of context
- Requires manual stop coordination via `defer em.epochManager.Stop()` (line 385 in daemon.go)

## Proposed Approach

Refactor all three components to use context-based cancellation derived from the daemon's context:

### 1. DHTDiscovery Context Derivation

**Pass daemon context to DHTDiscovery**:
- Modify `NewDHTDiscovery` signature to accept a parent context parameter
- Use `context.WithCancel(parentCtx)` instead of `context.Background()`
- Update `pkg/discovery/init.go` factory function to pass daemon context
- Update `pkg/daemon/daemon.go` where DHT is created (line 367)

**Changes**:
```go
// pkg/discovery/dht.go:68
func NewDHTDiscovery(ctx context.Context, config *daemon.Config, localNode *LocalNode, peerStore *daemon.PeerStore) (*DHTDiscovery, error) {
    dhtCtx, cancel := context.WithCancel(ctx)  // Derive from parent
    // ...
}
```

**Factory update**:
```go
// pkg/discovery/init.go
func createDHTDiscovery(ctx context.Context, config *daemon.Config, localNode *daemon.LocalNode, peerStore *daemon.PeerStore) (daemon.DiscoveryLayer, error) {
    // ...
    return NewDHTDiscovery(ctx, config, discoveryLocalNode, peerStore)
}
```

**Daemon update**:
```go
// pkg/daemon/daemon.go (around line 367)
dht, err := dhtFactory(d.ctx, d.config, d.localNode, d.peerStore)
```

### 2. StartCacheSaver Context Conversion

**Replace stop channel with context**:
- Change `StartCacheSaver` signature to accept `context.Context` instead of `stopCh`
- Update the select statement to listen on `ctx.Done()` instead of `stopCh`
- Remove `d.cacheStopCh` field from Daemon struct
- Remove the defer block that closes the channel (lines 354-361)

**Changes**:
```go
// pkg/daemon/cache.go:125
func StartCacheSaver(ctx context.Context, interfaceName string, peerStore *PeerStore) {
    ticker := time.NewTicker(CacheSaveInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            // Final save on shutdown
            if err := SavePeerCache(interfaceName, peerStore); err != nil {
                log.Printf("[Cache] Failed to save peer cache on shutdown: %v", err)
            }
            return
        case <-ticker.C:
            if err := SavePeerCache(interfaceName, peerStore); err != nil {
                log.Printf("[Cache] Failed to save peer cache: %v", err)
            }
        }
    }
}
```

**Daemon invocation**:
```go
// pkg/daemon/daemon.go:353 (remove cacheStopCh field and defer block)
go StartCacheSaver(d.ctx, d.config.InterfaceName, d.peerStore)
```

### 3. EpochManager Context Conversion

**Replace stop channel with context throughout the privacy layer**:
- Update `EpochManager` to accept and store daemon context
- Change `Start()` to pass context instead of stop channel
- Update `privacy.DandelionRouter.EpochRotationLoop` signature to accept context
- Remove `stopCh` field from `EpochManager`
- Remove `Stop()` method (cleanup happens automatically via context)

**Changes**:
```go
// pkg/daemon/epoch.go:10-13
type EpochManager struct {
    router *privacy.DandelionRouter
    ctx    context.Context
}
```

```go
// pkg/daemon/epoch.go:16
func NewEpochManager(ctx context.Context, epochSeed [32]byte) *EpochManager {
    return &EpochManager{
        router: privacy.NewDandelionRouter(epochSeed),
        ctx:    ctx,
    }
}
```

```go
// pkg/daemon/epoch.go:24
func (em *EpochManager) Start(getPeers func() []privacy.PeerInfo) {
    go em.router.EpochRotationLoop(em.ctx, getPeers)
    log.Printf("[Epoch] Epoch management started (rotation every %v)", privacy.DefaultEpochDuration)
}
```

```go
// pkg/privacy/dandelion.go:243
func (d *DandelionRouter) EpochRotationLoop(ctx context.Context, getPeers func() []PeerInfo) {
    ticker := time.NewTicker(DefaultEpochDuration)
    defer ticker.Stop()

    d.RotateEpoch(getPeers())

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            d.RotateEpoch(getPeers())
        }
    }
}
```

**Daemon invocation**:
```go
// pkg/daemon/daemon.go:383-385
if d.config.Privacy {
    d.epochManager = NewEpochManager(d.ctx, d.config.Keys.EpochSeed)
    d.epochManager.Start(d.getPrivacyPeers)
    // No defer needed - context cancellation handles cleanup
    log.Printf("Privacy mode enabled (Dandelion++ relay)")
}
```

### Benefits of This Refactoring

1. **Consistent patterns**: All goroutines use context for cancellation
2. **Automatic cleanup**: Context cancellation propagates automatically
3. **Simpler code**: Removes manual channel management and defer blocks
4. **Better hierarchy**: Clear parent-child context relationships
5. **Idiomatic Go**: Follows standard Go context patterns

## Affected Files

### Code Changes

1. **`pkg/discovery/dht.go`**:
   - Line 68: Add `ctx context.Context` parameter to `NewDHTDiscovery`
   - Line 69: Change `context.Background()` to use the passed `ctx`

2. **`pkg/discovery/init.go`**:
   - Update factory function signature to accept context
   - Pass context to `NewDHTDiscovery`

3. **`pkg/daemon/daemon.go`**:
   - Line 37: Remove `cacheStopCh chan struct{}` field
   - Line 352-361: Replace cache stop channel code with context-based call
   - Line 367: Pass `d.ctx` to DHT factory function
   - Line 383: Pass `d.ctx` to `NewEpochManager`
   - Line 385: Remove `defer d.epochManager.Stop()`

4. **`pkg/daemon/cache.go`**:
   - Line 125: Change `StartCacheSaver` signature to accept `context.Context`
   - Line 131: Change `case <-stopCh:` to `case <-ctx.Done():`

5. **`pkg/daemon/epoch.go`**:
   - Line 12: Replace `stopCh chan struct{}` with `ctx context.Context`
   - Line 16: Add `ctx context.Context` parameter to `NewEpochManager`
   - Line 18-19: Store context instead of creating stop channel
   - Line 24-25: Pass `em.ctx` to `EpochRotationLoop` instead of `em.stopCh`
   - Line 30-32: Remove `Stop()` method (no longer needed)

6. **`pkg/privacy/dandelion.go`**:
   - Line 243: Change `EpochRotationLoop` signature from `stopCh <-chan struct{}` to `ctx context.Context`
   - Line 252: Change `case <-stopCh:` to `case <-ctx.Done():`

## Test Strategy

### Existing Test Coverage

The changes are structural refactoring that shouldn't change behavior - the goroutines should still stop when expected, just via context cancellation instead of explicit stop calls.

### Manual Testing

1. **Basic daemon lifecycle**:
   ```bash
   # Start daemon with DHT discovery
   sudo wgmesh join --secret test-secret --interface wg0
   
   # Verify it starts successfully
   # Send SIGINT (Ctrl+C)
   # Verify all components shut down cleanly
   # Check logs for proper cleanup messages
   ```

2. **DHT Discovery shutdown**:
   - Start daemon with DHT enabled
   - Send shutdown signal
   - Verify DHT goroutines (announce, query, persist) exit cleanly
   - Check that no goroutine leaks occur

3. **Cache saver shutdown**:
   - Start daemon
   - Wait for cache save to occur
   - Send shutdown signal  
   - Verify final cache save happens on shutdown
   - Check `/var/lib/wgmesh/*-peers.json` has recent timestamp

4. **Epoch manager shutdown**:
   - Start daemon with `--privacy` flag
   - Verify epoch rotation is running
   - Send shutdown signal
   - Verify epoch rotation stops cleanly
   - Check logs for clean epoch manager shutdown

5. **Context propagation test**:
   - Use race detector: `go test -race ./...`
   - Verify no data races introduced
   - Verify no goroutine leaks using profiling

### Unit Testing

Since this is primarily a refactoring with no behavior change:

1. **Existing tests should pass**: Run `make test` to ensure all existing tests still pass
2. **No new tests required**: The changes are internal implementation details
3. **Race detection**: Run with `-race` flag to catch concurrency issues

### Integration Testing

1. Run the existing integration tests if any
2. Test actual mesh formation between two nodes
3. Test graceful shutdown with active peers
4. Test shutdown during active DHT operations

### Verification Criteria

- ✅ All existing tests pass
- ✅ No race conditions detected with `-race` flag
- ✅ Clean shutdown logs (no hung goroutines)
- ✅ All three components (DHT, cache, epoch) stop on context cancellation
- ✅ No goroutine leaks (can verify with pprof if needed)

## Estimated Complexity

**medium**

**Reasoning**:
- Changes touch 6 files across 3 packages
- Requires understanding of Go context patterns
- Must carefully update all call sites
- Need to ensure no goroutine leaks introduced
- Privacy layer changes require understanding Dandelion++ implementation
- Testing requires careful verification of shutdown behavior

**Time estimate**: 2-4 hours
- 1 hour: Implementation of context changes
- 1 hour: Testing and verification  
- 1 hour: Code review and cleanup
- 0-1 hour: Handling any edge cases discovered during testing
