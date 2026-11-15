# Task ADR-0007-T1: Read `wg` state through one snapshot with a deadline, and give it a test seam

**Depends-on:** none
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `wireguard.ReadState(iface, maxAge)`, `wireguard.State`, `wireguard.SetWGPath(path) (restore func())`, unexported `runWG` and `wgCommandTimeout`
**Consumes:** none
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`pkg/wireguard` is the only place that forks `wg` for state, forks `wg show <iface> dump` at most once per `maxAge`, kills any `wg` child after 10 seconds, and can be tested without a real `wg` binary.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/wireguard/state.go` | add | `State` (peers with public key, endpoint, latest handshake, rx and tx bytes), `ReadState`, the per-interface slot with its mutex and time, `SetWGPath` |
| `pkg/wireguard/apply.go` | edit | `runWG` (`exec.CommandContext` with `wgCommandTimeout`) replaces the five `exec.Command` calls; `GetPeers`, `GetLatestHandshakes`, `GetPeerTransfers` become readers over `ReadState(iface, 0)` and keep their signatures |
| `pkg/wireguard/keys.go` | edit | the two `exec.Command` calls (`genkey`, `pubkey`) go through `runWG` |
| `pkg/wireguard/state_test.go` | add | the tests below; each writes a small shell script as the fake `wg` and points `wgPath` at it |

Selection: the legacy readers forward to `ReadState`, so every existing caller runs through the new code before T3 moves them; a mutant that makes a legacy reader fork on its own turns `TestLegacyReadersForwardToReadState` red.

## Ordered Steps

1. [S1] Add `state_test.go` with the seven tests and compiling stubs (`ReadState` returns an empty `State`, `SetWGPath` stores the path) so the first run fails on assertions, not on the build. Run red.
2. [S2] Read `parseWGDump` and `peersEqual` (`pkg/wireguard/config.go:63`, `:155`) and write `State` as a separate type from the same line format (columns 1 to 7: key, preshared, endpoint, allowed IPs, handshake, rx, tx); `Peer` and the SSH path stay unchanged. State the choice in the commit message.
3. [S3] Implement `ReadState`: under one mutex, a stored result for the interface younger than `maxAge` is returned as is, otherwise fork `wg show <iface> dump`, parse, store with `time.Now()`, return; `maxAge <= 0` always forks; a failed fork or parse is returned and never stored.
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

## Invariants

- `GetPeers`, `GetLatestHandshakes`, `GetPeerTransfers`, `SetPeer`, `RemovePeer`, `GenerateKeyPair` keep their signatures and results.
- `parseWGDump` and the SSH path (`GetCurrentConfig`) are not changed.
- No new dependency; no change to `wgPath` lookup (`apply.go:21-27`).
- A fork error is never cached; a stored result is never older than the `maxAge` of the call that returned it.

## Risks

- Two goroutines call `ReadState(iface, 0)` at once and both fork; accepted (a fresh read is what they asked for), the mutex covers the slot only.
- A fake shell script needs `sh`; the project releases for Linux and Darwin only (`CLAUDE.md`, CI/CD), so the tests run on both.

## Stop Condition

Stop and ask if `wg show <iface> dump` differs between the `wg` builds the project supports in a way the parser cannot handle (for example a userspace build that prints fewer columns), or if an existing test needs a changed expectation.

## Out of Scope

- Moving callers to `ReadState` with a `maxAge` (T3).
- A deadline for the 37 `cmdExecutor` calls (deferred: docs/adr/BACKLOG.md).
- Replacing the binary with a netlink client (permanent: boundary: a new dependency on the key-handling path).

## Verification Log
