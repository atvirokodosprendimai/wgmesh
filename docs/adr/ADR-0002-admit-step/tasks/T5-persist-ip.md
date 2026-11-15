# Task ADR-0002-T5: Persist the collision-resolved mesh IP (#830)

**Depends-on:** T4
**Covers:** none — no spec
**Estimated scope:** S (single file)
**Owner:** unassigned
**Produces:** the resolved mesh IP in the local node state file
**Consumes:** `ResolveClaims`, the collision path in `CheckAndResolveCollisions` (T4)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

When this node loses a collision and takes a nonce-derived address, that address is written to the local node state file, survives a restart, and is the address `service add` reads.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/daemon.go` | edit | a `stateFile` field on `Daemon`, set where `initLocalNode` builds it (`:353`) so tests can point it at a temp directory |
| `pkg/daemon/collision.go` | edit | after `d.localNode.MeshIP = newIP` (`:120`) call `saveLocalNode(d.stateFile, d.localNode)`; a failure logs and keeps the in-memory address (as the derivation paths do, `daemon.go:376`) |
| `pkg/daemon/mesh_ip_secret_change_test.go` | edit | the restart test beside the existing secret-rotation one |
| `pkg/daemon/collision_test.go` | edit | the persistence test |
| `service_test.go` (package `main`) | edit | `TestServiceAddReadsResolvedIP`; `service add` reads the address through `deriveMeshIPForService` (`service.go:455`), which already returns a persisted in-subnet address (`:471-474`) |

Selection: `initLocalNode` already keeps a persisted address that lies in the subnet (`daemon.go:358-364`, `meshIPInSubnet`), so persisting is the whole change; deleting the new `saveLocalNode` call turns `TestCollisionLoserPersistsResolvedIP` red.

## Ordered Steps

1. [S1] Add `TestCollisionLoserPersistsResolvedIP`, `TestRestartKeepsResolvedIP` and `TestServiceAddReadsResolvedIP`. The third test writes a state file the way `saveLocalNode` does after a collision and calls `deriveMeshIPForService` on it; it starts green (pin) because that function already honours a persisted address, and fails if the persisted field is not what T5 writes. Run the first two red.
2. [S2] Add the `stateFile` field and the `saveLocalNode` call.
3. [S3] Run the mutants with `adr-verify --mutant`: the save call removed; the save writes the pre-collision IP. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/daemon . -run '^(TestCollisionLoserPersistsResolvedIP|TestRestartKeepsResolvedIP|TestServiceAddReadsResolvedIP)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 3 ] && go test -race ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestCollisionLoserPersistsResolvedIP` | `pkg/daemon/collision_test.go` | after the local node loses, the state file holds the nonce-derived IP, not the natural one | — | S1, S2 |
| `TestRestartKeepsResolvedIP` | `pkg/daemon/mesh_ip_secret_change_test.go` | `initLocalNode` on that file keeps the resolved address | — | S1, S2 |
| `TestServiceAddReadsResolvedIP` | `service_test.go` | `deriveMeshIPForService` returns the persisted resolved IP | — | S1, S2 |
| `TestCollisionKeepsAddressWhenSaveFails` | `pkg/daemon/collision_test.go` | when the state file cannot be written the in-memory address and the interface address still move to the resolved IP (added after review) | — | S1 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the three tests |
| 2 — something selects it | `CheckAndResolveCollisions` runs from `reconcile`; deleting the save turns the first test red |
| 3 — the caller can discover it | doc comment on `CheckAndResolveCollisions`; T11 amends the daemon support spec |
| 4 — it is used | the state file on a node that lost a collision |

## Mutation Log
- 2026-11-15 · d7d842c* · mutant killed · exit 1 · `pkg/daemon/collision.go` · the save call is removed: TestCollisionLoserPersistsResolvedIP must go red · acceptance-sha256:250e44cb94620947c4f3a99c3640da7c22c760b326cb3ffc62297cd7b2e10722
- 2026-11-15 · d7d842c* · mutant killed · exit 1 · `pkg/daemon/collision.go` · the save writes the pre-collision IP: TestCollisionLoserPersistsResolvedIP and TestRestartKeepsResolvedIP must go red · acceptance-sha256:250e44cb94620947c4f3a99c3640da7c22c760b326cb3ffc62297cd7b2e10722
- 2026-11-15 · 4d65d36* · mutant killed · exit 1 · `pkg/daemon/collision.go` · round 2: the save call is removed · acceptance-sha256:250e44cb94620947c4f3a99c3640da7c22c760b326cb3ffc62297cd7b2e10722
- 2026-11-15 · 4d65d36* · mutant killed · exit 1 · `pkg/daemon/collision.go` · round 2: the save writes the pre-collision IP · acceptance-sha256:250e44cb94620947c4f3a99c3640da7c22c760b326cb3ffc62297cd7b2e10722
- 2026-11-15 · 4d65d36* · mutant killed · exit 1 · `pkg/daemon/collision.go` · round 2: a failed save puts the old address back in memory: TestCollisionKeepsAddressWhenSaveFails must go red · acceptance-sha256:250e44cb94620947c4f3a99c3640da7c22c760b326cb3ffc62297cd7b2e10722
- 2026-11-15 · 4d65d36* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · round 2: initLocalNode leaves d.stateFile empty: TestRestartKeepsResolvedIP must go red · acceptance-sha256:250e44cb94620947c4f3a99c3640da7c22c760b326cb3ffc62297cd7b2e10722

## Invariants

- The state file keeps its present fields and format; an older release reading it keeps the address if it lies in the subnet.
- The IP is persisted, not the nonce; no new field is added.
- The loser keeps the address after the other peer leaves (stickiness; ADR decision 3).

## Risks

- `saveLocalNode` is not atomic (#870); a crash during the write can lose the file. Owned by the atomic-load group; this task uses the existing function.
- Test doubles: a temp-dir state file and the interface-address seam from T4.

## Stop Condition

Stop and ask if `deriveMeshIPForService` stops honouring a persisted address, or if persisting makes `initLocalNode` reject the address.

## Out of Scope

- Letting a loser reclaim its natural address (ADR decision 3; permanent: boundary: stability of an address over convergence to the natural one, the invariant of #827).
- Atomic state file writes (permanent: boundary: #870, another group).
- `service add` for a daemon with another interface name: `deriveMeshIPForService` reads `wg0.json` only (`service.go:455`), so the resolved IP is persisted but not read there (found by the T5 delivery reviewer; pre-existing for the public key too; deferred: docs/adr/BACKLOG.md, row written by T11).

## Verification Log
- 2026-11-15 · d7d842c* · exit 1 · `set -o pipefail …` · acceptance-sha256:250e44cb94620947c4f3a99c3640da7c22c760b326cb3ffc62297cd7b2e10722 · ms:1909 · test-lock-sha256:a0752af384665f014f2d1addd44231df8e434438b051f808817cd1fed9c9aa4e · test-lock-file:docs/adr/locks/a0752af384665f014f2d1addd44231df8e434438b051f808817cd1fed9c9aa4e.lock
  ```
  --- last 10 line(s) of stderr (of 17 after folding 17 raw)
  2026/11/15 21:56:52 [Collision] We lost collision, re-deriving mesh IP: 10.42.1.1 -> 10.170.106.187
      mesh_ip_secret_change_test.go:96: after restart the IP is 10.42.1.1, want the resolved address 10.170.106.187
  --- FAIL: TestRestartKeepsResolvedIP (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/daemon	0.007s
  === RUN   TestServiceAddReadsResolvedIP
  --- PASS: TestServiceAddReadsResolvedIP (0.00s)
  PASS
  ok  	github.com/atvirokodosprendimai/wgmesh	0.006s
  FAIL
  ```
- 2026-11-15 · d7d842c* · exit 0 · `set -o pipefail …` · acceptance-sha256:250e44cb94620947c4f3a99c3640da7c22c760b326cb3ffc62297cd7b2e10722 · ms:6389
- 2026-11-15 · d7d842c* · exit 0 · `set -o pipefail …` · acceptance-sha256:250e44cb94620947c4f3a99c3640da7c22c760b326cb3ffc62297cd7b2e10722 · ms:4142
- 2026-11-15 · d7d842c* · exit 0 · `set -o pipefail …` · acceptance-sha256:250e44cb94620947c4f3a99c3640da7c22c760b326cb3ffc62297cd7b2e10722 · ms:6460
- 2026-11-15 · 4d65d36* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:250e44cb94620947c4f3a99c3640da7c22c760b326cb3ffc62297cd7b2e10722 · ms:0 · test-lock-sha256:ec0dd8b0f3d925b89aa017cc5a77dac7f8ccf5386464ffd7e8b00a51426c02eb · test-lock-file:docs/adr/locks/ec0dd8b0f3d925b89aa017cc5a77dac7f8ccf5386464ffd7e8b00a51426c02eb.lock · test-lock-kind:replace
- 2026-11-15 · 4d65d36* · exit 0 · `set -o pipefail …` · acceptance-sha256:250e44cb94620947c4f3a99c3640da7c22c760b326cb3ffc62297cd7b2e10722 · ms:6945
- 2026-11-15 · 4d65d36* · exit 0 · `set -o pipefail …` · acceptance-sha256:250e44cb94620947c4f3a99c3640da7c22c760b326cb3ffc62297cd7b2e10722 · ms:6320
- 2026-11-15 · 4d65d36* · exit 0 · `set -o pipefail …` · acceptance-sha256:250e44cb94620947c4f3a99c3640da7c22c760b326cb3ffc62297cd7b2e10722 · ms:6389
- 2026-11-15 · 4d65d36* · exit 0 · `set -o pipefail …` · acceptance-sha256:250e44cb94620947c4f3a99c3640da7c22c760b326cb3ffc62297cd7b2e10722 · ms:6966
- 2026-11-15 · 4d65d36* · exit 0 · `set -o pipefail …` · acceptance-sha256:250e44cb94620947c4f3a99c3640da7c22c760b326cb3ffc62297cd7b2e10722 · ms:4108
