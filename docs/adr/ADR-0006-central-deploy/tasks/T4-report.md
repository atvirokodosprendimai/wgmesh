# Task ADR-0006-T4: Record a result per node and continue past a failed one (#865)

**Depends-on:** T2
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `NodeResult`, `Phase`, `DeployError`, `mesh.PersistFailureFatal`, `deployNode`
**Consumes:** `pkg/ssh/sshtest` (T2)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`Deploy` visits the nodes in sorted order, configures every reachable node whatever another node does, never dials a node twice after it failed to connect, and ends with one error that lists each failed node with its phase, or nil.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/mesh/report.go` | add | `Phase` (`detect`, `connect`, `install`, `apply`, `routes`, `persist`), `NodeResult{Hostname, Phase, Err, Warnings}`, `DeployError` (its `Error()` lists failed nodes one per line and the nodes that succeeded), `PersistFailureFatal` (`false`) |
| `pkg/mesh/deploy.go` | edit | the apply loop body becomes `deployNode(hostname, node) NodeResult`; the loop sorts hostnames and collects results; `detectEndpoints` returns per-node failures and the apply loop skips a node that failed in `detect`; the config file write failure (`:90-92`) becomes a warning in the result, or a `persist` failure when `PersistFailureFatal` |
| `pkg/mesh/deploy_report_test.go` | add | tests below |

`main.go:176-181` is not edited: `Deploy` already returns an error that it prints. Selection: `Deploy` is the only caller; deleting the collection turns `TestDeployContinuesPastUnreachableNode` red.

## Ordered Steps

1. [S1] Add the tests with a stub that keeps the old early return; run red: `TestDeployContinuesPastUnreachableNode` fails because the second node is never configured. Nodes come from `sshtest` servers; the unreachable node is a closed local port (no timeout wait).
2. [S2] Add `report.go` and `deployNode`; keep the lock discipline of PR #842: `Deploy` holds `m.mu` for the loop and `deployNode` calls only the helpers that require it held (`generateConfigForNode`, `collectAllRoutesForNode`), never `collectRoutesForNode` or a method that locks.
3. [S3] Sort the hostnames; make `detectEndpoints` record a `connect`-phase failure and `continue`; skip such a node in the apply loop.
4. [S4] Make the persist failure a warning at N and fatal under `PersistFailureFatal`.
5. [S5] Run the whole of `go test -race ./pkg/mesh` including `TestDeploymentConfigUnderLock`; record the exit line in the commit message. [proof: acceptance]
6. [S6] Run the mutants with `adr-verify --mutant`: the loop returns on the first failure; a failed `detect` node is dialled again; the hostnames are not sorted; `PersistFailureFatal` is ignored; `DeployError` omits the phase. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/mesh -run '^(TestDeployContinuesPastUnreachableNode|TestDeployReportsEveryFailureWithPhase|TestDeployAllReachableReturnsNil|TestDeployVisitsNodesInSortedOrder|TestDeployDoesNotRedialAFailedDetect|TestPersistWriteFailureIsAWarningAtN|TestPersistWriteFailureFailsWhenFatal|TestDeploymentConfigUnderLock)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 8 ] \
 && go build ./... && go vet ./... && go test -race ./pkg/mesh -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestDeployContinuesPastUnreachableNode` | `pkg/mesh/deploy_report_test.go` | with nodes `a` (closed port), `b` and `c` (servers) `b` and `c` receive their config and the error names `a` | — | S1, S2, S3 |
| `TestDeployReportsEveryFailureWithPhase` | `pkg/mesh/deploy_report_test.go` | two failing nodes (one `connect`, one `apply` through a scripted server failure) are both in the error text with their phase, once each | — | S1, S2 |
| `TestDeployAllReachableReturnsNil` | `pkg/mesh/deploy_report_test.go` | three servers, no failure: `Deploy` returns nil | — | S1, S2 |
| `TestDeployVisitsNodesInSortedOrder` | `pkg/mesh/deploy_report_test.go` | the order in which servers receive their first command is the sorted hostname order, over 20 runs | — | S1, S3 |
| `TestDeployDoesNotRedialAFailedDetect` | `pkg/mesh/deploy_report_test.go` | a node that fails in `detect` is accepted by a counting listener exactly once | — | S1, S3 |
| `TestPersistWriteFailureIsAWarningAtN` | `pkg/mesh/deploy_report_test.go` | a server that fails the config file write leaves the node `ok` with one warning and `Deploy` returns nil | — | S1, S4 |
| `TestPersistWriteFailureFailsWhenFatal` | `pkg/mesh/deploy_report_test.go` | the same with `PersistFailureFatal` returns an error with phase `persist` (reset with `t.Cleanup`) | — | S1, S4 |
| `TestDeploymentConfigUnderLock` | `pkg/mesh/deploy_test.go` | the PR #842 test still passes (exists today; this task must not break it) | — | S5 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the eight tests |
| 2 — something selects it | `Deploy` calls `deployNode` for each node; the early-return mutant turns the continue test red |
| 3 — the caller can discover it | the error text names nodes and phases; T6 documents it |
| 4 — it is used | every `wgmesh -deploy` with more than one node |

## Mutation Log

## Invariants

- A fully successful deploy prints what it printed and exits 0; a failing one exits 1 as before, after the other nodes ran.
- The configuration pushed to a node does not depend on whether any other node was reachable.
- `m.mu` is taken once per phase and never re-entered; the run holds it across the loop as since PR #842.
- Nothing in this task writes the state file (`Deploy` has no `Save`).

## Risks

- A failed run leaves some nodes one generation ahead; the summary lists them and a second run converges because each node step diffs against the live config.
- Sorting changes the order of the output lines for a mesh; the order was map order, i.e. random, so no script can rely on it.
- Test doubles: the fake server; the closed port is a real refused connection.

## Stop Condition

Stop and ask if a failure in one node's apply can corrupt the config of a node processed later (it cannot today: each node uses its own client), or if `detectEndpoints` and the apply loop need the lock in a way that cannot be held across both.

## Out of Scope

- `-only` and `-skip` (deferred: docs/adr/BACKLOG.md).
- Parallel node pushes (permanent: boundary: the sequential order is the report's order).
- Setting `PersistFailureFatal` (deferred: docs/adr/BACKLOG.md, the N+2 row written by T6).

## Verification Log
