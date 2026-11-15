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
| `pkg/ssh/quote.go` | add | `Quote(s string) string` (safe set `[A-Za-z0-9._:/,@%+=-]` unchanged (`[` and `]` are glob characters in `sh`, so they are quoted; review 2026-11-15), else single-quoted with `'` written as `'\''`, empty string `''`) and `Cmd(name string, args ...string) string` (`name` joined to each quoted argument with one space) |
| `pkg/ssh/quote_test.go` | add | tests below |
| `pkg/ifname/validate.go` | edit | `ValidateLinux(name string) error` holds the Linux branch (length, `linuxRegex`); `Validate` calls it in its default case; the empty string stays valid in `Validate` and is an error in `ValidateLinux` (a deploy needs a name) |
| `pkg/ifname/validate_test.go` | edit | tests below |

Selection: nothing calls these yet; T2 and T3 do. The mutant that makes `Cmd` skip `Quote` turns `TestCmdQuotesEachArgument` red.

## Ordered Steps

1. [S1] Add the tests and compiling stubs (`Quote` returns its argument, `Cmd` joins unquoted, `ValidateLinux` returns nil) so the first run fails on assertions. Run red.
2. [S2] Implement `Quote` and `Cmd`.
3. [S3] Implement `ValidateLinux` and make `Validate` call it for the non-darwin branch; the existing `Validate` tests must pass unchanged.
4. [S4] Run `go test ./pkg/ifname -count=1` and record the exit line in the commit message: the daemon's existing interface-name tests are the proof that `Validate` did not change. [proof: acceptance]
5. [S5] Run the mutants with `adr-verify --mutant`: `Quote` returns its input; `Quote` does not escape an embedded single quote; `Cmd` skips `Quote`; `ValidateLinux` skips the grammar; `ValidateLinux` accepts one character more than `MaxLen` (a mutant that reads `runtime.GOOS` cannot be killed on a Linux host: GOOS independence is structural, `runtime` appears only in `Validate`). [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/ssh -run '^(TestQuoteLeavesSafeValuesUnchanged|TestQuoteNeutralisesHostileValues|TestQuoteRoundTripsThroughSh|TestCmdQuotesEachArgument)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -eq 4 ] \
 && b=$(go test ./pkg/ifname -run '^(TestValidateLinuxIgnoresGOOS|TestValidateLinuxRejectsEmpty)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -eq 2 ] \
 && go test -race ./pkg/ssh ./pkg/ifname -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestQuoteLeavesSafeValuesUnchanged` | `pkg/ssh/quote_test.go` | `wg0`, `10.99.0.2/32`, `192.168.1.10:51820`, a 44-character base64 key and `0.0.0.0/0` come back byte for byte | — | S1, S2 |
| `TestQuoteNeutralisesHostileValues` | `pkg/ssh/quote_test.go` | `a; touch PWNED`, `$(id)`, a backtick, a newline, a double quote, a single quote, a space, a pipe, an ampersand, a redirect `>x` and the empty string, the glob characters `*`, `?` and `[x]` and the IPv6 endpoint `[fd00::1]:51820` each come back wrapped so that the result is one shell word | — | S1, S2 |
| `TestQuoteRoundTripsThroughSh` | `pkg/ssh/quote_test.go` | for each hostile value, `/bin/sh -c 'printf %s ' + Quote(v)` prints exactly `v`, run in an empty temp directory that holds files named `x` and `PWNED` so a glob would expand and a command would show (fails, does not skip, when `sh` is missing) | — | S1, S2 |
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
- 2026-11-15 · 02692f3* · mutant killed · exit 1 · `pkg/ssh/quote.go` · Quote never wraps: TestQuoteNeutralisesHostileValues and the round trip must go red · acceptance-sha256:fa989fff07d069d03d0bb90979cecb5ee07f7281093f5694a3f0fbc0269867af
- 2026-11-15 · 02692f3* · mutant killed · exit 1 · `pkg/ssh/quote.go` · an embedded single quote is not escaped: TestQuoteNeutralisesHostileValues and TestQuoteRoundTripsThroughSh must go red · acceptance-sha256:fa989fff07d069d03d0bb90979cecb5ee07f7281093f5694a3f0fbc0269867af
- 2026-11-15 · 02692f3* · mutant killed · exit 1 · `pkg/ssh/quote.go` · Cmd skips Quote: TestCmdQuotesEachArgument must go red · acceptance-sha256:fa989fff07d069d03d0bb90979cecb5ee07f7281093f5694a3f0fbc0269867af
- 2026-11-15 · 02692f3* · mutant killed · exit 1 · `pkg/ifname/validate.go` · ValidateLinux accepts 16 characters: TestValidateLinuxIgnoresGOOS must go red · acceptance-sha256:fa989fff07d069d03d0bb90979cecb5ee07f7281093f5694a3f0fbc0269867af
- 2026-11-15 · 02692f3* · mutant killed · exit 1 · `pkg/ifname/validate.go` · ValidateLinux skips the grammar: TestValidateLinuxIgnoresGOOS must go red · acceptance-sha256:fa989fff07d069d03d0bb90979cecb5ee07f7281093f5694a3f0fbc0269867af
- 2026-11-15 · 15230e9* · mutant killed · exit 1 · `pkg/ssh/quote.go` · Quote never wraps: hostile-value tests and the round trip must go red · acceptance-sha256:add5507172bc31016b7187b4b1b55fefd863d68084cc1f17a3f9cf7ddee74df2
- 2026-11-15 · 15230e9* · mutant killed · exit 1 · `pkg/ssh/quote.go` · an embedded single quote is not escaped: TestQuoteNeutralisesHostileValues and TestQuoteRoundTripsThroughSh must go red · acceptance-sha256:add5507172bc31016b7187b4b1b55fefd863d68084cc1f17a3f9cf7ddee74df2
- 2026-11-15 · 15230e9* · mutant killed · exit 1 · `pkg/ssh/quote.go` · Cmd skips Quote: TestCmdQuotesEachArgument must go red · acceptance-sha256:add5507172bc31016b7187b4b1b55fefd863d68084cc1f17a3f9cf7ddee74df2
- 2026-11-15 · 15230e9* · mutant killed · exit 1 · `pkg/ifname/validate.go` · ValidateLinux accepts 16 characters: TestValidateLinuxIgnoresGOOS must go red · acceptance-sha256:add5507172bc31016b7187b4b1b55fefd863d68084cc1f17a3f9cf7ddee74df2
- 2026-11-15 · 15230e9* · mutant killed · exit 1 · `pkg/ifname/validate.go` · ValidateLinux skips the grammar: TestValidateLinuxIgnoresGOOS must go red · acceptance-sha256:add5507172bc31016b7187b4b1b55fefd863d68084cc1f17a3f9cf7ddee74df2
- 2026-11-15 · 15230e9* · mutant killed · exit 1 · `pkg/ssh/quote.go` · brackets are glob characters again: TestQuoteNeutralisesHostileValues ([x], [fd00::1]:51820) must go red · acceptance-sha256:add5507172bc31016b7187b4b1b55fefd863d68084cc1f17a3f9cf7ddee74df2
- 2026-11-15 · 15230e9* · mutant killed · exit 1 · `pkg/ifname/validate.go` · ValidateLinux rejects 15 characters: TestValidateLinuxIgnoresGOOS accepted-15 case must go red · acceptance-sha256:add5507172bc31016b7187b4b1b55fefd863d68084cc1f17a3f9cf7ddee74df2

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
- 2026-11-15 · 02692f3* · exit 1 · `set -o pipefail …` · acceptance-sha256:fa989fff07d069d03d0bb90979cecb5ee07f7281093f5694a3f0fbc0269867af · ms:1839 · test-lock-sha256:e16727575df47094b9f7df72fa1e6812ce1b818707bcc4d54573f1e128279959 · test-lock-file:docs/adr/locks/e16727575df47094b9f7df72fa1e6812ce1b818707bcc4d54573f1e128279959.lock
  ```
  --- last 10 line(s) of stderr (of 35 after folding 35 raw)
      quote_test.go:45: Quote("") = "", want single-quoted
      quote_test.go:49: Quote("") = "", want ''
      quote_test.go:52: Quote(it's) = "it's", want "'it'\\''s'"
  --- FAIL: TestQuoteNeutralisesHostileValues (0.00s)
  === NAME  TestQuoteRoundTripsThroughSh
      quote_test.go:62: sh for "a; rm -rf /": exit status 1
  --- FAIL: TestQuoteRoundTripsThroughSh (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/ssh	0.007s
  FAIL
  ```
- 2026-11-15 · 02692f3* · exit 0 · `set -o pipefail …` · acceptance-sha256:fa989fff07d069d03d0bb90979cecb5ee07f7281093f5694a3f0fbc0269867af · ms:11478
- 2026-11-15 · 02692f3* · exit 0 · `set -o pipefail …` · acceptance-sha256:fa989fff07d069d03d0bb90979cecb5ee07f7281093f5694a3f0fbc0269867af · ms:7814
- 2026-11-15 · 02692f3* · exit 0 · `set -o pipefail …` · acceptance-sha256:fa989fff07d069d03d0bb90979cecb5ee07f7281093f5694a3f0fbc0269867af · ms:11382
- 2026-11-15 · 02692f3* · exit 0 · `set -o pipefail …` · acceptance-sha256:fa989fff07d069d03d0bb90979cecb5ee07f7281093f5694a3f0fbc0269867af · ms:7315
- 2026-11-15 · 02692f3* · exit 0 · `set -o pipefail …` · acceptance-sha256:fa989fff07d069d03d0bb90979cecb5ee07f7281093f5694a3f0fbc0269867af · ms:7110
- 2026-11-15 · 02692f3* · exit 0 · `set -o pipefail …` · acceptance-sha256:fa989fff07d069d03d0bb90979cecb5ee07f7281093f5694a3f0fbc0269867af · ms:5962
- 2026-11-15 · 15230e9* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:add5507172bc31016b7187b4b1b55fefd863d68084cc1f17a3f9cf7ddee74df2 · ms:2 · test-lock-sha256:6e91aff52c51189ccb6211132a4630b58663626409f0726330b121a1635509f6 · test-lock-file:docs/adr/locks/6e91aff52c51189ccb6211132a4630b58663626409f0726330b121a1635509f6.lock · test-lock-kind:replace
- 2026-11-15 · 15230e9* · exit 0 · `set -o pipefail …` · acceptance-sha256:add5507172bc31016b7187b4b1b55fefd863d68084cc1f17a3f9cf7ddee74df2 · ms:7640
- 2026-11-15 · 15230e9* · exit 0 · `set -o pipefail …` · acceptance-sha256:add5507172bc31016b7187b4b1b55fefd863d68084cc1f17a3f9cf7ddee74df2 · ms:5899
- 2026-11-15 · 15230e9* · exit 0 · `set -o pipefail …` · acceptance-sha256:add5507172bc31016b7187b4b1b55fefd863d68084cc1f17a3f9cf7ddee74df2 · ms:5166
- 2026-11-15 · 15230e9* · exit 0 · `set -o pipefail …` · acceptance-sha256:add5507172bc31016b7187b4b1b55fefd863d68084cc1f17a3f9cf7ddee74df2 · ms:4869
- 2026-11-15 · 15230e9* · exit 0 · `set -o pipefail …` · acceptance-sha256:add5507172bc31016b7187b4b1b55fefd863d68084cc1f17a3f9cf7ddee74df2 · ms:5744
- 2026-11-15 · 15230e9* · exit 0 · `set -o pipefail …` · acceptance-sha256:add5507172bc31016b7187b4b1b55fefd863d68084cc1f17a3f9cf7ddee74df2 · ms:6397
- 2026-11-15 · 15230e9* · exit 0 · `set -o pipefail …` · acceptance-sha256:add5507172bc31016b7187b4b1b55fefd863d68084cc1f17a3f9cf7ddee74df2 · ms:6176
- 2026-11-15 · 15230e9* · exit 0 · `set -o pipefail …` · acceptance-sha256:add5507172bc31016b7187b4b1b55fefd863d68084cc1f17a3f9cf7ddee74df2 · ms:5309
