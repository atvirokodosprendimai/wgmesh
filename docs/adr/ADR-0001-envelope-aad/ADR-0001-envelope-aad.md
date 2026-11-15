# ADR-0001: Authenticate the envelope type and refuse replays, in a staged wire rollout

**Status:** Proposed
**Date:** 2026-11-15
**Owner:** wgmesh maintainers
**Spec:** None — no spec stage; `eidos/spec - crypto - secret-derived keys envelope encryption membership proofs and rotation.md` is amended by T4
**Cross-references:** `eidos/spec - crypto - secret-derived keys envelope encryption membership proofs and rotation.md`, `eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md`, `testdata/compat/envelope/v1/README.md`, `pkg/crypto/capability_version.go`
**Governs:** `pkg/crypto/envelope.go`, `pkg/crypto/capability_version.go`, `pkg/discovery/exchange.go`, `testdata/compat/envelope/**`
**Enforced-by:** `pkg/crypto/envelope_aad_test.go::TestEnvelopeTypeIsAuthenticated`
**Invalidates:** none — checked (no other decision record exists in this repository; `adr-state` reported none on 2026-11-15)
**Served-path change:** A captured packet re-sent with another message type is dropped by `OpenEnvelopeRaw` instead of acting as that type (for example a HELLO re-sent as GOODBYE removing a peer in `PeerExchange.handleMessage`), and a datagram seen twice inside the freshness window is processed once.

## Context

`SealEnvelope` and `OpenEnvelopeRaw` call `gcm.Seal` and `gcm.Open` with nil additional data (`pkg/crypto/envelope.go:222`, `:281`, read 2026-11-15 at `44ad8f4`), so the plaintext `type` field of the `Envelope` is outside the GCM tag. `PeerExchange.handleMessage` dispatches on that field (`pkg/discovery/exchange.go:219`). The `goodbyeMessage` payload has the same JSON keys as `PeerAnnouncement` (`protocol`, `timestamp`, `wg_pubkey`), so a HELLO captured on the LAN, in gossip or on the DHT path and re-sent with `type` changed to `GOODBYE` opens, passes the 60 s timestamp check and calls `peerStore.Remove` on the named key. No shared secret is needed. Verified 2026-11-15 by a four-reviewer panel plus a synthesis against `44ad8f4` (report and verdicts kept in the maintainers' notes, not in this repository).

Replay is bounded only by the timestamp window: 10 minutes (`MaxMessageAge`, `envelope.go:18`) and 60 s for GOODBYE (`exchange.go`). A replayed HELLO makes `handleHello` send a reply every time, and a REPLY can carry up to 1000 known peers (`MaxKnownPeers`), so replays are also a reflection amplifier (the factor is not measured).

Changing the AAD changes the wire format: old nodes cannot open new packets and new nodes cannot open old ones. A single-release flip partitions every mixed mesh and `exchange.go:213` would log the misleading "possibly DHT or wrong secret". The repository already has a mechanism for wire compatibility: `EnvelopeCapabilityVersion` (`capability_version.go`) and frozen fixtures under `testdata/compat/envelope/v<N>/` that every future build must still open.

## Existing Primitives Audit

- `Envelope`, `SealEnvelope`, `OpenEnvelopeRaw` (`pkg/crypto/envelope.go`): reshape. The struct gains one field and the open path learns two AAD modes. Signatures of `SealEnvelope`, `OpenEnvelope` and `OpenEnvelopeRaw` do not change.
- `EnvelopeCapabilityVersion` and the fixture replay in `pkg/crypto/envelope_compat_test.go`: reuse. This ADR bumps the version and adds a `v2` fixture beside `v1`; `v1` stays frozen and must keep opening.
- `RateLimiter` in `PeerExchange` (`exchange.go:196`): not a replacement. It limits packets per source address; it does not recognise a repeated packet.
- Timestamp checks in `OpenEnvelopeRaw` and in the GOODBYE case: kept. They bound the replay window; the nonce cache closes it inside the window.

## Decision

1. **Bind the type to the tag.** A new envelope field `v` (JSON, integer, omitted when 0) selects the open mode. `v` = 2 means the GCM additional data is the bytes `"wgmesh-v1\x00" + type` (the `ProtocolVersion` constant, a NUL byte, the type string). An envelope without `v` is legacy and opens with nil additional data, as today. A packet sealed with AAD cannot be opened by stripping `v`: the tag then fails under nil AAD. This is why an explicit field is used instead of trying AAD and then nil on every packet.
2. **Legacy envelopes keep opening during the rollout but lose authority.** `OpenEnvelopeRaw` marks them (`Envelope.LegacyAAD`). `PeerExchange.handleMessage` ignores a GOODBYE that opened only as legacy and counts every legacy open, so an operator sees how many old senders remain. Retyping a legacy HELLO into REPLY or ANNOUNCE stays possible during the window; those types carry the same payload and the same validation, so the effect is that of a second announcement from the same sender.
3. **A bounded nonce cache in the exchange path.** After a successful open, `handleMessage` drops a datagram whose 12-byte nonce was seen within `MaxMessageAge`. The cache holds at most 65536 entries and evicts the oldest first. It lives in `pkg/discovery` and not in `OpenEnvelopeRaw`: `registry.go` re-reads the same issue body on purpose and LAN multicast can deliver a datagram twice.
4. **Three releases.** N (this ADR's tasks): readers accept both modes, writers still seal legacy, `EnvelopeCapabilityVersion` becomes 2. N+1: writers seal with AAD (`SealEnvelope` calls `SealEnvelopeAAD`). N+2: readers refuse legacy envelopes. N+1 and N+2 are one-line flips recorded as deferred follow-ups, each with its own release note.

What would make this decision fail: (a) a `v1` fixture that no longer opens (`TestEnvelopeCompatibilityFixtures`), (b) a retyped AAD envelope that opens (`TestEnvelopeTypeIsAuthenticated`), (c) a legacy GOODBYE that removes a peer (T2). Each has a test that exists in this repository, so each can go red today.

## Alternatives Considered

- **Flip to AAD in one release.** Rejected: old and new nodes stop understanding each other, so every mesh that upgrades node by node partitions until the last node is upgraded, and the failure is silent.
- **Bump the payload `protocol` to `wgmesh-v2`.** Rejected: `OpenEnvelopeRaw` rejects an unknown protocol, which is the same hard cut, and it authenticates nothing by itself.
- **Try AAD, then nil, on every packet (no `v` field).** Rejected: it doubles the open cost for every legacy packet and gives no reliable way to say which mode a packet used, which T2 needs.
- **A separate HMAC over the type next to the ciphertext.** Rejected: GCM additional data is the standard mechanism for exactly this and adds no field to verify by hand.
- **A per-sender signature on every message.** Rejected here: it would stop a mesh member who holds the secret from forging messages, but needs per-node signing keys and a way to distribute them. Own decision, not this one.
- **Put the nonce cache in `OpenEnvelopeRaw`.** Rejected: the registry re-read and multicast duplicates are legitimate repeats there.

## Component / Boundary Impact

- `pkg/crypto`: owns the envelope format and the AAD rule; one reason to change (wire format).
- `pkg/discovery`: owns what a received message type is allowed to do (GOODBYE gate) and the replay cache; one reason to change (exchange protocol behaviour).
- `pkg/daemon` does not import `discovery`; nothing in that decoupling changes.

## Wiring & Contract Changes

| Surface | Change | Producer | Consumer(s) |
|---------|--------|----------|-------------|
| Envelope JSON | optional `v` field (2 = AAD-bound) | `SealEnvelopeAAD` | `OpenEnvelopeRaw` on every node |
| `crypto.SealEnvelopeAAD` | new exported function | `pkg/crypto` | tests now, `SealEnvelope` at N+1 |
| `crypto.Envelope.LegacyAAD` | new field, not serialised | `OpenEnvelopeRaw` | `PeerExchange.handleMessage` |
| `crypto.EnvelopeCapabilityVersion` | 1 to 2 | `pkg/crypto` | `envelope_compat_test.go` |
| `testdata/compat/envelope/v2` | new frozen fixture | T1 | `TestEnvelopeCompatibilityFixtures` |
| Exchange behaviour | duplicate datagram dropped, legacy GOODBYE ignored | `pkg/discovery` | peers |

## Inter-task Contracts

| Contract | Producing task | Consuming task(s) | Breaking? |
|----------|----------------|-------------------|-----------|
| `Envelope.LegacyAAD`, `SealEnvelopeAAD` | T1 | T2 | No: additive |
| `EnvelopeCapabilityVersion = 2`, `v2` fixture | T1 | T4 | No |
| `nonceCache` | T3 | T4 | No |

## Implementation

See `tasks/README.md`. Four tasks: T1 AAD in the envelope, T2 legacy GOODBYE gate and counter, T3 nonce cache, T4 spec and operator documentation.

## Consequences

- **Positive:** the retyping attack is closed for AAD senders; a replay is processed once; an operator can see remaining legacy senders.
- **Negative:** the full effect needs three releases; during N and N+1 the retyping of legacy packets other than GOODBYE stays open. The cache is lost on restart, so a replay inside the window after a restart is accepted once.
- **Neutral:** mixed-version meshes keep working until N+2; an old node receiving an AAD packet at N+1 cannot open it and reports it as a non-wgmesh packet.

## Out of Scope

- Per-sender signatures, so a member holding the secret can still forge any message (permanent: boundary: needs a per-node signing key distribution design of its own).
- Changing the winner rule of mesh IP collisions or binding a mesh IP to a public key (deferred: docs/adr/BACKLOG.md).
- Flipping writers to AAD (N+1) and refusing legacy envelopes (N+2) (deferred: docs/adr/BACKLOG.md).
- A replay cache in LAN, gossip and registry readers (permanent: boundary: multicast duplicates and registry re-reads are legitimate repeats there).

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Mixed mesh partitions at N+1 because some nodes still run N-1 | Med | High | N readers already accept both; N+1 is a separate release with a note, and the legacy counter shows who is left |
| Cache eviction under a flood lets a replay through | Med | Low | the timestamp window still applies; the cap bounds memory |
| Legacy retyping of non-GOODBYE types during N and N+1 | Med | Low | same payload and validation as a normal announcement; closed at N+2 |
| `v` field read by an old node as unknown JSON | Low | Low | Go's decoder ignores unknown fields, covered by the v2 fixture test |

## Rollback

N is additive: readers accept both modes and writers still seal legacy, so reverting the release returns to today's behaviour with no state to undo. `v2` fixtures stay in the repository (frozen fixtures are never deleted). A revert after N+1 would make N+1 writers unreadable by reverted nodes, which is why N+1 ships separately.

## Follow-ups

- [ ] N+1: switch `SealEnvelope` to AAD, with a release note (deferred: docs/adr/BACKLOG.md)
- [ ] N+2: refuse legacy envelopes (deferred: docs/adr/BACKLOG.md)
