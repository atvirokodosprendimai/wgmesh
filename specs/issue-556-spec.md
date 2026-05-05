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

### Task 1: Fix `shouldRelayPeerWithSubnets` — collapse the stale-handshake block to always relay (addresses both Root Cause 1 and Root Cause 2)

**File:** `pkg/daemon/daemon.go`

**Location:** `shouldRelayPeerWithSubnets`, inside the
`if ts, ok := handshakes[peer.WGPubKey]; ok && ts > 0 {` block, after the
`time.Since(lastHandshake) < HandshakeStaleAfter` guard.

This is the only code path that needs to change. The no-handshake path (further down in the
same function, lines ≈682–688) already correctly relays transitive peers without any `"dht"`
guard, so that block does **not** need to be modified.

**Current code (the stale-handshake block after the "direct path is working" guard):**

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

**Replacement (single `return true`):**

```go
// Handshake is stale (> HandshakeStaleAfter). Fall back to relay so
// traffic is not blacked out while WireGuard attempts to re-establish.
//
// HandshakeStaleAfter (150 s) is far longer than any WG rekey window
// (< 5 s), so a stale timestamp reliably indicates a real connectivity
// failure — not a transient rekey.
//
// The second guard previously excluded peers discovered via "dht" (added
// when a control-path UDP exchange succeeds without a WG handshake).
// Removing it means the confirmed WireGuard handshake — not the UDP
// control exchange — is the sole gate for dropping relay.
//
// The relay→direct hysteresis (RelayHysteresisThreshold cycles) will
// transition back to direct once a fresh handshake is confirmed.
return true
```

**Exact diff:**

```diff
-			// Handshake stale — but only relay if NAT situation warrants it.
-			// For cone/unknown NAT or IPv6, the staleness is likely transient
-			// (e.g., WG rekey timing). Only relay for symmetric+symmetric.
-			if d.localNode.NATType == "symmetric" && peer.NATType == "symmetric" {
-				return true
-			}
-			// For transitive-only peers with stale handshake, relay to avoid blackhole
-			if hasDiscoveryMethod(peer.DiscoveredVia, "dht-transitive") &&
-				!hasDiscoveryMethod(peer.DiscoveredVia, "dht") {
-				return true
-			}
-			return false
+			// Handshake is stale (> HandshakeStaleAfter). Fall back to relay so
+			// traffic is not blacked out while WireGuard attempts to re-establish.
+			//
+			// HandshakeStaleAfter (150 s) is far longer than any WG rekey window
+			// (< 5 s), so a stale timestamp reliably indicates a real connectivity
+			// failure — not a transient rekey.
+			//
+			// The second guard previously excluded peers discovered via "dht" (added
+			// when a control-path UDP exchange succeeds without a WG handshake).
+			// Removing it means the confirmed WireGuard handshake — not the UDP
+			// control exchange — is the sole gate for dropping relay.
+			//
+			// The relay→direct hysteresis (RelayHysteresisThreshold cycles) will
+			// transition back to direct once a fresh handshake is confirmed.
+			return true
```

### Task 2: Update existing tests that asserted the old (incorrect) behaviour

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

### Task 3: Add new regression tests for the two root causes

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

### Task 4: Verify with `go test -race ./pkg/daemon/...`

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

- `pkg/daemon/daemon.go` — `shouldRelayPeerWithSubnets` function (Task 1)
- `pkg/daemon/relay_test.go` — update `TestShouldRelayPeer_StaleHandshake_ConeCone_NoRelay`
  and add two new regression tests (Tasks 2 and 3)

## Test Strategy

1. Run existing `TestShouldRelayPeer_*` tests — all must pass after Task 2 (one test renamed
   and inverted, all others unchanged).
2. Two new regression tests (Task 3) cover the exact failure modes reported in Issue #556.
3. `go test -race ./pkg/daemon/...` must pass with no race detector warnings.
4. Optional integration verification: the `testlab/nat-relay/run-test.sh` Docker-compose lab
   can be used to confirm end-to-end stability with simulated NAT, but is not a CI gate.

## Estimated Complexity
low
