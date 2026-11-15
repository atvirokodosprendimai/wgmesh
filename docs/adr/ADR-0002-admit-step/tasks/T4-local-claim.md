# Task ADR-0002-T4: Put the local node into the mesh IP claim table and order claims (#856)

**Depends-on:** T1
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `node.Claim`, `node.ResolveClaims`, the claim order (bound, then lower key), `node.EnforceIPBinding`, the daemon-built `Policy` (local claim, derive closure, `OnWarn`) and `wgmesh_admit_warnings_total{rule}`
**Consumes:** `Policy`, `PeerStore.SetPolicy`, `Decision` (T1)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

A mesh IP collision between the local node and a remote peer is detected and decided by the same order as any other pair, and the order is "bound claim first, then lower public key" only when `EnforceIPBinding` is true; at release N the lower key still decides and the difference is counted.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/node/claims.go` | add | `Claim{Key, IP string}`, `Bound(claim, derive, maxNonce)`, `Winner(a, b Claim, derive, warn func(rule string))`, `ResolveClaims(local Claim, peers []*PeerInfo, derive) []Collision`; `maxNonce` is 10, the loop bound of `DeriveMeshIPWithCollisionCheck` (`pkg/daemon/collision.go:171`) |
| `pkg/node/admit.go` | edit | `EnforceIPBinding` variable (false) |
| `pkg/daemon/daemon.go` | edit | build the `Policy` (`Local` closure over `localNode`, `Derive` closure, `OnWarn`) and call `peerStore.SetPolicy` before discovery starts. `Derive(key, 0)` is the natural derivation (`crypto.DeriveMeshIP`, or `DeriveMeshIPInSubnet` with a custom subnet) and `Derive(key, n)` for n of 1 or more is `DeriveMeshIPWithNonce` (or `DeriveMeshIPInSubnetWithNonce`): the two families hash different inputs (`derive.go:155-156` against `collision.go:75-76`) |
| `pkg/daemon/metrics.go` | edit | counter `wgmesh_admit_warnings_total{rule}` and the function `OnWarn` calls |
| `pkg/daemon/collision.go` | edit | `DetectCollisions(ps, local)` delegates to `node.ResolveClaims` and includes the local node; `DeterministicWinner` delegates to `node.Winner`; `CheckAndResolveCollisions` builds the local claim and passes it |
| `pkg/node/claims_test.go`, `pkg/daemon/collision_test.go`, `pkg/daemon/daemon_test.go`, `pkg/daemon/metrics_test.go` | add / edit | tests below; the callers of `DetectCollisions` at `collision_test.go:12`, `:20`, `:27` take the new `local` argument |

Selection: `CheckAndResolveCollisions` is called from `reconcile` (`pkg/daemon/daemon.go:544`); deleting the local-claim argument turns `TestLocalNodeLosesCollisionAgainstRemote` red.

## Ordered Steps

1. [S1] Add the six tests below with compiling stubs (`ResolveClaims` ignores the local claim, `Winner` is the lower key) and run them red; `TestLocalNodeLosesCollisionAgainstRemote` reproduces #856 (the detector returns no collision for local versus remote).
2. [S2] Implement `Bound` (the claim's IP equals `derive(key, n)` for some `n` in 0 to 10; a nil `derive` means nothing is bound) and `Winner` (bound over unbound, then the lower key; when `EnforceIPBinding` is false return the lower key and call `warn("winner-differs")` if the new order would choose the other; this is the only place that rule is emitted).
3. [S3] Implement `ResolveClaims` with the local claim as one more member; keep the output deterministic (sorted by IP, then key) so every observer computes the same list.
4. [S4] Build the policy and the counter in the daemon and call `SetPolicy`; change `collision.go` to use `ResolveClaims` and `Winner`; leave `setInterfaceAddress` and the nonce-1 derivation where they are (T5 persists the result).
5. [S5] Run the mutants with `adr-verify --mutant`: the local claim is dropped from `ResolveClaims`; `Winner` ignores `EnforceIPBinding`; `Bound` tries only nonce 0. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/node -run '^(TestClaimOrderBoundBeatsUnbound|TestClaimOrderFallsToLowerKey|TestResolveClaimsIncludesLocal|TestBoundAcceptsNonceOneToTen)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 4 ] \
 && b=$(go test ./pkg/daemon -run '^(TestLocalNodeLosesCollisionAgainstRemote|TestLegacyWinnerAtReleaseN|TestMixedWinnerRulesConverge|TestMixedWinnerNeitherMovesIsNotWorse|TestDaemonInstallsAdmitPolicy|TestPolicyDeriveZeroIsNaturalDerivation|TestAdmitWarningCounterCounts)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -ge 7 ] \
 && go test -race ./pkg/node ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestClaimOrderBoundBeatsUnbound` | `pkg/node/claims_test.go` | with `EnforceIPBinding` true, the claim equal to its own derived IP beats a rotated-address claim even when its key is higher | — | S1, S2 |
| `TestClaimOrderFallsToLowerKey` | `pkg/node/claims_test.go` | two bound or two unbound claims fall to the lower key, in both argument orders | — | S1, S2 |
| `TestResolveClaimsIncludesLocal` | `pkg/node/claims_test.go` | a remote peer with the local IP and another key yields one collision naming the local key | — | S1, S3 |
| `TestBoundAcceptsNonceOneToTen` | `pkg/node/claims_test.go` | an IP derived with nonce 1 and with nonce 10 is bound, nonce 11 is not | — | S1, S2 |
| `TestLocalNodeLosesCollisionAgainstRemote` | `pkg/daemon/collision_test.go` | the daemon re-derives the local IP when the local key is the loser against a remote peer in the store (red today) | — | S1, S4 |
| `TestLegacyWinnerAtReleaseN` | `pkg/daemon/collision_test.go` | with `EnforceIPBinding` false the lower key moves, and a `winner-differs` warning is counted when the bound order would choose the other | — | S1, S2, S4 |
| `TestMixedWinnerRulesConverge` | `pkg/daemon/collision_test.go` | an old-order node with the higher key and a bound address against a bound-order node with the lower key and an unbound address: both re-derive and end on different addresses; two claims of one class make the rules agree and only the higher key moves | — | S1, S4 |
| `TestMixedWinnerNeitherMovesIsNotWorse` | `pkg/daemon/collision_test.go` | old node with the lower key and an unbound address against a new node with a bound address: neither moves, which is what main does today where the local node is not in the table (#856), and the old node counts one warning | — | S1, S4 |
| `TestDaemonInstallsAdmitPolicy` | `pkg/daemon/daemon_test.go` | after daemon construction the store has a policy whose `Local` returns the local key and IP | — | S1, S4 |
| `TestPolicyDeriveZeroIsNaturalDerivation` | `pkg/daemon/daemon_test.go` | `Derive(key, 0)` equals `crypto.DeriveMeshIP` (and the custom-subnet form), `Derive(key, 1)` equals `DeriveMeshIPWithNonce(.., 1)` | — | S1, S4 |
| `TestAdmitWarningCounterCounts` | `pkg/daemon/metrics_test.go` | `OnWarn("winner-differs")` increments `wgmesh_admit_warnings_total{rule="winner-differs"}` | — | S1, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the seven tests |
| 2 — something selects it | `reconcile` calls `CheckAndResolveCollisions`, which calls `ResolveClaims`; dropping the local claim turns `TestLocalNodeLosesCollisionAgainstRemote` red |
| 3 — the caller can discover it | doc comments; T11 amends the daemon support spec |
| 4 — it is used | the `[Collision]` log lines and the `winner-differs` counter registered by T4 in `RegisterMetrics` (T6 registers nothing) |

## Mutation Log
- 2026-11-15 · 45f695e* · mutant killed · exit 1 · `pkg/node/claims.go` · the local claim is dropped from ResolveClaims: TestResolveClaimsIncludesLocal and TestLocalNodeLosesCollisionAgainstRemote must go red · acceptance-sha256:a2bdff9ac41cd2fc11ab39010b55340f6a9092bd84464626ef3a95c3b132b837
- 2026-11-15 · 45f695e* · mutant killed · exit 1 · `pkg/node/claims.go` · Winner ignores EnforceIPBinding: TestClaimOrderBoundBeatsUnbound must go red · acceptance-sha256:a2bdff9ac41cd2fc11ab39010b55340f6a9092bd84464626ef3a95c3b132b837
- 2026-11-15 · 45f695e* · mutant killed · exit 1 · `pkg/node/claims.go` · Bound tries only nonce 0: TestBoundAcceptsNonceOneToTen must go red · acceptance-sha256:a2bdff9ac41cd2fc11ab39010b55340f6a9092bd84464626ef3a95c3b132b837

## Invariants

- At release N the node that moves is the one the old rule chose (the lower key stays, the higher moves), so a mixed mesh does not change behaviour.
- `ResolveClaims` is deterministic: the same peers give the same list on every node.
- `pkg/node` holds no secret: derivation arrives as a closure from the daemon.

## Risks

- The unsynchronised write of `LocalNode.MeshIP` (`collision.go:120`) against reads in discovery stays; the claim is read through the `Policy.Local` closure at decision time. Out of scope here (see the ADR).
- Test doubles: the daemon tests use a fake `setInterfaceAddress` seam the existing collision tests already use; if none exists the first step adds one function variable and says so in the commit.

## Stop Condition

Stop and ask if the daemon cannot build a local claim without a lock the daemon does not have, or if `TestMixedWinnerRulesConverge` shows two nodes that keep colliding.

## Out of Scope

- Persisting the resolved address (T5).
- Refusing a claim (T6): this task orders claims, it does not refuse them.

## Verification Log
- 2026-11-15 · 45f695e* · exit 1 · `set -o pipefail …` · acceptance-sha256:a2bdff9ac41cd2fc11ab39010b55340f6a9092bd84464626ef3a95c3b132b837 · ms:946 · test-lock-sha256:993eb20d5df4fefcad4448536e2a595c404bebcc15c78b94cff8fe8dc799eb16 · test-lock-file:docs/adr/locks/993eb20d5df4fefcad4448536e2a595c404bebcc15c78b94cff8fe8dc799eb16.lock
  ```
  --- last 10 line(s) of stderr (of 35 after folding 35 raw)
      claims_test.go:141: an address derived with nonce 5 is not bound
      claims_test.go:141: an address derived with nonce 6 is not bound
      claims_test.go:141: an address derived with nonce 7 is not bound
      claims_test.go:141: an address derived with nonce 8 is not bound
      claims_test.go:141: an address derived with nonce 9 is not bound
      claims_test.go:141: an address derived with nonce 10 is not bound
  --- FAIL: TestBoundAcceptsNonceOneToTen (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/node	0.004s
  FAIL
  ```
- 2026-11-15 · 45f695e* · exit 0 · `set -o pipefail …` · acceptance-sha256:a2bdff9ac41cd2fc11ab39010b55340f6a9092bd84464626ef3a95c3b132b837 · ms:8695
- 2026-11-15 · 45f695e* · exit 0 · `set -o pipefail …` · acceptance-sha256:a2bdff9ac41cd2fc11ab39010b55340f6a9092bd84464626ef3a95c3b132b837 · ms:9592
- 2026-11-15 · 45f695e* · exit 0 · `set -o pipefail …` · acceptance-sha256:a2bdff9ac41cd2fc11ab39010b55340f6a9092bd84464626ef3a95c3b132b837 · ms:8206
- 2026-11-15 · 45f695e* · exit 0 · `set -o pipefail …` · acceptance-sha256:a2bdff9ac41cd2fc11ab39010b55340f6a9092bd84464626ef3a95c3b132b837 · ms:5663
