# ADR-0007 Tasks

Implementation tasks for ADR-0007: Give every long-lived collection and spawned process one bound and one owner. See the parent ADR for the decision.

**Source of truth:** the task files' `Depends-on` / `Produces` / `Consumes` / `Covers` headers.
This README is a derived index — when it disagrees with a task file, the task file wins and the
README must be regenerated.

## Execution Order

| Wave | Tasks | Depends-on |
|------|-------|------------|
| 1 | T1 | none |
| 2 | T2, T3, T5, T6, T7, T8 | the last task of ADR-0002 merged (T2, T3, T5, T6, T8), T1 (T3, T7) |
| 3 | T4 | T2, T3 |

## Task Index

| ID | Title | Status | Covers | Acceptance |
|----|-------|--------|--------|------------|
| T1 | Read `wg` state through one snapshot with a deadline, and give it a test seam | done | — | seven named tests in `pkg/wireguard`, no `exec.Command(` left in `apply.go` and `keys.go`, then `-race` |
| T2 | Bound the three DHT maps that never shrink | done | — | seven named tests in `pkg/discovery`, then `-race` |
| T3 | Move every `wg` state reader to the snapshot with a stated staleness | done | — | six named tests in two packages, direct-reader count is 0, then `-race` |
| T4 | Sweep `temporaryOffline` and guard every map field with a stated bound | done | — | five named tests in `pkg/daemon`, then `-race` |
| T5 | Replace the deprecated `Announce` with `AnnounceTraversal` (#836) | done | — | three named tests, no `.Announce(` left in `dht.go`, `go vet`, then `-race` |
| T6 | Print usage for an unknown command and honour `--version` only before the subcommand (#873) | done | — | four named `TestScript` cases, then `-race` |
| T7 | First tests of the `wg` wrappers through the fake-binary seam (#833) | done | — | seven named tests in `pkg/wireguard`, then `-race` |
| T8 | One join-secret lookup for `join`, `status`, `qr`, `install-service` and `wgmesh service` (#891) | done | — | four helper tests and four `TestScript` cases, then `-race` |

## Contract Coupling

| Producer | Contract | Consumer(s) | Ordering note |
|----------|----------|-------------|---------------|
| T1 | `wireguard.ReadState`, `State`, `SetWGPath`, `runWG` | T3, T7 | T1 before each |
| T2 | `ContactDedupWindow`, `MaxContactedPeers`, `sweepPeerKeyed` | T4 | T2 before T4 (the guard table names the rules) |
| T3 | every non-test caller reads through `ReadState` | T4 | T3 before T4 (same file `daemon.go`) |

## Notes

- Based on `origin/main` `b8b83f3`. ADR-0002 is Accepted but its code is not on main; T2, T3, T5, T6 edit files its tasks edit (`dht.go`: T2, T9; `exchange.go`: T2, T3, T7, T8, T9; `daemon.go`: T3, T4, T5, T6, T10; `main.go`: T10), so they start after the last task of ADR-0002 (its T11) is merged. T1 and T7 touch only `pkg/wireguard`, which no accepted record governs, and may run earlier.
- Run the tasks of wave 2 one after another: T2 and T3 both edit `dht.go`, T3 and T4 both edit `daemon.go`.
- Go tests run on the laptop with `-race` for the packages each fence names; the whole gate is `go build ./... && go vet ./... && go test -race ./...`.
- Each code task is reviewed per commit under the repository's code-review flow before the next starts.
- M decided on 2026-11-15 (Decisions 1 to 7 of the ADR): 1 shapes T3, 2 shapes T1, 3 shapes T2, 4 shapes T6, 5 and 7 are deferrals, 6 is a recommendation with no task.
