# Task ADR-0002-T3: Keep a tombstone for removed peers and refuse second-hand re-adds (#854)

**Depends-on:** T2
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `PeerStore.Withdraw(key, cause)`, tombstones, reason `tombstoned`
**Consumes:** every production claim goes through `Offer` (T2)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

A peer removed by GOODBYE, health eviction or stale cleanup is not brought back by a `KnownPeers` mention or a cache restore for `TombstoneTTL` (5 minutes), and its own first-hand packet clears the tombstone at once.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/node/admit.go` | edit | rule R2 in `Decide`: refuse `SecondHand` and `Restored` for a tombstoned key, reason `tombstoned`; a `FirstHand` offer clears it |
| `pkg/node/types.go` | edit | `PeerInfo.heardFirstHand bool` (unexported: only the store reads it) |
| `pkg/node/store.go` | edit | `Withdraw(key, cause)` deletes and tombstones; `CleanupStale` tombstones what it removes only when `PeerInfo.heardFirstHand` is true (a new field the merge sets on a `FirstHand` offer), so a peer known only by mentions is not hidden for ten more minutes each time cleanup drops it; the tombstone map is bounded at `DefaultMaxPeers` (oldest dropped first) and an injectable clock for the tombstone timestamps and `CleanupStale` (`ps.now`, default `time.Now`; `merge` keeps `time.Now`) |
| `pkg/discovery/exchange.go` | edit | `:348` GOODBYE calls `Withdraw` with cause `goodbye` |
| `pkg/daemon/daemon.go` | edit | `:1394` eviction calls `Withdraw` with cause `evicted` |
| `pkg/node/admit_guard_test.go` | edit | the guard's name list gains `Remove` |
| `pkg/node/admit_test.go`, `pkg/discovery/exchange_aad_test.go`, `pkg/daemon/health_test.go` | edit | tests below |

Selection: `Withdraw` is selected by its two production callers and by the extended guard; reverting `exchange.go:348` to `Remove` turns `TestNoDirectPeerStoreMutation` and `TestGoodbyeLeavesTombstone` red.

## Ordered Steps

1. [S1] Add the tests below with a compiling `Withdraw` stub that only deletes; run red.
2. [S2] Implement the tombstone map, the constant `TombstoneTTL = 5 * time.Minute` (its own constant, not `PeerRemoveTimeout`), `Withdraw`, the `CleanupStale` hook and the clock seam.
3. [S3] Add rule R2 to `Decide`; keep every other rule as T2 left it.
4. [S4] Switch the two callers; extend the guard's name list with `Remove`; re-run `grep -rnE "peerStore\.Remove\(" --include=*.go . | grep -v _test.go | wc -l`, which prints 0, and record it in the commit message.
5. [S5] Run the mutants with `adr-verify --mutant`: R2 returns `Admit`; `FirstHand` no longer clears the tombstone; `CleanupStale` stops tombstoning; the tombstone lifetime is 0. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/node -run '^(TestTombstoneBlocksSecondHandReadd|TestFirstHandClearsTombstone|TestTombstoneExpiresAfterTombstoneTTL|TestTombstoneTTLIsFiveMinutes|TestCleanupStaleLeavesTombstone|TestTombstoneSetIsBounded|TestNoDirectPeerStoreMutation)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 7 ] \
 && b=$(go test ./pkg/discovery -run '^TestGoodbyeLeavesTombstone$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -ge 1 ] \
 && c=$(go test ./pkg/daemon -run '^TestEvictionLeavesTombstone$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$c" -ge 1 ] \
 && go test -race ./pkg/node ./pkg/discovery ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestTombstoneBlocksSecondHandReadd` | `pkg/node/admit_test.go` | after `Withdraw`, a `SecondHand` offer with `Introducer: true` is refused with reason `tombstoned` and the store has no entry | — | S1, S2, S3 |
| `TestFirstHandClearsTombstone` | `pkg/node/admit_test.go` | the peer's own first-hand offer is admitted and later second-hand offers are admitted again | — | S1, S3 |
| `TestTombstoneExpiresAfterTombstoneTTL` | `pkg/node/admit_test.go` | with the injected clock just before `TombstoneTTL` the offer is refused, just after it is admitted | — | S1, S2, S3 |
| `TestTombstoneTTLIsFiveMinutes` | `pkg/node/admit_test.go` | `TombstoneTTL` equals 5 minutes and is a constant of its own, not an alias of `PeerRemoveTimeout` (the two differ) | — | S1, S2 |
| `TestCleanupStaleLeavesTombstone` | `pkg/node/admit_test.go` | a once-first-hand key removed by `CleanupStale` refuses a second-hand re-add; a mention-only key does not | — | S1, S2 |
| `TestTombstoneSetIsBounded` | `pkg/node/admit_test.go` | 2000 withdrawals keep at most `DefaultMaxPeers` tombstones, the newest kept | — | S1, S2 |
| `TestNoDirectPeerStoreMutation` | `pkg/node/admit_guard_test.go` | `Remove` on a store-named receiver outside `pkg/node` fails the guard | — | S1, S4 |
| `TestGoodbyeLeavesTombstone` | `pkg/discovery/exchange_aad_test.go` | an AAD GOODBYE removes the peer and a following `KnownPeers` mention does not bring it back | — | S1, S4 |
| `TestEvictionLeavesTombstone` | `pkg/daemon/health_test.go` | `evictPeerFromPool` leaves a tombstone that refuses a second-hand re-add | — | S1, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the eight tests |
| 2 — something selects it | `Withdraw` is called from `exchange.go` and `daemon.go`; reverting either turns a named test red |
| 3 — the caller can discover it | doc comment on `Withdraw`; T11 amends the peer store spec |
| 4 — it is used | the log line `[Admit] refused tombstoned peer` (rate-limited per key) shows the first suppressed resurrection of each tombstone |

## Mutation Log
- 2026-11-15 · ba34965* · mutant killed · exit 1 · `pkg/node/admit.go` · R2 returns Admit: TestTombstoneBlocksSecondHandReadd must go red · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a
- 2026-11-15 · ba34965* · mutant killed · exit 1 · `pkg/node/admit.go` · FirstHand no longer clears the tombstone: TestFirstHandClearsTombstone must go red · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a
- 2026-11-15 · ba34965* · mutant killed · exit 1 · `pkg/node/store.go` · CleanupStale stops tombstoning: TestCleanupStaleLeavesTombstone must go red · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a
- 2026-11-15 · ba34965* · mutant killed · exit 1 · `pkg/node/store.go` · TombstoneTTL is 0: TestTombstoneTTLIsFiveMinutes and the expiry test must go red · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a
- 2026-11-15 · ad237a9* · mutant killed · exit 1 · `pkg/node/store.go` · expiry boundary: the exact-TTL read in TestTombstoneExpiresAfterTombstoneTTL must go red · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a
- 2026-11-15 · ad237a9* · mutant killed · exit 1 · `pkg/node/store.go` · Withdraw tombstones keys not in the store: TestWithdrawOfUnknownKeyLeavesNoTombstone must go red · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a
- 2026-11-15 · ad237a9* · mutant killed · exit 1 · `pkg/node/store.go` · expired entries not pruned at the cap: TestTombstonePrunesExpiredBeforeEvictingOldest must go red · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a

## Invariants

- A peer's own first-hand packet is never refused by a tombstone.
- Tombstones are memory only and bounded; nothing is written to disk.
- `markTemporarilyOffline` and its 30-second flag are unchanged.
- Through `Update` the stored fields are as T2 left them; the only addition is the unexported `heardFirstHand` flag on stored entries.

## Risks

- The store lock is held while pruning expired tombstones; pruning is done at most once per offer and is bounded by the cap.
- A health-evicted peer is hidden up to five minutes unless heard first-hand; today it can return after the 30-second `TemporaryOfflineTTL`. Its own packet ends it.
- A member can forge GOODBYE packets (the AAD key is the shared gossip key). `Withdraw` therefore tombstones only a key that is in the store, so a flood of junk-key GOODBYEs cannot fill the tombstone set; a forged GOODBYE for a real peer is as strong as the `Remove` it replaces, plus five minutes of hiding.

## Stop Condition

Stop and ask if clearing on first-hand would let a replayed first-hand packet (ADR-0001's window) resurrect a removed peer in a way a test can show; that would need a nonce-aware rule.

## Out of Scope

- Persisting tombstones across restarts (permanent: boundary: a restart forgets them and the cache restore is itself refused only while the process lives; revisit if #851's cache work makes it matter).
- Changing when a peer is considered dead (permanent: boundary: liveness group).

## Verification Log
- 2026-11-15 · ba34965* · exit 1 · `set -o pipefail …` · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a · ms:696 · test-lock-sha256:c2b2b24cbf1bbe3bc4b3eaac7c9c317df978ed439648e2f8ab5e3420b352328e · test-lock-file:docs/adr/locks/c2b2b24cbf1bbe3bc4b3eaac7c9c317df978ed439648e2f8ab5e3420b352328e.lock
  ```
  --- last 10 line(s) of stderr (of 43 after folding 43 raw)
      admit_test.go:343: the newest withdrawal: decision = {Verdict:0 Reason: Dropped:[] Warnings:[]}, want Refuse tombstoned
  --- FAIL: TestTombstoneSetIsBounded (0.00s)
  === NAME  TestTombstoneBlocksSecondHandReadd
      admit_test.go:255: hand 1: refused offer left an entry: &{WGPubKey:k Hostname: MeshIP: MeshIPv6: Endpoint: Introducer:true RoutableNetworks:[] LastSeen:2026-11-15 20:47:37.212490864 +0200 EET m=+0.031991869 DiscoveredVia:[dht-transitive] Latency:<nil> NATType: EndpointMethod: HeardFirstHand:false}
      admit_test.go:253: hand 2: decision = {Verdict:0 Reason: Dropped:[] Warnings:[]}, want Refuse tombstoned
      admit_test.go:255: hand 2: refused offer left an entry: &{WGPubKey:k Hostname: MeshIP: MeshIPv6: Endpoint: Introducer:true RoutableNetworks:[] LastSeen:2026-11-15 20:47:37.212490864 +0200 EET m=+0.031991869 DiscoveredVia:[dht-transitive dht] Latency:<nil> NATType: EndpointMethod: HeardFirstHand:false}
  --- FAIL: TestTombstoneBlocksSecondHandReadd (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/node	0.035s
  FAIL
  ```
- 2026-11-15 · ba34965* · exit 0 · `set -o pipefail …` · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a · ms:12439
- 2026-11-15 · ba34965* · exit 0 · `set -o pipefail …` · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a · ms:15220
- 2026-11-15 · ba34965* · exit 0 · `set -o pipefail …` · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a · ms:14741
- 2026-11-15 · ba34965* · exit 0 · `set -o pipefail …` · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a · ms:14156
- 2026-11-15 · ba34965* · exit 0 · `set -o pipefail …` · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a · ms:9776
- 2026-11-15 · ad237a9* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a · ms:1 · test-lock-sha256:4419eea0a1473d1e5d1daf116c2f5f866584a5284344a99c67886d991dd56914 · test-lock-file:docs/adr/locks/4419eea0a1473d1e5d1daf116c2f5f866584a5284344a99c67886d991dd56914.lock · test-lock-kind:replace
- 2026-11-15 · ad237a9* · exit 0 · `set -o pipefail …` · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a · ms:13786
- 2026-11-15 · ad237a9* · exit 0 · `set -o pipefail …` · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a · ms:15649
- 2026-11-15 · ad237a9* · exit 0 · `set -o pipefail …` · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a · ms:14238
- 2026-11-15 · ad237a9* · exit 0 · `set -o pipefail …` · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a · ms:10390
- 2026-11-16 · 51e018f* · exit 0 · `set -o pipefail …` · acceptance-sha256:f97ccc7ab11d6627c996e19da0af66a55df87f1512005275768602a9b188555a · ms:10039
