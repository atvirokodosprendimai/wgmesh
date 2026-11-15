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
- 2026-11-15 · e6a1df0* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · the legacy GOODBYE gate is switched off: a legacy GOODBYE removes a peer · acceptance-sha256:834959bb173789d22d14bf83cdfccd3040b6269697dbf3e87e20197e0afb7213
- 2026-11-15 · e6a1df0* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · the gate refuses every GOODBYE, including AAD-bound ones · acceptance-sha256:834959bb173789d22d14bf83cdfccd3040b6269697dbf3e87e20197e0afb7213
- 2026-11-15 · e6a1df0* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · SendGoodbye goes back to the legacy form and graceful shutdown stops working between upgraded nodes · acceptance-sha256:834959bb173789d22d14bf83cdfccd3040b6269697dbf3e87e20197e0afb7213
- 2026-11-15 · e6a1df0* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · legacy opens are no longer counted · acceptance-sha256:834959bb173789d22d14bf83cdfccd3040b6269697dbf3e87e20197e0afb7213
- 2026-11-15 · 9899d38* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · the log throttle map is no longer bounded · acceptance-sha256:834959bb173789d22d14bf83cdfccd3040b6269697dbf3e87e20197e0afb7213
- 2026-11-15 · 9899d38* · mutant inconclusive · exit 1 · `pkg/discovery/exchange.go` · the log throttle no longer throttles · acceptance-sha256:834959bb173789d22d14bf83cdfccd3040b6269697dbf3e87e20197e0afb7213
  ```
  the fence failed on a build/parse error, not an assertion
  ```
- 2026-11-15 · 9899d38* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · the log throttle cooldown is zero, so every packet is logged (compiles, unlike dropping the condition) · acceptance-sha256:834959bb173789d22d14bf83cdfccd3040b6269697dbf3e87e20197e0afb7213

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
- 2026-11-15 · e44ce98* · exit 1 · `set -o pipefail …` · acceptance-sha256:834959bb173789d22d14bf83cdfccd3040b6269697dbf3e87e20197e0afb7213 · ms:1346 · test-lock-sha256:dc75ca16741647272b085b0a2c3615e4e52263c3c7a169fb3e8349973846a9db · test-lock-file:docs/adr/locks/dc75ca16741647272b085b0a2c3615e4e52263c3c7a169fb3e8349973846a9db.lock
  ```
  --- last 10 line(s) of stderr (of 45 after folding 48 raw)
  2026/11/15 17:25:49 [Exchange] Received valid HELLO from 198.51.100.7:51821
  2026/11/15 17:25:49 [Exchange] Dropping invalid HELLO from 198.51.100.7:51821: MeshIP: empty IP
  2026/11/15 17:25:49 [Exchange] Received valid REPLY from 198.51.100.7:51821
  2026/11/15 17:25:49 [Exchange] Dropping invalid REPLY from 198.51.100.7:51821: MeshIP: empty IP
  2026/11/15 17:25:49 [Exchange] Received valid ANNOUNCE from 198.51.100.7:51821
  2026/11/15 17:25:49 [Exchange] Dropping invalid ANNOUNCE from 198.51.100.7:51821: MeshIP: empty IP
  --- PASS: TestLegacyGoodbyeRetypedAsHelloIsRejected (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/discovery	0.010s
  FAIL
  ```
- 2026-11-15 · e6a1df0* · exit 0 · `set -o pipefail …` · acceptance-sha256:834959bb173789d22d14bf83cdfccd3040b6269697dbf3e87e20197e0afb7213 · ms:17583
- 2026-11-15 · e6a1df0* · exit 0 · `set -o pipefail …` · acceptance-sha256:834959bb173789d22d14bf83cdfccd3040b6269697dbf3e87e20197e0afb7213 · ms:14732
- 2026-11-15 · e6a1df0* · exit 0 · `set -o pipefail …` · acceptance-sha256:834959bb173789d22d14bf83cdfccd3040b6269697dbf3e87e20197e0afb7213 · ms:8216
- 2026-11-15 · e6a1df0* · exit 0 · `set -o pipefail …` · acceptance-sha256:834959bb173789d22d14bf83cdfccd3040b6269697dbf3e87e20197e0afb7213 · ms:9362
- 2026-11-15 · 9899d38* · exit 0 · `set -o pipefail …` · acceptance-sha256:834959bb173789d22d14bf83cdfccd3040b6269697dbf3e87e20197e0afb7213 · ms:16583
- 2026-11-15 · 9899d38* · exit 0 · `set -o pipefail …` · acceptance-sha256:834959bb173789d22d14bf83cdfccd3040b6269697dbf3e87e20197e0afb7213 · ms:8831
- 2026-11-15 · 9899d38* · exit 0 · `set -o pipefail …` · acceptance-sha256:834959bb173789d22d14bf83cdfccd3040b6269697dbf3e87e20197e0afb7213 · ms:8683
- 2026-11-15 · 9899d38* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:834959bb173789d22d14bf83cdfccd3040b6269697dbf3e87e20197e0afb7213 · ms:1 · test-lock-sha256:860fcc1f8718023ffaf25b029bc8ffdc038927695a40c90b4fc868235d81124e · test-lock-file:docs/adr/locks/860fcc1f8718023ffaf25b029bc8ffdc038927695a40c90b4fc868235d81124e.lock · test-lock-kind:replace
