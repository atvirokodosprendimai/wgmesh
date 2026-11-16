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
| `pkg/daemon/bounded_guard_test.go` | add | the guard: parses `daemon.go`, `../discovery/dht.go`, `exchange.go`, `gossip.go`, `lan.go` with `go/parser`, lists map-typed struct fields, compares them with the table (19 rows on 2026-11-15: field, file, rule kind, how) |
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
| `TestEveryMapFieldHasABound` | `pkg/daemon/bounded_guard_test.go` | every map-typed struct field in the five files has a table row | — | S1, S2 |
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

## Invariants

- `markTemporarilyOffline`, `clearTemporarilyOffline` and `isTemporarilyOffline` keep their behaviour (existing tests).
- The tick still removes stale peers and logs as before (`daemon.go:942-950`).
- The guard reads source; it does not import `discovery`, so `daemon` still does not depend on it.

## Risks

- The guard is a field-declaration heuristic: a map in a nested type, a slice of maps or a map declared in another file is outside it. The test's doc comment says so, and `TestGuardFailsOnUnlistedMapField` proves the case it does cover.
- The table is long (19 rows) and drifts if the five files gain fields; that is the intended failure.
- Scope amendment (ADR-0007 Decision 1, 2026-11-16): the guard also parses `pkg/node/store.go` and `pkg/discovery/replay_cache.go`; the row count is whatever the code has when T4 runs (19 at `b8b83f3` plus the new fields), and the table, not this text, is the count. Concurrency: every map field in the table names the lock that guards it (M's rule 2026-11-16: no field accessed by more than one goroutine without an atomic or a lock); T4 reads each access once and any bare access it finds is a finding, fixed in its own commit.

## Stop Condition

Stop and ask if the guard cannot read `../discovery` from `pkg/daemon` under the repository's test setup, or if a field has no bound that can be named in a row (that is a new finding, not a row to invent).

## Out of Scope

- A shared TTL map type (permanent: boundary: four members, three release rules, see ADR-0007 Alternatives).
- Maps in other packages (permanent: boundary: ADR-0007 Out of Scope).

## Verification Log
