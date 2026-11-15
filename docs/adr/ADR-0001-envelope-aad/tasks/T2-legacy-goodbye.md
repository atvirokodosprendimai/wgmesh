# Task ADR-0001-T2: Legacy GOODBYE gate, AAD `SendGoodbye`, legacy counter

**Depends-on:** T1
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `handleMessage` order (open, legacy gate, dispatch), `legacyEnvelopeOpens` counter in `pkg/discovery`
**Consumes:** `Envelope.LegacyAAD`, `SealEnvelopeAAD` (T1)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`PeerExchange.handleMessage` ignores a GOODBYE that opened only as a legacy envelope, `SendGoodbye` seals with AAD so GOODBYE between upgraded nodes still works, and legacy opens are counted.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/discovery/exchange.go` | edit | GOODBYE gate on `envelope.LegacyAAD`; counter; `SendGoodbye` seals with `SealEnvelopeAAD`; clearer line for undecryptable packets |
| `pkg/discovery/exchange_aad_test.go` | add | tests below |

Selection: `handleMessage` is the only dispatcher on `envelope.MessageType` in the exchange path; the gate is inside the GOODBYE case, and `TestLegacyGoodbyeIsIgnored` goes red if the `LegacyAAD` check is deleted. `SendGoodbye` is called from the shutdown path; `TestSendGoodbyeIsAADBound` goes red if it goes back to `SealEnvelope`.

## Ordered Steps

1. [S1] Add `pkg/discovery/exchange_aad_test.go` with the five tests and run them red.
2. [S2] In `handleMessage`, after a successful open: when `envelope.LegacyAAD` is true, increment the counter; for `MessageTypeGoodbye` log "ignored GOODBYE sent without type authentication" and return before `peerStore.Remove`.
3. [S3] Change `SendGoodbye` to seal with `crypto.SealEnvelopeAAD`.
4. [S4] Throttle the legacy-open log with the existing `logIncomingPacket` / `ExchangeLogCooldown` mechanism, not a new one.
5. [S5] Change the undecryptable-packet line so it also mentions that a node older than the AAD release may be the sender. [proof: acceptance]
6. [S6] Run the mutants: delete the `LegacyAAD` check; revert `SendGoodbye` to `SealEnvelope`. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/discovery -run '^(TestLegacyGoodbyeIsIgnored|TestAADGoodbyeRemovesPeer|TestSendGoodbyeIsAADBound|TestLegacyGoodbyeRetypedAsHelloIsRejected|TestLegacyOpensAreCounted)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 5 ] && go test -race ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestLegacyGoodbyeIsIgnored` | `pkg/discovery/exchange_aad_test.go` | a GOODBYE sealed without AAD leaves the peer in the store | — | S1, S2 |
| `TestAADGoodbyeRemovesPeer` | `pkg/discovery/exchange_aad_test.go` | a GOODBYE sealed with `SealEnvelopeAAD` still removes the peer (the gate is not a blanket refusal) | — | S1, S2 |
| `TestSendGoodbyeIsAADBound` | `pkg/discovery/exchange_aad_test.go` | the datagram `SendGoodbye` writes to a loopback socket opens with `LegacyAAD` false, and the receiver removes the peer | — | S1, S3 |
| `TestLegacyGoodbyeRetypedAsHelloIsRejected` | `pkg/discovery/exchange_aad_test.go` | a legacy GOODBYE re-sent as HELLO does not reach the store (relies on `validateExchangeAnnouncement` from PR #876) | — | S1 |
| `TestLegacyOpensAreCounted` | `pkg/discovery/exchange_aad_test.go` | the counter grows by one per legacy packet and not for AAD packets | — | S1, S2, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the five tests |
| 2 — something selects it | `handleMessage` runs the gate for every received packet; deleting the check turns `TestLegacyGoodbyeIsIgnored` red; reverting `SendGoodbye` turns `TestSendGoodbyeIsAADBound` red |
| 3 — the caller can discover it | the log line names the cause and the fix (upgrade the sender) |
| 4 — it is used | the counter is read only by tests here; it is meaningful from release N+1 and is not in `daemon.status` (deferred: docs/adr/BACKLOG.md) |

## Mutation Log

## Invariants

- A GOODBYE sealed with AAD still removes a peer, with the same timestamp rules as today.
- The legacy gate never drops HELLO, REPLY or ANNOUNCE: mixed meshes keep discovering each other until N+2.
- A legacy GOODBYE retyped as HELLO, REPLY or ANNOUNCE never reaches the store.

## Risks

- A node older than N cannot open the AAD GOODBYE and evicts the peer through its health check minutes later. Accepted for the rollout window and stated in the ADR.
- Test doubles: the tests build a `PeerExchange` with an in-memory `PeerStore` and call `handleMessage` directly; `TestSendGoodbyeIsAADBound` uses a real loopback UDP socket. The real receive loop and its rate limiter are not exercised.

## Stop Condition

Stop and ask if the gate would also have to cover a type other than GOODBYE.

## Out of Scope

- The nonce cache (T3).
- Exposing the counter in `daemon.status` (deferred: docs/adr/BACKLOG.md).
- Refusing legacy envelopes altogether (deferred: docs/adr/BACKLOG.md).

## Verification Log
