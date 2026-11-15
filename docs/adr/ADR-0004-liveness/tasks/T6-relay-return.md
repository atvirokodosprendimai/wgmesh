# Task ADR-0004-T6: Keep a relayed peer on the interface without AllowedIPs so a direct handshake can bring it back (#852)

**Depends-on:** T2
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `desiredPeerConfig.probeOnly`, `wireguard.SetPeerArgs`, the explicit AllowedIPs clear, `daemon.EnforceRelayProbe`, `wgmesh_relay_route_age_seconds`
**Consumes:** the evidence writers and `Alive` (T2)
**Data dependency:** hermetic; S1 and S6 need a Linux host with `wg` and root
**Proof map:** v1

## Goal

A peer that is relayed through an introducer stays configured on the WireGuard interface with its endpoint and an empty AllowedIPs list, so its direct handshakes are recorded, `shouldRelayPeerWithSubnets` sees a fresh handshake and returns false before it consults the `dht-transitive` prior, and the 3-cycle hysteresis returns the peer to the direct path; a peer that never handshakes stays relayed.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/wireguard/apply.go` | edit | `SetPeerArgs(...) []string` builds the `wg set` arguments (pure); an `allowedIPs` of exactly `"-"` is sent as `allowed-ips ""` (clear), an empty string still omits the flag as today (`:152-154`); `SetPeer` calls `SetPeerArgs` |
| `pkg/daemon/daemon.go` | edit | `desiredPeerConfig` (`:561-564`) gains `probeOnly`; `buildDesiredPeerConfigsWithHandshakes` (`:571-648`) adds the relayed peer to `desired` with `probeOnly` when `EnforceRelayProbe` is true; `applyDesiredPeerConfigs` (`:773-828`) no longer skips an entry for an empty allowed list when it is `probeOnly`, and sends `"-"`; `lastAppliedPeerConfigs` signature includes the mode so a switch from probe-only to direct re-sends |
| `pkg/daemon/metrics.go` | edit | `wgmesh_relay_route_age_seconds{peer}`: seconds since the peer entered `relayRoutes` (`daemon.go:539-542`), exported from N |
| `pkg/wireguard/apply_args_test.go`, `pkg/daemon/relay_probe_test.go` | add | tests below |

Selection: `applyDesiredPeerConfigs` reads `probeOnly`; `EnforceRelayProbe` selects the whole behaviour; a mutant that adds the relayed peer's mesh IP to its allowed list turns `TestProbeOnlyPeerCarriesNoAllowedIPs` red.

## Ordered Steps

1. [S2] Add the tests below with compiling stubs; run red. `TestProbeOnlyHandshakeReturnsDirect` builds a daemon with one introducer and one relayed peer, runs `buildDesiredPeerConfigsWithHandshakes` three times with a handshake 10 seconds old, and asserts the peer is in the relay route for cycles 1 and 2 and direct from cycle 3 (`RelayHysteresisThreshold`).
2. [S1] On a scratch interface on a Linux host, run `wg set <iface> peer <key> allowed-ips ""` on a peer that has an AllowedIPs list and `wg show <iface> allowed-ips`; record the result (and run the same on a macOS host, or record that it was not run). The result decides whether `SetPeerArgs` sends the clear or the probe-only call is remove-then-add. [proof: human: needs a real interface; adr-verify cannot see it]
3. [S3] Add `SetPeerArgs` and switch `SetPeer` to it with no change of behaviour for the existing callers (`TestSetPeerArgsUnchangedForExistingCallers` pins the argument list for the three existing call shapes).
4. [S4] Add `probeOnly` and the `EnforceRelayProbe` branch in the desired set and the apply step; with the switch false the relayed peer is removed as today.
5. [S5] Add the relay-age gauge, set when a peer enters `relayRoutes` and deleted when it leaves.
6. [S6] Run the mutants with `adr-verify --mutant`: `probeOnly` entries carry the peer's mesh IP; the apply step skips an empty list again; the switch is read inverted; the signature ignores the mode. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/daemon -run '^(TestBuildDesiredKeepsRelayedPeerProbeOnly|TestProbeOnlyPeerCarriesNoAllowedIPs|TestProbeOnlyHandshakeReturnsDirect|TestNeverHandshakenStaysRelayed|TestRelayProbeOffByDefault|TestRelayRouteAgeMetric|TestShouldRelayPeer_TransitiveAndDHT_KeepsRelay)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 7 ] \
 && b=$(go test ./pkg/wireguard -run '^(TestSetPeerArgsClearsAllowedIPs|TestSetPeerArgsUnchangedForExistingCallers)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -ge 2 ] \
 && go test -race ./pkg/wireguard ./pkg/node ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestBuildDesiredKeepsRelayedPeerProbeOnly` | `pkg/daemon/relay_probe_test.go` | with the switch on, a relayed peer is in `desired` with `probeOnly` and an empty allowed set; the relay entry still holds its mesh IP | — | S2, S4 |
| `TestProbeOnlyPeerCarriesNoAllowedIPs` | `pkg/daemon/relay_probe_test.go` | the apply step sends `"-"` for it, never a CIDR | — | S2, S4 |
| `TestProbeOnlyHandshakeReturnsDirect` | `pkg/daemon/relay_probe_test.go` | the #852 scenario above, for a peer whose `DiscoveredVia` holds `dht-transitive` | — | S2, S4 |
| `TestNeverHandshakenStaysRelayed` | `pkg/daemon/relay_probe_test.go` | 100 cycles without a handshake keep the peer relayed (the `dht-transitive` prior still acts) | — | S2, S4 |
| `TestRelayProbeOffByDefault` | `pkg/daemon/relay_probe_test.go` | with the switch false the relayed peer is not in `desired`, as today | — | S2, S4 |
| `TestRelayRouteAgeMetric` | `pkg/daemon/relay_probe_test.go` | the gauge rises while the route lives and is deleted when the peer goes direct | — | S2, S5 |
| `TestShouldRelayPeer_TransitiveAndDHT_KeepsRelay` | `pkg/daemon/relay_test.go` | the existing test of the `dht-transitive` prior still passes unchanged (ADR-0002 Decision 9) | — | S2 |
| `TestSetPeerArgsClearsAllowedIPs` | `pkg/wireguard/apply_args_test.go` | `"-"` yields `allowed-ips` followed by an empty argument | — | S2, S3 |
| `TestSetPeerArgsUnchangedForExistingCallers` | `pkg/wireguard/apply_args_test.go` | an empty list omits the flag, a list is passed as today, the keepalive is last | — | S2, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the nine tests |
| 2 — something selects it | `applyDesiredPeerConfigs` reads `probeOnly`; `EnforceRelayProbe` selects it; the inverted-switch mutant turns `TestRelayProbeOffByDefault` red |
| 3 — the caller can discover it | the relay-age gauge from N, the release note from T7 |
| 4 — it is used | from N+2 on every relayed peer; N+1 operators read the gauge first |

## Mutation Log

## Invariants

- An empty AllowedIPs list is the only difference between a probe-only peer and a direct one; no data packet from it is accepted by WireGuard.
- `shouldRelayPeerWithSubnets` and the label `dht-transitive` are not edited (ADR-0002 Decision 9).
- The hysteresis of 3 cycles applies to the return.
- With `EnforceRelayProbe` false the interface holds what it holds today.

## Risks

- `wg set ... allowed-ips ""` may not clear on macOS userspace WireGuard; S1 decides, and the fall-back is remove-then-add of the peer, which loses its handshake time once.
- A keepalive and handshake attempt to every relayed peer every 25 seconds (accepted in M2).
- `applyDesiredPeerConfigs` and `wireguard.SetPeer` are in the apply-step group's area (ADR-0003 draft); this task starts after whichever of the two lands first and is rebased on it.

## Stop Condition

Stop and ask if S1 shows that an empty AllowedIPs list still lets WireGuard accept data from the peer, or if the apply-step ADR has changed `desiredPeerConfig` so that a peer without AllowedIPs cannot be represented.

## Out of Scope

- The choice of relay (`selectRelayForPeer`) and the relay candidate rules other than the age term (T2).
- A probe window instead of a permanent probe-only entry (M2, the rejected option; permanent: boundary: M chose the permanent probe-only entry).

## Verification Log
