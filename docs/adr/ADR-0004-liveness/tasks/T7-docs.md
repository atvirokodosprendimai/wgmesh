# Task ADR-0004-T7: Amend the specs, write the release notes and the backlog rows

**Depends-on:** T3, T5, T6
**Covers:** none — no spec
**Estimated scope:** S (few files)
**Owner:** unassigned
**Produces:** none
**Consumes:** `RestoredAt` and `LastAlive` in the cache file (T3); `DHTDiscovery.OnResume` (T5); `daemon.EnforceRelayProbe`, `wgmesh_relay_route_age_seconds` (T6)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The peer store and daemon support specs and a release note describe `Assess`, the evidence times, the wake grace, the restore grace, probe-only relayed peers, the two switches and the three metrics as built, and a test fails if they stop naming them.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `eidos/spec - peer store - thread-safe registry with endpoint ranking and pub-sub.md` | edit | `Assess`, `Alive`, the four evidence times, `GetProbeable`, `ObserveDataPlane` |
| `eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md` | edit | the restore grace, `LastAlive` in the cache file, the resume loop and grace, probe-only relayed peers |
| `README.md`, `docs/envelope-rollout.md` | edit | one link each to the new note |
| `docs/liveness-rollout.md` | add | operator and release note for N, N+1 and N+2: what each switch does; `wgmesh_liveness_disagree_total{site}` read as the N+1 go/no-go; `wgmesh_relay_route_age_seconds` read before N+2; `wgmesh_resume_detected_total`; the keepalive cost of probe-only peers; the 60-second wake grace; what a restored peer looks like in `daemon.status` |
| `docs/adr/BACKLOG.md` | edit | rows for N+1, N+2, OS power events through cgo or a helper, and rebinding the transport socket on a wake |
| `pkg/node/spec_liveness_test.go` | add | the specs, the note and the backlog must name the pieces |

## Ordered Steps

1. [S1] Add `pkg/node/spec_liveness_test.go` with `TestSpecNamesLiveness`: it reads the two specs, `docs/liveness-rollout.md` and `docs/adr/BACKLOG.md` and fails when `Assess`, `ObserveDataPlane` and `GetProbeable` (peer store spec), `ResumeGrace` and `probeOnly` (daemon spec), `EnforceLiveness`, `EnforceRelayProbe`, `N+1`, `N+2`, `wgmesh_liveness_disagree_total` and `wgmesh_relay_route_age_seconds` (note) or `ADR-0004` (backlog) are missing. Run it red.
2. [S2] Amend the two specs.
3. [S3] Write `docs/liveness-rollout.md`.
4. [S4] Edit `docs/adr/BACKLOG.md`; run `adr-debt docs/adr` and paste that no deferred row of ADR-0004 is reported unreceipted.
5. [S5] Run the mutants with `adr-verify --mutant`: the peer store spec stops naming `Assess`; the note stops naming the disagreement counter; the backlog loses the N+2 row. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/node -run '^TestSpecNamesLiveness$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 1 ] && test -f docs/liveness-rollout.md && grep -c 'ADR-0004' docs/adr/BACKLOG.md | grep -qv '^0$' && adr-debt docs/adr
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestSpecNamesLiveness` | `pkg/node/spec_liveness_test.go` | the two specs, the note and the backlog name the pieces above | — | S1, S2, S3, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the test and the note |
| 2 — something selects it | README and `docs/envelope-rollout.md` link the note; `adr-debt` reads the backlog rows |
| 3 — the caller can discover it | the README link |
| 4 — it is used | the N+1 and N+2 releases read the note and the rows |

## Mutation Log

## Invariants

- The note states no number that was not measured; the wake time comes from T4 S6 with its date and host, or it says "not measured".
- The ADR's catalog row stays Proposed until M accepts.

## Risks

- The specs drift from the code again; the test fails on a missing name only, not on a wrong description.

## Stop Condition

Stop and ask if T3, T5 or T6 renamed a symbol this task's test names, or if ADR-0002 T11 edited the same spec sections in a way that cannot be merged line by line.

## Out of Scope

- Flipping `EnforceLiveness` or `EnforceRelayProbe` (deferred: docs/adr/BACKLOG.md).
- ADR-0002's own spec amendments (permanent: boundary: ADR-0002 T11 owns them; this task edits other sections of the same files).

## Verification Log
