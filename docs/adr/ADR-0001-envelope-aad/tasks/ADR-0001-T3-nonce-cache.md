# Task ADR-0001-T3: Drop a datagram whose nonce was seen inside the freshness window

**Depends-on:** none
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `nonceCache` in `pkg/discovery`
**Consumes:** none
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`PeerExchange.handleMessage` processes a given datagram once: a second arrival of the same nonce inside `MaxMessageAge` is dropped, and the cache never holds more than 65536 entries.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/discovery/replay_cache.go` | add | `nonceCache` with TTL, hard cap, injectable clock |
| `pkg/discovery/exchange.go` | edit | create the cache in the constructor, check it right after `OpenEnvelopeRaw` succeeds |
| `pkg/discovery/replay_cache_test.go` | add | cache unit tests |
| `pkg/discovery/exchange_aad_test.go` | edit | replay through `handleMessage` |

Selection: the check sits in `handleMessage` after the envelope opens; `TestReplayedHelloIsAnsweredOnce` goes red if that line is deleted.

## Ordered Steps

1. [S1] Add the tests and run them red.
2. [S2] Implement `nonceCache`: `Seen(nonce [12]byte) bool` records the nonce and returns whether it was already present; entries older than the TTL are ignored and removed; at the cap the oldest entry is evicted. Guard it with one mutex, and use an injectable `now`.
3. [S3] Give `PeerExchange` a `nonceCache` (TTL = `crypto.MaxMessageAge`, cap 65536 as a named constant) and drop the datagram when `Seen` returns true, logging at debug level.
4. [S4] Run the mutants: remove the check in `handleMessage`; remove the eviction. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/discovery -run '^(TestNonceCacheRejectsRepeat|TestNonceCacheExpires|TestNonceCacheStaysUnderCap|TestReplayedHelloIsAnsweredOnce)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 4 ] && go test -race ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestNonceCacheRejectsRepeat` | `pkg/discovery/replay_cache_test.go` | the second `Seen` of one nonce is true, a different nonce is false | — | S1, S2 |
| `TestNonceCacheExpires` | `pkg/discovery/replay_cache_test.go` | after the TTL the same nonce is accepted again | — | S2 |
| `TestNonceCacheStaysUnderCap` | `pkg/discovery/replay_cache_test.go` | inserting more than the cap keeps the size at or below the cap and evicts the oldest | — | S2 |
| `TestReplayedHelloIsAnsweredOnce` | `pkg/discovery/exchange_aad_test.go` | the same HELLO datagram delivered twice produces one reply | — | S1, S3 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the four tests |
| 2 — something selects it | the call in `handleMessage`; deleting it turns `TestReplayedHelloIsAnsweredOnce` red |
| 3 — the caller can discover it | the debug log line "dropped replayed datagram" |
| 4 — it is used | nothing measures drops yet |

## Mutation Log

## Invariants

- The cache is bounded in memory by the cap, whatever the traffic.
- Datagrams sealed with a fresh nonce (every `SealEnvelope` call) are never dropped.
- `OpenEnvelopeRaw`, LAN, gossip and registry readers do not use the cache.

## Risks

- A restart empties the cache, so a replay inside the window after a restart is accepted once; the timestamp window still bounds it.
- Test doubles: a fake clock replaces `time.Now` in the cache tests; the real clock is only exercised through `handleMessage`.

## Stop Condition

Stop and ask if a sender is found that re-sends identical bytes on purpose (a retransmit of the same sealed datagram).

## Out of Scope

- A replay cache in the LAN, gossip or registry readers (see the ADR's Out of Scope).

## Verification Log
