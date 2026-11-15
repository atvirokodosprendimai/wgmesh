# Task ADR-0007-T2: Bound the three DHT maps that never shrink

**Depends-on:** none
**Blocked-on:** the last task of ADR-0002 (its documentation task) is merged on main: `git cat-file -e origin/main:pkg/node/spec_admit_test.go` exits 0, because the files this task edits are also edited by ADR-0002 tasks
**Covers:** none — no spec
**Estimated scope:** S (single file)
**Owner:** unassigned
**Produces:** `ContactDedupWindow`, `MaxContactedPeers`, `DHTDiscovery.sweepPeerKeyed`
**Consumes:** none
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`contactedPeers`, `controlPeers` and `rendezvousBackoff` stop growing for the life of the process: the first by age with a hard cap, the other two by membership in the PeerStore.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/discovery/dht.go` | edit | `ContactDedupWindow` (60 s) and `MaxContactedPeers` (8192) beside the other constants (`dht.go:30-45`); `markContacted` sweeps and caps (`dht.go:1460`); `sweepPeerKeyed` new; `checkStaleHandshakes` calls it first (`dht.go:888`, driven by `staleTicker`, `dht.go:841`, `:857`); a `clock()` method that returns `time.Now` unless a test sets `d.now` |
| `pkg/discovery/dht_bounds_test.go` | add | the tests below |

Selection: `sweepPeerKeyed` is reached only through `checkStaleHandshakes`, which the 10-second `staleTicker` case calls; `TestCheckStaleHandshakesRunsPeerKeyedSweep` fails when that call is deleted, and the loop line itself is covered by the mutant in S6.

## Ordered Steps

1. [S1] Add `dht_bounds_test.go` with the seven tests and compiling stubs (the constants, `sweepPeerKeyed` that does nothing, the `now` field) so the first run fails on assertions. Run red.
2. [S2] Add the two constants and the clock seam. `ContactDedupWindow` is the longest interval any `markContacted` caller passes (`dht.go:825` passes 60 s); the test in S1 reads every call.
3. [S3] In `markContacted`: when `len(d.contactedPeers) > 256` and at least one window has passed since the last sweep, delete entries older than `ContactDedupWindow`; after that, at `MaxContactedPeers` entries return false for a new address (log once per window); an address already present keeps its normal interval check.
4. [S4] Add `sweepPeerKeyed`: copy the keys of `controlPeers` and `rendezvousBackoff` under `d.mu.RLock`, release it, ask `d.peerStore.Get(key)` for each, then delete the keys the store lacks under `d.mu.Lock`; call it first in `checkStaleHandshakes`.
5. [S5] [proof: acceptance] Update the DHT core spec (`eidos/spec - discovery dht core - bittorent mainline dht announce query persistence and endpoint detection.md`) with the three bounds, in the same commit.
6. [S6] Run the mutants with `adr-verify --mutant`: the sweep threshold removed; the cap compared with `>` instead of `>=`; `sweepPeerKeyed` not called from `checkStaleHandshakes`; the window constant raised to 120 s. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/discovery -run '^(TestMarkContactedSweepsEntriesOlderThanWindow|TestMarkContactedKeepsEntriesInsideWindow|TestMarkContactedFailsClosedAtCap|TestEveryCallerIntervalFitsDedupWindow|TestSweepPeerKeyedDropsKeysNotInStore|TestSweepPeerKeyedKeepsKeysInStore|TestCheckStaleHandshakesRunsPeerKeyedSweep)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 7 ] && go test -race ./pkg/discovery ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestMarkContactedSweepsEntriesOlderThanWindow` | `pkg/discovery/dht_bounds_test.go` | with 300 entries 61 s old and the clock moved on, one `markContacted` call leaves only the new entry | — | S1, S3 |
| `TestMarkContactedKeepsEntriesInsideWindow` | `pkg/discovery/dht_bounds_test.go` | entries 59 s old survive the sweep and still block a second contact inside their interval | — | S1, S3 |
| `TestMarkContactedFailsClosedAtCap` | `pkg/discovery/dht_bounds_test.go` | at `MaxContactedPeers` fresh entries a new address returns false and the map does not grow; a known address is still decided by its interval | — | S1, S3 |
| `TestEveryCallerIntervalFitsDedupWindow` | `pkg/discovery/dht_bounds_test.go` | parsing `dht.go`, every `markContacted` call passes a constant or literal duration no larger than `ContactDedupWindow`; a non-constant argument fails the test | — | S1, S2 |
| `TestSweepPeerKeyedDropsKeysNotInStore` | `pkg/discovery/dht_bounds_test.go` | a key in `controlPeers` and `rendezvousBackoff` that the store lacks is deleted | — | S1, S4 |
| `TestSweepPeerKeyedKeepsKeysInStore` | `pkg/discovery/dht_bounds_test.go` | a key the store holds keeps both entries | — | S1, S4 |
| `TestCheckStaleHandshakesRunsPeerKeyedSweep` | `pkg/discovery/dht_bounds_test.go` | calling `checkStaleHandshakes` with a stale key present removes it | — | S1, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the seven tests |
| 2 — something selects it | `markContacted` is called from four sites; `checkStaleHandshakes` is called by the 10-second ticker; the not-called mutant turns the last test red |
| 3 — the caller can discover it | constants exported with doc comments; the DHT core spec names them |
| 4 — it is used | every DHT contact and every stale check in a running daemon |

## Mutation Log

## Invariants

- `markContacted` still returns true for an address not contacted inside its interval and false inside it (existing `dht_test.go` cases).
- Rendezvous backoff doubling for a key that stays in the store is unchanged (`dht_test.go:87-175`).
- No new goroutine, no new ticker.
- ADR-0002's admit rules are not touched: nothing here decides whether a claim enters the store.

## Risks

- A peer swept from `rendezvousBackoff` while still failing restarts at 3 s backoff only if it left the store first, which is what a new peer has.
- a key can come back between the store check and the delete; the delete is then of an entry for a peer that is present, which restarts its backoff at the minimum, the same as a new peer. `d.mu` is never held while the store lock is taken.

## Stop Condition

Stop and ask if ADR-0002's tasks have changed `markContacted`, `checkStaleHandshakes` or the three maps by the time this starts, or if a caller needs a dedup interval above 60 s.

## Out of Scope

- The cap value 8192 and the fail-closed behaviour are Open Question 3 of ADR-0007; this task implements the recommendation and changes if M answers otherwise.
- Bounding collections outside the daemon and discovery layers (permanent: boundary: the audit covers the long-running peer path).

## Verification Log
