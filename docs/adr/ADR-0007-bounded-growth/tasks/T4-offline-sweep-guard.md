# Task ADR-0007-T4: Sweep `temporaryOffline` and guard every map field with a stated bound

**Depends-on:** T2, T3
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `Daemon.sweepTemporaryOffline`, `staleCleanupTick`, `TestEveryMapFieldHasABound` and its table
**Consumes:** `ContactDedupWindow`, `MaxContactedPeers`, `sweepPeerKeyed` (T2); the callers on `ReadState` (T3)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`temporaryOffline` drops entries past their deadline without being asked about, and a map-typed field added to a long-lived struct without a row naming its bound fails the build.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/daemon.go` | edit | `sweepTemporaryOffline` new (under `offlineMu`); the body of the `staleCleanupLoop` tick (`daemon.go:934-950`) moves to `staleCleanupTick` and calls the sweep |
| `pkg/daemon/bounded_guard_test.go` | add | the guard: parses `daemon.go`, `../discovery/dht.go`, `exchange.go`, `gossip.go`, `lan.go`, `../node/store.go`, `../discovery/replay_cache.go` with `go/parser`, lists map-typed struct fields, compares them with the table (19 rows on 2026-11-15: field, file, rule kind, how) |
| `pkg/daemon/offline_sweep_test.go` | add | the sweep tests |

Selection: `staleCleanupLoop` calls `staleCleanupTick`; the guard test is selected by `go test ./...` and fails when the table and the source differ either way.

## Ordered Steps

1. [S1] Add the five tests and compiling stubs (an empty table, a `sweepTemporaryOffline` that does nothing, an unextracted tick) so the first run fails on assertions. Run red; `TestEveryMapFieldHasABound` must fail by listing the 19 fields.
2. [S2] Fill the table, one row per field with the rule kind of ADR-0007 Decision 1 (`replaced-per-tick`, `swept-vs-active-set`, `age-sweep`, `age-sweep-cap`, `store-membership`, `deadline-sweep`, `paired-set-clear`, `session-local`) and a one-line how that cites the function that releases it.
3. [S3] Implement `sweepTemporaryOffline` and extract `staleCleanupTick`.
4. [S4] Make the checker a function over source text (`checkBoundTable(src map[string]string, table []row) []string`) so a test can feed it a synthetic file.
5. [S5] Run the mutants with `adr-verify --mutant`: one table row deleted; a stale row for a field that no longer exists; `sweepTemporaryOffline` not called from the tick; the sweep deleting live entries. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/daemon -run '^(TestEveryMapFieldHasABound|TestBoundTableHasNoStaleRows|TestGuardFailsOnUnlistedMapField|TestSweepTemporaryOfflineDropsExpiredKeepsLive|TestStaleCleanupTickRunsOfflineSweep)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 5 ] && go test -race ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestEveryMapFieldHasABound` | `pkg/daemon/bounded_guard_test.go` | every map-typed struct field in the seven files (the five of the audit plus `pkg/node/store.go` and `pkg/discovery/replay_cache.go`) has a table row | — | S1, S2 |
| `TestBoundTableHasNoStaleRows` | `pkg/daemon/bounded_guard_test.go` | every table row names a field that exists | — | S1, S2 |
| `TestGuardFailsOnUnlistedMapField` | `pkg/daemon/bounded_guard_test.go` | `checkBoundTable` on a synthetic struct with an unlisted `map[string]time.Time` field returns one finding naming it | — | S1, S4 |
| `TestSweepTemporaryOfflineDropsExpiredKeepsLive` | `pkg/daemon/offline_sweep_test.go` | an entry whose deadline passed is deleted without a lookup; one inside `TemporaryOfflineTTL` stays and `isTemporarilyOffline` still returns true | — | S1, S3 |
| `TestStaleCleanupTickRunsOfflineSweep` | `pkg/daemon/offline_sweep_test.go` | `staleCleanupTick` removes an expired key | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the five tests |
| 2 — something selects it | `staleCleanupLoop` calls the tick (the not-called mutant turns the last test red); the guard runs in every `go test ./...` |
| 3 — the caller can discover it | the guard's failure message names the field and says to add a row; ADR-0007 holds the rule kinds |
| 4 — it is used | every build |

## Mutation Log
- 2026-11-16 · f0f1d0f* · mutant killed · exit 1 · `pkg/daemon/bounded_guard_test.go` · a field without a row: TestEveryMapFieldHasABound red · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036
- 2026-11-16 · f0f1d0f* · mutant killed · exit 1 · `pkg/daemon/bounded_guard_test.go` · a row for a field that does not exist: TestEveryMapFieldHasABound red · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036
- 2026-11-16 · f0f1d0f* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · tick never sweeps: TestStaleCleanupTickRunsOfflineSweep red · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036
- 2026-11-16 · f0f1d0f* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · bare map access: -race test red · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036
- 2026-11-16 · f0f1d0f* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · expired entries stay: TestSweepTemporaryOfflineDropsExpiredKeepsLive red · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036
- 2026-11-16 · 7fd9ed8* · mutant killed · exit 1 · `pkg/daemon/bounded_guard_test.go` · a field without a row and a stale row: TestEveryMapFieldHasABound red · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036
- 2026-11-16 · 7fd9ed8* · mutant killed · exit 1 · `pkg/daemon/bounded_guard_test.go` · a row for a field that does not exist: TestBoundTableHasNoStaleRows red · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036
- 2026-11-16 · 7fd9ed8* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · tick never sweeps: TestStaleCleanupTickRunsOfflineSweep red · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036
- 2026-11-16 · 7fd9ed8* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · bare map access: -race test red · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036
- 2026-11-16 · 7fd9ed8* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · expired entries stay: TestSweepTemporaryOfflineDropsExpiredKeepsLive red · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036
- 2026-11-16 · 7fd9ed8* · mutant killed · exit 1 · `pkg/daemon/bounded_guard_test.go` · bad kind passes: TestGuardFailsOnUnlistedMapField red · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036
- 2026-11-16 · 7fd9ed8* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · a failure count outlives its peer: TestCleanupProbeSessionsDropsFailuresOfInactiveKeysWithoutSession red · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036
- 2026-11-16 · 7fd9ed8* · mutant survived · exit 0 · `pkg/daemon/daemon.go` · lazy init overwrites the start: ? · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036
  ```
  the fence passed with the mechanism broken; it may not materialize, compile, load, or assert on the changed path
  ```
- 2026-11-16 · 7fd9ed8* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · live entries dropped: TestSweepTemporaryOfflineDropsExpiredKeepsLive red · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036

## Invariants

- `markTemporarilyOffline`, `clearTemporarilyOffline` and `isTemporarilyOffline` keep their behaviour (existing tests).
- The tick still removes stale peers and logs as before (`daemon.go:942-950`).
- The guard reads source; it does not import `discovery`, so `daemon` still does not depend on it.

## Risks

- The guard is a field-declaration heuristic: a map in a nested type, a slice of maps or a map declared in another file is outside it. The test's doc comment says so, and `TestGuardFailsOnUnlistedMapField` proves the case it does cover.
- The table is long (19 rows) and drifts if the five files gain fields; that is the intended failure.
- Scope amendment (ADR-0007 Decision 1, 2026-11-16): the guard also parses `pkg/node/store.go` and `pkg/discovery/replay_cache.go`; the row count is whatever the code has when T4 runs (19 at `b8b83f3` plus the new fields), and the table, not this text, is the count. Concurrency: every map field in the table names the lock that guards it (M's rule 2026-11-16: no field accessed by more than one goroutine without an atomic or a lock); T4 reads each access once and any bare access it finds is a finding, fixed in its own commit.
- Beyond the task text, found by the panel and fixed in T4's own commits: `cleanupProbeSessions` also drops `probeFailures` keys outside the active set (a pass with an old peers snapshot could write a count after `removePeer` deleted it and no session would ever release it); `Daemon.startTime` is an `atomic.Pointer` (`GetUptime` wrote it lazily from the RPC goroutine, bare); the guard reports a row whose file was not parsed and names the table, the row shape and the valid kinds. The guard is a checklist (every map-typed struct field in the seven files has a row, a known kind, a how and a lock), not a proof of a bound. Known and left: map fields in the same files that are lifecycle-ordered bare fields are outside the guard (`DHTDiscovery.gossip`, `lan`, `server`, `dhtPort`; `Daemon.dhtDiscovery`, `rpcServer`, `epochManager`), reported to M. One mutant is accepted as surviving: `startTime.CompareAndSwap(nil, &now)` replaced by `Store(&now)` differs only when two first `GetUptime` calls race, by microseconds.

## Stop Condition

Stop and ask if the guard cannot read `../discovery` from `pkg/daemon` under the repository's test setup, or if a field has no bound that can be named in a row (that is a new finding, not a row to invent).

## Out of Scope

- A shared TTL map type (permanent: boundary: four members, three release rules, see ADR-0007 Alternatives).
- Maps in other packages (permanent: boundary: ADR-0007 Out of Scope).

## Verification Log
- 2026-11-16 · f0f1d0f* · exit 1 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:1857 · test-lock-sha256:c05661da1d787acbd1f55876fa9a1639599a90bb723e01f4bab9bea4c8b58df6 · test-lock-file:docs/adr/locks/c05661da1d787acbd1f55876fa9a1639599a90bb723e01f4bab9bea4c8b58df6.lock
  ```
  --- last 10 line(s) of stderr (of 39 after folding 39 raw)
  --- PASS: TestGuardFailsOnUnlistedMapField (0.00s)
  === RUN   TestSweepTemporaryOfflineDropsExpiredKeepsLive
      offline_sweep_test.go:26: expired entry survived the sweep
  --- FAIL: TestSweepTemporaryOfflineDropsExpiredKeepsLive (0.00s)
  === RUN   TestStaleCleanupTickRunsOfflineSweep
      offline_sweep_test.go:65: staleCleanupTick left an expired temporaryOffline entry
  --- FAIL: TestStaleCleanupTickRunsOfflineSweep (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/daemon	0.061s
  FAIL
  ```
- 2026-11-16 · f0f1d0f* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:18211
- 2026-11-16 · f0f1d0f* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:17100
- 2026-11-16 · f0f1d0f* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:16524
- 2026-11-16 · f0f1d0f* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:19650
- 2026-11-16 · f0f1d0f* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:25012
- 2026-11-16 · f0f1d0f* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:14341
- 2026-11-16 · 7fd9ed8* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:1 · test-lock-sha256:9a7bae0e2bd98bef24a4a992ada7e081e217c62a7f519825bf0a8d70dfc5ba35 · test-lock-file:docs/adr/locks/9a7bae0e2bd98bef24a4a992ada7e081e217c62a7f519825bf0a8d70dfc5ba35.lock · test-lock-kind:replace
- 2026-11-16 · 7fd9ed8* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:16151
- 2026-11-16 · 7fd9ed8* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:17082
- 2026-11-16 · 7fd9ed8* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:14592
- 2026-11-16 · 7fd9ed8* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:14860
- 2026-11-16 · 7fd9ed8* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:13438
- 2026-11-16 · 7fd9ed8* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:17678
- 2026-11-16 · 7fd9ed8* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:15076
- 2026-11-16 · 7fd9ed8* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:16343
- 2026-11-16 · 7fd9ed8* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:10580
- 2026-11-16 · 7fd9ed8* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:13085
- 2026-11-16 · c0ad7db* · exit 0 · `set -o pipefail …` · acceptance-sha256:b363e0b9223057feab8f0c209116ffa3b00dc7bd511f2b31393b26442c664036 · ms:13022
