# Admit step: rollout

Decision record: [ADR-0002](adr/ADR-0002-admit-step/ADR-0002-admit-step.md). This page is for the operator of a mesh and is the release note for the three releases that bring it in. Two of the three are not done yet: they are rows in [the backlog](adr/BACKLOG.md), not tasks.

Every claim a node hears about a peer (its public key, mesh IP, routes, endpoint) now passes one decision in `pkg/node` before it reaches the peer store. No byte on the wire changes, so old and new nodes talk to each other during the whole rollout.

## What changes and in which release

| Release | What the node does | Switches |
|---|---|---|
| N (the first with this change) | counts every case an enforced rule would act on (`wgmesh_admit_warnings_total{rule}`) and applies the claim as before; the other rules below act at once; the local-collision re-address (below) is not behind a switch and happens from N | `node.EnforceIPBinding = false`, `node.EnforceRouteBinding = false` |
| N+1 | the bound claim wins an IP collision; a first-hand claim from a new key for an address another key holds by derivation is refused | `EnforceIPBinding = true` |
| N+2 | route entries inside the mesh network or equal to a member's mesh IP are dropped; with an `accept-routes` list, networks it does not approve are dropped | `EnforceRouteBinding = true` |

Both switches are `false` in release N and nothing in production sets them. Flipping each one is a one-line change with its own release note.

## Release note for N

- **A node that loses a mesh IP collision changes its interface address while running.** Until now the local node never took part in a collision with a remote peer, so the "we lost" branch could not run. From N it does: a collision with the local node is decided by the same order as any other pair (in N the lower public key wins), and the loser re-derives its mesh IP and sets the new address on its WireGuard interface at runtime. The first free address of nonce 1 to 10 is taken. This is not behind a switch: it happens from N. Expect it only on a mesh where two members derived the same address.
- **The resolved address survives a restart.** The IP is written to the state file (`/var/lib/wgmesh/<interface>.json`), the IP and not the nonce. `wgmesh service add` reads the state file `wg0.json` only: on another interface name it does not see the resolved address (known limit).
- **A withdrawn peer is not brought back by a mention for 5 minutes.** A GOODBYE or a health eviction of a stored peer leaves a tombstone for 5 minutes, and so does stale cleanup, but only of a peer that was heard first-hand (`TombstoneTTL`). Mentions in other peers' lists and cache entries are refused meanwhile; the peer's own first-hand packet clears the tombstone at once. Limits: the tombstone is in memory, so a restart forgets it; other nodes keep their stale entry until their 10-minute remove timeout, so one more resurrection by mention is possible in the second five minutes; a replayed or new first-hand HELLO after a GOODBYE re-admits the peer (BACKLOG).
- **A guessed endpoint ranks below everything.** A peer that advertises no usable endpoint gets the sender's IP with the default WireGuard port (an advertisement with an empty port counts as nothing). That guess ranks below the peer cache, is replaced by the first advertisement, is not announced in `KnownPeers` and is not written to the peer cache. `wgmesh peers list` and `daemon.status` show a guess like any endpoint (BACKLOG: an `endpoint_guessed` field). In a mixed fleet a node from before this release still announces its guesses, so a wrong address keeps spreading until every node runs this release.
- **One announcement filter.** `crypto.FilterAnnouncement` is the one validation path of the LAN, gossip, registry and exchange readers. An invalid entry in `KnownPeers` is dropped with one warning per message and the message is accepted. This is receiver-side: a node from before this release still rejects the whole message. A replayed REPLY or ANNOUNCE is no longer logged as "Received valid"; RENDEZVOUS, GOODBYE and unknown types are still logged before their payload is parsed.
- **The local endpoint changes more slowly on STUN alone.** `LocalNode.ObserveEndpoint` applies a STUN answer that differs from a usable endpoint only on the third identical observation in a row (about two minutes after the first differing answer at the 60 s refresh). A peer reflection and an interface (global IPv6) address apply at once and discard the pending STUN count; any source applies at once while the endpoint is empty or `0.0.0.0`; a reflected unspecified address is ignored. Reflection against reflection is not damped (second half of issue #828, BACKLOG). ADR-0004 M5 adds a `stun-resume` source (`daemon.EndpointSTUNResume`) that applies the first STUN after a wake at once; see [Liveness rollout](liveness-rollout.md).
- **`--accept-routes`** is new (see below) and does nothing unless set.

## Upgrading a mixed fleet

- Upgrade every node to N before anything else. A node from before N never moves after losing a mesh IP collision: only N code runs the "we lost" branch, and for a remote loser an N node only logs the address it expects.
- Flip a switch only when every node runs the release before it. An N+1 node refuses a claim that an N node admits, so the two disagree about who owns an address; the same holds for routes with N+2.

## Reading `wgmesh_admit_warnings_total{rule}` as the go/no-go

The counter needs `--metrics` (see the README, Metrics): enable it on every node and check that each one answers before you read a zero. It counts each time an offer or a collision comparison meets a case an enforced rule would act on, so a case that stands is counted again on every announcement and every collision pass, and once per observing node. Read it as zero or not zero, not as a rate. Only `winner-differs` is created at start; every other series is absent until its first case is counted, so write an alert as `sum by (rule) (wgmesh_admit_warnings_total) or vector(0)`. Counters reset when a node restarts, so the gate is "zero since process start on every node": look at every node, not at one.

| Rule | A count means | Gate |
|---|---|---|
| `ip-held-by-bound` | a first-hand claim from a key not yet in the store for an address another key derives | zero for a release before N+1 |
| `winner-differs` | a collision where the bound-first order would pick another winner than the lower key does today | zero before N+1 |
| `route-inside-mesh` | a route entry equal to or inside the mesh network | zero before N+2 |
| `route-inside-member-ip` | a host route equal to another member's mesh IP | zero before N+2 |
| `route-not-approved` | a first-hand or restored route that the receiver's `accept-routes` list would drop | zero before N+2 on every node that sets a list |

A non-zero reading says who claims what in the log lines of the node that counted; fix the claimant or decide that the case is legitimate before flipping. The `route-inside-mesh` and `route-inside-member-ip` rules also count second-hand mentions, and every announcement counts a standing case again, so a gossip-heavy mesh over-counts.

## What is never refused

An address that a node keeps after a secret rotation, an address derived with a collision nonce, and an address in a custom subnet are unbound, not wrong. They are admitted in every release and every phase, and a test fails if one is refused (`testdata/compat/admit/v1`, frozen hand-computed recordings that are never regenerated). Binding only orders claims, from N+1 (`EnforceIPBinding`): a bound claim (an address the claim's own key derives at nonce 0 to 10) beats an unbound one, then the lower public key wins. In N the lower public key alone decides.

## The grinding limit

Binding has no new wire bytes, so it ties an address to a key and the secret, not to the owner of the key. A member who holds the secret can generate key pairs until one derives to a victim's address. Measured once, 2026-11-15, on host mars (Intel Core i5-9300H, 4 cores): 20 trials, nonce 0, default subnet, mean 61004 key pairs, median 51345, min 4232, max 173042 (`testdata/compat/admit/v1/README.md`). Nonces 1 to 10 were not measured. The limit is documented, not closed: signatures per node key are a separate decision (ADR-0002, Out of Scope).

## `accept-routes`: the networks this node approves

- Set it with `--accept-routes 10.1.0.0/16,192.168.5.0/24` on `join` and `install-service`, or with a line `accept-routes=...` in the reload file (the same file as `advertise-routes`, signalled with SIGHUP).
- Entries are parsed like `advertise-routes` entries (`NormalizeRoutes`): a bare IP is a `/32` or `/128` host route, host bits are cleared with a logged warning, duplicates are dropped.
- **Unset or empty means open, as today.** A node with no list approves every network and nothing is counted. That stays true after N+2: no deny by default; an operator opts in by setting a list.
- A network is approved when it equals one entry or lies inside one. Approval is by network only, never by the advertising member, so several members may advertise one approved network (HA gateways for one subnet keep working, each is approved). An entry such as `10.0.0.5/24` is stored as `10.0.0.0/24` and logs the same normalisation warning as `advertise-routes` (the text says "advertise route"; known wording limit).
- A default route stays advertisable. With a list set, `0.0.0.0/0` and `::/0` are approved only by an entry that is itself a `/0`: `10.0.0.0/8` does not cover them. Two HA exit nodes still each drop the other's default route through `routes.ExcludeOwn`, which is unchanged.
- The list judges first-hand and restored routes. A second-hand mention is not judged for approval.
- Staged: in N a network the list would drop is counted as `route-not-approved` and kept; at N+2 it is dropped. Configure the list in N and watch the counter.
- Reload: an empty value (`accept-routes=`) clears the list; a removed line keeps the old list; a reload file with an invalid line is refused whole, the old lists stay and the reason is logged. A reload narrows future offers only: routes already stored change when the peer next announces.
- `install-service` does not validate `--accept-routes` or `--advertise-routes` when it installs: a bad list is first refused when the service starts (BACKLOG).

## Known limits

- The `ip-held-by-bound` refusal needs the claimant's own claim to be unbound, and only a key not yet in the store is refused; a known key is only ordered.
- A host route equal to a member's IPv6 mesh address (an IPv6 /128) is not caught.
- The route counters count second-hand mentions and count a standing case again on every announcement.
- `Policy.Local` reads the local node without a lock inside `Offer`, and `NATType` is written without the endpoint lock (ADR-0005 owns the local node).
- Reflection against reflection is not damped; only STUN is.

## Rollback

N only counts, apart from the visible changes above. Reverting N restores direct peer-store calls; the state file may hold a collision-resolved `mesh_ip`, which an older node keeps if it lies in the subnet. Tombstones are memory only. After N+1 or N+2, set the switch back to `false` in the next release: nothing persisted needs rewriting.
