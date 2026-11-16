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

Each tick compares the addresses on the mesh interface with the local node's mesh IP and mesh IPv6, adds a missing one, removes a stale one inside the mesh network, and never flushes the family, so an address lost to an external change, or left behind by a collision-resolved IP, converges without touching an address an operator added.

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
