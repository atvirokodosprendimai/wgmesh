# ADR-0001 Tasks

Implementation tasks for ADR-0001: Authenticate the envelope type and refuse replays, in a staged wire rollout. See the parent ADR for the decision.

**Source of truth:** the task files' `Depends-on` / `Produces` / `Consumes` / `Covers` headers.
This README is a derived index — when it disagrees with a task file, the task file wins and the
README must be regenerated.

## Execution Order

| Order | Task | Depends-on |
|-------|------|------------|
| 1 | T1 | none |
| 2 | T2 | T1 |
| 3 | T3 | T2 |
| 4 | T4 | T1, T2, T3 |

## Task Index

| ID | Title | Status | Covers | Acceptance |
|----|-------|--------|--------|------------|
| T1 | Bind the envelope type to the GCM tag, keep legacy envelopes opening | done | — | `go test ./pkg/crypto` four named tests, then `-race` |
| T2 | Legacy GOODBYE gate, AAD `SendGoodbye`, legacy counter | done | — | `go test ./pkg/discovery` five named tests, then `-race` |
| T3 | Drop a non-HELLO datagram whose nonce was seen inside the acceptance window | done | — | `go test ./pkg/discovery` five named tests, then `-race` |
| T4 | Amend the crypto and exchange specs and the operator notes | pending | — | `go test ./pkg/crypto -run TestSpecNamesAADRollout` |

## Contract Coupling

| Producer | Contract | Consumer(s) | Ordering note |
|----------|----------|-------------|---------------|
| T1 | `Envelope.LegacyAAD`, `SealEnvelopeAAD` | T2 | T1 before T2 |
| T2 | `handleMessage` order: open, legacy gate, nonce check, dispatch | T3 | T2 before T3 (same function) |
| T1 | `EnvelopeCapabilityVersion = 2`, `v2` fixture | T4 | T1 before T4 |
| T3 | `nonceCache` | T4 | T3 before T4 |

## Notes

- The branch is built on PR #876 because T2 relies on `validateExchangeAnnouncement` from commit `5942a78`.
- Go tests run on the laptop with `go test -race` only for the packages named in each fence; the whole repository gate is `go build ./... && go vet ./... && go test -race ./...`.
- N+1 and N+2 of the rollout are not tasks here; they are in `docs/adr/BACKLOG.md`.
