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
| 3 | T3 | none |
| 4 | T4 | T1, T2, T3 |

## Task Index

| ID | Title | Status | Covers | Acceptance |
|----|-------|--------|--------|------------|
| T1 | Bind the envelope type to the GCM tag, keep legacy envelopes opening | pending | — | `go test ./pkg/crypto -run '^(TestEnvelopeTypeIsAuthenticated\|...)$'` |
| T2 | A legacy envelope cannot remove a peer, and legacy opens are counted | pending | — | `go test ./pkg/discovery -run '^(TestLegacyGoodbyeIsIgnored\|...)$'` |
| T3 | Drop a datagram whose nonce was seen inside the freshness window | pending | — | `go test ./pkg/discovery -run '^(TestNonceCache...)$'` |
| T4 | Amend the crypto and exchange specs and the operator notes | pending | — | `go test ./pkg/crypto -run '^TestSpecNamesAADRollout$'` |

## Contract Coupling

| Producer | Contract | Consumer(s) | Ordering note |
|----------|----------|-------------|---------------|
| T1 | `Envelope.LegacyAAD`, `SealEnvelopeAAD` | T2 | T1 before T2 |
| T1 | `EnvelopeCapabilityVersion = 2`, `v2` fixture | T4 | T1 before T4 |
| T3 | `nonceCache` | T4 | T3 before T4 |

## Notes

- Go tests run on the laptop with `go test -race` only for the packages named in each fence; the whole repository gate is `go build ./... && go vet ./... && go test -race ./...`.
- N+1 and N+2 of the rollout are not tasks here; they are in `docs/adr/BACKLOG.md`.
