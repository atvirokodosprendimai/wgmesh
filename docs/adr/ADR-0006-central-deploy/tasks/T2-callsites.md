# Task ADR-0006-T2: Send every remote command through `ssh.Cmd`, render wg-quick safely, and fail the build on a raw one

**Depends-on:** T1
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** every remote command built with `ssh.Cmd`; `GenerateWgQuickConfig` returning `(string, error)`; `pkg/ssh/sshtest`; `ssh.RouteArgs`; `TestNoUnquotedRemoteCommand`
**Consumes:** `ssh.Quote`, `ssh.Cmd` (T1)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The 29 command builders and 7 appends of the class audit send their values quoted, the wg-quick text refuses a value that cannot be quoted, the bytes sent for a valid state are unchanged, and a test fails when a new raw command appears in `pkg/ssh`, `pkg/wireguard` or `pkg/mesh`.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/ssh/client.go` | edit | `WriteFile` builds `cat > <path> && chmod <mode> <path>` with `Cmd`-style quoting of the path |
| `pkg/ssh/routes.go` | edit | `GetCurrentRoutes`, `RestoreConnectedRoute`, `ApplyRouteDiff` (`:18`, `:74`, `:122`, `:125`, `:145`, `:148`); also adds the exported helper `RouteArgs(verb, network, gateway, iface) []string` (the `ip route <verb>` arguments, `via` only for a non-empty gateway) used by `routes.go`, `wireguard.go`, `persist.go` and `deploy.go` |
| `pkg/ssh/wireguard.go` | edit | `UpdateRoutingTable`, `UpdateRoutingTableWithGateways` (unreachable, `:80`, `:100`) and `EnsureWireGuardInstalled` (its command slice becomes three constants so the guard can resolve each argument) |
| `pkg/wireguard/config.go` | edit | `GetCurrentConfig`, `ApplyDiff`, `addOrUpdatePeer` (`:50`, `:196`, `:220-239`) |
| `pkg/wireguard/persist.go` | edit | `GenerateWgQuickConfig` returns an error and quotes route values with `ssh.Quote`; `systemctl`, `rm` lines; its two callers here (`:71`, `:102`) |
| `pkg/wireguard/apply.go` | edit | `shellQuote` is deleted and `ApplyFullConfiguration` (unreachable) uses `Cmd` |
| `pkg/mesh/deploy.go` | edit | the fallback route commands (`:225`, `:228`) and the config file write (`:89-90`) |
| `pkg/ssh/sshtest/server.go` | add | an in-process SSH server (`golang.org/x/crypto/ssh`, no client auth) that records every `exec` command and answers from a table; `Start(t, replies)` writes its host key line to a temp file and sets `WGMESH_SSH_KNOWN_HOSTS` with `t.Setenv` |
| `pkg/ssh/remote_guard_test.go` | add | `TestNoUnquotedRemoteCommand` |
| `pkg/wireguard/persist_test.go`, `pkg/ssh/remote_e2e_test.go` | add | tests below |
| `testdata/central/commands-valid.golden` | add | the command text recorded from the unchanged code (S1) |

Selection: the guard test parses the three packages, so deleting a `Cmd` call turns it red; `TestHostileStateCommandsAreOneWordEach` drives the functions the deploy calls, so a site that still interpolates raw turns it red.

## Ordered Steps

1. [S1] Before changing any builder, run the existing functions against the `sshtest` server with the valid fixture (interface `wg0`, two peers, one routable network) and write every received command and the `GenerateWgQuickConfig` text to `testdata/central/commands-valid.golden`. Then add the other tests with compiling stubs and run red. The golden test passes at this point by construction (it records unchanged code); it is the byte-equality oracle for S3, not a red test.
2. [S2] Add `pkg/ssh/sshtest` and `TestNoUnquotedRemoteCommand` (red: the 16 reachable and 13 unreachable builders fail it).
3. [S3] Migrate every builder of the class audit to `Cmd` or `Quote`; `GenerateWgQuickConfig` gains the error (a value with `\n`, `\r` or NUL is refused with the field named); move its three callers. `TestRecordedCommandsMatchGolden` stays green.
4. [S4] Run `grep -rnE 'Sprintf\("[^"]*\b(ip|wg|systemctl|rm|cat)\b' --include=*.go pkg/ssh pkg/wireguard pkg/mesh | grep -v _test.go | wc -l` and record the new count in the commit message: the lines that remain must each be a `Cmd` argument or a log line, and the guard test is the judge. [proof: human: the count is a hint for the reviewer, the guard test is the gate]
5. [S5] Run the mutants with `adr-verify --mutant`: one builder goes back to a raw `Sprintf`; one builder concatenates a variable; `Quote` is skipped in the route `PostUp` text; the newline refusal is removed. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/ssh -run '^(TestNoUnquotedRemoteCommand|TestHostileStateCommandsAreOneWordEach|TestRecordedCommandsMatchGolden)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 3 ] \
 && b=$(go test ./pkg/wireguard -run '^(TestWgQuickRefusesControlCharacters|TestWgQuickQuotesRouteValues|TestWgQuickValidConfigUnchanged)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -ge 3 ] \
 && go build ./... && go vet ./... && go test -race ./pkg/ssh/... ./pkg/wireguard ./pkg/mesh -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestNoUnquotedRemoteCommand` | `pkg/ssh/remote_guard_test.go` | no non-test file in the three packages passes a non-constant that is not a `Cmd` call (a constant suffix may be appended) to `Run`, `RunQuiet`, `RunWithStdin` or `Start` (the call inside `WriteFile`, which quotes its own path; the pass-through methods of `Client` are exempt); the doc comment states the limit (receiver and call shape) | — | S1, S2, S3 |
| `TestHostileStateCommandsAreOneWordEach` | `pkg/ssh/remote_e2e_test.go` | with an interface name, endpoint, key and route each containing `; touch /x`, every command the server records keeps the hostile text inside one quoted word (a small quote-aware splitter in the test) | — | S1, S2, S3 |
| `TestRecordedCommandsMatchGolden` | `pkg/ssh/remote_e2e_test.go` | the commands for the valid fixture equal `commands-valid.golden` byte for byte | — | S1, S3 |
| `TestWgQuickRefusesControlCharacters` | `pkg/wireguard/persist_test.go` | a newline, carriage return or NUL in the address, key, endpoint, allowed IP or route returns an error naming the field | — | S1, S3 |
| `TestWgQuickQuotesRouteValues` | `pkg/wireguard/persist_test.go` | a route network `10.0.0.0/8; id` appears inside quotes in the `PostUp` and `PreDown` lines | — | S1, S3 |
| `TestWgQuickValidConfigUnchanged` | `pkg/wireguard/persist_test.go` | the text for the valid fixture equals the recorded one | — | S1, S3 |
| `TestRemoteGuardJudgesShapes` | `pkg/ssh/remote_guard_test.go` | a table of source snippets pins the guard: it fails on a `Sprintf`, a variable, a concatenation with a variable, a `Cmd` with a built name or no arguments, a leading `Quote`, `strconv.Quote`, a function result and a same-named method of another type, and passes constants, `Cmd`, `Cmd` plus a constant suffix and `"cat > " + Quote(p)` | — | S2 |
| `TestSSHTestHostKeyIsVerified` | `pkg/ssh/remote_e2e_test.go` | the client accepts the `sshtest` key through the temp `known_hosts` and refuses a different key | — | S1, S2 |
| `TestIPv6EndpointIsQuotedOnWgSet` | `pkg/ssh/remote_e2e_test.go` | an IPv6 endpoint `[2001:db8::1]:51820` reaches `wg set` as one single-quoted word (brackets are glob characters; the golden holds no bracket) | — | S3 |
| `TestWgQuickRouteWithoutGatewayHasNoVia` | `pkg/wireguard/persist_test.go` | a route without gateway renders `PostUp = ip route add 10.9.0.0/24 dev %i` (plus the shell fallback) and the matching `PreDown`, with no `via` and no `''` (the old text was the broken `via  dev`, hidden by the shell fallback; deploy never passes such a route) | — | S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the six tests and the fake server |
| 2 — something selects it | the guard test lists the three packages by name; deleting a `Cmd` call fails it |
| 3 — the caller can discover it | doc comments on `Cmd`; T6 names the rule in the spec |
| 4 — it is used | every `-deploy` command from this task on |

## Mutation Log
- 2026-11-15 · 2e07fdd* · mutant killed · exit 1 · `pkg/ssh/routes.go` · GetCurrentRoutes raw Sprintf: TestNoUnquotedRemoteCommand must go red · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8
- 2026-11-15 · 2e07fdd* · mutant killed · exit 1 · `pkg/wireguard/persist.go` · rm -f concatenates configPath: TestNoUnquotedRemoteCommand must go red · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8
- 2026-11-15 · 2e07fdd* · mutant killed · exit 1 · `pkg/wireguard/persist.go` · PostUp route unquoted: TestWgQuickQuotesRouteValues must go red · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8
- 2026-11-15 · 2e07fdd* · mutant killed · exit 1 · `pkg/wireguard/persist.go` · newline and CR no longer refused: TestWgQuickRefusesControlCharacters must go red · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8
- 2026-11-15 · 2e07fdd* · mutant killed · exit 1 · `pkg/ssh/client.go` · WriteFile path unquoted: guard and TestHostileStateCommandsAreOneWordEach must go red · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8
- 2026-11-15 · 2e07fdd* · mutant killed · exit 1 · `pkg/ssh/routes.go` · route commands lose via: golden and e2e must go red · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8
- 2026-11-15 · 2e07fdd* · mutant killed · exit 1 · `pkg/wireguard/config.go` · wg set unquoted for PSK peers: guard must go red · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8
- 2026-11-15 · 79222e5* · mutant killed · exit 1 · `pkg/ssh/routes.go` · GetCurrentRoutes raw Sprintf: TestNoUnquotedRemoteCommand must go red · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8
- 2026-11-15 · 79222e5* · mutant killed · exit 1 · `pkg/wireguard/persist.go` · rm -f concatenates configPath: TestNoUnquotedRemoteCommand must go red · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8
- 2026-11-15 · 79222e5* · mutant killed · exit 1 · `pkg/wireguard/persist.go` · PostUp route unquoted: TestWgQuickQuotesRouteValues must go red · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8
- 2026-11-15 · 79222e5* · mutant killed · exit 1 · `pkg/wireguard/persist.go` · newline and CR no longer refused: TestWgQuickRefusesControlCharacters must go red · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8
- 2026-11-15 · 79222e5* · mutant killed · exit 1 · `pkg/ssh/client.go` · WriteFile path unquoted: guard and TestHostileStateCommandsAreOneWordEach must go red · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8
- 2026-11-15 · 79222e5* · mutant killed · exit 1 · `pkg/ssh/routes.go` · route commands lose via: golden and e2e must go red · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8
- 2026-11-15 · 79222e5* · mutant killed · exit 1 · `pkg/wireguard/config.go` · wg set unquoted for PSK peers: guard must go red · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8
- 2026-11-15 · 79222e5* · mutant killed · exit 1 · `pkg/ssh/routes.go` · an empty gateway prints via '': TestWgQuickRouteWithoutGatewayHasNoVia must go red · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8

## Invariants

- For a valid state every command and the wg-quick text are byte-identical to the unchanged code (the golden).
- `shellQuote` no longer exists; there is one quoting function.
- The daemon's local-apply functions in `pkg/wireguard/apply.go` (`SetPeer`, `RemovePeer`, `GetPeers`, `GetLatestHandshakes`, `GetPeerTransfers`) are not touched.
- `pkg/ssh`, `pkg/wireguard` and `pkg/mesh` import nothing from `pkg/node`, `pkg/daemon`, `pkg/discovery`.
- PR #876's host key verification is exercised, not bypassed: the fake server's key is in a temp `known_hosts`, `WGMESH_SSH_INSECURE_HOST_KEY` is not set.

## Risks

- A wg-quick consumer reads a quoted value differently from an unquoted one; for valid values nothing is quoted, so the file is identical, and only hostile values change.
- An empty route gateway: the old wg-quick text was `ip route add N via  dev %i || true` (a broken line that `|| true` hid, so the route was never installed); `RouteArgs` omits `via`, so a gateway-less route would now be installed at `wg-quick up`. Deploy never passes one (`TestDeploymentConfigUnderLock` asserts no `via  dev`; T3 refuses a node without a mesh IP); `TestWgQuickRouteWithoutGatewayHasNoVia` pins the new text.
- `Cmd(routes.ConnectedRouteCmd(...))` quotes nothing (`Cmd` quotes arguments, not its name): the guard names it as the one exception, and its inputs are the parsed network, an interface name checked by `ifaceNameRe` and a parsed address.
- `sshtest` is a new package with a server in it; it is imported only from `_test.go` files, and the guard test `TestNoUnquotedRemoteCommand` does not list it. A non-test import would put a test server in the binary; `go list -deps . | grep sshtest` must print nothing (run in S4).
- Test doubles: the fake server is a double of the node; the golden is recorded from the real code, not written by hand.

## Stop Condition

Stop and ask if the golden differs after S3 for a valid value (the safe set is wrong), if `Run`'s callers outside the three packages exist (`grep -rn "\.Run(" --include=*.go . | grep -v _test.go` lists any `ssh.Client` use elsewhere), or if wg-quick would be passed a value the renderer refuses in an existing documented state.

## Out of Scope

- State validation (T3) and the per-node result (T4).
- Deleting the unreachable builders (deferred: docs/adr/BACKLOG.md).
- `routes.ConnectedRouteCmd` in `pkg/routes` (permanent: boundary: its inputs are a parsed network, an interface name checked by `ifaceNameRe` and a parsed address; T2 only wraps the finished command at the call site in `pkg/ssh/routes.go`).

## Verification Log
- 2026-11-15 · 2e07fdd* · exit 1 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:1273 · test-lock-sha256:27cfcf28781eaac35f9229fba539b55ab20d606105334ad5e654ebf0d7a88ee1 · test-lock-file:docs/adr/locks/27cfcf28781eaac35f9229fba539b55ab20d606105334ad5e654ebf0d7a88ee1.lock
  ```
  --- last 10 line(s) of stderr (of 228 after folding 234 raw)
      remote_e2e_test.go:332: fresh=false bravo: `;` outside quotes in "cat > /etc/wireguard/wg0; touch /x.conf && chmod 600 /etc/wireguard/wg0; touch /x.conf"
      remote_e2e_test.go:336: fresh=false bravo: injected word "touch" in "cat > /etc/wireguard/wg0; touch /x.conf && chmod 600 /etc/wireguard/wg0; touch /x.conf"  (x2)
      remote_e2e_test.go:349: no command carries the hostile route inside one word
      remote_e2e_test.go:349: no command carries the hostile endpoint inside one word
      remote_e2e_test.go:349: no command carries the hostile interface name inside one word
      remote_e2e_test.go:349: no command carries the hostile public key inside one word
  --- FAIL: TestHostileStateCommandsAreOneWordEach (0.04s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/ssh	0.133s
  FAIL
  ```
- 2026-11-15 · 2e07fdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:13203
- 2026-11-15 · 2e07fdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:12873
- 2026-11-15 · 2e07fdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:13956
- 2026-11-15 · 2e07fdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:14053
- 2026-11-15 · 2e07fdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:12045
- 2026-11-15 · 2e07fdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:10320
- 2026-11-15 · 2e07fdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:11082
- 2026-11-15 · 2e07fdd* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:8000
- 2026-11-15 · 79222e5* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:0 · test-lock-sha256:81fffd91ecf23bc747deef41ddce07582635a5d92bf550e1cb350fd0ce7557dd · test-lock-file:docs/adr/locks/81fffd91ecf23bc747deef41ddce07582635a5d92bf550e1cb350fd0ce7557dd.lock · test-lock-kind:replace
- 2026-11-15 · 79222e5* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:10554
- 2026-11-15 · 79222e5* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:10730
- 2026-11-15 · 79222e5* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:10966
- 2026-11-15 · 79222e5* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:11512
- 2026-11-15 · 79222e5* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:13739
- 2026-11-15 · 79222e5* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:11480
- 2026-11-15 · 79222e5* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:12078
- 2026-11-15 · 79222e5* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:13030
- 2026-11-15 · 79222e5* · exit 0 · `set -o pipefail …` · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8 · ms:7703
- 2026-11-15 · human-observed · observed: class grep Sprintf builders 29 to 1 (the /tmp/wg-key path, a quoted argument), cmd += appends 7 to 0, go list -deps of the binary shows no sshtest; the guard test is the gate · acceptance-sha256:e53ecfdd1fcaba33c83f43761fd97b674f9c96d7dc009ee8ba4f2ea93ed263b8
