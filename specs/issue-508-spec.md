# Specification: Issue #508

## Classification
feature

## Deliverables
code

## Problem Analysis

In `pkg/discovery/dht.go`, `initDHTServer()` bootstraps the DHT routing table once. The bootstrap flow is:

1. Resolve bootstrap node hostnames via DNS. If **all** fail → `return fmt.Errorf("no bootstrap nodes resolved")` → `Start()` propagates the error and the daemon fails to start.
2. Create the DHT server.
3. Spawn a goroutine that calls `d.server.Announce()` to trigger the DHT routing-table population.
4. Poll `server.NumNodes()` for up to 10 seconds, then continue regardless (returns `nil`).

Two failure modes exist on unreliable networks:

- **Hard failure**: If all DNS resolutions fail (e.g., temporary DNS outage), `initDHTServer()` returns an error and `Start()` returns that error, stopping the daemon entirely.
- **Soft failure**: If the announce goroutine gets 0 nodes (UDP unreachable), the goroutine exits after 30 s and never retries. Discovery stays broken for the lifetime of the process.

The fix must add exponential backoff retry to cover both failure modes without blocking the `Start()` return path (LAN discovery, gossip, and other layers must keep running while DHT retries).

Existing rendezvous backoff pattern in the same file (`backoffEntry`, `recordRendezvousAttempt`, `canAttemptRendezvous`) provides a reference for style and test conventions.

## Implementation Tasks

### Task 1: Add bootstrap backoff constants to `pkg/discovery/dht.go`

In the existing `const` block (lines 24–42), after the `DHTPersistInterval` constant, add:

```go
DHTBootstrapInitialDelay = 5 * time.Second
DHTBootstrapMaxDelay     = 60 * time.Second
```

### Task 2: Add `math/rand` to imports in `pkg/discovery/dht.go`

The file currently does not import `math/rand`. Add it to the stdlib import group:

```go
import (
    "context"
    "fmt"
    "hash/fnv"
    "log"
    "math/rand"
    "net"
    "os"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/anacrolix/dht/v2"
    "github.com/anacrolix/dht/v2/krpc"
    "github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
    "github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
    "github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)
```

### Task 3: Add `dhtBackoffDelay` helper function in `pkg/discovery/dht.go`

Add this package-level function anywhere before `initDHTServer()` (e.g., just before the function):

```go
// dhtBackoffDelay applies ±25% jitter to d and returns the result.
// This prevents thundering-herd retries when multiple nodes restart simultaneously.
func dhtBackoffDelay(d time.Duration) time.Duration {
    // jitter factor in [-0.25, +0.25]
    jitter := (rand.Float64() - 0.5) * 0.5 // range: -0.25 … +0.25
    return time.Duration(float64(d) * (1 + jitter))
}
```

### Task 4: Extract bootstrap lookup into `attemptBootstrapLookup` in `pkg/discovery/dht.go`

This method contains the logic currently inside the anonymous goroutine in `initDHTServer()`.
Add the following method to `DHTDiscovery`:

```go
// attemptBootstrapLookup performs one DHT bootstrap lookup and returns true
// if at least one DHT node was discovered. It blocks for up to DHTBootstrapTimeout.
func (d *DHTDiscovery) attemptBootstrapLookup() bool {
    ctx, cancel := context.WithTimeout(d.ctx, DHTBootstrapTimeout)
    defer cancel()

    var randomID [20]byte
    copy(randomID[:], d.config.Keys.NetworkID[:])

    a, err := d.server.Announce(randomID, 0, false)
    if err != nil {
        log.Printf("[DHT] Bootstrap lookup failed: %v", err)
        return false
    }
    defer a.Close()

    for {
        select {
        case <-ctx.Done():
            return d.server.NumNodes() > 0
        case _, ok := <-a.Peers:
            if !ok {
                return d.server.NumNodes() > 0
            }
        }
    }
}
```

### Task 5: Add `bootstrapWithRetry` method to `DHTDiscovery` in `pkg/discovery/dht.go`

This goroutine method replaces the current one-shot anonymous goroutine + 10-second poll loop.
Add the following method:

```go
// bootstrapWithRetry calls attemptBootstrapLookup in a loop with exponential
// backoff until the routing table is populated or the context is cancelled.
// Delay starts at DHTBootstrapInitialDelay, doubles on each failure, and is
// capped at DHTBootstrapMaxDelay. Each delay has ±25% jitter applied.
func (d *DHTDiscovery) bootstrapWithRetry() {
    delay := DHTBootstrapInitialDelay
    for attempt := 0; ; attempt++ {
        if attempt > 0 {
            jittered := dhtBackoffDelay(delay)
            log.Printf("[DHT] Bootstrap attempt %d failed, retrying in %v...", attempt, jittered.Round(time.Millisecond))
            select {
            case <-d.ctx.Done():
                return
            case <-time.After(jittered):
            }
        }

        select {
        case <-d.ctx.Done():
            return
        default:
        }

        if d.attemptBootstrapLookup() {
            nodes := d.server.NumNodes()
            if attempt > 0 {
                log.Printf("[DHT] Bootstrap succeeded after %d retries, DHT has %d nodes", attempt, nodes)
            } else {
                log.Printf("[DHT] Bootstrap complete, DHT has %d nodes", nodes)
            }
            return
        }

        // Double delay for next attempt, capped at max
        delay *= 2
        if delay > DHTBootstrapMaxDelay {
            delay = DHTBootstrapMaxDelay
        }
    }
}
```

### Task 6: Rewrite the bootstrap section of `initDHTServer` in `pkg/discovery/dht.go`

Replace the block starting at `log.Printf("[DHT] Bootstrapping into DHT network on port %d...", d.dhtPort)` through the end of the function (the anonymous goroutine, the 10-second poll loop, and the final timeout log) with:

```go
log.Printf("[DHT] Bootstrapping into DHT network on port %d...", d.dhtPort)
go d.bootstrapWithRetry()
return nil
```

The complete end of `initDHTServer()` after the change should look like:

```go
    d.server = server
    d.loadPersistedNodes()

    log.Printf("[DHT] Bootstrapping into DHT network on port %d...", d.dhtPort)
    go d.bootstrapWithRetry()
    return nil
}
```

### Task 7: Add unit tests in `pkg/discovery/dht_test.go`

Append the following test functions at the end of `pkg/discovery/dht_test.go`.
The file already imports `"testing"` and `"time"`; no new imports are needed.

```go
// TestDHTBackoffDelay_JitterInBounds verifies that dhtBackoffDelay produces
// values within ±25% of the input duration over many samples.
func TestDHTBackoffDelay_JitterInBounds(t *testing.T) {
    base := 10 * time.Second
    low := time.Duration(float64(base) * 0.75)
    high := time.Duration(float64(base) * 1.25)

    for i := 0; i < 1000; i++ {
        got := dhtBackoffDelay(base)
        if got < low || got > high {
            t.Errorf("dhtBackoffDelay(%v) = %v, want in [%v, %v]", base, got, low, high)
        }
    }
}

// TestDHTBootstrapDelay_Progression verifies the doubling sequence with max cap.
func TestDHTBootstrapDelay_Progression(t *testing.T) {
    delay := DHTBootstrapInitialDelay // 5s
    want := []time.Duration{
        5 * time.Second,
        10 * time.Second,
        20 * time.Second,
        40 * time.Second,
        60 * time.Second, // capped at max
        60 * time.Second, // remains at max
    }
    for i, w := range want {
        if delay != w {
            t.Errorf("step %d: delay = %v, want %v", i, delay, w)
        }
        delay *= 2
        if delay > DHTBootstrapMaxDelay {
            delay = DHTBootstrapMaxDelay
        }
    }
}

// TestBootstrapWithRetry_StopsOnContextCancel verifies that bootstrapWithRetry
// exits promptly when the context is cancelled, without blocking.
func TestBootstrapWithRetry_StopsOnContextCancel(t *testing.T) {
    cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: "wgmesh-test-bootstrap-cancel-1"})
    if err != nil {
        t.Fatalf("NewConfig failed: %v", err)
    }
    ctx, cancel := context.WithCancel(context.Background())
    d, err := NewDHTDiscovery(ctx, cfg, &daemon.LocalNode{WGPubKey: "a"}, daemon.NewPeerStore())
    if err != nil {
        t.Fatalf("NewDHTDiscovery failed: %v", err)
    }

    // Cancel the context so the goroutine should exit on the first attempt check
    cancel()

    done := make(chan struct{})
    go func() {
        defer close(done)
        d.bootstrapWithRetry()
    }()

    select {
    case <-done:
        // OK
    case <-time.After(3 * time.Second):
        t.Error("bootstrapWithRetry did not stop within 3 seconds after context cancel")
    }
}
```

## Affected Files

| File | Change |
|---|---|
| `pkg/discovery/dht.go` | Add `DHTBootstrapInitialDelay`, `DHTBootstrapMaxDelay` constants; add `math/rand` import; add `dhtBackoffDelay()`, `attemptBootstrapLookup()`, `bootstrapWithRetry()` methods; rewrite the bootstrap section of `initDHTServer()` |
| `pkg/discovery/dht_test.go` | Add `TestDHTBackoffDelay_JitterInBounds`, `TestDHTBootstrapDelay_Progression`, `TestBootstrapWithRetry_StopsOnContextCancel` |

No other files require changes.

## Test Strategy

1. **Unit tests** (new, in `pkg/discovery/dht_test.go`):
   - `TestDHTBackoffDelay_JitterInBounds`: samples `dhtBackoffDelay` 1000 times and asserts every result is within ±25% of the input.
   - `TestDHTBootstrapDelay_Progression`: manually steps through the doubling sequence and asserts each step matches 5s → 10s → 20s → 40s → 60s → 60s.
   - `TestBootstrapWithRetry_StopsOnContextCancel`: creates a `DHTDiscovery` with a pre-cancelled context and asserts `bootstrapWithRetry()` returns within 3 seconds.

2. **Existing tests** — run `go test ./pkg/discovery/...` to confirm all 14 existing tests in `dht_test.go` still pass.

3. **Build** — `go build ./...` must succeed with no compilation errors.

4. **Race detector** — `go test -race ./pkg/discovery/...` must pass with no data-race warnings. (The new goroutine accesses `d.server` and `d.ctx` which are set before the goroutine is started.)

5. **Integration smoke test** (manual, on a host with firewall blocking UDP 6881):
   - Block outbound UDP: `sudo iptables -A OUTPUT -p udp --dport 6881 -j DROP`
   - Start daemon: `sudo wgmesh join --secret "wgmesh://v1/<secret>" --log-level debug`
   - Observe `[DHT] Bootstrap attempt 1 failed, retrying in 4.XXXs...` log lines.
   - Unblock: `sudo iptables -D OUTPUT -p udp --dport 6881 -j DROP`
   - Observe `[DHT] Bootstrap succeeded after N retries, DHT has X nodes` within 60s.

## Estimated Complexity
low
