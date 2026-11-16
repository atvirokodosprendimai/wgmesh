# Task ADR-0003-T7: Converge the interface address instead of writing it once

**Depends-on:** T3
**Blocked-on:** ADR-0002's persist-the-resolved-IP task has landed on main: `grep -q 'saveLocalNode' pkg/daemon/collision.go` exits 0 (exits 1 at `b8b83f3`) Landed on main at `f3860c6` (2026-11-16): the command exits 0.
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `addressTarget`
**Consumes:** `kernelObserved`, `planKernel`, `applyKernel`, the guard test (T3); the collision-resolved, persisted `localNode.MeshIP` (ADR-0002's persist-the-resolved-IP task)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

Each tick compares the addresses on the mesh interface with the local node's mesh IP and mesh IPv6, adds a missing one, never flushes the family, and finds a stale one inside the mesh network (removed behind `EnforceAddressRemoval`, `false` at N: at N one log line and `wgmesh_kernel_warnings_total{rule="stale-address"}`, no delete; removals run before adds because of how the kernel treats a same-address add and a deleted primary), so an address lost to an external change, or left behind by a collision-resolved IP, converges without touching an address an operator added.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/observe.go` | edit | Linux only: `ip -o addr show dev <if>` (both families); Darwin is out of scope, as for forwarding |
| `pkg/daemon/desired.go` | edit | `desiredState.Addresses`: `<MeshIP>/<prefix>` and `<MeshIPv6>/64` |
| `pkg/daemon/converge.go` | edit | plan: add a missing desired address (`ip addr add`), remove an address inside the mesh network that is not desired (`ip addr del`), leave others; apply with a result per item |
| `pkg/daemon/converge_guard_test.go` | edit | `ip addr add` and `ip addr del` outside `converge.go` and the setup helper `setInterfaceAddress` (also reached through the `applyInterfaceAddress` variable of `collision.go`) fail the guard |
| `pkg/daemon/address_test.go` | add | the five tests below |
| `pkg/daemon/collision.go`, `pkg/daemon/helpers.go` | none | `collision.go` is governed by ADR-0002 and is not edited; its call at `:123` becomes a no-op beside this target and a follow-up row removes it; `setInterfaceAddress` keeps its flush for first setup |

Selection: the address target is part of every plan; deleting it turns `TestMissingAddressReadded` red.

## Ordered Steps

1. [S1] Add the five tests with compiling stubs and run red.
2. [S2] Implement the observe, the plan and the apply for Linux; on Darwin the target is not wanted (T6 states it).
3. [S3] Extend `TestKernelWritesOnlyInConverge`. [proof: acceptance]
4. [S4] Run the mutants with `adr-verify --mutant`: the plan flushes the family before adding; any address not desired is removed; the comparison ignores the prefix length. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/daemon -run '^(TestObserveAddresses|TestMissingAddressReadded|TestStaleMeshAddressRemovedOtherAddressKept|TestAddressSteadyStateQuiet|TestAddressConvergesAfterCollisionIPChange)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 5 ] && go test -race ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestObserveAddresses` | `pkg/daemon/address_test.go` | `ip -o addr show dev wg0` output for IPv4 and IPv6 parses to prefixes | — | S1, S2 |
| `TestMissingAddressReadded` | `pkg/daemon/address_test.go` | with the address gone from the fake interface the next tick issues one `ip addr add` | — | S1, S2 |
| `TestStaleMeshAddressRemovedOtherAddressKept` | `pkg/daemon/address_test.go` | an old mesh address inside the mesh network is deleted, an address outside it is not touched, and no `flush` is issued | — | S1, S2 |
| `TestAddressSteadyStateQuiet` | `pkg/daemon/address_test.go` | five ticks with the correct address issue no write | — | S1, S2 |
| `TestAddressConvergesAfterCollisionIPChange` | `pkg/daemon/address_test.go` | after `localNode.MeshIP` changes the next tick adds the new address and deletes the old one | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the five tests |
| 2 — something selects it | the address target is in every plan; the guard test fails when `ip addr` is run elsewhere |
| 3 — the caller can discover it | `wgmesh_kernel_changes_total{target="address"}`; T9 documents it |
| 4 — it is used | every tick |

## Mutation Log
- 2026-11-16 · c02ccf6* · mutant killed · exit 1 · `pkg/daemon/converge.go` · flush before add: TestStaleMeshAddressRemovedOtherAddressKept must go red · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7
- 2026-11-16 · c02ccf6* · mutant killed · exit 1 · `pkg/daemon/converge.go` · operator address outside the mesh removed: TestStaleMeshAddressRemovedOtherAddressKept must go red · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7
- 2026-11-16 · c02ccf6* · mutant killed · exit 1 · `pkg/daemon/converge.go` · prefix length ignored: an address held with another prefix length is taken as present; the address tests must go red · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7
- 2026-11-16 · f110717* · mutant killed · exit 1 · `pkg/daemon/converge.go` · flush before add: TestStaleMeshAddressRemovedOtherAddressKept must go red · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7
- 2026-11-16 · f110717* · mutant killed · exit 1 · `pkg/daemon/converge.go` · operator address outside the mesh removed: TestStaleMeshAddressRemovedOtherAddressKept must go red · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7
- 2026-11-16 · f110717* · mutant killed · exit 1 · `pkg/daemon/converge.go` · prefix length ignored: an address held with another prefix length is taken as present; the address tests must go red · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7
- 2026-11-16 · 6b0f7e1* · mutant killed · exit 1 · `pkg/daemon/converge.go` · flush before add: TestStaleMeshAddressRemovedOtherAddressKept must go red · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7
- 2026-11-16 · 6b0f7e1* · mutant killed · exit 1 · `pkg/daemon/converge.go` · operator address outside the mesh removed: TestStaleMeshAddressRemovedOtherAddressKept must go red · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7
- 2026-11-16 · 6b0f7e1* · mutant killed · exit 1 · `pkg/daemon/converge.go` · prefix length ignored: an address held with another prefix length is taken as present; the address tests must go red · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7

## Invariants

- First setup still runs `setInterfaceAddress` (with its flush) before the interface is up.
- A node whose address is correct issues no address write.
- The mesh IPv6 `/64` and IPv4 prefix come from `d.config.PrefixLen()` as today.

## Risks

- Two writers for a collision-resolved address (`applyInterfaceAddress` at `collision.go:142`, a variable holding `setInterfaceAddress`, and this target) until the follow-up row is done; the second finds nothing to change.
- `localNode.MeshIP` is written bare by collision resolution (`collision.go`, `d.localNode.MeshIP = newIP`) and read by other goroutines; with `reconcile` serialised (T2 S7) the target reads it only on the reconcile path, where the writer also runs; the remaining bare accesses belong to the atomic-state draft (permanent: boundary).
- Test doubles: `MockCommandExecutor` with a fake address table.

## Stop Condition

Stop and ask if ADR-0002's persist-IP task has not landed, or if the interface carries an address inside the mesh network that an operator added on purpose (the plan would delete it; the answer is a rule, not a flag).

## Out of Scope

- Removing the call at `collision.go:123` (deferred: docs/adr/BACKLOG.md).
- IPv6 address management when IPv6 is disabled (permanent: boundary: `setupWireGuard` already falls back to IPv4-only, `daemon.go:486-492`).

## Verification Log
- 2026-11-16 · c02ccf6* · exit 1 · `set -o pipefail …` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:1956 · test-lock-sha256:bda1704bc8e8d685695c5bb196c1719d9ec413a47a46339f8ad042fdf732b29c · test-lock-file:docs/adr/locks/bda1704bc8e8d685695c5bb196c1719d9ec413a47a46339f8ad042fdf732b29c.lock
  ```
  --- last 10 line(s) of stderr (of 23 after folding 23 raw)
  --- FAIL: TestStaleMeshAddressRemovedOtherAddressKept (0.00s)
  === RUN   TestAddressSteadyStateQuiet
  --- PASS: TestAddressSteadyStateQuiet (0.00s)
  === RUN   TestAddressConvergesAfterCollisionIPChange
      address_test.go:266: address commands [], want ["ip addr add 10.99.4.4/16 dev wg0" "ip addr del 10.99.0.1/16 dev wg0"] (the new address first, then the old one)
      address_test.go:269: interface holds ["10.99.0.1/16"]
  --- FAIL: TestAddressConvergesAfterCollisionIPChange (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/daemon	0.006s
  FAIL
  ```
- 2026-11-16 · c02ccf6* · exit 0 · `set -o pipefail …` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:10123
- 2026-11-16 · c02ccf6* · exit 0 · `set -o pipefail …` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:10853
- 2026-11-16 · c02ccf6* · exit 0 · `set -o pipefail …` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:7095
- 2026-11-16 · c02ccf6* · exit 0 · `set -o pipefail …` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:11696
- 2026-11-16 · f110717* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:0 · test-lock-sha256:e6a4cb9baa42b1704480c6a5aba40079ba549240029a5c314b7458d62a8872f8 · test-lock-file:docs/adr/locks/e6a4cb9baa42b1704480c6a5aba40079ba549240029a5c314b7458d62a8872f8.lock · test-lock-kind:replace
- 2026-11-16 · f110717* · exit 0 · `set -o pipefail …` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:10874
- 2026-11-16 · f110717* · exit 0 · `set -o pipefail …` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:13516
- 2026-11-16 · f110717* · exit 0 · `set -o pipefail …` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:12246
- 2026-11-16 · f110717* · exit 0 · `set -o pipefail …` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:7680
- 2026-11-16 · fe6f879* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:2 · test-lock-sha256:e6a4cb9baa42b1704480c6a5aba40079ba549240029a5c314b7458d62a8872f8 · test-lock-file:docs/adr/locks/e6a4cb9baa42b1704480c6a5aba40079ba549240029a5c314b7458d62a8872f8.lock · test-lock-kind:replace
- 2026-11-16 · 6b0f7e1* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:0 · test-lock-sha256:ee6a24b86e01219b031753132338280a703b931f99f5987ac6e66d763a378bf4 · test-lock-file:docs/adr/locks/ee6a24b86e01219b031753132338280a703b931f99f5987ac6e66d763a378bf4.lock · test-lock-kind:replace
- 2026-11-16 · 6b0f7e1* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:0 · test-lock-sha256:ee6a24b86e01219b031753132338280a703b931f99f5987ac6e66d763a378bf4 · test-lock-file:docs/adr/locks/ee6a24b86e01219b031753132338280a703b931f99f5987ac6e66d763a378bf4.lock · test-lock-kind:replace
- 2026-11-16 · 6b0f7e1* · exit 0 · `set -o pipefail …` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:10966
- 2026-11-16 · 6b0f7e1* · exit 0 · `set -o pipefail …` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:10484
- 2026-11-16 · 6b0f7e1* · exit 0 · `set -o pipefail …` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:6907
- 2026-11-16 · 6b0f7e1* · exit 0 · `set -o pipefail …` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:10656
- 2026-11-16 · 0f45e87* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:2 · test-lock-sha256:ee6a24b86e01219b031753132338280a703b931f99f5987ac6e66d763a378bf4 · test-lock-file:docs/adr/locks/ee6a24b86e01219b031753132338280a703b931f99f5987ac6e66d763a378bf4.lock · test-lock-kind:replace
- 2026-11-16 · 7c1d459* · exit 0 · `set -o pipefail …` · acceptance-sha256:8ca663c2c3ca84e335fb1a00e752a394d77fe661d3aaedd7c65a4c9c6614b3f7 · ms:7454
