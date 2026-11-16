# Task ADR-0007-T8: One join-secret lookup for `join`, `status`, `qr`, `install-service` and `wgmesh service` (#891)

**Depends-on:** none
**Resolved:** ADR-0002 is merged on main (f3860c6, 2026-11-16; `git cat-file -e origin/main:pkg/node/spec_admit_test.go` exits 0)
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `secretFromFlagOrEnv(flagValue string) (string, error)` in `main.go`; `resolveSecret` in `service.go` calls it
**Consumes:** none
**Data dependency:** hermetic
**Proof map:** v1

## Goal

On a host set up from the package (unit with `WGMESH_SECRET_FILE=/etc/wgmesh/secret`), `wgmesh status`, `qr`, `install-service` and `wgmesh service add|list|remove` find the join secret the way `join` does: `--secret`, then `WGMESH_SECRET`, then the file named by `WGMESH_SECRET_FILE`, so the secret never has to appear on a command line.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `main.go` | edit | `secretFromFlagOrEnv` holds the lookup now inline in `joinCmd` (`main.go:351-363`); `joinCmd`, `statusCmd` (`:599-607`), `qrCmd` (`:680-686`) and `installServiceCmd` (`:783-801`) call it; the four usage texts name the two variables |
| `service.go` | edit | `resolveSecret` (`:346`) returns `normalizeSecret` of the helper's result, so `service add`, `list` and `remove` get the file lookup too; their "Error: --secret is required (or set WGMESH_SECRET)" text names `WGMESH_SECRET_FILE` |
| `main_test.go` | edit | unit tests of the helper (precedence, file trimming, unreadable file) |
| `service_test.go` | edit | `TestResolveSecretReadsFile` beside `TestResolveSecret` (`:91`) |
| `testdata/script/status-secret-env.txtar`, `status-secret-file.txtar`, `qr-secret-env.txtar`, `join-secret-file-missing.txtar` | add | `exec wgmesh ...` scripts run by `TestScript` (`main_test.go:20`) |

Selection: the four commands call the helper; the scripts run `status` and `qr` with no `--secret`, so removing one call turns its script red.
The four `.txtar` scripts (`status-secret-env`, `status-secret-file`, `qr-secret-env`, `join-secret-file-missing`) stay in the acceptance fence but are not Tests-table rows: adr-lint cannot hash a `.txtar` row, so the table names Go tests that say the same through a built binary (lesson of ADR-0007 T6). `main.go` line numbers here are those of 2026-11-15 and have drifted by about 8 to 20.

## Ordered Steps

1. [S1] Add the helper tests and the four scripts with a compiling stub (`secretFromFlagOrEnv` returns `flagValue, nil`) so the first run fails on assertions: today `wgmesh status` with only `WGMESH_SECRET` set exits 1 with "--secret is required". Run red.
2. [S2] Read `joinCmd` at `main.go:351-370` and move its lookup, unchanged in behaviour, into `secretFromFlagOrEnv`: flag value if non-empty, else `WGMESH_SECRET` if non-empty, else the file `WGMESH_SECRET_FILE` read and trimmed; a file that cannot be read returns an error that names the path; nothing set returns `""` and no error.
3. [S3] Use it in `joinCmd` (message and exit code unchanged), `statusCmd`, `qrCmd` and `installServiceCmd`; an error from the helper prints "Error reading secret file <path>: <err>" and exits 1 as `join` does today; update the four "--secret is required" usage texts to mention `WGMESH_SECRET` and `WGMESH_SECRET_FILE`, as `join` does.
4. [S4] Make `resolveSecret` in `service.go` call the helper and keep `normalizeSecret`; the file error exits 1 in the three `service` commands.
5. [S5] Run the mutants with `adr-verify --mutant`: the file branch removed from the helper; precedence swapped (env before flag); `statusCmd` not calling the helper; `resolveSecret` not calling the helper. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test . -run '^(TestSecretFromFlagOrEnvPrecedence|TestSecretFromFlagOrEnvFileTrimmed|TestSecretFromFlagOrEnvUnreadableFile|TestResolveSecretReadsFile|TestCommandsFindSecretFromEnvAndFile|TestJoinUnreadableSecretFileExitsOne)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c -- '^--- PASS') && [ "$n" -ge 6 ] && go test . -run '^TestScript/(status-secret-env|status-secret-file|qr-secret-env|join-secret-file-missing)$' -count=1 && go test -race . -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestSecretFromFlagOrEnvPrecedence` | `main_test.go` | flag beats `WGMESH_SECRET`, which beats `WGMESH_SECRET_FILE`; nothing set gives `""` and no error | — | S1, S2 |
| `TestSecretFromFlagOrEnvFileTrimmed` | `main_test.go` | a file holding the secret and a newline gives the secret without it | — | S1, S2 |
| `TestSecretFromFlagOrEnvUnreadableFile` | `main_test.go` | a missing file returns an error containing the path | — | S1, S2 |
| `TestResolveSecretReadsFile` | `service_test.go` | with only `WGMESH_SECRET_FILE` set `resolveSecret("")` returns the normalized secret (existing `TestResolveSecret` stays) | — | S1, S4 |
| `TestCommandsFindSecretFromEnvAndFile` | `main_test.go` | a built binary: `wgmesh status --json` finds the secret from `WGMESH_SECRET` and from `WGMESH_SECRET_FILE` (no flag) and prints the JSON of the `--secret` run; `wgmesh qr` with the variable prints a QR block, not "--secret is required" (the four scripts say the same in `testscript` form) | — | S1, S3 |
| `TestJoinUnreadableSecretFileExitsOne` | `main_test.go` | a built binary: `join` with an unreadable `WGMESH_SECRET_FILE` exits 1 and stderr names the path (behaviour kept) | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the helper and its tests |
| 2 — something selects it | five commands call it; the not-calling mutants turn a script or `TestResolveSecretReadsFile` red |
| 3 — the caller can discover it | the usage text of each command names both variables |
| 4 — it is used | every `status`, `qr`, `install-service` and `service` call on a packaged host |

## Mutation Log
- 2026-11-16 · 32664af* · mutant killed · exit 1 · `main.go` · file never read: TestSecretFromFlagOrEnvFileTrimmed red · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96
- 2026-11-16 · 32664af* · mutant killed · exit 1 · `main.go` · precedence swapped: TestSecretFromFlagOrEnvPrecedence red · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96
- 2026-11-16 · 32664af* · mutant killed · exit 1 · `main.go` · status ignores env/file: status-secret-env red · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96
- 2026-11-16 · 32664af* · mutant killed · exit 1 · `main.go` · qr ignores env: qr-secret-env red · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96
- 2026-11-16 · 32664af* · mutant killed · exit 1 · `service.go` · service commands ignore env/file: TestResolveSecretReadsFile red · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96
- 2026-11-16 · 32664af* · mutant killed · exit 1 · `main.go` · join ignores env/file: join-secret-file-missing / join tests red · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96
- 2026-11-16 · 32664af* · mutant survived · exit 0 · `main.go` · install-service ignores env/file: ? · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96
  ```
  the fence passed with the mechanism broken; it may not materialize, compile, load, or assert on the changed path
  ```
- 2026-11-16 · a686785* · mutant killed · exit 1 · `main.go` · file never read: TestSecretFromFlagOrEnvFileTrimmed red · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96
- 2026-11-16 · a686785* · mutant killed · exit 1 · `main.go` · precedence swapped: TestSecretFromFlagOrEnvPrecedence red · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96
- 2026-11-16 · a686785* · mutant killed · exit 1 · `main.go` · status ignores env/file: status-secret-env red · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96
- 2026-11-16 · a686785* · mutant killed · exit 1 · `main.go` · qr ignores env: qr-secret-env red · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96
- 2026-11-16 · a686785* · mutant killed · exit 1 · `service.go` · service commands ignore env/file: TestResolveSecretReadsFile red · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96
- 2026-11-16 · a686785* · mutant killed · exit 1 · `main.go` · join ignores env/file: join tests red · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96
- 2026-11-16 · a686785* · mutant killed · exit 1 · `main.go` · install-service ignores env/file: install-service subtest red · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96
- 2026-11-16 · a686785* · mutant killed · exit 1 · `main.go` · blank env counts as a secret: precedence test red · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96
- 2026-11-16 · a686785* · mutant killed · exit 1 · `main.go` · empty file gives a blank secret: FileTrimmed empty case red · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96

## Invariants

- `--secret` on the command line still wins over both variables.
- `join` keeps its messages and exit codes; a secret from the environment is not normalized in `join` (the daemon config does that), `resolveSecret` keeps normalizing.
- No flag is added, no default path changes, the secret is never printed.
- The helper is the only place that reads `WGMESH_SECRET` and `WGMESH_SECRET_FILE`: `grep -n 'WGMESH_SECRET' main.go service.go` lists the helper and text only.

## Risks

- `main.go` conflicts with ADR-0002 T10 and with PR #893 (`installServiceCmd`); the task starts after both are on main.
- `status` run as another user cannot read `/etc/wgmesh/secret` (mode 0600): it now prints the file error instead of "--secret is required"; the message names the path, so the operator sees why.

## Stop Condition

Stop and ask if a documented invocation relies on `WGMESH_SECRET_FILE` being ignored by `status`, `qr` or `install-service`, or if `service.go` has a caller of `resolveSecret` that must not read a file.

## Out of Scope

- The documentation gaps named in issue #891 (a Linux page on `/etc/wgmesh/secret` and `/etc/wgmesh/secret.env`; `eidos/spec - daemon support - ...systemd.md:51` still describing `${WGMESH_SECRET}` in the unit) (deferred: docs/adr/BACKLOG.md).
- `test-peer` (`main.go:499`), a debug command that takes `--secret` only (permanent: boundary: it is a developer tool and #891 does not name it).

## Verification Log
- 2026-11-16 · 32664af* · exit 1 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:5201 · test-lock-sha256:dc52ad422b1eb4fcb8b5a27f717fd5d28ba0e5743a0ef919d7c59c797dcbad9f · test-lock-file:docs/adr/locks/dc52ad422b1eb4fcb8b5a27f717fd5d28ba0e5743a0ef919d7c59c797dcbad9f.lock
  ```
  --- last 10 line(s) of stderr (of 44 after folding 44 raw)
      --- FAIL: TestCommandsFindSecretFromEnvAndFile/status_WGMESH_SECRET_FILE (0.01s)
      --- FAIL: TestCommandsFindSecretFromEnvAndFile/qr_WGMESH_SECRET_FILE (0.01s)
  === RUN   TestJoinUnreadableSecretFileExitsOne
  --- PASS: TestJoinUnreadableSecretFileExitsOne (1.79s)
  === RUN   TestResolveSecretReadsFile
      service_test.go:115: expected file-value, got ""
  --- FAIL: TestResolveSecretReadsFile (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh	3.583s
  FAIL
  ```
- 2026-11-16 · 32664af* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:41907
- 2026-11-16 · 32664af* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:39476
- 2026-11-16 · 32664af* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:40960
- 2026-11-16 · 32664af* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:38284
- 2026-11-16 · 32664af* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:41156
- 2026-11-16 · 32664af* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:46792
- 2026-11-16 · 32664af* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:42150
- 2026-11-16 · a58b98f* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:43436
- 2026-11-16 · a686785* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:0 · test-lock-sha256:18bb6fa70f9a6c14ff104541ebbcd48675a4513efa24c40fa11f772b71b3082e · test-lock-file:docs/adr/locks/18bb6fa70f9a6c14ff104541ebbcd48675a4513efa24c40fa11f772b71b3082e.lock · test-lock-kind:replace
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:40385
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:40103
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:39061
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:38673
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:36010
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:37499
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:35675
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:34418
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:39050
- 2026-11-16 · a686785* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:35003
- 2026-11-16 · c0ad7db* · exit 0 · `set -o pipefail …` · acceptance-sha256:97d9fa802eccfedb6835021b2b956b1b9390a2be7e3110db9c584f25dee10b96 · ms:29820
