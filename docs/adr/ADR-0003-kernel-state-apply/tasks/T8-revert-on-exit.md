# Task ADR-0003-T8: Undo the FORWARD rule and ip_forward at graceful exit (#872)

**Depends-on:** T4, T9
**Awaiting-decision:** Open Question 4 of ADR-0003: revert at exit (A, recommended), behind a new flag (B), or never (C). Withdraw this task if the answer is C.
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `EnforceRevertOnExit`, `Daemon.revertKernel`, the ownership record of what this run changed
**Consumes:** the forwarding target and its observed values (T4); the specs and note (T9)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

At a graceful exit the daemon removes the FORWARD rule only if this run added it and restores `net.ipv4.ip_forward` only if this run changed it from 0; at release N it only logs what it will do from N+1.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/converge.go` | edit | the forwarding apply records `addedRule` (the rule was absent and `-A` succeeded) and `prevIPForward` (the value read before the first write); `revertKernel` issues `iptables -D` and the sysctl restore from those records; `EnforceRevertOnExit` (package variable, `false`) |
| `pkg/daemon/daemon.go` | edit | `Run` calls `revertKernel` after `d.wg.Wait()` and before the deferred `teardownWireGuard` (`:233`, `:292-295`) |
| `docs/kernel-state.md`, `pkg/daemon/spec_converge_test.go` | edit | the note and the spec test name the revert and its switch |
| `pkg/daemon/revert_test.go` | add | the four tests below |

Selection: `Run` calls `revertKernel`; deleting the call turns `TestShutdownRemovesRuleWeAdded` red.

## Ordered Steps

1. [S1] Add the four tests with compiling stubs and run red.
2. [S2] Record ownership in the forwarding apply and implement `revertKernel`.
3. [S3] Wire it into `Run`; at N with `EnforceRevertOnExit` false it logs once what N+1 will undo and issues no command.
4. [S4] Run the mutants with `adr-verify --mutant`: the rule is deleted whether or not this run added it; `ip_forward` is restored to 0 when it was already 1; the record is lost when the rule already existed at start. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/daemon -run '^(TestShutdownRemovesRuleWeAdded|TestShutdownKeepsPreexistingRule|TestShutdownRestoresIPForwardOnlyIfWeChangedIt|TestShutdownRevertIsWarnOnlyAtN)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 4 ] && go test -race ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestShutdownRemovesRuleWeAdded` | `pkg/daemon/revert_test.go` | with the switch on, a run that appended the rule deletes it at exit | — | S1, S2, S3 |
| `TestShutdownKeepsPreexistingRule` | `pkg/daemon/revert_test.go` | a rule that `-C` found at the first tick is not deleted at exit | — | S1, S2 |
| `TestShutdownRestoresIPForwardOnlyIfWeChangedIt` | `pkg/daemon/revert_test.go` | `ip_forward` read as 0 and written 1 is written 0 at exit; read as 1 it is left | — | S1, S2 |
| `TestShutdownRevertIsWarnOnlyAtN` | `pkg/daemon/revert_test.go` | with the switch off exit issues no command and prints one line | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the four tests |
| 2 — something selects it | `Run` calls `revertKernel`; deleting the call turns the first test red |
| 3 — the caller can discover it | the log line at N, the note (T9) |
| 4 — it is used | every graceful exit |

## Mutation Log

## Invariants

- A crash or `SIGKILL` leaves the host as today; nothing here claims otherwise.
- Nothing is undone that this run did not do.
- The interface teardown (`teardownWireGuard`) is unchanged.

## Risks

- A restart (upgrade, `Restart=always`) opens a short gap in forwarding for other services on the host; the note says so, and the answer to Open Question 4 may choose B.
- Another program that enabled the sysctl after us is not detected; we restore only the value we read.
- Test doubles: `MockCommandExecutor` and the file-read variable of T4.

## Stop Condition

Stop and ask if Open Question 4 is not answered, or if `Run` has more than one exit path that skips the deferred calls.

## Out of Scope

- Reverting after a crash (permanent: boundary: no persisted record of what a run changed is kept; that would be new state).
- Reverting routes and addresses (permanent: boundary: `teardownWireGuard` already deletes the interface, which removes both).

## Verification Log
