# Task ADR-0004-T3: Treat a restored cache entry as dialable for `PeerDeadTimeout` (#851)

**Depends-on:** T2
**Covers:** none — no spec
**Estimated scope:** S (few files)
**Owner:** unassigned
**Produces:** `RestoredAt` set by the `Restored` merge, `LastAlive` written to the cache file
**Consumes:** `Assess`, `PeerInfo.RestoredAt`, `PeerInfo.LastAlive`, `EnforceLiveness` (T1); the evidence writers (T2)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

After a restart more than five minutes after the last save, the peers read from the cache are in `GetActive` for `PeerDeadTimeout` from the restore, so exchange, DHT and reconcile contact them, and a peer nobody hears from leaves `GetActive` after that and the store after `PeerRemoveTimeout`; the cache file keeps the newest evidence time of each peer.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/node/store.go` | edit | the merge's `!exists` branch (`:123-134`) sets `RestoredAt = ps.now()` when `c.Hand == Restored`; it never sets `LastSeen` or `HeardFirstHand` (ADR-0002); a key already in the store keeps its evidence. `Decide` already refuses a tombstoned key for any hand but first-hand (`admit.go:105`) |
| `pkg/daemon/cache.go` | edit | `buildPeerCache` (`:59-85`, `LastSeen: p.LastSeen.Unix()` at `:81`) and `SavePeerCache` (`:88-103`) write `p.LastAlive(now)` (T1) in the `last_seen` field; `RestoreFromCache` (`:106-116`) and `restorePeers` (`:120-153`, which has its own `now := time.Now()` and the 24-hour `CacheExpiration` test at `:121`, `:128`) keep the expiry on the stored time and log the count of entries that are `Alive` after the restore (`:149`) |
| `pkg/daemon/cache_test.go` | edit | tests below; the existing `TestPeerCacheSaveAndLoad`, `TestCacheExpiration` (`:80`), `TestCacheRestoreOffersRestored` and `TestSavePeerCacheDropsGuessedEndpoint` stay green, and `TestCacheExpiration` is extended (not duplicated) to assert that an entry older than 24 hours is not restored |
| `pkg/node/liveness_test.go` | edit | the tests that need the store's injected clock (unexported `ps.now`) |

Selection: `RestoreFromCache` is called in `RunWithDHTDiscovery` (`daemon.go:1688`; `Run()`, `:349`, never restores and is not changed here); the `Restored` merge is what `Assess` reads; a mutant that stops setting `RestoredAt` turns `TestRestoreGivesGrace` red.

## Ordered Steps

1. [S1] Add the tests below with compiling stubs; run red. `TestRestoreGivesGrace` writes a cache file whose entry has `last_seen` one hour ago, restores it into a store with `EnforceLiveness.Store(true)` (restored by `t.Cleanup`), and asserts the peer is in `GetActive`.
2. [S2] Set `RestoredAt = ps.now()` in the merge's `!exists` branch for a `Restored` candidate (guard `c.Hand == Restored`); a key already in the store keeps its evidence.
3. [S3] Switch `buildPeerCache` to `p.LastAlive(now)` (`LastAlive` itself is T1); keep the JSON field name `last_seen`.
4. [S4] Change the restore log (`:149`) to say how many peers are alive and how many are only suspected after the restore. [proof: acceptance]
5. [S5] Run the mutants with `adr-verify --mutant`: the merge sets `LastSeen` instead of `RestoredAt`; the restore expiry is removed; `SavePeerCache` writes `LastSeen` again. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/daemon -run '^(TestRestoreGivesGrace|TestSaveWritesLastAlive)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 2 ] \
 && b=$(go test ./pkg/node -run '^(TestRestoredAtIsSetOnlyForNewKeys|TestRestoredNeverHeardLeavesAfterRemoveTimeout|TestRestoredPeerIsNotFirstHandAndLeavesNoTombstone)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -ge 3 ] \
 && go test ./pkg/daemon -run '^(TestCacheExpiration|TestCacheRestoreOffersRestored|TestPeerCacheSaveAndLoad|TestSavePeerCacheDropsGuessedEndpoint)$' -count=1 \
 && go test -race ./pkg/node ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestRestoreGivesGrace` | `pkg/daemon/cache_test.go` | the one-hour-old entry is in `GetActive` after the restore (switch on); with the switch off it is not, as today | — | S1, S2 |
| `TestSaveWritesLastAlive` | `pkg/daemon/cache_test.go` | a peer with an old `LastSeen` and a fresh `LastDataPlane` is saved with the fresh time | — | S1, S3 |
| `TestCacheExpiration` (extended; exists, `cache_test.go:80`) | `pkg/daemon/cache_test.go` | an entry older than 24 hours is not restored, with the restore grace in place | — | S1, S2 |
| `TestRestoredAtIsSetOnlyForNewKeys` | `pkg/node/liveness_test.go` | a `Restored` offer for a key already stored does not change its evidence times | — | S1, S2 |
| `TestRestoredNeverHeardLeavesAfterRemoveTimeout` | `pkg/node/liveness_test.go` | with `ps.now` injected, a restored entry is Alive until `PeerDeadTimeout`, Suspect until `PeerRemoveTimeout`, gone from `CleanupStale` after | — | S1, S2 |
| `TestRestoredPeerIsNotFirstHandAndLeavesNoTombstone` | `pkg/node/liveness_test.go` | `HeardFirstHand` is false for a restored peer and `CleanupStale` removing it leaves no tombstone (first-hand peers only, `store.go:383`) | — | S1, S2 |

The two clock tests are in `pkg/node` on purpose: `pkg/daemon` tests cannot set the unexported `ps.now`.

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the six tests |
| 2 — something selects it | `RunWithDHTDiscovery` calls `RestoreFromCache` (`daemon.go:1688`); the mutant that drops `RestoredAt` turns a named test red |
| 3 — the caller can discover it | the restore log line counts alive and suspected peers |
| 4 — it is used | exchange, DHT and reconcile read `GetActive` after every start from N+1 |

## Mutation Log

## Invariants

- A restored entry is never first-hand and never refreshes `LastSeen`; it is not offered while its own tombstone lives (ADR-0002 R2).
- The cache file keeps its path, field names and types; an older version reads it as before.
- `CacheExpiration` stays 24 hours.
- No field, variable or counter accessed by more than one goroutine is bare: `atomic.*` or a lock on every access (M 2026-11-16); the task has a `-race` test for each such field it adds. This task adds no field of its own: `RestoredAt` is written inside the merge under `ps.mu` and read through the store's copies, and T1's `TestLivenessStateIsRaceFree` and the final `-race` run cover the store.

## Risks

- A cached endpoint that no longer works is configured on WireGuard for five minutes; the same happens today for any peer for five minutes after its last announcement.
- The `Restored` branch is shared with ADR-0002's merge rules (merged); a change there must keep the `!exists` guard.

## Stop Condition

Stop and ask if the `Restored` merge has no place that knows the key is new, so that `RestoredAt` would also be set for a known key.

## Out of Scope

- Persisting tombstones (permanent: boundary: a restart ends tombstones, so no restore is refused by one).
- Writing the cache more often than `CacheSaveInterval` (permanent: boundary: not a liveness decision).
- Restoring in `Run()` (`daemon.go:349`), which never restores today (permanent: boundary: not part of #851's path).

## Verification Log
