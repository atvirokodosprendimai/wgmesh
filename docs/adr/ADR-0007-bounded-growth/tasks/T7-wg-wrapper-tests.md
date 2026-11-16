# Task ADR-0007-T7: First tests of the `wg` wrappers through the fake-binary seam (#833, no target)

**Depends-on:** T1
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** none
**Consumes:** `wireguard.SetWGPath`, `runWG` (T1)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`SetPeer`, `RemovePeer`, `GenerateKeyPair`, `GenerateWgQuickConfig` and `shellQuote` have tests that can fail, and `pkg/wireguard` coverage is read before and after without a target.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/wireguard/apply_test.go` | add | `SetPeer` and `RemovePeer` arguments, the preshared key on stdin, `shellQuote` |
| `pkg/wireguard/keys_test.go` | add | `GenerateKeyPair` with a fake `genkey` and `pubkey`, and a failing one |
| `pkg/wireguard/persist_test.go` | add | `GenerateWgQuickConfig` with and without routes |

Selection: tests only; they run in `go test ./pkg/wireguard`.

## Ordered Steps

1. [S1] Add the seven tests; they test behaviour that exists after T1, so the count guard in the fence is what is red until they are written; for each test, break the line it pins by hand once and see it fail, and delete a test that does not. Run `go test -cover ./pkg/wireguard` and record the percentage, the date and the commit in the commit message (20.4% on 2026-11-15 at `b8b83f3` before T1).
2. [S2] [proof: acceptance] Fix only what a test shows to be wrong, one commit per finding; no refactor.
3. [S3] [proof: human: a person reads the coverage percentage before and after and records both] Run `go test -cover ./pkg/wireguard` again and record the number; no minimum is asserted.
4. [S4] Run the mutants with `adr-verify --mutant`: the preshared key moved from stdin to argv; `persistent-keepalive` dropped from `SetPeer`; `shellQuote` returning its input; `GenerateKeyPair` ignoring a `pubkey` failure. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/wireguard -run '^(TestSetPeerPassesPresharedKeyOnStdinNotArgv|TestSetPeerArgsOrder|TestRemovePeerArgs|TestGenerateKeyPairParsesGenkeyAndPubkey|TestGenerateKeyPairReportsFailure|TestGenerateWgQuickConfigRoutes)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 6 ] && go test -race ./pkg/wireguard -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestSetPeerPassesPresharedKeyOnStdinNotArgv` | `pkg/wireguard/apply_test.go` | a non-zero key appears on the fake `wg`'s stdin and nowhere in its argument list | — | S1 |
| `TestSetPeerArgsOrder` | `pkg/wireguard/apply_test.go` | `set <iface> peer <key> [endpoint ..] [allowed-ips ..] persistent-keepalive 25`, with empty endpoint and allowed IPs omitted | — | S1 |
| `TestRemovePeerArgs` | `pkg/wireguard/apply_test.go` | `set <iface> peer <key> remove`; a non-zero exit returns an error containing the output | — | S1 |
| `TestGenerateKeyPairParsesGenkeyAndPubkey` | `pkg/wireguard/keys_test.go` | the fake `genkey` output is trimmed, piped to `pubkey`, and both values returned | — | S1 |
| `TestGenerateKeyPairReportsFailure` | `pkg/wireguard/keys_test.go` | a failing `pubkey` returns an error and empty keys | — | S1 |
| `TestGenerateWgQuickConfigRoutes` | `pkg/wireguard/persist_test.go` | without routes there is no route `PostUp`/`PreDown` line (the `ip_forward` `PostUp` stays); with two routes each has two lines | — | S1 |
| `TestGenerateKeyPairReportsGenkeyFailure` | `pkg/wireguard/keys_test.go` | a failing `genkey` returns an error and empty keys (added after review) | — | S1 |
| `TestGenerateWgQuickConfigPeerWithoutOptionalFields` | `pkg/wireguard/persist_test.go` | a peer with only a public key writes only that line (added after review) | — | S1 |

Merge note (2026-11-16): ADR-0006 deleted `shellQuote` from `pkg/wireguard/apply.go` (the SSH path uses `ssh.Quote`), so `TestShellQuoteEscapesSingleQuote` and its mutant are gone; the same quoting case is covered by `TestQuoteNeutralisesHostileValues` in `pkg/ssh/quote_test.go`. The three `TestGenerateWgQuickConfig*` tests changed only to follow `GenerateWgQuickConfig` returning `(string, error)` (ADR-0004); their assertions are unchanged.

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the seven tests |
| 2 — something selects it | `go test ./...` in CI |
| 3 — the caller can discover it | not applicable: tests only |
| 4 — it is used | not applicable: tests only |

## Mutation Log
- 2026-11-15 · 84172d7* · mutant killed · exit 1 · `pkg/wireguard/apply.go` · preshared key moved to argv: TestSetPeerPassesPresharedKeyOnStdinNotArgv red · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514
- 2026-11-15 · 84172d7* · mutant killed · exit 1 · `pkg/wireguard/apply.go` · persistent-keepalive dropped: TestSetPeerArgsOrder red · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514
- 2026-11-15 · 84172d7* · mutant killed · exit 1 · `pkg/wireguard/apply.go` · shellQuote escapes nothing: TestShellQuoteEscapesSingleQuote red · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514
- 2026-11-15 · 84172d7* · mutant killed · exit 1 · `pkg/wireguard/keys.go` · pubkey failure ignored: TestGenerateKeyPairReportsFailure red · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514
- 2026-11-15 · 84172d7* · mutant killed · exit 1 · `pkg/wireguard/keys.go` · genkey failure ignored: TestGenerateKeyPairReportsGenkeyFailure red · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514
- 2026-11-15 · 84172d7* · mutant killed · exit 1 · `pkg/wireguard/persist.go` · empty endpoint written: TestGenerateWgQuickConfigPeerWithoutOptionalFields red · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514
- 2026-11-16 · c0ad7db* · mutant killed · exit 1 · `pkg/wireguard/apply.go` · preshared key moved to argv: TestSetPeerPassesPresharedKeyOnStdinNotArgv red · acceptance-sha256:072709e39e8de0a848c32f5e5bf4a9078e3b4f876a0fc69053e4312ba740bf5b
- 2026-11-16 · c0ad7db* · mutant killed · exit 1 · `pkg/wireguard/apply.go` · persistent-keepalive dropped: TestSetPeerArgsOrder red · acceptance-sha256:072709e39e8de0a848c32f5e5bf4a9078e3b4f876a0fc69053e4312ba740bf5b
- 2026-11-16 · c0ad7db* · mutant killed · exit 1 · `pkg/wireguard/keys.go` · pubkey failure ignored: TestGenerateKeyPairReportsFailure red · acceptance-sha256:072709e39e8de0a848c32f5e5bf4a9078e3b4f876a0fc69053e4312ba740bf5b
- 2026-11-16 · c0ad7db* · mutant killed · exit 1 · `pkg/wireguard/keys.go` · genkey failure ignored: TestGenerateKeyPairReportsGenkeyFailure red · acceptance-sha256:072709e39e8de0a848c32f5e5bf4a9078e3b4f876a0fc69053e4312ba740bf5b
- 2026-11-16 · c0ad7db* · mutant killed · exit 1 · `pkg/wireguard/persist.go` · empty endpoint written: TestGenerateWgQuickConfigPeerWithoutOptionalFields red · acceptance-sha256:072709e39e8de0a848c32f5e5bf4a9078e3b4f876a0fc69053e4312ba740bf5b

## Invariants

- No production file changes except a fix a test proves necessary.
- The preshared key never appears in an argument list (the property the first test pins, `apply.go:140-145` after T1).
- No coverage target is asserted anywhere (Decision 7 of ADR-0007).

## Risks

- The tests depend on a POSIX `sh`; the project releases for Linux and Darwin only.
- Coverage may rise less than expected because `ApplyFullConfiguration`, `ApplyPersistentConfig`, `UpdatePersistentConfig` and `RemovePersistentConfig` need an SSH client (`pkg/ssh/client.go` has no test file) and stay out.

## Stop Condition

Stop and ask if a test shows a defect in `SetPeer` or `GenerateKeyPair` that changes behaviour operators rely on.

## Out of Scope

- A coverage target or a CI floor, and tests for `pkg/discovery`, `pkg/ssh`, `cmd/analytics-dashboard` (deferred: docs/adr/BACKLOG.md).
- The SSH-driven functions of `apply.go` and `persist.go` (deferred: docs/adr/BACKLOG.md).

## Verification Log
- 2026-11-15 · 84172d7* · exit 1 · `set -o pipefail …` · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514 · ms:766 · test-lock-sha256:0ef16dc71762af5afdcb165c4b524389f80bc0f3908905c0984ecf0ec027164f · test-lock-file:docs/adr/locks/0ef16dc71762af5afdcb165c4b524389f80bc0f3908905c0984ecf0ec027164f.lock
  ```
  --- last 3 line(s) of stderr
  testing: warning: no tests to run
  PASS
  ok  	github.com/atvirokodosprendimai/wgmesh/pkg/wireguard	0.003s [no tests to run]
  ```
- 2026-11-15 · 84172d7* · exit 0 · `set -o pipefail …` · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514 · ms:5168
- 2026-11-15 · 84172d7* · exit 0 · `set -o pipefail …` · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514 · ms:4939
- 2026-11-15 · 84172d7* · exit 0 · `set -o pipefail …` · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514 · ms:4873
- 2026-11-15 · 84172d7* · exit 0 · `set -o pipefail …` · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514 · ms:5154
- 2026-11-15 · 84172d7* · exit 0 · `set -o pipefail …` · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514 · ms:4795
- 2026-11-15 · 84172d7* · exit 0 · `set -o pipefail …` · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514 · ms:4804
- 2026-11-15 · 84172d7* · exit 0 · `set -o pipefail …` · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514 · ms:4317
- 2026-11-15 · 5bc447c* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514 · ms:0 · test-lock-sha256:d7bbfc503e9f7a0412a02eb692bc4672788cb3b73a70be9ce2a954205111dd79 · test-lock-file:docs/adr/locks/d7bbfc503e9f7a0412a02eb692bc4672788cb3b73a70be9ce2a954205111dd79.lock · test-lock-kind:replace
- 2026-11-16 · b6554cc* · exit 0 · `set -o pipefail …` · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514 · ms:4179
- 2026-11-16 · 7004197* · exit 0 · `set -o pipefail …` · acceptance-sha256:04f6cebba12817d6d0e1ffecc2668d03e85cb9ddc687e545858686b9917f2514 · ms:4174
- 2026-11-16 · 820cfe4* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:072709e39e8de0a848c32f5e5bf4a9078e3b4f876a0fc69053e4312ba740bf5b · ms:0 · test-lock-sha256:80429201db54248f9c01b35ef920cf68208cea82194961238ce8938b9dfc2544 · test-lock-file:docs/adr/locks/80429201db54248f9c01b35ef920cf68208cea82194961238ce8938b9dfc2544.lock · test-lock-kind:replace
- 2026-11-16 · c0ad7db* · exit 0 · `set -o pipefail …` · acceptance-sha256:072709e39e8de0a848c32f5e5bf4a9078e3b4f876a0fc69053e4312ba740bf5b · ms:5544
- 2026-11-16 · c0ad7db* · exit 0 · `set -o pipefail …` · acceptance-sha256:072709e39e8de0a848c32f5e5bf4a9078e3b4f876a0fc69053e4312ba740bf5b · ms:5066
- 2026-11-16 · c0ad7db* · exit 0 · `set -o pipefail …` · acceptance-sha256:072709e39e8de0a848c32f5e5bf4a9078e3b4f876a0fc69053e4312ba740bf5b · ms:5072
- 2026-11-16 · c0ad7db* · exit 0 · `set -o pipefail …` · acceptance-sha256:072709e39e8de0a848c32f5e5bf4a9078e3b4f876a0fc69053e4312ba740bf5b · ms:4922
- 2026-11-16 · c0ad7db* · exit 0 · `set -o pipefail …` · acceptance-sha256:072709e39e8de0a848c32f5e5bf4a9078e3b4f876a0fc69053e4312ba740bf5b · ms:4938
- 2026-11-16 · c0ad7db* · exit 0 · `set -o pipefail …` · acceptance-sha256:072709e39e8de0a848c32f5e5bf4a9078e3b4f876a0fc69053e4312ba740bf5b · ms:4226
- 2026-11-16 · c0ad7db* · exit 0 · `set -o pipefail …` · acceptance-sha256:072709e39e8de0a848c32f5e5bf4a9078e3b4f876a0fc69053e4312ba740bf5b · ms:4042
