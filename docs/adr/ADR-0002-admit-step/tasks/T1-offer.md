# Task ADR-0002-T1: Add `PeerStore.Offer` with an explicit hand, and pin the merged rules

**Depends-on:** none
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `node.Candidate`, `node.Source`, `node.Hand`, `node.Decision`, `PeerStore.Offer`, `node.Policy` and `PeerStore.SetPolicy`
**Consumes:** none
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`PeerStore.Offer(Candidate) Decision` is the one place that decides what a claim may do, trust is read from an explicit `Hand` instead of the substring `transitive`, and the rules already merged (`d50c51e`, `7414656`) behave exactly as before.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/node/admit.go` | add | `Source`, `Hand`, `Candidate`, `Decision`, `Reason`, `Policy` (fields `Local func() (key, ip string)`, `MeshNet *net.IPNet`, `Derive func(pubKey string, nonce int) string`, `OnWarn func(rule string)`; all optional), `Decide`, `Offer`, `SetPolicy` |
| `pkg/daemon/peerstore_compat.go` | edit | aliases for `Candidate`, `Hand`, `Source`, `Decision`, `Policy` so `pkg/discovery`, which imports `pkg/daemon` and not `pkg/node`, can name them |
| `pkg/node/store.go` | edit | the body of `Update` becomes the merge step keyed on `Hand`; `Update` forwards to `Offer` through `legacyCandidate`; an unexported `offers` counter is incremented by `Offer`; the peer-cap drop returns a named refusal |
| `pkg/node/admit_test.go` | add | tests below and `BenchmarkOfferAtCap` |

Selection: `Update` forwards to `Offer`, so the 58 existing test call sites (`grep -rn "peerStore.Update\|[sS]tore.Update(\|ps.Update(" --include=*_test.go . | wc -l`, 2026-11-15) and every production call until T2 run through the new code; a mutant that makes `Update` skip `Offer` turns `TestUpdateGoesThroughOffer` red (the characterisation test alone cannot tell, the stored values are the same).

## Ordered Steps

1. [S1] Add `pkg/node/admit_test.go` with the five tests and compiling stubs (`Offer` that applies nothing, the types) so the first run fails on assertions, not on the build. `TestOfferMatchesLegacyUpdate` asserts literal field values for the scenarios of `store_transitive_test.go` and `store_routes_test.go` (transitive never rewrites mesh IP, routes or introducer; a first-hand empty list withdraws routes; a transitive or restored empty list does not; a transitive fill of an empty field). Run red.
2. [S2] Add the types. `Candidate.Method()` returns `dht`, `dht-transitive`, `dht-rendezvous`, `dht-ipv6-sync`, `gossip`, `gossip-transitive`, `lan`, `cache` exactly as today (a `Source` is a typed string, so a new label such as `dht-ipv6-sync` passes through unchanged) so `DiscoveredVia`, `EndpointMethod` and the cache file are unchanged. `Hand` is `FirstHand`, `SecondHand` or `Restored`.
3. [S3] Implement `Decide` (pure: snapshot in, `Decision` out; this task has one refusal, the peer cap at `DefaultMaxPeers`, with reason `peer-cap`) and make the merge read `Hand`: `SecondHand` takes the branches that `isTransitiveMethod` takes today, `Restored` takes the branches that `method == "cache"` takes today (no `LastSeen` refresh, no route withdrawal, may rewrite fields as before).
4. [S4] Implement `legacyCandidate(info, method)` for `Update`: `cache` is `Restored`, a method containing `transitive` is `SecondHand`, anything else `FirstHand`. Mark `Update` in its doc comment as kept for tests; T2 removes the production callers.
5. [S5] Run `go test -bench BenchmarkOfferAtCap -run '^$' ./pkg/node` with 1000 peers in the store and record the ns/op, the date and the machine in the commit message. [proof: human: the number is the input to the Consequences note on lock hold time; adr-verify cannot see it]
6. [S6] Run the mutants with `adr-verify --mutant`: `Update` bypasses `Offer`; the merge reads the method string instead of `Hand`; the peer-cap branch returns `Admit`. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/node -run '^(TestOfferMatchesLegacyUpdate|TestOfferHandIsExplicitNotSubstring|TestOfferRefusesAtPeerCapWithReason|TestOfferNilPolicyIsPermissive|TestUpdateGoesThroughOffer)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 5 ] && go test -race ./pkg/node ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestOfferMatchesLegacyUpdate` | `pkg/node/admit_test.go` | the merged transitive and withdraw rules give the same stored fields through `Offer` and through `Update` | — | S1, S3, S4 |
| `TestOfferHandIsExplicitNotSubstring` | `pkg/node/admit_test.go` | a `FirstHand` candidate whose method string contains `transitive` rewrites first-hand fields; a `SecondHand` one does not | — | S1, S2, S3 |
| `TestOfferRefusesAtPeerCapWithReason` | `pkg/node/admit_test.go` | the 1001st new key is refused with reason `peer-cap` and the store keeps 1000 | — | S1, S3 |
| `TestOfferNilPolicyIsPermissive` | `pkg/node/admit_test.go` | a store without `SetPolicy` admits every candidate the old `Update` admitted | — | S1, S3 |
| `TestUpdateGoesThroughOffer` | `pkg/node/admit_test.go` | one `Update` call increments the store's offer counter by one | — | S1, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the four tests |
| 2 — something selects it | `Update` calls `Offer`; the `Update`-bypass mutant turns `TestUpdateGoesThroughOffer` red |
| 3 — the caller can discover it | exported with doc comments; T10 amends the peer store spec |
| 4 — it is used | every production claim from T2 on |

## Mutation Log

## Invariants

- Stored values for every scenario in `store_transitive_test.go`, `store_routes_test.go` and `pkg/daemon/peerstore_test.go` do not change.
- `DiscoveredVia` and `EndpointMethod` strings are unchanged, so the peer cache file and `daemon.status` look the same.
- `pkg/node` imports nothing from `pkg/daemon` or `pkg/discovery`, and the secret never enters it.
- A store with no policy behaves as it does today.

## Risks

- The substring and the explicit hand disagree for a method string nobody labelled; `legacyCandidate` keeps today's reading for `Update` callers, and T2 removes production use of it.
- Test doubles: none; the tests use the real store.

## Stop Condition

Stop and ask if any existing store test needs a changed expectation, or if `Decide` cannot stay a function of a snapshot (for example if a rule needs a clock outside the store).

## Out of Scope

- Moving call sites to `Offer` (T2).
- Any new refusal rule other than the peer cap (T3, T4, T6, T7).

## Verification Log
