# Task ADR-0005-T6: `rpc.NewServer` owns its socket path before it removes anything (#860)

**Depends-on:** T5
**Covers:** none — no spec
**Estimated scope:** S (single file)
**Owner:** unassigned
**Produces:** a socket lock `<socket-path>.lock` held from `NewServer` to `Stop`
**Consumes:** `flock.TryLock` (T5)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

A second daemon on the same host that picks the same RPC socket path (the default has no interface in it) does not unlink the live socket of the first; its `createRPCServer` fails and, as `main.go:459-465` already does for that failure, it runs without RPC.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/rpc/server.go` | edit | `NewServer` takes the lock `config.SocketPath + ".lock"` before the `os.Remove` at `:70` and returns an error that names the path and the holder pid when it is held; the release is stored in `Server` and called in `Stop` after the socket is removed (`:356-369`); a failed `NewServer` releases what it took |
| `pkg/rpc/server_ownership_test.go` | add | tests below |

Selection: `main.go:971` calls `rpc.NewServer`; the first test fails if the lock call is removed.

## Ordered Steps

1. [S1] Add the four tests with the current `NewServer` and run red (the second `NewServer` today unlinks the first's socket and succeeds).
2. [S2] Take the lock in `NewServer`, release it in `Stop`; `Stop` stays safe to call twice.
3. [S3] Read `main.go:459-465` again and confirm in the commit message that a `NewServer` error is a warning there (read, not run). [proof: human: the continue-without-RPC path is code-read; running two interfaces on a host is the sign-off]
4. [S4] Run the mutants with `adr-verify --mutant`: the lock taken after the `os.Remove`; `Stop` does not release; the error path leaks the lock. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/rpc -run '^(TestNewServerRefusesOwnedSocket|TestNewServerLeavesLiveSocketAlone|TestStopReleasesSocketLock|TestNewServerRemovesStaleUnownedSocket)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 4 ] && go test -race ./pkg/rpc -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestNewServerRefusesOwnedSocket` | `pkg/rpc/server_ownership_test.go` | a second `NewServer` on the same path returns an error naming the path | — | S1, S2 |
| `TestNewServerLeavesLiveSocketAlone` | `pkg/rpc/server_ownership_test.go` | after the refused second call the first server's socket still exists and a client dial succeeds | — | S1, S2 |
| `TestStopReleasesSocketLock` | `pkg/rpc/server_ownership_test.go` | after `Stop` (twice) a new `NewServer` on the path succeeds | — | S1, S2 |
| `TestNewServerRemovesStaleUnownedSocket` | `pkg/rpc/server_ownership_test.go` | pin: a leftover socket file with no holder is removed and the server starts (starts green) | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the four tests |
| 2 — something selects it | `NewServer` is the only constructor; the lock call is inside it |
| 3 — the caller can discover it | the error text; T11 amends the RPC spec |
| 4 — it is used | every daemon that starts an RPC server |

## Mutation Log

## Invariants

- One daemon on a host behaves exactly as before: the stale-socket removal, the 0600 socket mode and `GetSocketPath` are unchanged.
- The lock file is `<socket-path>.lock`; the socket path default is unchanged (Open Question 3, option c is not taken).
- `pkg/rpc` imports `pkg/flock` and nothing from `pkg/daemon`.

## Risks

- The lock file sits beside the socket (`/var/run`, `$XDG_RUNTIME_DIR` or `/tmp`) and may be left by a crash; it holds no lock then, the next start reuses it.
- The check that `main.go` continues without RPC is code-read only (S3).
- Test doubles: temp directory socket paths short enough for the platform's socket limit.

## Stop Condition

Stop and ask if `NewServer` gains a second caller that treats an error as fatal.

## Out of Scope

- The interface lock (T5).
- A per-interface default socket path (permanent: boundary: it would change what `wgmesh peers` finds; Open Question 3).

## Verification Log
