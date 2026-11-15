# Task ADR-0005-T2: Write the four in-place state files through `atomicfile` (#870)

**Depends-on:** T1
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `saveLocalNode`, `SavePeerCache`, `Mesh.Save` and `saveSecretFile` write atomically; signatures unchanged
**Consumes:** `atomicfile.WriteFile` (T1)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The local node identity, the peer cache, the centralized mesh state and the init secret file can no longer be left truncated by a crash during the write.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/helpers.go` | edit | `saveLocalNode` ends with `atomicfile.WriteFile(path, data, 0600)` instead of `os.WriteFile` (`:94`); the signature is unchanged because ADR-0002 T5 calls it |
| `pkg/daemon/cache.go` | edit | `SavePeerCache` ends with the helper (`:90`); `RestoreFromCache` is ADR-0002 T2's and is not touched |
| `pkg/mesh/mesh.go` | edit | `Mesh.Save` calls the helper (`:105`); the encrypted and plain paths both reach it |
| `main.go` | edit | `saveSecretFile` calls the helper (`:321`) |
| `pkg/daemon/helpers_atomic_test.go` | add | the identity and cache tests |
| `pkg/mesh/mesh_atomic_test.go` | add | the mesh state test |
| `main_atomic_test.go` | add | the secret file test |

Selection: `initLocalNode` calls `saveLocalNode` (`daemon.go:383`, `:392`, `:433`), `StartCacheSaver` calls `SavePeerCache`, `Mesh.Save` is called by every centralized command, `saveSecretFile` by `wgmesh init --save-to`; reverting any of the four to `os.WriteFile` turns its test red.

Ordering note: ADR-0002 T5 adds a call to `saveLocalNode` in `collision.go`; this task edits only the body in `helpers.go`, so it may land before or after T5 and the call is atomic either way.

## Ordered Steps

1. [S1] Add the five tests below and run them red against the current `os.WriteFile` bodies (the reader tests read a large file while it is rewritten; the failure-keeps-old tests make the directory read-only and skip when the process is root, say so in the skip message).
2. [S2] Switch the four writers. [proof: acceptance]
3. [S3] Run `grep -rn "os.WriteFile(" --include=*.go . | grep -v _test.go | wc -l` and record the count in the commit message (14 before this task, 10 after; T3 takes it to the allowlist). [proof: acceptance]
4. [S4] Time one `SavePeerCache` of 1000 peers and one `saveLocalNode` before and after on the development machine, record both, the date and the machine in the commit message. [proof: human: fsync cost is the input to the Consequences note; adr-verify cannot see it]
5. [S5] Run the mutants with `adr-verify --mutant`: each of the four writers reverted to `os.WriteFile`. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/daemon ./pkg/mesh . -run '^(TestSaveLocalNodeIsAtomic|TestSaveLocalNodeKeepsOldFileWhenWriteFails|TestSavePeerCacheIsAtomic|TestMeshSaveIsAtomic|TestSaveSecretFileIsAtomic)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 5 ] && test "$(grep -rn 'os.WriteFile(' --include=*.go . | grep -v _test.go | wc -l)" -le 10 && go test -race ./pkg/daemon ./pkg/mesh -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestSaveLocalNodeIsAtomic` | `pkg/daemon/helpers_atomic_test.go` | no temp file remains, the file parses, mode is 0600, and a reader looping during 300 saves with alternating key strings of 64 KiB (test-only padding in `MeshIP`) never sees a partial file | — | S1, S2 |
| `TestSaveLocalNodeKeepsOldFileWhenWriteFails` | `pkg/daemon/helpers_atomic_test.go` | with a read-only directory the save errors and the old file is byte-identical (skipped as root) | — | S1, S2 |
| `TestSavePeerCacheIsAtomic` | `pkg/daemon/helpers_atomic_test.go` | a 2000-peer cache rewritten while read is never partial | — | S1, S2 |
| `TestMeshSaveIsAtomic` | `pkg/mesh/mesh_atomic_test.go` | same for the plain and the encrypted state | — | S1, S2 |
| `TestSaveSecretFileIsAtomic` | `main_atomic_test.go` | the secret file is replaced whole and keeps 0600 | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the five tests |
| 2 — something selects it | the four production callers above; each revert turns a test red |
| 3 — the caller can discover it | doc comments on the four functions say "atomic"; T11 amends the daemon support and CLI specs |
| 4 — it is used | every daemon start, every cache tick, every centralized command |

## Mutation Log

## Invariants

- State file formats, field names, modes and paths are unchanged; an older release reads what this one writes.
- `saveLocalNode(path string, node *LocalNode) error` keeps its signature (ADR-0002 T5 depends on it).
- A failed save returns the error to the caller exactly as `os.WriteFile` did; callers keep their present handling (`initLocalNode` logs and continues).

## Risks

- `fsync` adds a disk flush per save; measured in S4, not assumed.
- The reader tests detect truncation probabilistically; the mutant run records the red rate.
- Test doubles: temp directories; `encryptionPassword` set and restored in the mesh test.

## Stop Condition

Stop and ask if ADR-0002 T5 has changed the signature of `saveLocalNode`, or if any of the four writers is also read by a process that expects to hold the file open across a save.

## Out of Scope

- The four private temp-and-rename copies (T3).
- Refusing to regenerate a key when the file is corrupt (T4).
- Locking the read-modify-write of `Mesh.Save` across CLI runs (deferred: docs/adr/BACKLOG.md).

## Verification Log
