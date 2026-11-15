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
