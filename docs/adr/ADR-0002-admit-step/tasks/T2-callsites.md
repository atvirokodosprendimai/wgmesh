# Task ADR-0002-T2: Send every production claim through `Offer`, and fail the build on a direct one

**Depends-on:** T1
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** every production claim goes through `Offer`; `TestNoDirectPeerStoreMutation`
**Consumes:** `Candidate`, `Hand`, `Source`, `Offer` (T1)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The ten production `peerStore.Update` calls name their source and hand, the first-hand punch reply at `dht.go:1068` stops being labelled second-hand, and a test fails when any production file outside `pkg/node` mutates the store directly.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/node/admit_guard_test.go` | add | `TestNoDirectPeerStoreMutation`: parses every non-test `.go` file outside `pkg/node` and fails on `Update` or `SetPeerDirectly` called on a receiver whose last identifier is `peerStore`, `PeerStore`, `ps` or `store` |
| `pkg/discovery/lan.go` | edit | `:229` LAN, first hand |
| `pkg/discovery/exchange.go` | edit | `:403` HELLO sender, first hand; `:436` REPLY sender, first hand; `:891` rendezvous, first hand; `:1035` transitive, second hand |
| `pkg/discovery/dht.go` | edit | `:1068` and `:1129` through one new helper `admitPunchReply(peerInfo, source)`, first hand; `:1068` passes `dht` (today `dht-transitive`), `:1129` passes the caller's method unchanged (`dht`, `dht-ipv6-sync`) |
| `pkg/discovery/gossip.go` | edit | `:323` first hand, `:340` second hand |
| `pkg/daemon/cache.go` | edit | `:126` restored |
| `pkg/discovery/exchange_test.go`, `pkg/discovery/gossip_test.go`, `pkg/daemon/cache_test.go`, `pkg/discovery/dht_test.go` | edit | the four site tests below, next to the existing ones |

Selection: the guard test is what selects the rule for future code; deleting one converted call back to `Update` turns `TestNoDirectPeerStoreMutation` red, and the guard's own limit (a store held in a variable with another name is not seen) is stated in its doc comment.

## Ordered Steps

1. [S1] Add `TestNoDirectPeerStoreMutation` and the four site tests; run the guard red (it reports the 10 sites of `grep -rn "peerStore\.Update(" --include=*.go . | grep -v _test.go`). Three of the site tests (reply, gossip, cache) describe behaviour that already holds through `legacyCandidate` and start green: they are characterisation tests that keep the conversion honest; only the guard and `TestPunchReplyIsFirstHand` start red.
2. [S2] Convert the ten sites per the table above. Extract `admitPunchReply(peerInfo *node.PeerInfo, source node.Source)` in `dht.go` and call it from `:1068` and `:1129`.
3. [S3] Re-run `grep -rn "peerStore\.Update(" --include=*.go . | grep -v _test.go | wc -l`; it prints 0. Record the command and the count in the commit message.
4. [S4] Run the mutants with `adr-verify --mutant`: one converted site back to `Update`; the guard's name list loses `SetPeerDirectly`; `admitPunchReply` passes `SecondHand`; `:1129` loses its caller's method. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/node ./pkg/discovery ./pkg/daemon -run '^(TestNoDirectPeerStoreMutation|TestHandleReplyOffersFirstHand|TestGossipTransitiveOffersSecondHand|TestCacheRestoreOffersRestored|TestPunchReplyIsFirstHand)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 5 ] && [ "$(grep -rn "peerStore\.Update(" --include=*.go . | grep -v _test.go | wc -l)" -eq 0 ] && go test -race ./pkg/node ./pkg/discovery ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestNoDirectPeerStoreMutation` | `pkg/node/admit_guard_test.go` | no non-test file outside `pkg/node` calls `Update` or `SetPeerDirectly` on a store-named receiver | — | S1, S2, S3 |
| `TestHandleReplyOffersFirstHand` | `pkg/discovery/exchange_test.go` | a REPLY that carries a new mesh IP for a known sender replaces it, a `KnownPeers` entry for the same key does not | — | S1, S2 |
| `TestGossipTransitiveOffersSecondHand` | `pkg/discovery/gossip_test.go` | a gossip `KnownPeers` mention cannot rewrite the named peer's mesh IP, its own announcement can | — | S1, S2 |
| `TestCacheRestoreOffersRestored` | `pkg/daemon/cache_test.go` | restored entries do not refresh `LastSeen` and do not withdraw routes | — | S1, S2 |
| `TestPunchReplyIsFirstHand` | `pkg/discovery/dht_test.go` | `admitPunchReply` refreshes `LastSeen` and may rewrite the peer's own fields | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the five tests |
| 2 — something selects it | the guard test selects the rule for every future file; a site converted back turns it red |
| 3 — the caller can discover it | the `Update` doc comment says test-only and names `Offer`; T11 amends the specs |
| 4 — it is used | all ten production claims |

## Mutation Log
- 2026-11-15 · 56b3aa8* · mutant killed · exit 1 · `pkg/discovery/lan.go` · one converted site back to Update: guard and grep fence must go red · acceptance-sha256:28c1222a1f7402915aeab617458091a3dbeb44cbad14766a83bdd9e9f453a9c1
- 2026-11-15 · 56b3aa8* · mutant survived · exit 0 · `pkg/node/admit_guard_test.go` · guard loses SetPeerDirectly: a guard test with a SetPeerDirectly bait must go red · acceptance-sha256:28c1222a1f7402915aeab617458091a3dbeb44cbad14766a83bdd9e9f453a9c1
  ```
  the fence passed with the mechanism broken; it may not materialize, compile, load, or assert on the changed path
  ```
- 2026-11-15 · 56b3aa8* · mutant killed · exit 1 · `pkg/discovery/dht.go` · admitPunchReply passes SecondHand: TestPunchReplyIsFirstHand must go red · acceptance-sha256:28c1222a1f7402915aeab617458091a3dbeb44cbad14766a83bdd9e9f453a9c1
- 2026-11-15 · 56b3aa8* · mutant survived · exit 0 · `pkg/discovery/dht.go` · :1129 loses its caller's method: dht-ipv6-sync subtest must go red · acceptance-sha256:28c1222a1f7402915aeab617458091a3dbeb44cbad14766a83bdd9e9f453a9c1
  ```
  the fence passed with the mechanism broken; it may not materialize, compile, load, or assert on the changed path
  ```
- 2026-11-15 · dad3239* · mutant killed · exit 1 · `pkg/node/admit_guard_test.go` · guard loses SetPeerDirectly: TestGuardFlagsEveryMutatingMethod must go red · acceptance-sha256:28c1222a1f7402915aeab617458091a3dbeb44cbad14766a83bdd9e9f453a9c1
- 2026-11-15 · dad3239* · mutant killed · exit 1 · `pkg/discovery/dht.go` · :1129 loses its caller's method: TestExchangeWithAddressStoresCallersMethod must go red · acceptance-sha256:28c1222a1f7402915aeab617458091a3dbeb44cbad14766a83bdd9e9f453a9c1

## Invariants

- Method strings stored in `DiscoveredVia` are unchanged for every site except `dht.go:1068`, which now says `dht` (was `dht-transitive`); `dht-ipv6-sync` and its endpoint rank are kept.
- No source changes its parsing, its sender-address handling or its rate limiting.
- `pkg/daemon` still does not import `pkg/discovery`; `pkg/discovery` names the new types through the aliases T1 added.

## Risks

- The guard works by receiver name, so a store assigned to a variable named otherwise is missed; the doc comment says so, and the mutants include a renamed receiver being caught by the name list.
- Test doubles: none; the site tests build the real `PeerExchange`, `MeshGossip` and `PeerStore` the neighbouring tests already build.

## Stop Condition

Stop and ask if a site needs a hand that is neither first, second nor restored, or if `daemon` would have to import `discovery` to reach `Offer`.

## Out of Scope

- Tombstones and `Remove` (T3).
- Any change to what a source parses or validates (T7, T8).

## Verification Log
- 2026-11-15 · 56b3aa8* · exit 1 · `set -o pipefail …` · acceptance-sha256:28c1222a1f7402915aeab617458091a3dbeb44cbad14766a83bdd9e9f453a9c1 · ms:1772 · test-lock-sha256:ad1f372f33fa1aa7da02640407cfd573ea9769075c999f7275d2c890ddda9b59 · test-lock-file:docs/adr/locks/ad1f372f33fa1aa7da02640407cfd573ea9769075c999f7275d2c890ddda9b59.lock
  ```
  --- last 10 line(s) of stderr (of 43 after folding 44 raw)
  2026/11/15 20:19:59 [WARN] Secret does not use wgmesh:// format — it may have low entropy. Use 'wgmesh init --secret' to generate a cryptographically strong secret.
  --- PASS: TestGossipTransitiveOffersSecondHand (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/discovery	0.009s
  === RUN   TestCacheRestoreOffersRestored
  2026/11/15 20:19:59 [Cache] Restored 1 peers from cache
  --- PASS: TestCacheRestoreOffersRestored (0.00s)
  PASS
  ok  	github.com/atvirokodosprendimai/wgmesh/pkg/daemon	0.010s
  FAIL
  ```
- 2026-11-15 · 56b3aa8* · exit 0 · `set -o pipefail …` · acceptance-sha256:28c1222a1f7402915aeab617458091a3dbeb44cbad14766a83bdd9e9f453a9c1 · ms:16978
- 2026-11-15 · 56b3aa8* · exit 0 · `set -o pipefail …` · acceptance-sha256:28c1222a1f7402915aeab617458091a3dbeb44cbad14766a83bdd9e9f453a9c1 · ms:13912
- 2026-11-15 · 56b3aa8* · exit 0 · `set -o pipefail …` · acceptance-sha256:28c1222a1f7402915aeab617458091a3dbeb44cbad14766a83bdd9e9f453a9c1 · ms:13061
- 2026-11-15 · 56b3aa8* · exit 0 · `set -o pipefail …` · acceptance-sha256:28c1222a1f7402915aeab617458091a3dbeb44cbad14766a83bdd9e9f453a9c1 · ms:14263
- 2026-11-15 · dad3239* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:28c1222a1f7402915aeab617458091a3dbeb44cbad14766a83bdd9e9f453a9c1 · ms:1 · test-lock-sha256:2af4a8d60ef3a29e68fa65e1301aa8c03dc599447f74e04b729374b3d6cf8ba4 · test-lock-file:docs/adr/locks/2af4a8d60ef3a29e68fa65e1301aa8c03dc599447f74e04b729374b3d6cf8ba4.lock · test-lock-kind:replace
- 2026-11-15 · dad3239* · exit 0 · `set -o pipefail …` · acceptance-sha256:28c1222a1f7402915aeab617458091a3dbeb44cbad14766a83bdd9e9f453a9c1 · ms:14936
- 2026-11-15 · dad3239* · exit 0 · `set -o pipefail …` · acceptance-sha256:28c1222a1f7402915aeab617458091a3dbeb44cbad14766a83bdd9e9f453a9c1 · ms:13711
- 2026-11-15 · dad3239* · exit 0 · `set -o pipefail …` · acceptance-sha256:28c1222a1f7402915aeab617458091a3dbeb44cbad14766a83bdd9e9f453a9c1 · ms:10408
