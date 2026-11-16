# ADR-0004 Tasks

Implementation tasks for ADR-0004: Decide that a peer is alive from all the evidence the node has, in one stateless step. See the parent ADR for the decision.

**Source of truth:** the task files' `Depends-on` / `Produces` / `Consumes` / `Covers` headers.
This README is a derived index — when it disagrees with a task file, the task file wins and the
README must be regenerated.

## Execution Order

| Wave | Tasks | Depends-on |
|------|-------|------------|
| 1 | T1 | none |
| 2 | T2 | T1 |
| 3 | T3, T4, T6 | T2 |
| 4 | T5 | T4 |
| 5 | T7 | T3, T5, T6 |

## Task Index

| ID | Title | Status | Covers | Acceptance |
|----|-------|--------|--------|------------|
| T1 | Add the pure `Assess` and make the store's three liveness readers use it | done | — | `go test ./pkg/node` eleven named tests, then `-race` on node, daemon, discovery |
| T2 | Feed the handshake and the probe into the store and move every liveness reader to `Alive` (#850) | pending | — | eight named tests in two packages, `LastSeen` age grep count is 0 outside `pkg/node`, then `-race` |
| T3 | Treat a restored cache entry as dialable for `PeerDeadTimeout` (#851) | pending | — | five named tests in two packages plus the existing cache tests, then `-race` |
| T4 | Detect a wake from sleep and give the peers a grace period (#829) | pending | — | nine named tests, then `-race`; a human step times a real wake |
| T5 | Make the DHT layer refresh its endpoint and announce at once on a wake (#829) | pending | — | eight named tests in two packages, then `-race` |
| T6 | Keep a relayed peer on the interface without AllowedIPs so a direct handshake can bring it back (#852) | pending | — | twelve named tests in two packages, then `-race`; a human step runs `wg set` on a real interface |
| T7 | Amend the specs, write the release notes and the backlog rows | pending | — | `go test ./pkg/node -run TestSpecNamesLiveness`, note exists, the `will add` sentences are reworded, `adr-debt` reports no unreceipted row |

## Contract Coupling

| Producer | Contract | Consumer(s) | Ordering note |
|----------|----------|-------------|---------------|
| T1 | `Assess`, `PeerInfo.Alive`, `PeerInfo.LastAlive`, `ObserveDataPlane`, `NoteResume`, `ResumedAt`, `GetProbeable`, `EnforceLiveness`, `SetLivenessDisagreeHook` | T2, T3, T4, T6 | T1 before each |
| T2 | evidence writers, every reader on `Alive`, `TestNoDirectLastSeenAge` | T3, T4, T6 | T2 before each (same files: `daemon.go`, `store.go`) |
| T3 | `RestoredAt` set by the `Restored` merge, `LastAlive` written to the cache file | T7 | T3 before T7 |
| T4 | `DetectResume`, `ResumeGrace`, `node.Resumer` | T5, T7 | T4 before T5 |
| T5 | `DHTDiscovery.OnResume`, `daemon.EndpointSTUNResume` | T7 | T5 before T7 |
| T6 | `probeOnly`, `SetPeerArgs`, `setPeer` seam, `EnforceRelayProbe`, `wgmesh_relay_route_age_seconds` | T7 | T6 before T7 |

## Notes

- Based on `origin/main` `f3860c6` (ADR-0002 merged; the worktree is at `fdd5af2`, which merges it into this branch). Every prerequisite on ADR-0002 is met, M confirmed the R7 amendment (M5) on 2026-11-15 22:46, and T6 is rebased on the apply-step ADR (ADR-0003 draft) if that lands first. Each commit message names the base commit, `f3860c6`, or the later main commit it was rebuilt on.
- Go tests run on the laptop with `-race` for the packages each fence names; the whole gate is `go build ./... && go vet ./... && go test -race ./...`.
- The waves show dependencies, not safe parallelism: `store.go` is edited by T1 and T3, `daemon.go` by T2, T4, T5 and T6, `dht.go` by T2 and T5. Run the tasks one after another in the order T1 to T7 on one branch.
- Each code task is reviewed per commit under the repository's code-review flow before the next starts.
- N+1 (`EnforceLiveness`) and N+2 (`EnforceRelayProbe`) are not tasks; they are rows in `docs/adr/BACKLOG.md` written by T7.
- Every code task carries the invariant that no field, variable or counter accessed by more than one goroutine is bare (M 2026-11-16) and a `-race` test for each shared field it adds.
