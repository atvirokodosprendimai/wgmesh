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
3. [S3] Add `InterfaceLockPath`, `acquireInterfaceLock(path)`, and the `joinCmd` call, placed before `handleAccountFlag` and `recordReferralCode` as well (a refused join must not overwrite the running node's account file); the message reads like: `another wgmesh join holds the lock /var/lib/wgmesh/wg0.lock (pid 4242); stop it first (systemctl stop wgmesh, or brew services stop wgmesh)`; a permission error adds `run wgmesh join as root (or as the user that owns /var/lib/wgmesh)`.
4. [S4] [proof: human: whether `join` under systemd on a real host exits 1 and leaves the first service running was not run here; the test covers the lock and the order, a one-time check on a development host is the sign-off] Start the service, run `wgmesh join` by hand, confirm exit 1, the service's socket and interface intact; record host, date and release in the sign-off line.
5. [S5] Run the mutants with `adr-verify --mutant`: `TryLock` ignores `EWOULDBLOCK`; the lock is taken after `createRPCServer`; the release is called before the daemon exits. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/flock . -run '^(TestTryLockExcludesSecondHolder|TestTryLockReportsHolderPid|TestTryLockReleaseAllowsNext|TestTryLockFreedWhenHolderDies|TestAcquireInterfaceLockMessage|TestJoinTakesLockBeforeDaemonAndSocket|TestTryLockEmptyHolderHasNoPid|TestTryLockTruncatesStalePid|TestTryLockReleaseTwiceKeepsReusedDescriptor|TestTryLockReleaseIsSafeConcurrently|TestAcquireInterfaceLockCreatesDirectory0700|TestAcquireInterfaceLockHolderWithoutPid|TestAcquireInterfaceLockPermissionHint)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 12 ] && go test -race ./pkg/flock ./pkg/daemon -count=1
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
| `TestTryLockEmptyHolderHasNoPid` | `pkg/flock/flock_test.go` | a lock held without a pid written gives a `HeldError` with an empty holder | — | S1, S2 |
| `TestTryLockTruncatesStalePid` | `pkg/flock/flock_test.go` | an old pid line is replaced by exactly the new pid line | — | S1, S2 |
| `TestTryLockReleaseTwiceKeepsReusedDescriptor` | `pkg/flock/flock_test.go` | a second `release` does not touch a descriptor number reused by another file | — | S1, S2 |
| `TestTryLockReleaseIsSafeConcurrently` | `pkg/flock/flock_test.go` | 16 goroutines call `release` under `-race` | — | S1, S2 |
| `TestAcquireInterfaceLockPermissionHint` | `main_lock_test.go` | an unwritable lock directory gives the error plus the run-as-root hint (skipped as root) | — | S1, S3 |
| `TestAcquireInterfaceLockCreatesDirectory0700` | `main_lock_test.go` | a missing parent directory is created with mode 0700 | — | S1, S3 |
| `TestAcquireInterfaceLockHolderWithoutPid` | `main_lock_test.go` | a holder that has not written its pid prints `pid unknown` | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the six tests |
| 2 — something selects it | `joinCmd` calls `acquireInterfaceLock`; the order test is red without it |
| 3 — the caller can discover it | the error message; T11 amends the lifecycle spec and the release note |
| 4 — it is used | every `wgmesh join`, including the systemd service |

## Mutation Log
- 2026-11-16 · 2a5f687 · mutant killed · exit 1 · `pkg/flock/flock.go` · a held lock is not reported as held: TestTryLockExcludesSecondHolder must go red · acceptance-sha256:80669b0252c24df8b8abcbb2f2ad1c9f99e152c6757846fc4558bcaf091d3e43
- 2026-11-16 · 2a5f687* · mutant killed · exit 1 · `main.go` · joinCmd no longer takes the lock: TestJoinTakesLockBeforeDaemonAndSocket must go red · acceptance-sha256:80669b0252c24df8b8abcbb2f2ad1c9f99e152c6757846fc4558bcaf091d3e43
- 2026-11-16 · 2a5f687* · mutant killed · exit 1 · `main.go` · the lock is released at once: TestJoinTakesLockBeforeDaemonAndSocket (defer check) must go red · acceptance-sha256:80669b0252c24df8b8abcbb2f2ad1c9f99e152c6757846fc4558bcaf091d3e43
- 2026-11-16 · 2a5f687* · mutant killed · exit 1 · `pkg/flock/flock.go` · the holder pid is not written: TestTryLockReportsHolderPid must go red · acceptance-sha256:80669b0252c24df8b8abcbb2f2ad1c9f99e152c6757846fc4558bcaf091d3e43
- 2026-11-16 · 2a5f687* · mutant killed · exit 1 · `pkg/flock/flock.go` · a stale pid line survives: TestTryLockTruncatesStalePid must go red · acceptance-sha256:80669b0252c24df8b8abcbb2f2ad1c9f99e152c6757846fc4558bcaf091d3e43
- 2026-11-16 · 2a5f687* · mutant killed · exit 1 · `pkg/flock/flock.go` · the second TryLock blocks instead of failing: TestTryLockExcludesSecondHolder must go red (bounded wait) · acceptance-sha256:80669b0252c24df8b8abcbb2f2ad1c9f99e152c6757846fc4558bcaf091d3e43
- 2026-11-16 · f04f321 · mutant killed · exit 1 · `pkg/flock/flock.go` · a held lock is not reported as held (fence with the final test list) · acceptance-sha256:14f7a42d3ef5eb28b14b093c72308fbde88a63f59a155b6b67afc6e4993e74db
- 2026-11-16 · f04f321* · mutant killed · exit 1 · `main.go` · the lock is released at once (final fence) · acceptance-sha256:14f7a42d3ef5eb28b14b093c72308fbde88a63f59a155b6b67afc6e4993e74db
- 2026-11-16 · f04f321* · mutant killed · exit 1 · `pkg/flock/flock.go` · a stale pid line survives (final fence) · acceptance-sha256:14f7a42d3ef5eb28b14b093c72308fbde88a63f59a155b6b67afc6e4993e74db
- 2026-11-16 · f04f321* · mutant killed · exit 1 · `pkg/flock/flock.go` · a held lock is not reported as held (final fence, all table tests) · acceptance-sha256:152408ee19737e0a0b79b7275f4b250262d4ecc8192396ea3d4c746855585212

## Invariants

- A single `join` per interface starts exactly as before (the lock file is created and held; it is never removed, and its content is the last holder's pid line only; the kernel drops the lock when the process dies).
- The lock is held for the life of the process; nothing releases it before `Run` returns.
- `pkg/flock` imports only the standard library.

## Risks

- A daemon of an older release does not take the lock, so a new `join` beside an old running daemon is not stopped (stated in the release note).
- `flock` on a network filesystem may not exclude; the state directory is local by convention (parent Risks).
- Test doubles: `localNodeStateDir` pointed at a temp directory; the child process of the death test.
- Behaviour taken on (panel 2026-11-16, not covered by a test): the lock path is `/var/lib/wgmesh/<iface>.lock` whatever `--state-dir` says (the interface is a host resource, so two joins with different state dirs on one interface are refused); `handleAccountFlag` and `recordReferralCode` now run after `NewConfig` and the lock, so a join whose config is invalid no longer saves the account key or the referral; `wgmesh install-service` starts the unit with `systemctl start`, which succeeds before a lock-refused `join` exits 1, so the install reports success while the unit crash-loops every 5 s (T11 release note); a lock file removed while a daemon runs lets a second join take a new inode (known limit, commented above `TryLock`; closing it needs a hook between open and flock); `pkg/flock` is `unix` only and `main.go` imports it unconditionally, so a non-unix build fails (Out of Scope: releases are linux and darwin); a daemon of an older release does not hold the lock; Docker Desktop bind mounts on macOS may not exclude across the VM; the lock file keeps the previous holder's pid until the next holder writes its own, so a joiner that reads in that window may print the old pid or `pid unknown`.

## Stop Condition

Stop and ask if `joinCmd` builds the RPC server or touches the interface before the interface name is known, or if another command (`service`, `install-service`) starts a daemon and needs the same lock.

## Out of Scope

- The RPC socket path shared across interfaces (T6).
- Whether teardown should delete an adopted interface (deferred: docs/adr/BACKLOG.md).
- Non-unix platforms (permanent: fact: releases are built for linux and darwin only; citation: file `.goreleaser.yml:15`).

## Verification Log
- 2026-11-16 · 2a5f687 · exit 0 · `set -o pipefail …` · acceptance-sha256:80669b0252c24df8b8abcbb2f2ad1c9f99e152c6757846fc4558bcaf091d3e43 · ms:17468
- 2026-11-16 · 2a5f687* · exit 0 · `set -o pipefail …` · acceptance-sha256:80669b0252c24df8b8abcbb2f2ad1c9f99e152c6757846fc4558bcaf091d3e43 · ms:17325
- 2026-11-16 · 2a5f687* · exit 0 · `set -o pipefail …` · acceptance-sha256:80669b0252c24df8b8abcbb2f2ad1c9f99e152c6757846fc4558bcaf091d3e43 · ms:19993
- 2026-11-16 · 2a5f687* · exit 0 · `set -o pipefail …` · acceptance-sha256:80669b0252c24df8b8abcbb2f2ad1c9f99e152c6757846fc4558bcaf091d3e43 · ms:17909
- 2026-11-16 · 2a5f687* · exit 0 · `set -o pipefail …` · acceptance-sha256:80669b0252c24df8b8abcbb2f2ad1c9f99e152c6757846fc4558bcaf091d3e43 · ms:18265
- 2026-11-16 · 2a5f687* · exit 0 · `set -o pipefail …` · acceptance-sha256:80669b0252c24df8b8abcbb2f2ad1c9f99e152c6757846fc4558bcaf091d3e43 · ms:19358
- 2026-11-16 · 2a5f687* · exit 0 · `set -o pipefail …` · acceptance-sha256:80669b0252c24df8b8abcbb2f2ad1c9f99e152c6757846fc4558bcaf091d3e43 · ms:20105
- 2026-11-16 · 2a5f687* · exit 1 · `set -o pipefail …` · acceptance-sha256:80669b0252c24df8b8abcbb2f2ad1c9f99e152c6757846fc4558bcaf091d3e43 · ms:1729 · test-lock-sha256:071613171a6d873010e30f68335bb2bbdf99a718cebffa90fa13cb5a9323e848 · test-lock-file:docs/adr/locks/071613171a6d873010e30f68335bb2bbdf99a718cebffa90fa13cb5a9323e848.lock
  ```
  --- last 10 line(s) of stderr (of 22 after folding 22 raw)
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/flock	0.005s
  === RUN   TestAcquireInterfaceLockMessage
      main_lock_test.go:32: second acquire succeeded while the first is held
  --- FAIL: TestAcquireInterfaceLockMessage (0.00s)
  === RUN   TestJoinTakesLockBeforeDaemonAndSocket
      main_lock_test.go:82: joinCmd does not call acquireInterfaceLock
  --- FAIL: TestJoinTakesLockBeforeDaemonAndSocket (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh	0.010s
  FAIL
  ```
- 2026-11-16 · f04f321 · exit 0 · `set -o pipefail …` · acceptance-sha256:14f7a42d3ef5eb28b14b093c72308fbde88a63f59a155b6b67afc6e4993e74db · ms:17027
- 2026-11-16 · f04f321* · exit 0 · `set -o pipefail …` · acceptance-sha256:14f7a42d3ef5eb28b14b093c72308fbde88a63f59a155b6b67afc6e4993e74db · ms:20330
- 2026-11-16 · f04f321* · exit 0 · `set -o pipefail …` · acceptance-sha256:14f7a42d3ef5eb28b14b093c72308fbde88a63f59a155b6b67afc6e4993e74db · ms:22601
- 2026-11-16 · f04f321* · exit 0 · `set -o pipefail …` · acceptance-sha256:14f7a42d3ef5eb28b14b093c72308fbde88a63f59a155b6b67afc6e4993e74db · ms:17670
- 2026-11-16 · f04f321* · exit 0 · `set -o pipefail …` · acceptance-sha256:152408ee19737e0a0b79b7275f4b250262d4ecc8192396ea3d4c746855585212 · ms:21967
- 2026-11-16 · f04f321* · exit 0 · `set -o pipefail …` · acceptance-sha256:152408ee19737e0a0b79b7275f4b250262d4ecc8192396ea3d4c746855585212 · ms:14752
