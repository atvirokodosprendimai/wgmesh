# Task ADR-0001-T2: A legacy envelope cannot remove a peer, and legacy opens are counted

**Depends-on:** T1
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `legacyEnvelopeOpens` counter in `pkg/discovery`
**Consumes:** `Envelope.LegacyAAD`, `SealEnvelopeAAD` (T1)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`PeerExchange.handleMessage` ignores a GOODBYE that opened only as a legacy envelope, and every legacy open is counted and logged once per source address per minute.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/discovery/exchange.go` | edit | GOODBYE gate on `envelope.LegacyAAD`; counter; clearer log line for undecryptable packets |
| `pkg/discovery/exchange_aad_test.go` | add | tests below |

Selection: `handleMessage` is the only dispatcher on `envelope.MessageType` in the exchange path; the gate is inside the GOODBYE case, and `TestLegacyGoodbyeIsIgnored` goes red if the `LegacyAAD` check is deleted.

## Ordered Steps

1. [S1] Add `pkg/discovery/exchange_aad_test.go` with the three tests and run them red.
2. [S2] In `handleMessage`, when `envelope.LegacyAAD` is true: increment the counter, and for `MessageTypeGoodbye` log "ignored GOODBYE sent without type authentication" and return before `peerStore.Remove`.
3. [S3] Log legacy opens at most once per source IP per minute so a flood cannot fill the log.
4. [S4] Change the undecryptable-packet line so it also mentions that a node older than the AAD release may be the sender. [proof: acceptance]
5. [S5] Run the mutant that deletes the `LegacyAAD` check. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/discovery -run '^(TestLegacyGoodbyeIsIgnored|TestAADGoodbyeRemovesPeer|TestLegacyOpensAreCounted)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 3 ] && go test -race ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestLegacyGoodbyeIsIgnored` | `pkg/discovery/exchange_aad_test.go` | a GOODBYE sealed without AAD leaves the peer in the store | — | S1, S2 |
| `TestAADGoodbyeRemovesPeer` | `pkg/discovery/exchange_aad_test.go` | a GOODBYE sealed with `SealEnvelopeAAD` still removes the peer (the gate is not a blanket refusal) | — | S1, S2 |
| `TestLegacyOpensAreCounted` | `pkg/discovery/exchange_aad_test.go` | the counter grows by one per legacy packet and not for AAD packets | — | S1, S2, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the three tests |
| 2 — something selects it | `handleMessage` runs the gate for every received packet; deleting the check turns `TestLegacyGoodbyeIsIgnored` red |
| 3 — the caller can discover it | the log line names the cause and the fix (upgrade the sender) |
| 4 — it is used | the counter is read by the test; exposing it in `daemon.status` is not done here (nothing measures this yet) |

## Mutation Log

## Invariants

- A GOODBYE sealed with AAD still removes a peer, with the same timestamp rules as today.
- The legacy gate never drops HELLO, REPLY or ANNOUNCE: mixed meshes keep discovering each other until N+2.

## Risks

- A legitimate old node shutting down is not removed at once by its GOODBYE; the health check evicts it a few minutes later. Accepted for the rollout window.
- Test doubles: the tests build a `PeerExchange` with an in-memory `PeerStore` and call `handleMessage` directly; the real UDP receive loop and its rate limiter are not exercised.

## Stop Condition

Stop and ask if the gate would also have to cover a type other than GOODBYE.

## Out of Scope

- The nonce cache (T3).
- Refusing legacy envelopes altogether (deferred: docs/adr/BACKLOG.md).

## Verification Log
