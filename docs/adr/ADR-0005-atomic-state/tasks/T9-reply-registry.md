# Task ADR-0005-T9: Register every exchange caller's reply channel separately (#855)

**Depends-on:** T8
**Covers:** none — no spec
**Estimated scope:** S (single file)
**Owner:** unassigned
**Produces:** `PeerExchange.registerReply(remote string) (ch chan *PeerInfo, unregister func())`
**Consumes:** none (it edits the file T8 edits, hence the order, but uses no symbol of T8)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

Two `ExchangeWithPeer` calls to the same address each wait on their own channel; a reply reaches both; one caller returning removes only its own registration.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/discovery/exchange.go` | edit | `pendingReplies` becomes `map[string]map[uint64]chan *daemon.PeerInfo` with a counter under `pendingMu` (`:83-84`, `:118`); `registerReply` replaces `setPendingReplyChannel` and `clearPendingReplyChannel` (`:1039-1049`); `ExchangeWithPeer` registers at `:550-552` and defers the returned `unregister`; `handleReply` delivers to every channel registered for the sender with the existing non-blocking send (`:438-444`), replacing `getPendingReplyChannel` (`:1051-1056`) |
| `pkg/discovery/exchange_pending_test.go` | add | tests below |

Selection: `ExchangeWithPeer` is called by `dht.go` (punch, rendezvous, transitive connect); the two-caller test fails if `ExchangeWithPeer` goes back to a single-slot map.

Ordering note: follows ADR-0002 T9 and T8 of this record (same file). `handleMessage`'s order (open, count legacy, nonce cache, dispatch) is ADR-0001's and is not touched; the fixture test `go test ./pkg/crypto ./pkg/discovery -run Envelope` is re-run.

## Ordered Steps

1. [S1] Add the four tests with a compiling stub (`registerReply` over the old single slot) and run red: the second registration replaces the first, and the first caller's unregister deletes the second's.
2. [S2] Implement the registry; the map entry for an address is removed with its last registration so the map does not grow.
3. [S3] Switch `ExchangeWithPeer` and `handleReply`; keep the log lines as they are.
4. [S4] [proof: human: the issue's evidence is code-traced and not reproduced, it needs timed UDP peers; two real nodes exchanging with a third at once on loopback was not run here, the unit tests drive `handleReply` directly] Record in the sign-off whether a loopback run with two simultaneous exchanges was done, with date and host.
5. [S5] Run the mutants with `adr-verify --mutant`: `handleReply` delivers to one channel only; `unregister` deletes the whole address entry; the address entry is never removed. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/discovery -run '^(TestRegisterReplyKeepsBothCallers|TestHandleReplyReachesEveryRegisteredCaller|TestUnregisterRemovesOnlyOwnRegistration|TestUnregisterLastRemovesAddressEntry)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 4 ] && go test -race ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestRegisterReplyKeepsBothCallers` | `pkg/discovery/exchange_pending_test.go` | two registrations for one address give two distinct channels | — | S1, S2 |
| `TestHandleReplyReachesEveryRegisteredCaller` | `pkg/discovery/exchange_pending_test.go` | one reply from that address is received on both channels | — | S1, S2, S3 |
| `TestUnregisterRemovesOnlyOwnRegistration` | `pkg/discovery/exchange_pending_test.go` | caller 1 unregisters, then a reply reaches caller 2 | — | S1, S2 |
| `TestUnregisterLastRemovesAddressEntry` | `pkg/discovery/exchange_pending_test.go` | after both unregister the map has no entry for the address | — | S1, S2 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the four tests |
| 2 — something selects it | `ExchangeWithPeer` registers through it; the old function names are deleted so a stale caller does not compile |
| 3 — the caller can discover it | doc comment on `registerReply` states "every caller gets the reply"; T11 amends the exchange spec |
| 4 — it is used | every punch and rendezvous exchange |

## Mutation Log

## Invariants

- No wire byte changes; the same HELLO and REPLY.
- A reply for an address with no registration is still logged as unsolicited and still updates the peer store (`handleReply` order is unchanged except for the delivery).
- Delivery stays non-blocking: a full channel (capacity 1) is skipped, never waited on.

## Risks

- A reply that answers caller 1's HELLO is also given to caller 2; both are the same peer's announcement (parent Risks).
- Test doubles: none; `PeerExchange` is built with `NewPeerExchange` and `handleReply` is called with a constructed reply, as `exchange_test.go:75` does.

## Stop Condition

Stop and ask if ADR-0002 T8 or T9 changed the signature of `handleReply`, or if a caller outside `exchange.go` uses `pendingReplies`.

## Out of Scope

- A request id on the wire (permanent: boundary: a wire change; M5, option c).
- Retry and backoff policy of the callers in `dht.go` (permanent: boundary: owned by the discovery liveness group).

## Verification Log
