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
| `pkg/daemon/converge.go` | edit | plan: write the sysctl only when it reads 0, append the rule only when `-C` fails; a missing binary is an outcome, not an error: logged once, no item result (the result log would print it every tick); `EnforceForwardScope` (`atomic.Bool`, `false`; every access `Load`/`Store`) and the rule `forward-scope` |
| `pkg/daemon/desired.go` | edit | `desiredState.Forward{Wanted, Needed bool}`: `Needed` is `config.Introducer` or at least one `advertise-routes` entry (the only relay is an introducer, `daemon.go:586`); at N `Wanted` is true on Linux always, with `EnforceForwardScope` it equals `Needed` |
| `pkg/daemon/converge_guard_test.go` | edit | the pattern list gains `sysctl` and `iptables` |
| `pkg/daemon/routes.go` | none | the blind `sysctl` and `iptables` calls at `:150-153` and `ensureWGForwardingRule` (`:158-170`) at f3860c6 stay until T5 removes them, so no tick runs without forwarding between T4 and T5 |
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
- 2026-11-16 · ef8b23a* · mutant killed · exit 1 · `pkg/daemon/converge.go` · sysctl written every tick: TestForwardAppliedOnceThenQuiet must go red · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68
- 2026-11-16 · ef8b23a* · mutant killed · exit 1 · `pkg/daemon/converge.go` · missing iptables logged every tick: TestMissingIptablesLoggedOnce must go red · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68
- 2026-11-16 · ef8b23a* · mutant killed · exit 1 · `pkg/daemon/desired.go` · Needed ignores Introducer: TestForwardNeededForIntroducerAndAdvertiser must go red · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68
- 2026-11-16 · ef8b23a* · mutant killed · exit 1 · `pkg/daemon/desired.go` · Needed ignores advertise-routes: TestForwardNeededForIntroducerAndAdvertiser must go red · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68
- 2026-11-16 · ef8b23a* · mutant killed · exit 1 · `pkg/daemon/desired.go` · Wanted ignores EnforceForwardScope: TestForwardScopeEnforcedStopsForcing must go red · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68
- 2026-11-16 · ef8b23a* · mutant killed · exit 1 · `pkg/daemon/converge.go` · rule appended although present: TestForwardPreexistingValueNotRewritten must go red · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68
- 2026-11-16 · 823f97e* · mutant killed · exit 1 · `pkg/daemon/converge.go` · sysctl written every tick: TestForwardAppliedOnceThenQuiet must go red · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68
- 2026-11-16 · 823f97e* · mutant killed · exit 1 · `pkg/daemon/converge.go` · missing iptables logged every tick: TestMissingIptablesLoggedOnce must go red · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68
- 2026-11-16 · 823f97e* · mutant killed · exit 1 · `pkg/daemon/desired.go` · Needed ignores Introducer: TestForwardNeededForIntroducerAndAdvertiser must go red · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68
- 2026-11-16 · 823f97e* · mutant killed · exit 1 · `pkg/daemon/desired.go` · Needed ignores advertise-routes: TestForwardNeededForIntroducerAndAdvertiser must go red · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68
- 2026-11-16 · 823f97e* · mutant killed · exit 1 · `pkg/daemon/desired.go` · Wanted ignores EnforceForwardScope: TestForwardScopeEnforcedStopsForcing must go red · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68
- 2026-11-16 · 823f97e* · mutant killed · exit 1 · `pkg/daemon/converge.go` · rule appended although present: TestForwardPreexistingValueNotRewritten must go red · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68

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
- 2026-11-16 · ef8b23a* · exit 1 · `set -o pipefail …` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:2143 · test-lock-sha256:b2d1a3dae467672afe7b0fdd82842b241cf9117b07e8486a7c129780fc6b13e0 · test-lock-file:docs/adr/locks/b2d1a3dae467672afe7b0fdd82842b241cf9117b07e8486a7c129780fc6b13e0.lock
  ```
  --- last 10 line(s) of stderr (of 38 after folding 38 raw)
      forward_test.go:315: Forward = {Wanted:false Needed:false}, want Needed and Wanted both true with the switch on
      forward_test.go:322: writes = [], want ["sysctl -w net.ipv4.ip_forward=1" "iptables -A FORWARD -i wgmesh-t3-none -o wgmesh-t3-none -j ACCEPT"]
  === RUN   TestForwardNeededForIntroducerAndAdvertiser/neither
  --- FAIL: TestForwardNeededForIntroducerAndAdvertiser (0.00s)
      --- FAIL: TestForwardNeededForIntroducerAndAdvertiser/introducer (0.00s)
      --- FAIL: TestForwardNeededForIntroducerAndAdvertiser/advertiser (0.00s)
      --- PASS: TestForwardNeededForIntroducerAndAdvertiser/neither (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/daemon	0.008s
  FAIL
  ```
- 2026-11-16 · ef8b23a* · exit 0 · `set -o pipefail …` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:9991
- 2026-11-16 · ef8b23a* · exit 0 · `set -o pipefail …` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:10153
- 2026-11-16 · ef8b23a* · exit 0 · `set -o pipefail …` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:10048
- 2026-11-16 · ef8b23a* · exit 0 · `set -o pipefail …` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:9364
- 2026-11-16 · ef8b23a* · exit 0 · `set -o pipefail …` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:9806
- 2026-11-16 · ef8b23a* · exit 0 · `set -o pipefail …` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:9772
- 2026-11-16 · ef8b23a* · exit 0 · `set -o pipefail …` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:6716
- 2026-11-16 · 823f97e* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:1 · test-lock-sha256:04b98b328ea4ec48619b208da00775ad3db501c1c6f124d304137190c0cdf020 · test-lock-file:docs/adr/locks/04b98b328ea4ec48619b208da00775ad3db501c1c6f124d304137190c0cdf020.lock · test-lock-kind:replace
- 2026-11-16 · 823f97e* · exit 0 · `set -o pipefail …` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:13767
- 2026-11-16 · 823f97e* · exit 0 · `set -o pipefail …` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:16204
- 2026-11-16 · 823f97e* · exit 0 · `set -o pipefail …` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:16632
- 2026-11-16 · 823f97e* · exit 0 · `set -o pipefail …` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:16039
- 2026-11-16 · 823f97e* · exit 0 · `set -o pipefail …` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:12620
- 2026-11-16 · 823f97e* · exit 0 · `set -o pipefail …` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:10389
- 2026-11-16 · 823f97e* · exit 0 · `set -o pipefail …` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:13036
- 2026-11-16 · 8e9f14f* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:0 · test-lock-sha256:04b98b328ea4ec48619b208da00775ad3db501c1c6f124d304137190c0cdf020 · test-lock-file:docs/adr/locks/04b98b328ea4ec48619b208da00775ad3db501c1c6f124d304137190c0cdf020.lock · test-lock-kind:replace
- 2026-11-16 · f110717* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:0 · test-lock-sha256:04b98b328ea4ec48619b208da00775ad3db501c1c6f124d304137190c0cdf020 · test-lock-file:docs/adr/locks/04b98b328ea4ec48619b208da00775ad3db501c1c6f124d304137190c0cdf020.lock · test-lock-kind:replace
- 2026-11-16 · fe6f879* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:f66ca0d9dc34ca3629c02f0cb73945ec021fcbf46e79bbcdb6a12b14790a3b68 · ms:1 · test-lock-sha256:04b98b328ea4ec48619b208da00775ad3db501c1c6f124d304137190c0cdf020 · test-lock-file:docs/adr/locks/04b98b328ea4ec48619b208da00775ad3db501c1c6f124d304137190c0cdf020.lock · test-lock-kind:replace
