# Task ADR-0007-T3: Move every `wg` state reader to the snapshot with a stated staleness

**Depends-on:** T1
**Resolved:** ADR-0002 is merged on main (f3860c6, 2026-11-16; `git cat-file -e origin/main:pkg/node/spec_admit_test.go` exits 0)
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** every non-test caller of `wireguard.GetPeers`, `GetLatestHandshakes`, `GetPeerTransfers` reads through `ReadState` with the `maxAge` of ADR-0007 Decision 5
**Consumes:** `wireguard.ReadState`, `wireguard.SetWGPath` (T1)
**Data dependency:** needs a running node for step S7 only (the unit tests are hermetic); the sign-off line records the host, the date and the strace output
**Proof map:** v1

## Goal

At steady state the daemon forks `wg show` about once per reconcile tick instead of about once and a half per second, and no decision of the probe or health loop depends on data older than 5 seconds.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/daemon.go` | edit | `reconcile` reads `ReadState(iface, 0)` once and passes it to `buildDesiredPeerConfigsWithHandshakes` and `applyDesiredPeerConfigs` (`:567`, `:774`); `probePeersOverMesh` (`:1068`) and `checkPeerHealth` (`:1285`, `:1289`) read with `ReconcileInterval` |
| `pkg/discovery/dht.go` | edit | `getPeerHandshakeTS` (`:1132`) takes a `maxAge`; `tryRendezvousForPeer` (`:960`) and the introducer ranking (`:1298`) pass `ReconcileInterval` |
| `pkg/discovery/exchange.go` | edit | `getLatestHandshake` (`:897`) keeps `maxAge` 0 for the punch wait (`waitForHandshake`) |
| `pkg/daemon/wg_fork_test.go` | add | fork counting through `SetWGPath` |
| `pkg/discovery/wg_fork_test.go` | add | the discovery side |

Selection: the acceptance fence counts the remaining direct `Get*` callers in non-test code and fails while any exists; deleting one `ReadState` call in `daemon.go` turns the fork-count test red.

## Ordered Steps

1. [S1] Add the two test files with the six tests and a fake `wg` that appends one line per `show` call to a file and prints a fixed dump, using `wireguard.SetWGPath`; run red (today `reconcile`, `probePeersOverMesh` and `checkPeerHealth` fork five times).
2. [S2] `reconcile`: one `ReadState(iface, 0)`; the desired-config builder and the removal step take the `State` as an argument (`buildDesiredPeerConfigs` keeps its signature and reads for its own callers).
3. [S3] `probePeersOverMesh` and `checkPeerHealth`: `ReadState(iface, ReconcileInterval)`. Add the staleness next to each call as a one-line comment naming the 150-second threshold it is compared with.
4. [S4] `pkg/discovery`: `getPeerHandshakeTS(iface, key, maxAge)`; the punch wait stays at 0, the per-peer stale check and the introducer ranking use `ReconcileInterval`.
5. [S5] [proof: acceptance] Check `grep -rnE 'wireguard\.(GetPeers|GetLatestHandshakes|GetPeerTransfers)\(' --include=*.go . | grep -v _test.go | wc -l` is 0 (it is 8 on 2026-11-15 at `b8b83f3`).
6. [S6] Run the mutants with `adr-verify --mutant`: reconcile reads with `maxAge` 1 hour; the probe reads with 0; the punch wait reads with `ReconcileInterval`; one caller left on the legacy reader. [proof: mutation]
7. [S7] On one running node, count `execve` of `wg` for 60 seconds before and after this change (`strace -f -c -e trace=execve -p <daemon pid>`, or `perf trace`), and record the two numbers, the host, the peer count and the date in the commit message and the sign-off. The ADR's 90 and 12 a minute are computed from call sites and must be replaced by these. [proof: human: the number needs a live daemon that adr-verify cannot start]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/daemon ./pkg/discovery -run '^(TestOneShowForkPerReconcileTick|TestProbeAndHealthReuseSnapshotInsideReconcileInterval|TestProbeRefreshesSnapshotOlderThanReconcileInterval|TestReconcileAlwaysReadsFresh|TestStaleCheckReadsOneSnapshotForAllPeers|TestPunchWaitForksEveryPoll)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 6 ] && [ "$(grep -rnE 'wireguard\.(GetPeers|GetLatestHandshakes|GetPeerTransfers)\(' --include=*.go . | grep -v _test.go | wc -l)" -eq 0 ] && go test -race ./pkg/wireguard ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestOneShowForkPerReconcileTick` | `pkg/daemon/wg_fork_test.go` | one `reconcile`, one `probePeersOverMesh` and one `checkPeerHealth` inside 5 s log exactly one `show` fork | — | S1, S2, S3 |
| `TestProbeAndHealthReuseSnapshotInsideReconcileInterval` | `pkg/daemon/wg_fork_test.go` | after a fresh read, 100 probe ticks and 10 health checks log no further `show` | — | S1, S3 |
| `TestProbeRefreshesSnapshotOlderThanReconcileInterval` | `pkg/daemon/wg_fork_test.go` | with the stored result 6 s old, the probe forks once and a handshake that changed in the dump is seen | — | S1, S3 |
| `TestReconcileAlwaysReadsFresh` | `pkg/daemon/wg_fork_test.go` | with a result 1 s old, `reconcile` still forks and removes a peer the new dump no longer lists | — | S1, S2 |
| `TestStaleCheckReadsOneSnapshotForAllPeers` | `pkg/discovery/wg_fork_test.go` | `checkStaleHandshakes` over five candidate peers logs at most one `show` | — | S1, S4 |
| `TestPunchWaitForksEveryPoll` | `pkg/discovery/wg_fork_test.go` | `waitForHandshake` with a 1 s timeout forks on each 250 ms poll and returns the new handshake within one poll | — | S1, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the six tests |
| 2 — something selects it | the caller-count fence; the one-legacy-caller mutant turns it red |
| 3 — the caller can discover it | the `maxAge` and its reason are a comment at each call, and ADR-0007 Decision 5 holds the table |
| 4 — it is used | the three loops of every running daemon |

## Mutation Log
- 2026-11-16 · e4abbdd* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · reconcile decides on stale data: TestReconcileAlwaysReadsFresh red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
- 2026-11-16 · e4abbdd* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · probe forks every tick: TestProbeAndHealthReuseSnapshotInsideReconcileInterval red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
- 2026-11-16 · e4abbdd* · mutant survived · exit 0 · `pkg/discovery/dht.go` · punch wait cached: TestPunchWaitForksEveryPoll red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
  ```
  the fence passed with the mechanism broken; it may not materialize, compile, load, or assert on the changed path
  ```
- 2026-11-16 · e4abbdd* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · legacy reader forks: caller-count fence red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
- 2026-11-16 · e4abbdd* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · health forks every check: TestProbeAndHealthReuseSnapshotInsideReconcileInterval red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
- 2026-11-16 · e4abbdd* · mutant killed · exit 1 · `pkg/discovery/dht.go` · stale check forks per peer: TestStaleCheckReadsOneSnapshotForAllPeers red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
- 2026-11-16 · e4abbdd* · mutant survived · exit 0 · `pkg/discovery/dht.go` · introducer ranking forks: ? · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
  ```
  the fence passed with the mechanism broken; it may not materialize, compile, load, or assert on the changed path
  ```
- 2026-11-16 · e4abbdd* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · obsolete peers never removed: TestReconcileAlwaysReadsFresh red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
- 2026-11-16 · 0a11a45* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · reconcile decides on stale data: TestReconcileAlwaysReadsFresh red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
- 2026-11-16 · 0a11a45* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · probe forks every tick: TestProbeAndHealthReuseSnapshotInsideReconcileInterval red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
- 2026-11-16 · 0a11a45* · mutant killed · exit 1 · `pkg/discovery/dht.go` · punch wait cached: TestPunchWaitReadsFreshEveryPoll red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
- 2026-11-16 · 0a11a45* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · legacy reader forks: caller-count fence red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
- 2026-11-16 · 0a11a45* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · health forks every check: TestProbeAndHealthReuseSnapshotInsideReconcileInterval red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
- 2026-11-16 · 0a11a45* · mutant killed · exit 1 · `pkg/discovery/dht.go` · stale check forks per peer: TestStaleCheckReadsOneSnapshotForAllPeers red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
- 2026-11-16 · 0a11a45* · mutant killed · exit 1 · `pkg/discovery/dht.go` · introducer ranking forks: TestIntroducerRankingReadsSnapshotWithinReconcileInterval red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
- 2026-11-16 · 0a11a45* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · obsolete peers never removed: TestReconcileAlwaysReadsFresh red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
- 2026-11-16 · 0a11a45* · mutant killed · exit 1 · `pkg/discovery/dht.go` · baseline may count an old handshake as new: TestPunchSuccessKeepsTransitiveLabel red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8
- 2026-11-16 · 0a11a45* · mutant killed · exit 1 · `pkg/discovery/dht.go` · final read after the timeout cached: TestPunchWaitReadsFreshEveryPoll red · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8

## Invariants

- Reconcile's decisions use data read in the same tick; the probe and health loops compare handshakes with `HandshakeStaleAfter` (150 s) and tolerate 5 s.
- The punch wait still sees a new handshake within one 250 ms poll.
- No constant (`MeshProbeInterval`, `MeshProbeFailLimit`, `ReconcileInterval`, `HealthCheckInterval`) changes.
- ADR-0002's admit rules and `markTemporarilyOffline` are untouched.

## Risks

- `reconcile` cannot be run hermetically (it also calls `ip` through `cmdExecutor`); the tests use the existing mock executor of `pkg/daemon` (`helpers_test.go`), and if that is not enough the first two tests call `buildDesiredPeerConfigs` and `applyDesiredPeerConfigs` directly.
- The 5-second tolerance hides a handshake that just failed for up to 5 s; the thresholds are 150 s.
- Line numbers in the Affected Files table are those of 2026-11-15 (`b8b83f3`); after ADR-0002 the sites are `daemon.go` 675/684/1213/1432, `dht.go` 1022/1395, `exchange.go` 896. There were 7 direct callers at ADR-0002's merge, not 8.
- Beyond the table: `State.Handshakes()` (`pkg/wireguard/state.go`, nil-safe, one place that builds the key-to-handshake map); the `dht.go` test seams became `readWGState` (an `atomic.Pointer`) and `handshakeWaitTimeout` (an `atomic.Int64`) set in `init()`, because the punch goroutine reads them while a test writes (M's rule 2026-11-16); `buildDesiredPeerConfigsWithHandshakes` keeps its map parameter (12 test call sites), `applyDesiredPeerConfigs` takes the `*State`; the wrapper `buildDesiredPeerConfigs` had no caller left and was deleted; `selectRendezvousIntroducers` calls `wireguard.ReadState` directly, as it called the legacy reader directly before.
- `ReadState` holds its mutex across the fork (T1's design, so concurrent readers share one fork): a hung `wg` costs each queued caller up to the 10 s command timeout, where the legacy readers forked in parallel. A failed read in `reconcile` skips the removal step silently, as the legacy `GetPeers` error did.
- The 12 forks a minute in the ADR are computed from the call sites. A snapshot of exactly one `ReconcileInterval` can be just too old for a probe, stale-check or health tick that lands before the next reconcile, so the measured steady state may be between 12 and about 24 a minute; S7's number replaces both.
- Merge with ADR-0003 (2026-11-16): `reconcile` is now `convergeTick` (ADR-0003): it reads the kernel once through `wireguard.Dump` (strict `ParseDump`, with the 10 s deadline of `runWG`) and feeds `planPeers`/`desire`/`converge`; `applyDesiredPeerConfigs`, `buildDesiredPeerConfigs` and `lastAppliedPeerConfigs` no longer exist, and the `getPeers` seam is gone. `Dump` also stores a `State` snapshot in the `ReadState` slot, so the probe and the health check (`ReadState(iface, ReconcileInterval)`) share the tick's fork: one fork per reconcile tick holds, the tick test `TestOneShowForkPerReconcileTick` passes unchanged. `SetPeer`/`RemovePeer` do not invalidate the snapshot, so a probe or health read inside the 5 s tolerance can still show the peer list from before the tick's change (accepted: the thresholds are 150 s; follow-up if wanted: clear the slot in both).

## Stop Condition

Stop and ask if any existing probe, health or relay test needs a changed expectation, or if reconcile cannot take the snapshot without changing a function signature that ADR-0002's tasks changed.

## Out of Scope

- Changing any probe, health or reconcile interval (Decision 1 of ADR-0007 changes none).
- Reading transfer counters at a finer grain than the health check (permanent: boundary: the health check is every 20 s).

## Verification Log
- 2026-11-16 · e4abbdd* · exit 1 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:7185 · test-lock-sha256:d739aa62006f921c5bef7ecb4353afad6b4d5cad40b7f566eb762780899012c0 · test-lock-file:docs/adr/locks/d739aa62006f921c5bef7ecb4353afad6b4d5cad40b7f566eb762780899012c0.lock
  ```
  --- last 10 line(s) of stderr (of 22 after folding 22 raw)
  --- FAIL: TestProbeAndHealthReuseSnapshotInsideReconcileInterval (0.60s)
  === RUN   TestProbeRefreshesSnapshotOlderThanReconcileInterval
  --- PASS: TestProbeRefreshesSnapshotOlderThanReconcileInterval (5.23s)
  === RUN   TestReconcileAlwaysReadsFresh
      wg_fork_test.go:175: show forks = 3, want 2 (priming read + the reconcile read)
  --- FAIL: TestReconcileAlwaysReadsFresh (0.01s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/daemon	5.863s
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/discovery [build failed]
  FAIL
  ```
- 2026-11-16 · e4abbdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:19652
- 2026-11-16 · e4abbdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:18544
- 2026-11-16 · e4abbdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:20553
- 2026-11-16 · e4abbdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:18863
- 2026-11-16 · e4abbdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:18604
- 2026-11-16 · e4abbdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:18126
- 2026-11-16 · e4abbdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:18341
- 2026-11-16 · e4abbdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:18225
- 2026-11-16 · e4abbdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:14311
- 2026-11-16 · 0a11a45* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:0 · test-lock-sha256:2a4f6ee58841a4ec8cbb0cacef7c802b34d81b5ee724170a00f99940df09c325 · test-lock-file:docs/adr/locks/2a4f6ee58841a4ec8cbb0cacef7c802b34d81b5ee724170a00f99940df09c325.lock · test-lock-kind:replace
- 2026-11-16 · 0a11a45* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:21519
- 2026-11-16 · 0a11a45* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:20594
- 2026-11-16 · 0a11a45* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:20684
- 2026-11-16 · 0a11a45* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:19542
- 2026-11-16 · 0a11a45* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:28882
- 2026-11-16 · 0a11a45* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:28097
- 2026-11-16 · 0a11a45* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:25114
- 2026-11-16 · 0a11a45* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:34498
- 2026-11-16 · 0a11a45* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:26289
- 2026-11-16 · 0a11a45* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:33974
- 2026-11-16 · 0a11a45* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:31914
- 2026-11-16 · c0ad7db* · exit 0 · `set -o pipefail …` · acceptance-sha256:58582c0f2146ae8c4c7b6e63693bd112b14b13b15afdd447b27941c31c9755e8 · ms:18351
