# Task ADR-0003-T1: Make routes canonical per family and give each shared network one owner

**Depends-on:** none
**Blocked-on:** ADR-0002's route approval task has landed on main: `grep -q 'func Approved' pkg/node/binding.go` exits 0 (its fence asserts an empty diff of `pkg/routes` at its own commit) Landed on main at `f3860c6` (2026-11-16): the command exits 0.
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `routes.Canonical`, `routes.Advertiser`, `routes.Resolve`, a deterministic `routes.CalculateDiff`
**Consumes:** none
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The pure route code treats `default` and `0.0.0.0/0` as one network per family, a network advertised by several peers has exactly one owner chosen the same way on every call, and `CalculateDiff` no longer alternates when handed two entries for one network.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/routes/routes.go` | edit | export `Canonical(network string, ipv6 bool) string` (today's unexported `canonicalNetwork`, with `default` mapped per family); add `Advertiser{Key, MeshIP, MeshIPv6 string; Fresh bool; Networks []string}` and `Resolve(adv []Advertiser, own []string, current map[string]string) map[string]string` (canonical network to owner key; applies `ExcludeOwn`, trims blanks); `CalculateDiff` compares canonical networks and, for two desired entries on one network, keeps the one whose gateway is numerically lowest |
| `pkg/routes/routes_test.go` | edit | the eight tests below; `TestExcludeOwn`, `TestCalculateDiff_*` stay and must pass unchanged |
| `pkg/ssh/routes_test.go`, `pkg/mesh/deploy_test.go` | none | run unchanged as the proof that the two callers of `CalculateDiff` keep their behaviour |

Selection: nothing selects `Resolve` until T2; `Canonical` is called by `ExcludeOwn` and `CalculateDiff` from this task on, and deleting that call turns `TestCalculateDiffDefaultNoChurn` red.

## Ordered Steps

1. [S1] Add the eight tests with compiling stubs (`Canonical` returns its input, `Resolve` returns an empty map, `CalculateDiff` unchanged) so the first run fails on assertions, not on the build. `TestCalculateDiffDuplicateDesiredStable` feeds the result of cycle 1 back as `current` and asserts cycle 2 is empty (the 2026-11-15 scratch run showed `del .3 / replace .2` then `del .2 / replace .3`). Run red.
2. [S2] Implement `Canonical` and make `ExcludeOwn` call it; `default` is `0.0.0.0/0` for `ipv6=false` and `::/0` for `ipv6=true`.
3. [S3] Make `CalculateDiff` canonical and deterministic for duplicates.
4. [S4] Implement `Resolve`: candidates per canonical network after `ExcludeOwn`; fresh before not fresh; a candidate that already owns the network in `current` and is fresh keeps it; otherwise the lowest mesh IP compared as an address, then the lowest key (decision 4, recommendation A).
5. [S5] Run `go test ./pkg/ssh ./pkg/mesh` without editing them. [proof: acceptance]
6. [S6] Run the mutants with `adr-verify --mutant`: `Resolve` compares mesh IPs as strings (`10.99.0.10` before `10.99.0.9`); `Resolve` ignores `Fresh`; `Resolve` ignores `current`; `CalculateDiff` keeps the last duplicate. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/routes -run '^(TestCanonicalDefaultPerFamily|TestResolveSharedSubnetOneOwner|TestResolveKeepsFreshObservedOwner|TestResolveFailoverWhenOwnerStale|TestResolveAllStaleLowestIPNumeric|TestResolveDropsOwnNetworkAndHADefault|TestCalculateDiffDefaultNoChurn|TestCalculateDiffDuplicateDesiredStable)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 8 ] && go test -race ./pkg/routes ./pkg/ssh ./pkg/mesh ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestCanonicalDefaultPerFamily` | `pkg/routes/routes_test.go` | `default` is `0.0.0.0/0` for IPv4 and `::/0` for IPv6, bare hosts get `/32` and `/128`, a host-bit CIDR is masked | — | S1, S2 |
| `TestResolveSharedSubnetOneOwner` | `pkg/routes/routes_test.go` | two fresh peers advertising one network give one owner, the same for every permutation of the input order | — | S1, S4 |
| `TestResolveKeepsFreshObservedOwner` | `pkg/routes/routes_test.go` | a fresh owner that is not the lowest mesh IP keeps the network when `current` names it | — | S1, S4 |
| `TestResolveFailoverWhenOwnerStale` | `pkg/routes/routes_test.go` | when the current owner is not fresh and another candidate is, the fresh one owns the network | — | S1, S4 |
| `TestResolveAllStaleLowestIPNumeric` | `pkg/routes/routes_test.go` | with no fresh candidate the lowest mesh IP as a number wins (`10.99.0.9` over `10.99.0.10`), ties fall to the lowest key | — | S1, S4 |
| `TestResolveDropsOwnNetworkAndHADefault` | `pkg/routes/routes_test.go` | a network the node advertises is never owned by a peer, including `0.0.0.0/0` advertised by both HA exit nodes | — | S1, S4 |
| `TestCalculateDiffDefaultNoChurn` | `pkg/routes/routes_test.go` | current `default via G` and desired `0.0.0.0/0 via G` give an empty diff | — | S1, S3 |
| `TestCalculateDiffDuplicateDesiredStable` | `pkg/routes/routes_test.go` | two desired entries on one network produce one add on cycle 1 and nothing on cycle 2 | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the eight tests |
| 2 — something selects it | `ExcludeOwn` and `CalculateDiff` call `Canonical`; `Resolve` is selected by `desire` in T2 (its `TestDesireRoutesAndAllowedIPsFromOneOwner` turns red if the call is deleted) |
| 3 — the caller can discover it | exported with doc comments; T9 amends the specs |
| 4 — it is used | `reconcile` through T2 |

## Mutation Log
- 2026-11-16 · 7de5aff* · mutant killed · exit 1 · `pkg/routes/routes.go` · string compare of mesh IPs: TestResolveAllStaleLowestIPNumeric must go red · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2
- 2026-11-16 · 7de5aff* · mutant killed · exit 1 · `pkg/routes/routes.go` · Fresh ignored: TestResolveFailoverWhenOwnerStale must go red · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2
- 2026-11-16 · 7de5aff* · mutant killed · exit 1 · `pkg/routes/routes.go` · observed owner ignored: TestResolveKeepsFreshObservedOwner must go red · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2
- 2026-11-16 · 7de5aff* · mutant killed · exit 1 · `pkg/routes/routes.go` · last duplicate wins: TestCalculateDiffDuplicateDesiredStable must go red · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2
- 2026-11-16 · 7de5aff* · mutant killed · exit 1 · `pkg/routes/routes.go` · default kept as a word: TestCanonicalDefaultPerFamily and TestCalculateDiffDefaultNoChurn must go red · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2
- 2026-11-16 · 57a41f0* · mutant killed · exit 1 · `pkg/routes/routes.go` · string compare of mesh IPs: TestResolveAllStaleLowestIPNumeric must go red · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2
- 2026-11-16 · 57a41f0* · mutant killed · exit 1 · `pkg/routes/routes.go` · Fresh ignored: TestResolveFailoverWhenOwnerStale must go red · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2
- 2026-11-16 · 57a41f0* · mutant killed · exit 1 · `pkg/routes/routes.go` · observed owner ignored: TestResolveKeepsFreshObservedOwner must go red · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2
- 2026-11-16 · 57a41f0* · mutant killed · exit 1 · `pkg/routes/routes.go` · last duplicate wins: TestCalculateDiffDuplicateDesiredStable must go red · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2
- 2026-11-16 · 57a41f0* · mutant killed · exit 1 · `pkg/routes/routes.go` · default kept as a word: TestCanonicalDefaultPerFamily and TestCalculateDiffDefaultNoChurn must go red · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2

## Invariants

- Every existing test in `pkg/routes`, `pkg/ssh` and `pkg/mesh` passes unchanged; the signatures of `CalculateDiff`, `ExcludeOwn`, `NormalizeNetwork`, `MakeKey` do not change.
- `ExcludeOwn` still drops a peer's `0.0.0.0/0` on a node that advertises it (ADR-0002 decision item 4a (e)).
- `pkg/routes` imports nothing from `pkg/daemon`, `pkg/ssh` or `pkg/mesh`.

## Risks

- `CalculateDiff` changing for duplicate input alters what the centralized deploy does when two nodes advertise one network (it now picks the lowest gateway instead of alternating); that pipeline's draft (`task/adr-0006-central-deploy`, not on main) must be told; the `pkg/ssh` and `pkg/mesh` tests are the proof here.
- A string compare of mesh IPs would pass most tests and fail at `.9` against `.10`; the mutant above covers it.
- Test doubles: none; the tests call the pure functions.

## Stop Condition

Stop and ask if an existing test in `pkg/ssh` or `pkg/mesh` needs a changed expectation, or if ADR-0002's route approval task has not landed (its fence requires `pkg/routes` untouched at its commit).

## Out of Scope

- Building the desired state and calling `Resolve` (T2).
- Any change to `pkg/ssh` or `pkg/mesh` code (permanent: boundary: the centralized pipeline belongs to another group).

## Verification Log
- 2026-11-16 · 7de5aff* · exit 1 · `set -o pipefail …` · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2 · ms:688 · test-lock-sha256:2785b39506e9c89686241d2712dd1c1d04091b67d2ed426b902a3c007c9f4a54 · test-lock-file:docs/adr/locks/2785b39506e9c89686241d2712dd1c1d04091b67d2ed426b902a3c007c9f4a54.lock
  ```
  --- last 10 line(s) of stderr (of 58 after folding 58 raw)
  --- FAIL: TestResolveKeepsFreshObservedOwner (0.00s)
  === CONT  TestResolveDropsOwnNetworkAndHADefault
      routes_test.go:443: Resolve = map[], want only {10.0.0.0/8: a}
  --- FAIL: TestResolveDropsOwnNetworkAndHADefault (0.00s)
  === CONT  TestCalculateDiffDuplicateDesiredStable
      routes_test.go:468: cycle 1 add = [{192.168.50.0/24 10.99.0.10} {192.168.50.0/24 10.99.0.9}], want one add via 10.99.0.9
  --- FAIL: TestCalculateDiffDuplicateDesiredStable (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/routes	0.003s
  FAIL
  ```
- 2026-11-16 · 7de5aff* · exit 0 · `set -o pipefail …` · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2 · ms:6843
- 2026-11-16 · 7de5aff* · exit 0 · `set -o pipefail …` · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2 · ms:6296
- 2026-11-16 · 7de5aff* · exit 0 · `set -o pipefail …` · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2 · ms:5610
- 2026-11-16 · 7de5aff* · exit 0 · `set -o pipefail …` · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2 · ms:5945
- 2026-11-16 · 7de5aff* · exit 0 · `set -o pipefail …` · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2 · ms:6456
- 2026-11-16 · 7de5aff* · exit 0 · `set -o pipefail …` · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2 · ms:5597
- 2026-11-16 · 57a41f0* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2 · ms:0 · test-lock-sha256:7741b1f0681dc986acdffbfb4a6ccca324ed23bc2c4098e2fd5481e50b806f25 · test-lock-file:docs/adr/locks/7741b1f0681dc986acdffbfb4a6ccca324ed23bc2c4098e2fd5481e50b806f25.lock · test-lock-kind:replace
- 2026-11-16 · 57a41f0* · exit 0 · `set -o pipefail …` · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2 · ms:7023
- 2026-11-16 · 57a41f0* · exit 0 · `set -o pipefail …` · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2 · ms:5340
- 2026-11-16 · 57a41f0* · exit 0 · `set -o pipefail …` · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2 · ms:5371
- 2026-11-16 · 57a41f0* · exit 0 · `set -o pipefail …` · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2 · ms:6274
- 2026-11-16 · 57a41f0* · exit 0 · `set -o pipefail …` · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2 · ms:6200
- 2026-11-16 · 57a41f0* · exit 0 · `set -o pipefail …` · acceptance-sha256:7f886b4aa8cd7891ac3c9d97080b37542b53f52ec01f01a3009b0ca6a5f445e2 · ms:4589
