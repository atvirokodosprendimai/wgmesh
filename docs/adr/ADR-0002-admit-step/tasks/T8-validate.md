# Task ADR-0002-T8: One validation path for announcements on LAN, gossip, registry and exchange (#875)

**Depends-on:** T2
**Covers:** none — no spec
**Estimated scope:** M (multi-file)
**Owner:** unassigned
**Produces:** `crypto.FilterAnnouncement`
**Consumes:** every production claim goes through `Offer` (T2)
**Data dependency:** hermetic
**Proof map:** v1

## Goal

An announcement with a valid sender and one invalid `KnownPeers` entry is accepted with that entry dropped on every reader, not only on the exchange path, and the "Received valid" log line follows validation.

## Affected Files

| File | Change | Why |
|------|--------|-----|
| `pkg/crypto/envelope.go` | edit | `FilterAnnouncement(a *PeerAnnouncement) error` (the body of `validateExchangeAnnouncement`, `exchange.go:365-382`); `OpenEnvelope` (`:288`) calls it instead of `Validate`; the AAD and nonce code of ADR-0001 is not touched |
| `pkg/discovery/exchange.go` | edit | `validateExchangeAnnouncement` stays as a one-line delegate to the crypto function (a locked test, `TestValidateExchangeAnnouncementKnownPeersLimit`, calls it); `logIncomingPacket` (`:240`) moves behind validation in the HELLO, REPLY and ANNOUNCE cases |
| `pkg/crypto/envelope_test.go`, `pkg/discovery/exchange_log_test.go`, `pkg/discovery/lan_test.go` | edit | tests below |

Selection: LAN, gossip, registry and `main.go` reach it through `OpenEnvelope`, exchange through `FilterAnnouncement` directly; deleting the call in `OpenEnvelope` turns `TestOpenEnvelopeDropsInvalidKnownPeerKeepsMessage` red.

## Ordered Steps

1. [S1] Add the four tests below; run red (the first reproduces item 2 of #875: today `OpenEnvelope` rejects the whole message).
2. [S2] Move the filter into `pkg/crypto`; keep the error for a too-long `KnownPeers` list and for an invalid sender.
3. [S3] Make `OpenEnvelope` use it; make the exchange handlers call the same function.
4. [S4] Move `logIncomingPacket` after validation; the legacy-GOODBYE exception at `:239` stays.
5. [S5] Run `go test ./pkg/crypto -run TestEnvelopeCompatibilityFixtures -count=1`; ADR-0001's `v1` and `v2` fixtures must still open. Record the exit line in the commit message.
6. [S6] Run the mutants with `adr-verify --mutant`: `OpenEnvelope` goes back to `Validate`; the filter keeps an invalid entry; the log line moves back before validation. [proof: mutation]

## Acceptance

```bash
set -o pipefail
a=$(go test ./pkg/crypto -run '^(TestOpenEnvelopeDropsInvalidKnownPeerKeepsMessage|TestOpenEnvelopeRejectsInvalidSender|TestEnvelopeCompatibilityFixtures)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$a" -ge 3 ] \
 && b=$(go test ./pkg/discovery -run '^(TestValidLogFollowsValidation|TestLANDropsInvalidKnownPeerKeepsSender)$' -count=1 -v 2>&1 | tee /dev/stderr | grep -c '^--- PASS') && [ "$b" -ge 2 ] \
 && go test -race ./pkg/crypto ./pkg/discovery -count=1
```

## Tests

| Test name | File | Verifies | Covers | Steps |
|-----------|------|----------|--------|-------|
| `TestOpenEnvelopeDropsInvalidKnownPeerKeepsMessage` | `pkg/crypto/envelope_test.go` | a sealed announcement with one invalid known peer opens, the sender is intact and the entry is gone | — | S1, S2, S3 |
| `TestOpenEnvelopeRejectsInvalidSender` | `pkg/crypto/envelope_test.go` | an invalid sender mesh IP or key still fails to open | — | S1, S3 |
| `TestEnvelopeCompatibilityFixtures` | `pkg/crypto/envelope_compat_test.go` | the frozen `v1` and `v2` fixtures still open (exists today; this task must not break it) | — | S5 |
| `TestLANDropsInvalidKnownPeerKeepsSender` | `pkg/discovery/lan_test.go` | a multicast announcement with one invalid known peer stores the sender | — | S1, S3 |
| `TestValidLogFollowsValidation` | `pkg/discovery/exchange_log_test.go` | an invalid HELLO produces no "Received valid HELLO" line, a valid one produces exactly one | — | S1, S4 |

## Reachability

| Rung | How this task shows it |
|------|------------------------|
| 1 — exists | the five tests |
| 2 — something selects it | `OpenEnvelope` and the exchange handlers call it; reverting either turns a named test red |
| 3 — the caller can discover it | doc comment; T11 amends the exchange spec |
| 4 — it is used | one invalid entry no longer hides a whole LAN or gossip announcement |

## Mutation Log
- 2026-11-15 · 03b7113* · mutant killed · exit 1 · `pkg/crypto/envelope.go` · OpenEnvelope goes back to Validate: TestOpenEnvelopeDropsInvalidKnownPeerKeepsMessage must go red · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396
- 2026-11-15 · 03b7113* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · the valid log line moves back before validation: TestValidLogFollowsValidation must go red · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396
- 2026-11-15 · 03b7113* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · the exchange handlers stop using the shared filter · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396
- 2026-11-15 · 03b7113* · mutant killed · exit 1 · `pkg/crypto/envelope.go` · the filter keeps an invalid known peer (no continue): TestOpenEnvelopeDropsInvalidKnownPeerKeepsMessage must go red · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396
- 2026-11-15 · bf9a229* · mutant killed · exit 1 · `pkg/crypto/envelope.go` · r2: OpenEnvelope goes back to Validate · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396
- 2026-11-15 · bf9a229* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · r2: the valid log line moves back before validation · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396
- 2026-11-15 · bf9a229* · mutant killed · exit 1 · `pkg/discovery/exchange.go` · r2: exchange handlers stop using the shared filter · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396
- 2026-11-15 · bf9a229* · mutant killed · exit 1 · `pkg/crypto/envelope.go` · r2: exactly MaxKnownPeers is rejected · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396
- 2026-11-15 · bf9a229* · mutant killed · exit 1 · `pkg/crypto/envelope.go` · r2: a failed call empties the caller's KnownPeers · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396
- 2026-11-15 · bf9a229* · mutant killed · exit 1 · `pkg/crypto/envelope.go` · r2: no warning per message that dropped entries · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396
- 2026-11-15 · bf9a229* · mutant killed · exit 1 · `pkg/crypto/envelope.go` · r2: the filter keeps an invalid known peer (no continue) · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396

## Invariants

- The sender's own fields are validated strictly on every path.
- ADR-0001's seal and open behaviour, the `v` field and the nonce cache are unchanged; its fixtures still open.
- The `MaxKnownPeers` limit still rejects the message.

## Risks

- Readers become more lenient: an older node that stored an invalid entry before validation existed now no longer hides itself on LAN and gossip. That is the intended effect.
- Receiver-only change: a node before this release still rejects the whole message, so a sender gains nothing until its readers upgrade (T11 release note). A replayed REPLY or ANNOUNCE is no longer logged "Received valid" (the line follows the replay check and validation). `RENDEZVOUS_*`, `GOODBYE` and unknown types still log before their payload is parsed (out of scope). The registry reader no longer logs a failure for a bad `KnownPeers` entry; one warning per message that dropped entries is logged by `FilterAnnouncement`.
- Test doubles: none; the LAN test uses the real listener the existing LAN tests use.

## Stop Condition

Stop and ask if `OpenEnvelope` has a caller that relies on the whole-message rejection, found with `grep -rn "OpenEnvelope(" --include=*.go . | grep -v _test.go` (5 sites on 2026-11-15, 4 outside `pkg/crypto`).

## Out of Scope

- `install-service` validating `--advertise-routes` (item 1 of #875) (deferred: docs/adr/BACKLOG.md).
- Any change to the envelope bytes or the AAD rules (permanent: boundary: ADR-0001 owns them).

## Verification Log
- 2026-11-15 · 03b7113* · exit 1 · `set -o pipefail …` · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396 · ms:1080 · test-lock-sha256:cbd6bff9da1466f572795ba4c70b8efd432af0d0baf57703e4e7c7e88fe87580 · test-lock-file:docs/adr/locks/cbd6bff9da1466f572795ba4c70b8efd432af0d0baf57703e4e7c7e88fe87580.lock
  ```
  --- last 10 line(s) of stderr (of 22 after folding 22 raw)
  === RUN   TestOpenEnvelopeRejectsInvalidSender/bad_mesh_ip
  === RUN   TestOpenEnvelopeRejectsInvalidSender/bad_key
  === RUN   TestOpenEnvelopeRejectsInvalidSender/too_many_known_peers
  --- PASS: TestOpenEnvelopeRejectsInvalidSender (0.01s)
      --- PASS: TestOpenEnvelopeRejectsInvalidSender/bad_mesh_ip (0.00s)
      --- PASS: TestOpenEnvelopeRejectsInvalidSender/bad_key (0.00s)
      --- PASS: TestOpenEnvelopeRejectsInvalidSender/too_many_known_peers (0.01s)
  FAIL
  FAIL	github.com/atvirokodosprendimai/wgmesh/pkg/crypto	0.018s
  FAIL
  ```
- 2026-11-15 · 03b7113* · exit 0 · `set -o pipefail …` · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396 · ms:18392
- 2026-11-15 · 03b7113* · exit 0 · `set -o pipefail …` · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396 · ms:20277
- 2026-11-15 · 03b7113* · exit 0 · `set -o pipefail …` · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396 · ms:20622
- 2026-11-15 · 03b7113* · exit 0 · `set -o pipefail …` · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396 · ms:17808
- 2026-11-15 · 03b7113* · exit 0 · `set -o pipefail …` · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396 · ms:22316
- 2026-11-15 · bf9a229* · exit 0 · `adr-verify --relock --replace-hashes` · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396 · ms:1 · test-lock-sha256:1aceffcce18d212be666939d45ca67f2332a49ebf1dc65529e76e16512f6077d · test-lock-file:docs/adr/locks/1aceffcce18d212be666939d45ca67f2332a49ebf1dc65529e76e16512f6077d.lock · test-lock-kind:replace
- 2026-11-15 · bf9a229* · exit 0 · `set -o pipefail …` · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396 · ms:17975
- 2026-11-15 · bf9a229* · exit 0 · `set -o pipefail …` · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396 · ms:25605
- 2026-11-15 · bf9a229* · exit 0 · `set -o pipefail …` · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396 · ms:14970
- 2026-11-15 · bf9a229* · exit 0 · `set -o pipefail …` · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396 · ms:14682
- 2026-11-15 · bf9a229* · exit 0 · `set -o pipefail …` · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396 · ms:17284
- 2026-11-15 · bf9a229* · exit 0 · `set -o pipefail …` · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396 · ms:19076
- 2026-11-15 · bf9a229* · exit 0 · `set -o pipefail …` · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396 · ms:16545
- 2026-11-15 · bf9a229* · exit 0 · `set -o pipefail …` · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396 · ms:16947
- 2026-11-16 · 51e018f* · exit 0 · `set -o pipefail …` · acceptance-sha256:0bc8bdb6b78549e183b5ff2f9ebe31ffb413c5c117ebab64ddf10f88f38d3396 · ms:9370
