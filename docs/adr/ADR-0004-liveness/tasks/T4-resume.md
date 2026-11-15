# Task ADR-0004-T4: Detect a wake from sleep and give the peers a grace period (#829)

**Depends-on:** T2
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `DetectResume`, `ResumeGrace`, `ResumeCheckInterval`, `ResumeGapThreshold`, the resume loop, `node.Resumer`, `wgmesh_resume_detected_total`
**Consumes:** `Assess` and `PeerStore.NoteResume`, `EnforceLiveness` (T1); the evidence writers (T2)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The daemon notices within about two seconds that the machine slept, marks every peer as seen at the wake, and for `ResumeGrace` does not evict a peer or choose a relay because of a handshake age that includes the time asleep.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/resume.go` | add | `DetectResume(prev, now, threshold) bool` (pure: wall gap with the monotonic reading stripped), `ResumeCheckInterval = 2 * time.Second`, `ResumeGapThreshold = 10 * time.Second`, `ResumeGrace = 60 * time.Second`, `resumeLoop`, `inResumeGrace` |
| `pkg/node/liveness.go` | edit | `Resumer` interface (`OnResume()`) |
| `pkg/daemon/daemon.go` | edit | `RunWithDHTDiscovery` (`:1525-1650`) starts `resumeLoop` next to the other loops (`:1614-1620`); on a resume it calls `PeerStore.NoteResume`, `reconcile()` and `OnResume()` on each layer that implements `Resumer`; `checkPeerHealth` (`:1284-1341`) and `probePeersOverMesh` (`:1112-1115`) skip eviction and `shouldRelayPeerWithSubnets` (`:683-703`) skips the stale-handshake branch while `inResumeGrace` and `node.EnforceLiveness` are true |
| `pkg/daemon/metrics.go` | edit | `wgmesh_resume_detected_total` |
| `pkg/daemon/resume_test.go` | add | tests below |

Selection: `RunWithDHTDiscovery` starts the loop; a mutant that does not start it turns `TestResumeLoopCallsHooks` red; the eviction call sites check `inResumeGrace`, and a mutant that drops the check turns the two grace tests red.

## Ordered Steps

1. [S1] Add the tests below with compiling stubs (a `DetectResume` that returns false) so the first run fails on assertions; run red. The loop takes an injected clock and tick source so the test does not sleep.
2. [S2] Implement `DetectResume` and the loop; count `wgmesh_resume_detected_total` and log one line `[Resume] wall clock jumped <gap>`.
3. [S3] Call `NoteResume`, `reconcile()` and the `Resumer` layers from the loop; these act from N (additive, local).
4. [S4] Add the grace to the three sites, active only with `node.EnforceLiveness`; with the switch off, count `wgmesh_liveness_disagree_total{site="resume-grace"}` when the grace would have blocked an eviction.
5. [S5] Run the mutants with `adr-verify --mutant`: `DetectResume` uses the monotonic reading; the grace is never applied; the grace never ends; the loop does not call `NoteResume`. [proof: mutation]
6. [S6] On a Mac with an established mesh link, run `pmset sleepnow`, wake, and time to the first successful ping to a peer; record the time and the host. [proof: human: the sleep path cannot be reproduced hermetically; adr-verify cannot see this number and the ADR states none]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/daemon -run '^(TestDetectResume|TestResumeLoopCallsHooks|TestResumeGraceBlocksHealthEviction|TestResumeGraceBlocksProbeEviction|TestResumeGraceExpires|TestNoGraceWhileLivenessNotEnforced|TestResumeKeepsAllPeersAlive)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 7 ] && go test -race ./pkg/node ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestDetectResume` | `pkg/daemon/resume_test.go` | a table: gap 2 s false, 10 s false, 11 s true, a negative gap (clock stepped back) false, a `prev` with a monotonic reading and a `now` that is 8 hours later by wall clock only gives true | — | S1, S2 |
| `TestResumeLoopCallsHooks` | `pkg/daemon/resume_test.go` | with an injected clock that jumps one hour, the loop calls `NoteResume`, `reconcile` and the `Resumer` once each | — | S1, S3 |
| `TestResumeGraceBlocksHealthEviction` | `pkg/daemon/resume_test.go` | within `ResumeGrace` of a resume, a peer with a stale handshake and no transfer growth is not reconnected or evicted on two health cycles | — | S1, S4 |
| `TestResumeGraceBlocksProbeEviction` | `pkg/daemon/resume_test.go` | within the grace, `MeshProbeFailLimit` failed probes do not evict | — | S1, S4 |
| `TestResumeGraceExpires` | `pkg/daemon/resume_test.go` | after the grace the same inputs evict as before | — | S1, S4 |
| `TestNoGraceWhileLivenessNotEnforced` | `pkg/daemon/resume_test.go` | with the switch off the eviction happens and the counter `resume-grace` is incremented | — | S1, S4 |
| `TestResumeKeepsAllPeersAlive` | `pkg/daemon/resume_test.go` | after `NoteResume`, every stored peer is `Alive` for `PeerDeadTimeout` even if its other evidence is hours old | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the seven tests |
| 2 — something selects it | `RunWithDHTDiscovery` starts `resumeLoop`; the mutant that does not start it turns `TestResumeLoopCallsHooks` red |
| 3 — the caller can discover it | the log line and `wgmesh_resume_detected_total`; T7 documents them |
| 4 — it is used | a wake on a laptop; S6 records the time to the first ping |

## Mutation Log

## Invariants

- The watcher reads only the wall clock and starts no process; nothing is written to disk.
- Outside the grace, health, probe and relay decisions are the ones of T2.
- A detection never removes a peer or a WireGuard configuration by itself.

## Risks

- A false positive (a long pause or an NTP step forward) causes one reconcile, one announce and a 60-second grace; accepted (M3).
- A real peer death during the sleep is noticed 60 seconds later than it could be; accepted (M6).
- `daemon.go` conflicts with ADR-0002 T3 (`evictPeerFromPool`) and T4; see Ordering.

## Stop Condition

Stop and ask if the loop cannot be tested without a real sleep, or if the relay choice needs the grace flag in a function that has no access to the daemon (`shouldRelayPeerWithSubnets` already is a method of `Daemon`).

## Out of Scope

- OS power notifications (deferred: docs/adr/BACKLOG.md).
- The actions of the DHT layer on a resume (T5).

## Verification Log
