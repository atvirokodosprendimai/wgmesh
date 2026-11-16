# ADR-0007: Give every long-lived collection and spawned process one bound and one owner

**Status:** Accepted
**Date:** 2026-11-15
**Owner:** wgmesh maintainers
**Spec:** None — no spec stage; `eidos/spec - discovery dht core - bittorent mainline dht announce query persistence and endpoint detection.md` is amended by T7
**Cross-references:** `docs/adr/ADR-0002-admit-step/ADR-0002-admit-step.md` (governs `pkg/discovery/dht.go` and `exchange.go`, which this record edits in three functions each; read its Out of Scope entries on liveness before T2), `docs/adr/ADR-0001-envelope-aad/ADR-0001-envelope-aad.md` (governs `exchange.go`), `docs/adr/BACKLOG.md`; issues #874, #873, #836, #833 (open on 2026-11-15), #837 and #827 (closed on 2026-11-15); open PR #831 (nycterent, mesh IP stability)
**Governs:** `pkg/wireguard/*.go`, `pkg/daemon/daemon.go`, `main.go`
**Enforced-by:** None — the check is `TestEveryMapFieldHasABound` in `pkg/daemon/bounded_guard_test.go`, which does not exist until T4 adds it red-first; it fails when a map-typed struct field in `daemon.go`, `dht.go`, `exchange.go`, `gossip.go` or `lan.go` has no row naming how it is bounded
**Invalidates:** none — checked (`adr-state` on 2026-11-15 reports ADR-0001 and ADR-0002 only; ADR-0002 governs `dht.go`, `exchange.go`, `lan.go`, `gossip.go`, `registry.go`, `cache.go`, `collision.go`, `config.go` and `pkg/node`; this record edits `dht.go` in `markContacted`, `setControlEndpoint`, `recordRendezvousAttempt`, `checkStaleHandshakes`, `getPeerHandshakeTS` and three `Announce` calls, and `exchange.go` in `getLatestHandshake`, none of which ADR-0002 changes, and its tasks run after ADR-0002's T11)
**Served-path change:** A daemon that runs for weeks no longer keeps one map entry per address it ever contacted on the DHT, and it reads WireGuard state with about one `wg show` fork per reconcile tick instead of about one and a half per second; an unknown subcommand prints usage and exits 2 and `--version` is honoured only before the subcommand.

## Decisions

M answered the seven questions of the first draft on 2026-11-15 (harness question): "Pagal visas rekomendacijas, ADR po ADR (Recommended)". Every question is decided as its recommendation, except the PR, which is recorded as a recommendation only.

1. **Share one snapshot, no constant changes** (M 2026-11-15, as recommended): one `wg show <iface> dump` per reconcile tick, read by the probe loop and the health check at most 5 seconds old (`HandshakeStaleAfter` is 150 seconds, `daemon.go:34`); `MeshProbeInterval` and `MeshProbeFailLimit` stay.
2. **10-second timeout for every `wg` call through `pkg/wireguard`** (M 2026-11-15, as recommended): a constant, no flag.
3. **DHT contact map: 60-second window and a hard cap of 8192 distinct addresses** (M 2026-11-15, as recommended): past the cap `markContacted` returns false (fail-closed, the shape of `lastPacketLog`, `exchange.go:1267`); 8192 is not measured on a public DHT.
4. **Usage and exit 2 for no subcommand or an unknown first word** (M 2026-11-15, as recommended): when the first argument does not start with `-` and is not a known subcommand; arguments starting with `-` still reach the legacy centralized flags; the legacy mode stays.
5. **#873 items 3 to 6 are deferred** (M 2026-11-15, as recommended): one backlog row group, no tasks here.
6. **PR #831: recommended, not decided** (M 2026-11-15: every question as recommended except this one): recommended is to close it as superseded with thanks to its author, because main already holds the behaviour (see Context, "PR #831"). Closing is an outward GitHub action that M decides separately when it is time; nothing is done in this ADR and no task depends on it.
7. **No coverage target or CI floor now** (M 2026-11-15, as recommended): T7 adds the first tests of the `wg` wrappers and the numbers are re-read after it; `AGENTS.md:132` says ">80% coverage on new code", not per package.

## Open Questions for M

None. Question 6 stays a recommendation for a separate decision by M, outside this record's work.

## Context

**The pipe.** The daemon is `discover -> PeerStore -> reconcile (derive desired, diff actual, apply wg, ip, iptables, files)`, with side loops (probe, health, stale cleanup, DHT rendezvous) that read the same state again. In ETL terms each side loop is a long-lived reader that keeps its own bookkeeping and shells out to `wg` for its extract. The missing step is **bound**: a stateless rule that says, for each collection, how its entries leave, and for each child process, who may start it, how often, and when it is killed. Today each author chose a different answer (a sweep against the active set, a replaced map, a TTL, a hard cap, or nothing).

**Class audit** (2026-11-15, read at `b8b83f3`). Class: *every long-lived map in the daemon and the discovery layers that is keyed by peer or address and every code path that forks `wg`, without a stated bound or a single owner of its release.*

| Command | Count | Members |
|---|---|---|
| `grep -nE "^\s+\w+\s+map\[" pkg/daemon/daemon.go pkg/discovery/dht.go pkg/discovery/exchange.go pkg/discovery/gossip.go pkg/discovery/lan.go \| wc -l` | 19 | 8 `Daemon` fields (`daemon.go:54-69`), 3 `DHTDiscovery` fields (`dht.go:75-77`), 5 `PeerExchange` fields (`exchange.go:84-94`), 2 inside `rendezvousState` (`exchange.go:64-65`), 1 inside `desiredPeerConfig` (`daemon.go:563`) |
| `grep -rnE "wireguard\.(GetPeers\|GetLatestHandshakes\|GetPeerTransfers)\(" --include=*.go . \| grep -v _test.go \| wc -l` | 8 | `daemon.go:567`, `:774`, `:1068`, `:1285`, `:1289`; `exchange.go:902`; `dht.go:1136`, `:1298` |
| `grep -rnE "exec\.Command\(" --include=*.go . \| grep -v _test.go \| wc -l` | 8 | `pkg/wireguard/apply.go` (5), `keys.go` (2), `pkg/daemon/executor.go:46` (1) |
| `grep -rnE "cmdExecutor\.Command\(" --include=*.go . \| grep -v _test.go \| wc -l` | 37 | every `ip`, `iptables`, `systemctl`, `wg-quick`-style call; all reach `executor.go:46` |

Of the 19 maps, 15 are bounded today, each in its own way: replaced on every reconcile (`relayRoutes`, `directStableCycles`), deleted for peers that left (`lastAppliedPeerConfigs`, `daemon.go:782`, `:817`), swept against the active peer set every 20 or 1 seconds (`peerHealthFailures`, `lastPeerTransferTotal`, `daemon.go:1334`; `probeSessions`, `probeFailures`, `cleanupProbeSessions`, `daemon.go:1206-1221`), swept by age (`rendezvousSessions`, `rendezvousStarts`, `activePunches`, `exchange.go:705-720`, on the next offer, so an idle introducer keeps its entries until one arrives), swept by age with a hard cap (`lastPacketLog`, `exchange.go:1252-1271`, the pattern this record copies), paired set and clear (`pendingReplies`, `exchange.go:551-552`), or dying with their owner (`offers`, `endpoints`, `allowed`). **Four are not bounded:**

- `contactedPeers` (`dht.go:75`, written at `:1470`, no `delete` anywhere in the repository: `grep -n "delete(d.contactedPeers" pkg/discovery/*.go` is empty). One entry per `ip:port` ever passed to `markContacted`, whose callers dedupe DHT peers (`dht.go:825`), exchange targets (`:984`) and introducers (`:1003`, `:1039`) over windows of 3, 20 and 60 seconds. This is #874's first item; the line numbers of the issue (1460-1471) are still right at `b8b83f3`.
- `controlPeers` (`dht.go:76`, written at `:1166`, never deleted): one entry per public key that ever sent a control endpoint, kept after the peer leaves the store.
- `rendezvousBackoff` (`dht.go:77`, `recordRendezvousAttempt`, `:918-946`): deleted only on a successful rendezvous, so a key that never succeeds and then leaves the store stays for the life of the process.
- `temporaryOffline` (`daemon.go:69`, `markTemporarilyOffline`, `clearTemporarilyOffline`, `isTemporarilyOffline`): an entry is deleted when the same key is asked about after its 30 seconds (`TemporaryOfflineTTL`, `daemon.go:39`), or cleared on a good probe; a key evicted and never seen again stays.

**The process side of #874.** The mesh probe loop ticks every second (`MeshProbeInterval`, `daemon.go:35`) and each tick forks `wg show <iface> latest-handshakes` once (`probePeersOverMesh`, `daemon.go:1068`). Reconcile, every 5 seconds, forks it again for the desired-config build (`daemon.go:567`) and forks `wg show <iface> peers` (`:774`); the health check, every 20 seconds, forks `latest-handshakes` and `transfer` (`:1285`, `:1289`). That is 60 + 24 + 6 = 90 forks a minute from three loops at steady state, counted from the call sites, **not measured** on a running node (T3 records the measured number). On top, `checkStaleHandshakes` (every 10 seconds, `dht.go:888`) reaches `getPeerHandshakeTS` (`dht.go:1136`) once per candidate peer, and `waitForHandshake` forks every 250 milliseconds during a punch (`HandshakePollInterval`, `exchange.go:31`, `:902`). All three sub-commands print different columns of the same `wg show <iface> dump` (`parseWGDump`, `pkg/wireguard/config.go:63`, already parses the peer lines of the dump for the SSH path and drops the handshake and transfer columns). The `wg` child has no deadline: all five calls in `apply.go` and both in `keys.go` use `exec.Command`, not `exec.CommandContext`.

**The other issues of the group.**

- #837 (analytics `flush()` ignores the `Close` error): closed on 2026-11-15, and main already checks it (`pkg/analytics/logger.go:129`, read). Nothing to do.
- #827 (mesh IP stable across option changes): closed on 2026-11-15; main holds the fix in a different form (commits `8cb80b7`, `629555c`, `d1d2a46`, `b01f29b`: `daemon.MeshIPKept` at `pkg/daemon/daemon.go:314`, used by `initLocalNode` and by `deriveMeshIPForService`, `service.go:471`, which also reads the right `wg_pubkey` tag). Not a member of the class (it is mesh IP identity, owned by ADR-0002 T4 to T6).
- **PR #831** adds a shared `crypto.MeshIPInSubnet`, `resolveLoadedNodeIPs`, the `wg_pubkey` tag fix, `--state-dir` threading in `service.go` and 14 tests; read through `gh pr diff 831`, not run. Main has the same behaviour under other names (`MeshIPKept`), and `service.go` on main already takes `stateDir` and the `wg_pubkey` tag. The overlap is therefore a duplicate, not an input to this decision; the one item the PR notes and main does not close is the collision-nonce IP kept only in memory (#830), which ADR-0002 T5 persists. Decision 6 records the PR as a recommendation only.
- #836 (move from `Announce` to `AnnounceTraversal`): the issue says the migration "is not a mechanical rename". In the vendored version it is: `anacrolix/dht/v2@v2.23.0` `announce.go:76-90` defines `Announce(h, port, implied, opts...)` as `AnnounceTraversal(h, opts...)` with `AnnouncePeer(AnnouncePeerOpts{Port, ImpliedPort})` prepended when `port != 0 || implied` (read). The three call sites (`dht.go:544`, `:710`, `:787`) pass `implied = false`; two have port 0 and one has a real port. There is no `//nolint` in the repository and no `.golangci.yml` (`grep -rn nolint --include=*.go .` is empty), so the "remove the nolint" half of the issue is already true; whether CI runs staticcheck's SA1019 was not checked.
- #873 (CLI): items 1, 2 and 3 verified at `main.go:38` (the version scan covers every argument), `main.go:96-145` (the fall-through to legacy mode and its default `defaultStateDir/mesh-state.json`) and `main.go:882` (`mesh list` defaults to `./mesh-state.json`); item 5 at `main.go:837` (the message is printed with no `is-active` check; `pkg/daemon/systemd.go:216` has a helper that runs it); item 6 at `daemon.go:1706-1712` (`reloadConfig` stores the level and `configureLogging`, `daemon.go:159`, is not called from it). Item 4 (`status` exit 0 with no daemon) was read at `main.go:597-640` and not run.
- #833 (coverage), measured 2026-11-15 with `go test -cover` at `b8b83f3`: `pkg/wireguard` 20.4%, `pkg/discovery` 33.4%, `pkg/daemon` 37.9%, `pkg/mesh` 44.6%, `pkg/ssh` 17.6%, `pkg/analytics` 64.7%, `cmd/analytics-dashboard` 0.0%. The table in the issue (9.1%, 19.5%, 30.1%, 34.8%, "0 tests" for `pkg/ssh` and analytics) is stale. In `pkg/wireguard` every function of `apply.go`, `keys.go`, `persist.go` and `convert.go` is at 0.0% (`go tool cover -func`); those files have no `_test.go`, and `apply.go`'s only seam is the package variable `wgPath` (`apply.go:21`).

**Debt pulled in.** `adr-debt docs/adr` on 2026-11-15 lists ADR-0001 and ADR-0002 rows only; none concerns this class. Nothing is pulled in.

## Existing Primitives Audit

- `lastPacketLog` throttle (`exchange.go:1252-1271`): the pattern to copy for address-keyed dedup (age sweep once per window above a size threshold, plus a hard cap). Reused by shape, not by code, because the value types differ.
- The active-set sweep of `checkPeerHealth` and `cleanupProbeSessions` (`daemon.go:1334`, `:1206`): the pattern for pubkey-keyed state; `DHTDiscovery` already holds `d.peerStore`, so its sweep is the same loop against `peerStore.GetAll()` (`pkg/node/store.go:276`).
- `wgPath` (`apply.go:21`, `daemon/helpers.go:22`): the existing seam for the binary; T1 reuses it for tests and adds the deadline beside it.
- `parseWGDump` (`config.go:63`): reused for the line format; T1 extends the parse to the handshake and transfer columns in a new type, because `peersEqual` (`config.go:155`) compares named fields, so `Peer` could grow, but a separate `State` type leaves the SSH path untouched.
- `cmdExecutor` and `RealCommand` (`executor.go:46`): the single construct point of 37 call sites; left unchanged (Decision 2 and Out of Scope: `systemctl` may take long).
- `markTemporarilyOffline` (`daemon.go`): kept; ADR-0002 keeps it too ("a 30-second reconcile flag, not a tombstone").
- Nothing else bounds a keyed map: `grep -rn "maxLogThrottleEntries\|DefaultMaxPeers" --include=*.go pkg | grep -v _test` finds two constants only (`exchange.go`, `pkg/node/store.go`), one per structure, no shared helper.

## Decision

1. **One rule per key kind, written down and guarded.** Every map field of the long-lived structs belongs to exactly one row:

| Key kind | Rule | Members |
|---|---|---|
| address (`ip:port`) | dedup window, sweep by age once per window when the map is above 256 entries, hard cap that fails closed | `contactedPeers` |
| public key | swept against the store membership (`peerStore.Get` per key) on an existing tick; an entry for a key the store no longer holds is deleted | `controlPeers`, `rendezvousBackoff` |
| public key, lazy | a sweep in the existing 1-minute `staleCleanupLoop` deletes entries past their deadline | `temporaryOffline` |
| already bounded | listed in the guard table with how (replaced per tick, active-set sweep, age sweep, hard cap, session-local, paired set and clear) | the other 15 |

   The table is a Go test, `TestEveryMapFieldHasABound`, which parses the five files with `go/parser` and fails on a map-typed field without a row, and on a row whose field no longer exists. It is the same shape as ADR-0002's `TestNoDirectPeerStoreMutation`: a heuristic over field declarations, so a map hidden in a nested type or a slice of maps is outside it (T4 states this and mutation-tests it).

   Amendment 2026-11-16 (coordinator, deciding a small detail by this ADR's own scope "the long-running peer path"): the guard parses seven files, the five above plus `pkg/node/store.go` (`peers`, `tombstones`, `ipHeldLogged`, added by ADR-0002) and `pkg/discovery/replay_cache.go` (`seen`, ADR-0001), because both hold long-lived maps on the peer path. The counts 19 and 15 above are those of `b8b83f3`; the guard's table is the count from T4 on.

   Amendment 2026-11-16 (merge with ADR-0003/0004/0006): on main the reconcile tick reads the kernel through `wireguard.Dump` (ADR-0003 `observeKernel`), not through `applyDesiredPeerConfigs`/`lastAppliedPeerConfigs` named in Decisions 1 and 5; `Dump` runs under the `ReadState` slot lock with the same deadline and stores the snapshot, so Decision 5's "one fork per tick, probe and health on a 5-second snapshot" is met through `Dump`. The guard table lost `lastAppliedPeerConfigs` and gained the ADR-0003/0004 map fields (23 rows became 35).
2. **`contactedPeers`**: constants `ContactDedupWindow = 60 * time.Second` (the longest interval any caller passes, `dht.go:825`) and `MaxContactedPeers = 8192`. `markContacted` sweeps entries older than the window when `len > 256` and at most once per window, then, at the cap, returns false (Decision 3). A test pins that no caller passes an interval above the window, so the sweep cannot delete an entry a caller still needs.
3. **`controlPeers` and `rendezvousBackoff`**: one method `sweepPeerKeyed` called first in `checkStaleHandshakes` (already driven by the 10-second `staleTicker`, `dht.go:841`, `:857`). A peer that comes back starts with no backoff, which is what a new peer has today.
4. **`temporaryOffline`**: a method `sweepTemporaryOffline` called in the `staleCleanupLoop` tick.
5. **One `wg show` snapshot, one owner.** `pkg/wireguard` gains `ReadState(iface string, maxAge time.Duration) (*State, error)`: it forks `wg show <iface> dump` once, parses peers with their endpoint, latest handshake and transfer counters, and keeps the last good result with its time under one mutex; a call with `maxAge` newer than the stored result returns it without forking, `maxAge 0` always forks and refreshes the store, an error is never stored. `GetPeers`, `GetLatestHandshakes` and `GetPeerTransfers` stay as thin readers over `ReadState(iface, 0)` so no test or caller breaks before T2. The staleness each caller accepts, written next to the call:

| Caller | `maxAge` | Why |
|---|---|---|
| `applyDesiredPeerConfigs` (the peer list before removals) and `buildDesiredPeerConfigs`, one `ReadState(iface, 0)` per reconcile tick | 0 | decisions about what to remove need the current list; this one fork refreshes the store for everyone else |
| `probePeersOverMesh`, `checkPeerHealth` | `ReconcileInterval` (5 s) | they compare handshakes with a 150-second threshold (`HandshakeStaleAfter`); 5-second-old data cannot change a decision by more than the tolerance the threshold already has |
| `getPeerHandshakeTS`, `getLatestHandshake` (the punch wait) | 0 for the wait; `ReconcileInterval` for the per-peer check in `tryRendezvousForPeer` | the wait compares against a baseline taken just before the punch and must see a new handshake within 250 ms |
| introducer ranking (`dht.go:1395` after ADR-0002) | `ReconcileInterval` | it already fetches once for all candidates |

   Steady state: one fork per 5-second reconcile tick, 12 a minute instead of 90 (computed from the call sites, not measured; T3 measures, and expect up to about 24 because a snapshot exactly one reconcile period old can be too old for a probe, stale-check or health tick that lands just before the next reconcile).
6. **A deadline on every `wg` call made through `pkg/wireguard`**: one unexported helper `runWG(stdin io.Reader, args ...string)` replaces the seven `exec.Command` sites and takes an optional stdin (`SetPeer` passes the preshared key on `/dev/stdin`, `apply.go:149`) (`apply.go` 5, `keys.go` 2) and runs `exec.CommandContext` with `wgCommandTimeout = 10 * time.Second` (Decision 2); on expiry the child is killed and the error names the sub-command and the timeout. `cmdExecutor` is not changed.
7. **The small items, as tasks in this record because they share files with the class tasks** (`dht.go`, `daemon.go`, `main.go`): T5 migrates the three `Announce` calls (#836); T6 prints usage for an unknown first word and honours `--version` only before the subcommand (#873 items 1 and 2, Decision 4); T7 adds the first tests of the `wg` wrappers through the T1 seam (#833, no target); T8 reads the join secret for `status`, `qr`, `install-service` and `wgmesh service` the way `join` does, flag then `WGMESH_SECRET` then `WGMESH_SECRET_FILE` (#891, added 2026-11-15 after the draft; it shares `main.go` with T6). Items 3 to 6 of #873, the rest of #833, #837, #827 and PR #831 are dispositions, not work (Out of Scope).

**Staging.** None of these changes a wire byte, a persisted file, or a flag. The two-release rule is not applied because there is no old reader to keep working: a dedup map that forgets after 60 seconds, a store sweep, and a snapshot read are internal; the 10-second child deadline changes behaviour only for a call that would otherwise never return; the cap acts above 8192 distinct addresses in a minute. T6 changes an exit code for input that is an error today (usage and 2 instead of a failed state load and 1). Each is reverted by reverting its commit. What would make this decision fail: (a) a new long-lived map without a row, caught by the guard test; (b) a sweep that deletes an entry a caller still needs, caught by the window test and the store-sweep test; (c) a stale snapshot changing a probe or health decision, caught by T2's decision tests with a handshake that crosses the threshold between two reads; (d) a hung `wg` blocking a loop, caught by T1's deadline test.

## Alternatives Considered

- **Fix #874's two lines in place** (delete old `contactedPeers` entries, lengthen the probe interval). Rejected: it leaves `controlPeers`, `rendezvousBackoff` and `temporaryOffline` with the same defect, found only by the audit above, and the next map added without a bound would repeat it; a lengthened probe interval changes failure detection (Decision 1).
- **A shared generic `TTLMap` type in a new package.** Rejected: four members, three different release rules (age with a cap, store membership, deadline), no second user for the type; the age-sweep-plus-cap shape already exists once (`lastPacketLog`) and a second copy of ten lines is cheaper than a package. Revisit when a fifth member appears.
- **Replace the `wg show` forks with a netlink or `wgctrl` client.** Rejected here: a new dependency on the key-handling path (the project's list of areas that need review before change, `CLAUDE.md`), and Windows or macOS userspace builds use the `wg` binary; a snapshot of the binary's output removes the churn without a new dependency. It is the real fix for the process cost and its own decision.
- **Change the probe interval to 5 seconds instead of sharing a snapshot.** Considered under Decision 1 and not chosen: it slows failure detection fivefold to save forks that a read-through snapshot saves without touching it.
- **Put the sweeps in a new background goroutine.** Rejected: three existing tickers (the 10-second stale check, the 1-minute stale cleanup) and the call path of `markContacted` already run at the right cadence; a new goroutine needs its own shutdown and test.

## Component / Boundary Impact

- `pkg/wireguard`: becomes the only package that forks `wg` for state and holds its last snapshot; one reason to change (how `wg`'s output is read and bounded). It imports nothing new. `pkg/ssh`-based `GetCurrentConfig` is untouched.
- `pkg/daemon`: loses three of its five direct `wg show` forks per cycle by reading the snapshot, gains `sweepTemporaryOffline` and the guard test. No new import.
- `pkg/discovery`: `DHTDiscovery` bounds its own three maps and reads the snapshot through `pkg/wireguard`; `discovery` still does not need to import `node`. ADR-0002's boundary (admit decides claims) is untouched: nothing here decides whether a claim enters the PeerStore.
- `main.go`: usage and `--version` scan only; no flag is added.

## Wiring & Contract Changes

| Surface | Change | Producer | Consumer(s) |
|---------|--------|----------|-------------|
| `wireguard.ReadState(iface, maxAge)`, `wireguard.State` | new | `pkg/wireguard/state.go` | `pkg/daemon/daemon.go`, `pkg/discovery/dht.go`, `exchange.go` |
| `wireguard.SetWGPath(path) (restore func())` | new, documented test-only | `pkg/wireguard` | daemon and discovery tests (fork counting) |
| `wgCommandTimeout` (10 s), `runWG` | new, unexported | `pkg/wireguard` | the seven former `exec.Command` sites |
| `ContactDedupWindow`, `MaxContactedPeers` | new constants | `pkg/discovery/dht.go` | `markContacted` |
| `sweepPeerKeyed`, `sweepTemporaryOffline` | new methods, no new goroutine | `dht.go`, `daemon.go` | `checkStaleHandshakes`, `staleCleanupLoop` |
| `TestEveryMapFieldHasABound` | new guard | `pkg/daemon/bounded_guard_test.go` | the build |
| `wgmesh <unknown>` and bare `wgmesh` | usage on stderr, exit 2 (T6) | `main.go` | operators, scripts that relied on legacy mode with no flag |
| join-secret lookup (`WGMESH_SECRET`, `WGMESH_SECRET_FILE`) | `status`, `qr`, `install-service`, `wgmesh service` read it as `join` does (T8); no new flag or variable | `main.go`, `service.go` | operators on a packaged host |
| Wire, announcement JSON, state files, flags, config | None — no change | — | — |

## Inter-task Contracts

| Contract | Producing task | Consuming task(s) | Breaking? |
|----------|----------------|-------------------|-----------|
| `ReadState`, `State`, `SetWGPath`, `runWG` | T1 | T2, T3, T7 | No: the old `Get*` functions forward |
| `ContactDedupWindow`, `MaxContactedPeers`, `sweepPeerKeyed` | T2 | T4 | No: additive |
| every non-test `wireguard.Get*` caller reads through `ReadState` with the table's `maxAge` | T3 | T4 | No |
| `bounded_guard_test.go` table | T4 | later maps | No: a new map must add a row |

## Implementation

See `tasks/README.md`. Eight tasks in four waves: T1 the `wg` reader, deadline and test seam (`pkg/wireguard` only, can start before ADR-0002 lands); T2 bound the three DHT maps; T3 move the callers to the snapshot; T4 `temporaryOffline` sweep and the guard test; T5 `AnnounceTraversal` (#836); T6 CLI usage and `--version` (#873); T7 first tests of the `wg` wrappers (#833); T8 one join-secret lookup (#891). T2 to T6 and T8 edit files that ADR-0002's T2 to T10 edit (`dht.go`, `exchange.go`, `daemon.go`, `main.go`) and start after ADR-0002's T11.

## Consequences

- **Positive:** a long-lived daemon's memory no longer grows with the number of distinct addresses or evicted keys it has seen; `wg` forks fall from about 90 to about 12 a minute at steady state; a hung `wg` stops costing a whole loop; a new map without a bound fails the build; the first tests exist for the `wg` wrappers.
- **Negative:** a snapshot read can be up to 5 seconds old for the probe and health decisions (the thresholds they feed are 150 seconds); a peer that returns after its entries were swept restarts its rendezvous backoff at the minimum; above 8192 distinct addresses in a minute the DHT contact loop skips new ones. The guard is a field-declaration heuristic.
- **Neutral:** `Update` and ADR-0002's admit step are untouched; `registry.go` stays dormant; the 37 `cmdExecutor` sites keep no deadline.

## Out of Scope

- #837 (analytics `Close` error): already fixed on main at `pkg/analytics/logger.go:129`, issue closed (permanent: fact: the `Close` error is checked; citation: file `pkg/analytics/logger.go:129`).
- #827 and PR #831 (mesh IP stability): main holds the behaviour as `MeshIPKept`; the collision-nonce IP persistence is ADR-0002 T5 (permanent: boundary: owned by ADR-0002 and by M's separate decision on the PR, Decision 6).
- #873 items 3 to 6 (`mesh list` default path, `status` config-only message, `install-service` `is-active` check, SIGHUP `log-level` applied) (deferred: docs/adr/BACKLOG.md).
- #833 beyond the `wg` wrappers: tests for `pkg/discovery` (`registry.go` and `init.go` have no test file; `registry.go` is dormant, see ADR-0002), `pkg/ssh/client.go`, `cmd/analytics-dashboard`, and any coverage target or CI floor (deferred: docs/adr/BACKLOG.md).
- A deadline for the 37 `cmdExecutor` calls (`ip`, `iptables`, `systemctl`): `systemctl start` can legitimately take longer than 10 seconds, so one number would be wrong for them (deferred: docs/adr/BACKLOG.md).
- Replacing the `wg` binary reads with a netlink or `wgctrl` client (permanent: boundary: a new dependency on the key-handling path, its own decision).
- Bounding collections in `pkg/privacy`, `pkg/pilot`, `pkg/referral`, `pkg/promo`, `pkg/nurture`, `pkg/mesh` (permanent: boundary: the audit covers the daemon and discovery layers; those packages are not part of the long-running peer path, and `grep` of them was read only for map field names, not for pruning).
- Whether a peer, once swept, should also lose its introducer or route state: peer state is ADR-0002's (permanent: boundary: owned by ADR-0002).

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| The sweep deletes a dedup entry a caller still needs, so a peer is contacted twice inside its interval | Low | Low | the window equals the longest caller interval and a test pins every caller's interval to it |
| A snapshot up to 5 seconds old makes a probe or health decision wrong at the threshold | Low | Med | thresholds are 150 seconds; T3 has a decision test where a handshake crosses the threshold between two reads, and reconcile reads with `maxAge 0` |
| The 10-second deadline kills a slow but healthy `wg set` on a loaded host | Low | Med | one constant, logged with the sub-command; M may raise it later; `wg show` and `wg set` on a local interface return in milliseconds |
| `ReadState` caches one interface only by name; a second interface (tests, a future multi-interface daemon) shares a slot | Low | Low | the store is keyed by interface name (T1 test) |
| T2 to T6 conflict with ADR-0002 tasks on `dht.go`, `exchange.go`, `daemon.go`, `main.go` | High if run together | Med | they start after ADR-0002's T11; only T1 and T7 (`pkg/wireguard`) may run earlier |
| The guard test misses a map declared in a nested or embedded type | Med | Low | T4 states the limit and mutation-tests the declared case |

## Rollback

Every task is an additive or local change with no persisted or wire effect. Reverting T2 to T4 restores the unbounded maps (memory growth returns, nothing else); reverting T3 restores direct forks; reverting T1 restores `exec.Command` and drops the seam, after T3 is reverted first because it consumes `ReadState`; reverting T6 restores the fall-through to legacy mode. The compatibility fixtures and state files are not touched, so there is nothing to migrate back.

## Follow-ups

- [ ] #873 items 3 to 6, one row each (deferred: docs/adr/BACKLOG.md)
- [ ] Deadline for `cmdExecutor` calls, after the `systemctl` case is split out (deferred: docs/adr/BACKLOG.md)
- [ ] Coverage of `pkg/discovery`, `pkg/ssh`, `cmd/analytics-dashboard` and any target, after T7's numbers (deferred: docs/adr/BACKLOG.md)
