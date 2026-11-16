# Decision records

One record per area; a small change amends the record that owns its area instead of adding a new one. The Status column is each record's own Status line. Deferred work from accepted records is kept in [BACKLOG.md](BACKLOG.md).

## Records

| Record | Area | Status | Decision |
|---|---|---|---|
| [ADR-0001](ADR-0001-envelope-aad/ADR-0001-envelope-aad.md) | pkg/crypto envelope, pkg/discovery exchange | Accepted | Authenticate the envelope type with GCM additional data and refuse replays of non-HELLO datagrams, in a three-release wire rollout |
| [ADR-0002](ADR-0002-admit-step/ADR-0002-admit-step.md) | pkg/node peer store, pkg/discovery sources, pkg/daemon collisions | Accepted | Add one stateless admit step (`PeerStore.Offer`) between discovery sources and the PeerStore: tombstones, the local node in collisions, a staged mesh IP and route binding, one endpoint and validation rule set |
| [ADR-0005](ADR-0005-atomic-state/ADR-0005-atomic-state.md) | pkg/atomicfile, pkg/flock, pkg/daemon state files, pkg/rpc socket, pkg/node LocalNode, pkg/discovery exchange | Accepted | Give every piece of shared state one owner: atomic state files, exclusive interface and socket claims, per-caller exchange replies, one LocalNode type under one lock |
| [ADR-0003](ADR-0003-kernel-state-apply/ADR-0003-kernel-state-apply.md) | pkg/routes, pkg/daemon routes and kernel writes | Accepted | Add one observe, diff, apply step between the desired kernel state (peers, routes, forwarding, interface address) and the kernel: one canonical route owner, one desired state per tick, staged enforcement |
| [ADR-0004](ADR-0004-liveness/ADR-0004-liveness.md) | pkg/node peer store, pkg/daemon health and resume, pkg/discovery dht, pkg/wireguard apply | Accepted | Decide that a peer is alive from all the evidence the node has, in one stateless step (`Assess`), with a resume grace and a relay-return probe, staged behind `EnforceLiveness` (N+1) and `EnforceRelayProbe` (N+2) |
| [ADR-0006](ADR-0006-central-deploy/ADR-0006-central-deploy.md) | pkg/mesh deploy, pkg/ssh, pkg/wireguard, pkg/ifname | Accepted | Quote every remote command, validate the state before the first connection, continue past a failed node with a per-node result, and decide each node's endpoint in one pure function |
| [ADR-0007](ADR-0007-bounded-growth/ADR-0007-bounded-growth.md) | pkg/wireguard, pkg/daemon, pkg/discovery maps, main.go | Accepted | Give every long-lived collection and spawned process one bound and one owner: one `wg` snapshot with a deadline, bounded DHT maps, a guard test for every map field |
