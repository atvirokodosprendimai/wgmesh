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
3. [S3] Sort the hostnames; make `detectEndpoints` record a `detect`-phase failure (a dial failure there) and `continue`; a dial failure in the apply loop (a local node, which detect skips) is a `connect`-phase failure; skip such a node in the apply loop.
4. [S4] Make the persist failure a warning at N and fatal under `PersistFailureFatal`.
5. [S5] Run the whole of `go test -race ./pkg/mesh` including `TestDeploymentConfigUnderLock`; record the exit line in the commit message. [proof: acceptance]
6. [S6] Run the mutants with `adr-verify --mutant`: the loop returns on the first failure; a failed `detect` node is dialled again; the hostnames are not sorted; `PersistFailureFatal` is ignored; `DeployError` omits the phase. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/mesh -run '^(TestDeployContinuesPastUnreachableNode|TestDeployReportsEveryFailureWithPhase|TestDeployAllReachableReturnsNil|TestDeployVisitsNodesInSortedOrder|TestDeployDoesNotRedialAFailedDetect|TestPersistWriteFailureIsAWarningAtN|TestPersistWriteFailureFailsWhenFatal|TestDeployRenderErrorIsAPersistFailureAndLeavesTheNodeUntouched|TestDeploymentConfigUnderLock)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -eq 9 ] \
 && go build ./... && go vet ./... && go test -race ./pkg/mesh -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestDeployContinuesPastUnreachableNode` | `pkg/mesh/deploy_report_test.go` | with nodes `a` (closed port), `b` and `c` (servers) `b` and `c` receive their config and the error names `a` | — | S1, S2, S3 |
| `TestDeployReportsEveryFailureWithPhase` | `pkg/mesh/deploy_report_test.go` | two failing nodes (one `connect`, one `apply` through a scripted server failure) are both in the error text with their phase, once each | — | S1, S2 |
| `TestDeployAllReachableReturnsNil` | `pkg/mesh/deploy_report_test.go` | three servers, no failure: `Deploy` returns nil | — | S1, S2 |
| `TestDeployVisitsNodesInSortedOrder` | `pkg/mesh/deploy_report_test.go` | the printed detect and deploy lines of both loops are in the sorted hostname order (sshtest records no arrival times) | — | S1, S3 |
| `TestDeployDoesNotRedialAFailedDetect` | `pkg/mesh/deploy_report_test.go` | a node that fails in `detect` is accepted by a counting listener exactly once | — | S1, S3 |
| `TestPersistWriteFailureIsAWarningAtN` | `pkg/mesh/deploy_report_test.go` | a server that fails the config file write prints one warning (and a `Warnings:` block) and `Deploy` returns nil, the config file written once as the last step of the node | — | S1, S4 |
| `TestPersistWriteFailureFailsWhenFatal` | `pkg/mesh/deploy_report_test.go` | the same with `PersistFailureFatal` returns an error with phase `persist` (reset with `t.Cleanup`) | — | S1, S4 |
| `TestDeploymentConfigUnderLock` | `pkg/mesh/deploy_test.go` | the PR #842 test still passes (exists today; this task must not break it) | — | S5 |
| `TestDeployRenderErrorIsAPersistFailureAndLeavesTheNodeUntouched` | `pkg/mesh/deploy_report_test.go` | a state whose wg-quick text cannot be rendered (a newline in a route gateway) fails that node in phase `persist` before any command is sent to it, and the other nodes are still configured | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the eight tests |
| 2 — something selects it | `Deploy` calls `deployNode` for each node; the early-return mutant turns the continue test red |
| 3 — the caller can discover it | the error text names nodes and phases; T6 documents it |
| 4 — it is used | every `wgmesh -deploy` with more than one node |

## Mutation Log
- 2026-11-16 · ff66005* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · the run stops at the first failed node: TestDeployContinuesPastUnreachableNode must go red · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3
- 2026-11-16 · ff66005* · mutant inconclusive · exit 1 · `pkg/mesh/deploy.go` · a node that failed detect goes to deployNode again: TestDeployDoesNotRedialAFailedDetect must go red · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3
  ```
  the fence failed on a build/parse error, not an assertion
  ```
- 2026-11-16 · ff66005* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · map order in the apply loop: TestDeployVisitsNodesInSortedOrder must go red · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3
- 2026-11-16 · ff66005* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · the fatal switch does nothing: TestPersistWriteFailureFailsWhenFatal must go red · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3
- 2026-11-16 · ff66005* · mutant killed · exit 1 · `pkg/mesh/report.go` · the phase is not in the text: TestDeployReportsEveryFailureWithPhase must go red · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3
- 2026-11-16 · ff66005* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · failures are swallowed: the continue and report tests must go red · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3
- 2026-11-16 · ff66005* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · a render refusal is ignored: the render-error test must go red · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3
- 2026-11-16 · ff66005* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · a node that failed detect goes to deployNode again: TestDeployDoesNotRedialAFailedDetect must go red · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3
- 2026-11-16 · e3546a6* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · the run stops at the first failed node: TestDeployContinuesPastUnreachableNode must go red · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7
- 2026-11-16 · e3546a6* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · a node that failed detect goes to deployNode again: TestDeployDoesNotRedialAFailedDetect must go red · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7
- 2026-11-16 · e3546a6* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · map order in the apply loop: TestDeployVisitsNodesInSortedOrder must go red · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7
- 2026-11-16 · e3546a6* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · the fatal switch does nothing: TestPersistWriteFailureFailsWhenFatal must go red · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7
- 2026-11-16 · e3546a6* · mutant killed · exit 1 · `pkg/mesh/report.go` · the phase is not in the text: TestDeployReportsEveryFailureWithPhase must go red · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7
- 2026-11-16 · e3546a6* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · failures are swallowed: the continue and report tests must go red · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7
- 2026-11-16 · e3546a6* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · a render refusal is ignored: the render-error test must go red · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7
- 2026-11-16 · e3546a6* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · install failures are labelled apply: the install subtest must go red · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7
- 2026-11-16 · e3546a6* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · route failures are labelled apply: the routes subtest must go red · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7
- 2026-11-16 · e3546a6* · mutant killed · exit 1 · `pkg/mesh/report.go` · multi-line errors split the report: the folded-line subtest must go red · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7
- 2026-11-16 · e3546a6* · mutant killed · exit 1 · `pkg/mesh/report.go` · errors.As cannot reach the node error: the net.OpError assertion must go red · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7
- 2026-11-16 · e3546a6* · mutant inconclusive · exit 1 · `pkg/mesh/deploy.go` · the persist warning is invisible: TestPersistWriteFailureIsAWarningAtN must go red · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7
  ```
  the fence failed on a build/parse error, not an assertion
  ```
- 2026-11-16 · e3546a6* · mutant killed · exit 1 · `pkg/mesh/deploy.go` · the persist warning is invisible in the summary: TestPersistWriteFailureIsAWarningAtN must go red · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7

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
- 2026-11-16 · ff66005* · exit 1 · `set -o pipefail …` · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3 · ms:889 · test-lock-sha256:ddf76022fe94b938e67f0ed4d71f1d55c10dbc90d4bf8cda394707a124958ec4 · test-lock-file:docs/adr/locks/ddf76022fe94b938e67f0ed4d71f1d55c10dbc90d4bf8cda394707a124958ec4.lock
  ```
  --- last 10 line(s) of stderr (of 42 after folding 42 raw)
  === CONT  TestDeploymentConfigUnderLock/mesh_access_only
  === CONT  TestDeploymentConfigUnderLock/groups_without_policies
  --- PASS: TestDeploymentConfigUnderLock (0.00s)
      --- PASS: TestDeploymentConfigUnderLock/full_mesh (0.00s)
      --- PASS: TestDeploymentConfigUnderLock/mesh_access_only (0.00s)
      --- PASS: TestDeploymentConfigUnderLock/routed_access (0.00s)
      --- PASS: TestDeploymentConfigUnderLock/groups_without_policies (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/mesh	0.081s
  FAIL
  ```
- 2026-11-16 · ff66005* · exit 0 · `set -o pipefail …` · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3 · ms:11232
- 2026-11-16 · ff66005* · exit 0 · `set -o pipefail …` · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3 · ms:11406
- 2026-11-16 · ff66005* · exit 0 · `set -o pipefail …` · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3 · ms:11977
- 2026-11-16 · ff66005* · exit 0 · `set -o pipefail …` · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3 · ms:13802
- 2026-11-16 · ff66005* · exit 0 · `set -o pipefail …` · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3 · ms:11649
- 2026-11-16 · ff66005* · exit 0 · `set -o pipefail …` · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3 · ms:11426
- 2026-11-16 · ff66005* · exit 0 · `set -o pipefail …` · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3 · ms:11826
- 2026-11-16 · ff66005* · exit 0 · `set -o pipefail …` · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3 · ms:9069
- 2026-11-16 · ff66005* · exit 0 · `set -o pipefail …` · acceptance-sha256:b0df103fe45e6ef448fa2a37ab7d50cad593e4349a58696e738f57e18b1e82d3 · ms:11238
- 2026-11-16 · e3546a6* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7 · ms:0 · test-lock-sha256:bf482bd3af6856417eac7c5f73f73f45abf90f6ae2ac0b6c01dfd2ee094f2d61 · test-lock-file:docs/adr/locks/bf482bd3af6856417eac7c5f73f73f45abf90f6ae2ac0b6c01dfd2ee094f2d61.lock · test-lock-kind:replace
- 2026-11-16 · e3546a6* · exit 0 · `set -o pipefail …` · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7 · ms:12019
- 2026-11-16 · e3546a6* · exit 0 · `set -o pipefail …` · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7 · ms:11916
- 2026-11-16 · e3546a6* · exit 0 · `set -o pipefail …` · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7 · ms:12972
- 2026-11-16 · e3546a6* · exit 0 · `set -o pipefail …` · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7 · ms:11956
- 2026-11-16 · e3546a6* · exit 0 · `set -o pipefail …` · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7 · ms:12015
- 2026-11-16 · e3546a6* · exit 0 · `set -o pipefail …` · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7 · ms:12946
- 2026-11-16 · e3546a6* · exit 0 · `set -o pipefail …` · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7 · ms:13647
- 2026-11-16 · e3546a6* · exit 0 · `set -o pipefail …` · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7 · ms:11816
- 2026-11-16 · e3546a6* · exit 0 · `set -o pipefail …` · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7 · ms:11563
- 2026-11-16 · e3546a6* · exit 0 · `set -o pipefail …` · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7 · ms:11795
- 2026-11-16 · e3546a6* · exit 0 · `set -o pipefail …` · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7 · ms:13534
- 2026-11-16 · e3546a6* · exit 0 · `set -o pipefail …` · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7 · ms:11995
- 2026-11-16 · e3546a6* · exit 0 · `set -o pipefail …` · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7 · ms:8669
- 2026-11-16 · e3546a6* · exit 0 · `set -o pipefail …` · acceptance-sha256:649e92a966c533d223a3ec19e11d8e6bb977c721b24392f2bf0daab0894847d7 · ms:12427
