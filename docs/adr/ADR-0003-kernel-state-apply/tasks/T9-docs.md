# Task ADR-0003-T9: Amend the specs, write the operator note and the backlog rows

**Depends-on:** T3, T4, T5, T6, T7
**Blocked-on:** ADR-0002's documentation task has landed on main: `test -s docs/admit-rollout.md` exits 0 (both amend the daemon support spec) Landed on main at `f3860c6` (2026-11-16): the command exits 0.
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** none
**Consumes:** the result log, `wgmesh_kernel_changes_total`, `wgmesh_kernel_warnings_total` (T3); `EnforceForwardScope`, rule `forward-scope` (T4); `EnforceRouteGuard`, rules `attached-network`, `host-default` (T5); the Darwin adapter (T6); the address target (T7)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The reconciliation and daemon support specs and an operator note describe observe, diff, apply, the single owner of a shared network, forwarding and the N+1 switches (T8 adds the revert one) as built, and a test fails if they stop naming them.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `eidos/spec - daemon reconciliation - peer config sync with relay routing fallback.md` | edit | the signature check is replaced by a read of `wg show dump` (the lines at 24 and 51 of the file at `b8b83f3` say a string cache and an optimistic lock); the cycle reads kernel state, computes one desired state, applies the difference |
| `eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md` | edit | Route sync: both families, one owner per network, no "no-op on other platforms" (line 45), forwarding converged once instead of "after each route apply" (line 47) |
| `docs/kernel-state.md` | add | operator note for N and N+1: the owner rule and its failover time (stated as a bound, not measured), what `forward-scope` and `attached-network` do, how to read `wgmesh_kernel_warnings_total{rule}` as the go/no-go, the default route and the transport-loop caveat of decision 6, macOS routes, the log lines (T3 replaced `Failed to configure peer` and `Failed to remove obsolete peer` with change-only `Failed to set peer` and `Failed to remove peer`), that a failed `wg show dump` skips the whole tick including routes, that a restart pushes every peer endpoint once, the unbounded `wg` fork held under the converge lock until ADR-0007 lands, and the stickiness limit for a relayed owner of a shared network (a BACKLOG row) |
| `README.md` | edit | one link to the note |
| `docs/adr/BACKLOG.md` | edit | rows for N+1 (`EnforceForwardScope`, `EnforceRouteGuard`, `EnforceRevertOnExit`, `EnforceAddressRemoval`), the transport exemption of an accepted default route, and removing the `setInterfaceAddress` call in `collision.go` |
| `pkg/daemon/spec_converge_test.go` | add | the specs, the note and the backlog must name the rules |

## Ordered Steps

1. [S1] Add `TestSpecNamesConverge`: it reads the two specs, `docs/kernel-state.md` and `docs/adr/BACKLOG.md` and fails when `observe`, `desired state` and `dump` (reconciliation spec), `one owner` and `forwarding` (daemon support spec), `EnforceForwardScope`, `EnforceRouteGuard`, `wgmesh_kernel_warnings_total` and `fwmark` (note), or `ADR-0003` (backlog) are missing. Run it red.
2. [S2] Amend the two specs.
3. [S3] Write `docs/kernel-state.md`.
4. [S4] Edit `docs/adr/BACKLOG.md`; run `adr-debt docs/adr` and paste that no deferred row of ADR-0003 is reported unreceipted. [proof: acceptance]
5. [S5] Run the mutants with `adr-verify --mutant`: the reconciliation spec stops naming the dump; the note stops naming the counter; the backlog loses the N+1 row. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/daemon -run '^TestSpecNamesConverge$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 1 ] && test -s docs/kernel-state.md && grep -q 'ADR-0003' docs/adr/BACKLOG.md && adr-debt docs/adr
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestSpecNamesConverge` | `pkg/daemon/spec_converge_test.go` | the specs, the note and the backlog name the mechanisms and switches listed in S1 | — | S1, S2, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the test |
| 2 — something selects it | `go test ./pkg/daemon` runs it |
| 3 — the caller can discover it | README links the note |
| 4 — it is used | the N+1 go/no-go reads the note |

## Mutation Log
- 2026-11-16 · 6b0f7e1* · mutant killed · exit 1 · `eidos/spec - daemon reconciliation - peer config sync with relay routing fallback.md` · the dump is not named: TestSpecNamesConverge must go red · acceptance-sha256:a06123c62eaffd073098a3fbf51e5be235a25467594d20492449373834533e78
- 2026-11-16 · 6b0f7e1* · mutant killed · exit 1 · `docs/kernel-state.md` · the counter is not named: TestSpecNamesConverge must go red · acceptance-sha256:a06123c62eaffd073098a3fbf51e5be235a25467594d20492449373834533e78
- 2026-11-16 · 6b0f7e1* · mutant killed · exit 1 · `docs/adr/BACKLOG.md` · N+1 row head gone: TestSpecNamesConverge must go red · acceptance-sha256:a06123c62eaffd073098a3fbf51e5be235a25467594d20492449373834533e78

## Invariants

- The documents say N+1 is not done; it is a backlog row, not a task here.
- Every measured number carries its date and what it was measured on; failover time is stated as a bound from `HandshakeStaleAfter`, not as a measurement.

## Risks

- The documents drift from the code; the test checks names, not meaning, and review owns the rest.
- ADR-0002's documentation task edits the same daemon support spec; this task starts after it.
- Test doubles: none.

## Stop Condition

Stop and ask if a spec section named here does not exist in the file (the sections were read on 2026-11-15; the file's line numbers above are from `b8b83f3`).

## Out of Scope

- Flipping the switches (deferred: docs/adr/BACKLOG.md).

## Verification Log
- 2026-11-16 · 6b0f7e1* · exit 1 · `set -o pipefail …` · acceptance-sha256:a06123c62eaffd073098a3fbf51e5be235a25467594d20492449373834533e78 · ms:1177 · test-lock-sha256:5de66f935ff447b2f7534d620c613376d503878eb610b6edafc0924f5e3e8736 · test-lock-file:docs/adr/locks/5de66f935ff447b2f7534d620c613376d503878eb610b6edafc0924f5e3e8736.lock
  ```
  --- last 10 line(s) of stderr (of 24 after folding 24 raw)
      spec_converge_test.go:158: eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md: line starting "- A default route (" matched 0 lines, want 1
      spec_converge_test.go:158: eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md: line starting "- A network equal to or inside one this host is attached to" matched 0 lines, want 1
      spec_converge_test.go:158: eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md: line starting "- Linux uses `ip`; macOS reads" matched 0 lines, want 1
      spec_converge_test.go:158: eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md: line starting "- **Forwarding is converged once:**" matched 0 lines, want 1
      spec_converge_test.go:158: eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md: line starting "> [[pkg/daemon/converge.go]]" matched 0 lines, want 1
      spec_converge_test.go:157: docs/kernel-state.md: cannot be read: open ../../docs/kernel-state.md: no such file or directory
  --- FAIL: TestSpecNamesConverge (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/daemon	0.004s
  FAIL
  ```
- 2026-11-16 · 6b0f7e1* · exit 0 · `set -o pipefail …` · acceptance-sha256:a06123c62eaffd073098a3fbf51e5be235a25467594d20492449373834533e78 · ms:2823
- 2026-11-16 · 6b0f7e1* · exit 0 · `set -o pipefail …` · acceptance-sha256:a06123c62eaffd073098a3fbf51e5be235a25467594d20492449373834533e78 · ms:3405
- 2026-11-16 · 6b0f7e1* · exit 0 · `set -o pipefail …` · acceptance-sha256:a06123c62eaffd073098a3fbf51e5be235a25467594d20492449373834533e78 · ms:2775
- 2026-11-16 · 6b0f7e1* · exit 0 · `set -o pipefail …` · acceptance-sha256:a06123c62eaffd073098a3fbf51e5be235a25467594d20492449373834533e78 · ms:1355
