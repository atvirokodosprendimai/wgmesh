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
n=$(go test ./pkg/atomicfile -run '^(TestWriteFileReplacesWhole|TestWriteFileKeepsOldFileWhenRenameFails|TestWriteFileLeavesNoTempFile|TestWriteFileSyncsFileThenDirectory|TestConcurrentReaderNeverSeesPartialFile)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 5 ] && go test -race ./pkg/atomicfile -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestWriteFileReplacesWhole` | `pkg/atomicfile/atomicfile_test.go` | new content replaces old; mode is the requested `perm` | — | S1, S2 |
| `TestWriteFileKeepsOldFileWhenRenameFails` | `pkg/atomicfile/atomicfile_test.go` | when `path` is a non-empty directory the rename fails, the error is returned and no temp file is left | — | S1, S2 |
| `TestWriteFileLeavesNoTempFile` | `pkg/atomicfile/atomicfile_test.go` | after success and after failure the directory holds only the target | — | S1, S2 |
| `TestWriteFileSyncsFileThenDirectory` | `pkg/atomicfile/atomicfile_test.go` | the `syncFile` seam is called before the rename and the `syncDir` seam after it (recorded order) | — | S1, S2 |
| `TestConcurrentReaderNeverSeesPartialFile` | `pkg/atomicfile/atomicfile_test.go` | 300 alternating 1 MiB writes of two contents while a reader loops: every read equals one of the two whole contents | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the five tests |
| 2 — something selects it | n/a until T2 and T3: no caller in this task; the guard test of T3 is what forbids bypassing it |
| 3 — the caller can discover it | package doc comment; T11 amends the daemon support spec |
| 4 — it is used | T2, T3 |

## Mutation Log

## Invariants

- The package imports only the standard library and holds no package-level mutable state other than the two test seams.
- The target is never opened for writing; only the temp file is.
- The temp file is created in the target's directory (same filesystem, so rename is atomic).

## Risks

- A directory `Sync` unsupported by the filesystem returns `EINVAL` or `ENOTSUP`; both are ignored, other errors are returned. Not tested on NFS or FUSE.
- The reader test is probabilistic against a truncating writer; the mutant run records how many of 5 runs it turned red.
- Test doubles: the two seams; the rest uses a temp directory.

## Stop Condition

Stop and ask if the temp file cannot be created in the target's directory on a supported platform (read-only parent with a writable file), or if a caller needs a mode different from `0600` or `0644`.

## Out of Scope

- Replacing any caller (T2, T3).
- Locking between two writers of one file (permanent: boundary: the writers named in this record are single-process; a lost update is a different class, see the parent Out of Scope).

## Verification Log
