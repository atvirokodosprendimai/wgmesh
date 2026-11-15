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
n=$(go test ./pkg/atomicfile -run '^(TestNoInPlaceStateWrite|TestNoPrivateRenameCopy|TestGuardSkipsHiddenAndNestedModuleDirs)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 3 ] && go test -race ./pkg/mesh ./pkg/promo . -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestNoInPlaceStateWrite` | `pkg/atomicfile/guard_test.go` | the per-file `os.WriteFile` counts equal the table; a file not in the table has none | — | S1, S2, S3 |
| `TestNoPrivateRenameCopy` | `pkg/atomicfile/guard_test.go` | `os.Rename` appears in production code only in `pkg/atomicfile/atomicfile.go` | — | S1, S2 |
| `TestGuardSkipsHiddenAndNestedModuleDirs` | `pkg/atomicfile/guard_test.go` | `countCalls` over a temp tree skips a hidden directory and a nested module, and counts a normal directory | — | S1 |
| `TestSaveServicesLeavesOnlyTheFile` | `pkg/mesh/services_atomic_test.go` | after two saves the directory holds exactly the one file (the existing test checks only the old `.tmp` name, which no longer exists) | — | S2 |
| `TestPromoSaveLeavesOnlyTheStoreFile` | `pkg/promo/store_atomic_test.go` | the same for the promo store | — | S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the two guard tests |
| 2 — something selects it | `go test ./...`; the three mutants turn them red |
| 3 — the caller can discover it | the table is in the test; T11 names the rule in the daemon support spec |
| 4 — it is used | every future change that adds a writer |

## Mutation Log
- 2026-11-15 · ac3c8fd* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · a new os.WriteFile in a file not in the table: TestNoInPlaceStateWrite must go red · acceptance-sha256:c876a4d281ba2f73305b043d58f60d43259fe0aa9cf9b71fd6492300a609c7bd
- 2026-11-15 · ac3c8fd* · mutant killed · exit 1 · `pkg/mesh/services.go` · a private rename copy reappears: TestNoPrivateRenameCopy must go red · acceptance-sha256:c876a4d281ba2f73305b043d58f60d43259fe0aa9cf9b71fd6492300a609c7bd
- 2026-11-15 · ac3c8fd* · mutant killed · exit 1 · `pkg/atomicfile/guard_test.go` · the table row of systemd.go is deleted: TestNoInPlaceStateWrite must go red · acceptance-sha256:c876a4d281ba2f73305b043d58f60d43259fe0aa9cf9b71fd6492300a609c7bd
- 2026-11-15 · fc71fd9* · mutant killed · exit 1 · `pkg/mesh/services.go` · a private rename copy reappears: TestNoPrivateRenameCopy must go red · acceptance-sha256:3f6ba5a13f7f26a774cdc864f08c016f69c95b69c9b1bdc82d00e1a05b47e7fb
- 2026-11-15 · fc71fd9* · mutant killed · exit 1 · `pkg/atomicfile/guard_test.go` · the table row of systemd.go is deleted: TestNoInPlaceStateWrite must go red · acceptance-sha256:3f6ba5a13f7f26a774cdc864f08c016f69c95b69c9b1bdc82d00e1a05b47e7fb
- 2026-11-15 · fc71fd9* · mutant killed · exit 1 · `pkg/atomicfile/guard_test.go` · hidden directories are walked: TestGuardSkipsHiddenAndNestedModuleDirs must go red · acceptance-sha256:3f6ba5a13f7f26a774cdc864f08c016f69c95b69c9b1bdc82d00e1a05b47e7fb
- 2026-11-15 · fc71fd9* · mutant killed · exit 1 · `pkg/atomicfile/guard_test.go` · nested modules are walked: TestGuardSkipsHiddenAndNestedModuleDirs must go red · acceptance-sha256:3f6ba5a13f7f26a774cdc864f08c016f69c95b69c9b1bdc82d00e1a05b47e7fb
- 2026-11-15 · fc71fd9* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · a new os.WriteFile in a file not in the table: TestNoInPlaceStateWrite must go red · acceptance-sha256:3f6ba5a13f7f26a774cdc864f08c016f69c95b69c9b1bdc82d00e1a05b47e7fb

## Invariants

- Account, services, promo and referral files keep their format, path and mode `0600`; their existing tests are not edited.
- The guard reads files from the module root found with `go list -m -f {{.Dir}}`, never a path typed into the test.

## Risks

- The table holds line-free counts: an added call fails (the count differs from the row), but swapping one allowed call for a state writer in the same file passes. The guard does not count `os.Create`, `os.OpenFile` with `O_TRUNC` or `syscall.Rename`, and assumes no `go.work` file (`go list -m` prints one line per module there). The doc comment says so.
- Behaviour taken on (panel 2026-11-15, not covered by a test): `atomicfile.WriteFile` can return an error after the rename when only the directory fsync fails; `pkg/promo` rolls its memory back on any save error, so after such an error memory and disk differ until restart, and `service` exits 1 after the change is committed. Question for M in the report. The existing tests `services_test.go` and `store_test.go` look for the old `.tmp` name and now assert nothing about leftovers (the invariant forbids editing them); the two new tests above cover it. Read-modify-write of the services and referral files without a lock is the lost-update class of the parent Out of Scope (deferred: docs/adr/BACKLOG.md).
- Test doubles: none; the guard parses real files.

## Stop Condition

Stop and ask if a file outside the table already has an `os.WriteFile` that is state (the table was built from the grep of 2026-11-15, not from reading each caller's callers).

## Out of Scope

- `pkg/daemon/systemd.go` writes (deferred: docs/adr/BACKLOG.md).
- The report and generated-file writers in the table (permanent: boundary: not state read again by the program).

## Verification Log
- 2026-11-15 · ac3c8fd* · exit 1 · `set -o pipefail …` · acceptance-sha256:c876a4d281ba2f73305b043d58f60d43259fe0aa9cf9b71fd6492300a609c7bd · ms:785 · test-lock-sha256:59a4fd6579cdde56cd63522f67b05b925d014a3ac3ff453f173274de16642c77 · test-lock-file:docs/adr/locks/59a4fd6579cdde56cd63522f67b05b925d014a3ac3ff453f173274de16642c77.lock
  ```
  --- last 10 line(s) of stderr (of 15 after folding 15 raw)
  --- FAIL: TestNoInPlaceStateWrite (0.06s)
  === RUN   TestNoPrivateRenameCopy
      guard_test.go:160: main.go: found 1 os.Rename calls, allowed 0; a state file must be replaced with atomicfile.WriteFile, not a private temp-and-rename copy
      guard_test.go:160: pkg/mesh/account.go: found 1 os.Rename calls, allowed 0; a state file must be replaced with atomicfile.WriteFile, not a private temp-and-rename copy
      guard_test.go:160: pkg/mesh/services.go: found 1 os.Rename calls, allowed 0; a state file must be replaced with atomicfile.WriteFile, not a private temp-and-rename copy
      guard_test.go:160: pkg/promo/store.go: found 1 os.Rename calls, allowed 0; a state file must be replaced with atomicfile.WriteFile, not a private temp-and-rename copy
  --- FAIL: TestNoPrivateRenameCopy (0.07s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/atomicfile	0.141s
  FAIL
  ```
- 2026-11-15 · ac3c8fd* · exit 0 · `set -o pipefail …` · acceptance-sha256:c876a4d281ba2f73305b043d58f60d43259fe0aa9cf9b71fd6492300a609c7bd · ms:37764
- 2026-11-15 · ac3c8fd* · exit 0 · `set -o pipefail …` · acceptance-sha256:c876a4d281ba2f73305b043d58f60d43259fe0aa9cf9b71fd6492300a609c7bd · ms:34213
- 2026-11-15 · ac3c8fd* · exit 0 · `set -o pipefail …` · acceptance-sha256:c876a4d281ba2f73305b043d58f60d43259fe0aa9cf9b71fd6492300a609c7bd · ms:30317
- 2026-11-15 · ac3c8fd* · exit 0 · `set -o pipefail …` · acceptance-sha256:c876a4d281ba2f73305b043d58f60d43259fe0aa9cf9b71fd6492300a609c7bd · ms:27543
- 2026-11-15 · fc71fd9* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:3f6ba5a13f7f26a774cdc864f08c016f69c95b69c9b1bdc82d00e1a05b47e7fb · ms:0 · test-lock-sha256:0167356a5dd0c48ec97c2e1afa7cfd1fda1ec22500b3d586a91ff8d16cd39b7f · test-lock-file:docs/adr/locks/0167356a5dd0c48ec97c2e1afa7cfd1fda1ec22500b3d586a91ff8d16cd39b7f.lock · test-lock-kind:replace
- 2026-11-15 · fc71fd9* · exit 1 · `set -o pipefail …` · acceptance-sha256:3f6ba5a13f7f26a774cdc864f08c016f69c95b69c9b1bdc82d00e1a05b47e7fb · ms:32635
  ```
  --- last 7 line(s) of stdout
  ok  	github.com/atvirokodosprendimai/wgmesh/pkg/mesh	24.998s
  --- FAIL: TestValidateCodeChecksum (0.00s)
      generate_test.go:104: only 3/8 payload mutations detected (expected >=4)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/promo	0.072s
  ok  	github.com/atvirokodosprendimai/wgmesh	26.932s
  FAIL
  --- last 8 line(s) of stderr
  === RUN   TestNoInPlaceStateWrite
  --- PASS: TestNoInPlaceStateWrite (0.06s)
  === RUN   TestNoPrivateRenameCopy
  --- PASS: TestNoPrivateRenameCopy (0.05s)
  === RUN   TestGuardSkipsHiddenAndNestedModuleDirs
  --- PASS: TestGuardSkipsHiddenAndNestedModuleDirs (0.00s)
  PASS
  ok  	github.com/atvirokodosprendimai/wgmesh/pkg/atomicfile	0.107s
  ```
- 2026-11-15 · fc71fd9* · exit 0 · `set -o pipefail …` · acceptance-sha256:3f6ba5a13f7f26a774cdc864f08c016f69c95b69c9b1bdc82d00e1a05b47e7fb · ms:29570
- 2026-11-15 · fc71fd9* · exit 0 · `set -o pipefail …` · acceptance-sha256:3f6ba5a13f7f26a774cdc864f08c016f69c95b69c9b1bdc82d00e1a05b47e7fb · ms:32872
- 2026-11-15 · fc71fd9* · exit 0 · `set -o pipefail …` · acceptance-sha256:3f6ba5a13f7f26a774cdc864f08c016f69c95b69c9b1bdc82d00e1a05b47e7fb · ms:28855
- 2026-11-15 · fc71fd9* · exit 0 · `set -o pipefail …` · acceptance-sha256:3f6ba5a13f7f26a774cdc864f08c016f69c95b69c9b1bdc82d00e1a05b47e7fb · ms:31834
- 2026-11-15 · fc71fd9* · exit 0 · `set -o pipefail …` · acceptance-sha256:3f6ba5a13f7f26a774cdc864f08c016f69c95b69c9b1bdc82d00e1a05b47e7fb · ms:28321
- 2026-11-15 · fc71fd9* · exit 0 · `set -o pipefail …` · acceptance-sha256:3f6ba5a13f7f26a774cdc864f08c016f69c95b69c9b1bdc82d00e1a05b47e7fb · ms:29442
