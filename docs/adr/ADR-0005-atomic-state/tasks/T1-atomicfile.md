# Task ADR-0005-T1: Add `atomicfile.WriteFile`, the one commit step for state files

**Depends-on:** none
**Covers:** none — no spec
**Estimated scope:** S (single file)
**Owner:** unassigned
**Produces:** `atomicfile.WriteFile(path string, data []byte, perm os.FileMode) error`
**Consumes:** none
**Data dependency:** hermetic
**Proof map:** v1

## Goal

One function replaces a file whole or not at all: a crash, a full disk or a failed rename leaves the old file and no temp file behind, and the data is on disk before the rename.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/atomicfile/atomicfile.go` | add | `WriteFile`: `os.CreateTemp` in the target directory, `Chmod(perm)` before the data, write, `Sync`, close, `os.Rename`, directory `Sync` (`EINVAL` and `ENOTSUP` ignored, other errors returned), temp removed on every failure path; package-level unexported `syncFile` and `syncDir` function variables are the test seam |
| `pkg/atomicfile/atomicfile_test.go` | add | tests below |

Selection: nothing selects it until T2 and T3 call it; reachability is shown there. The exported name is the only entry.

## Ordered Steps

1. [S1] Add `atomicfile_test.go` with the five tests and a compiling stub (`WriteFile` that returns nil and writes nothing) so the first run fails on assertions, not on the build. Run red.
2. [S2] Implement `WriteFile` as described; the data is written to the temp file, never to `path`.
3. [S3] Run the mutants with `adr-verify --mutant`: skip the file `Sync`; skip the directory `Sync`; write the data to `path` directly then rename nothing; leave the temp file on a rename failure. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/atomicfile -run '^(TestWriteFileReplacesWhole|TestWriteFileKeepsOldFileWhenRenameFails|TestWriteFileLeavesNoTempFile|TestWriteFileSyncsFileThenDirectory|TestConcurrentReaderNeverSeesPartialFile|TestWriteFileKeepsOldFileWhenSyncFails|TestWriteFileIgnoresUnsupportedDirSync|TestWriteFileReportsDirSyncFailureAfterRename)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 8 ] && go test -race ./pkg/atomicfile -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestWriteFileReplacesWhole` | `pkg/atomicfile/atomicfile_test.go` | new content replaces old; mode is the requested `perm` | — | S1, S2 |
| `TestWriteFileKeepsOldFileWhenRenameFails` | `pkg/atomicfile/atomicfile_test.go` | when `path` is a non-empty directory the rename fails, the error is returned and no temp file is left | — | S1, S2 |
| `TestWriteFileLeavesNoTempFile` | `pkg/atomicfile/atomicfile_test.go` | after success and after failure the directory holds only the target | — | S1, S2 |
| `TestWriteFileSyncsFileThenDirectory` | `pkg/atomicfile/atomicfile_test.go` | the `syncFile` seam is called before the rename and the `syncDir` seam after it (recorded order) | — | S1, S2 |
| `TestConcurrentReaderNeverSeesPartialFile` | `pkg/atomicfile/atomicfile_test.go` | 300 alternating 1 MiB writes of two contents while a reader loops: every read equals one of the two whole contents | — | S1, S2 |
| `TestWriteFileKeepsOldFileWhenSyncFails` | `pkg/atomicfile/atomicfile_test.go` | the `syncFile` seam returns EIO on an existing path: the error is returned, the old content is intact and no temp file is left | — | S1, S2 |
| `TestWriteFileIgnoresUnsupportedDirSync` | `pkg/atomicfile/atomicfile_test.go` | the `syncDir` seam returns EINVAL, then ENOTSUP: `WriteFile` returns nil and the new content is in place | — | S1, S2 |
| `TestWriteFileReportsDirSyncFailureAfterRename` | `pkg/atomicfile/atomicfile_test.go` | the `syncDir` seam returns EIO: the error is returned, the new content is already at `path` and no temp file is left | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the five tests |
| 2 — something selects it | n/a until T2 and T3: no caller in this task; the guard test of T3 is what forbids bypassing it |
| 3 — the caller can discover it | package doc comment; T11 amends the daemon support spec |
| 4 — it is used | T2, T3 |

## Mutation Log
- 2026-11-15 · f644f14* · mutant killed · exit 1 · `pkg/atomicfile/atomicfile.go` · the file Sync is skipped: TestWriteFileSyncsFileThenDirectory must go red · acceptance-sha256:3e2fd6f99beb42d347858072e560ab4caa34a76eed8db4674ae38dad357e3785
- 2026-11-15 · f644f14* · mutant killed · exit 1 · `pkg/atomicfile/atomicfile.go` · the directory Sync is skipped: TestWriteFileSyncsFileThenDirectory must go red · acceptance-sha256:3e2fd6f99beb42d347858072e560ab4caa34a76eed8db4674ae38dad357e3785
- 2026-11-15 · f644f14* · mutant killed · exit 1 · `pkg/atomicfile/atomicfile.go` · data goes to path instead of the temp file: the replace and sync-order tests must go red · acceptance-sha256:3e2fd6f99beb42d347858072e560ab4caa34a76eed8db4674ae38dad357e3785
- 2026-11-15 · f644f14* · mutant killed · exit 1 · `pkg/atomicfile/atomicfile.go` · temp file is not removed on failure: TestWriteFileKeepsOldFileWhenRenameFails and TestWriteFileLeavesNoTempFile must go red · acceptance-sha256:3e2fd6f99beb42d347858072e560ab4caa34a76eed8db4674ae38dad357e3785

## Invariants

- The package imports only the standard library and holds no package-level mutable state other than the two test seams.
- The target is never opened for writing; only the temp file is.
- The temp file is created in the target's directory (same filesystem, so rename is atomic).

## Risks

- A directory `Sync` unsupported by the filesystem returns `EINVAL` or `ENOTSUP`; both are ignored, other errors are returned. Not tested on NFS or FUSE.
- The reader test is probabilistic against a truncating writer; the mutant run records how many of 5 runs it turned red.
- Test doubles: the two seams; the rest uses a temp directory.
- Differences from `os.WriteFile`, stated in the package doc (panel 2026-11-15): `perm` is applied exactly, not filtered by umask; a symlink at `path` is replaced by a regular file; the new file has the process's owner and group; the directory must be writable. An error from the directory sync arrives after the rename, so the data is committed. A crash between create and rename leaves a `<name>.tmp-*` file that nothing sweeps.

## Stop Condition

Stop and ask if the temp file cannot be created in the target's directory on a supported platform (read-only parent with a writable file), or if a caller needs a mode different from `0600` or `0644`.

## Out of Scope

- Replacing any caller (T2, T3).
- Locking between two writers of one file (permanent: boundary: the writers named in this record are single-process; a lost update is a different class, see the parent Out of Scope).

## Verification Log
- 2026-11-15 · f644f14* · exit 1 · `set -o pipefail …` · acceptance-sha256:3e2fd6f99beb42d347858072e560ab4caa34a76eed8db4674ae38dad357e3785 · ms:2924 · test-lock-sha256:353ab41cc6f80ad1ae4bb439901634d6d6e7bcfa5ac88f27a3c18feca976a721 · test-lock-file:docs/adr/locks/353ab41cc6f80ad1ae4bb439901634d6d6e7bcfa5ac88f27a3c18feca976a721.lock
  ```
  --- last 10 line(s) of stderr (of 18 after folding 18 raw)
  --- FAIL: TestWriteFileLeavesNoTempFile (0.00s)
  === RUN   TestWriteFileSyncsFileThenDirectory
      atomicfile_test.go:133: sync order = [], want [file dir]
  --- FAIL: TestWriteFileSyncsFileThenDirectory (0.00s)
  === RUN   TestConcurrentReaderNeverSeesPartialFile
      atomicfile_test.go:222: final content is not the last write (err <nil>)
  --- FAIL: TestConcurrentReaderNeverSeesPartialFile (0.01s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/atomicfile	0.018s
  FAIL
  ```
- 2026-11-15 · f644f14* · exit 0 · `set -o pipefail …` · acceptance-sha256:3e2fd6f99beb42d347858072e560ab4caa34a76eed8db4674ae38dad357e3785 · ms:5131
- 2026-11-15 · f644f14* · exit 0 · `set -o pipefail …` · acceptance-sha256:3e2fd6f99beb42d347858072e560ab4caa34a76eed8db4674ae38dad357e3785 · ms:7746
- 2026-11-15 · f644f14* · exit 0 · `set -o pipefail …` · acceptance-sha256:3e2fd6f99beb42d347858072e560ab4caa34a76eed8db4674ae38dad357e3785 · ms:7652
- 2026-11-15 · f644f14* · exit 0 · `set -o pipefail …` · acceptance-sha256:3e2fd6f99beb42d347858072e560ab4caa34a76eed8db4674ae38dad357e3785 · ms:8307
- 2026-11-15 · f644f14* · exit 0 · `set -o pipefail …` · acceptance-sha256:3e2fd6f99beb42d347858072e560ab4caa34a76eed8db4674ae38dad357e3785 · ms:5571
