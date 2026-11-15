# ADR-0001: Authenticate the envelope type and refuse replays, in a staged wire rollout

**Status:** Accepted
**Date:** 2026-11-15
**Owner:** wgmesh maintainers
**Spec:** None — no spec stage; `eidos/spec - crypto - secret-derived keys envelope encryption membership proofs and rotation.md` and `eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md` are amended by T4
**Cross-references:** `eidos/spec - crypto - secret-derived keys envelope encryption membership proofs and rotation.md`, `eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md`, `testdata/compat/envelope/v1/README.md`, `pkg/crypto/capability_version.go`; depends on PR #876 (commit `5942a78`, `validateExchangeAnnouncement` in `pkg/discovery/exchange.go`), which this branch is built on
**Governs:** `pkg/crypto/envelope.go`, `pkg/crypto/capability_version.go`, `pkg/discovery/exchange.go`, `testdata/compat/envelope/**`
**Enforced-by:** `pkg/crypto/envelope_aad_test.go::TestEnvelopeTypeIsAuthenticated`
**Invalidates:** none — checked (no other decision record exists in this repository; `adr-state` reported none on 2026-11-15)
**Served-path change:** A captured packet re-sent with another message type no longer acts as that type (for example a HELLO re-sent as GOODBYE no longer removes a peer in `PeerExchange.handleMessage`), and a non-HELLO datagram seen twice inside the acceptance window is processed once.

## Context

`SealEnvelope` and `OpenEnvelopeRaw` call `gcm.Seal` and `gcm.Open` with nil additional data (`pkg/crypto/envelope.go:222`, `:281`, read 2026-11-15 at `44ad8f4`), so the plaintext `type` field of the `Envelope` is outside the GCM tag. `PeerExchange.handleMessage` dispatches on that field (`pkg/discovery/exchange.go:219`). The `goodbyeMessage` payload has the same JSON keys as `PeerAnnouncement` (`protocol`, `timestamp`, `wg_pubkey`), so a HELLO captured on the LAN, in gossip or on the DHT path and re-sent with `type` changed to `GOODBYE` opens, passes the 60 s timestamp check and calls `peerStore.Remove` on the named key. No shared secret is needed. Verified 2026-11-15 by a four-reviewer panel plus a synthesis against `44ad8f4`; this record was then reviewed cold by three Opus reviewers and a synthesis, whose eight blocking findings are folded in below.

The reverse direction also exists: a GOODBYE captured and re-sent as HELLO reaches `handleHello`, which, before PR #876, had no validation of the payload and could point the named peer's endpoint at the sender's address (`resolvePeerEndpoint`, `exchange.go:977`). PR #876 validates HELLO, REPLY and ANNOUNCE payloads (`validateExchangeAnnouncement`; a GOODBYE has no `mesh_ip`, so it fails `PeerAnnouncement.Validate`). This record depends on that.

Replay is bounded only by the timestamp window: 10 minutes either side (`MaxMessageAge`, `envelope.go:303-307`) and 60 s for GOODBYE. A replayed HELLO makes `handleHello` send a reply each time, and a REPLY can carry up to 1000 known peers (`MaxKnownPeers`), so replays are also a reflection amplifier (the factor is not measured).

One sender writes the same sealed bytes more than once: the HELLO retry loop in `ExchangeWithPeer` (the `sendHello` closure in `ExchangeWithPeer`: one `SealEnvelope` before the loop, a write every `PunchInterval` until `ExchangeTimeout`). Class audit, 2026-11-15: `grep -n "WriteToUDP" pkg/discovery/exchange.go` lists 6 write sites; only the `sendHello` closure in `ExchangeWithPeer` writes inside a retry loop. Every other write follows its own `SealEnvelope` call. Senders older than this change do the same.

Changing the AAD changes the wire format: old nodes cannot open new packets and new nodes cannot open old ones. A single-release flip partitions every mixed mesh. The repository already has a mechanism for wire compatibility: `EnvelopeCapabilityVersion` (`capability_version.go`) and frozen fixtures under `testdata/compat/envelope/v<N>/` that the replay test opens with current code.

## Existing Primitives Audit

- `Envelope`, `SealEnvelope`, `OpenEnvelopeRaw` (`pkg/crypto/envelope.go`): reshape. The struct gains fields and the open path learns two AAD modes. Signatures of `SealEnvelope`, `OpenEnvelope` and `OpenEnvelopeRaw` do not change.
- `EnvelopeCapabilityVersion` and the fixture replay in `pkg/crypto/envelope_compat_test.go`: reuse. This ADR bumps the version and adds a `v2` fixture beside `v1`; the replay test is tightened so each fixture version asserts its own mode.
- `validateExchangeAnnouncement` (PR #876): reuse; it is what rejects a retyped GOODBYE in the HELLO, REPLY and ANNOUNCE cases.
- `RateLimiter` in `PeerExchange` (`exchange.go:196`) and `logIncomingPacket` with `ExchangeLogCooldown` (`exchange.go:1141`): reuse for per-source limiting and log throttling. Neither recognises a repeated packet.
- Timestamp checks in `OpenEnvelopeRaw` and in the GOODBYE case: kept. They bound the window; the nonce cache closes it inside the window for the types that are never retransmitted.

## Decision

1. **Bind the type to the tag.** A new envelope field `v` (JSON, integer, omitted when 0) selects the open mode. `v` = 2 means the GCM additional data is the bytes `"wgmesh-v1\x00" + type` (the `ProtocolVersion` constant, a NUL byte, the type string). An envelope without `v` is legacy and opens with nil additional data, as today; any other `v` is refused. A packet sealed with AAD cannot be opened by stripping `v`: the tag then fails under nil AAD. This is why an explicit field is used instead of trying AAD and then nil on every packet. Package variables `sealWithAAD` and `acceptLegacyAAD` are the two switches the later releases flip; they are also the seams the tests use.
2. **Legacy envelopes keep opening during the rollout but lose the right to remove a peer.** `OpenEnvelopeRaw` marks them (`Envelope.LegacyAAD`). `PeerExchange.handleMessage` ignores a GOODBYE that opened only as legacy. So that graceful shutdown keeps working between upgraded nodes, `SendGoodbye` seals with `SealEnvelopeAAD` already at release N; it is the only writer that changes at N. A node older than N cannot open that GOODBYE and evicts the peer through its health check a few minutes later. Other legacy retyping during N and N+1: a legacy GOODBYE retyped as HELLO, REPLY or ANNOUNCE fails `Validate` (no `mesh_ip`) and is dropped; a legacy HELLO, REPLY or ANNOUNCE retyped as another of those three has the effect of a second announcement from the same sender; retyping into a `RENDEZVOUS_*` type fails to parse the payload.
3. **A bounded nonce cache for every type except HELLO.** In `handleMessage` the order is: open the envelope, count legacy opens, check the nonce cache, dispatch; the legacy GOODBYE gate sits inside the GOODBYE case of the dispatch. A nonce is recorded only after the envelope opened successfully. A datagram whose 12-byte nonce and message type are already cached is dropped and counted (`ReplayedDatagrams`); the type is part of the key because a legacy envelope does not authenticate it, and a retyped copy of a captured packet must not use up the nonce of the real one. The cache holds at most 65536 entries, evicts the oldest first, and keeps an entry for 2 × `MaxMessageAge`, because `OpenEnvelopeRaw` accepts timestamps up to `MaxMessageAge` in the future and a replay must stay refused until the packet is stale on the other side too. HELLO is exempt because the HELLO retry loop deliberately sends identical bytes (see Context), and so do senders from before this change; a replay cache that drops them would stop the NAT punch from recovering a lost reply. A replayed HELLO is therefore still answered each time; it is bounded by the per-source rate limiter and, for AAD senders, can no longer be retyped.
4. **Three releases.** N (this ADR's tasks): readers accept both modes, `SendGoodbye` seals with AAD, all other writers still seal legacy, `EnvelopeCapabilityVersion` becomes 2. N+1: writers seal with AAD (`SealEnvelope` calls `SealEnvelopeAAD`). N+2: readers refuse legacy envelopes, `EnvelopeCapabilityVersion` becomes 3, and the frozen `v1` fixture changes from "must open" to "must be refused with the legacy error" (the fixture file stays byte-identical). N+1 and N+2 are one-line flips recorded as deferred follow-ups, each with its own release note.
5. **The legacy counter is a measurement for N+1, not for N.** `handleMessage` counts legacy opens. At N every writer except `SendGoodbye` is legacy, so the counter then counts nearly all traffic; from N+1 on it counts only senders older than N+1. LAN, gossip, registry and join readers do not see the marker and are not counted.

What would make this decision fail: (a) a `v1` fixture that stops opening before N+2, caught by `TestEnvelopeCompatibilityFixtures` (exists today); (b) a retyped AAD envelope that opens, caught by `TestEnvelopeTypeIsAuthenticated` (added red-first by T1); (c) a legacy GOODBYE that removes a peer, or a GOODBYE between upgraded nodes that does not, caught by T2's tests (added red-first); (d) a HELLO retransmit dropped by the cache, caught by T3. Only (a) can go red today; the others exist once their task's first step has run, against compiling stubs so the first red run fails on assertions.

## Alternatives Considered

- **Flip to AAD in one release.** Rejected: old and new nodes stop understanding each other, so every mesh that upgrades node by node partitions until the last node is upgraded, and the failure is silent.
- **Bump the payload `protocol` to `wgmesh-v2`.** Rejected: `OpenEnvelopeRaw` rejects an unknown protocol, which is the same hard cut, and it authenticates nothing by itself.
- **Try AAD, then nil, on every packet (no `v` field).** Rejected: it doubles the open cost for every legacy packet and gives no reliable way to say which mode a packet used, which the GOODBYE gate needs.
- **A separate HMAC over the type next to the ciphertext.** Rejected: GCM additional data is the standard mechanism for exactly this and adds no field to verify by hand.
- **A per-sender signature on every message.** Rejected here: it would stop a mesh member who holds the secret from forging messages, but needs per-node signing keys and a way to distribute them. Own decision, not this one.
- **Put the nonce cache in `OpenEnvelopeRaw`.** Rejected: the registry re-reads the same issue body on purpose and multicast can deliver a datagram twice, so repeats there are legitimate.
- **Cache HELLO keyed by (nonce, source address).** Rejected: the retransmits come from the same source address, so the key does not tell a retransmit from a replay.

## Component / Boundary Impact

- `pkg/crypto`: owns the envelope format and the AAD rule; one reason to change (wire format).
- `pkg/discovery`: owns what a received message type is allowed to do (GOODBYE gate, `SendGoodbye`) and the replay cache; one reason to change (exchange protocol behaviour).
- `pkg/daemon` does not import `discovery`; nothing in that decoupling changes.

## Wiring & Contract Changes

| Surface | Change | Producer | Consumer(s) |
|---------|--------|----------|-------------|
| Envelope JSON | optional `v` field (2 = AAD-bound) | `SealEnvelopeAAD` | `OpenEnvelope` and `OpenEnvelopeRaw` on every node |
| `crypto.SealEnvelopeAAD` | new exported function | `pkg/crypto` | `SendGoodbye` now, `SealEnvelope` at N+1 |
| `crypto.Envelope.LegacyAAD` | new field, not serialised | `OpenEnvelopeRaw` | `PeerExchange.handleMessage` |
| `crypto.EnvelopeCapabilityVersion` | 1 to 2 | `pkg/crypto` | `envelope_compat_test.go` |
| `testdata/compat/envelope/v2` | new frozen fixture | T1 | `TestEnvelopeCompatibilityFixtures` |
| GOODBYE on the wire | sealed with AAD at N | `SendGoodbye` | nodes at N and later; nodes older than N cannot open it |
| Exchange behaviour | duplicate non-HELLO datagram dropped, legacy GOODBYE ignored | `pkg/discovery` | peers |

## Inter-task Contracts

| Contract | Producing task | Consuming task(s) | Breaking? |
|----------|----------------|-------------------|-----------|
| `Envelope.LegacyAAD`, `SealEnvelopeAAD` | T1 | T2 | No: additive |
| `handleMessage` order (open, gate, cache, dispatch) | T2 | T3 | No |
| `EnvelopeCapabilityVersion = 2`, `v2` fixture | T1 | T4 | No |
| `nonceCache` | T3 | T4 | No |

## Implementation

See `tasks/README.md`. Four tasks: T1 AAD in the envelope, T2 legacy GOODBYE gate, AAD `SendGoodbye` and counter, T3 nonce cache, T4 spec and operator documentation.

## Consequences

- **Positive:** the retyping attack is closed for AAD senders; a non-HELLO replay is processed once; GOODBYE between upgraded nodes works at once.
- **Negative:** the full effect needs three releases; during N and N+1 retyping among HELLO, REPLY and ANNOUNCE of legacy packets stays possible. A replayed HELLO is still answered. A node older than N does not process GOODBYE from nodes at N. The cache is lost on restart, so a replay inside the window after a restart is accepted once.
- **Neutral:** the type is authenticated but the channel is not: ANNOUNCE has the same additional data on LAN, gossip, registry and exchange, so a packet captured on one channel is valid on another until the cache or the window stops it. Mixed-version meshes keep working until N+2. An old node receiving an AAD packet at N+1 cannot open it; the symptom differs per reader: `exchange.go:213` logs "possibly DHT or wrong secret", LAN and gossip drop silently, and `main.go` prints "wrong secret?". T4 lists them.

## Out of Scope

- Per-sender signatures, so a member holding the secret can still forge any message (permanent: boundary: needs a per-node signing key distribution design of its own).
- Changing the winner rule of mesh IP collisions or binding a mesh IP to a public key (deferred: docs/adr/BACKLOG.md).
- Flipping writers to AAD (N+1) and refusing legacy envelopes (N+2) (deferred: docs/adr/BACKLOG.md).
- A replay cache in the gossip reader and exposing the legacy counter in `daemon.status` (deferred: docs/adr/BACKLOG.md).
- A replay cache in the LAN and registry readers (permanent: boundary: multicast duplicates and registry re-reads are legitimate repeats there).
- Stopping a replayed HELLO from being answered (deferred: docs/adr/BACKLOG.md).

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Mixed mesh partitions at N+1 because some nodes still run N-1 | Med | High | N readers already accept both; N+1 is a separate release with a note; the legacy counter is meaningful from N+1 |
| Cache eviction flood lets a replay through | Low | Low | needs valid captured datagrams from the mesh; the timestamp window still applies; the cap bounds memory |
| A sender is found that retransmits non-HELLO bytes | Low | Med | T3 audits the send sites with a command and has a stop condition |
| Legacy retyping among HELLO, REPLY and ANNOUNCE during N and N+1 | Med | Low | same payload and same validation as a normal announcement; closed at N+2 |
| An old node ignores the `v` field and then fails the GCM tag | High | Low | expected and logged; nodes older than N do not process GOODBYE from N, and evict by health check |

## Rollback

N is additive on the read side: readers accept both modes. The one write change is `SendGoodbye`; a revert returns it to legacy and nodes older than N simply start seeing GOODBYE again. No state to undo. `v2` fixtures stay in the repository (frozen fixtures are never deleted). A revert after N+1 would make N+1 writers unreadable by reverted nodes, which is why N+1 ships separately.

## Follow-ups

- [ ] N+1: switch `SealEnvelope` to AAD, with a release note (deferred: docs/adr/BACKLOG.md)
- [ ] N+2: refuse legacy envelopes, capability version 3, `v1` fixture becomes an expected refusal (deferred: docs/adr/BACKLOG.md)
