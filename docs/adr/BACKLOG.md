# ADR backlog

Deferred work from accepted decisions, so `adr-debt` can resurface it.

- ADR-0001 follow-up N+1: switch `SealEnvelope` to the AAD-bound form (`SealEnvelopeAAD`) in the release after the one that ships ADR-0001 T1 to T4. Needs a release note: nodes older than N cannot open the new packets.
- ADR-0001 follow-up N+2: refuse legacy (nil-AAD) envelopes in `OpenEnvelopeRaw`, one release after N+1.
- ADR-0001 deferred: change the mesh IP collision winner rule and bind a mesh IP to its public key (members can still claim other members' mesh IPs; see issue #864).
