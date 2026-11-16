# Liveness: rollout

Decision record: [ADR-0004](adr/ADR-0004-liveness/ADR-0004-liveness.md). This page is for the operator of a mesh and is the release note for the three releases that bring it in. Two of the three are not done yet: they are rows in [the backlog](adr/BACKLOG.md), not tasks.

A node used to call a peer alive only when it had heard an announcement from it (`LastSeen`). A peer that WireGuard kept in contact, a peer restored from the cache, and every peer after the laptop woke from sleep could look dead for that reason alone. From N the node also records what it measures itself (WireGuard handshakes, successful mesh probes), notes the wake from sleep, and judges a peer by the newest of these times (`node.Assess`: younger than 5 minutes `Alive`, up to 10 minutes `Suspect`, older `Dead`). Nothing on the wire changes, so old and new nodes talk to each other during the whole rollout.

## What changes and in which release

| Release | What the node does | Switches |
|---|---|---|
| N (the first with this change) | records the evidence, detects wakes and acts on them, counts every case the verdict would change (`wgmesh_liveness_disagree_total{site}`), exports `wgmesh_relay_route_age_seconds`; the legacy `LastSeen` ages decide the active set, `IsDead` and the stale cleanup, while the mesh probe loop already uses the evidence (see the note for N) | `node.EnforceLiveness = false`, `daemon.EnforceRelayProbe = false` |
| N+1 | the verdict acts | `node.EnforceLiveness` true |
| N+2 | a relayed peer stays configured probe-only | `daemon.EnforceRelayProbe` true |

Both switches are package variables (`atomic.Bool`), `false` in release N. There is no config flag and no runtime key: flipping one is a code change (`Store(true)` at start-up) and a new build. Rollback is the previous binary, or a build with `Store(false)`. Nothing in production sets them.

## Release note for N

What acts at N, whatever the switches:

- **Evidence is written.** Every non-zero WireGuard handshake that reconcile reads, and every successful mesh probe, is recorded as the peer's `LastDataPlane` (`PeerStore.ObserveDataPlane`). It is a local measurement, never a claim from another node: it never moves backwards, never touches `LastSeen`, and is never announced.
- **The mesh probe loop visits Suspect peers too**, while they are configured on the WireGuard interface or relay-routed (`GetProbeable`: Alive and Suspect peers). A pong makes a Suspect peer Alive again once the verdict is enforced.
- **The mesh probe loop acts on the evidence from N, switch or not.** `GetProbeable` always judges by `Assess`, which includes `RestoredAt`, so a restored peer is probed for the restore grace (`PeerDeadTimeout` after the restore) although `GetActive` (legacy) keeps it off the interface. After `MeshProbeFailLimit` (8) failed probes the existing probe eviction (`Withdraw`, a 5-minute tombstone) removes it from the store, and so from the next cache save; before this release the stale cleanup removed a restored old peer anyway.
- **A wake is detected and acted on.** The resume loop reads the wall clock every 2 seconds; a gap above 10 seconds is a wake. You see the `[Resume]` log line and `wgmesh_resume_detected_total`; the node then makes one extra reconcile, refreshes its endpoint (the `stun-resume` source, applied at once) and announces to the DHT infohash and to every active peer. The mechanism is written in the specs, not here: [daemon support, wake from sleep](../eidos/spec%20-%20daemon%20support%20-%20cache%20persistence%20collision%20resolution%20epoch%20management%20route%20sync%20and%20systemd.md#wake-from-sleep-adr-0004) and [DHT core, `OnResume`](../eidos/spec%20-%20discovery%20dht%20core%20-%20bittorent%20mainline%20dht%20announce%20query%20persistence%20and%20endpoint%20detection.md#wake-from-sleep-dhtdiscoveryonresume-adr-0004). The announce burst costs O(N^2) data (N packets of size O(N)); an announce over about 260 peers can exceed the UDP limit, as the steady gossip loop does. This is an estimate, not measured.
- **The peer cache writes the newest measured contact.** The `last_seen` field of the cache file is now `LastContact`: the newer of `LastSeen` and `LastDataPlane`, never the restore time or the wake grace. The field and its type are unchanged, so an older version still reads the file, but it reads `last_seen` as `LastSeen`: after a rollback a peer kept only by handshakes looks announced-fresh to it. The log line on restore says how many restored peers are alive by evidence and how many only suspected. `RestoredAt` is recorded for a restored peer from N, but the active set only follows it once the verdict is enforced (the probe loop already does, above).
- **The health loop and the reconnect skip relay-routed peers.** A relay-routed peer has no handshake of its own; judging it by handshake age would evict a peer the relay serves, or give it its mesh IP back and take the address from the relay. This holds whatever `EnforceRelayProbe` is. A dead relayed peer is still removed by the mesh probe (relay-routed peers are always enforced; `MeshProbeFailLimit`, 8 failures) and by the stale cleanup at 10 minutes.
- **`wgmesh_relay_route_age_seconds{peer}`** is exported from N: seconds since this daemon first saw the peer relayed, 0 again after a restart. `peer` is the first 8 characters of the public key. Read it before N+2 to see how long peers stay relayed.

What a restored peer looks like in `daemon.status` and `peers.list`: its `last_seen` is the time in the cache file (the newest contact before the save). `RestoredAt`, `LastDataPlane` and the wake time are not shown there.

## Reading `wgmesh_liveness_disagree_total{site}` as the N+1 go/no-go

The counter needs `--metrics` (see the README, Metrics): enable it on every node (`join --metrics :9090`) and check that each one answers before you read a zero. While `EnforceLiveness` is false each series counts a case where the legacy `LastSeen` decision differs from the one the evidence would give (the three store sites count only a peer the legacy test drops and the evidence keeps; the other sites count a difference either way), so it shows what N+1 would change on that node. It counts per call, not per distinct peer: a case that stands is counted again on every pass. An extra reconcile and the announce after a wake each read the store, so expect a spike of `wgmesh_liveness_disagree_total{site="get-active"}` at each wake; the number per wake is not measured (`GetActive` has 12 call sites across the loops). Counters reset when the daemon restarts. Every site is created at 0 at start.

| Site | A count means |
|---|---|
| `get-active` | the legacy test drops a peer from the active set that the evidence keeps |
| `cleanup` | the stale cleanup would remove a peer the evidence keeps |
| `is-dead` | `IsDead` calls a peer dead that the evidence keeps |
| `relay-candidate` | an introducer is judged a relay candidate (seen within `RelayCandidateMaxAge`) differently by `LastSeen` age than by the evidence |
| `probe-enforce` | the probe loop judges a peer older than 45 seconds (so its failures are enforced) differently by `LastSeen` age than by the evidence |
| `rendezvous-skip` | a rendezvous attempt to a peer first reached that way is skipped as "seen within 2 minutes" by one age and not by the other |
| `introducer-candidate` | a peer is dropped as an automatic introducer candidate (older than 2 minutes) by one age and not by the other |
| `resume-grace` | an eviction or a stale-handshake relay choice that the 60-second wake grace would have blocked |

The go/no-go for N+1, per node: `curl -s http://<host>:<port>/metrics | grep wgmesh_liveness_disagree_total` (the port is the one given to `join --metrics`). Read it on every node for a release. Each count is either explained (a wake, a restored peer, a peer kept connected by handshakes only) or the counter is zero; any count you cannot explain keeps `EnforceLiveness` false and holds N+1.

## Release note for N+1

`EnforceLiveness` is true and the verdict acts:

- A peer kept alive by a handshake or a mesh probe stays in the active set although no announcement arrived from it. The active set (`GetActive`, hence the WireGuard interface) still drops a peer at 5 minutes of evidence; it is not widened.
- A peer whose control plane is dead but whose tunnel works stays Alive by its handshakes, so it is advertised in `KnownPeers` and in gossip from N+1 (ADR-0004 M1).
- A restored peer is dialable for `PeerDeadTimeout` (5 minutes) after the restore, although its cache `last_seen` is older.
- For 60 seconds after a wake (`ResumeGrace`) the health loop and the mesh probe do not evict, and a stale handshake does not choose a relay: handshake ages include the time asleep until WireGuard has had one rekey. The cost: a peer that died while the machine slept stays for up to 60 seconds after the wake.
- `IsDead` moves from 5 to 10 minutes of silence, and the stale cleanup removes after 10 minutes of evidence instead of 10 minutes of `LastSeen`.
- Upgrade every node to N first. A node from before N never writes the evidence and judges by `LastSeen` alone; the two kinds of node can disagree on who is alive.

## Release note for N+2

`EnforceRelayProbe` is true:

- A relayed peer that has a usable endpoint stays configured on the interface with its endpoint and no AllowedIPs (probe-only). WireGuard keeps sending handshake attempts to the endpoint, so a fresh handshake can bring the peer back to a direct path; the relay carries its traffic meanwhile. Each such peer costs a keepalive every 25 seconds, one per relayed peer: the daemon applies every peer with `SetPeer`, which always sets `persistent-keepalive 25` (the bulk apply in `pkg/wireguard/apply.go`, used by the SSH mesh deploy, sets it only when the peer config has one). The traffic this adds is not measured.
- **A mixed-reachability window exists.** While node A holds B probe-only and B holds A direct, B's direct packets are dropped at A (A has no AllowedIPs for B). A returns B to direct when its hysteresis sees a fresh handshake, after about 3 reconcile cycles, about 15 seconds. This is from the code; it is not measured. Whether WireGuard drops packets of a peer with an empty list is from its semantics, not measured here.
- `wg set <iface> peer <key> allowed-ips ""` was run on a scratch Linux interface: it clears the list and keeps the endpoint. On macOS (userspace WireGuard) it was NOT run: that is a human step that is still open (ADR-0004 T6 S2). If it fails there the only signal is the log line `Failed to apply WireGuard peer configuration` on every reconcile.

## Still open

- ADR-0004 T4 S6: `pmset sleepnow` on a Mac, then the time to the first successful ping to a peer. Not measured: this note states no wake time.
- ADR-0004 T6 S2 on macOS (above).
- The wake detector can fire on a long pause of the process or a clock step above 10 seconds: one extra reconcile, one announce burst, and at N+1 a 60-second wake grace.
- The `Policy.Local` read without a lock stays open (ADR-0005).
- OS power events and rebinding the transport socket on a wake are backlog rows, not built.

## Closed

- The `NATType` guard added by ADR-0004 T5: the local `NATType` is read and written through `GetNATType` and `SetNATType` under one lock (`endpointMu`), which closes that half of the known limit in [the backlog](adr/BACKLOG.md).

## Rollback

N adds evidence, counters, the wake actions and the changes listed in the note for N (the probe loop on the evidence, the health skip of relayed peers); reverting N loses the in-memory evidence and nothing else, and the cache file stays readable by the older version, which reads `last_seen` as `LastSeen` (see the cache bullet). Reverting after N+1 returns the `LastSeen` rules. Reverting after N+2 returns relayed peers to being removed from the interface; no file or wire format changes in any of the three releases.
