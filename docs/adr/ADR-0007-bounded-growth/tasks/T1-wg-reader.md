# Task ADR-0007-T1: Read `wg` state through one snapshot with a deadline, and give it a test seam

**Depends-on:** none
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `wireguard.ReadState(iface, maxAge)`, `wireguard.State`, `wireguard.StatePeer`, `wireguard.SetWGPath(path) (restore func())`, unexported `runWG` and `wgCommandTimeout`
**Consumes:** none
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`pkg/wireguard` is the only place that forks `wg` for state, forks `wg show <iface> dump` at most once per `maxAge`, kills any `wg` child after 10 seconds, and can be tested without a real `wg` binary.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/wireguard/state.go` | add | `State` (peers with public key, endpoint, latest handshake, rx and tx bytes), `ReadState`, the per-interface slot with its mutex and time, `SetWGPath`, `runWG` with `wgCommandTimeout` and a one-second `WaitDelay` so a grandchild holding the pipes cannot outlast the deadline; a malformed number in a peer line reads as 0, as the old `Sscanf` readers did |
| `pkg/wireguard/apply.go` | edit | the five `exec.Command` calls go through `runWG` (state.go); `GetPeers`, `GetLatestHandshakes`, `GetPeerTransfers` become readers over `ReadState(iface, 0)` and keep their signatures |
| `pkg/wireguard/keys.go` | edit | the two `exec.Command` calls (`genkey`, `pubkey`) go through `runWG` |
| `pkg/wireguard/state_test.go` | add | the tests below; each writes a small shell script as the fake `wg` and points `wgPath` at it |

Selection: the legacy readers forward to `ReadState`, so every existing caller runs through the new code before T3 moves them; a mutant that makes a legacy reader fork on its own turns `TestLegacyReadersForwardToReadState` red.

## Ordered Steps

1. [S1] Add `state_test.go` with the seven tests and compiling stubs (`ReadState` returns an empty `State`, `SetWGPath` stores the path) so the first run fails on assertions, not on the build. Run red.
2. [S2] Read `parseWGDump` and `peersEqual` (`pkg/wireguard/config.go:63`, `:155`) and write `State` as a separate type from the same line format (columns 1 to 7: key, preshared, endpoint, allowed IPs, handshake, rx, tx); `Peer` and the SSH path stay unchanged. State the choice in the commit message.
3. [S3] Implement `ReadState`: under one mutex, a stored result for the interface younger than `maxAge` is returned as is, otherwise fork `wg show <iface> dump`, parse, store with the time taken before the fork started, return; the mutex is held across the fork on purpose; `maxAge <= 0` always forks; a failed fork or parse is returned and never stored.
4. [S4] Add `runWG` and route the seven former `exec.Command` sites through it; on `context.DeadlineExceeded` the error names the sub-command and the timeout. `grep -n 'exec\.Command(' pkg/wireguard/apply.go pkg/wireguard/keys.go` must list nothing.
5. [S5] Make the three `Get*` functions forward to `ReadState(iface, 0)` and build their old return values from the `State`.
6. [S6] Run the mutants with `adr-verify --mutant`: `maxAge` ignored (every call forks); an error result stored; the deadline removed; a legacy reader forks directly. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/wireguard -run '^(TestReadStateParsesDumpColumns|TestReadStateReusesResultInsideMaxAge|TestReadStateZeroMaxAgeAlwaysForks|TestReadStateDoesNotStoreErrors|TestRunWGKillsChildAfterTimeout|TestReadStateKeepsSlotPerInterface|TestLegacyReadersForwardToReadState)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 7 ] && ! grep -n 'exec\.Command(' pkg/wireguard/apply.go pkg/wireguard/keys.go && go test -race ./pkg/wireguard ./pkg/daemon ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestReadStateParsesDumpColumns` | `pkg/wireguard/state_test.go` | a fake `wg` printing an interface line and two peer lines gives the endpoint, handshake and rx/tx of each peer, `(none)` as empty | — | S1, S2 |
| `TestReadStateReusesResultInsideMaxAge` | `pkg/wireguard/state_test.go` | two calls with `maxAge` one hour run the fake script once | — | S1, S3 |
| `TestReadStateZeroMaxAgeAlwaysForks` | `pkg/wireguard/state_test.go` | two calls with `maxAge` 0 run it twice and the second sees the changed output | — | S1, S3 |
| `TestReadStateDoesNotStoreErrors` | `pkg/wireguard/state_test.go` | a failing script returns an error; the next call forks again instead of returning the error | — | S1, S3 |
| `TestRunWGKillsChildAfterTimeout` | `pkg/wireguard/state_test.go` | with `wgCommandTimeout` 200 ms and a script that sleeps 30 s, the call returns in under 3 s with an error naming the timeout, and the child is gone | — | S1, S4 |
| `TestReadStateKeepsSlotPerInterface` | `pkg/wireguard/state_test.go` | a snapshot of `wg0` is not returned for `wg1` | — | S1, S3 |
| `TestLegacyReadersForwardToReadState` | `pkg/wireguard/state_test.go` | `GetLatestHandshakes`, `GetPeers` and `GetPeerTransfers` return the values the old sub-commands gave for the same dump, and every fork is the single `show wg0 dump` sub-command: four forks for one `ReadState` inside `maxAge` and the three readers, which ask `maxAge` 0 | — | S1, S5 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the seven tests |
| 2 — something selects it | the legacy readers call `ReadState`; the direct-fork mutant turns `TestLegacyReadersForwardToReadState` red |
| 3 — the caller can discover it | exported with doc comments; the `maxAge` table is in ADR-0007 Decision 5 |
| 4 — it is used | T3 moves the daemon and discovery callers |

## Mutation Log
- 2026-11-15 · 1905e4b* · mutant killed · exit 1 · `pkg/wireguard/state.go` · maxAge ignored, every call forks: TestReadStateReusesResultInsideMaxAge must go red · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1
- 2026-11-15 · 1905e4b* · mutant killed · exit 1 · `pkg/wireguard/state.go` · a failed fork is stored: TestReadStateDoesNotStoreErrors must go red · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1
- 2026-11-15 · 1905e4b* · mutant killed · exit 1 · `pkg/wireguard/state.go` · no deadline on the child: TestRunWGKillsChildAfterTimeout must go red · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1
- 2026-11-15 · 1905e4b* · mutant killed · exit 1 · `pkg/wireguard/apply.go` · a legacy reader forks on its own: TestLegacyReadersForwardToReadState must go red · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1
- 2026-11-15 · a07e465* · mutant killed · exit 1 · `pkg/wireguard/state.go` · maxAge ignored: TestReadStateReusesResultInsideMaxAge red · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1
- 2026-11-15 · a07e465* · mutant killed · exit 1 · `pkg/wireguard/state.go` · failed fork stored: TestReadStateDoesNotStoreErrors red · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1
- 2026-11-15 · a07e465* · mutant killed · exit 1 · `pkg/wireguard/state.go` · no deadline: TestRunWGKillsChildAfterTimeout red · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1
- 2026-11-15 · a07e465* · mutant killed · exit 1 · `pkg/wireguard/apply.go` · legacy reader forks directly: TestLegacyReadersForwardToReadState red · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1
- 2026-11-15 · a07e465* · mutant killed · exit 1 · `pkg/wireguard/state.go` · stored result never expires: TestReadStateReforksPastMaxAge red · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1
- 2026-11-15 · a07e465* · mutant killed · exit 1 · `pkg/wireguard/state.go` · grandchild holds the pipes past the deadline: TestRunWGKillsChildAfterTimeout red · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1
- 2026-11-15 · a07e465* · mutant killed · exit 1 · `pkg/wireguard/state.go` · wrong column: TestReadStateParsesDumpColumns red · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1
- 2026-11-15 · a07e465* · mutant killed · exit 1 · `pkg/wireguard/apply.go` · GetPeers swallows the read error: TestLegacyReadersPropagateErrors red · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1

## Invariants

- `GetPeers`, `GetLatestHandshakes`, `GetPeerTransfers`, `SetPeer`, `RemovePeer`, `GenerateKeyPair` keep their signatures and results.
- `parseWGDump` and the SSH path (`GetCurrentConfig`) are not changed.
- No new dependency; no change to `wgPath` lookup (`apply.go:21-27`).
- A fork error is never cached; a stored result is never older than the `maxAge` of the call that returned it.

## Risks

- Two goroutines call `ReadState` for one interface at once: they queue on the slot mutex, which is held across the fork, so a hung `wg` costs each queued caller up to 10 seconds; accepted (one fork per request, and T3 moves the loops to `maxAge > 0` where they share one fork).
- A fake shell script needs `sh`; the project releases for Linux and Darwin only (`CLAUDE.md`, CI/CD), so the tests run on both.

## Stop Condition

Stop and ask if `wg show <iface> dump` differs between the `wg` builds the project supports in a way the parser cannot handle (for example a userspace build that prints fewer columns), or if an existing test needs a changed expectation.

## Out of Scope

- Moving callers to `ReadState` with a `maxAge` (T3).
- A deadline for the 37 `cmdExecutor` calls (deferred: docs/adr/BACKLOG.md).
- Replacing the binary with a netlink client (permanent: boundary: a new dependency on the key-handling path).

## Verification Log
- 2026-11-15 · 1905e4b* · exit 1 · `set -o pipefail …` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:2848 · test-lock-sha256:dadcdebf81ad6bf17f44e3662adf3eefe6f8661f6f5bbafbdbcdfb5f30d3e318 · test-lock-file:docs/adr/locks/dadcdebf81ad6bf17f44e3662adf3eefe6f8661f6f5bbafbdbcdfb5f30d3e318.lock
  ```
  --- last 10 line(s) of stderr (of 27 after folding 27 raw)
  --- FAIL: TestReadStateKeepsSlotPerInterface (0.00s)
  === RUN   TestLegacyReadersForwardToReadState
      state_test.go:219: handshakes = map[PRIV:0 keyA:0 keyB:0]
      state_test.go:222: peers = [{PublicKey:PRIV	PUB	51820	off Endpoint: AllowedIPs:[] PersistentKeepalive:0} {PublicKey:keyA	(none)	203.0.113.5:51820	10.0.0.2/32	1700000000	111	222	25 Endpoint: AllowedIPs:[] PersistentKeepalive:0} {PublicKey:keyB	(none)	(none)	10.0.0.3/32	0	0	0	off Endpoint: AllowedIPs:[] PersistentKeepalive:0}]
      state_test.go:225: transfers = map[PRIV:{0 51820} keyA:{0 203} keyB:{0 0}]
      state_test.go:231: forks = 3, want 4: ["show wg0 latest-handshakes" "show wg0 peers" "show wg0 transfer"]
  --- FAIL: TestLegacyReadersForwardToReadState (0.07s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/wireguard	0.106s
  FAIL
  ```
- 2026-11-15 · 1905e4b* · exit 0 · `set -o pipefail …` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:21341
- 2026-11-15 · 1905e4b* · exit 0 · `set -o pipefail …` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:18135
- 2026-11-15 · 1905e4b* · exit 0 · `set -o pipefail …` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:15516
- 2026-11-15 · 1905e4b* · exit 0 · `set -o pipefail …` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:13981
- 2026-11-15 · 1905e4b* · exit 0 · `set -o pipefail …` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:10910
- 2026-11-15 · a07e465* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:1 · test-lock-sha256:90ec0126972f43ebb949d4d9f687827178ecddae3814b394adcdc6a459ec21f2 · test-lock-file:docs/adr/locks/90ec0126972f43ebb949d4d9f687827178ecddae3814b394adcdc6a459ec21f2.lock · test-lock-kind:replace
- 2026-11-15 · a07e465* · exit 0 · `set -o pipefail …` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:13084
- 2026-11-15 · a07e465* · exit 0 · `set -o pipefail …` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:11922
- 2026-11-15 · a07e465* · exit 0 · `set -o pipefail …` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:13002
- 2026-11-15 · a07e465* · exit 0 · `set -o pipefail …` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:19376
- 2026-11-15 · a07e465* · exit 0 · `set -o pipefail …` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:18042
- 2026-11-15 · a07e465* · exit 0 · `set -o pipefail …` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:17091
- 2026-11-15 · a07e465* · exit 0 · `set -o pipefail …` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:16785
- 2026-11-15 · a07e465* · exit 0 · `set -o pipefail …` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:16017
- 2026-11-15 · a07e465* · exit 0 · `set -o pipefail …` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:11019
- 2026-11-16 · b6554cc · exit 0 · `set -o pipefail …` · acceptance-sha256:16c9fc522d71f9ee59b585ad496978aef113c2dc33b726854196606a71c0dbd1 · ms:7878
