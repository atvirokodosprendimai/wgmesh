# Task ADR-0003-T4: Converge ip_forward and the FORWARD rule once, and log a missing iptables once (#872)

**Depends-on:** T3
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `forwardTarget`, `EnforceForwardScope`, rule `forward-scope`
**Consumes:** `kernelObserved`, `planKernel`, `applyKernel`, the result log, `wgmesh_kernel_warnings_total`, `TestKernelWritesOnlyInConverge` (T3)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

A Linux node reads `net.ipv4.ip_forward` and the FORWARD rule before writing them, writes only what is missing, logs a missing `iptables` once, counts a node that neither relays nor advertises routes as `forward-scope` at release N, and stops forcing forwarding on it when `EnforceForwardScope` is true.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/daemon/observe.go` | edit | forwarding target: read `/proc/sys/net/ipv4/ip_forward` through a `Daemon` field (`readFile`), `iptables -C FORWARD -i <if> -o <if> -j ACCEPT` only when forwarding is wanted, `LookPath("iptables")` result |
| `pkg/daemon/converge.go` | edit | plan: write the sysctl only when it reads 0, append the rule only when `-C` fails; apply result "unavailable" for a missing binary; `EnforceForwardScope` (`atomic.Bool`, `false`; every access `Load`/`Store`) and the rule `forward-scope` |
| `pkg/daemon/desired.go` | edit | `desiredState.Forward{Wanted, Needed bool}`: `Needed` is `config.Introducer` or at least one `advertise-routes` entry (the only relay is an introducer, `daemon.go:586`); at N `Wanted` is true on Linux always, with `EnforceForwardScope` it equals `Needed` |
| `pkg/daemon/converge_guard_test.go` | edit | the pattern list gains `sysctl` and `iptables` |
| `pkg/daemon/routes.go` | none | the blind `sysctl` and `iptables` calls at `:172-175` and `ensureWGForwardingRule` (`:180-192`) stay until T5 removes them, so no tick runs without forwarding between T4 and T5 |
| `pkg/daemon/forward_test.go` | add | the seven tests below |

Selection: `reconcile` calls `converge`, which now includes the forwarding target; deleting the forwarding step from the plan turns `TestForwardAppliedOnceThenQuiet` red.

## Ordered Steps

1. [S1] Add the seven tests with compiling stubs (an empty forwarding plan) and run red; the fake host is a struct holding `ipForward`, `ruleInstalled`, `iptablesPresent` and a command log, reached through `MockCommandExecutor` and the `Daemon` field that replaces the file read.
2. [S2] Implement the forwarding target: observe, plan, apply, result.
3. [S3] Implement `Needed` and the `forward-scope` rule: at N one log line and one counter increment per transition, no change of behaviour; with `EnforceForwardScope` the target is not wanted when not needed.
4. [S4] Extend `TestKernelWritesOnlyInConverge` with `sysctl` and `iptables` for the files other than `routes.go`, which T5 cleans; the test lists `routes.go` as a known exception that T5 removes. [proof: acceptance]
5. [S5] Run the mutants with `adr-verify --mutant`: the plan writes the sysctl every tick; a missing `iptables` is logged every tick; `Needed` ignores `Introducer`; `Needed` ignores `advertise-routes`. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/daemon -run '^(TestForwardAppliedOnceThenQuiet|TestForwardReappliedWhenKernelDrifts|TestForwardPreexistingValueNotRewritten|TestMissingIptablesLoggedOnce|TestForwardNotNeededWarnsOnceAtN|TestForwardScopeEnforcedStopsForcing|TestForwardNeededForIntroducerAndAdvertiser)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 7 ] && go test -race ./pkg/daemon -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestForwardAppliedOnceThenQuiet` | `pkg/daemon/forward_test.go` | on a host with `ip_forward` 0 and no rule, tick 1 writes both once and ticks 2 to 6 issue no write (only the reads) | — | S1, S2 |
| `TestForwardReappliedWhenKernelDrifts` | `pkg/daemon/forward_test.go` | after the fake host resets `ip_forward` to 0, or drops the rule, the next tick writes it again | — | S1, S2 |
| `TestForwardPreexistingValueNotRewritten` | `pkg/daemon/forward_test.go` | `ip_forward` already 1 and the rule already present: no write, ever | — | S1, S2 |
| `TestMissingIptablesLoggedOnce` | `pkg/daemon/forward_test.go` | with no `iptables` binary ten ticks print one line, the tick is not an error, and no `iptables -A` is attempted | — | S1, S2 |
| `TestForwardNotNeededWarnsOnceAtN` | `pkg/daemon/forward_test.go` | a node that is not an introducer and has no `advertise-routes` still gets forwarding at N, one `forward-scope` log line and one counter increment | — | S1, S3 |
| `TestForwardScopeEnforcedStopsForcing` | `pkg/daemon/forward_test.go` | with `EnforceForwardScope` true that node issues no sysctl write and no rule append | — | S1, S3 |
| `TestForwardNeededForIntroducerAndAdvertiser` | `pkg/daemon/forward_test.go` | an introducer and a node with `advertise-routes` are needed and keep forwarding with the switch on | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the seven tests |
| 2 — something selects it | the forwarding target is part of the plan every tick; the `EnforceForwardScope` variable is read by `desire` |
| 3 — the caller can discover it | the log line says what N+1 will do; `wgmesh_kernel_warnings_total{rule="forward-scope"}`; T9 documents both |
| 4 — it is used | every tick of every Linux node |

## Mutation Log

## Invariants

- At N a Linux node ends with `ip_forward` 1 and the FORWARD rule present, as today.
- The rule text is unchanged: `iptables -A FORWARD -i <if> -o <if> -j ACCEPT`.
- A write failure is an item result, not an abort of the tick.

## Risks

- A node that relays or advertises but whose config is changed by SIGHUP (`reloadConfig`, `daemon.go:1719`) is judged by the next tick; `Needed` reads the current value.
- Reading `/proc` through a `Daemon` field: the production default is set in `New`; each test sets its own field, so no test touches a shared variable. `Needed` reads advertise-routes through `GetAdvertiseRoutes()` (config lock), never `d.config.AdvertiseRoutes`.
- Test doubles: `MockCommandExecutor` and the file-read field.

## Stop Condition

Stop and ask if `iptables -C` cannot be told apart from "binary missing" by the mock without a new interface, or if an existing test relies on `applyRouteDiff` issuing the forwarding commands before T5.

## Out of Scope

- Removing the blind calls from `applyRouteDiff` (T5).
- Undoing anything at exit (T8).
- nftables and firewall policy (permanent: boundary: wgmesh converges the one rule it already installs).

## Verification Log
