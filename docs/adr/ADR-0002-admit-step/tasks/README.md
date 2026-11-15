# ADR-0002 Tasks

Implementation tasks for ADR-0002: Add one stateless admit step between peer discovery and the PeerStore. See the parent ADR for the decision.

**Source of truth:** the task files' `Depends-on` / `Produces` / `Consumes` / `Covers` headers.
This README is a derived index — when it disagrees with a task file, the task file wins and the
README must be regenerated.

## Execution Order

| Wave | Tasks | Depends-on |
|------|-------|------------|
| 1 | T1 | none |
| 2 | T2, T4 | T1 |
| 3 | T3, T5, T6, T7, T8, T9 | T2 (T3, T7, T8, T9) or T4 (T5, T6) |
| 4 | T10 | T6 |
| 5 | T11 | T3, T5, T6, T7, T8, T9, T10 |

## Task Index

| ID | Title | Status | Covers | Acceptance |
|----|-------|--------|--------|------------|
| T1 | Add `PeerStore.Offer` with an explicit hand, and pin the merged rules | done | — | `go test ./pkg/node` five named tests, then `-race` on node, daemon, discovery |
| T2 | Send every production claim through `Offer`, and fail the build on a direct one | done | — | five named tests, `grep` count of direct `Update` calls is 0, then `-race` |
| T3 | Keep a tombstone for removed peers and refuse second-hand re-adds (#854) | done | — | eight named tests in three packages, then `-race` |
| T4 | Put the local node into the mesh IP claim table and order claims (#856) | pending | — | seven named tests in two packages, then `-race` |
| T5 | Persist the collision-resolved mesh IP (#830) | pending | — | three named tests, then `-race` |
| T6 | Bind mesh IPs and routes to what the key derives, in warn mode, with frozen compat cases (#864) | pending | — | eight named tests in two packages, fixture file exists, then `-race` |
| T7 | One endpoint rule set: guessed endpoints rank last and are not re-announced (#839) | pending | — | six named tests in two packages, then `-race` |
| T8 | One validation path for announcements on LAN, gossip, registry and exchange (#875) | pending | — | five named tests in two packages (ADR-0001 fixtures included), then `-race` |
| T9 | Observe the local endpoint through one rule instead of eight writers (#828) | pending | — | four named tests, `SetEndpoint` count is 1, then `-race` |
| T10 | Approve non-mesh routes by an operator list on the receiving node, open when unset (#864) | pending | — | seven named tests in two packages, `accept-routes` flag present, then `-race` |
| T11 | Amend the specs, write the release notes and the backlog rows | pending | — | `go test ./pkg/node -run TestSpecNamesAdmit`, note exists, `adr-debt` reports no unreceipted row |

## Contract Coupling

| Producer | Contract | Consumer(s) | Ordering note |
|----------|----------|-------------|---------------|
| T1 | `Candidate`, `Hand`, `Source`, `Decision`, `Offer`, `Policy`, `SetPolicy` | T2, T4, T6 | T1 before each |
| T2 | every production claim goes through `Offer`; `TestNoDirectPeerStoreMutation` | T3, T7, T8, T9 | T2 before each (same files: `exchange.go`, `dht.go`, `gossip.go`, `lan.go`) |
| T3 | `Withdraw`, tombstones, `TombstoneTTL` | T11 | T3 before T11 |
| T4 | `Claim`, `ResolveClaims`, claim order, `EnforceIPBinding` | T5, T6 | T4 before each |
| T5 | persisted resolved IP | T11 | T5 before T11 |
| T6 | `EnforceRouteBinding`, compat fixtures, `wgmesh_admit_warnings_total` | T11 | T6 before T11 |
| T7 | `ResolveEndpoint`, `PeerInfo.EndpointGuessed` | T11 | T7 before T11 |
| T8 | `crypto.FilterAnnouncement` | T11 | T8 before T11 |
| T9 | `LocalNode.ObserveEndpoint` | T11 | T9 before T11 |
| T6 | `EnforceRouteBinding`, rule R5 (route part) | T10 | T6 before T10 (same switch, same `Decide`) |
| T10 | `Policy.AcceptRoutes`, `Config.AcceptRoutes`, rule R9 | T11 | T10 before T11 |

## Notes

- Based on `origin/main` `5edcdd0`. ADR-0001's tasks are merged there; T8 re-runs its fixture test.
- Go tests run on the laptop with `-race` for the packages each fence names; the whole gate is `go build ./... && go vet ./... && go test -race ./...`.
- The waves show dependencies, not safe parallelism: `Decide` in `admit.go` is edited by T3, T4 and T6, `daemon.go` by T3, T4, T5, T6 and T10, `store.go` by T1, T3 and T7, `exchange.go` by T2, T3, T7, T8 and T9. Run the tasks one after another in the order T1 to T11 on one branch.
- Each code task is reviewed per commit under the repository's code-review flow before the next starts.
- N+1 (`EnforceIPBinding`) and N+2 (`EnforceRouteBinding`) are not tasks; they are rows in `docs/adr/BACKLOG.md` written by T11, and the ADR's catalog row stays Proposed until M accepts.
