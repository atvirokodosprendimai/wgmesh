# Task ADR-0001-T3: Drop a non-HELLO datagram whose nonce was seen inside the acceptance window

**Depends-on:** T2
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `nonceCache` in `pkg/discovery`
**Consumes:** `handleMessage` order (open, legacy gate, dispatch) (T2)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

`PeerExchange.handleMessage` processes a given non-HELLO datagram once: a second arrival of the same nonce inside twice `MaxMessageAge` is dropped, HELLO retransmits are never dropped, and the cache never holds more than 65536 entries.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/discovery/replay_cache.go` | add | `nonceCache` with TTL, hard cap, injectable clock |
| `pkg/discovery/exchange.go` | edit | create the cache in the constructor; check it in `handleMessage` after the legacy gate and before dispatch, for every type except HELLO |
| `pkg/discovery/replay_cache_test.go` | add | cache unit tests |
| `pkg/discovery/exchange_replay_test.go` | add | replay and retransmit through `handleMessage` |

Selection: the check sits in `handleMessage` between the legacy gate (T2) and dispatch; `TestReplayedAnnounceIsProcessedOnce` goes red if that line is deleted, and `TestHelloRetransmitIsNotDropped` goes red if HELLO is not exempted.

## Ordered Steps

1. [S1] Add the five tests and run them red, against a compiling stub of `nonceCache`.
2. [S2] Re-run the send-site audit and record the result in this task: `grep -n "WriteToUDP" pkg/discovery/exchange.go`. As of 2026-11-15 it lists 6 sites and only the `sendHello` closure in `ExchangeWithPeer` (the HELLO retry loop) writes the same sealed bytes more than once. If a second looping site exists, stop (see Stop Condition). [proof: acceptance]
3. [S3] Implement `nonceCache`: `Seen(nonce [12]byte) bool` records the nonce and returns whether it was already present; an entry is ignored and removed once older than the TTL; at the cap the oldest entry is evicted. One mutex, injectable `now`.
4. [S4] Give `PeerExchange` a `nonceCache` (TTL = 2 × `crypto.MaxMessageAge`, cap 65536 as a named constant). In `handleMessage`, for every type except HELLO, drop the datagram when `Seen` returns true, logging at debug level. Record the nonce only after the envelope opened successfully, never before.
5. [S5] Run the mutants: remove the check in `handleMessage`; remove the HELLO exemption; remove the eviction; shorten the TTL to `MaxMessageAge`. [proof: mutation]

## Acceptance

```bash
set -o pipefail
n=$(go test ./pkg/discovery -run '^(TestNonceCacheRejectsRepeat|TestNonceCacheExpires|TestNonceCacheStaysUnderCap|TestReplayedAnnounceIsProcessedOnce|TestHelloRetransmitIsNotDropped)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$n" -ge 5 ] && go test -race ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestNonceCacheRejectsRepeat` | `pkg/discovery/replay_cache_test.go` | the second `Seen` of one nonce is true, a different nonce is false | — | S1, S3 |
| `TestNonceCacheExpires` | `pkg/discovery/replay_cache_test.go` | after the TTL the same nonce is accepted again, and a datagram dated `MaxMessageAge` in the future is still refused at `MaxMessageAge` + 1 s | — | S1, S3 |
| `TestNonceCacheStaysUnderCap` | `pkg/discovery/replay_cache_test.go` | inserting more than the cap keeps the size at or below the cap and evicts the oldest | — | S1, S3 |
| `TestReplayedAnnounceIsProcessedOnce` | `pkg/discovery/exchange_replay_test.go` | the same ANNOUNCE datagram delivered twice reaches the handler once | — | S1, S4 |
| `TestHelloRetransmitIsNotDropped` | `pkg/discovery/exchange_replay_test.go` | the same HELLO datagram delivered twice is answered both times | — | S1, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the five tests |
| 2 — something selects it | the call in `handleMessage`; deleting it turns `TestReplayedAnnounceIsProcessedOnce` red |
| 3 — the caller can discover it | the debug log line "dropped replayed datagram" |
| 4 — it is used | nothing measures drops yet |

## Mutation Log
- 2026-11-15 · 0d3baff* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · handleMessage no longer drops a replayed datagram · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8
- 2026-11-15 · 0d3baff* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · HELLO retransmits are dropped by the cache, so the NAT punch cannot recover a lost reply · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8
- 2026-11-15 · 0d3baff* · mutant killed · exit 1 · `pkg/discovery/replay_cache.go` · the nonce cache is no longer bounded by its capacity · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8
- 2026-11-15 · 0d3baff* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · the cache forgets a nonce before a future-dated packet goes stale · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8
- 2026-11-15 · 98b9b28* · mutant killed · exit 1 · `pkg/discovery/replay_cache.go` · an entry is forgotten at exactly the TTL, one instant before a future-dated replay is stale · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8
- 2026-11-15 · 98b9b28* · mutant killed · exit 1 · `pkg/discovery/replay_cache.go` · a retyped legacy packet uses up the nonce of the real packet · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8
- 2026-11-15 · 98b9b28* · mutant killed · exit 1 · `pkg/discovery/replay_cache.go` · the FIFO slice grows without bound as entries expire · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8
- 2026-11-15 · 98b9b28* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · dropped replays are no longer counted · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8

## Invariants

- The cache is bounded in memory by the cap, whatever the traffic.
- The nonce is recorded only after a successful open, so a packet that fails the tag cannot poison the cache.
- Datagrams sealed with a fresh nonce (every `SealEnvelope` call) are never dropped.
- HELLO is never dropped by the cache.
- `OpenEnvelopeRaw`, LAN, gossip and registry readers do not use the cache.

## Risks

- A restart empties the cache, so a replay inside the window after a restart is accepted once; the timestamp window still bounds it.
- A replayed HELLO is still answered each time (deferred: docs/adr/BACKLOG.md).
- Test doubles: a fake clock replaces `time.Now` in the cache tests; the real clock is only exercised through `handleMessage`.

## Stop Condition

Stop and ask if the send-site audit in S1 finds a second site that resends identical bytes, or if HELLO is not the only retransmitted type.

## Out of Scope

- A replay cache in the LAN, gossip or registry readers (see the ADR's Out of Scope).
- Stopping a replayed HELLO from being answered (deferred: docs/adr/BACKLOG.md).

## Verification Log
- 2026-11-15 · fc5db46* · exit 1 · `set -o pipefail …` · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8 · ms:1513 · test-lock-sha256:3518175c2dea39904da1a46b021593835ff91c3b5f249e074182bc9a168bc2bc · test-lock-file:docs/adr/locks/3518175c2dea39904da1a46b021593835ff91c3b5f249e074182bc9a168bc2bc.lock
  ```
  --- last 10 line(s) of stderr (of 32 after folding 33 raw)
  2026/11/15 17:33:24 [WARN] Secret does not use wgmesh:// format — it may have low entropy. Use 'wgmesh init --secret' to generate a cryptographically strong secret.  (x2)
  2026/11/15 17:33:24 [Exchange] Received valid ANNOUNCE from 198.51.100.7:51821
  === NAME  TestReplayedAnnounceIsProcessedOnce
      exchange_replay_test.go:26: the same ANNOUNCE datagram reached the handler 2 times, want 1
  --- FAIL: TestReplayedAnnounceIsProcessedOnce (0.00s)
  2026/11/15 17:33:24 [Exchange] Received valid HELLO from 127.0.0.1:48981
  --- PASS: TestHelloRetransmitIsNotDropped (0.00s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/discovery	0.010s
  FAIL
  ```
- 2026-11-15 · 0d3baff* · exit 0 · `set -o pipefail …` · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8 · ms:9469
- 2026-11-15 · 0d3baff* · exit 0 · `set -o pipefail …` · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8 · ms:10506
- 2026-11-15 · 0d3baff* · exit 0 · `set -o pipefail …` · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8 · ms:8124
- 2026-11-15 · 0d3baff* · exit 0 · `set -o pipefail …` · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8 · ms:8969
- 2026-11-15 · 98b9b28* · exit 0 · `set -o pipefail …` · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8 · ms:8695
- 2026-11-15 · 98b9b28* · exit 0 · `set -o pipefail …` · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8 · ms:9774
- 2026-11-15 · 98b9b28* · exit 0 · `set -o pipefail …` · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8 · ms:9282
- 2026-11-15 · 98b9b28* · exit 0 · `set -o pipefail …` · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8 · ms:8269
- 2026-11-15 · 98b9b28* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:632bf2b1ddfc177e708eaaaa2aadcff28f7ee2b305d6ab9e3762a32e823fb3f8 · ms:1 · test-lock-sha256:817de3934fb5cff8d0cabd8fcc2011ab450aa97c7cae6a3d04dab923b80bbe86 · test-lock-file:docs/adr/locks/817de3934fb5cff8d0cabd8fcc2011ab450aa97c7cae6a3d04dab923b80bbe86.lock · test-lock-kind:replace
