# Specification: Issue #107

## Classification
fix

## Deliverables
code

## Problem Analysis

The `PeerStore` in `pkg/daemon/peerstore.go` has a critical security vulnerability: it accepts unlimited peer additions without any capacity checks. The `Update()` method at lines 40-77 adds new peers to the internal `peers` map without enforcing a maximum count.

**Attack Vector:**
An attacker can flood the mesh with malicious announcements containing unique WireGuard public keys through multiple discovery layers:
1. **DHT discovery** (`pkg/discovery/dht.go:d.peerStore.Update()`) - publicly accessible via BitTorrent DHT
2. **Gossip protocol** (`pkg/discovery/gossip.go:g.peerStore.Update()`) - transitive peer announcements
3. **LAN discovery** (`pkg/discovery/lan.go:l.peerStore.Update()`) - multicast announcements
4. **Peer exchange** (`pkg/discovery/exchange.go:pe.peerStore.Update()`) - both direct and transitive peers

**Why CleanupStale() is Insufficient:**
The `CleanupStale()` method (lines 150-163) only removes peers that haven't been seen for `PeerRemoveTimeout` (10 minutes). An attacker can:
- Generate unique public keys at a rate faster than the cleanup interval
- Re-announce stale peers to refresh their `LastSeen` timestamp
- Exploit the fact that `Update()` updates `LastSeen` to `time.Now()` on line 64 for existing peers

**Impact:**
- Memory exhaustion leading to daemon crash or OOM kill
- Degraded performance due to large map operations
- Potential for denial-of-service attack on mesh nodes

**Evidence from Code:**
```go
// pkg/daemon/peerstore.go:46-51
if !exists {
    info.LastSeen = time.Now()
    info.DiscoveredVia = []string{discoveryMethod}
    ps.peers[info.WGPubKey] = info  // ⚠️ No cap check
    return
}
```

No validation currently prevents unlimited growth. The `Count()` method at lines 166-170 only reports the current size but doesn't enforce any limits.

## Proposed Approach

Add a configurable maximum peer count with two complementary strategies:

### 1. Add MaxPeers Constant and Configuration

Define a sensible default maximum peer count (e.g., 1000 peers) that balances:
- Mesh scalability (most deployments will have < 100 peers)
- Memory constraints (each PeerInfo is ~150-200 bytes)
- DoS protection

Add to `pkg/daemon/peerstore.go`:
```go
const DefaultMaxPeers = 1000
```

### 2. Implement Capacity Check in Update()

Before adding a new peer (in the `!exists` branch), check if the store is at capacity. If at capacity, implement one of these eviction policies:

**Option A: Reject new peers when at capacity** (simplest)
- Return an error or log a warning
- Prevents unbounded growth
- Existing legitimate peers are preserved

**Option B: LRU eviction** (more sophisticated)
- When at capacity, evict the peer with the oldest `LastSeen` timestamp
- Ensures most active peers remain
- Requires finding minimum `LastSeen` across the map

**Recommendation: Option A** for the initial fix because:
- Simpler implementation with fewer edge cases
- Legitimate mesh networks won't hit the limit
- Attack scenarios are clearly rejected
- Can add Option B later if needed

### 3. Implementation Details

Modify `Update()` method in `pkg/daemon/peerstore.go`:

```go
func (ps *PeerStore) Update(info *PeerInfo, discoveryMethod string) bool {
    ps.mu.Lock()
    defer ps.mu.Unlock()

    existing, exists := ps.peers[info.WGPubKey]
    if !exists {
        // Check capacity before adding new peer
        if len(ps.peers) >= DefaultMaxPeers {
            // Log warning and reject
            return false
        }
        // ... existing code to add peer
        return true
    }
    // ... existing update logic
    return true
}
```

### 4. Add Metrics/Logging

Add logging when capacity is reached to help operators detect potential attacks:
```go
log.Printf("[PeerStore] At capacity (%d peers), rejecting new peer %s via %s", 
    DefaultMaxPeers, info.WGPubKey[:16]+"...", discoveryMethod)
```

### 5. Testing Strategy

Add tests to `pkg/daemon/peerstore_test.go`:
- Test that exactly `DefaultMaxPeers` can be added
- Test that the (DefaultMaxPeers + 1)th peer is rejected
- Test that updating existing peers still works when at capacity
- Test that after cleanup, new peers can be added again

## Affected Files

### Code Changes Required

1. **`pkg/daemon/peerstore.go`**:
   - Line 8-12: Add `DefaultMaxPeers` constant (after existing constants)
   - Line 40-77: Modify `Update()` method to check capacity before adding new peers
   - Add return value (bool) to `Update()` to indicate success/failure

2. **`pkg/daemon/peerstore_test.go`**:
   - Add new test: `TestPeerStoreMaxPeers()`
   - Add new test: `TestPeerStoreCapacityRejection()`
   - Add new test: `TestPeerStoreUpdateExistingAtCapacity()`

### Potential Cascade Changes

The following files call `PeerStore.Update()` and may need to handle the boolean return value (if we change the signature):

1. **`pkg/discovery/dht.go`** (line with `d.peerStore.Update()`)
2. **`pkg/discovery/gossip.go`** (2 lines with `g.peerStore.Update()`)
3. **`pkg/discovery/lan.go`** (line with `l.peerStore.Update()`)
4. **`pkg/discovery/exchange.go`** (3 lines with `pe.peerStore.Update()`)

**Decision:** Since this is a security fix and the callers don't currently handle errors, we can either:
- Keep `Update()` as `void` and log internally (simpler, no cascade changes)
- Return `bool` and have callers log/handle (better design, requires updates)

**Recommendation:** Return `bool` but make callers ignore it initially. This preserves the fix while allowing future improvements.

## Test Strategy

### Unit Tests

Add to `pkg/daemon/peerstore_test.go`:

```go
func TestPeerStoreMaxPeers(t *testing.T) {
    ps := NewPeerStore()
    
    // Add exactly DefaultMaxPeers
    for i := 0; i < DefaultMaxPeers; i++ {
        peer := &PeerInfo{
            WGPubKey: fmt.Sprintf("key%d", i),
            MeshIP:   fmt.Sprintf("10.0.%d.%d", i/255, i%255),
        }
        ps.Update(peer, "test")
    }
    
    if ps.Count() != DefaultMaxPeers {
        t.Errorf("Expected %d peers, got %d", DefaultMaxPeers, ps.Count())
    }
    
    // Try to add one more - should fail
    extraPeer := &PeerInfo{
        WGPubKey: "extra",
        MeshIP:   "10.1.0.1",
    }
    ps.Update(extraPeer, "test")
    
    if ps.Count() != DefaultMaxPeers {
        t.Errorf("Store should still have %d peers after rejection, got %d", 
            DefaultMaxPeers, ps.Count())
    }
    
    if _, exists := ps.Get("extra"); exists {
        t.Error("Extra peer should not be in store")
    }
}

func TestPeerStoreUpdateExistingAtCapacity(t *testing.T) {
    ps := NewPeerStore()
    
    // Fill to capacity
    for i := 0; i < DefaultMaxPeers; i++ {
        peer := &PeerInfo{
            WGPubKey: fmt.Sprintf("key%d", i),
            MeshIP:   fmt.Sprintf("10.0.0.%d", i),
            Endpoint: "1.2.3.4:51820",
        }
        ps.Update(peer, "test")
    }
    
    // Update an existing peer - should succeed
    updated := &PeerInfo{
        WGPubKey: "key0",
        Endpoint: "5.6.7.8:51820",
    }
    ps.Update(updated, "lan")
    
    peer, _ := ps.Get("key0")
    if peer.Endpoint != "5.6.7.8:51820" {
        t.Error("Should be able to update existing peer when at capacity")
    }
}

func TestPeerStoreCapacityAfterCleanup(t *testing.T) {
    ps := NewPeerStore()
    
    // Fill to capacity with old peers
    for i := 0; i < DefaultMaxPeers; i++ {
        ps.mu.Lock()
        ps.peers[fmt.Sprintf("old%d", i)] = &PeerInfo{
            WGPubKey: fmt.Sprintf("old%d", i),
            LastSeen: time.Now().Add(-15 * time.Minute),
        }
        ps.mu.Unlock()
    }
    
    // Cleanup should remove all stale peers
    removed := ps.CleanupStale()
    if len(removed) != DefaultMaxPeers {
        t.Errorf("Expected %d removed, got %d", DefaultMaxPeers, len(removed))
    }
    
    // Should now be able to add new peers
    newPeer := &PeerInfo{
        WGPubKey: "fresh",
        MeshIP:   "10.0.0.1",
    }
    ps.Update(newPeer, "test")
    
    if _, exists := ps.Get("fresh"); !exists {
        t.Error("Should be able to add peer after cleanup")
    }
}
```

### Integration Tests

Manual testing scenario to verify DoS protection:

1. Start a daemon in decentralized mode
2. Use a script to simulate malicious announcements with unique pubkeys
3. Monitor memory usage - should plateau at ~200MB (1000 peers × 200 bytes)
4. Verify logs show rejection messages
5. Verify legitimate peer updates still work

### Performance Testing

Verify that the fix doesn't impact performance:
- Measure `Update()` latency with mutex + capacity check
- Should add < 1µs overhead for the `len(ps.peers)` check
- No impact on existing peer updates (fast path)

## Estimated Complexity

**low** (1-2 hours)

**Justification:**
- Simple capacity check: `len(ps.peers) >= DefaultMaxPeers`
- Minimal code changes (< 10 lines of new code)
- Tests are straightforward
- No complex eviction logic needed for Option A
- No configuration file changes needed (using const)
- Clear success criteria

**Breakdown:**
- Implementation: 30 minutes
- Unit tests: 30 minutes
- Manual testing/verification: 30 minutes
- Documentation/comments: 15 minutes
