# ADR-0003 Tasks

Implementation tasks for ADR-0003: Add one observe, diff, apply step between the desired kernel state and the kernel. See the parent ADR for the decision.

**Source of truth:** the task files' `Depends-on` / `Produces` / `Consumes` / `Covers` headers.
This README is a derived index — when it disagrees with a task file, the task file wins and the
README must be regenerated.

## Execution Order

| Wave | Tasks | Depends-on |
|------|-------|------------|
| 1 | T1 | none (Blocked-on: ADR-0002's route approval task on main) |
| 2 | T2 | T1 (Blocked-on: the same) |
| 3 | T3 | T2 |
| 4 | T4, T7 | T3 (T4); T3 (T7, Blocked-on: ADR-0002's persist-IP task on main) |
| 5 | T5 | T4 |
| 6 | T6 | T5 |
| 7 | T9 | T3, T4, T5, T6, T7 (Blocked-on: ADR-0002's documentation task on main) |
| 8 | T8 | T4, T9 |

## Task Index

| ID | Title | Status | Covers | Acceptance |
|----|-------|--------|--------|------------|
| T1 | Make routes canonical per family and give each shared network one owner | done | — | `go test ./pkg/routes` eight named tests, then `-race` on routes, ssh, mesh, daemon |
| T2 | Build one desired state per tick for AllowedIPs and routes | done | — | eight named tests, three pinned tests unchanged, `ExcludeOwn` call count outside `desired.go` is 0, then `-race` |
| T3 | Compare peers with the kernel, and move every peer writer into converge | done | — | eleven named tests in two packages, `lastAppliedPeerConfigs` count is 0, then `-race` |
| T4 | Converge ip_forward and the FORWARD rule once, and log a missing iptables once (#872) | done | — | seven named tests, then `-race` |
| T5 | Converge routes for both families, and stop re-adding what is already there (#859) | done | — | ten named tests, no forwarding code left in `routes.go`, then `-race` |
| T6 | Install peers' advertised networks on macOS (#862) | pending | — | six named tests and a human sign-off on a Mac |
| T7 | Converge the interface address instead of writing it once | pending | — | five named tests, then `-race` |
| T8 | Undo the FORWARD rule and ip_forward at graceful exit (#872) | pending | — | four named tests, then `-race`; waits for decision 7 |
| T9 | Amend the specs, write the operator note and the backlog rows | pending | — | `go test ./pkg/daemon -run TestSpecNamesConverge`, note exists, `adr-debt` clean |

## Contract Coupling

| Producer | Contract | Consumer(s) | Ordering note |
|----------|----------|-------------|---------------|
| T1 | `routes.Canonical`, `routes.Resolve`, deterministic `CalculateDiff` | T2, T5, T6 | T1 before each |
| T2 | `desiredState`, `Daemon.desire` | T3, T4, T5, T6, T7 | T2 before each (same files: `daemon.go`, `desired.go`) |
| T3 | `kernelObserved`, `planKernel`, `applyKernel`, the result log, counters, `TestKernelWritesOnlyInConverge` | T4, T5, T7, T8 | T3 before each (same files: `converge.go`, `observe.go`) |
| T4 | forwarding target, `EnforceForwardScope` | T5, T8 | T4 before T5 (T5 removes the blind calls T4 replaces) |
| T5 | route target, `EnforceRouteGuard` | T6 | T5 before T6 |
| T6 | Darwin adapter | T9 | T6 before T9 |
| T7 | address target | T9 | T7 before T9 |
| T8 | revert on exit | none | after T9; amends the note and the spec test |

## Notes

- Based on `origin/main` `b8b83f3`. ADR-0002 is Accepted; its tasks are being executed on branch `task/adr-0002-exec` and are not on main. Three tasks carry a `Blocked-on` line with a command that exits 0 once the ADR-0002 task they follow has landed (the lint cannot resolve a qualified `Depends-on` here because the records of this corpus live in their own directories); nothing here depends on code that is not on main once those have landed.
- The waves show dependencies, not safe parallelism: `converge.go` and `observe.go` are edited by T3, T4, T5, T6 and T7, `desired.go` by T2, T4, T5 and T7, `daemon.go` by T2, T3 and T8. Run the tasks one after another in the order T1, T2, T3, T4, T5, T6, T7, T9, T8 on one branch.
- T6 needs a Mac for S1 and S6. M decided on 2026-11-15 (decision 7) that revert on exit is built, so T8 stays and runs last.
- Each code task is reviewed per commit under the repository's code-review flow before the next starts.
- N+1 (`EnforceForwardScope`, `EnforceRouteGuard`, `EnforceRevertOnExit`) is not a task; it is a row in `docs/adr/BACKLOG.md` written by T9, and the ADR's catalog row stays Proposed until M accepts.
