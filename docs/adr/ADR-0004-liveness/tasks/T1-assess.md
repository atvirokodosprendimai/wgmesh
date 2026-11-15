# Task ADR-0004-T1: Add the pure `Assess` and make the store's three liveness readers use it

**Depends-on:** none
**External prerequisite:** ADR-0002 T3 merged on main (it rewrites `CleanupStale` and adds `Withdraw`)
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `node.Evidence`, `node.Verdict`, `node.Assess`, `PeerInfo.Alive`, `PeerInfo.LastDataPlane`, `PeerInfo.RestoredAt`, `PeerStore.ObserveDataPlane`, `PeerStore.NoteResume`, `PeerStore.GetProbeable`, `node.EnforceLiveness`, `node.SetLivenessDisagreeHook`
**Consumes:** none
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`GetActive`, `CleanupStale` and `IsDead` ask one pure function, `Assess`, which takes the newest of a peer's four evidence times, ages it as a wall-clock instant, and answers `Alive` (under `PeerDeadTimeout`), `Suspect` (up to `PeerRemoveTimeout`) or `Dead`; while `node.EnforceLiveness` is false the legacy `LastSeen` test still acts and every case where `Assess` would have kept a peer is reported through a hook.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/node/liveness.go` | add | `Evidence`, `Verdict`, `Assess`, `PeerInfo.Alive`, `ObserveDataPlane`, `NoteResume`, `GetProbeable`, `EnforceLiveness`, the disagree hook |
| `pkg/node/types.go` | edit | `PeerInfo.LastDataPlane`, `PeerInfo.RestoredAt` (memory only; no tag, not in `PeerCacheEntry`) |
| `pkg/node/store.go` | edit | `GetActive` (`:294-307`), `CleanupStale` (`:317-330`, after ADR-0002 T3's tombstone hook), `IsDead` (`:352-361`) call `Alive`; the legacy age test stays beside it while the switch is false; a store-wide `resumedAt` field |
| `pkg/daemon/peerstore_compat.go` | edit | aliases for `Verdict` and `Evidence` so `daemon` and `discovery` can name them |
| `pkg/node/liveness_test.go` | add | tests below |

Selection: `GetActive`, `CleanupStale` and `IsDead` call `PeerInfo.Alive`; a mutant that makes `Alive` return `Assess` of an empty `Evidence` turns `TestGetActiveUsesAssessWhenEnforced` red. Nothing in `pkg/discovery` or `pkg/daemon` changes in this task, so the served behaviour is unchanged while `EnforceLiveness` is false.

## Ordered Steps

1. [S1] Add `pkg/node/liveness_test.go` with the seven tests and compiling stubs (`Assess` returning `Alive`, empty store methods) so the first run fails on assertions, not on the build. `TestAssessThresholds` is a table with literal ages (4m59s Alive, 5m Suspect, 10m Suspect, 10m1s Dead). Run red.
2. [S2] Implement `Assess`: `ref` is the newest non-zero of `LastSeen`, `LastDataPlane`, `RestoredAt`, `resumedAt`, each passed through `Round(0)`; `age = now.Round(0).Sub(ref)`; an all-zero `Evidence` is `Dead`.
3. [S3] Implement `ObserveDataPlane` (a time never moves backwards; no event is sent; no claim field is touched), `NoteResume`, `GetProbeable`; route the three readers through `Alive`; keep the legacy test and call the disagree hook with the site name (`get-active`, `cleanup`, `is-dead`) when the legacy test drops a peer that `Assess` calls `Alive`.
4. [S4] Run the mutants with `adr-verify --mutant`: `Assess` ignores `LastDataPlane`; `Assess` keeps the monotonic reading; `EnforceLiveness` is read inverted; `GetProbeable` drops `Suspect`. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/node -run '^(TestAssessThresholds|TestAssessNewestEvidenceWins|TestAssessIgnoresMonotonicReading|TestGetActiveUsesAssessWhenEnforced|TestLegacyAgeStillActsAndReportsDisagreement|TestObserveDataPlaneNeverMovesBack|TestGetProbeableIncludesSuspect)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 7 ] && go test -race ./pkg/node ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestAssessThresholds` | `pkg/node/liveness_test.go` | `Assess` gives Alive below `PeerDeadTimeout`, Suspect from it up to `PeerRemoveTimeout`, Dead after, for literal ages | — | S1, S2 |
| `TestAssessNewestEvidenceWins` | `pkg/node/liveness_test.go` | a fresh `LastDataPlane` keeps a peer Alive whose `LastSeen` is an hour old; a fresh `RestoredAt` and a fresh `resumedAt` do the same | — | S1, S2 |
| `TestAssessIgnoresMonotonicReading` | `pkg/node/liveness_test.go` | a `LastSeen` made by `time.Now()` and one made by `time.Unix` with the same wall instant give the same verdict when `now` is advanced by wall time only | — | S1, S2 |
| `TestGetActiveUsesAssessWhenEnforced` | `pkg/node/liveness_test.go` | with the switch on, a peer with an old `LastSeen` and a fresh `LastDataPlane` is in `GetActive`, absent from `CleanupStale`'s result, and `IsDead` is false | — | S1, S3 |
| `TestLegacyAgeStillActsAndReportsDisagreement` | `pkg/node/liveness_test.go` | with the switch off, the same peer is dropped as today and the hook receives `get-active` once | — | S1, S3 |
| `TestObserveDataPlaneNeverMovesBack` | `pkg/node/liveness_test.go` | an older time does not replace a newer one; an unknown key is a no-op and sends no event | — | S1, S3 |
| `TestGetProbeableIncludesSuspect` | `pkg/node/liveness_test.go` | a peer 7 minutes silent is not in `GetActive` and is in `GetProbeable` | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the seven tests |
| 2 — something selects it | the three store readers call `Alive`; the mutant that returns the legacy answer turns `TestGetActiveUsesAssessWhenEnforced` red |
| 3 — the caller can discover it | exported with doc comments; T7 amends the peer store spec |
| 4 — it is used | every `GetActive` caller (13 sites) from N+1; the disagreement hook is wired to a metric in T2 |

## Mutation Log

## Invariants

- With `EnforceLiveness` false, every list `GetActive`, `CleanupStale` and `IsDead` returned before this task is returned unchanged.
- `LastSeen` is written only where it is written today; `ObserveDataPlane` and `NoteResume` never write it.
- `pkg/node` imports nothing new and `Assess` reads no clock except its `now` argument.
- ADR-0002's tombstone hook in `CleanupStale` is untouched: what `Assess` calls Dead is withdrawn the way ADR-0002 T3 says.

## Risks

- Merge conflicts with ADR-0002 T3 in `store.go`; this task starts only after it (see the ADR's Ordering).
- Keeping both tests for one release doubles the age arithmetic in three places; T7 and the N+1 follow-up remove the legacy branch's warning code only after the counter has been read.

## Stop Condition

Stop and ask if an existing store test needs a changed expectation with the switch off, or if ADR-0002 T3 left `CleanupStale` in a shape where the verdict cannot replace its age test without changing which keys are tombstoned.

## Out of Scope

- Writing the evidence from reconcile and the probe, and moving the four outside readers (T2).
- Any change to `LastSeen` merge rules (ADR-0002).

## Verification Log
