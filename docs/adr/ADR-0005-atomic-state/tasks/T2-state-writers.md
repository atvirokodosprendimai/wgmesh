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
| `pkg/daemon/cache.go` (`CacheFilePath`) | edit | the directory literal `/var/lib/wgmesh` becomes the existing variable `localNodeStateDir` (same default, `daemon.go:369`) so a test can redirect the cache file; behaviour unchanged. Added by the coding agent, accepted in review 2026-11-15 |

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
n=$(go test ./pkg/daemon ./pkg/mesh . -run '^(TestSaveLocalNodeIsAtomic|TestSaveLocalNodeKeepsOldFileWhenWriteFails|TestSavePeerCacheIsAtomic|TestMeshSaveIsAtomic|TestSaveSecretFileIsAtomic|TestMeshSaveKeepsOldFileWhenWriteFails|TestSaveSecretFileKeepsOldFileWhenWriteFails)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 7 ] && test "$(grep -rn 'os.WriteFile(' --include=*.go . | grep -v _test.go | wc -l)" -le 10 && go test -race ./pkg/daemon ./pkg/mesh -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestSaveLocalNodeIsAtomic` | `pkg/daemon/helpers_atomic_test.go` | no temp file remains, the file parses, mode is 0600, and a reader looping during 300 saves with alternating key strings of 64 KiB (test-only padding in `MeshIP`) never sees a partial file | — | S1, S2 |
| `TestSaveLocalNodeKeepsOldFileWhenWriteFails` | `pkg/daemon/helpers_atomic_test.go` | with a read-only directory the save errors and the old file is byte-identical (skipped as root) | — | S1, S2 |
| `TestSavePeerCacheIsAtomic` | `pkg/daemon/helpers_atomic_test.go` | a 1000-peer cache (the `PeerStore` cap) rewritten while read is never partial | — | S1, S2 |
| `TestMeshSaveIsAtomic` | `pkg/mesh/mesh_atomic_test.go` | same for the plain and the encrypted state | — | S1, S2 |
| `TestSaveSecretFileIsAtomic` | `main_atomic_test.go` | the secret file is replaced whole and keeps 0600 | — | S1, S2 |
| `TestMeshSaveKeepsOldFileWhenWriteFails` | `pkg/mesh/mesh_atomic_test.go` | with a read-only directory `Mesh.Save` errors and the old state is byte-identical (skipped as root) | — | S1, S2 |
| `TestSaveSecretFileKeepsOldFileWhenWriteFails` | `main_atomic_test.go` | the same for `saveSecretFile` (skipped as root) | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the five tests |
| 2 — something selects it | the four production callers above; each revert turns a test red |
| 3 — the caller can discover it | doc comments on the four functions say "atomic"; T11 amends the daemon support and CLI specs |
| 4 — it is used | every daemon start, every cache tick, every centralized command |

## Mutation Log
- 2026-11-15 · 5b1b687* · mutant inconclusive · exit 1 · `pkg/daemon/helpers.go` · saveLocalNode back to os.WriteFile: TestSaveLocalNodeIsAtomic and TestSaveLocalNodeKeepsOldFileWhenWriteFails must go red · acceptance-sha256:654ff7bff7c6a90d4c4164ff172b5f571cc0e350bd724a4052ad134707c8c5a3
  ```
  the fence failed on a build/parse error, not an assertion
  ```
- 2026-11-15 · 5b1b687* · mutant inconclusive · exit 1 · `pkg/daemon/cache.go` · SavePeerCache back to os.WriteFile: TestSavePeerCacheIsAtomic must go red · acceptance-sha256:654ff7bff7c6a90d4c4164ff172b5f571cc0e350bd724a4052ad134707c8c5a3
  ```
  the fence failed on a build/parse error, not an assertion
  ```
- 2026-11-15 · 5b1b687* · mutant inconclusive · exit 1 · `pkg/mesh/mesh.go` · Mesh.Save back to os.WriteFile: TestMeshSaveIsAtomic must go red · acceptance-sha256:654ff7bff7c6a90d4c4164ff172b5f571cc0e350bd724a4052ad134707c8c5a3
  ```
  the fence failed on a build/parse error, not an assertion
  ```
- 2026-11-15 · 5b1b687* · mutant inconclusive · exit 1 · `main.go` · saveSecretFile back to os.WriteFile: TestSaveSecretFileIsAtomic must go red · acceptance-sha256:654ff7bff7c6a90d4c4164ff172b5f571cc0e350bd724a4052ad134707c8c5a3
  ```
  the fence failed on a build/parse error, not an assertion
  ```
- 2026-11-15 · 64aff92* · mutant killed · exit 1 · `pkg/daemon/helpers.go` · saveLocalNode back to os.WriteFile: TestSaveLocalNodeIsAtomic and TestSaveLocalNodeKeepsOldFileWhenWriteFails must go red · acceptance-sha256:d9b85aa5b121aedb3b8cdf8c390aa68bc59d2bc49ce2d571a126a2929acc996b
- 2026-11-15 · 64aff92* · mutant killed · exit 1 · `pkg/daemon/cache.go` · SavePeerCache back to os.WriteFile: TestSavePeerCacheIsAtomic must go red · acceptance-sha256:d9b85aa5b121aedb3b8cdf8c390aa68bc59d2bc49ce2d571a126a2929acc996b
- 2026-11-15 · 64aff92* · mutant killed · exit 1 · `pkg/mesh/mesh.go` · Mesh.Save back to os.WriteFile: TestMeshSaveIsAtomic must go red · acceptance-sha256:d9b85aa5b121aedb3b8cdf8c390aa68bc59d2bc49ce2d571a126a2929acc996b
- 2026-11-15 · 64aff92* · mutant killed · exit 1 · `main.go` · saveSecretFile back to os.WriteFile: TestSaveSecretFileIsAtomic must go red · acceptance-sha256:d9b85aa5b121aedb3b8cdf8c390aa68bc59d2bc49ce2d571a126a2929acc996b

## Invariants

- State file formats, field names, modes and paths are unchanged; an older release reads what this one writes.
- `saveLocalNode(path string, node *LocalNode) error` keeps its signature (ADR-0002 T5 depends on it).
- A failed save returns the error to the caller exactly as `os.WriteFile` did; callers keep their present handling (`initLocalNode` logs and continues).

## Risks

- `fsync` adds a disk flush per save; measured in S4, not assumed.
- The reader tests detect truncation probabilistically; the mutant run records the red rate.
- Test doubles: temp directories; `encryptionPassword` set and restored in the mesh test.
- Behaviour differences from `os.WriteFile` that this task takes on (panel 2026-11-15; the package doc of `atomicfile` states them; none is covered by a test, the release note of T11 says them): a symlink at the state path is replaced by a regular file; the mode is reset to `0600` and the owner becomes the process user on every save (a file an admin made `0640`, or a file re-created by root, changes); the directory must be writable, so a user-owned file inside a root-owned read-only directory no longer saves; a single-file bind mount of a state file fails with EBUSY on rename (the daemon logs and continues, the identity is then not persisted); `.tmp-*` files left by `kill -9` are not swept and a leftover temp of the identity holds the private key at mode `0600`; on darwin `File.Sync` does not flush the drive cache (no `F_FULLFSYNC`), so the power-loss guarantee holds on linux only. Whether to sweep stale temp files is a follow-up, not this task.
- `SavePeerCache` takes at most `node.DefaultMaxPeers` (1000) peers, so the 2000-peer figure of the first draft is unreachable; the test uses 1000.

## Stop Condition

Stop and ask if ADR-0002 T5 has changed the signature of `saveLocalNode`, or if any of the four writers is also read by a process that expects to hold the file open across a save.

## Out of Scope

- The four private temp-and-rename copies (T3).
- Refusing to regenerate a key when the file is corrupt (T4).
- Locking the read-modify-write of `Mesh.Save` across CLI runs (deferred: docs/adr/BACKLOG.md).

## Verification Log
- 2026-11-15 · 5b1b687* · exit 1 · `set -o pipefail …` · acceptance-sha256:654ff7bff7c6a90d4c4164ff172b5f571cc0e350bd724a4052ad134707c8c5a3 · ms:5760 · test-lock-sha256:481a085974d6dabd164f9c47fe53c2226ce3e10402567f12433298987876bcc5 · test-lock-file:docs/adr/locks/481a085974d6dabd164f9c47fe53c2226ce3e10402567f12433298987876bcc5.lock
  ```
  --- last 10 line(s) of stderr (of 27 after folding 27 raw)
      --- FAIL: TestMeshSaveIsAtomic/plain (0.00s)
      --- FAIL: TestMeshSaveIsAtomic/encrypted (0.35s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/mesh	0.401s
  === RUN   TestSaveSecretFileIsAtomic
      main_atomic_test.go:36: secret file was rewritten in place (same inode); want a temp file renamed over it
  --- FAIL: TestSaveSecretFileIsAtomic (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh	0.132s
  FAIL
  ```
- 2026-11-15 · 5b1b687* · exit 0 · `set -o pipefail …` · acceptance-sha256:654ff7bff7c6a90d4c4164ff172b5f571cc0e350bd724a4052ad134707c8c5a3 · ms:49009
- 2026-11-15 · 5b1b687* · exit 0 · `set -o pipefail …` · acceptance-sha256:654ff7bff7c6a90d4c4164ff172b5f571cc0e350bd724a4052ad134707c8c5a3 · ms:46554
- 2026-11-15 · 5b1b687* · exit 0 · `set -o pipefail …` · acceptance-sha256:654ff7bff7c6a90d4c4164ff172b5f571cc0e350bd724a4052ad134707c8c5a3 · ms:33644
- 2026-11-15 · 5b1b687* · exit 0 · `set -o pipefail …` · acceptance-sha256:654ff7bff7c6a90d4c4164ff172b5f571cc0e350bd724a4052ad134707c8c5a3 · ms:29717
- 2026-11-15 · 5b1b687* · exit 0 · `set -o pipefail …` · acceptance-sha256:654ff7bff7c6a90d4c4164ff172b5f571cc0e350bd724a4052ad134707c8c5a3 · ms:29953
- 2026-11-15 · 64aff92* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:d9b85aa5b121aedb3b8cdf8c390aa68bc59d2bc49ce2d571a126a2929acc996b · ms:1 · test-lock-sha256:ee935bff9ed1ad6a4f3ed34f29a1269c109d9535b7290d7304561c238b05296e · test-lock-file:docs/adr/locks/ee935bff9ed1ad6a4f3ed34f29a1269c109d9535b7290d7304561c238b05296e.lock · test-lock-kind:replace
- 2026-11-15 · 64aff92* · exit 0 · `set -o pipefail …` · acceptance-sha256:d9b85aa5b121aedb3b8cdf8c390aa68bc59d2bc49ce2d571a126a2929acc996b · ms:46965
- 2026-11-15 · 64aff92* · exit 0 · `set -o pipefail …` · acceptance-sha256:d9b85aa5b121aedb3b8cdf8c390aa68bc59d2bc49ce2d571a126a2929acc996b · ms:33871
- 2026-11-15 · 64aff92* · exit 0 · `set -o pipefail …` · acceptance-sha256:d9b85aa5b121aedb3b8cdf8c390aa68bc59d2bc49ce2d571a126a2929acc996b · ms:27766
- 2026-11-15 · 64aff92* · exit 0 · `set -o pipefail …` · acceptance-sha256:d9b85aa5b121aedb3b8cdf8c390aa68bc59d2bc49ce2d571a126a2929acc996b · ms:31442
- 2026-11-15 · 64aff92* · exit 0 · `set -o pipefail …` · acceptance-sha256:d9b85aa5b121aedb3b8cdf8c390aa68bc59d2bc49ce2d571a126a2929acc996b · ms:29631
- 2026-11-16 · 9171f6d* · exit 0 · `set -o pipefail …` · acceptance-sha256:d9b85aa5b121aedb3b8cdf8c390aa68bc59d2bc49ce2d571a126a2929acc996b · ms:27204
- 2026-11-16 · 22b6448* · exit 0 · `set -o pipefail …` · acceptance-sha256:d9b85aa5b121aedb3b8cdf8c390aa68bc59d2bc49ce2d571a126a2929acc996b · ms:30924
- 2026-11-16 · 195438a* · exit 0 · `set -o pipefail …` · acceptance-sha256:d9b85aa5b121aedb3b8cdf8c390aa68bc59d2bc49ce2d571a126a2929acc996b · ms:26659
