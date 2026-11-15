# Task ADR-0002-T11: Amend the specs, write the release notes and the backlog rows

**Depends-on:** T3, T5, T6, T7, T8, T9, T10
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** none
**Consumes:** `Withdraw`, tombstones (T3); persisted resolved IP (T5); `EnforceRouteBinding`, compat fixtures, `wgmesh_admit_warnings_total` (T6); `ResolveEndpoint` (T7); `crypto.FilterAnnouncement` (T8); `LocalNode.ObserveEndpoint` (T9); `Policy.AcceptRoutes`, `Config.AcceptRoutes`, rule R9 (T10)
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
| `docs/admit-rollout.md` | add | operator and release note for N, N+1 and N+2: what each switch does, how to read `wgmesh_admit_warnings_total{rule}` as the go/no-go, what a rotated address is, the grinding limit with the number measured in T6, the 5-minute tombstone, the `accept-routes` list and what open means, how HA networks and the default route are treated |
| `docs/adr/BACKLOG.md` | edit | the ADR-0001 row about the winner rule and binding is rewritten to say it is taken up by ADR-0002; rows for N+1, N+2 (which now names `accept-routes`), the multi-candidate local endpoint and the `install-service` route check |
| `pkg/node/spec_admit_test.go` | add | the specs, the note and the backlog must name the rules |

## Ordered Steps

1. [S1] Add `pkg/node/spec_admit_test.go` with `TestSpecNamesAdmit`: it reads the three specs, `docs/admit-rollout.md` and `docs/adr/BACKLOG.md` and fails when `Offer`, `Withdraw` and `tombstone` (peer store spec), `FilterAnnouncement` (exchange spec), `ResolveClaims` (daemon spec), `EnforceIPBinding`, `EnforceRouteBinding`, `N+1`, `N+2`, `accept-routes` and `wgmesh_admit_warnings_total` (note) or `ADR-0002` (backlog) are missing. Run it red.
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
- 2026-11-16 · db371c0* · mutant killed · exit 1 · `pkg/node/spec_admit_test.go` · the peer store spec stops naming the tombstone (the doc path has spaces: the name list of the test is mutated instead) · acceptance-sha256:37212b5ab071f173c64dd4af724256dada8f0ac18daa1ef032111e10e965882f
- 2026-11-16 · db371c0* · mutant killed · exit 1 · `docs/admit-rollout.md` · docs/admit-rollout.md stops naming wgmesh_admit_warnings_total · acceptance-sha256:37212b5ab071f173c64dd4af724256dada8f0ac18daa1ef032111e10e965882f
- 2026-11-16 · db371c0* · mutant killed · exit 1 · `docs/adr/BACKLOG.md` · the backlog loses the ADR-0002 rows (N+1, N+2 and the known limits) · acceptance-sha256:37212b5ab071f173c64dd4af724256dada8f0ac18daa1ef032111e10e965882f

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

## Verification Log
- 2026-11-16 · db371c0* · exit 0 · `set -o pipefail …` · acceptance-sha256:37212b5ab071f173c64dd4af724256dada8f0ac18daa1ef032111e10e965882f · ms:1796
- 2026-11-16 · db371c0* · exit 0 · `set -o pipefail …` · acceptance-sha256:37212b5ab071f173c64dd4af724256dada8f0ac18daa1ef032111e10e965882f · ms:1835
- 2026-11-16 · db371c0* · exit 0 · `set -o pipefail …` · acceptance-sha256:37212b5ab071f173c64dd4af724256dada8f0ac18daa1ef032111e10e965882f · ms:1530
- 2026-11-16 · db371c0* · exit 0 · `set -o pipefail …` · acceptance-sha256:37212b5ab071f173c64dd4af724256dada8f0ac18daa1ef032111e10e965882f · ms:1030
- 2026-11-16 · db371c0* · exit 1 · `set -o pipefail …` · acceptance-sha256:37212b5ab071f173c64dd4af724256dada8f0ac18daa1ef032111e10e965882f · ms:723 · test-lock-sha256:2f36df406f31baaf3cb6638c0aa3d635bebc3506ed7c2f89281ebd89ae09079d · test-lock-file:docs/adr/locks/2f36df406f31baaf3cb6638c0aa3d635bebc3506ed7c2f89281ebd89ae09079d.lock
  ```
  --- last 10 line(s) of stderr (of 12 after folding 12 raw)
      spec_admit_test.go:33: eidos/spec - peer store - thread-safe registry with endpoint ranking and pub-sub.md does not name "Withdraw"
      spec_admit_test.go:33: eidos/spec - peer store - thread-safe registry with endpoint ranking and pub-sub.md does not name "tombstone"
      spec_admit_test.go:33: eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md does not name "FilterAnnouncement"
      spec_admit_test.go:33: eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md does not name "ResolveClaims"
      spec_admit_test.go:33: eidos/spec - discovery dht core - bittorent mainline dht announce query persistence and endpoint detection.md does not name "ObserveEndpoint"
      spec_admit_test.go:28: docs/admit-rollout.md: open ../../docs/admit-rollout.md: no such file or directory
  --- FAIL: TestSpecNamesAdmit (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/node	0.003s
  FAIL
  ```
