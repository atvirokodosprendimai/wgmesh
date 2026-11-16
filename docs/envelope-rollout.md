# Envelope message-type authentication: rollout

Decision record: [ADR-0001](adr/ADR-0001-envelope-aad/ADR-0001-envelope-aad.md). This page is for the operator of a mesh.

The staged rollout of the admit step (mesh IP binding, route rules, `accept-routes`) is described in [Admit step rollout](admit-rollout.md).

## What changes and in which release

| Release | Readers | Writers | What it closes |
|---|---|---|---|
| N (first release with this change) | open both forms | everything except GOODBYE still seals the old form; GOODBYE seals the new form | a GOODBYE re-sent with a changed type can no longer remove a peer; a repeated non-HELLO datagram is processed once |
| N+1 | open both forms | all messages seal the new form | the retyping attack for every message type from upgraded nodes |
| N+2 | refuse the old form | new form | nodes older than N+1 are cut off |

## Release note for N

- **Not everything is closed yet.** The old form does not authenticate the message type. Until N+1 is deployed on every node, a captured HELLO, REPLY or ANNOUNCE can still be re-sent as one of the other two (the effect is a second announcement from the same sender). A GOODBYE that arrives in the old form is ignored.
- **GOODBYE between upgraded nodes works at once.** A node older than N cannot open the new GOODBYE; it logs a packet it cannot decrypt and removes the peer a few minutes later through its health check.
- **A replayed HELLO is still answered.** HELLO is exempt from the replay cache because the punch loop sends one sealed packet repeatedly. The per-source rate limiter bounds it.
- Mixed meshes keep working: nothing else changes on the wire in N.

## What an older node sees when it meets the new form

| Reader | Symptom |
|---|---|
| peer exchange | log line "Received non-wgmesh packet ... possibly DHT or wrong secret" |
| LAN and gossip | the packet is dropped without a log line |
| join (`wgmesh join`) | "Failed to decrypt (wrong secret?)" |

If you see these after upgrading some nodes, upgrade the rest; the secret is not the problem.

## Reading the counters

- `LegacyEnvelopeOpens` counts envelopes received in the old form, per packet and not per sender. At N it counts nearly all traffic (every writer except GOODBYE is old). From N+1 on it counts only traffic from nodes older than N+1, so it is the number to watch before deploying N+2.
- `ReplayedDatagrams` counts datagrams dropped as replays.
- Neither counter is exposed in `wgmesh status` yet. They live on the exchange object, and each legacy open and each dropped replay is also logged, at most once per 30 seconds per kind and source.

## Rollback

N only adds readers and moves GOODBYE; reverting N returns GOODBYE to the old form and loses nothing. Reverting after N+1 would make N+1 writers unreadable by the reverted nodes, which is why N+1 ships separately.
