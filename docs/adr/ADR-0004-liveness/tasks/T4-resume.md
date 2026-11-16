# Task ADR-0004-T4: Detect a wake from sleep and give the peers a grace period (#829)

**Depends-on:** T2
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `DetectResume`, `ResumeGrace`, `ResumeCheckInterval`, `ResumeGapThreshold`, the resume loop (started in both run paths), `node.Resumer`, `wgmesh_resume_detected_total`, a clock seam for the grace sites
**Consumes:** `Assess`, `PeerStore.NoteResume`, `PeerStore.ResumedAt`, `EnforceLiveness` (T1); the evidence writers (T2)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The daemon notices within about two seconds that the machine slept, marks every peer as seen at the wake, and for `ResumeGrace` does not evict a peer or choose a relay because of a handshake age that includes the time asleep.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/resume.go` | add | `DetectResume(prev, now, threshold) bool` (pure: wall gap with the monotonic reading stripped), `ResumeCheckInterval = 2 * time.Second`, `ResumeGapThreshold = 10 * time.Second`, `ResumeGrace = 60 * time.Second`, `resumeLoop`, `startResumeLoop`, `inResumeGrace(now)`, `d.now()` (the clock seam: returns `time.Now()` when the daemon's clock field is nil, so `&Daemon{...}` literals keep working) |
| `pkg/node/liveness.go` | edit | `Resumer` interface (`OnResume()`) |
| `pkg/daemon/daemon.go` | edit | **two run paths start loops**: `Run()` (`:349-431`, loops `:387-407`) and `RunWithDHTDiscovery` (`:1662-1782`, loops `:1736-1757`); both call `startResumeLoop` (factored once, not copied). On a resume the loop calls `PeerStore.NoteResume`, signals the reconcile loop through a buffered channel (capacity 1, non-blocking send; `reconcileLoop`, `:655-667`, selects on it), and calls `OnResume()` on the one discovery field `d.dhtDiscovery` (`:78`) when it implements `Resumer` (one type assertion; LAN and gossip live inside `DHTDiscovery` and are driven by its `OnResume`, T5). `reconcile()` is never called from the resume goroutine: it would run concurrently with `reconcileLoop` and the apply step is not serialised. **Grace sites**: `attemptPeerReconnect` (`:1462`) and `evictPeerFromPool` (`:1466`) in `checkPeerHealth` (`:1421`), probe eviction (`:1251`), and the stale-handshake branch of `shouldRelayPeerWithSubnets` (`:819-840`) skip while `inResumeGrace` and `node.EnforceLiveness` are true. `markTemporarilyOffline` / `isTemporarilyOffline` (`:1560`, `:1578`) and `checkPeerHealth`'s `now := time.Now()` (`:1432`) read `d.now()` so the loop's injected clock reaches them |
| `pkg/daemon/metrics.go` | edit | `wgmesh_resume_detected_total`, following `admitWarnings` (`:56`), registered in `registerMetrics` (`:81-95`); the `resume-grace` site of `wgmesh_liveness_disagree_total` is pre-seeded like `winner-differs` (`:92`); do not duplicate `wgmesh_admit_warnings_total` |
| `pkg/daemon/resume_test.go` | add | tests below; the daemon-literal style is `TestEvictionLeavesTombstone` (`health_test.go:37`) |

Selection: `Run` and `RunWithDHTDiscovery` start the loop; a mutant that does not start it turns `TestResumeLoopCallsHooks` or `TestBothRunPathsStartResumeLoop` red; the eviction call sites check `inResumeGrace`, and a mutant that drops the check turns the two grace tests red.

## Ordered Steps

1. [S1] Add the tests below with compiling stubs (a `DetectResume` that returns false) so the first run fails on assertions; run red. The loop takes an injected clock (`d.now`) and tick source so the test does not sleep.
2. [S2] Implement `DetectResume`, the loop and `startResumeLoop`; count `wgmesh_resume_detected_total` and log one line `[Resume] wall clock jumped <gap>`. Call `startResumeLoop` from both `Run` and `RunWithDHTDiscovery`.
3. [S3] From the loop call `NoteResume`, signal the reconcile channel and call the `Resumer` on `d.dhtDiscovery`; these act from N (additive, local).
4. [S4] Add the grace to the four sites, active only with `node.EnforceLiveness`; with the switch off, count `wgmesh_liveness_disagree_total{site="resume-grace"}` when the grace would have blocked an eviction.
5. [S5] Run the mutants with `adr-verify --mutant`: `DetectResume` uses the monotonic reading; the grace is never applied; the grace never ends; the loop does not call `NoteResume`; the loop calls `reconcile()` directly instead of signalling; one run path does not start the loop. [proof: mutation]
6. [S6] On a Mac with an established mesh link, run `pmset sleepnow`, wake, and time to the first successful ping to a peer; record the time and the host. [proof: human: the sleep path cannot be reproduced hermetically; adr-verify cannot see this number and the ADR states none]. NOT RECORDED (2026-11-16): it needs a real macOS machine with a live mesh and someone to time it; the automated fence below is green without it, so T4 is done on the fence and S6 stays open for M.

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/daemon -run '^(TestDetectResume|TestResumeLoopCallsHooks|TestBothRunPathsStartResumeLoop|TestResumeGraceBlocksHealthEviction|TestResumeGraceBlocksProbeEviction|TestResumeGraceExpires|TestNoGraceWhileLivenessNotEnforced|TestResumeKeepsAllPeersAlive|TestResumeLoopStateIsRaceFree|TestInResumeGraceBounds|TestOnlyReconcileLoopCallsReconcile|TestDaemonNowIsWallClock)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 12 ] && go test -race ./pkg/node ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestDetectResume` | `pkg/daemon/resume_test.go` | a table: gap 2 s false, 10 s false, 11 s true, a negative gap (clock stepped back) false, a `prev` with a monotonic reading and a `now` that is 8 hours later by wall clock only gives true | — | S1, S2 |
| `TestResumeLoopCallsHooks` | `pkg/daemon/resume_test.go` | with an injected clock that jumps one hour, the loop calls `NoteResume`, sends one signal on the reconcile channel (it does not call `reconcile`) and calls the `Resumer` on `d.dhtDiscovery` once each | — | S1, S3 |
| `TestBothRunPathsStartResumeLoop` | `pkg/daemon/resume_test.go` | a go/parser walk of `daemon.go` finds a call to `startResumeLoop` in the bodies of both `Run` and `RunWithDHTDiscovery` | — | S1, S2 |
| `TestResumeGraceBlocksHealthEviction` | `pkg/daemon/resume_test.go` | within `ResumeGrace` of a resume, a peer with a stale handshake and no transfer growth is not reconnected or evicted on two health cycles | — | S1, S4 |
| `TestResumeGraceBlocksProbeEviction` | `pkg/daemon/resume_test.go` | within the grace, `MeshProbeFailLimit` failed probes do not evict | — | S1, S4 |
| `TestResumeGraceExpires` | `pkg/daemon/resume_test.go` | after the grace the same inputs evict as before | — | S1, S4 |
| `TestNoGraceWhileLivenessNotEnforced` | `pkg/daemon/resume_test.go` | with the switch off the eviction happens and the counter `resume-grace` is incremented | — | S1, S4 |
| `TestResumeKeepsAllPeersAlive` | `pkg/daemon/resume_test.go` | after `NoteResume`, every stored peer, as a copy from `GetActive`, is `Alive` for `PeerDeadTimeout` even if its other evidence is hours old | — | S1, S3 |
| `TestResumeLoopStateIsRaceFree` | `pkg/daemon/resume_test.go` | the loop runs with a jumping clock while `checkPeerHealth`, `inResumeGrace`, `reconcileLoop`'s signal receiver and `NoteResume` run in other goroutines; the final fence runs it under `-race` | — | S1, S3 |
| `TestInResumeGraceBounds` | `pkg/daemon/resume_test.go` | the grace needs a non-negative age: a clock stepped back after the wake is outside it, exactly `ResumeGrace` after is outside it | — | S1, S4 |
| `TestOnlyReconcileLoopCallsReconcile` | `pkg/daemon/resume_test.go` | a go/parser walk finds `reconcile` called only by `reconcileLoop` and `handleSIGHUP`, never by the resume goroutine | — | S1, S3 |
| `TestDaemonNowIsWallClock` | `pkg/daemon/resume_test.go` | `Daemon.now` returns a wall-clock instant without a monotonic reading, so temporary-offline expiry counts the time asleep | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the nine tests |
| 2 — something selects it | `Run` and `RunWithDHTDiscovery` start `resumeLoop`; the mutant that does not start it turns `TestResumeLoopCallsHooks` or `TestBothRunPathsStartResumeLoop` red |
| 3 — the caller can discover it | the log line and `wgmesh_resume_detected_total`; T7 documents them |
| 4 — it is used | a wake on a laptop; S6 records the time to the first ping |

## Mutation Log
- 2026-11-16 · 40b637b* · mutant survived · exit 0 · `pkg/daemon/resume.go` · DetectResume keeps the monotonic reading · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387
  ```
  the fence passed with the mechanism broken; it may not materialize, compile, load, or assert on the changed path
  ```
- 2026-11-16 · 40b637b* · mutant killed · exit 1 · `pkg/daemon/resume.go` · grace never applied · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387
- 2026-11-16 · 40b637b* · mutant killed · exit 1 · `pkg/daemon/resume.go` · grace never ends · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387
- 2026-11-16 · 40b637b* · mutant killed · exit 1 · `pkg/daemon/resume.go` · loop does not call NoteResume · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387
- 2026-11-16 · 40b637b* · mutant survived · exit 0 · `pkg/daemon/resume.go` · loop calls reconcile directly · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387
  ```
  the fence passed with the mechanism broken; it may not materialize, compile, load, or assert on the changed path
  ```
- 2026-11-16 · 40b637b* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · Run does not start the resume loop · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387
- 2026-11-16 · 40b637b* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · RunWithDHTDiscovery does not start the resume loop · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387
- 2026-11-16 · 40b637b* · mutant killed · exit 1 · `pkg/daemon/resume.go` · Resumer never called · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387
- 2026-11-16 · 48aa152* · mutant killed · exit 1 · `pkg/daemon/resume.go` · threshold boundary · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944
- 2026-11-16 · 48aa152* · mutant survived · exit 0 · `pkg/daemon/resume.go` · DetectResume keeps the monotonic reading (equivalent in-process, expected to survive) · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944
  ```
  the fence passed with the mechanism broken; it may not materialize, compile, load, or assert on the changed path
  ```
- 2026-11-16 · 48aa152* · mutant killed · exit 1 · `pkg/daemon/resume.go` · grace never applied · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944
- 2026-11-16 · 48aa152* · mutant killed · exit 1 · `pkg/daemon/resume.go` · grace never ends · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944
- 2026-11-16 · 48aa152* · mutant killed · exit 1 · `pkg/daemon/resume.go` · loop does not call NoteResume · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944
- 2026-11-16 · 48aa152* · mutant killed · exit 1 · `pkg/daemon/resume.go` · loop calls reconcile directly · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944
- 2026-11-16 · 48aa152* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · Run does not start the resume loop · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944
- 2026-11-16 · 48aa152* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · RunWithDHTDiscovery does not start the resume loop · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944
- 2026-11-16 · 48aa152* · mutant killed · exit 1 · `pkg/daemon/resume.go` · Resumer never called · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944
- 2026-11-16 · 48aa152* · mutant killed · exit 1 · `pkg/daemon/resume.go` · grace lower bound removed · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944
- 2026-11-16 · 48aa152* · mutant killed · exit 1 · `pkg/daemon/resume.go` · daemon clock keeps monotonic reading · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944

## Invariants

- The watcher reads only the wall clock and starts no process; nothing is written to disk.
- Outside the grace, health, probe and relay decisions are the ones of T2.
- A detection never removes a peer or a WireGuard configuration by itself.
- `reconcile()` has one caller at a time: the resume loop only signals `reconcileLoop`.
- No field, variable or counter accessed by more than one goroutine is bare: `atomic.*` or a lock on every access (M 2026-11-16); the task has a `-race` test for each such field it adds. Here: the clock field read through `d.now()` is set before the loops start and never written after (or is guarded); the reconcile signal is a channel; the resume counter follows the `admitWarnings` pattern; `ResumedAt` is read through the store's lock. `TestResumeLoopStateIsRaceFree` covers them.

## Risks

- A false positive (a long pause or an NTP step forward) causes one reconcile, one announce and a 60-second grace; accepted (M3).
- A real peer death during the sleep is noticed 60 seconds later than it could be; accepted (M6).
- `daemon.go` is shared with the health, probe and reconcile code; see Ordering.

## Stop Condition

Stop and ask if the loop cannot be tested without a real sleep, or if the relay choice needs the grace flag in a function that has no access to the daemon (`shouldRelayPeerWithSubnets` already is a method of `Daemon`).

## Out of Scope

- OS power notifications (deferred: docs/adr/BACKLOG.md).
- The actions of the DHT layer on a resume (T5).

## Verification Log
- 2026-11-16 · 40b637b* · exit 1 · `set -o pipefail …` · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387 · ms:1123 · test-lock-sha256:1bd8e58efeda19aad61a150c6c7521e9aa97fbd683ac4ff257a1453734d3a936 · test-lock-file:docs/adr/locks/1bd8e58efeda19aad61a150c6c7521e9aa97fbd683ac4ff257a1453734d3a936.lock
  ```
  --- last 10 line(s) of stderr (of 93 after folding 93 raw)
      --- FAIL: TestDetectResume/11_seconds (0.00s)
      --- PASS: TestDetectResume/clock_stepped_back (0.00s)
      --- FAIL: TestDetectResume/8_hours_by_wall_clock_only (0.00s)
  === NAME  TestBothRunPathsStartResumeLoop
      resume_test.go:170: Run does not call startResumeLoop
      resume_test.go:170: RunWithDHTDiscovery does not call startResumeLoop
  --- FAIL: TestBothRunPathsStartResumeLoop (0.01s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/daemon	0.024s
  FAIL
  ```
- 2026-11-16 · 40b637b* · exit 0 · `set -o pipefail …` · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387 · ms:10343
- 2026-11-16 · 40b637b* · exit 0 · `set -o pipefail …` · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387 · ms:10252
- 2026-11-16 · 40b637b* · exit 0 · `set -o pipefail …` · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387 · ms:11198
- 2026-11-16 · 40b637b* · exit 0 · `set -o pipefail …` · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387 · ms:10644
- 2026-11-16 · 40b637b* · exit 0 · `set -o pipefail …` · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387 · ms:10901
- 2026-11-16 · 40b637b* · exit 0 · `set -o pipefail …` · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387 · ms:11438
- 2026-11-16 · 40b637b* · exit 0 · `set -o pipefail …` · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387 · ms:10611
- 2026-11-16 · 40b637b* · exit 0 · `set -o pipefail …` · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387 · ms:10493
- 2026-11-16 · 40b637b* · exit 0 · `set -o pipefail …` · acceptance-sha256:78c0e55fb90595f7d31f76a49944860235b2b663eaa34feef26c8c64ce07c387 · ms:7233
- 2026-11-16 · 48aa152* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944 · ms:2 · test-lock-sha256:0631a8fdfce9ba200f1feced50c46022b7139d56b7069b8332e040981cf661ed · test-lock-file:docs/adr/locks/0631a8fdfce9ba200f1feced50c46022b7139d56b7069b8332e040981cf661ed.lock · test-lock-kind:replace
- 2026-11-16 · 48aa152* · exit 0 · `set -o pipefail …` · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944 · ms:28591
- 2026-11-16 · 48aa152* · exit 0 · `set -o pipefail …` · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944 · ms:20063
- 2026-11-16 · 48aa152* · exit 0 · `set -o pipefail …` · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944 · ms:22936
- 2026-11-16 · 48aa152* · exit 0 · `set -o pipefail …` · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944 · ms:17681
- 2026-11-16 · 48aa152* · exit 0 · `set -o pipefail …` · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944 · ms:21919
- 2026-11-16 · 48aa152* · exit 0 · `set -o pipefail …` · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944 · ms:24145
- 2026-11-16 · 48aa152* · exit 0 · `set -o pipefail …` · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944 · ms:23383
- 2026-11-16 · 48aa152* · exit 0 · `set -o pipefail …` · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944 · ms:16787
- 2026-11-16 · 48aa152* · exit 0 · `set -o pipefail …` · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944 · ms:17548
- 2026-11-16 · 48aa152* · exit 0 · `set -o pipefail …` · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944 · ms:20220
- 2026-11-16 · 48aa152* · exit 0 · `set -o pipefail …` · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944 · ms:11719
- 2026-11-16 · 48aa152* · exit 0 · `set -o pipefail …` · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944 · ms:11471
- 2026-11-16 · 7c1d459* · exit 0 · `set -o pipefail …` · acceptance-sha256:a38c3aeb61433a31334c4b2e439b1b8b982a83630ce1282640b3102168b34944 · ms:8133
