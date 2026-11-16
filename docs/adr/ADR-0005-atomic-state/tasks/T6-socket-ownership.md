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
n=$(go test ./pkg/rpc ./pkg/flock . -run '^(TestNewServerRefusesOwnedSocket|TestNewServerRefusalSaysWhatToDo|TestNewServerLeavesLiveSocketAlone|TestStopReleasesSocketLock|TestNewServerRemovesStaleUnownedSocket|TestNewServerErrorReleasesLock|TestStopIsSafeConcurrently|TestSecondStopKeepsNewOwnersSocket|TestStartFailureReleasesLock|TestStartAfterStopFails|TestStopWithoutNewServerDoesNotPanic|TestStartStopRaceFree|TestCreateRPCServerReturnsUntypedNilOnError|TestTryLockRefusesSymlink)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 14 ] && go test -race ./pkg/rpc ./pkg/flock -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestNewServerRefusesOwnedSocket` | `pkg/rpc/server_ownership_test.go` | a second `NewServer` on the same path returns an error naming the path | — | S1, S2 |
| `TestNewServerLeavesLiveSocketAlone` | `pkg/rpc/server_ownership_test.go` | after the refused second call the first server's socket still exists and a client dial succeeds | — | S1, S2 |
| `TestStopReleasesSocketLock` | `pkg/rpc/server_ownership_test.go` | after `Stop` (twice) a new `NewServer` on the path succeeds | — | S1, S2 |
| `TestNewServerRemovesStaleUnownedSocket` | `pkg/rpc/server_ownership_test.go` | pin: a leftover socket file with no holder is removed and the server starts (starts green) | — | S1, S2 |
| `TestNewServerRefusalSaysWhatToDo` | `pkg/rpc/server_ownership_test.go` | the refusal names the path and the pid once and says to use `--socket-path` or `WGMESH_SOCKET` | — | S1, S2 |
| `TestNewServerErrorReleasesLock` | `pkg/rpc/server_ownership_test.go` | a `NewServer` that fails after taking the lock (the path is a non-empty directory) leaves the lock free | — | S1, S2 |
| `TestStopIsSafeConcurrently` | `pkg/rpc/server_ownership_test.go` | 16 goroutines call `Stop` under `-race`; all get the same result and the socket is gone | — | S1, S2 |
| `TestSecondStopKeepsNewOwnersSocket` | `pkg/rpc/server_ownership_test.go` | a second `Stop` of the old server does not unlink the new owner's socket (kills removing `stopOnce`) | — | S1, S2 |
| `TestStartFailureReleasesLock` | `pkg/rpc/server_ownership_test.go` | a failed `Start` (path too long for a unix socket) releases the lock | — | S1, S2 |
| `TestStartAfterStopFails` | `pkg/rpc/server_ownership_test.go` | a stopped `Server` cannot `Start` | — | S1, S2 |
| `TestStopWithoutNewServerDoesNotPanic` | `pkg/rpc/server_ownership_test.go` | `(&Server{}).Stop()` is safe | — | S1, S2 |
| `TestStartStopRaceFree` | `pkg/rpc/server_ownership_test.go` | `Start` and `Stop` run concurrently under `-race` (the listener is guarded by `mu`) | — | S1, S2 |
| `TestCreateRPCServerReturnsUntypedNilOnError` | `main_rpc_test.go` | a refused `createRPCServer` returns an untyped nil interface | — | S1, S2 |
| `TestTryLockRefusesSymlink` | `pkg/flock/flock_test.go` | a symlink at the lock path is not followed (`O_NOFOLLOW`) and its target is untouched | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the four tests |
| 2 — something selects it | `NewServer` is the only constructor; the lock call is inside it |
| 3 — the caller can discover it | the error text; T11 amends the RPC spec |
| 4 — it is used | every daemon that starts an RPC server |

## Mutation Log
- 2026-11-16 · 5be2569* · mutant killed · exit 1 · `pkg/rpc/server.go` · NewServer takes no lock: TestNewServerRefusesOwnedSocket must go red · acceptance-sha256:6d57a05172e2e1e1595ea99a0588140cf4c3abba367a38afefa9bbf48a8698de
- 2026-11-16 · 5be2569* · mutant killed · exit 1 · `pkg/rpc/server.go` · Stop does not release the lock: TestStopReleasesSocketLock must go red · acceptance-sha256:6d57a05172e2e1e1595ea99a0588140cf4c3abba367a38afefa9bbf48a8698de
- 2026-11-16 · 5be2569* · mutant killed · exit 1 · `pkg/rpc/server.go` · the error path after the lock leaks it: TestNewServerErrorReleasesLock must go red · acceptance-sha256:6d57a05172e2e1e1595ea99a0588140cf4c3abba367a38afefa9bbf48a8698de
- 2026-11-16 · 5be2569* · mutant killed · exit 1 · `pkg/rpc/server.go` · a second Stop runs again: TestSecondStopKeepsNewOwnersSocket must go red · acceptance-sha256:6d57a05172e2e1e1595ea99a0588140cf4c3abba367a38afefa9bbf48a8698de
- 2026-11-16 · 5be2569* · mutant killed · exit 1 · `pkg/flock/flock.go` · TryLock follows a planted symlink: TestTryLockRefusesSymlink must go red · acceptance-sha256:6d57a05172e2e1e1595ea99a0588140cf4c3abba367a38afefa9bbf48a8698de

## Invariants

- One daemon on a host behaves exactly as before: the stale-socket removal, the 0600 socket mode and `GetSocketPath` are unchanged.
- The lock file is `<socket-path>.lock`; the socket path default is unchanged (M3, option c is not taken).
- `pkg/rpc` imports `pkg/flock` and nothing from `pkg/daemon`.

## Risks

- The lock file sits beside the socket (`/var/run`, `$XDG_RUNTIME_DIR` or `/tmp`) and may be left by a crash; it holds no lock then, the next start reuses it.
- The check that `main.go` continues without RPC is code-read only (S3).
- Test doubles: temp directory socket paths short enough for the platform's socket limit.
- Behaviour taken on (panel 2026-11-16, not all covered by a test): protection is one-sided: an older release (no lock) on the same path still unlinks a live socket, and a new release starting after an old one still unlinks the old one's socket; the second daemon on a host runs without RPC and `wgmesh status` talks to the FIRST daemon (clients choose a socket with `WGMESH_SOCKET`; `status` has no `--socket-path` flag), the unit stays active while one interface has no RPC; the lock file stays beside the socket (`/var/run`, `$XDG_RUNTIME_DIR`, `/tmp`) and is never removed; a lock file in the world-writable `/tmp` fallback owned by another user gives EACCES, and a planted symlink is not followed (`O_NOFOLLOW`); `Start` failing releases the lock and marks the server stopped; `Stop` now always logs `RPC server stopped` and the daemon logs a failed socket removal; `pkg/rpc` now builds on unix only (T5's `pkg/flock`). The release note (T11) says all of this.

## Stop Condition

Stop and ask if `NewServer` gains a second caller that treats an error as fatal.

## Out of Scope

- The interface lock (T5).
- A per-interface default socket path (permanent: boundary: it would change what `wgmesh peers` finds; M3).

## Verification Log
- 2026-11-16 · 5be2569* · exit 1 · `set -o pipefail …` · acceptance-sha256:6d57a05172e2e1e1595ea99a0588140cf4c3abba367a38afefa9bbf48a8698de · ms:1845 · test-lock-sha256:c3043ed4ab07aa80c34d1d2d3ffad9ffdb949fd09f40ba941700f32aff0b5c91 · test-lock-file:docs/adr/locks/c3043ed4ab07aa80c34d1d2d3ffad9ffdb949fd09f40ba941700f32aff0b5c91.lock
  ```
  --- last 10 line(s) of stderr (of 108 after folding 143 raw)
      flock_test.go:272: TryLock followed a symlink at the lock path
  --- FAIL: TestTryLockRefusesSymlink (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/flock	0.003s
  === RUN   TestCreateRPCServerReturnsUntypedNilOnError
      main_rpc_test.go:26: createRPCServer on an owned socket path succeeded
  --- FAIL: TestCreateRPCServerReturnsUntypedNilOnError (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh	0.005s
  FAIL
  ```
- 2026-11-16 · 5be2569* · exit 0 · `set -o pipefail …` · acceptance-sha256:6d57a05172e2e1e1595ea99a0588140cf4c3abba367a38afefa9bbf48a8698de · ms:5195
- 2026-11-16 · 5be2569* · exit 0 · `set -o pipefail …` · acceptance-sha256:6d57a05172e2e1e1595ea99a0588140cf4c3abba367a38afefa9bbf48a8698de · ms:6154
- 2026-11-16 · 5be2569* · exit 0 · `set -o pipefail …` · acceptance-sha256:6d57a05172e2e1e1595ea99a0588140cf4c3abba367a38afefa9bbf48a8698de · ms:6223
- 2026-11-16 · 5be2569* · exit 0 · `set -o pipefail …` · acceptance-sha256:6d57a05172e2e1e1595ea99a0588140cf4c3abba367a38afefa9bbf48a8698de · ms:5743
- 2026-11-16 · 5be2569* · exit 0 · `set -o pipefail …` · acceptance-sha256:6d57a05172e2e1e1595ea99a0588140cf4c3abba367a38afefa9bbf48a8698de · ms:5975
- 2026-11-16 · 5be2569* · exit 0 · `set -o pipefail …` · acceptance-sha256:6d57a05172e2e1e1595ea99a0588140cf4c3abba367a38afefa9bbf48a8698de · ms:3332
