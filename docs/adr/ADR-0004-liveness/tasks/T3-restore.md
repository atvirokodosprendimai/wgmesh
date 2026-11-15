# Task ADR-0004-T3: Treat a restored cache entry as dialable for `PeerDeadTimeout` (#851)

**Depends-on:** T2
**External prerequisite:** ADR-0002 T2 merged on main (it moves `cache.go:126` to `Offer` with hand `Restored`)
**Covers:** none — no spec
**Estimated scope:** S (few files)
**Owner:** unassigned
**Produces:** `RestoredAt` set by the `Restored` merge, `LastAlive` written to the cache file
**Consumes:** `Assess`, `PeerInfo.RestoredAt`, `EnforceLiveness` (T1); the evidence writers (T2)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

After a restart more than five minutes after the last save, the peers read from the cache are in `GetActive` for `PeerDeadTimeout` from the restore, so exchange, DHT and reconcile contact them, and a peer nobody hears from leaves `GetActive` after that and the store after `PeerRemoveTimeout`; the cache file keeps the newest evidence time of each peer.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/node/store.go` | edit | the merge that handles a `Restored` candidate for a key the store does not hold sets `RestoredAt = now`; it never sets `LastSeen` or `HeardFirstHand` (ADR-0002) |
| `pkg/daemon/cache.go` | edit | `SavePeerCache` (`:59-91`) writes `p.LastAlive(now)` (the newest of `LastSeen`, `LastDataPlane`, `RestoredAt`) in `LastSeen`; `RestoreFromCache` (`:94-135`) keeps `CacheExpiration` (24 hours) on the stored time and logs the count of entries that are `Alive` after the restore |
| `pkg/node/liveness.go` | edit | `PeerInfo.LastAlive(now)` (the same newest-of rule `Assess` reads) |
| `pkg/daemon/cache_test.go` | edit | tests below |

Selection: `RestoreFromCache` is called at start (`daemon.go:1551`); the `Restored` merge is what `Assess` reads; a mutant that stops setting `RestoredAt` turns `TestRestoreGivesGrace` red.

## Ordered Steps

1. [S1] Add the tests below with compiling stubs; run red. `TestRestoreGivesGrace` writes a cache file whose entry has `last_seen` one hour ago, restores it into a store with `EnforceLiveness = true`, and asserts the peer is in `GetActive`.
2. [S2] Set `RestoredAt` in the merge for a `Restored` candidate whose key is not in the store; a key already in the store keeps its evidence.
3. [S3] Add `LastAlive` and switch `SavePeerCache` to it; keep the JSON field name `last_seen`.
4. [S4] Change the restore log to say how many peers are alive and how many are only suspected after the restore. [proof: acceptance]
5. [S5] Run the mutants with `adr-verify --mutant`: the merge sets `LastSeen` instead of `RestoredAt`; the restore expiry is removed; `SavePeerCache` writes `LastSeen` again. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/daemon -run '^(TestRestoreGivesGrace|TestRestoredNeverHeardLeavesAfterRemoveTimeout|TestRestoredPeerIsNotFirstHandAndLeavesNoTombstone|TestSaveWritesLastAlive|TestRestoreStillSkipsEntriesOlderThanCacheExpiration)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 5 ] \
 && b=$(go test ./pkg/node -run '^TestRestoredAtIsSetOnlyForNewKeys$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -ge 1 ] \
 && go test -race ./pkg/node ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestRestoreGivesGrace` | `pkg/daemon/cache_test.go` | the one-hour-old entry is in `GetActive` after the restore (switch on); with the switch off it is not, as today | — | S1, S2 |
| `TestRestoredNeverHeardLeavesAfterRemoveTimeout` | `pkg/daemon/cache_test.go` | with the injected clock, the entry is Alive until `PeerDeadTimeout`, Suspect until `PeerRemoveTimeout`, gone from `CleanupStale` after | — | S1, S2 |
| `TestRestoredPeerIsNotFirstHandAndLeavesNoTombstone` | `pkg/daemon/cache_test.go` | `HeardFirstHand` is false for a restored peer and `CleanupStale` removing it leaves no tombstone (ADR-0002 T3) | — | S1, S2 |
| `TestSaveWritesLastAlive` | `pkg/daemon/cache_test.go` | a peer with an old `LastSeen` and a fresh `LastDataPlane` is saved with the fresh time | — | S1, S3 |
| `TestRestoreStillSkipsEntriesOlderThanCacheExpiration` | `pkg/daemon/cache_test.go` | an entry older than 24 hours is not restored | — | S1, S2 |
| `TestRestoredAtIsSetOnlyForNewKeys` | `pkg/node/liveness_test.go` | a `Restored` offer for a key already stored does not change its evidence times | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the six tests |
| 2 — something selects it | `RunWithDHTDiscovery` calls `RestoreFromCache` (`daemon.go:1551`); the mutant that drops `RestoredAt` turns a named test red |
| 3 — the caller can discover it | the restore log line counts alive and suspected peers |
| 4 — it is used | exchange, DHT and reconcile read `GetActive` after every start from N+1 |

## Mutation Log

## Invariants

- A restored entry is never first-hand and never refreshes `LastSeen`; it is not offered while its own tombstone lives (ADR-0002 R2).
- The cache file keeps its path, field names and types; an older version reads it as before.
- `CacheExpiration` stays 24 hours.

## Risks

- A cached endpoint that no longer works is configured on WireGuard for five minutes; the same happens today for any peer for five minutes after its last announcement.
- Merge conflicts with ADR-0002 T1 and T2 on the `Restored` branch; see Ordering.

## Stop Condition

Stop and ask if the `Restored` merge of ADR-0002 does not have a place that knows the key is new, so that `RestoredAt` would also be set for a known key.

## Out of Scope

- Persisting tombstones (permanent: boundary: a restart ends tombstones, so no restore is refused by one).
- Writing the cache more often than `CacheSaveInterval` (permanent: boundary: not a liveness decision).

## Verification Log
