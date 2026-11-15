# Decision records

One record per area; a small change amends the record that owns its area instead of adding a new one. The Status column is each record's own Status line. Deferred work from accepted records is kept in [BACKLOG.md](BACKLOG.md).

## Records

| Record | Area | Status | Decision |
|---|---|---|---|
| [ADR-0001](ADR-0001-envelope-aad/ADR-0001-envelope-aad.md) | pkg/crypto envelope, pkg/discovery exchange | Accepted | Authenticate the envelope type with GCM additional data and refuse replays of non-HELLO datagrams, in a three-release wire rollout |
| [ADR-0002](ADR-0002-admit-step/ADR-0002-admit-step.md) | pkg/node peer store, pkg/discovery sources, pkg/daemon collisions | Accepted | Add one stateless admit step (`PeerStore.Offer`) between discovery sources and the PeerStore: tombstones, the local node in collisions, a staged mesh IP and route binding, one endpoint and validation rule set |
| [ADR-0005](ADR-0005-atomic-state/ADR-0005-atomic-state.md) | pkg/atomicfile, pkg/flock, pkg/daemon state files, pkg/rpc socket, pkg/node LocalNode, pkg/discovery exchange | Accepted | Give every piece of shared state one owner: atomic state files, exclusive interface and socket claims, per-caller exchange replies, one LocalNode type under one lock |
