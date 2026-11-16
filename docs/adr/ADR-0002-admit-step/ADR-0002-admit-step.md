# ADR-0002: Add one stateless admit step between peer discovery and the PeerStore

**Status:** Accepted
**Date:** 2026-11-15
**Owner:** wgmesh maintainers
**Spec:** None — no spec stage; `eidos/spec - peer store - thread-safe registry with endpoint ranking and pub-sub.md`, `eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md` and `eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md` are amended by T11
**Cross-references:** `docs/adr/ADR-0001-envelope-aad/ADR-0001-envelope-aad.md` (its deferred item "change the mesh IP collision winner rule and bind a mesh IP to its public key" is taken up here), `docs/adr/BACKLOG.md`, `testdata/compat/envelope/v1/README.md`; issues #864, #854, #856, #830, #839, #828, #875 (all open on 2026-11-15); commits `d50c51e` (transitive mentions cannot rewrite first-hand fields) and `7414656` (an own empty announcement withdraws routes, PR #881), both already in `pkg/node/store.go` at `5edcdd0`
**Governs:** `pkg/node/*.go`, `pkg/daemon/collision.go`, `pkg/daemon/cache.go`, `pkg/discovery/exchange.go`, `pkg/discovery/lan.go`, `pkg/discovery/gossip.go`, `pkg/discovery/dht.go`, `pkg/discovery/registry.go`, `pkg/daemon/config.go`
**Enforced-by:** None — the check is `TestNoDirectPeerStoreMutation` in `pkg/node/admit_guard_test.go`, which does not exist until T2 adds it red-first; it fails when a production file outside `pkg/node` calls `Update`, `Remove` or `SetPeerDirectly` on a store-named receiver
**Invalidates:** none — checked (`adr-state` on 2026-11-15 reports ADR-0001 as the only record; this ADR edits `PeerExchange.handleMessage` in `pkg/discovery/exchange.go`, which ADR-0001 governs, and keeps ADR-0001's order: open, count legacy, nonce cache, dispatch; T8 and T9 re-run ADR-0001's fixture test)
**Served-path change:** A peer claim arriving from LAN, gossip, DHT exchange or the cache now passes one `PeerStore.Offer` decision (`pkg/node`) instead of `PeerStore.Update`, so a removed peer is not resurrected by a transitive mention, the local node takes part in mesh IP collisions, a resolved mesh IP survives a restart, and, after the staged enforcement releases, a claim for a mesh IP that another key holds by derivation, or a route inside the mesh address space, is refused.

## Decisions

M answered the six questions of the first draft on 2026-11-15. Recorded here, dated, in M's words.

1. **Binding without new wire bytes** (M 2026-11-15: "Taip, be naujų wire baitų"). Readers verify the binding themselves (derive the IP from the sender's key and try nonces 0 to 10). Derivation binds an IP to a key and the secret, not to the owner of the key: the default derivation uses two hash bytes (`pkg/crypto/derive.go:160-161`), so a member who holds the secret can generate key pairs until one derives to a victim's address (about 65 thousand tries for the default subnet at nonce 0, divided by the 11 nonces that count as bound, so about 6 thousand; amended 2026-11-15 (T6, T11): measured for nonce 0 only, 20 trials, mean 61004 key pairs, median 51345, min 4232, max 173042, on host mars (Intel Core i5-9300H, 4 cores), see `testdata/compat/admit/v1/README.md`; the division by the 11 nonces is not measured). The limit is documented, not closed.
2. **Tombstone lifetime is 5 minutes** (M 2026-11-15: "5 min (kaip issue)"), a separate constant `TombstoneTTL` with its own test, not `PeerRemoveTimeout`. Consequence stated in Consequences: other nodes hold a stale entry until `PeerRemoveTimeout` (10 minutes, `pkg/node/store.go:12-13`), so one more resurrection by mention is possible in the second five minutes.
3. **Winner order** (M 2026-11-15: "Susietas pirmas, tada mažesnis raktas"): a bound claim first, then the lower public key; a collision loser keeps its resolved address (persist the IP).
4. **Staging** (M 2026-11-15: "Taip: N+1 IP, N+2 maršrutai"): N warns, N+1 enforces the IP rules, N+2 enforces the route rules.
5. **#828 stays** (M 2026-11-15: "Taip, lieka kaip T9").
6. **Route approval is in this ADR** (M 2026-11-15: "Daryti šiame ADR"). Design in Decision item 4a, task T10. M's earlier decision of 2026-11-15 is kept: two HA exit nodes each drop the other's `0.0.0.0/0`, and an advertised default route stays a feature (#859).
7. **An unset `accept-routes` stays open after N+2** (M 2026-11-15: "Lieka atviras kaip šiandien (Recommended)"): no deny-by-default; an operator opts in by setting the list.
8. **Approval is by network only** (M 2026-11-15: "Ne, tik tinklas (Recommended)"): an `accept-routes` entry never names an advertiser key.
9. **The punch reply keeps its label** (M 2026-11-15, harness question during T2 review: "Hand=FirstHand, Source lieka dht-transitive (Recommended)"). `dht.go:1068` offers the reply of a successful punch first hand, and its `Source` stays `dht-transitive`: `pkg/daemon/daemon.go:720` prefers a relay for a peer whose `DiscoveredVia` holds `dht-transitive` and that has no handshake yet, and `endpointMethodRank` ranks the label 40. Trust is read from `Hand`; the label is a path marker. T2's first version passed `dht` there; the review found the consumer.
10. **`ObserveEndpoint` lives on the daemon's local node** (M 2026-11-15, harness question before T9: "Pataisyti T9: ObserveEndpoint ant daemon.LocalNode (Recommended)"). `pkg/node/types.go:35` declares a `LocalNode` that nothing uses (`grep -rn "node\.LocalNode" --include=*.go .` lists no user, checked 2026-11-15 at `b8b83f3`); the type discovery calls `SetEndpoint` on is `daemon.LocalNode` (`pkg/daemon/daemon.go:102`). T9 adds `ObserveEndpoint` there, with the endpoint lock that type already has or one it gets in the same task. Merging the two types is ADR-0005's work, not this ADR's.

## Open Questions for M

None. M answered the last two on 2026-11-15 (decisions 7 and 8 above).

## Context

**The pipe.** Discovery today is `discover (DHT, LAN, gossip, exchange, STUN, handshake) -> PeerStore.Update -> reconcile (derive desired, diff actual, apply wg, ip, iptables, files)`. In ETL terms the extract step has six sources, the load step is `PeerStore.Update`, and the transform in between does not exist as a step: each source builds a `PeerInfo` itself and the store fixes what it can afterwards. The missing step is **admit**: given a candidate claim, its source and whether the sender made it itself, decide admit, merge only some fields, or refuse, with a reason.

**Class audit** (2026-11-15, read at `5edcdd0`). Class: *every place a claim about a peer enters or leaves the PeerStore, or the node changes its own claim, without passing one shared trust step.*

| Command | Count | Members |
|---|---|---|
| `grep -rn "peerStore\.Update(" --include=*.go . \| grep -v _test.go \| wc -l` | 10 | `pkg/discovery/lan.go:229`, `exchange.go:403`, `:436`, `:891`, `:1035`, `dht.go:1068`, `:1129`, `gossip.go:323`, `:340`, `pkg/daemon/cache.go:126` |
| `grep -rnE "peerStore\.Remove\(\|CleanupStale\(\)" --include=*.go . \| grep -v _test.go \| grep -v "func (ps"` | 3 | `exchange.go:348` (GOODBYE), `pkg/daemon/daemon.go:1394` (health eviction), `daemon.go:934` (stale cleanup) |
| `grep -rn "OpenEnvelope(\|OpenEnvelopeRaw(" --include=*.go . \| grep -v _test.go \| grep -v pkg/crypto/envelope.go \| wc -l` | 5 | `exchange.go:231`, `lan.go:203`, `gossip.go:281`, `registry.go:169`, `main.go:573` |
| `grep -rn "\.SetEndpoint(" --include=*.go . \| grep -v _test.go \| grep -v "func (n" \| wc -l` | 8 | `dht.go:255`, `:271`, `:284`, `:420`, `:440`, `:457`, `exchange.go:494`, `daemon.go:1738` |

Left out on purpose: `main.go:573` prints a reply and writes nothing (the join diagnostic); `registry.go:169` decodes a peer list but `FindOrCreate` has no caller (`grep -rn FindOrCreate --include=*.go .` lists only its definition), so the registry is dormant and is covered by the same guard the day it is wired; `daemon.go:1738` sets the initial `0.0.0.0:<port>` and stays a plain set.

**What each source re-implements** (all `5edcdd0`):

- *Trust is a substring.* `isTransitiveMethod` is `strings.Contains(method, "transitive")` (`pkg/node/store.go:185-187`); it decides whether a claim may rewrite mesh IP, routes and introducer (`:126-144`) and whether `LastSeen` is refreshed (`:175-183`). `dht.go:1068` passes `dht-transitive` for the reply of a peer that just answered a punch, which is first-hand (the same reply object was already stored first-hand at `exchange.go:436`); `dht.go:1129` forwards the caller's method (`dht` from `:832`, `dht-ipv6-sync` from `:985`), and that string is kept. The same string also ranks endpoints (`endpointMethodRank`, `:191-217`).
- *Endpoint rules differ per source.* LAN takes the sender's IP and the advertised port (`lan.go:235-244`); exchange keeps an advertised host unless it is unspecified and falls back to the sender IP with `DefaultWGPort` (`exchange.go:1058-1076`); standalone gossip uses the advertised value (`gossip.go:306-310`). The guessed `DefaultWGPort` is stored like an advertisement and re-announced to others through `getKnownPeers` (`exchange.go:1140-1162`). A new peer with an empty endpoint is inserted without a check (`store.go:99-114`); an empty endpoint never overwrites a known one only because of the guard at `store.go:117`.
- *Validation differs per source.* The exchange handlers drop one invalid `KnownPeers` entry and keep the message (`validateExchangeAnnouncement`, `exchange.go:365-382`); LAN and gossip go through `OpenEnvelope`, whose `Validate` rejects the whole message (`pkg/crypto/envelope.go:125-129`); `logIncomingPacket` says "Received valid" before the payload is validated (`exchange.go:240`, `:1235-1238`). These are the items 2 and 3 of #875.
- *Removal leaves no trace the store reads.* GOODBYE (`exchange.go:348`) and health eviction (`daemon.go:1394`) call `Remove`; the only memory of an eviction is a 30-second daemon flag (`markTemporarilyOffline`, `daemon.go:1393`, `TemporaryOfflineTTL` `daemon.go:38`) that `Update` never consults. A transitive mention re-inserts the peer with `LastSeen = now` and its introducer flag (`store.go:104-108`), so an evicted introducer comes back (#854).
- *IP claims.* `DetectCollisions` scans only the PeerStore (`collision.go:22-47`), the local node is never in it, so the branch "we lost" (`collision.go:107`) is unreachable for a collision with a remote peer (#856). The resolved IP is set in memory only (`collision.go:120`); the only `saveLocalNode` calls are in `initLocalNode` (`daemon.go:375`, `:384`, `:425`), so a collision loser reverts on restart (#830). `DeriveMeshIPWithCollisionCheck` (`collision.go:146`) has no non-test caller.
- *Reconcile trusts the store.* The mesh IP becomes an AllowedIP (`daemon.go:627`) and routes are installed from `RoutableNetworks` (`daemon.go:617`, `:631`) with no check against what other members hold (#864).
- *A persisted mesh IP outlives the secret.* After a secret rotation any IPv4 address in `10.0.0.0/8` is kept (`daemon.go:313-326`, commit `c3d153a`), so a node's address is not always derivable from its key and the current secret. A rule "the IP must equal the derived IP" would refuse rotated nodes; the rule must be an ordering between claims, not a gate on every claim.

**Debt pulled in.** `adr-debt docs/adr` on 2026-11-15 lists the ADR-0001 deferral "Changing the winner rule of mesh IP collisions or binding a mesh IP to a public key" (`docs/adr/BACKLOG.md`); this record takes it up, and T11 rewrites that row to point here. The other ADR-0001 deferrals (N+1 and N+2 of the AAD rollout, the gossip replay cache, the HELLO replay) are unrelated and stay.

## Existing Primitives Audit

- `PeerStore.Update` merge rules (`pkg/node/store.go:86-173`): reuse as the merge step, driven by an explicit hand instead of a substring; behaviour of the already merged rules (transitive never rewrites, first-hand empty routes withdraw) is pinned by T1 and must not change.
- `endpointMethodRank` and `shouldUpdateEndpoint` (`store.go:189-247`): reuse; T7 adds one rank below all others for guessed endpoints.
- `DeterministicWinner`, `DetectCollisions`, `ResolveCollision`, `CheckAndResolveCollisions` (`pkg/daemon/collision.go`): reshape. The pure parts (detection, winner) move to `pkg/node` and include the local node; the effects (set interface address, persist) stay in the daemon.
- `validateExchangeAnnouncement` (`exchange.go:365`): reuse; moves to `pkg/crypto` so LAN and gossip use the same filter.
- `markTemporarilyOffline` (`daemon.go:1415`): kept; it is a 30-second reconcile flag, not a tombstone, and does not guard the store.
- `RateLimiter` and `nonceCache` (ADR-0001): untouched; they act before dispatch, admit acts after it.
- `crypto.DeriveMeshIP`, `DeriveMeshIPInSubnet`, `DeriveMeshIPWithNonce`: reuse through a closure supplied by the daemon, so `pkg/node` holds no secret.
- `PeerStore` is the single owner of peer state and takes one mutex; the decision is made under that mutex so that read-then-write cannot race. A separate admit object that snapshots the store and then calls `Update` would reopen that race.
- Nothing like a tombstone exists (`grep -rn -i tombstone --include=*.go .` returns nothing at `5edcdd0`).

## Decision

1. **One entry point, `PeerStore.Offer(Candidate) Decision`, in `pkg/node`.** `pkg/node` is the shared-types package (`types.go:1-4`); `daemon` imports it and `discovery` reaches it through the aliases in `pkg/daemon/peerstore_compat.go` (`grep -l 'wgmesh/pkg/node"' pkg/discovery/*.go` lists nothing; six non-test files import `pkg/daemon`). The decoupling that matters, `daemon` not importing `discovery`, is untouched; T1 adds the aliases for the new types to `peerstore_compat.go`. A `Candidate` is the claimed `PeerInfo`, a typed `Source` (`lan`, `dht`, `dht-rendezvous`, `gossip`, `cache`) and a typed `Hand`: `FirstHand` (the sender describes itself), `SecondHand` (a `KnownPeers` mention) or `Restored` (the on-disk cache). `Candidate.Method()` returns the same strings as today (`dht`, `dht-transitive`, `gossip-transitive`, `cache`, ...), so `DiscoveredVia`, `EndpointMethod` ranks and the cache file do not change. A `Decision` is one of `Admit`, `AdmitPartial` (fields named in `Dropped`), `Refuse`, plus a named `Reason` and a list of `Warnings` (rules that would refuse in enforce mode).
2. **The decision is a pure function.** `Decide(in)` reads a snapshot (the peer map, the tombstones, the local node's claim, the policy) and the candidate and returns the `Decision`; it does no I/O and reads no clock except through the store. `Offer` takes the store lock, calls `Decide`, then applies the merge (the existing `Update` body, keyed on `Hand`). `Update` stays exported only for the 58 existing test call sites and for fixtures, forwards to `Offer`, and is documented as test-only. **The guard is a test, not a hope**: `TestNoDirectPeerStoreMutation` parses every non-test `.go` file outside `pkg/node` and fails on `Update` or `SetPeerDirectly` (from T2) and `Remove` (from T3) called on a store-named receiver; `SetLatency` and `SetEndpointMethod` are not covered.
3. **Rules, one list, each a case of the step** (a rule is a table row in `Decide` plus a test; none lives in a source):

| Rule | Issue | Decision |
|---|---|---|
| R1 hand is explicit | (class) | trust is read from `Hand`, never from the method string; the one first-hand reply labelled `-transitive` (`dht.go:1068`) gets the first hand and keeps its label, which `daemon.go:720` reads (decision 9); method strings such as `dht-ipv6-sync` are kept |
| R2 tombstone | #854 | a stored key withdrawn by GOODBYE or health eviction, or removed by stale cleanup (only a peer heard first-hand), refuses `SecondHand` and `Restored` offers for `TombstoneTTL` (5 minutes); a `FirstHand` offer clears it |
| R3 local claim | #856 | the node's own `(key, mesh IP)` is a member of the IP claim table; a candidate that claims the local IP is a collision decided by the same order as any other pair |
| R4 claim order | #864, #856 | on one IP: a bound claim (IP equals the IP derived from that claim's own key, nonce 0 to 10) beats an unbound one; equal classes fall to the lower public key, which is what every observer computes; this order applies from N+1 (`EnforceIPBinding`), in N the lower key alone decides and the other case is only counted |
| R5 binding | #864 | a first-hand claim from a key not yet in the store, for an IP held by a bound claim of another key, is refused (amended 2026-11-15, T11: only when the claimant's own claim is unbound) (a key already stored is only ordered by R4, never refused, so a rotated node that is known stays known); a route entry equal to or inside the mesh network, or equal to another member's mesh IP, is dropped from the candidate (amended 2026-11-15, T11: "inside another member's mesh IP" means a host route EQUAL to that mesh IP); supernets such as `0.0.0.0/0` are allowed because WireGuard matches AllowedIPs by longest prefix |
| R6 endpoint | #839 | an empty first-hand endpoint never clears a known one (pinned); an endpoint guessed from the sender address with the default port is stored at the lowest rank, replaced by any advertisement, and never re-announced in `KnownPeers`; no endpoint is built from the sender's port |
| R7 local endpoint | #828 | the node's own endpoint changes only when a peer reflection says so, when an interface (global IPv6) address is found, or when STUN gives the same new value three times in a row; alternating STUN observations do not flip it (amended 2026-11-15, T11: the `interface` source was missing from this row; ADR-0004 M5, confirmed by M 2026-11-15 22:46, adds a `stun-resume` source (`daemon.EndpointSTUNResume`) that applies the first STUN after a wake at once) |
| R9 route approval | #864 | a non-mesh network in a first-hand or restored candidate is dropped from it when the receiving node has an `accept-routes` list and the network is not equal to or inside one entry; an unset list approves everything; approval is by network, never by advertiser; staged: a network it would drop is warned (`route-not-approved`) at N and dropped when `EnforceRouteBinding` is set at N+2 |
| R8 one validation | #875 | sender fields are strict, invalid `KnownPeers` entries are dropped one by one on every path; the "valid" log line follows validation |

4a. **Route approval (R9).** Who may advertise a non-mesh network is decided by the *receiving* node's operator, so no byte on the wire changes and no advertiser has to upgrade. The source is a new list `accept-routes` (CIDRs; config field, `--accept-routes` flag beside `--advertise-routes` at `main.go:333` and `:788`, and the reload file like `advertise-routes`), parsed by the same parser. Rules: (a) unset or empty means open, which is today's behaviour; (b) with a list, a network is approved when it is equal to or inside one entry, so `0.0.0.0/0` and `::/0` are approved only by an entry that is itself a `/0` (a `10.0.0.0/8` entry does not cover them); (c) approval is per network, not per advertiser, so any number of members may advertise one approved network, which is what HA shared subnets need; (d) a default route is never refused for being a default or a supernet, it is an ordinary network under (a) and (b), so it stays advertisable (#859); (e) the apply-side rule that a node drops a peer's advertisement of a network it advertises itself (`routes.ExcludeOwn`, `pkg/routes/routes.go:116-131`, which is how two HA exit nodes each drop the other's `0.0.0.0/0`) stays in reconcile and is neither duplicated nor replaced by admit. Staging per decision 4: a network that the list would drop is counted at N as `route-not-approved` and dropped at N+2 when `EnforceRouteBinding` is flipped, the same switch as the mesh-network route rule of R5, so an operator can configure the list at N and watch the counter. The list is read through `Policy.AcceptRoutes`, a closure over the daemon config, so a reload applies to later announcements; routes already stored drop when the peer next announces.
4b. **The effects stay where the I/O is.** The daemon applies a collision (interface address) and now also persists it (`saveLocalNode`), so the resolved IP is the persisted IP and `initLocalNode` keeps it at restart through the existing check at `daemon.go:361` (#830; the IP is persisted, not the nonce).
5. **Staging, the two-release rule (as ADR-0001 does).** Two switches in `pkg/node`, both `false` in release N: `EnforceIPBinding` (R4's winner order and R5's IP refusal) and `EnforceRouteBinding` (R5's route drop). In release N the rules are computed and every case where enforcement would act is counted and logged (`wgmesh_admit_warnings_total{rule}`) while the offer is applied as today; the winner of a collision is still the lower key. N+1 flips `EnforceIPBinding`, N+2 flips `EnforceRouteBinding`, each a one-line change with its own release note, recorded as a deferred follow-up. R1, R2, R3 (detection), R6 and R8 act from N: they refuse only claims no older node relies on (a resurrected removed peer, a guessed endpoint), or make a reader more lenient. **What the wire looks like**: no byte changes (see Open Question 1), so there is no new capability version; the compatibility contract is `testdata/compat/admit/v1/`, frozen recordings of claims from older releases (a rotated address, a collision-nonce address, a custom-subnet address, a node without any route) with the decision each phase must give; the rotated and nonce cases must be admitted in every phase, and a test fails if any is refused. Rotated addresses are admitted permanently: they are unbound, not wrong.

What would make this decision fail: (a) a source that calls `Update` or `Remove` directly, caught by the guard test; (b) a rotated or collision-nonce node refused at N+1, caught by the compat fixtures; (c) the merge rules of `d50c51e` and `7414656` changing, caught by T1's characterisation test; (d) a removed peer coming back through a mention, caught by T3. Only (c) can be red today; the others exist once their task's first step has run, against compiling stubs so the first red run fails on assertions.

## Alternatives Considered

- **Fix each issue in the source that shows it.** Rejected: that is how the class arose (ten call sites, three endpoint rules, two validation rules). Each fix lands in one of six sources and the next source added, or the cache, starts without it. A guard test that finds a direct call is cheaper than reviewing every source on every change.
- **Put the checks inside `PeerStore.Update` and keep its signature.** Rejected: the signature carries a method string as the only context, which is the stringly trust of R1; the claim table, the tombstone and the local claim would be read from fields of the store without the caller saying how far the claim can be trusted.
- **A separate `Admitter` object that reads a snapshot and then calls the store.** Rejected: a read followed by a write is not atomic against a concurrent offer from another source; `PeerStore` already has the one mutex that makes the decision and the merge one step.
- **Require the derived IP for every claim (the "short term" fix in #864).** Rejected: a node keeps its address across a secret rotation (`daemon.go:313-326`), so the rule would refuse valid nodes; used as an ordering (R4) it keeps them.
- **Sign the claim with a per-node key** (so a member cannot claim an IP for a key it does not own). Rejected here: WireGuard keys are Curve25519 key-agreement keys, a signature needs a conversion or a second key and a way to distribute it (same reason as ADR-0001, "per-sender signature"); it is the real fix for the grinding limit in Open Question 1 and its own decision.
- **Add `mesh_ip_nonce` to the announcement.** Rejected: readers can try the 11 nonces themselves (11 hashes), and a field would be one more thing an old node ignores and a malicious one lies about.
- **Tombstone in the daemon (extend `markTemporarilyOffline`).** Rejected: that flag guards reconcile, not discovery, and the sources run in `pkg/discovery`, and the store is what every source already shares.

## Component / Boundary Impact

- `pkg/node`: gains the admit step (`admit.go`, `claims.go`, `binding.go`, `endpoint.go`) beside the store; one reason to change (what a claim may do to the shared peer state). It imports nothing new; the secret reaches it only as a derive closure.
- `pkg/discovery`: sources call `Offer` and `Withdraw`; they keep parsing, sender address handling and transport. No source holds a trust rule any more.
- `pkg/daemon`: builds the policy (local claim, derive closure, warning counter from T4; mesh network from T6), applies collisions and persists the result. `daemon` still does not import `discovery`.
- `pkg/crypto`: gains the one lenient announcement filter used by every reader (T8); the AAD and nonce code of ADR-0001 is not touched.

## Wiring & Contract Changes

| Surface | Change | Producer | Consumer(s) |
|---------|--------|----------|-------------|
| `node.Candidate`, `Source`, `Hand`, `Decision`, `PeerStore.Offer` | new | `pkg/node` | `lan.go`, `exchange.go`, `gossip.go`, `dht.go`, `daemon/cache.go` |
| `PeerStore.Withdraw(key, cause)` and tombstones | new; `Remove` leaves the production call sites | `pkg/node` | `exchange.go` (GOODBYE), `daemon.go` (eviction); `CleanupStale` writes tombstones itself |
| `PeerStore.SetPolicy(Policy)` | new; nil policy keeps today's behaviour | `pkg/node` | `pkg/daemon` (set before discovery starts) |
| `node.EnforceIPBinding`, `node.EnforceRouteBinding` | new package variables, `false` at N | `pkg/node` | the later releases flip them; tests set them |
| `wgmesh_admit_warnings_total{rule}` | new Prometheus counter (T4) | `pkg/daemon/metrics.go` | operators, the N+1 go/no-go |
| `accept-routes` (config field, `--accept-routes` flag, reload file), `Policy.AcceptRoutes` | new, receiver-side only; unset means open | `pkg/daemon/config.go`, `main.go` | `Decide` (R9) |
| `LocalNode.ObserveEndpoint(source, endpoint)` | new; the 7 observation `SetEndpoint` calls move to it | `pkg/daemon` | `dht.go`, `exchange.go` |
| Local node state file | the collision-resolved mesh IP is written | `pkg/daemon` | `initLocalNode`, `service add` |
| `daemon.ResolveCollision(loser *PeerInfo, ...)`, the `applyInterfaceAddress` seam, `PeerStore.Policy()` | added or changed by T4 (listed 2026-11-15, T11): `ResolveCollision` takes the loser instead of a `CollisionInfo`, `applyInterfaceAddress` is the variable the collision tests replace, `Policy()` returns the policy set by `SetPolicy` | `pkg/daemon`, `pkg/node` | `CheckAndResolveCollisions`, the collision tests |
| `crypto.OpenEnvelope` | invalid `KnownPeers` entries are dropped, the message is kept | `pkg/crypto` | LAN, gossip, registry, `main.go` |
| `testdata/compat/admit/v1/cases.json` | new frozen fixture | T6 | `TestAdmitCompatFixtures` |
| Envelope bytes, announcement JSON | None — no field is added | — | — |

## Inter-task Contracts

| Contract | Producing task | Consuming task(s) | Breaking? |
|----------|----------------|-------------------|-----------|
| `Candidate`, `Hand`, `Source`, `Decision`, `Offer`, `Policy`, `SetPolicy`, aliases in `peerstore_compat.go` | T1 | T2, T4, T6, T7 | No: `Update` forwards to `Offer` |
| every production claim goes through `Offer`; the guard test | T2 | T3, T7, T8, T9 | No |
| `Withdraw`, tombstones | T3 | T11 | No: additive |
| `Claim`, `ResolveClaims`, claim order, `EnforceIPBinding`, the daemon-built `Policy` (local claim, derive closure, `OnWarn`) and `wgmesh_admit_warnings_total` | T4 | T5, T6 | No for the wire and the state file; Go importers see the new signatures `DetectCollisions(ps, local)` and `ResolveCollision(loser, ...)` (2026-11-15, T11) |
| persisted resolved IP | T5 | T11 | No: the state file already has the field |
| `EnforceRouteBinding`, compat fixtures | T6 | T11 | No: both false at N |
| `ResolveEndpoint`, guessed-endpoint rank | T7 | T11 | No |
| `crypto.FilterAnnouncement` | T8 | T11 | No: more lenient |
| `LocalNode.ObserveEndpoint` | T9 | T11 | No |
| `Policy.AcceptRoutes`, `Config.AcceptRoutes`, rule R9 | T10 | T11 | No: unset means open |

## Implementation

See `tasks/README.md`. Eleven tasks in five waves (the tasks share files and run one after another on one branch): T1 the `Offer` entry point with the existing rules pinned, T2 every `Update` call site through it plus the guard test (`Remove` joins the guard in T3, when its two callers move), T3 tombstones (#854), T4 the local node in the claim table and the claim order (#856), T5 persist the resolved IP (#830), T6 mesh IP and route binding in warn mode with the compat fixtures (#864), T7 endpoint rules (#839), T8 one validation path (#875), T9 the local endpoint observations (#828), T10 route approval (#864), T11 specs, release notes and backlog rows.

## Consequences

- **Positive:** one place decides what a claim may do, so the next source (the registry, or a new transport) cannot skip a rule without failing a test; a removed peer stays removed for five minutes; a two-node collision with the local node resolves; a resolved IP survives restarts and is the one services register; a claim of a bound member's address and a route into the mesh network are visible from N and refused from N+1.
- **Negative:** the full effect on #864 needs three releases. Until N+1 the winner of a collision is still the lower key. A grinding member (Open Question 1) can still take an address. The tombstone is in memory, so a restart forgets it and one transitive mention can bring a peer back once. `Offer` takes the store lock for the length of `Decide`, which scans the peer map for claims; at the 1000-peer cap (`DefaultMaxPeers`) that is a linear pass per offer, which T1 measures with a benchmark before the tombstone and claim rules land.
- **Neutral:** `PeerStore.Update` stays in the API for tests; `registry.go` stays dormant. The local node's `MeshIP` field is written by collision resolution (`collision.go:120`) and read by discovery goroutines without a lock (`exchange.go:507`); this record reads it through a closure but does not fix the race (see Out of Scope).

## Out of Scope

- Signature-based binding of a mesh IP to the owner of a key, which closes the grinding limit (permanent: boundary: needs a per-node signing key and its distribution, the same separate decision as ADR-0001's per-sender signatures).
- Route approval bound to an advertiser key, or operator-signed route grants (permanent: boundary: announcements carry an unauthenticated public key, so a per-key rule would not hold without the signature decision above).
- A multi-candidate local endpoint state, so both addresses of a dual-egress network are announced (the second half of #828) (deferred: docs/adr/BACKLOG.md).
- `install-service` validating `--advertise-routes` at install time, item 1 of #875: a CLI check, not a discovery source (deferred: docs/adr/BACKLOG.md).
- #859, #862, #872 (route state: duplicate subnets, macOS routes, iptables re-apply): they act on reconcile's apply step, after admit (permanent: boundary: another group owns the apply step; this record only decides what enters the store).
- #850, #851, #852, #829 (liveness: handshake-based aliveness, cache expiry, relay return, sleep/wake): they decide when a peer is alive, not whether a claim is admitted (permanent: boundary: another group owns liveness; the tombstone lifetime here is its own constant, `TombstoneTTL`).
- #855, #860, #870 (reply channel overwrite, second `join` kills the node, non-atomic state files) and the unsynchronised `LocalNode.MeshIP` field: single-writer and atomic-load work (permanent: boundary: another group owns it; T5 writes the state file with the existing `saveLocalNode`).
- #865, #866, #867, #868 (centralized pipeline): `pkg/ssh` and `pkg/mesh` never touch the PeerStore (permanent: boundary: a different pipeline).
- #858 and #857 are already merged (PR #881); #849 is closed. T1 pins their behaviour and does not redo it (permanent: boundary: done).
- A replay cache or per-packet source authentication: ADR-0001 (permanent: boundary: owned by ADR-0001 and its backlog rows).

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Enforcing at N+1 refuses a node that is valid (rotated address, collision nonce, custom subnet) | Med | High | R4 is an ordering, not a gate; the compat fixtures must admit those cases in every phase; the warning counter must read zero for a release before the flip |
| Old and new nodes disagree on the collision winner during the rollout: both move, or neither moves (old node holds the lower key and an unbound address, new node holds a bound one and wins) | Med | Low | the winner order changes only at N+1; both-move converges because a mover derives an unclaimed nonce address; neither-moves is today's behaviour (the collision stays), not a regression, and T4 and T6 test both |
| A tombstone hides a peer that is back but heard only through mentions; stale cleanup would otherwise make mention-only peers vanish for five minutes in a loop | Med | Low | cleanup tombstones only peers once heard first-hand (`HeardFirstHand`); GOODBYE and health eviction always tombstone; its own first-hand packet clears it at once |
| `Decide` under the store lock slows discovery at 1000 peers | Low | Med | T1 benchmarks it at the cap; the claim index is built once per offer, not per rule |
| A new source is added and calls `Update` | Med | High | `TestNoDirectPeerStoreMutation` fails the build; the receiver-name heuristic is the guard's limit (T2 states it and mutation-tests it) |
| Offering `dht.go:1068` as first hand (its label stays, decision 9) refreshes `LastSeen` of a peer that answered a punch | Low | Low | that is the intended behaviour (it just answered, and `exchange.go:436` already stored it first-hand); T2 has a test |
| `ObserveEndpoint` delays a real endpoint change until the third identical STUN answer (about 2 minutes after the first differing one at the 60-second refresh) | Med | Low | a peer reflection applies at once, and so does any observation while the current endpoint is empty or unspecified (`0.0.0.0`), so start-up is not delayed; only STUN-only changes of a usable endpoint wait |

## Rollback

N is additive on the read side: `Offer` applies the same merge as `Update`, and every binding rule only counts and logs, with one visible exception: from N the local node takes part in collisions (#856), so a node that loses one re-derives its mesh IP and its interface address changes at runtime (T11 release note). Reverting N restores direct `Update` calls; nothing persisted changes shape except that the state file may hold a collision-resolved `MeshIP`, which the old `initLocalNode` keeps when it is inside the subnet, so a revert leaves that address in place (it was valid when written). Tombstones are memory only. A revert after N+1 returns the mesh IP rules to warn mode by flipping the variable back; no data is rewritten. `testdata/compat/admit/v1` stays in the repository, as frozen fixtures are never deleted.

## Follow-ups

- [ ] N+1: set `EnforceIPBinding = true`, with a release note and the warning counter read as the go/no-go (deferred: docs/adr/BACKLOG.md)
- [ ] N+2: set `EnforceRouteBinding = true`, with a release note (deferred: docs/adr/BACKLOG.md)

## Amendments

- 2026-11-15 (T11): decision 1 now carries the measured grinding number (nonce 0 only); Rule R5 is read as "a host route equal to a member's mesh IP" and its IP refusal needs a claimant whose own claim is unbound; Rule R7 lists the `interface` source and notes the `stun-resume` source that ADR-0004 M5 adds (M confirmed 2026-11-15 22:46). The text of each is changed in place and marked.
- 2026-11-15 (T11): Go importers see two signature changes, `DetectCollisions(ps, local)` and `ResolveCollision(loser, ...)`; "Breaking? No" below means no wire and no persisted-format change.
