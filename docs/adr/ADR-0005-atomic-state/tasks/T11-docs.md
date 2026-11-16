# Task ADR-0005-T11: Amend the specs, write the release note and the backlog rows

**Depends-on:** T3, T4, T6, T8, T9, T10
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** none
**Consumes:** `TestNoInPlaceStateWrite` and its allowlist (T3); `ErrStateCorrupt`, `RefuseCorruptState` (T4); the socket lock (T6); local-node accessors and `Snapshot()` (T8); `registerReply` (T9); restart-safe `listenLoop` and DHT context (T10)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The daemon support, daemon lifecycle, RPC, peer exchange and DHT core specs and a release note describe the atomic state files, the two locks, the corrupt-identity handling, the reply registry and the restart contract as built, and a test fails if they stop naming them.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md` | edit | `atomicfile.WriteFile`, the guard test, the peer cache written atomically |
| `eidos/spec - daemon lifecycle - secret-derived identity with interface setup and hot-reload.md` | edit | the interface lock, `ErrStateCorrupt`, `RefuseCorruptState`, the moved-aside file |
| `eidos/spec - rpc - unix socket json-rpc server and client for peer and daemon queries.md` | edit | the socket lock and what a second daemon does |
| `eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md` | edit | `registerReply` and fan-out; restart-safe loop |
| `eidos/spec - discovery dht core - bittorent mainline dht announce query persistence and endpoint detection.md` | edit | a fresh context per `Start` |
| `docs/state-rollout.md` | add | operator and release note: what changes at N, the lock messages and what to do, how to recover a moved-aside identity, that an older running daemon holds no lock, stale `.tmp-*` files, N+1 `RefuseCorruptState`; the measured `fsync` cost from T2 with its date and machine |
| `docs/adr/BACKLOG.md` | edit | rows: N+1 `RefuseCorruptState`; `Mesh.Save` read-modify-write lock; the adopted-interface teardown; `install-service` atomic writes; `EpochManager` and RPC server restart |
| `README.md` | edit | one link to the note |
| `pkg/atomicfile/spec_test.go` | add | the specs, the note and the backlog must name the pieces |

## Ordered Steps

1. [S1] Add `pkg/atomicfile/spec_test.go` with `TestSpecNamesAtomicState`: it reads the five specs, `docs/state-rollout.md` and `docs/adr/BACKLOG.md` and fails when `atomicfile.WriteFile` (daemon support spec), `ErrStateCorrupt` and `RefuseCorruptState` (lifecycle spec and note), `flock` or `.lock` (lifecycle and RPC specs), `registerReply` (exchange spec), `parentCtx` or `beginRun` (DHT core spec) and `RefuseCorruptState` (backlog) are missing. Run red.
2. [S2] Amend the five specs.
3. [S3] Write `docs/state-rollout.md`.
4. [S4] Edit `docs/adr/BACKLOG.md`; run `adr-debt docs/adr` and paste that no deferred row of ADR-0005 is reported unreceipted.
5. [S5] Run the mutants with `adr-verify --mutant`: the lifecycle spec stops naming `ErrStateCorrupt`; the note stops naming the lock; the backlog loses the `RefuseCorruptState` row. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/atomicfile -run '^TestSpecNamesAtomicState$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 1 ] && test -f docs/state-rollout.md && ! adr-debt docs/adr | grep -q UNRECEIPTED
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestSpecNamesAtomicState` | `pkg/atomicfile/spec_test.go` | the five specs, the note and the backlog name the helper, the lock, the typed errors, the switch, the registry and the context | — | S1, S2, S3, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the test and the documents |
| 2 — something selects it | n/a: documentation |
| 3 — the caller can discover it | the note is linked from the README |
| 4 — it is used | nothing measures this yet |

## Mutation Log
- 2026-11-16 · 2c46f56* · mutant killed · exit 1 · `eidos/spec - daemon lifecycle - secret-derived identity with interface setup and hot-reload.md` · the lifecycle spec no longer names ErrStateCorrupt: TestSpecNamesAtomicState must go red · acceptance-sha256:d5ecdfdefb3cf141226849db388a21596385a8df9994d39ff4f5d7cae6f7d5c2
- 2026-11-16 · 2c46f56* · mutant killed · exit 1 · `docs/adr/BACKLOG.md` · the backlog row for N+1 no longer names RefuseCorruptState: TestSpecNamesAtomicState must go red · acceptance-sha256:d5ecdfdefb3cf141226849db388a21596385a8df9994d39ff4f5d7cae6f7d5c2
- 2026-11-16 · 2c46f56* · mutant killed · exit 1 · `eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md` · the exchange spec no longer names registerReply: TestSpecNamesAtomicState must go red · acceptance-sha256:d5ecdfdefb3cf141226849db388a21596385a8df9994d39ff4f5d7cae6f7d5c2

## Invariants

- The documents say N+1 is not done; it is a backlog row, not a task here.
- Every measured number carries its date and what it was measured on.
- The documents say a lock protects only against a running daemon of a release that takes it.

## Risks

- The documents drift from the code; the test checks names, not meaning.
- Test doubles: none.

## Stop Condition

Stop and ask if a spec section named here does not exist in the file (the file names were read on 2026-11-15 from `ls eidos`; headings were not read).

## Out of Scope

- Flipping `RefuseCorruptState` (deferred: docs/adr/BACKLOG.md).

## Verification Log
- 2026-11-16 · 2c46f56* · exit 1 · `set -o pipefail …` · acceptance-sha256:d5ecdfdefb3cf141226849db388a21596385a8df9994d39ff4f5d7cae6f7d5c2 · ms:536 · test-lock-sha256:49567656b4288b2060db62fa2711476a79cd570d56b60575f7022e9cc87f32b4 · test-lock-file:docs/adr/locks/49567656b4288b2060db62fa2711476a79cd570d56b60575f7022e9cc87f32b4.lock
  ```
  --- last 10 line(s) of stderr (of 17 after folding 17 raw)
      spec_test.go:58: eidos/spec - rpc - unix socket json-rpc server and client for peer and daemon queries.md: does not name ".lock"
      spec_test.go:58: eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md: does not name "registerReply"
      spec_test.go:58: eidos/spec - discovery dht core - bittorent mainline dht announce query persistence and endpoint detection.md: does not name "parentCtx"
      spec_test.go:58: eidos/spec - discovery dht core - bittorent mainline dht announce query persistence and endpoint detection.md: does not name "beginRun"
      spec_test.go:53: docs/state-rollout.md: cannot be read: open /claude/scratch/2026-11-15/adr-g4/docs/state-rollout.md: no such file or directory
      spec_test.go:58: docs/adr/BACKLOG.md: does not name "RefuseCorruptState"
  --- FAIL: TestSpecNamesAtomicState (0.01s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/atomicfile	0.015s
  FAIL
  ```
- 2026-11-16 · 2c46f56* · exit 0 · `set -o pipefail …` · acceptance-sha256:d5ecdfdefb3cf141226849db388a21596385a8df9994d39ff4f5d7cae6f7d5c2 · ms:973
- 2026-11-16 · 2c46f56* · exit 0 · `set -o pipefail …` · acceptance-sha256:d5ecdfdefb3cf141226849db388a21596385a8df9994d39ff4f5d7cae6f7d5c2 · ms:1329
- 2026-11-16 · 2c46f56* · exit 0 · `set -o pipefail …` · acceptance-sha256:d5ecdfdefb3cf141226849db388a21596385a8df9994d39ff4f5d7cae6f7d5c2 · ms:1124
- 2026-11-16 · 2c46f56* · exit 0 · `set -o pipefail …` · acceptance-sha256:d5ecdfdefb3cf141226849db388a21596385a8df9994d39ff4f5d7cae6f7d5c2 · ms:1678
