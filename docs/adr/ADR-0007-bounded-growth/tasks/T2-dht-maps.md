# Task ADR-0007-T2: Bound the three DHT maps that never shrink

**Depends-on:** none
**Resolved:** ADR-0002 is merged on main (f3860c6, 2026-11-16; `git cat-file -e origin/main:pkg/node/spec_admit_test.go` exits 0)
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
- 2026-11-16 · d246b4a* · mutant killed · exit 1 · `pkg/discovery/dht.go` · no sweep ever: TestMarkContactedSweepsEntriesOlderThanWindow red · acceptance-sha256:7112c7a0c9e635e61c6f62f1bf6e21d211dd21d0dd16ae98883abcabaebb798d
- 2026-11-16 · d246b4a* · mutant killed · exit 1 · `pkg/discovery/dht.go` · off by one at the cap: TestMarkContactedFailsClosedAtCap red · acceptance-sha256:7112c7a0c9e635e61c6f62f1bf6e21d211dd21d0dd16ae98883abcabaebb798d
- 2026-11-16 · d246b4a* · mutant killed · exit 1 · `pkg/discovery/dht.go` · ticker never sweeps: TestCheckStaleHandshakesRunsPeerKeyedSweep red · acceptance-sha256:7112c7a0c9e635e61c6f62f1bf6e21d211dd21d0dd16ae98883abcabaebb798d
- 2026-11-16 · d246b4a* · mutant survived · exit 0 · `pkg/discovery/dht.go` · window above caller interval: TestEveryCallerIntervalFitsDedupWindow / sweep tests red · acceptance-sha256:7112c7a0c9e635e61c6f62f1bf6e21d211dd21d0dd16ae98883abcabaebb798d
  ```
  the fence passed with the mechanism broken; it may not materialize, compile, load, or assert on the changed path
  ```
- 2026-11-16 · d246b4a* · mutant killed · exit 1 · `pkg/discovery/dht.go` · deletes present keys: TestSweepPeerKeyedKeepsKeysInStore red · acceptance-sha256:7112c7a0c9e635e61c6f62f1bf6e21d211dd21d0dd16ae98883abcabaebb798d
- 2026-11-16 · d246b4a* · mutant killed · exit 1 · `pkg/discovery/dht.go` · rendezvousBackoff keeps absent keys: TestSweepPeerKeyedDropsKeysNotInStore red · acceptance-sha256:7112c7a0c9e635e61c6f62f1bf6e21d211dd21d0dd16ae98883abcabaebb798d
- 2026-11-16 · d246b4a* · mutant killed · exit 1 · `pkg/discovery/dht.go` · sweep drops entries inside the window: TestMarkContactedKeepsEntriesInsideWindow red · acceptance-sha256:7112c7a0c9e635e61c6f62f1bf6e21d211dd21d0dd16ae98883abcabaebb798d

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

- The cap value 8192 and the fail-closed behaviour are Decision 3 of ADR-0007 (M 2026-11-15).
- Bounding collections outside the daemon and discovery layers (permanent: boundary: the audit covers the long-running peer path).

## Verification Log
- 2026-11-16 · d246b4a* · exit 1 · `set -o pipefail …` · acceptance-sha256:7112c7a0c9e635e61c6f62f1bf6e21d211dd21d0dd16ae98883abcabaebb798d · ms:1834 · test-lock-sha256:530277ad72f56d1088060c59b7cb3ef27a1e36008601c231e7ee1014da41bc19 · test-lock-file:docs/adr/locks/530277ad72f56d1088060c59b7cb3ef27a1e36008601c231e7ee1014da41bc19.lock
  ```
  --- last 10 line(s) of stderr (of 29 after folding 29 raw)
  2026/11/16 08:41:56 [WARN] Secret does not use wgmesh:// format — it may have low entropy. Use 'wgmesh init --secret' to generate a cryptographically strong secret.
  --- PASS: TestSweepPeerKeyedKeepsKeysInStore (0.00s)
  === RUN   TestCheckStaleHandshakesRunsPeerKeyedSweep
  2026/11/16 08:41:56 [WARN] Secret does not use wgmesh:// format — it may have low entropy. Use 'wgmesh init --secret' to generate a cryptographically strong secret.
      dht_bounds_test.go:224: checkStaleHandshakes left a controlPeers key the store lacks
      dht_bounds_test.go:227: checkStaleHandshakes left a rendezvousBackoff key the store lacks
  --- FAIL: TestCheckStaleHandshakesRunsPeerKeyedSweep (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/discovery	0.018s
  FAIL
  ```
- 2026-11-16 · d246b4a* · exit 0 · `set -o pipefail …` · acceptance-sha256:7112c7a0c9e635e61c6f62f1bf6e21d211dd21d0dd16ae98883abcabaebb798d · ms:14103
- 2026-11-16 · d246b4a* · exit 0 · `set -o pipefail …` · acceptance-sha256:7112c7a0c9e635e61c6f62f1bf6e21d211dd21d0dd16ae98883abcabaebb798d · ms:11764
- 2026-11-16 · d246b4a* · exit 0 · `set -o pipefail …` · acceptance-sha256:7112c7a0c9e635e61c6f62f1bf6e21d211dd21d0dd16ae98883abcabaebb798d · ms:11926
- 2026-11-16 · d246b4a* · exit 0 · `set -o pipefail …` · acceptance-sha256:7112c7a0c9e635e61c6f62f1bf6e21d211dd21d0dd16ae98883abcabaebb798d · ms:10395
- 2026-11-16 · d246b4a* · exit 0 · `set -o pipefail …` · acceptance-sha256:7112c7a0c9e635e61c6f62f1bf6e21d211dd21d0dd16ae98883abcabaebb798d · ms:12197
- 2026-11-16 · d246b4a* · exit 0 · `set -o pipefail …` · acceptance-sha256:7112c7a0c9e635e61c6f62f1bf6e21d211dd21d0dd16ae98883abcabaebb798d · ms:11634
- 2026-11-16 · d246b4a* · exit 0 · `set -o pipefail …` · acceptance-sha256:7112c7a0c9e635e61c6f62f1bf6e21d211dd21d0dd16ae98883abcabaebb798d · ms:14343
- 2026-11-16 · d246b4a* · exit 0 · `set -o pipefail …` · acceptance-sha256:7112c7a0c9e635e61c6f62f1bf6e21d211dd21d0dd16ae98883abcabaebb798d · ms:14204
