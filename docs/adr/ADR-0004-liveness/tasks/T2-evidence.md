# Task ADR-0004-T2: Feed the handshake and the probe into the store and move every liveness reader to `Alive` (#850)

**Depends-on:** T1
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** evidence writers in reconcile and `probePeer`, `GetProbeable` in the probe loop, `TestNoDirectLastSeenAge`, `wgmesh_liveness_disagree_total{site}`
**Consumes:** `Assess`, `PeerInfo.Alive`, `ObserveDataPlane`, `GetProbeable`, `EnforceLiveness`, the disagree hook (T1)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

A peer with a recent WireGuard handshake, or one that answers the mesh probe through the tunnel, stays `Alive` while its announcements are silent, so a relay-only pair no longer cycles five minutes up and five down, and no production file outside `pkg/node` computes the age of a `LastSeen` itself.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/liveness.go` | add | `observeHandshakes(store, handshakes, now)`: calls `ObserveDataPlane` for each entry with a non-zero time |
| `pkg/daemon/daemon.go` | edit | `reconcile` (`:534-559`) reads handshakes once, calls `observeHandshakes`, and passes the same map to `buildDesiredPeerConfigsWithHandshakes` (`:566-569`); `probePeer` (`:1121-1155`) calls `ObserveDataPlane` on `pong`; `probePeersOverMesh` (`:1065`) iterates `GetProbeable`; `:586` and `:1090` use `p.Alive` for the age term |
| `pkg/discovery/dht.go` | edit | `:875` and `:1259` ask `peer.Alive(time.Now())` |
| `pkg/daemon/metrics.go` | edit | `wgmesh_liveness_disagree_total{site}` and the hook wiring at daemon start |
| `pkg/daemon/liveness_guard_test.go` | add | `TestNoDirectLastSeenAge` |
| `pkg/daemon/liveness_test.go`, `pkg/discovery/dht_liveness_test.go` | add | tests below |

Selection: the reconcile loop is the caller of `observeHandshakes`; reverting that call turns `TestReconcileRecordsHandshakes` red; the guard fails if a reader goes back to `Sub(x.LastSeen)`.

## Ordered Steps

1. [S1] Add the tests below with compiling stubs; run red. `TestHandshakeKeepsPeerActiveWithSilentControlPlane` sets `EnforceLiveness = true`, stores a peer whose `LastSeen` is 20 minutes old, records a handshake 30 seconds old, and asserts the peer is in `GetActive`, in the desired set, and not removed by `CleanupStale`.
2. [S2] Add `observeHandshakes` and call it from `reconcile` with the map `buildDesiredPeerConfigs` already read (one `wg show` per reconcile, as today).
3. [S3] Call `ObserveDataPlane` on a successful probe; make the probe loop visit `GetProbeable`.
4. [S4] Move `daemon.go:586` and `:1090` and `dht.go:875` and `:1259` to `Alive`; keep the literal windows (90 s, 45 s, 2 min) as the callers' own thresholds on top of `Alive`.
5. [S5] Add `TestNoDirectLastSeenAge`: it parses every non-test `.go` file outside `pkg/node` and fails on a call to `time.Since` or a `.Sub` method whose argument is a selector named `LastSeen`; it states in its doc that an age computed through a local variable is not seen. Re-run `grep -rnE "(Sub|Since)\([a-zA-Z.]*LastSeen\)" --include=*.go . | grep -v _test.go | grep -v pkg/node`, which prints nothing, and record it in the commit message.
6. [S6] Wire the disagree hook to the counter; run the mutants with `adr-verify --mutant`: `observeHandshakes` is not called; the probe does not record; the guard's name check is removed; `dht.go:1259` returns to `time.Since(p.LastSeen)`. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/daemon -run '^(TestReconcileRecordsHandshakes|TestProbeSuccessRecordsDataPlane|TestHandshakeKeepsPeerActiveWithSilentControlPlane|TestRelayCandidateUsesAlive|TestProbeLoopVisitsSuspectPeers|TestNoDirectLastSeenAge)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 6 ] \
 && b=$(go test ./pkg/discovery -run '^TestIntroducerFreshnessUsesAlive$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -ge 1 ] \
 && [ "$(grep -rnE '(Sub|Since)\([a-zA-Z.]*LastSeen\)' --include=*.go . | grep -v _test.go | grep -vc pkg/node)" = 0 ] \
 && go test -race ./pkg/node ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestReconcileRecordsHandshakes` | `pkg/daemon/liveness_test.go` | a handshake map given to the reconcile path sets `LastDataPlane` for the peers in it and not for a zero entry | — | S1, S2 |
| `TestProbeSuccessRecordsDataPlane` | `pkg/daemon/liveness_test.go` | a `pong` records the time; a failed probe does not | — | S1, S3 |
| `TestHandshakeKeepsPeerActiveWithSilentControlPlane` | `pkg/daemon/liveness_test.go` | the #850 scenario above | — | S1, S2 |
| `TestRelayCandidateUsesAlive` | `pkg/daemon/liveness_test.go` | an introducer with an old `LastSeen` and a fresh handshake is a relay candidate; with neither it is not | — | S1, S4 |
| `TestProbeLoopVisitsSuspectPeers` | `pkg/daemon/liveness_test.go` | a peer 7 minutes silent is probed, and a `pong` makes it `Alive` again | — | S1, S3 |
| `TestIntroducerFreshnessUsesAlive` | `pkg/discovery/dht_liveness_test.go` | `isAutoIntroducerCandidate` accepts a peer with an old `LastSeen` and a fresh data-plane time | — | S1, S4 |
| `TestNoDirectLastSeenAge` | `pkg/daemon/liveness_guard_test.go` | a production file outside `pkg/node` that calls `time.Since(p.LastSeen)` fails the guard (run on a temporary file in the test) | — | S1, S5 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the seven tests |
| 2 — something selects it | `reconcile` and `probePeer` call the writers; reverting either turns a named test red |
| 3 — the caller can discover it | the counter is on `/metrics`; T7 documents it |
| 4 — it is used | from N+1 the verdict acts; from N `wgmesh_liveness_disagree_total{site}` shows each peer that would have been kept |

## Mutation Log

## Invariants

- One `wg show latest-handshakes` per reconcile, as today; no new fork.
- With `EnforceLiveness` false the daemon's decisions are the legacy ones; only counters and the new field change.
- `shouldRelayPeerWithSubnets` is not edited (the `dht-transitive` prior stays, ADR-0002 Decision 9).

## Risks

- A peer kept alive only by a probe that routes through a relay: the probe dials the peer's mesh IP and needs `pong` from its server, so a relay cannot answer for it.
- `daemon.go` conflicts with ADR-0002 T3 and T4; see the ADR's Ordering.

## Stop Condition

Stop and ask if `TestReconcileRecordsHandshakes` needs a second `wg show` per reconcile, or if the probe loop's per-peer session handling (`getOrDialProbeSession`) cannot be reached for a `Suspect` peer without changing the dial rules.

## Out of Scope

- Sharing one `wg show` between the health loop, the probe and discovery (permanent: boundary: a performance change; see the ADR).
- The restore grace (T3) and the wake grace (T4).

## Verification Log
