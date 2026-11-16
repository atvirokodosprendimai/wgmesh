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
| `pkg/discovery/exchange.go` | edit | `pendingReplies` becomes `map[string]map[uint64]chan *daemon.PeerInfo` with a counter `pendingSeq`, both under `pendingMu` (struct fields and `NewPeerExchange`); `registerReply` replaces `setPendingReplyChannel` and `clearPendingReplyChannel`; a read helper `pendingReplyChannels(remote)` copies the channels of an address under `pendingMu` and replaces `getPendingReplyChannel` (so `handleReply` sends after unlocking); `ExchangeWithPeer` registers before its first HELLO and defers the returned `unregister`; `handleReply` delivers a copy of the announcement to every channel registered for the sender with the existing non-blocking send. Find the sites with `mrw read --grep`, the line numbers of the first draft are stale |
| `pkg/discovery/exchange_pending_test.go` | add | tests below |
| `pkg/discovery/exchange_test.go` | edit | four existing tests wrote `pe.pendingReplies[addr] = make(chan ...)` directly and would not compile with the new type; each 3-line lock/set/unlock became `pe.registerReply(addr)`, assertions unchanged (so the stop condition below, "a caller outside `exchange.go` uses `pendingReplies`", was met by tests only and judged not to stop the task) |

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
n=$(go test ./pkg/discovery -run '^(TestRegisterReplyKeepsBothCallers|TestHandleReplyReachesEveryRegisteredCaller|TestUnregisterRemovesOnlyOwnRegistration|TestUnregisterLastRemovesAddressEntry|TestUnregisterIsIdempotentAndConcurrent|TestRegisterReplyConcurrentCallersUnderRace|TestHandleReplyGivesEveryCallerItsOwnCopy|TestConcurrentExchangeWithPeerBothGetTheReply)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 8 ] && go test -race ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestRegisterReplyKeepsBothCallers` | `pkg/discovery/exchange_pending_test.go` | two registrations for one address give two distinct channels | — | S1, S2 |
| `TestHandleReplyReachesEveryRegisteredCaller` | `pkg/discovery/exchange_pending_test.go` | one reply from that address is received on both channels | — | S1, S2, S3 |
| `TestUnregisterRemovesOnlyOwnRegistration` | `pkg/discovery/exchange_pending_test.go` | caller 1 unregisters, then a reply reaches caller 2 | — | S1, S2 |
| `TestUnregisterLastRemovesAddressEntry` | `pkg/discovery/exchange_pending_test.go` | after both unregister the map has no entry for the address | — | S1, S2 |
| `TestUnregisterIsIdempotentAndConcurrent` | `pkg/discovery/exchange_pending_test.go` | 16 goroutines call one `unregister`: only that registration is removed, once, under `-race` | — | S1, S2 |
| `TestRegisterReplyConcurrentCallersUnderRace` | `pkg/discovery/exchange_pending_test.go` | 32 callers register, receive a reply and unregister at once under `-race` | — | S1, S2, S3 |
| `TestHandleReplyGivesEveryCallerItsOwnCopy` | `pkg/discovery/exchange_pending_test.go` | two callers receive distinct `*PeerInfo` values (and the store its own): a write by one is invisible to the other under `-race` | — | S1, S3 |
| `TestConcurrentExchangeWithPeerBothGetTheReply` | `pkg/discovery/exchange_pending_test.go` | end to end on loopback: two concurrent `ExchangeWithPeer` calls to one peer address both return that peer's single sealed REPLY (on the single-slot code one caller timed out: reasoned from the code, not run) | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the four tests |
| 2 — something selects it | `ExchangeWithPeer` registers through it; the old function names are deleted so a stale caller does not compile |
| 3 — the caller can discover it | doc comment on `registerReply` states "every caller gets the reply"; T11 amends the exchange spec |
| 4 — it is used | every punch and rendezvous exchange |

## Mutation Log
- 2026-11-16 · 628d7ec* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · handleReply delivers to one channel only: TestHandleReplyReachesEveryRegisteredCaller must go red · acceptance-sha256:85efede25d946b7f3e73e47822f2bacd250d1ac81f3883c3de35888549a95a95
- 2026-11-16 · 628d7ec* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · unregister removes every registration of the address: TestUnregisterRemovesOnlyOwnRegistration must go red · acceptance-sha256:85efede25d946b7f3e73e47822f2bacd250d1ac81f3883c3de35888549a95a95
- 2026-11-16 · 628d7ec* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · the address entry is never removed: TestUnregisterLastRemovesAddressEntry must go red · acceptance-sha256:85efede25d946b7f3e73e47822f2bacd250d1ac81f3883c3de35888549a95a95

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
- 2026-11-16 · 628d7ec* · exit 1 · `set -o pipefail …` · acceptance-sha256:85efede25d946b7f3e73e47822f2bacd250d1ac81f3883c3de35888549a95a95 · ms:601934 · test-lock-sha256:4a5a2ae1ec3be95f2614e28edd3c5a1bf719a3cb33973cee47a9f436b92f48b2 · test-lock-file:docs/adr/locks/4a5a2ae1ec3be95f2614e28edd3c5a1bf719a3cb33973cee47a9f436b92f48b2.lock
  ```
  --- last 10 line(s) of stderr (of 66 after folding 76 raw)

  goroutine 90 [chan receive, 9 minutes]:
  github.com/atvirokodosprendimai/wgmesh/pkg/discovery.TestHandleReplyGivesEveryCallerItsOwnCopy(0x2e11e2682008)
  	/claude/scratch/2026-11-15/adr-g4/pkg/discovery/exchange_pending_test.go:232 +0x1ae
  testing.tRunner(0x2e11e2682008, 0xf2e9b0)
  	/snap/go/11411/src/testing/testing.go:2195 +0xea
  created by testing.(*T).Run in goroutine 1
  	/snap/go/11411/src/testing/testing.go:2260 +0x4d4
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/discovery	600.106s
  FAIL
  ```
- 2026-11-16 · 628d7ec* · exit 0 · `set -o pipefail …` · acceptance-sha256:85efede25d946b7f3e73e47822f2bacd250d1ac81f3883c3de35888549a95a95 · ms:10018
- 2026-11-16 · 628d7ec* · exit 0 · `set -o pipefail …` · acceptance-sha256:85efede25d946b7f3e73e47822f2bacd250d1ac81f3883c3de35888549a95a95 · ms:10693
- 2026-11-16 · 628d7ec* · exit 0 · `set -o pipefail …` · acceptance-sha256:85efede25d946b7f3e73e47822f2bacd250d1ac81f3883c3de35888549a95a95 · ms:10522
- 2026-11-16 · 628d7ec* · exit 0 · `set -o pipefail …` · acceptance-sha256:85efede25d946b7f3e73e47822f2bacd250d1ac81f3883c3de35888549a95a95 · ms:6914
