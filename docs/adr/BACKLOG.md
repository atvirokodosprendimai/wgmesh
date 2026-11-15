# ADR backlog

Deferred work from accepted decisions, so `adr-debt` can resurface it.

- ADR-0001 follow-up N+1: switch `SealEnvelope` to the AAD-bound form (`SealEnvelopeAAD`) in the release after the one that ships ADR-0001 T1 to T4. Needs a release note: nodes older than N cannot open the new packets.
- ADR-0001 follow-up N+2: refuse legacy (nil-AAD) envelopes in `OpenEnvelopeRaw`, one release after N+1. `EnvelopeCapabilityVersion` becomes 3 and `testdata/compat/envelope/v1` changes from "must open" to "must be refused with the legacy error"; the fixture files stay byte-identical.
- ADR-0001 deferred: change the mesh IP collision winner rule and bind a mesh IP to its public key (members can still claim other members' mesh IPs; see issue #864).
- ADR-0001 deferred: a replay cache in the gossip reader (gossip is unicast, `pkg/discovery/gossip.go:222`) and the legacy counter in `daemon.status`.
- ADR-0001 deferred: stop a replayed HELLO from being answered every time (HELLO is exempt from the nonce cache because the retry loop at `pkg/discovery/exchange.go:527` resends identical bytes; a fix needs a per-HELLO sequence or token).
