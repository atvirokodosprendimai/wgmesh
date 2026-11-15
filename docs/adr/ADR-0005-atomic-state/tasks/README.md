# ADR-0005 Tasks

Implementation tasks for ADR-0005: Give every piece of shared state one owner. See the parent ADR for the decision.

**Source of truth:** the task files' `Depends-on` / `Produces` / `Consumes` / `Covers` headers.
This README is a derived index — when it disagrees with a task file, the task file wins and the
README must be regenerated.

## Execution Order

| Wave | Tasks | Depends-on |
|------|-------|------------|
| 1 | T1, T5, T7 | none |
| 2 | T2, T6, T8 | T1 (T2), T5 (T6), T7 (T8) |
| 3 | T3, T4, T9 | T2 (T3, T4), T8 (T9) |
| 4 | T10 | T9 |
| 5 | T11 | T3, T4, T6, T8, T9, T10 |

## Task Index

| ID | Title | Status | Covers | Acceptance |
|----|-------|--------|--------|------------|
| T1 | Add `atomicfile.WriteFile`, the one commit step for state files | done | — | five named tests, then `-race` |
| T2 | Write the four in-place state files through `atomicfile` (#870) | pending | — | five named tests in three packages, `os.WriteFile` count at most 10, then `-race` |
| T3 | Replace the four private temp-and-rename copies and fail the build on a new one | pending | — | two guard tests, then `-race` on mesh, promo and the root package |
| T4 | A corrupt identity file is not treated as absent (#870) | pending | — | five named tests, then `-race` |
| T5 | Take an exclusive per-interface lock before `join` touches the socket or the interface (#860) | pending | — | six named tests in two packages, then `-race` |
| T6 | `rpc.NewServer` owns its socket path before it removes anything (#860) | pending | — | four named tests, then `-race` |
| T7 | One `LocalNode` type, so a method added to it reaches the node the daemon runs | pending | — | one named test, build, vet, then `-race` on three packages |
| T8 | Put the local node's mutable fields under one lock | pending | — | four named tests under `-race`, build, vet, then `-race` on three packages |
| T9 | Register every exchange caller's reply channel separately (#855) | pending | — | four named tests, then `-race` |
| T10 | Make `PeerExchange` and `DHTDiscovery` restart after `Stop` (#863, the part PR #884 left) | pending | — | four named tests, then `-race` |
| T11 | Amend the specs, write the release note and the backlog rows | pending | — | `go test ./pkg/atomicfile -run TestSpecNamesAtomicState`, note exists, `adr-debt` reports no unreceipted row |

## Contract Coupling

| Producer | Contract | Consumer(s) | Ordering note |
|----------|----------|-------------|---------------|
| T1 | `atomicfile.WriteFile` | T2, T3, T4 | T1 before each |
| T2 | `saveLocalNode`, `SavePeerCache`, `Mesh.Save`, `saveSecretFile` write atomically | T3, T4 | T2 before each; `saveLocalNode` keeps its signature for ADR-0002 T5 |
| T3 | `TestNoInPlaceStateWrite` and its allowlist | T11 | T3 before T11 |
| T4 | `ErrStateMissing`, `ErrStateCorrupt`, `RefuseCorruptState` | T11 | T4 before T11 |
| T5 | `flock.TryLock`, `daemon.InterfaceLockPath` | T6 | T5 before T6 |
| T6 | socket lock in `rpc.NewServer` | T11 | T6 before T11 |
| T7 | `daemon.LocalNode` moves to `pkg/node` and becomes an alias | T8 | T7 before T8; T7 after ADR-0002 T9 |
| T8 | `LocalNode` accessors, `Snapshot()` | T9, T11 | T8 before T9 (same file, `exchange.go`) |
| T9 | `registerReply` | T10, T11 | T9 before T10 (same file) |
| T10 | restart-safe `listenLoop`, `DHTDiscovery.beginRun` | T11 | T10 before T11 |

## Ordering against ADR-0002

ADR-0002 runs first on `task/adr-0002-exec` (T1 to T11). T1, T2, T3 of this record touch no hunk ADR-0002 edits (`helpers.go`, `mesh.go`, `account.go`, `services.go`, `promo/store.go`, `main.go:321` and `:1583`, `cache.go:90`). T4 follows ADR-0002 T5 (`initLocalNode`). T5 follows ADR-0002 T10 (`main.go`). T7 runs after ADR-0002 T9 (it moves the type that T9 extended). T8, T9 and T10 follow ADR-0002 T9 (`exchange.go`, `dht.go`, `pkg/node/types.go`). T11 is last.

## Notes

- Based on `origin/main` `b8b83f3` (ADR-0002 merged as a record, its tasks not built).
- Go tests run with `-race` for the packages each fence names; the whole gate is `go build ./... && go vet ./... && go test -race ./...`.
- The waves show dependencies, not safe parallelism: `exchange.go` is edited by T8, T9 and T10, `daemon.go` by T4, T7 and T8, `main.go` by T2, T3 and T5. Run the tasks one after another in the order T1 to T11 on one branch, with ADR-0002's tasks placed as the Ordering section says.
- Each code task is reviewed per commit under the repository's code-review flow before the next starts.
- N+1 (`RefuseCorruptState`) is not a task; it is a row in `docs/adr/BACKLOG.md` written by T11, and the ADR's catalog row is added when M accepts.
