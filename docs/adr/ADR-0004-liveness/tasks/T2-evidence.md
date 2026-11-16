# Task ADR-0004-T2: Feed the handshake and the probe into the store and move every liveness reader to `Alive` (#850)

**Depends-on:** T1
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** evidence writers in reconcile and `probePeer`, `GetProbeable` in the probe loop, `TestNoDirectLastSeenAge`, `wgmesh_liveness_disagree_total{site}`
**Consumes:** `Assess`, `PeerInfo.Alive`, `PeerInfo.LastAlive`, `ObserveDataPlane`, `GetProbeable`, `EnforceLiveness`, `SetLivenessDisagreeHook` (T1)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

A peer with a recent WireGuard handshake, or one that answers the mesh probe through the tunnel, stays `Alive` while its announcements are silent, so a relay-only pair no longer cycles five minutes up and five down, and no production file outside `pkg/node` computes the age of a `LastSeen` itself.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/liveness.go` | add | `observeHandshakes(store, handshakes, now)`: calls `ObserveDataPlane` for each entry with a non-zero time |
| `pkg/daemon/daemon.go` | edit | `reconcile` (`:670-695`) does not read handshakes today (`buildDesiredPeerConfigs`, `:702`, reads them at `:703`); **new S2 order in `reconcile`**: first `wireguard.GetLatestHandshakes`, then `observeHandshakes`, then `GetActive()` (`:673`), then `buildDesiredPeerConfigsWithHandshakes(peers, handshakes)` (`:707`); observing before `GetActive` matters, because a peer whose only evidence is the handshake must already be `Alive` when the desired set is built. `buildDesiredPeerConfigs` may become test-only: grep its callers before touching it, and delete it only if none is left. `probePeer` (`:1258-1292`) calls `ObserveDataPlane` on `pong`, beside `SetLatency` (`:1290`); `probePeersOverMesh` (`:1202`, `GetActive` at `:1203`) iterates `GetProbeable`; `:722` (relay candidate age), `:1227` (the 45-second "new" term) use `p.LastAlive(now)`; `NewDaemon` wires the store's hook to the counter |
| `pkg/discovery/dht.go` | edit | `:872` (skip a recent rendezvous) and `:1270` (`isAutoIntroducerCandidate(p, handshakes)`, `:1266`, which already takes the handshake map and also requires a handshake younger than 2 minutes, `:1280-1282`) use `now.Sub(p.LastAlive(now))` against their 2-minute window |
| `pkg/daemon/metrics.go` | edit | `wgmesh_liveness_disagree_total{site}` following `admitWarnings` (`:56`), registered in `registerMetrics` (`:81-95`) with the sites pre-seeded like `winner-differs` (`:92`); `recordLivenessDisagree(site)` is the hook body (precedent: `Policy.OnWarn` to `RecordAdmitWarning`, `:160`; do not duplicate `wgmesh_admit_warnings_total`) |
| `pkg/daemon/liveness_guard_test.go` | add | `TestNoDirectLastSeenAge`: the go/parser walk and `lastIdent` helper are copied from `pkg/node/admit_guard_test.go` (the helpers are package `node`, so they are duplicated here, not imported) |
| `pkg/daemon/liveness_test.go`, `pkg/discovery/dht_liveness_test.go` | add | tests below |

Selection: the reconcile loop is the caller of `observeHandshakes`; reverting that call turns `TestReconcileRecordsHandshakes` red; the guard fails if a reader goes back to `Sub(x.LastSeen)`.

## Ordered Steps

1. [S1] Add the tests below with compiling stubs; run red. `TestHandshakeKeepsPeerActiveWithSilentControlPlane` sets `EnforceLiveness.Store(true)` (restored by `t.Cleanup`), stores a peer whose `LastSeen` is 20 minutes old, records a handshake 30 seconds old, and asserts the peer is in `GetActive`, in the desired set, and not removed by `CleanupStale`. Daemon-package tests cannot set the unexported `ps.now`: tests that need an injected store clock live in `pkg/node` (T1), the daemon tests here use wall-clock offsets.
2. [S2] Add `observeHandshakes`; in `reconcile` call `wireguard.GetLatestHandshakes(d.config.InterfaceName)` first, then `observeHandshakes`, then `GetActive()`, then `buildDesiredPeerConfigsWithHandshakes(peers, handshakes)` (one `wg show` per reconcile, as today: the read moves from `buildDesiredPeerConfigs` to the top of `reconcile`). New Daemon fields this step adds are nil-safe: many tests build `&Daemon{...}` literals with nil maps (`health_test.go:38`, `relay_test.go`), so a map is created lazily where it is first written, never assumed in `NewDaemon` alone.
3. [S3] Call `ObserveDataPlane` on a successful probe. Make the probe loop visit `GetProbeable` peers that are still configured (in `lastAppliedPeerConfigs`, read under `appliedMu`) or relay-routed (`isRelayRoutedPeer`); a `Suspect` peer that reconcile already removed from the interface is skipped, because the desired set is built from `GetActive` and this task does not add `Suspect` peers to it (ADR, Existing Primitives).
4. [S4] Move `daemon.go:722` and `:1227` and `dht.go:872` and `:1270` to `LastAlive`. The windows (90 s, 45 s, 2 min) stay the callers' own constants: `Alive` returns a liveness, not a window, so each caller asks one helper on `PeerInfo` for the age to use: the legacy `LastSeen` age while `EnforceLiveness` is false (with the same comparison operator as today, and a count under its own site label `relay-candidate`, `probe-enforce`, `rendezvous-skip`, `introducer-candidate` when the evidence-based decision would differ) and the `LastAlive` age when it is true.
5. [S5] Add `TestNoDirectLastSeenAge` in `pkg/daemon/liveness_guard_test.go`: it parses every non-test `.go` file under `pkg` outside `pkg/node` and fails on a call to `time.Since` or a `.Sub` method whose argument is a selector named `LastSeen`; it states in its doc that an age computed through a local variable is not seen. Re-run `grep -rnE "(Sub|Since)\([a-zA-Z.]*LastSeen\)" --include=*.go . | grep -v _test.go | grep -v pkg/node`, which prints nothing, and record it in the commit message.
6. [S6] Wire the hook in `NewDaemon` to the counter; run the mutants with `adr-verify --mutant`: `observeHandshakes` is not called; `reconcile` observes after `GetActive`; the probe does not record; the guard's name check is removed; `dht.go:1270` returns to `time.Since(p.LastSeen)`. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/daemon -run '^(TestReconcileRecordsHandshakes|TestReconcileObservesBeforeGetActive|TestProbeSuccessRecordsDataPlane|TestHandshakeKeepsPeerActiveWithSilentControlPlane|TestRelayCandidateUsesAlive|TestProbeLoopVisitsSuspectPeers|TestNoDirectLastSeenAge)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 7 ] \
 && b=$(go test ./pkg/discovery -run '^TestIntroducerFreshnessUsesAlive$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -ge 1 ] \
 && [ "$(grep -rnE '(Sub|Since)\([a-zA-Z.]*LastSeen\)' --include=*.go . | grep -v _test.go | grep -vc pkg/node)" = 0 ] \
 && go test -race ./pkg/node ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestReconcileRecordsHandshakes` | `pkg/daemon/liveness_test.go` | a handshake map given to the reconcile path sets `LastDataPlane` for the peers in it and not for a zero entry | — | S1, S2 |
| `TestReconcileObservesBeforeGetActive` | `pkg/daemon/liveness_test.go` | a peer with an old `LastSeen` and a fresh handshake is in the peer list that `reconcile` passes on (so in the desired set) in the same pass that records the handshake | — | S1, S2 |
| `TestProbeSuccessRecordsDataPlane` | `pkg/daemon/liveness_test.go` | a `pong` records the time; a failed probe does not | — | S1, S3 |
| `TestHandshakeKeepsPeerActiveWithSilentControlPlane` | `pkg/daemon/liveness_test.go` | the #850 scenario above | — | S1, S2 |
| `TestRelayCandidateUsesAlive` | `pkg/daemon/liveness_test.go` | an introducer with an old `LastSeen` and a fresh handshake is a relay candidate (`now.Sub(p.LastAlive(now))` against the 90-second window); with neither it is not | — | S1, S4 |
| `TestProbeLoopVisitsSuspectPeers` | `pkg/daemon/liveness_test.go` | two cases: a peer 7 minutes silent that is still in `lastAppliedPeerConfigs` (or relay-routed) is probed, and a `pong` makes it `Alive` again; a 7-minute-silent peer that is neither is not probed | — | S1, S3 |
| `TestIntroducerFreshnessUsesAlive` | `pkg/discovery/dht_liveness_test.go` | `isAutoIntroducerCandidate(p, handshakes)` accepts a peer with an old `LastSeen`, a fresh data-plane time and a fresh handshake in the map (the test sets up the `controlPeers` entry and the handshake map the function reads), and refuses it with no fresh evidence | — | S1, S4 |
| `TestNoDirectLastSeenAge` | `pkg/daemon/liveness_guard_test.go` | a production file outside `pkg/node` that calls `time.Since(p.LastSeen)` fails the guard (run on a temporary file in the test) | — | S1, S5 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the eight tests |
| 2 — something selects it | `reconcile` and `probePeer` call the writers; reverting either turns a named test red |
| 3 — the caller can discover it | the counter is on `/metrics`; T7 documents it |
| 4 — it is used | from N+1 the verdict acts; from N `wgmesh_liveness_disagree_total{site}` shows each peer that would have been kept |

## Mutation Log
- 2026-11-16 · 82a6211* · mutant killed · exit 1 · `pkg/daemon/liveness.go` · observeHandshakes not called · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143
- 2026-11-16 · 82a6211* · mutant killed · exit 1 · `pkg/daemon/liveness.go` · observe after GetActive · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143
- 2026-11-16 · 82a6211* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · probe does not record · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143
- 2026-11-16 · 82a6211* · mutant inconclusive · exit 1 · `pkg/daemon/liveness_guard_test.go` · guard name check removed · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143
  ```
  the fence failed on a build/parse error, not an assertion
  ```
- 2026-11-16 · 82a6211* · mutant killed · exit 1 · `pkg/discovery/dht.go` · dht 1270 back to LastSeen · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143
- 2026-11-16 · 82a6211* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · probe loop back to GetActive · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143
- 2026-11-16 · 8511cd9* · mutant killed · exit 1 · `pkg/daemon/liveness.go` · observeHandshakes not called · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143
- 2026-11-16 · 8511cd9* · mutant killed · exit 1 · `pkg/daemon/liveness.go` · observe after GetActive · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143
- 2026-11-16 · 8511cd9* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · probe does not record · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143
- 2026-11-16 · 8511cd9* · mutant killed · exit 1 · `pkg/daemon/liveness_guard_test.go` · guard name check inverted · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143
- 2026-11-16 · 8511cd9* · mutant killed · exit 1 · `pkg/node/liveness.go` · outside readers act by LastAlive while the switch is off · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143
- 2026-11-16 · 8511cd9* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · probe loop back to GetActive · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143
- 2026-11-16 · 8511cd9* · mutant killed · exit 1 · `pkg/discovery/dht.go` · introducer window widened · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143
- 2026-11-16 · 8511cd9* · mutant killed · exit 1 · `pkg/node/liveness.go` · disagreement never counted · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143

## Invariants

- One `wg show latest-handshakes` per reconcile, as today; no new fork (the read moves from `buildDesiredPeerConfigs` to the top of `reconcile`).
- With `EnforceLiveness` false the daemon's decisions are the legacy ones; only counters and the new field change, and the probe loop visits `GetProbeable` peers (more probes, never fewer; it refuses nothing).
- `shouldRelayPeerWithSubnets` is not edited (the `dht-transitive` prior stays, ADR-0002 Decision 9).
- Any new `Daemon` map is nil-safe (created lazily on first write).
- No field, variable or counter accessed by more than one goroutine is bare: `atomic.*` or a lock on every access (M 2026-11-16); the task has a `-race` test for each such field it adds. This task adds no shared field of its own: the counter follows the `admitWarnings` pattern under its existing lock, and `ObserveDataPlane` is covered by T1's `TestLivenessStateIsRaceFree`.

## Risks

- A peer kept alive only by a probe that routes through a relay: the probe dials the peer's mesh IP and needs `pong` from its server, so a relay cannot answer for it.
- `daemon.go` conflicts with later work on `reconcile`; see the ADR's Ordering.

## Stop Condition

Stop and ask if `TestReconcileRecordsHandshakes` needs a second `wg show` per reconcile, or if the probe loop's per-peer session handling (`getOrDialProbeSession`) cannot be reached for a configured `Suspect` peer without changing the dial rules.

## Out of Scope

- Sharing one `wg show` between the health loop, the probe and discovery (permanent: boundary: a performance change; see the ADR).
- The restore grace (T3) and the wake grace (T4).

## Verification Log
- 2026-11-16 · 82a6211* · exit 1 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:2140 · test-lock-sha256:c0dcdd39b2cefc9d3e41d5401dd82fa9c9292ed058a1aa84aff93d1ca293008f · test-lock-file:docs/adr/locks/c0dcdd39b2cefc9d3e41d5401dd82fa9c9292ed058a1aa84aff93d1ca293008f.lock
  ```
  --- last 10 line(s) of stderr (of 52 after folding 52 raw)
      liveness_test.go:251: pings = 0, want 1
      liveness_test.go:256: Alive after the loop = false, want true
  === RUN   TestProbeLoopVisitsSuspectPeers/neither_configured_nor_relay_routed
  --- FAIL: TestProbeLoopVisitsSuspectPeers (0.00s)
      --- FAIL: TestProbeLoopVisitsSuspectPeers/still_configured (0.00s)
      --- FAIL: TestProbeLoopVisitsSuspectPeers/relay_routed (0.00s)
      --- PASS: TestProbeLoopVisitsSuspectPeers/neither_configured_nor_relay_routed (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/daemon	0.050s
  FAIL
  ```
- 2026-11-16 · 82a6211* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:13344
- 2026-11-16 · 82a6211* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:11992
- 2026-11-16 · 82a6211* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:14208
- 2026-11-16 · 82a6211* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:12655
- 2026-11-16 · 82a6211* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:14522
- 2026-11-16 · 82a6211* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:14905
- 2026-11-16 · 82a6211* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:9432
- 2026-11-16 · 8511cd9* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:1 · test-lock-sha256:c93d3e5363f2ecaae3aec957cab7af0218a32beed514b66b06ba6c5dea005ce3 · test-lock-file:docs/adr/locks/c93d3e5363f2ecaae3aec957cab7af0218a32beed514b66b06ba6c5dea005ce3.lock · test-lock-kind:replace
- 2026-11-16 · 8511cd9* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:13596
- 2026-11-16 · 8511cd9* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:12319
- 2026-11-16 · 8511cd9* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:12615
- 2026-11-16 · 8511cd9* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:13813
- 2026-11-16 · 8511cd9* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:14107
- 2026-11-16 · 8511cd9* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:13877
- 2026-11-16 · 8511cd9* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:21033
- 2026-11-16 · 8511cd9* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:15612
- 2026-11-16 · 8511cd9* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:8256
- 2026-11-16 · ffb9f29* · exit 0 · `set -o pipefail …` · acceptance-sha256:8d77cd2b3ef8f4a7840d623264fcd1972d5426e32edef7a5de04cc37ea358143 · ms:8447
