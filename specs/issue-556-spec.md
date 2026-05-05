# Specification: Issue #556

## Classification
fix

## Deliverables
code

## Problem Analysis

Nodes behind NAT that connect through an introducer (relay/jump node) experience periodic ~30–60
second connectivity outages. The pattern observed in the issue logs is:

```
64 bytes from 10.158.18.110: icmp_seq=2288 ... (relay working)
From 10.158.44.76 icmp_seq=2305 Destination Host Unreachable  (relay dropped)
...
64 bytes from 10.158.18.110: icmp_seq=2339 ... (relay restored)
```

### Root Cause 1 — Relay is dropped when control-path UDP exchange succeeds but WG handshake does not

When `checkStaleHandshakes` fires (every `RendezvousStaleCheck = 10s`) for a relay-routed peer
that has no direct WireGuard handshake, `tryRendezvousForPeer` is called. If the hole-punch UDP
exchange reaches the remote peer at the NAT level (the HELLO/REPLY handshake completes), the
following happens:

1. `handleReply` in `pkg/discovery/exchange.go` calls
   `pe.peerStore.Update(peerInfo, DHTMethod)` unconditionally — this appends `"dht"` to the
   peer's `DiscoveredVia` slice, even though no WireGuard handshake was established.
2. The relay guard in `shouldRelayPeerWithSubnets` (`pkg/daemon/daemon.go`, lines 662–665)
   reads:
   ```go
   if hasDiscoveryMethod(peer.DiscoveredVia, "dht-transitive") &&
       !hasDiscoveryMethod(peer.DiscoveredVia, "dht") {
       return true
   }
   ```
   Once `"dht"` is added to `DiscoveredVia`, the condition `!hasDiscoveryMethod(..., "dht")`
   becomes **false**, so the relay guard no longer fires.
3. With no active WG handshake and neither side having symmetric NAT (or not matching the
   remaining relay conditions), `shouldRelayPeerWithSubnets` returns `false`.
4. The relay hysteresis (`RelayHysteresisThreshold = 3` reconcile cycles × 5 s = 15 s) counts
   down, then the relay is dropped and the peer is configured as a direct WireGuard entry.
5. The direct path fails (NAT punch did not establish a WG handshake). The mesh probe evicts the
   peer after ~8 failures (≈8 s). The peer is marked temporarily offline for 30 s.
6. After 30 s, the peer is rediscovered via DHT as `"dht-transitive"` → relay re-established →
   cycle repeats.

### Root Cause 2 — Stale WG handshake does not fall back to relay for non-symmetric NAT

When a previously successful direct WireGuard connection goes stale (handshake age ≥
`HandshakeStaleAfter = 150 s`, e.g., because a cone-NAT mapping renewed on a different port),
the stale-handshake branch of `shouldRelayPeerWithSubnets` (lines 655–666):

```go
// Handshake stale — but only relay if NAT situation warrants it.
// For cone/unknown NAT or IPv6, the staleness is likely transient
// (e.g., WG rekey timing). Only relay for symmetric+symmetric.
if d.localNode.NATType == "symmetric" && peer.NATType == "symmetric" {
    return true
}
if hasDiscoveryMethod(peer.DiscoveredVia, "dht-transitive") &&
    !hasDiscoveryMethod(peer.DiscoveredVia, "dht") {
    return true
}
return false
```

…returns `false` for cone/unknown NAT pairs, leaving traffic in a blackhole while WireGuard
tries to re-establish a handshake to an endpoint that may no longer be valid. The comment's
reasoning ("staleness is likely transient — WG rekey timing") is incorrect: a HandshakeStaleAfter
of 150 s is far longer than any WG rekey window (which completes in < 5 s), so a 150 s stale
timestamp reliably indicates a real connectivity failure, not a transient rekey.

## Implementation Tasks

### Task 1: Fix `shouldRelayPeerWithSubnets` — keep relay when transitive peer also has "dht" in DiscoveredVia (Root Cause 1)

**File:** `pkg/daemon/daemon.go`

**Location:** `shouldRelayPeerWithSubnets`, inside the `if handshakes != nil` block and in the
block below it.

The current no-handshake fallthrough (lines 662–666 and 682–687 in daemon.go) reads:

```go
// For transitive-only peers with stale handshake, relay to avoid blackhole
if hasDiscoveryMethod(peer.DiscoveredVia, "dht-transitive") &&
    !hasDiscoveryMethod(peer.DiscoveredVia, "dht") {
    return true
}
return false
```

and (lines 682–687):

```go
if handshakes != nil {
    if ts, ok := handshakes[peer.WGPubKey]; !ok || ts == 0 {
        if hasDiscoveryMethod(peer.DiscoveredVia, "dht-transitive") {
            return true
        }
    }
}
```

Change **both** occurrences of the transitive guard (the one inside the stale-handshake block
AND the one in the no-handshake block) to remove the `&& !hasDiscoveryMethod(peer.DiscoveredVia, "dht")` qualifier:

**Stale handshake block** (inside `if ts, ok := handshakes[peer.WGPubKey]; ok && ts > 0 {`):

```go
// For transitive-only peers with stale handshake, relay to avoid blackhole.
// Do NOT gate on "dht" absence: a control-path UDP exchange (which adds "dht"
// to DiscoveredVia) is not the same as a confirmed WireGuard handshake.
if hasDiscoveryMethod(peer.DiscoveredVia, "dht-transitive") {
    return true
}
return false
```

**No-handshake block** (after the `if handshakes != nil` block, before final `return false`):

The lines 682–688 check is already correct (does not include `!hasDiscoveryMethod("dht")`), so
no change is needed there. The only change required is in the stale-handshake branch as shown
above.

**Exact diff** for `shouldRelayPeerWithSubnets`:

```diff
-			// For transitive-only peers with stale handshake, relay to avoid blackhole
-			if hasDiscoveryMethod(peer.DiscoveredVia, "dht-transitive") &&
-				!hasDiscoveryMethod(peer.DiscoveredVia, "dht") {
-				return true
-			}
+			// For peers that were ever reached transitively, relay to avoid blackhole.
+			// A successful control-path UDP exchange (which appends "dht" to DiscoveredVia)
+			// is NOT a confirmed WireGuard handshake; the "dht" guard must not block relay.
+			if hasDiscoveryMethod(peer.DiscoveredVia, "dht-transitive") {
+				return true
+			}
 			return false
```

### Task 2: Fix `shouldRelayPeerWithSubnets` — fall back to relay for ALL NAT types when handshake is stale (Root Cause 2)

**File:** `pkg/daemon/daemon.go`

**Location:** `shouldRelayPeerWithSubnets`, inside the `if ts, ok := handshakes[peer.WGPubKey]; ok && ts > 0 {` block.

Replace the stale-handshake handling block so that ANY stale handshake (not just
symmetric+symmetric) falls back to relay. The relay-to-direct hysteresis
(`RelayHysteresisThreshold = 3`) will transition the peer back to direct once a fresh WG
handshake is confirmed.

**Current code (lines 655–666 of daemon.go):**

```go
// Handshake stale — but only relay if NAT situation warrants it.
// For cone/unknown NAT or IPv6, the staleness is likely transient
// (e.g., WG rekey timing). Only relay for symmetric+symmetric.
if d.localNode.NATType == "symmetric" && peer.NATType == "symmetric" {
    return true
}
// For transitive-only peers with stale handshake, relay to avoid blackhole
if hasDiscoveryMethod(peer.DiscoveredVia, "dht-transitive") &&
    !hasDiscoveryMethod(peer.DiscoveredVia, "dht") {
    return true
}
return false
```

**Replacement:**

```go
// Handshake is stale (> HandshakeStaleAfter). Fall back to relay so
// traffic is not blacked out while WireGuard attempts to re-establish.
// HandshakeStaleAfter (150 s) is far longer than any WG rekey window
// (< 5 s), so a stale timestamp always indicates a real connectivity
// failure — not a transient rekey.
// The relay→direct hysteresis (RelayHysteresisThreshold cycles) will
// transition back to direct once a fresh handshake is confirmed.
return true
```

Note: Task 2 subsumes the transitive-peer guard that was removed in Task 1 from the stale
block, since `return true` covers all cases.

### Task 3: Update existing tests that asserted the old (incorrect) behaviour

**File:** `pkg/daemon/relay_test.go`

The test `TestShouldRelayPeer_StaleHandshake_ConeCone_NoRelay` (lines 108–123) asserts that
cone+cone with a stale handshake should **not** relay. With Task 2's fix this behaviour changes
to "should relay". Rename and invert the assertion:

```diff
-func TestShouldRelayPeer_StaleHandshake_ConeCone_NoRelay(t *testing.T) {
-	// ...
-	// Handshake 5 minutes ago → stale but cone+cone → transient, don't relay
-	staleTS := time.Now().Add(-5 * time.Minute).Unix()
-	handshakes := map[string]int64{"peer1": staleTS}
-
-	if d.shouldRelayPeer(peer, relays, handshakes) {
-		t.Error("should not relay cone+cone with stale handshake (likely transient WG rekey)")
-	}
-}
+func TestShouldRelayPeer_StaleHandshake_ConeCone_Relays(t *testing.T) {
+	d := &Daemon{
+		config:    &Config{},
+		localNode: &LocalNode{NATType: "cone"},
+	}
+	peer := &PeerInfo{WGPubKey: "peer1", NATType: "cone"}
+	relays := []*PeerInfo{{WGPubKey: "relay1", Introducer: true, Endpoint: "1.2.3.4:51820"}}
+
+	// Handshake 5 minutes ago → stale → fall back to relay (all NAT types)
+	staleTS := time.Now().Add(-5 * time.Minute).Unix()
+	handshakes := map[string]int64{"peer1": staleTS}
+
+	if !d.shouldRelayPeer(peer, relays, handshakes) {
+		t.Error("should relay when WG handshake is stale, regardless of NAT type")
+	}
+}
```

### Task 4: Add new regression tests for Root Cause 1

**File:** `pkg/daemon/relay_test.go`

Add the following two tests after the existing relay tests:

```go
// TestShouldRelayPeer_TransitiveAndDHT_KeepsRelay verifies that discovering a peer via
// both "dht-transitive" and "dht" (which happens when the control-path UDP exchange
// succeeds but the WireGuard handshake has not been established) keeps the relay active.
func TestShouldRelayPeer_TransitiveAndDHT_KeepsRelay(t *testing.T) {
	d := &Daemon{
		config:    &Config{},
		localNode: &LocalNode{NATType: "cone"},
	}
	peer := &PeerInfo{
		WGPubKey:      "peer1",
		NATType:       "cone",
		DiscoveredVia: []string{"dht-transitive", "dht"},
	}
	relays := []*PeerInfo{{WGPubKey: "relay1", Introducer: true, Endpoint: "1.2.3.4:51820"}}

	// No WG handshake yet — peer was seen via control exchange but not confirmed direct.
	if !d.shouldRelayPeer(peer, relays, map[string]int64{"peer1": 0}) {
		t.Error("should keep relay when peer has dht-transitive+dht but no WG handshake")
	}
}

// TestShouldRelayPeer_TransitiveStale_KeepsRelay verifies that a transitive peer with a
// stale WG handshake (direct succeeded once, now stale) falls back to relay.
func TestShouldRelayPeer_TransitiveStale_KeepsRelay(t *testing.T) {
	d := &Daemon{
		config:    &Config{},
		localNode: &LocalNode{NATType: "cone"},
	}
	peer := &PeerInfo{
		WGPubKey:      "peer1",
		NATType:       "cone",
		DiscoveredVia: []string{"dht-transitive", "dht"},
	}
	relays := []*PeerInfo{{WGPubKey: "relay1", Introducer: true, Endpoint: "1.2.3.4:51820"}}

	staleTS := time.Now().Add(-5 * time.Minute).Unix()
	if !d.shouldRelayPeer(peer, relays, map[string]int64{"peer1": staleTS}) {
		t.Error("should relay when transitive peer has stale WG handshake")
	}
}
```

### Task 5: Verify with `go test -race ./pkg/daemon/...`

After applying Tasks 1–4, run:

```bash
go test -race ./pkg/daemon/... -run TestShouldRelayPeer
```

All relay tests must pass. No race conditions must be reported.

Also run the full daemon test suite:

```bash
go test -race ./pkg/daemon/...
```

## Affected Files

- `pkg/daemon/daemon.go` — `shouldRelayPeerWithSubnets` function (Tasks 1 and 2)
- `pkg/daemon/relay_test.go` — update `TestShouldRelayPeer_StaleHandshake_ConeCone_NoRelay`
  and add two new regression tests (Tasks 3 and 4)

## Test Strategy

1. Run existing `TestShouldRelayPeer_*` tests — all must pass after Task 3 (one test renamed
   and inverted, all others unchanged).
2. Two new regression tests (Task 4) cover the exact failure modes reported in Issue #556.
3. `go test -race ./pkg/daemon/...` must pass with no race detector warnings.
4. Optional integration verification: the `testlab/nat-relay/run-test.sh` Docker-compose lab
   can be used to confirm end-to-end stability with simulated NAT, but is not a CI gate.

## Estimated Complexity
low
