# ADR-0006 Tasks

Implementation tasks for ADR-0006: Plan, quote and report every node of a centralized deploy. See the parent ADR for the decision.

**Source of truth:** the task files' `Depends-on` / `Produces` / `Consumes` / `Covers` headers.
This README is a derived index — when it disagrees with a task file, the task file wins and the
README must be regenerated.

## Execution Order

| Wave | Tasks | Depends-on |
|------|-------|------------|
| 1 | T1 | none |
| 2 | T2 | T1 |
| 3 | T3, T4 | T2 (and T1 for T3) |
| 4 | T5 | T4 |
| 5 | T6 | T3, T4, T5 |

## Task Index

| ID | Title | Status | Covers | Acceptance |
|----|-------|--------|--------|------------|
| T1 | Add `ssh.Quote`, `ssh.Cmd` and a Linux interface grammar that ignores GOOS | done | — | six named tests in two packages, then `-race` |
| T2 | Send every remote command through `ssh.Cmd`, render wg-quick safely, and fail the build on a raw one | done | — | six named tests in two packages, build, vet, then `-race` |
| T3 | Validate the loaded state before the first connection (#868) | done | — | seven named tests, build, vet, then `-race` |
| T4 | Record a result per node and continue past a failed one (#865) | done | — | eight named tests, build, vet, then `-race` |
| T5 | Decide a node's endpoint in one pure function, and skip the probe for private hosts (#866) | pending | — | five named tests, build, vet, then `-race` |
| T6 | Amend the specs, write the release note and the backlog rows | pending | — | `go test ./pkg/mesh -run TestSpecNamesCentralized`, note exists, `adr-debt` reports no unreceipted row |

## Contract Coupling

| Producer | Contract | Consumer(s) | Ordering note |
|----------|----------|-------------|---------------|
| T1 | `ssh.Quote`, `ssh.Cmd`, `ifname.ValidateLinux` | T2, T3 | T1 before each |
| T2 | every remote command through `Cmd`, `GenerateWgQuickConfig` returning an error, `pkg/ssh/sshtest`, `TestNoUnquotedRemoteCommand` | T3, T4, T5 | T2 before each (same files: `deploy.go`; the fake server) |
| T3 | `Mesh.Validate`, `Finding`, `StrictState` | T6 | T3 before T6 |
| T4 | `NodeResult`, `DeployError`, `PersistFailureFatal`, `deployNode` | T5, T6 | T4 before each (same file: `deploy.go`) |
| T5 | `resolveEndpoint`, the probe skip | T6 | T5 before T6 |

## Notes

- Based on `origin/main` `b8b83f3`. ADR-0002's tasks are on `task/adr-0002-exec` and not on main; this record depends on none of its code. Overlap: none of the files in the Affected Files tables is in ADR-0002's Governs list. `README.md` and `docs/adr/BACKLOG.md` are edited by this record's T6 and by ADR-0002's T11: both append, so the merge order does not matter.
- Go tests run on the laptop with `-race` for the packages each fence names; the whole gate is `go build ./... && go vet ./... && go test -race ./...`.
- The waves show dependencies, not safe parallelism: `deploy.go` is edited by T2, T3, T4 and T5. Run the tasks one after another in the order T1 to T6 on one branch.
- Each code task is reviewed per commit under the repository's code-review flow before the next starts.
- N+1 (`StrictState`) and N+2 (`PersistFailureFatal`) are not tasks; they are rows in `docs/adr/BACKLOG.md` written by T6, and the ADR's catalog row stays out of `docs/adr/README.md` until M accepts.
- M's decisions of 2026-11-15 (ADR Decisions 1 to 7, all the recommended options) are built into these tasks: 1 and 6 in T4, 2 and 7 in T3, 3 and 4 in T5, 5 adds no task (a BACKLOG row, written by T6).
