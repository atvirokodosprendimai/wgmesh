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
| `pkg/discovery/exchange.go` | edit | `validateExchangeAnnouncement` is replaced by the crypto function; `logIncomingPacket` (`:240`) moves behind validation in the HELLO, REPLY and ANNOUNCE cases |
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
| 3 — the caller can discover it | doc comment; T10 amends the exchange spec |
| 4 — it is used | one invalid entry no longer hides a whole LAN or gossip announcement |

## Mutation Log

## Invariants

- The sender's own fields are validated strictly on every path.
- ADR-0001's seal and open behaviour, the `v` field and the nonce cache are unchanged; its fixtures still open.
- The `MaxKnownPeers` limit still rejects the message.

## Risks

- Readers become more lenient: an older node that stored an invalid entry before validation existed now no longer hides itself on LAN and gossip. That is the intended effect.
- Test doubles: none; the LAN test uses the real listener the existing LAN tests use.

## Stop Condition

Stop and ask if `OpenEnvelope` has a caller that relies on the whole-message rejection, found with `grep -rn "OpenEnvelope(" --include=*.go . | grep -v _test.go` (5 sites on 2026-11-15, 4 outside `pkg/crypto`).

## Out of Scope

- `install-service` validating `--advertise-routes` (item 1 of #875) (deferred: docs/adr/BACKLOG.md).
- Any change to the envelope bytes or the AAD rules (permanent: boundary: ADR-0001 owns them).

## Verification Log
