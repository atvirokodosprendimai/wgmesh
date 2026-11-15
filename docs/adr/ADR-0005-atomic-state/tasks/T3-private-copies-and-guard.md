# Task ADR-0005-T3: Replace the four private temp-and-rename copies and fail the build on a new one

**Depends-on:** T2
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `TestNoInPlaceStateWrite` and its allowlist
**Consumes:** `atomicfile.WriteFile` (T1); the four in-place writers already switched (T2)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The rule "a state file is replaced by the one helper" exists once: the four private copies call it, and a test fails when a production file adds an `os.WriteFile` or an `os.Rename` that is not on the allowlist.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/mesh/account.go` | edit | `SaveAccount` calls the helper instead of `path + ".tmp"` and `os.Rename` (`:52-60`) |
| `pkg/mesh/services.go` | edit | the same at `:63-68` |
| `pkg/promo/store.go` | edit | the same at `:128-136` |
| `main.go` | edit | the referral state write at `:1583-1590` |
| `pkg/atomicfile/guard_test.go` | add | `TestNoInPlaceStateWrite` and `TestNoPrivateRenameCopy` parse every non-test `.go` file of the module with `go/parser` and count `os.WriteFile`, `ioutil.WriteFile` and `os.Rename` calls per file against a table in the test |

The table (2026-11-15, `b8b83f3`, after T2): `os.WriteFile` allowed in `main.go` 1 (the `--output` report), `pkg/pilot/report.go` 1, `pkg/pilot/config.go` 1, `pkg/daemon/systemd.go` 2, `cmd/status-gen/main.go` 1; `os.Rename` allowed only in `pkg/atomicfile/atomicfile.go` 1. A file not in the table must have 0.

Selection: the guard is a test, run by `go test ./...`; it is the selector. Deleting the table row for `systemd.go` turns it red, and so does adding one `os.WriteFile` to `daemon.go`.

## Ordered Steps

1. [S1] Add `guard_test.go` with both tests and run them red (before the four copies are replaced the `os.Rename` count is 4 outside `atomicfile`, and `main.go` has two `os.WriteFile` where the table says one).
2. [S2] Switch the four copies to `atomicfile.WriteFile`; the existing tests of `pkg/mesh`, `pkg/promo` and the referral code keep passing unchanged.
3. [S3] State the guard's limit in its doc comment: it counts calls by file, so a second `os.WriteFile` that replaces one the table allows in the same file is not seen; a call through an alias import is not seen.
4. [S4] Run the mutants with `adr-verify --mutant`: one `os.WriteFile` added to `pkg/daemon/daemon.go`; one `os.Rename` added to `pkg/mesh/services.go`; the `systemd.go` table row removed. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/atomicfile -run '^(TestNoInPlaceStateWrite|TestNoPrivateRenameCopy)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 2 ] && go test -race ./pkg/mesh ./pkg/promo . -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestNoInPlaceStateWrite` | `pkg/atomicfile/guard_test.go` | the per-file `os.WriteFile` counts equal the table; a file not in the table has none | — | S1, S2, S3 |
| `TestNoPrivateRenameCopy` | `pkg/atomicfile/guard_test.go` | `os.Rename` appears in production code only in `pkg/atomicfile/atomicfile.go` | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the two guard tests |
| 2 — something selects it | `go test ./...`; the three mutants turn them red |
| 3 — the caller can discover it | the table is in the test; T11 names the rule in the daemon support spec |
| 4 — it is used | every future change that adds a writer |

## Mutation Log

## Invariants

- Account, services, promo and referral files keep their format, path and mode `0600`; their existing tests are not edited.
- The guard reads files from the module root found with `go list -m -f {{.Dir}}`, never a path typed into the test.

## Risks

- The table holds line-free counts, so an unrelated edit that adds an allowed call elsewhere in an allowed file passes; the doc comment says so.
- Test doubles: none; the guard parses real files.

## Stop Condition

Stop and ask if a file outside the table already has an `os.WriteFile` that is state (the table was built from the grep of 2026-11-15, not from reading each caller's callers).

## Out of Scope

- `pkg/daemon/systemd.go` writes (deferred: docs/adr/BACKLOG.md).
- The report and generated-file writers in the table (permanent: boundary: not state read again by the program).

## Verification Log
