# Task ADR-0006-T6: Amend the specs, write the release note and the backlog rows

**Depends-on:** T3, T4, T5, T7
**Covers:** none — no spec
**Estimated scope:** S (documents and one test)
**Owner:** unassigned
**Produces:** none
**Consumes:** `Mesh.Validate`, `StrictState` (T3); `NodeResult`, `DeployError`, `PersistFailureFatal` (T4); `resolveEndpoint` (T5)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The centralized specs, `docs/centralized-mode.md` and a release note describe quoting, validation, the per-node summary and the endpoint rule as built and name the two later flips, and a test fails if they stop naming them.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `eidos/spec - centralized SSH deploy - diff-based persistent WireGuard config pushed via SSH.md` | edit | line 20 says a node whose public IP differs gets no endpoint: the new order of rules, the per-node result, quoting |
| `eidos/spec - mesh state and access control - centralized node registry with policy-filtered peer configuration.md` | edit | the validation rules and their classes |
| `docs/centralized-mode.md` | edit | the example keeps its placeholders but says so; the `public_endpoint` override, the all-private rule and the DNS-name caveat; the summary text; the two later flips |
| `docs/centralized-rollout.md` | add | operator and release note for N, N+1 and N+2: what class A and class B are, how to read the warnings as the go/no-go for `StrictState`, what a failed run leaves behind and that a second run converges, the persist warning and its flip |
| `README.md` | edit | one link to the note |
| `docs/adr/BACKLOG.md` | edit | rows for N+1 (`StrictState`), N+2 (`PersistFailureFatal`), `-only` and `-skip`, and deleting the unreachable builders, each naming ADR-0006 |
| `pkg/mesh/spec_central_test.go` | add | `TestSpecNamesCentralized` |

## Ordered Steps

1. [S1] Add `TestSpecNamesCentralized`: it reads the two specs, `docs/centralized-rollout.md` and `docs/adr/BACKLOG.md` and fails when `StrictState`, `PersistFailureFatal`, `public_endpoint`, `class A`, `class B` and `N+2` (note) or `ADR-0006` (backlog) are missing. Run it red.
2. [S2] Amend the two specs and `docs/centralized-mode.md`.
3. [S3] Write `docs/centralized-rollout.md`; link it from the README.
4. [S4] Edit `docs/adr/BACKLOG.md`; run `adr-debt docs/adr` and paste that no deferred row of ADR-0006 is reported unreceipted. [proof: acceptance]
5. [S5] Run the mutants with `adr-verify --mutant`: the note stops naming `PersistFailureFatal`; the backlog loses the N+2 row; the centralized spec keeps the old NAT sentence. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/mesh -run '^TestSpecNamesCentralized$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 1 ] && test -f docs/centralized-rollout.md && ! adr-debt docs/adr | grep -q UNRECEIPTED
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestSpecNamesCentralized` | `pkg/mesh/spec_central_test.go` | the two specs, the release note and the backlog name the switches, `public_endpoint`, both classes and ADR-0006, and the old NAT sentence is gone from the centralized spec | — | S1, S2, S3, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the test and the documents |
| 2 — something selects it | n/a: documentation |
| 3 — the caller can discover it | the note is linked from the README and from `docs/centralized-mode.md` |
| 4 — it is used | nothing measures this yet |

## Mutation Log

## Invariants

- The old sentence "gets no public endpoint" does not stay in a spec without the new condition.
- Every deferred row names ADR-0006 and a release, so `adr-debt` can resurface it.
- `docs/adr/BACKLOG.md` rows of other records are not edited (ADR-0002 T11 edits the same file: append, never rewrite).

## Risks

- The specs are descriptions, not gates; the test checks names, not meaning. Review owns the meaning.
- Merge order with ADR-0002 T11 on `README.md` and `docs/adr/BACKLOG.md`: both only add lines.

## Stop Condition

Stop and ask if T3, T4 or T5 changed a name this task's test requires.

## Out of Scope

- Flipping `StrictState` or `PersistFailureFatal` (deferred: docs/adr/BACKLOG.md).
- A change to `docs/envelope-rollout.md` (permanent: boundary: that note is ADR-0001's).

## Verification Log
