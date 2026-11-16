# Task ADR-0003-T3: Compare peers with the kernel, and move every peer writer into converge

**Depends-on:** T2
**Covers:** none — no spec
**Estimated scope:** L (cross-boundary)
**Owner:** unassigned
**Produces:** `wireguard.ParseDump`, `kernelObserved`, `observeKernel` (peer target), `planKernel`, `applyKernel`, `pushedEndpoint`, the result log, `wgmesh_kernel_changes_total`, `wgmesh_kernel_warnings_total`, `TestKernelWritesOnlyInConverge`
**Consumes:** `desiredState`, `Daemon.desire` (T2)
**Data dependency:** needs one recorded `wg show <iface> dump` from a Linux host with wireguard-tools, recorded by S1 into `pkg/wireguard/testdata/dump.txt` (the format below is read from wg(8), not run here)
**Proof map:** v1

## Goal

A peer's AllowedIPs are set when the kernel's differ from the desired set, not when a cached string changed; one `wg show dump` per tick replaces two forks; an observe error never reads as an empty kernel; a peer removed by stale cleanup is set again when it returns; and every `wireguard.SetPeer` and `RemovePeer` call is in `converge.go`.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/wireguard/dump.go` | add | `ParseDump(string) (DumpPeers, error)` and `Dump(iface)`: one line per peer, fields public key, preshared key, endpoint, allowed ips, latest handshake, rx, tx, keepalive; `(none)` is empty; the first line is the interface and is skipped |
| `pkg/wireguard/dump_test.go`, `pkg/wireguard/testdata/dump.txt` | add | parser test over the recorded fixture |
| `pkg/daemon/observe.go` | add | `kernelObserved{Peers map[string]observedPeer}` and `observeKernel` (peer target only here); an error is returned per target |
| `pkg/daemon/converge.go` | add | `planKernel`, `applyKernel`, `applyPeer`, `removePeerFromKernel` (the only callers of `SetPeer` and `RemovePeer`), `pushedEndpoint` (under one mutex), the per-item result and `logResult` that prints on change only |
| `pkg/daemon/daemon.go` | edit | `reconcile` becomes observe, desire, plan, apply; `applyDesiredPeerConfigs` and `lastAppliedPeerConfigs` are deleted (`:54`, `:191`, `:773-828`, `:782`, `:1392`, `:1407`); `attemptPeerReconnect` (`:1387`), `evictPeerFromPool` (`:1403`) and `removePeer` (`:915`, used by `staleCleanupLoop` `:944`) call the converge helpers |
| `pkg/daemon/metrics.go` | edit | `wgmesh_kernel_changes_total{target}` and `wgmesh_kernel_warnings_total{rule}` registered beside the existing counters (ADR-0002 T4 adds its own counter in this file; no overlap) |
| `pkg/daemon/converge_test.go`, `pkg/daemon/converge_guard_test.go` | add | the tests below |

Selection: `reconcile` calls `converge`; the guard test fails the build if a writer is added outside it; deleting the `observeKernel` call turns `TestPlanPeersAllowedIPsDriftReapplies` red.

## Ordered Steps

1. [S2] Add the tests below (the ten daemon names of the Acceptance command, TestParseDump, and the reconnect, concurrency and removal-order tests the T3 review added) with compiling stubs (`ParseDump` returns an error, `planKernel` returns an empty plan, `observeKernel` returns an empty kernel) so the first run fails on assertions; `TestParseDump` starts with a fixture written by hand from wg(8). Run red.
2. [S1] On a Linux host with wireguard-tools run `wg show <iface> dump` with two peers (one with an endpoint and AllowedIPs, one with `(none)` fields) and replace the hand-written `pkg/wireguard/testdata/dump.txt` with it. [proof: human: the command output is real data; adr-verify cannot see it, record the host and date in the commit message]
3. [S3] Implement `ParseDump`, `observeKernel`, `planKernel` (a peer missing from the kernel or with a different AllowedIPs set is set, shrinking peers before growing ones; an observed peer absent from the desired state is removed) and `applyKernel` (an error on one item is recorded and the others run).
4. [S4] Switch `reconcile`, reconnect, eviction and stale cleanup to the converge helpers and delete the string cache; keep `pushedEndpoint` for the endpoint.
5. [S5] Add `TestKernelWritesOnlyInConverge` (parses the non-test `.go` files of `pkg/daemon`; T3 covers `wireguard.SetPeer` and `wireguard.RemovePeer`, T4, T5 and T7 extend the pattern list) and register the two counters.
6. [S6] Run the mutants with `adr-verify --mutant`: the plan compares the old `endpoint|allowedIPs` string instead of the kernel's set; growing peers are applied before shrinking ones; `observeKernel` returns an empty kernel on error; the endpoint is pushed every tick; the result is logged every tick. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/wireguard -run '^TestParseDump$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 1 ] \
 && b=$(go test ./pkg/daemon -run '^(TestObserveFailureSkipsApply|TestPlanPeersAllowedIPsDriftReapplies|TestPlanPeersFailoverMovesNetwork|TestPlanPeersSteadyStateEmpty|TestEndpointPushedOnlyWhenDesiredChanges|TestApplyIsolatesPeerErrors|TestStaleCleanupRemovedPeerIsReadded|TestReconcileForksWgOncePerTick|TestResultLoggedOnChangeOnly|TestKernelWritesOnlyInConverge)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -ge 10 ] \
 && [ "$(grep -rn 'lastAppliedPeerConfigs' pkg --include=*.go | grep -v _test.go | wc -l)" -eq 0 ] && go test -race ./pkg/wireguard ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestParseDump` | `pkg/wireguard/dump_test.go` | the recorded fixture parses to the two peers, `(none)` endpoint and AllowedIPs are empty, the interface line is skipped, a short line is an error | — | S2, S3 |
| `TestObserveFailureSkipsApply` | `pkg/daemon/converge_test.go` | when `wg show dump` fails no `RemovePeer` or `SetPeer` runs that tick and the error is logged once | — | S2, S3 |
| `TestPlanPeersAllowedIPsDriftReapplies` | `pkg/daemon/converge_test.go` | a kernel that lost a network from a peer (another peer was set last) gets that peer set again although the desired set did not change | — | S2, S3 |
| `TestPlanPeersFailoverMovesNetwork` | `pkg/daemon/converge_test.go` | when the owner of a shared network disappears the survivor is set with the network, and a peer that shrinks is applied before one that grows | — | S2, S3 |
| `TestPlanPeersSteadyStateEmpty` | `pkg/daemon/converge_test.go` | observed equal to desired for five ticks gives an empty plan each time | — | S2, S3 |
| `TestEndpointPushedOnlyWhenDesiredChanges` | `pkg/daemon/converge_test.go` | a new peer is pushed with its endpoint; a roamed endpoint seen in the kernel is not overwritten; a changed desired endpoint is pushed once | — | S2, S3, S4 |
| `TestApplyIsolatesPeerErrors` | `pkg/daemon/converge_test.go` | one failing `wg set` is reported and the other peers in the plan are still applied | — | S2, S3 |
| `TestStaleCleanupRemovedPeerIsReadded` | `pkg/daemon/converge_test.go` | a peer removed by stale cleanup and rediscovered with the same endpoint and AllowedIPs is set again | — | S2, S4 |
| `TestReconcileForksWgOncePerTick` | `pkg/daemon/converge_test.go` | one reconcile runs `wg show <iface> dump` once and no `wg show <iface> peers` or `latest-handshakes` | — | S2, S4 |
| `TestResultLoggedOnChangeOnly` | `pkg/daemon/converge_test.go` | the same failure over ten ticks prints one line, and recovery prints one | — | S2, S3 |
| `TestKernelWritesOnlyInConverge` | `pkg/daemon/converge_guard_test.go` | no production file of `pkg/daemon` outside `converge.go` calls `wireguard.SetPeer` or `wireguard.RemovePeer` | — | S5 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the eleven tests |
| 2 — something selects it | `reconcile` calls `converge`; the guard test fails when a writer is added elsewhere |
| 3 — the caller can discover it | `wgmesh_kernel_changes_total{target}` on the metrics endpoint; T9 documents it |
| 4 — it is used | every tick, and the health, eviction and stale-cleanup paths |

## Mutation Log
- 2026-11-16 · ed18ca4* · mutant killed · exit 1 · `pkg/daemon/converge.go` · kernel set not compared: TestPlanPeersAllowedIPsDriftReapplies must go red · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a
- 2026-11-16 · ed18ca4* · mutant killed · exit 1 · `pkg/daemon/converge.go` · growing first: TestPlanPeersFailoverMovesNetwork must go red · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a
- 2026-11-16 · ed18ca4* · mutant killed · exit 1 · `pkg/daemon/observe.go` · error becomes empty kernel: TestObserveFailureSkipsApply must go red · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a
- 2026-11-16 · ed18ca4* · mutant killed · exit 1 · `pkg/daemon/converge.go` · endpoint always pushed: TestEndpointPushedOnlyWhenDesiredChanges must go red · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a
- 2026-11-16 · ed18ca4* · mutant inconclusive · exit 1 · `pkg/daemon/converge.go` · failure logged every tick: TestResultLoggedOnChangeOnly must go red · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a
  ```
  the fence failed on a build/parse error, not an assertion
  ```
- 2026-11-16 · ed18ca4* · mutant killed · exit 1 · `pkg/daemon/converge.go` · convergeMu removed from converge: TestConvergeConcurrentWithHealthWrites must go red under -race · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a
- 2026-11-16 · ed18ca4* · mutant killed · exit 1 · `pkg/daemon/converge.go` · relayed peer set directly: TestReconnectOnlyTouchesPeerWithOwnEntry must go red · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a
- 2026-11-16 · f2043d5* · mutant killed · exit 1 · `pkg/daemon/converge.go` · kernel set not compared: TestPlanPeersAllowedIPsDriftReapplies must go red · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a
- 2026-11-16 · f2043d5* · mutant killed · exit 1 · `pkg/daemon/converge.go` · growing first: TestPlanPeersFailoverMovesNetwork must go red · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a
- 2026-11-16 · f2043d5* · mutant killed · exit 1 · `pkg/daemon/observe.go` · error becomes empty kernel: TestObserveFailureSkipsApply must go red · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a
- 2026-11-16 · f2043d5* · mutant killed · exit 1 · `pkg/daemon/converge.go` · endpoint always pushed: TestEndpointPushedOnlyWhenDesiredChanges must go red · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a
- 2026-11-16 · f2043d5* · mutant inconclusive · exit 1 · `pkg/daemon/converge.go` · failure logged every tick: TestResultLoggedOnChangeOnly must go red · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a
  ```
  the fence failed on a build/parse error, not an assertion
  ```
- 2026-11-16 · f2043d5* · mutant killed · exit 1 · `pkg/daemon/converge.go` · convergeMu removed from converge: TestConvergeConcurrentWithHealthWrites must go red under -race · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a
- 2026-11-16 · f2043d5* · mutant killed · exit 1 · `pkg/daemon/converge.go` · relayed peer set directly: TestReconnectOnlyTouchesPeerWithOwnEntry must go red · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a
- 2026-11-16 · f2043d5* · mutant killed · exit 1 · `pkg/daemon/converge.go` · failure logged every tick: TestResultLoggedOnChangeOnly must go red · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a

## Invariants

- The `wg set` commands are the same as before (`SetPeer` passes the preshared key, the endpoint, the AllowedIPs and `persistent-keepalive 25`); only when they run changes.
- Obsolete peers are still removed when absent from the desired state; an observe error removes nothing.
- A peer without an endpoint is still not configured (`daemon.go:791-793`); IPv6 endpoints are still skipped with `--no-ipv6`.
- `GetLatestHandshakes` and `GetPeerTransfers` keep working for the health loop and discovery.
- Concurrency (M 2026-11-16): every field the step shares between goroutines (`pushedEndpoint`, the last results, the warn-once sets) is under one mutex on every access; the `SetPeer` seam is a `Daemon` field; `go test -race` runs a test with two goroutines calling `converge`.
- From the T2 review: (a) the handshakes of a tick and the peers' AllowedIPs come from the one `wg show dump`; when that read fails the whole tick skips `desire` and every write, so a failed read can never make every owner look stale and move routes (T2 passes handshakes read separately and ignores the error until this task); (b) `attemptPeerReconnect`, `evictPeerFromPool` and `removePeer` write through the converge helpers under the same lock that `reconcile` holds (`reconcileMu` or one lock inside converge, never both ordered two ways), and a forced reconnect of a relayed peer must not pull the peer's mesh `/32` off its relay (set it from the desired state of the peer's carrier, or not at all).

## Risks

- `wg show dump` differs on an old wireguard-tools: a parse error skips the tick and logs once; the fixture is real output (S1).
- Two reconciles at once (tick and `handleSIGHUP`, `daemon.go:1680`): `pushedEndpoint` and the result log are under one mutex, and `go test -race` is in the fence.
- The test double is `MockCommandExecutor`; `wireguard.SetPeer` uses `os/exec` directly (`pkg/wireguard/apply.go:159`), so the converge helpers call it through a field of the `Daemon` set at construction (a bare package variable replaced by tests is a race with leftover goroutines, M 2026-11-16).
- `convergeMu` is held across `wg show dump` and every `wg set`, and those forks have no deadline today (`pkg/wireguard` runs `exec.Command`): a hung `wg` also blocks eviction and stale cleanup, which took no lock before. ADR-0007 (bounded growth, `wg` reads and writes with a deadline) owns the bound; until it lands the window is the hung process (permanent: boundary: another group owns it).
- What T3 built, for the next reader: `applyKernel` is the loop over items (plus `removeLocked`, `setLocked`, `reconnectPeerInKernel`; there is no separate `applyPeer`); `observeKernel` returns `kernelObserved{Peers, Errs}`; the fixture was recorded by the coordinator on the laptop on 2026-11-16 (the commit message names the host); `owners()` returns the kernel peer key that holds a network, so for a network whose owner is relayed the kernel holds it under the relay and Resolve's stickiness does not match (the lowest-IP rule still gives one stable owner; BACKLOG row in T9). Log texts changed: `Failed to configure peer` and `Failed to remove obsolete peer` are replaced by `Failed to set peer ...` and `Failed to remove peer ...` printed on change (release note in T9). A failed `wg show dump` now skips the whole tick including routes and the relay table update (before, routes were synced anyway); T9 names it.

## Stop Condition

Stop and ask if a peer test in `pkg/daemon` needs a changed expectation, or if `SetPeer` cannot be replaced by a variable without changing `pkg/wireguard`'s exported behaviour.

## Out of Scope

- Routes, forwarding, the interface address (T5, T4, T7).
- Changing `GetPeerTransfers` or the health loop (permanent: boundary: liveness is owned by another group).

## Verification Log
- 2026-11-16 · ed18ca4* · exit 1 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:836 · test-lock-sha256:2f375e5fc5c6265d5f7dcc14b1b4d8c5597136a339d2417519834a7358f7263c · test-lock-file:docs/adr/locks/2f375e5fc5c6265d5f7dcc14b1b4d8c5597136a339d2417519834a7358f7263c.lock
  ```
  --- last 6 line(s) of stderr
  === RUN   TestParseDump
      dump_test.go:16: ParseDump(fixture): ParseDump: not implemented
  --- FAIL: TestParseDump (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/wireguard	0.004s
  FAIL
  ```
- 2026-11-16 · ed18ca4* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:11369
- 2026-11-16 · ed18ca4* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:9462
- 2026-11-16 · ed18ca4* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:11766
- 2026-11-16 · ed18ca4* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:11104
- 2026-11-16 · ed18ca4* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:12013
- 2026-11-16 · ed18ca4* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:11854
- 2026-11-16 · ed18ca4* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:11733
- 2026-11-16 · ed18ca4* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:6709
- 2026-11-16 · f2043d5* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:1 · test-lock-sha256:4700d84ba36b20bd64a89eb00ab73ddec1141b4369cc24ebcf9be776ec6ae893 · test-lock-file:docs/adr/locks/4700d84ba36b20bd64a89eb00ab73ddec1141b4369cc24ebcf9be776ec6ae893.lock · test-lock-kind:replace
- 2026-11-16 · f2043d5* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:12239
- 2026-11-16 · f2043d5* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:11599
- 2026-11-16 · f2043d5* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:12177
- 2026-11-16 · f2043d5* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:11762
- 2026-11-16 · f2043d5* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:10907
- 2026-11-16 · f2043d5* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:10521
- 2026-11-16 · f2043d5* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:11056
- 2026-11-16 · f2043d5* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:7864
- 2026-11-16 · f2043d5* · exit 0 · `set -o pipefail …` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:10710
- 2026-11-16 · 823f97e* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:0 · test-lock-sha256:ff46e4f5427ee43abf7124add55199cc590bf8829002c083443f7c3d969a3ee7 · test-lock-file:docs/adr/locks/ff46e4f5427ee43abf7124add55199cc590bf8829002c083443f7c3d969a3ee7.lock · test-lock-kind:replace
- 2026-11-16 · 8e9f14f* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:0 · test-lock-sha256:4b5b65162030fa033b491072bb37c79140c4d76fe80fd9582ff681e6fbec4849 · test-lock-file:docs/adr/locks/4b5b65162030fa033b491072bb37c79140c4d76fe80fd9582ff681e6fbec4849.lock · test-lock-kind:replace
- 2026-11-16 · f110717* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:0 · test-lock-sha256:abca8938bfe8f36e77a85da235081a555a295ddf1107155500699f3becd227d5 · test-lock-file:docs/adr/locks/abca8938bfe8f36e77a85da235081a555a295ddf1107155500699f3becd227d5.lock · test-lock-kind:replace
- 2026-11-16 · fe6f879* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:253f9195de820b35b2dac8f777e0468fb8166f647bb1a11755bf2e06da81557a · ms:0 · test-lock-sha256:85b6e1ee86aa2357c56cc8329b52feccf2b64d9967e1b14fce57459e37f7b420 · test-lock-file:docs/adr/locks/85b6e1ee86aa2357c56cc8329b52feccf2b64d9967e1b14fce57459e37f7b420.lock · test-lock-kind:replace
