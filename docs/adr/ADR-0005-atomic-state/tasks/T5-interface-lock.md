# Task ADR-0005-T5: Take an exclusive per-interface lock before `join` touches the socket or the interface (#860)

**Depends-on:** none
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `flock.TryLock(path string) (release func(), holder string, err error)`; `daemon.InterfaceLockPath(iface string) string`; `acquireInterfaceLock` in `main.go`
**Consumes:** none
**Data dependency:** hermetic
**Proof map:** v1

## Goal

A second `wgmesh join` for an interface that a running daemon holds exits 1 with a message that names the lock file and the holder's pid, before it creates the RPC server (which unlinks the socket), resets the interface or binds a port.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/flock/flock.go` | add | build tag `unix`; `TryLock` opens or creates the file (0600), `syscall.Flock(fd, LOCK_EX\|LOCK_NB)`; on success truncates and writes the pid, returns a `release` that unlocks and closes; on `EWOULDBLOCK` reads the pid line and returns it as `holder`; the lock is dropped by the kernel when the process dies |
| `pkg/flock/flock_test.go` | add | tests below |
| `pkg/daemon/helpers.go` | edit | `InterfaceLockPath(iface)` returns `filepath.Join(localNodeStateDir, iface+".lock")` so the one state directory stays in the daemon package |
| `main.go` | edit | `acquireInterfaceLock(iface string) (func(), error)` makes the directory (0700), calls `TryLock`, and builds the message of M2; `joinCmd` calls it once the interface name is known and before `daemon.NewDaemon` (`:422`) and `createRPCServer` (`:459`), and defers the release; failure prints to stderr and `os.Exit(1)` as the neighbouring error paths do |
| `main_lock_test.go` | add | the order and message tests |

Selection: `joinCmd` is the only caller of `NewDaemon` (`grep -rn "NewDaemon(" --include=*.go . | grep -v _test.go` lists `main.go:422`); `TestJoinTakesLockBeforeDaemonAndSocket` fails when that call is removed or moved after `NewDaemon`.

Ordering note: follows ADR-0002 T10, which edits `main.go` flags near `:333` and `:788`; this task edits `joinCmd` at `:422-459`.

## Ordered Steps

1. [S1] Add the tests with a compiling stub (`TryLock` that always succeeds, `acquireInterfaceLock` that returns a no-op) and run red.
2. [S2] Implement `TryLock`; `release` is idempotent.
3. [S3] Add `InterfaceLockPath`, `acquireInterfaceLock`, and the `joinCmd` call; the message reads like: `another wgmesh join holds interface wg0 (lock /var/lib/wgmesh/wg0.lock, pid 4242); stop it first`.
4. [S4] [proof: human: whether `join` under systemd on a real host exits 1 and leaves the first service running was not run here; the test covers the lock and the order, a one-time check on a development host is the sign-off] Start the service, run `wgmesh join` by hand, confirm exit 1, the service's socket and interface intact; record host, date and release in the sign-off line.
5. [S5] Run the mutants with `adr-verify --mutant`: `TryLock` ignores `EWOULDBLOCK`; the lock is taken after `createRPCServer`; the release is called before the daemon exits. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/flock . -run '^(TestTryLockExcludesSecondHolder|TestTryLockReportsHolderPid|TestTryLockReleaseAllowsNext|TestTryLockFreedWhenHolderDies|TestAcquireInterfaceLockMessage|TestJoinTakesLockBeforeDaemonAndSocket)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 6 ] && go test -race ./pkg/flock ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestTryLockExcludesSecondHolder` | `pkg/flock/flock_test.go` | a second `TryLock` on the same path, in the same process, fails while the first is held | — | S1, S2 |
| `TestTryLockReportsHolderPid` | `pkg/flock/flock_test.go` | the error carries the pid the first holder wrote | — | S1, S2 |
| `TestTryLockReleaseAllowsNext` | `pkg/flock/flock_test.go` | after `release` (called twice) a new `TryLock` succeeds | — | S1, S2 |
| `TestTryLockFreedWhenHolderDies` | `pkg/flock/flock_test.go` | the test binary re-executes itself as a child that takes the lock; the parent kills the child with `cmd.Process.Kill` (its own child, never `pkill`) and the next `TryLock` succeeds | — | S1, S2 |
| `TestAcquireInterfaceLockMessage` | `main_lock_test.go` | with the lock held the error names the lock path and the pid | — | S1, S3 |
| `TestJoinTakesLockBeforeDaemonAndSocket` | `main_lock_test.go` | parsing `main.go`: in `joinCmd` the call to `acquireInterfaceLock` precedes `daemon.NewDaemon` and `createRPCServer` | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the six tests |
| 2 — something selects it | `joinCmd` calls `acquireInterfaceLock`; the order test is red without it |
| 3 — the caller can discover it | the error message; T11 amends the lifecycle spec and the release note |
| 4 — it is used | every `wgmesh join`, including the systemd service |

## Mutation Log

## Invariants

- A single `join` per interface starts exactly as before (the lock file is created, held, and removed or left empty at exit; its content is a pid line only).
- The lock is held for the life of the process; nothing releases it before `Run` returns.
- `pkg/flock` imports only the standard library.

## Risks

- A daemon of an older release does not take the lock, so a new `join` beside an old running daemon is not stopped (stated in the release note).
- `flock` on a network filesystem may not exclude; the state directory is local by convention (parent Risks).
- Test doubles: `localNodeStateDir` pointed at a temp directory; the child process of the death test.

## Stop Condition

Stop and ask if `joinCmd` builds the RPC server or touches the interface before the interface name is known, or if another command (`service`, `install-service`) starts a daemon and needs the same lock.

## Out of Scope

- The RPC socket path shared across interfaces (T6).
- Whether teardown should delete an adopted interface (deferred: docs/adr/BACKLOG.md).
- Non-unix platforms (permanent: fact: releases are built for linux and darwin only; citation: file `.goreleaser.yml:15`).

## Verification Log
