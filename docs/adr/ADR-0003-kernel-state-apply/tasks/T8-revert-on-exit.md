# Task ADR-0003-T8: Undo the FORWARD rule and ip_forward at graceful exit (#872)

**Depends-on:** T4, T9
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `EnforceRevertOnExit`, `Daemon.revertKernel`, the ownership records `ruleOwner` and `ipForwardOwner` (each unknown, found or ours) of what this run changed
**Consumes:** the forwarding target and its observed values (T4); the specs and note (T9)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

At a graceful exit the daemon removes the FORWARD rule only if this run added it and restores `net.ipv4.ip_forward` only if this run changed it from 0; at release N it only logs what it will do from N+1.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/converge.go` | edit | the forwarding apply decides once per item whether the rule and `ip_forward` were found in place or are ours (first successful write of an item seen absent or 0; an observed `ip_forward` of 1 means the previous value was 0 only for items we wrote, so no previous value is stored); `revertKernel` issues `iptables -D` and the sysctl restore from those records; `EnforceRevertOnExit` (`atomic.Bool`, `false`; every access `Load`/`Store`) |
| `pkg/daemon/daemon.go` | edit | `Run` and `RunWithDHTDiscovery` (the one `wgmesh join` calls) call `revertKernel` after `d.wg.Wait()` and before the deferred `teardownWireGuard` |
| `docs/kernel-state.md`, `pkg/daemon/spec_converge_test.go` | edit | the note and the spec test name the revert and its switch |
| `pkg/daemon/revert_test.go` | add | the four tests below |

Selection: `Run` and `RunWithDHTDiscovery` call `revertKernel`; deleting a call turns `TestRunRevertsKernelAfterWait` red (the revert tests call `revertKernel` themselves).

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
| 2 — something selects it | `Run` and `RunWithDHTDiscovery` call `revertKernel`; deleting a call turns `TestRunRevertsKernelAfterWait` red |
| 3 — the caller can discover it | the log line at N, the note (T9) |
| 4 — it is used | every graceful exit |

## Mutation Log
- 2026-11-16 · 308320a* · mutant killed · exit 1 · `pkg/daemon/converge.go` · found rule deleted: TestShutdownKeepsPreexistingRule must go red · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc
- 2026-11-16 · 308320a* · mutant killed · exit 1 · `pkg/daemon/converge.go` · found ip_forward=1 restored to 0: TestShutdownRestoresIPForwardOnlyIfWeChangedIt must go red · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc
- 2026-11-16 · 308320a* · mutant killed · exit 1 · `pkg/daemon/converge.go` · record overwritten: TestShutdownRecordSurvivesDriftReapply must go red · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc
- 2026-11-16 · 308320a* · mutant killed · exit 1 · `pkg/daemon/converge.go` · found not recorded: a drift re-write makes the rule ours: TestShutdownRecordSurvivesDriftReapply must go red · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc
- 2026-11-16 · 308320a* · mutant killed · exit 1 · `pkg/daemon/converge.go` · commands at N: TestShutdownRevertIsWarnOnlyAtN must go red · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc
- 2026-11-16 · 308320a* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · revertKernel calls removed: TestRunRevertsKernelAfterWait must go red · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc
- 2026-11-16 · 0f45e87* · mutant killed · exit 1 · `pkg/daemon/converge.go` · found rule deleted: TestShutdownKeepsPreexistingRule must go red · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc
- 2026-11-16 · 0f45e87* · mutant killed · exit 1 · `pkg/daemon/converge.go` · found ip_forward=1 restored to 0: TestShutdownRestoresIPForwardOnlyIfWeChangedIt must go red · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc
- 2026-11-16 · 0f45e87* · mutant killed · exit 1 · `pkg/daemon/converge.go` · record overwritten: TestShutdownRecordSurvivesDriftReapply must go red · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc
- 2026-11-16 · 0f45e87* · mutant killed · exit 1 · `pkg/daemon/converge.go` · found not recorded: TestShutdownRecordSurvivesDriftReapply must go red · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc
- 2026-11-16 · 0f45e87* · mutant killed · exit 1 · `pkg/daemon/converge.go` · commands at N: TestShutdownRevertIsWarnOnlyAtN must go red · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc
- 2026-11-16 · 0f45e87* · mutant killed · exit 1 · `pkg/daemon/daemon.go` · revertKernel calls removed: TestRunRevertsKernelAfterWait must go red · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc

## Invariants

- A crash or `SIGKILL` leaves the host as today; nothing here claims otherwise.
- Nothing is undone that this run did not do. The two records (`ruleOwner`, `ipForwardOwner`) live on the `Daemon` under `convergeMu`, are set only by the FIRST successful write of a run and are never overwritten by a later drift re-apply (T4 re-writes on drift: the second write must not make a run think it owns a rule or a value it found), and `TestShutdownKeepsPreexistingRule` covers a re-apply after a drift.
- The interface teardown (`teardownWireGuard`) is unchanged.

## Risks

- A restart (upgrade, `Restart=always`) opens a short gap in forwarding for other services on the host; the note says so, and the answer to decision 7 may choose B.
- Another program that enabled the sysctl after us is not detected; we restore only the value we read.
- Test doubles: `MockCommandExecutor` and the file-read variable of T4.

## Stop Condition

Stop and ask if `Run` has more than one exit path that skips the deferred calls.

## Out of Scope

- Reverting after a crash (permanent: boundary: no persisted record of what a run changed is kept; that would be new state).
- Reverting routes and addresses (permanent: boundary: `teardownWireGuard` already deletes the interface, which removes both).

## Verification Log
- 2026-11-16 · 308320a* · exit 1 · `set -o pipefail …` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:2287 · test-lock-sha256:fd7252a0a2f7427a9c59959cd2ed7a53f7577880e2022f0547fa1023f335a0ab · test-lock-file:docs/adr/locks/fd7252a0a2f7427a9c59959cd2ed7a53f7577880e2022f0547fa1023f335a0ab.lock
  ```
  --- last 10 line(s) of stderr (of 22 after folding 22 raw)
      --- FAIL: TestShutdownRestoresIPForwardOnlyIfWeChangedIt/read_0_and_written_1 (0.00s)
      --- PASS: TestShutdownRestoresIPForwardOnlyIfWeChangedIt/read_1 (0.00s)
      --- PASS: TestShutdownRestoresIPForwardOnlyIfWeChangedIt/read_1,_drifted_to_0,_written_1_again (0.00s)
  === RUN   TestShutdownRevertIsWarnOnlyAtN
      revert_test.go:136: revert-on-exit logged 0 times, want 1; lines ["forward-scope: this node neither relays (introducer) nor advertises routes (advertise-routes), yet ip_forward and the FORWARD rule on wgmesh-t3-none are still forced; a later release stops forcing them (EnforceForwardScope); set --introducer or advertise-routes if this node must forward" "Cannot read the addresses of wgmesh-t3-none, skipping them this tick: read addresses: fake host: unexpected command \"ip -o addr show dev wgmesh-t3-none\"" "Cannot read the IPv4 routes of wgmesh-t3-none, skipping them this tick: failed to read routes: fake host: unexpected command \"ip route show dev wgmesh-t3-none\"" "Cannot read the IPv6 routes of wgmesh-t3-none, skipping them this tick: failed to read routes: fake host: unexpected command \"ip -6 route show dev wgmesh-t3-none\"" "set forward ip_forward... done" "append forward FORWARD... done"]
      revert_test.go:139: the line does not name EnforceRevertOnExit: ["forward-scope: this node neither relays (introducer) nor advertises routes (advertise-routes), yet ip_forward and the FORWARD rule on wgmesh-t3-none are still forced; a later release stops forcing them (EnforceForwardScope); set --introducer or advertise-routes if this node must forward" "Cannot read the addresses of wgmesh-t3-none, skipping them this tick: read addresses: fake host: unexpected command \"ip -o addr show dev wgmesh-t3-none\"" "Cannot read the IPv4 routes of wgmesh-t3-none, skipping them this tick: failed to read routes: fake host: unexpected command \"ip route show dev wgmesh-t3-none\"" "Cannot read the IPv6 routes of wgmesh-t3-none, skipping them this tick: failed to read routes: fake host: unexpected command \"ip -6 route show dev wgmesh-t3-none\"" "set forward ip_forward... done" "append forward FORWARD... done"]
  --- FAIL: TestShutdownRevertIsWarnOnlyAtN (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/daemon	0.006s
  FAIL
  ```
- 2026-11-16 · 308320a* · exit 0 · `set -o pipefail …` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:11488
- 2026-11-16 · 308320a* · exit 0 · `set -o pipefail …` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:12133
- 2026-11-16 · 308320a* · exit 0 · `set -o pipefail …` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:14632
- 2026-11-16 · 308320a* · exit 0 · `set -o pipefail …` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:13133
- 2026-11-16 · 308320a* · exit 0 · `set -o pipefail …` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:12139
- 2026-11-16 · 308320a* · exit 0 · `set -o pipefail …` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:14140
- 2026-11-16 · 308320a* · exit 0 · `set -o pipefail …` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:7894
- 2026-11-16 · 0f45e87* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:0 · test-lock-sha256:30fdfb5eb21b12f8be9658d788c00a9426656dfa950c062d6a51a83d231445bd · test-lock-file:docs/adr/locks/30fdfb5eb21b12f8be9658d788c00a9426656dfa950c062d6a51a83d231445bd.lock · test-lock-kind:replace
- 2026-11-16 · 0f45e87* · exit 0 · `set -o pipefail …` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:10722
- 2026-11-16 · 0f45e87* · exit 0 · `set -o pipefail …` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:12134
- 2026-11-16 · 0f45e87* · exit 0 · `set -o pipefail …` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:11031
- 2026-11-16 · 0f45e87* · exit 0 · `set -o pipefail …` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:11374
- 2026-11-16 · 0f45e87* · exit 0 · `set -o pipefail …` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:10692
- 2026-11-16 · 0f45e87* · exit 0 · `set -o pipefail …` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:11867
- 2026-11-16 · 0f45e87* · exit 0 · `set -o pipefail …` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:7304
- 2026-11-16 · 7c1d459* · exit 0 · `set -o pipefail …` · acceptance-sha256:324916049229918bc172c9b2f592c0edaf0111b59b96bbf9f32cc19670339dbc · ms:7831
