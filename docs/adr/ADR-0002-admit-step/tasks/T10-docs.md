# Task ADR-0002-T10: Amend the specs, write the release notes and the backlog rows

**Depends-on:** T3, T5, T6, T7, T8, T9
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** none
**Consumes:** `Withdraw`, tombstones (T3); persisted resolved IP (T5); `EnforceRouteBinding`, compat fixtures, `wgmesh_admit_warnings_total` (T6); `ResolveEndpoint` (T7); `crypto.FilterAnnouncement` (T8); `LocalNode.ObserveEndpoint` (T9)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The peer store, exchange, DHT core and daemon support specs and a release note describe `Offer`, the tombstone, the claim order, the three-release plan and the warning counter as built, and a test fails if they stop naming them.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `eidos/spec - peer store - thread-safe registry with endpoint ranking and pub-sub.md` | edit | `Offer`, `Hand`, `Withdraw`, the tombstone, the guessed-endpoint rank |
| `eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md` | edit | sources call `Offer` and `Withdraw`, one announcement filter |
| `eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md` | edit | claim order, the local node in collisions, the persisted resolved IP |
| `eidos/spec - discovery dht core - bittorent mainline dht announce query persistence and endpoint detection.md` | edit | `ObserveEndpoint` and the STUN rule |
| `README.md`, `docs/envelope-rollout.md` | edit | one link each to the new note |
| `docs/admit-rollout.md` | add | operator and release note for N, N+1 and N+2: what each switch does, how to read `wgmesh_admit_warnings_total{rule}` as the go/no-go, what a rotated address is, the grinding limit with the number measured in T6, the 10-minute tombstone |
| `docs/adr/BACKLOG.md` | edit | the ADR-0001 row about the winner rule and binding is rewritten to say it is taken up by ADR-0002; rows for N+1, N+2, route approval, the multi-candidate local endpoint and the `install-service` route check |
| `pkg/node/spec_admit_test.go` | add | the specs, the note and the backlog must name the rules |

## Ordered Steps

1. [S1] Add `pkg/node/spec_admit_test.go` with `TestSpecNamesAdmit`: it reads the three specs, `docs/admit-rollout.md` and `docs/adr/BACKLOG.md` and fails when `Offer`, `Withdraw` and `tombstone` (peer store spec), `FilterAnnouncement` (exchange spec), `ResolveClaims` (daemon spec), `EnforceIPBinding`, `EnforceRouteBinding`, `N+1`, `N+2` and `wgmesh_admit_warnings_total` (note) or `ADR-0002` (backlog) are missing. Run it red.
2. [S2] Amend the three specs.
3. [S3] Write `docs/admit-rollout.md`.
4. [S4] Edit `docs/adr/BACKLOG.md`; run `adr-debt docs/adr` and paste that no deferred row of ADR-0002 is reported unreceipted.
5. [S5] Run the mutants with `adr-verify --mutant`: the peer store spec stops naming the tombstone; the note stops naming the counter; the backlog loses the N+2 row. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/node -run '^TestSpecNamesAdmit$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 1 ] && test -f docs/admit-rollout.md && adr-debt docs/adr | grep -v UNRECEIPTED >/dev/null && ! adr-debt docs/adr | grep -q UNRECEIPTED
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestSpecNamesAdmit` | `pkg/node/spec_admit_test.go` | the three specs, the release note and the backlog name the admit rules, the switches, the counter and ADR-0002 | — | S1, S2, S3, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the test and the documents |
| 2 — something selects it | n/a: documentation |
| 3 — the caller can discover it | the note is linked from the README security section and from `docs/envelope-rollout.md` |
| 4 — it is used | nothing measures this yet |

## Mutation Log

## Invariants

- The documents say N+1 and N+2 are not done; they are backlog rows, not tasks here.
- Every measured number in them carries its date and what it was measured on.

## Risks

- The documents drift from the code; the test checks names, not meaning, and review owns the rest.
- Test doubles: none.

## Stop Condition

Stop and ask if a spec section named here does not exist in the file (the spec file names were read on 2026-11-15 from `ls eidos`; headings were not).

## Out of Scope

- Flipping the switches (deferred: docs/adr/BACKLOG.md).
- Route approval design (deferred: docs/adr/BACKLOG.md).

## Verification Log
