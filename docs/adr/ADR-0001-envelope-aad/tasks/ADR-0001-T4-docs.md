# Task ADR-0001-T4: Amend the crypto and exchange specs and the operator notes

**Depends-on:** T1, T2, T3
**Covers:** none — no spec
**Estimated scope:** S (single file)
**Owner:** unassigned
**Produces:** none
**Consumes:** `EnvelopeCapabilityVersion = 2`, `v2` fixture (T1); `nonceCache` (T3)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

The crypto spec, the exchange spec and `docs/` describe the AAD-bound envelope, the legacy mode and the three-release rollout as they are built, and a test fails if the spec stops naming them.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `eidos/spec - crypto - secret-derived keys envelope encryption membership proofs and rotation.md` | edit | envelope section: `v` field, AAD bytes, legacy mode, capability version 2 |
| `eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md` | edit | replay cache and the GOODBYE gate |
| `docs/envelope-rollout.md` | add | operator note: what each of N, N+1, N+2 changes and how to read the legacy counter |
| `pkg/crypto/spec_aad_test.go` | add | the spec must name the AAD rule and the rollout |

## Ordered Steps

1. [S1] Add `pkg/crypto/spec_aad_test.go` with `TestSpecNamesAADRollout`: it reads the crypto spec and `docs/envelope-rollout.md` and fails when `SealEnvelopeAAD`, `LegacyAAD`, `EnvelopeCapabilityVersion` or the strings `N+1` and `N+2` are missing. Run it red.
2. [S2] Amend the crypto spec envelope section and the exchange spec.
3. [S3] Write `docs/envelope-rollout.md`.
4. [S4] Run the mutant that removes the AAD paragraph from the spec. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/crypto -run '^TestSpecNamesAADRollout$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 1 ]
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestSpecNamesAADRollout` | `pkg/crypto/spec_aad_test.go` | the spec and the operator note name the AAD rule, the legacy flag and the rollout | — | S1, S2, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the test and the documents |
| 2 — something selects it | n/a: documentation |
| 3 — the caller can discover it | `docs/envelope-rollout.md` is linked from `docs/centralized-mode.md`-style docs index if one exists, otherwise from the README security section |
| 4 — it is used | nothing measures this yet |

## Mutation Log

## Invariants

- The documents describe what T1 to T3 built, not what N+1 or N+2 will do, except in the rollout note.

## Risks

- Documentation drifts from the code later; the test only checks that named terms exist.
- Test doubles: none.

## Stop Condition

Stop and ask if T1 to T3 changed a name this task documents.

## Out of Scope

- Documenting N+1 and N+2 as done (deferred: docs/adr/BACKLOG.md).

## Verification Log
