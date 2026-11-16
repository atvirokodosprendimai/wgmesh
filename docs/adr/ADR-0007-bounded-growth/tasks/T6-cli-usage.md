# Task ADR-0007-T6: Print usage for an unknown command and honour `--version` only before the subcommand (#873 items 1 and 2)

**Depends-on:** none
**Resolved:** ADR-0002 is merged on main (f3860c6, 2026-11-16; `git cat-file -e origin/main:pkg/node/spec_admit_test.go` exits 0)
**Covers:** none — no spec
**Estimated scope:** S (single file)
**Owner:** unassigned
**Produces:** none
**Consumes:** none
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`wgmesh`, `wgmesh help` and `wgmesh statu` print usage on stderr and exit 2, `wgmesh peers list --version` is handled by `peers`, and `wgmesh --version`, `wgmesh version` and the legacy flags (`wgmesh -list`) work as before.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `main.go` | edit | the version scan (`main.go:38-43`) looks at `os.Args[1]` only; before the legacy flag block (`:93-145`), a first argument that does not start with `-` and matched no subcommand prints usage to stderr and exits 2; `help`, `-h` and `--help` print usage and exit 0 |
| `testdata/script/cli-unknown-command.txtar`, `cli-bare-usage.txtar`, `cli-version-position.txtar`, `cli-legacy-flag.txtar` | add | `exec wgmesh ...` scripts run by `TestScript` (`main_test.go:20`) |

Selection: the scripts run in `go test .` through `TestScript`; deleting the new branch in `main` turns `cli-unknown-command` and `cli-bare-usage` red.

## Ordered Steps

1. [S1] Add the four scripts (stdout and stderr patterns, `! exec` for the exit code) and run `go test . -run 'TestScript/cli-'` red: today a typo prints "Failed to load mesh state" and `--version` after a subcommand prints the version.
2. [S2] Read the usage text at `main.go:190-200` and print it from one function used by `help` and by the unknown-command branch; do not change its content.
3. [S3] Limit the version scan to the first argument; keep `version` as a subcommand.
4. [S4] Add the unknown-first-word branch ahead of `flag.Parse` in the legacy block: `len(os.Args) == 1` or `os.Args[1]` not starting with `-` prints usage and exits 2; arguments starting with `-` fall through unchanged.
5. [S5] Run the mutants with `adr-verify --mutant`: the unknown branch removed; the version scan restored to all arguments; the legacy fall-through blocked. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test . -run '^TestScript/(cli-unknown-command|cli-bare-usage|cli-version-position|cli-legacy-flag)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c -- '--- PASS: TestScript/cli-') && [ "$n" -ge 4 ] && go test -race . -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestScript/cli-unknown-command` | `testdata/script/cli-unknown-command.txtar` | `wgmesh statu` exits 2 and stderr holds the usage text, not "Failed to load mesh state" | — | S1, S4 |
| `TestScript/cli-bare-usage` | `testdata/script/cli-bare-usage.txtar` | `wgmesh` and `wgmesh help` print usage; bare exits 2, `help` exits 0 | — | S1, S2, S4 |
| `TestScript/cli-version-position` | `testdata/script/cli-version-position.txtar` | `wgmesh --version` prints the version; `wgmesh peers list --version` does not | — | S1, S3 |
| `TestScript/cli-legacy-flag` | `testdata/script/cli-legacy-flag.txtar` | `wgmesh -list -state <missing file>` still reaches legacy mode and prints "Failed to load mesh state" | — | S1, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the four scripts |
| 2 — something selects it | `main` reaches the new branch for every non-flag first word; the branch-removed mutant turns two scripts red |
| 3 — the caller can discover it | the usage text lists the subcommands |
| 4 — it is used | every invocation of the binary |

## Mutation Log
- 2026-11-16 · d246b4a* · mutant killed · exit 1 · `main.go` · typo falls into legacy mode: cli-unknown-command and cli-bare-usage red · acceptance-sha256:919567ecc5a69a9cd04800fadecfbdc95f6e1ed280205d47ed11b95577de4893
- 2026-11-16 · d246b4a* · mutant killed · exit 1 · `main.go` · trailing --version hijacks a subcommand: cli-version-position red · acceptance-sha256:919567ecc5a69a9cd04800fadecfbdc95f6e1ed280205d47ed11b95577de4893
- 2026-11-16 · d246b4a* · mutant killed · exit 1 · `main.go` · legacy flags refused: cli-legacy-flag red · acceptance-sha256:919567ecc5a69a9cd04800fadecfbdc95f6e1ed280205d47ed11b95577de4893
- 2026-11-16 · d246b4a* · mutant killed · exit 1 · `main.go` · help falls to the typo branch and exits 2: cli-bare-usage red · acceptance-sha256:919567ecc5a69a9cd04800fadecfbdc95f6e1ed280205d47ed11b95577de4893

## Invariants

- Every existing subcommand and `-v`, `--version`, `version` behave as before; `TestVersionFlag` (`main_test.go:27`) and the existing scripts stay green.
- The legacy centralized flags keep working when the first argument starts with `-` (Decision 4 of ADR-0007 keeps the legacy mode; this task does not remove it).
- No flag is added and no default path changes.

## Risks

- A script that calls `wgmesh <word>` expecting legacy mode to fail with exit 1 now sees exit 2; the exit code for input that never worked changes.
- `main.go` is also edited by ADR-0002 T10 (`--accept-routes`), hence the dependency on ADR-0002's last task.

## Stop Condition

Stop and ask if a documented invocation (README, `docs/`, packaging unit files, `grep -rn "wgmesh " packaging deploy`) uses a bare word that this change would now refuse.

## Out of Scope

- #873 items 3 to 6 (deferred: docs/adr/BACKLOG.md).
- Removing the legacy centralized mode (permanent: boundary: a feature drop, M's decision).

## Verification Log
- 2026-11-16 · d246b4a* · exit 1 · `set -o pipefail …` · acceptance-sha256:919567ecc5a69a9cd04800fadecfbdc95f6e1ed280205d47ed11b95577de4893 · ms:3983 · test-lock-sha256:e1e4d5eaf763e0ff2abb1b0b5ac8716e5452dd77caad8cc338a96f3f03571e0c · test-lock-file:docs/adr/locks/e1e4d5eaf763e0ff2abb1b0b5ac8716e5452dd77caad8cc338a96f3f03571e0c.lock
  ```
  --- last 10 line(s) of stderr (of 106 after folding 106 raw)
          FAIL: testdata/script/cli-version-position.txtar:8: unexpected command success

  --- FAIL: TestScript (0.00s)
      --- FAIL: TestScript/cli-bare-usage (0.01s)
      --- PASS: TestScript/cli-legacy-flag (0.01s)
      --- FAIL: TestScript/cli-unknown-command (0.01s)
      --- FAIL: TestScript/cli-version-position (0.02s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh	0.026s
  FAIL
  ```
- 2026-11-16 · d246b4a* · exit 0 · `set -o pipefail …` · acceptance-sha256:919567ecc5a69a9cd04800fadecfbdc95f6e1ed280205d47ed11b95577de4893 · ms:28266
- 2026-11-16 · d246b4a* · exit 0 · `set -o pipefail …` · acceptance-sha256:919567ecc5a69a9cd04800fadecfbdc95f6e1ed280205d47ed11b95577de4893 · ms:29256
- 2026-11-16 · d246b4a* · exit 0 · `set -o pipefail …` · acceptance-sha256:919567ecc5a69a9cd04800fadecfbdc95f6e1ed280205d47ed11b95577de4893 · ms:30207
- 2026-11-16 · d246b4a* · exit 0 · `set -o pipefail …` · acceptance-sha256:919567ecc5a69a9cd04800fadecfbdc95f6e1ed280205d47ed11b95577de4893 · ms:30294
- 2026-11-16 · d246b4a* · exit 0 · `set -o pipefail …` · acceptance-sha256:919567ecc5a69a9cd04800fadecfbdc95f6e1ed280205d47ed11b95577de4893 · ms:26645
