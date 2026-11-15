# Task ADR-0006-T1: Add `ssh.Quote`, `ssh.Cmd` and a Linux interface grammar that ignores GOOS

**Depends-on:** none
**Covers:** none — no spec
**Estimated scope:** S (single package pair)
**Owner:** unassigned
**Produces:** `ssh.Quote`, `ssh.Cmd`, `ifname.ValidateLinux`
**Consumes:** none
**Data dependency:** hermetic
**Proof map:** v1

## Goal

One function decides how a value becomes shell text for a node (`ssh.Quote`, `ssh.Cmd`), a value made only of safe characters is returned unchanged, and one function validates an interface name against the Linux grammar whatever the operator's `runtime.GOOS` is.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/ssh/quote.go` | add | `Quote(s string) string` (safe set `[A-Za-z0-9._:/,@%+=\[\]-]` unchanged, else single-quoted with `'` written as `'\''`, empty string `''`) and `Cmd(name string, args ...string) string` (`name` joined to each quoted argument with one space) |
| `pkg/ssh/quote_test.go` | add | tests below |
| `pkg/ifname/validate.go` | edit | `ValidateLinux(name string) error` holds the Linux branch (length, `linuxRegex`); `Validate` calls it in its default case; the empty string stays valid in `Validate` and is an error in `ValidateLinux` (a deploy needs a name) |
| `pkg/ifname/validate_test.go` | edit | tests below |

Selection: nothing calls these yet; T2 and T3 do. The mutant that makes `Cmd` skip `Quote` turns `TestCmdQuotesEachArgument` red.

## Ordered Steps

1. [S1] Add the tests and compiling stubs (`Quote` returns its argument, `Cmd` joins unquoted, `ValidateLinux` returns nil) so the first run fails on assertions. Run red.
2. [S2] Implement `Quote` and `Cmd`.
3. [S3] Implement `ValidateLinux` and make `Validate` call it for the non-darwin branch; the existing `Validate` tests must pass unchanged.
4. [S4] Run `go test ./pkg/ifname -count=1` and record the exit line in the commit message: the daemon's existing interface-name tests are the proof that `Validate` did not change. [proof: acceptance]
5. [S5] Run the mutants with `adr-verify --mutant`: `Quote` returns its input; `Quote` does not escape an embedded single quote; `Cmd` skips `Quote`; `ValidateLinux` reads `runtime.GOOS`. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/ssh -run '^(TestQuoteLeavesSafeValuesUnchanged|TestQuoteNeutralisesHostileValues|TestQuoteRoundTripsThroughSh|TestCmdQuotesEachArgument)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 4 ] \
 && b=$(go test ./pkg/ifname -run '^(TestValidateLinuxIgnoresGOOS|TestValidateLinuxRejectsEmpty)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -ge 2 ] \
 && go test -race ./pkg/ssh ./pkg/ifname -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestQuoteLeavesSafeValuesUnchanged` | `pkg/ssh/quote_test.go` | `wg0`, `10.99.0.2/32`, `192.168.1.10:51820`, `[fd00::1]:51820`, a 44-character base64 key and `0.0.0.0/0` come back byte for byte | — | S1, S2 |
| `TestQuoteNeutralisesHostileValues` | `pkg/ssh/quote_test.go` | `a; rm -rf /`, `$(id)`, a backtick, a newline, a double quote, a single quote, a space, a pipe, an ampersand, a redirect `>x` and the empty string each come back wrapped so that the result is one shell word | — | S1, S2 |
| `TestQuoteRoundTripsThroughSh` | `pkg/ssh/quote_test.go` | for each hostile value, `/bin/sh -c 'printf %s ' + Quote(v)` prints exactly `v` (fails, does not skip, when `sh` is missing) | — | S1, S2 |
| `TestCmdQuotesEachArgument` | `pkg/ssh/quote_test.go` | `Cmd("wg", "set", "wg0; id", "peer")` is `wg set 'wg0; id' peer` | — | S1, S2 |
| `TestValidateLinuxIgnoresGOOS` | `pkg/ifname/validate_test.go` | `wg0` and `mesh-1` pass, `0bad`, `a b`, `a;b` and a 16-character name fail, whatever `runtime.GOOS` is | — | S1, S3 |
| `TestValidateLinuxRejectsEmpty` | `pkg/ifname/validate_test.go` | the empty name is an error in `ValidateLinux` and still valid in `Validate` | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the six tests |
| 2 — something selects it | not yet; T2 and T3 call it, and the `Cmd` mutant turns `TestCmdQuotesEachArgument` red |
| 3 — the caller can discover it | exported with doc comments; T6 names it in the spec |
| 4 — it is used | from T2 on: every remote command |

## Mutation Log

## Invariants

- `ifname.Validate` returns what it returned before for every input on every `GOOS` (the daemon's tests are unchanged).
- A value in the safe set is never wrapped, so a command for a valid state is the same string as today.
- `pkg/ssh` imports nothing from `pkg/mesh`, `pkg/wireguard`, `pkg/node`, `pkg/daemon` or `pkg/discovery`.

## Risks

- The safe set is a judgement about what a shell leaves alone (`%` and `@` are inert in `sh`, and `wg-quick@iface` needs `@`); `TestQuoteRoundTripsThroughSh` is the check, but only for the values it lists.
- Test doubles: none; the round-trip test runs the real `/bin/sh`.

## Stop Condition

Stop and ask if an existing `ifname` test needs a changed expectation, or if a command the code sends today contains a character outside the safe set for a valid value (that would make the byte-identical invariant false).

## Out of Scope

- Moving any command to `Cmd` (T2).
- Changing `routes.ifaceNameRe` (permanent: boundary: it checks names read from a node's output, not state).

## Verification Log
