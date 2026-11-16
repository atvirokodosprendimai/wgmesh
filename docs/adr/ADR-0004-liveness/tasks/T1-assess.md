# Task ADR-0004-T1: Add the pure `Assess` and make the store's three liveness readers use it

**Depends-on:** none
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `node.Evidence`, `node.Liveness` (`Alive`, `Suspect`, `Dead`), `node.Assess`, `PeerInfo.Alive`, `PeerInfo.LastAlive`, `PeerInfo.LastDataPlane`, `PeerInfo.RestoredAt`, `PeerStore.ObserveDataPlane`, `PeerStore.NoteResume`, `PeerStore.ResumedAt`, `PeerStore.GetProbeable`, `node.EnforceLiveness` (`atomic.Bool`), `PeerStore.SetLivenessDisagreeHook`
**Consumes:** none
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`GetActive`, `CleanupStale` and `IsDead` ask one pure function, `Assess`, which takes the newest of a peer's four evidence times, ages it as a wall-clock instant, and answers `Alive` (under `PeerDeadTimeout`), `Suspect` (up to `PeerRemoveTimeout`) or `Dead`; while `node.EnforceLiveness` is false the legacy `LastSeen` test still acts and every case where `Assess` would have kept a peer is reported through a per-store hook.

The type is `Liveness`, not `Verdict`: `node.Verdict` already exists (`pkg/node/admit.go:42`: `Admit`, `AdmitPartial`, `Refuse`). `Alive` is also the name of the method `PeerInfo.Alive`; a package constant and a method of a type do not clash, and the implementer checks that `go build ./pkg/node` accepts both (if it does not, the constants are `LiveAlive`, `LiveSuspect`, `LiveDead` and every file of this record follows).

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/node/liveness.go` | add | `Evidence`, `Liveness`, `Assess`, `PeerInfo.Alive`, `PeerInfo.LastAlive(now) time.Time` (the same newest-of rule as a time, for callers with their own window), `ObserveDataPlane`, `NoteResume`, `ResumedAt`, `GetProbeable`, `EnforceLiveness`, `SetLivenessDisagreeHook` |
| `pkg/node/types.go` | edit | `PeerInfo.LastDataPlane`, `PeerInfo.RestoredAt` and the unexported `resumedAt` (memory only; no tag, not in `PeerCacheEntry`); `PeerInfo` is copied by value, so all three are plain `time.Time` values |
| `pkg/node/store.go` | edit | `GetActive` (`:304-317`), `CleanupStale` (`:374-390`; it tombstones only first-hand peers, `:383`), `IsDead` (`:412-421`) call `Alive`; the legacy age test stays beside it while the switch is false; a store-wide `resumedAt` guarded by `ps.mu`, stamped into every copy returned by `Get`, `GetAll`, `GetActive` and `GetProbeable`; the per-store disagree hook (a field set by `SetLivenessDisagreeHook` under `ps.mu`, read under it and called outside it); `GetActive` (`:309`), `IsDead` (`:420`) and the merge (`:120`) read `ps.now()` (field `:58`) instead of `time.Now()` |
| `pkg/daemon/peerstore_compat.go` | edit | aliases for `Liveness` and `Evidence` beside the existing ones (`Candidate`, `Hand`, `Source`, `Decision`, `Policy`) so `daemon` and `discovery` can name them |
| `pkg/daemon/daemon.go` | edit | `NewDaemon` sets the hook on the store (no body yet: T2 wires it to the counter; here the hook is only settable) |
| `pkg/node/liveness_test.go` | add | tests below; they live in `pkg/node` because only that package can set the unexported `ps.now` |

Selection: `GetActive`, `CleanupStale` and `IsDead` call `PeerInfo.Alive`; a mutant that makes `Alive` return `Assess` of an empty `Evidence` turns `TestGetActiveUsesAssessWhenEnforced` red. Nothing in `pkg/discovery` changes in this task and `pkg/daemon` only gets aliases, so the served behaviour is unchanged while `EnforceLiveness` is false.

## Ordered Steps

1. [S1] Add `pkg/node/liveness_test.go` with the eleven tests and compiling stubs (`Assess` returning `Alive`, empty store methods) so the first run fails on assertions, not on the build. `TestAssessThresholds` is a table with literal ages (4m59s Alive, 5m Suspect, 10m Suspect, 10m1s Dead). Run red.
2. [S2] Implement `Assess`: `ref` is the newest non-zero of `LastSeen`, `LastDataPlane`, `RestoredAt`, `resumedAt`, each passed through `Round(0)`; `age = now.Round(0).Sub(ref)`; an all-zero `Evidence` is `Dead`. Implement `PeerInfo.LastAlive(now)` with the same `ref` rule (the zero time when all four are zero), and make `Alive` read it.
3. [S3] Switch `GetActive`, `IsDead` and the merge from `time.Now()` to `ps.now()`. Implement `ObserveDataPlane` (a time never moves backwards; no event is sent; no claim field is touched; takes `ps.mu.Lock` like `SetLatency`, `store.go:400`), `NoteResume`, `ResumedAt`, `GetProbeable`, the per-store hook (precedent: `Policy.OnWarn`, `admit.go:83`, wired by `Daemon.admitPolicy`, `daemon.go:294-307`, to `RecordAdmitWarning`, `metrics.go:160`; here the wiring is in `NewDaemon`, T2 adds the counter); route the three readers through `Alive`; keep the legacy test and call the hook with the site name (`get-active`, `cleanup`, `is-dead`) when the legacy test drops a peer that `Assess` calls `Alive`. There is no package-level hook setter: parallel tests would race on it.
4. [S4] Run the mutants with `adr-verify --mutant`: `Assess` ignores `LastDataPlane`; `Assess` keeps the monotonic reading; `EnforceLiveness` is read inverted; `GetProbeable` drops `Suspect`; a copy is returned without `resumedAt`. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/node -run '^(TestAssessThresholds|TestAssessNewestEvidenceWins|TestAssessIgnoresMonotonicReading|TestGetActiveUsesAssessWhenEnforced|TestLegacyAgeStillActsAndReportsDisagreement|TestObserveDataPlaneNeverMovesBack|TestGetProbeableIncludesSuspect|TestResumedAtIsStampedIntoCopies|TestLastAliveIsNewestEvidence|TestLivenessUsesStoreClock|TestLivenessStateIsRaceFree)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 11 ] && go test -race ./pkg/node ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestAssessThresholds` | `pkg/node/liveness_test.go` | `Assess` gives Alive below `PeerDeadTimeout`, Suspect from it up to `PeerRemoveTimeout`, Dead after, for literal ages | — | S1, S2 |
| `TestAssessNewestEvidenceWins` | `pkg/node/liveness_test.go` | a fresh `LastDataPlane` keeps a peer Alive whose `LastSeen` is an hour old; a fresh `RestoredAt` and a fresh `resumedAt` do the same | — | S1, S2 |
| `TestAssessIgnoresMonotonicReading` | `pkg/node/liveness_test.go` | a `LastSeen` made by `time.Now()` and one made by `time.Unix` with the same wall instant give the same liveness when `now` is advanced by wall time only | — | S1, S2 |
| `TestGetActiveUsesAssessWhenEnforced` | `pkg/node/liveness_test.go` | with the switch on (`EnforceLiveness.Store(true)`, restored by `t.Cleanup`), a peer with an old `LastSeen` and a fresh `LastDataPlane` is in `GetActive`, absent from `CleanupStale`'s result, and `IsDead` is false | — | S1, S3 |
| `TestLegacyAgeStillActsAndReportsDisagreement` | `pkg/node/liveness_test.go` | with the switch off, the same peer is dropped as today and the store's own hook receives `get-active` once; a second store with no hook is unaffected (no global) | — | S1, S3 |
| `TestObserveDataPlaneNeverMovesBack` | `pkg/node/liveness_test.go` | an older time does not replace a newer one; an unknown key is a no-op and sends no event | — | S1, S3 |
| `TestGetProbeableIncludesSuspect` | `pkg/node/liveness_test.go` | a peer 7 minutes silent is not in `GetActive` and is in `GetProbeable` | — | S1, S3 |
| `TestResumedAtIsStampedIntoCopies` | `pkg/node/liveness_test.go` | after `NoteResume(t)`, the copies returned by `Get`, `GetAll`, `GetActive` and `GetProbeable` all give `Alive` through `PeerInfo.Alive` for a peer whose other evidence is hours old, and `ResumedAt()` returns `t` | — | S1, S3 |
| `TestLastAliveIsNewestEvidence` | `pkg/node/liveness_test.go` | `LastAlive` returns the newest of the four times, the zero time for an empty `Evidence`, and `Alive` agrees with it at the thresholds | — | S1, S2 |
| `TestLivenessUsesStoreClock` | `pkg/node/liveness_test.go` | with `ps.now` set to an injected clock, `GetActive`, `IsDead` and the merge (a new key's first time) follow it, not `time.Now()` | — | S1, S3 |
| `TestLivenessStateIsRaceFree` | `pkg/node/liveness_test.go` | goroutines call `ObserveDataPlane`, `NoteResume`, `SetLivenessDisagreeHook`, `GetActive`, `GetProbeable` and `EnforceLiveness.Store` together; the test has no assertion beyond completing, and the final fence runs it under `-race` | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the eleven tests |
| 2 — something selects it | the three store readers call `Alive`; the mutant that returns the legacy answer turns `TestGetActiveUsesAssessWhenEnforced` red |
| 3 — the caller can discover it | exported with doc comments; T7 amends the peer store spec |
| 4 — it is used | every `GetActive` caller (13 sites) from N+1; the disagreement hook is wired to a metric in T2 |

## Mutation Log

## Invariants

- With `EnforceLiveness` false, every list `GetActive`, `CleanupStale` and `IsDead` returned before this task is returned unchanged.
- `LastSeen` is written only where it is written today; `ObserveDataPlane` and `NoteResume` never write it.
- `pkg/node` imports nothing new beyond `sync/atomic`, and `Assess` reads no clock except its `now` argument.
- The tombstone code of `CleanupStale` (first-hand peers only, ADR-0002 T3) is untouched: what `Assess` calls Dead is withdrawn the way ADR-0002 says.
- No field, variable or counter accessed by more than one goroutine is bare: `atomic.*` or a lock on every access (M 2026-11-16); the task has a `-race` test for each such field it adds. Here: `EnforceLiveness` is an `atomic.Bool`; the hook, `resumedAt` and the new `PeerInfo` fields are read and written under `ps.mu`; `TestLivenessStateIsRaceFree` covers the hook, `ObserveDataPlane` and `NoteResume`.

## Risks

- Keeping both tests for one release doubles the age arithmetic in three places; T7 and the N+1 follow-up remove the legacy branch's warning code only after the counter has been read.
- Moving `GetActive` and `IsDead` to `ps.now()` changes nothing in production (the default clock is `time.Now`) but existing store tests that set `ps.now` now also move those two readers; they are re-run in the fence.

## Stop Condition

Stop and ask if an existing store test needs a changed expectation with the switch off, or if `CleanupStale` is in a shape where the liveness cannot replace its age test without changing which keys are tombstoned.

## Out of Scope

- Writing the evidence from reconcile and the probe, and moving the four outside readers (T2).
- Any change to `LastSeen` merge rules (ADR-0002).

## Verification Log
