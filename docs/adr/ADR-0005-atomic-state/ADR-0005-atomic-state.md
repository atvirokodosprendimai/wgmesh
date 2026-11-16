# ADR-0005: Give every piece of shared state one owner: atomic state files, exclusive host claims, per-caller replies, a locked local node

**Status:** Accepted
**Date:** 2026-11-15
**Owner:** wgmesh maintainers
**Spec:** None — no spec stage; `eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md`, `eidos/spec - daemon lifecycle - secret-derived identity with interface setup and hot-reload.md`, `eidos/spec - rpc - unix socket json-rpc server and client for peer and daemon queries.md` and `eidos/spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md` are amended by T11
**Cross-references:** issues #870, #860, #855 (open on 2026-11-15), #863 (closed on 2026-11-15: PR #884 fixed LAN and gossip; this record covers what #884 left, see Context); #857 (merged, PR #881, only read); `docs/adr/ADR-0002-admit-step/ADR-0002-admit-step.md` (Accepted; its Out of Scope names #855, #860, #870 and the unsynchronised `LocalNode.MeshIP` as this group's work, and its T5 persists the collision-resolved mesh IP through `saveLocalNode`); `docs/adr/ADR-0001-envelope-aad/ADR-0001-envelope-aad.md` (governs `pkg/discovery/exchange.go` message handling, whose order this record keeps); `docs/adr/BACKLOG.md`
**Governs:** `pkg/atomicfile/*.go`, `pkg/flock/*.go`, `pkg/daemon/helpers.go`, `pkg/daemon/daemon.go`, `pkg/daemon/cache.go`, `pkg/mesh/mesh.go`, `pkg/mesh/account.go`, `pkg/mesh/services.go`, `pkg/promo/store.go`, `pkg/rpc/server.go`, `pkg/node/types.go`, `pkg/discovery/exchange.go`, `pkg/discovery/dht.go`, `main.go`
**Enforced-by:** None — the checks are `TestNoInPlaceStateWrite` in `pkg/atomicfile/guard_test.go` (does not exist until T3 adds it red-first; it fails when a production file calls `os.WriteFile` a different number of times than its allowlist entry says) and `TestLocalNodeFieldsNotWrittenDirectly` in `pkg/daemon/localnode_guard_test.go` (T8; it fails when a production file assigns `MeshIP`, `MeshIPv6`, `NATType`, `Introducer` or `RoutableNetworks` of a local-node receiver, or `WGPubKey`, `WGPrivateKey` or `Hostname` outside `initLocalNode` and `loadLocalNode`, with the fields unexported so the compiler is the first guard; on a local-node receiver outside the owner's setters)
**Invalidates:** none — checked (`adr-state` on 2026-11-15 reports ADR-0001 and ADR-0002 as the only governing records; `pkg/discovery/exchange.go` is already contested between them and this record adds a third reader of it, see Wiring; ADR-0002 decision 10 (T9 on `daemon.LocalNode`) is followed by T7, which moves that type, see M4)
**Served-path change:** A state file (local node identity, peer cache, centralized mesh state, the init secret) is replaced whole or not at all, so a crash or a full disk no longer leaves a truncated identity; a second `wgmesh join` on a host that already runs the same interface, or the same RPC socket, exits with a message instead of unlinking the socket and resetting the interface of the running node; every concurrent exchange to one address receives the reply; the local node's mesh IP, NAT type and routes are read and written under one lock; and, after the staged release, a corrupt identity file stops the daemon instead of silently creating a new key.

## Decisions

M answered the six open questions of the first draft and one extra question on 2026-11-15 (harness questions). Recorded here, dated, in M's words. They are numbered M1 to M6 so they are not confused with the numbered design items under "Decision" below.
M1. **A corrupt identity file is staged** (M 2026-11-15: "Pagal visas rekomendacijas, ADR po ADR (Recommended)"; recommended option b). Release N moves `<iface>.json` to `<iface>.json.corrupt-<unix>` (never overwritten, recoverable), logs loudly and continues as today; release N+1 sets `RefuseCorruptState` and refuses to start. Today an unreadable file is treated as absent and a new key and mesh IP are written over it (`pkg/daemon/daemon.go:362-401`, `:404-435`).

M2. **A second `wgmesh join` on the same interface exits 1** (same answer; option a), before touching the socket or the interface, with a message that names the lock file and the running pid. It protects only against a running daemon of a release that takes the lock.

M3. **Two interfaces sharing the default RPC socket path: the second daemon runs without RPC and warns** (same answer; option a). `NewServer` unlinks whatever is there (`pkg/rpc/server.go:70`) and the default path has no interface in it (`:373-391`); `--socket-path` gives the second daemon its own, and `main.go:459-465` already treats a `createRPCServer` error as "continue without RPC". The default path is not changed.

M4. **One `LocalNode` type, by alias, moved after ADR-0002** (same answer; option a, and the extra question M 2026-11-15: "Pataisyti T9: ObserveEndpoint ant daemon.LocalNode (Recommended)"). `pkg/node/types.go:34` and `pkg/daemon/daemon.go:102` hold identical structs and `grep -rn "node\.LocalNode" --include=*.go .` lists nothing at `b8b83f3`. ADR-0002 decision 10 puts `ObserveEndpoint` on `daemon.LocalNode` and leaves the type merge to this record. T7 therefore runs after ADR-0002 T9: it moves `daemon.LocalNode` with everything T9 put on it into `pkg/node/types.go`, replacing the unused struct there, and makes `daemon.LocalNode` an alias in `peerstore_compat.go` (the way `PeerInfo` and `PeerStore` are).

M5. **Concurrent exchanges to one address: every registered caller receives the reply** (same answer; option a), each removes only its own registration, no wire change.

M6. **The four private temp-and-rename copies are replaced by the helper** (same answer; option a; T3): `pkg/mesh/account.go:52-58`, `pkg/mesh/services.go:63-68`, `pkg/promo/store.go:128-136`, `main.go:1583-1590` never fsync and share a fixed `.tmp` name.
M7. **Amendment 2026-11-15 (coordinator, M approved the merge of the overlapping PR): PR #895 is on main (`14a89ff`).** It gave `daemon.LocalNode` a `routesMu` and `GetRoutableNetworks`/`SetRoutableNetworks`, so `RoutableNetworks` is no longer T8's to lock; T8 adds one mutex for `MeshIP`, `MeshIPv6`, `NATType` (one mutex per field group, the pattern #895 and `endpointMu` already follow) and unexports the routes field for the compiler guard. Issue #894 item 2 (`PeerExchange` Start/Stop unsynchronised fields, no wait for `listenLoop`, `DHTDiscovery.Start` marks running before a startup error) is covered by T10. Decision 5 and the Wiring row are amended to match. ADR-0005 stays Accepted.

## Open Questions for M

None. M answered all of them on 2026-11-15 (M1 to M6 above).

## Context

**The pipe.** State in wgmesh moves `discover / read file -> memory -> reconcile -> write file / interface / socket`. The missing step is the **commit**: every writer of state that outlives a call, or is shared by two callers, has to be the single owner of what it writes, and nothing says so. Four shapes of the same gap, one fix each:

1. *A file is its own temp file.* `saveLocalNode` (`pkg/daemon/helpers.go:94`), `SavePeerCache` (`cache.go:90`), `Mesh.Save` (`pkg/mesh/mesh.go:105`) and `saveSecretFile` (`main.go:321`) call `os.WriteFile` on the only copy; a crash between truncate and the last byte leaves a short file. The loader then calls the damage "absent": `initLocalNode` generates a key when `loadLocalNode` returns any error (`daemon.go:362-363`, `:403-435`), and a file that is valid JSON with empty keys (`{}`) is returned as a node with an empty key (`helpers.go:60-70`) and reaches the derivation of a mesh IP (`daemon.go:380`). Four other writers already write a temp file and rename it, each with its own copy of the code, none of them fsyncs, and they share one fixed temp name.
2. *A host resource is claimed by whoever starts last.* `rpc.NewServer` unlinks the socket path before it listens (`pkg/rpc/server.go:70`), `setupWireGuard` resets an interface that exists (`daemon.go:445-456`), and the deferred `teardownWireGuard` deletes it (`daemon.go:233`, `:1544`; `:503-515`); a second daemon that then fails to bind the exchange port (`exchange.go:141-144`, started at `daemon.go:1570`) runs that teardown. `main.go:459` creates the RPC server (and so unlinks the socket) before `Run`. The socket path has no interface in it, so two interfaces collide too.
3. *A shared map is keyed too coarsely.* `pendingReplies` is `map[string]chan` keyed by remote address (`exchange.go:84`, `:1039-1049`): the second `ExchangeWithPeer` overwrites the first's channel and the first's deferred clear deletes the second's (`:550-552`). Reads: `handleReply` delivers to one channel (`:438-444`).
4. *A field has writers on several goroutines and no owner.* `LocalNode.MeshIP` is assigned by collision resolution in the reconcile goroutine (`collision.go:120`) and read by exchange, LAN, gossip and the probe server without a lock (`exchange.go:507`, `:560`, `:1170`, `lan.go:161`, `gossip.go:228`, `daemon.go:974`); `NATType` is assigned by STUN goroutines (`dht.go:256`, `:272`, `:285`, `:422`, `:445`) and read by reconcile (`daemon.go:712`); `RoutableNetworks` is assigned by the SIGHUP handler (`daemon.go:1719`) and read by announcers (`exchange.go:563`). Only the endpoint has a mutex.

**What #884 left of #863.** Verified on main (`git log --grep 863`: `4e7158a`, PR #884): `LANDiscovery.Start` and `MeshGossip.Start` recreate `stopCh` and pass it, with the connection, to their loops as arguments (`lan.go:96-102`, `gossip.go:80-114`), and tests exist (`TestLANDiscoveryRestartAfterStop`, `TestMeshGossipRestartAfterStop`). Two members of the same class remain, code-read and not run: `PeerExchange.listenLoop` reads `pe.stopCh` and `pe.conn` from the struct on every iteration (`exchange.go:195-201`, with `pe.running` read without the lock at `:206`) while `Start` replaces both (`:146-149`), so after a restart the old loop sees the new, open channel and never exits, and two loops read one connection; and `DHTDiscovery.Stop` cancels `d.ctx` (`dht.go:194`) while `Start` never recreates it, so a restarted DHT layer reports running with every loop that selects on `d.ctx.Done()` (`:460`, `:574`, `:610`, `:674` and others) returning at once. Nothing restarts either today (`daemon.go:1570-1573` starts DHT discovery once); #829 (sleep/wake recovery, another group) would.

**Class audit** (2026-11-15, read at `b8b83f3`). Class: *every piece of state that outlives one call or is shared by two callers and is written, claimed or registered without a single owner of the commit.*

| Command | Count | Members |
|---|---|---|
| `grep -rn "os.WriteFile(\|ioutil.WriteFile(" --include=*.go . \| grep -v _test.go \| wc -l` | 14 | in place, state: `helpers.go:94`, `cache.go:90`, `mesh.go:105`, `main.go:321`; temp and rename without fsync, state: `account.go:53`, `services.go:63`, `promo/store.go:129`, `main.go:1584` |
| `grep -rnE "os\.Remove\(.*[sS]ocket\|resetInterface\(\|deleteInterface\(" --include=*.go . \| grep -v _test.go \| grep -v "func "` | 4 | `pkg/rpc/server.go:70`, `:364`, `daemon.go:454`, `:511` |
| `grep -rn "map\[string\]chan" --include=*.go . \| grep -v _test.go` | 2 | `exchange.go:84`, `:118` (one map) |
| `grep -rnE "\blocalNode\.(MeshIP\|MeshIPv6\|NATType\|RoutableNetworks) *=[^=]" --include=*.go . \| grep -v _test.go` | 12 writes (62 reads from `grep -rnE "\blocalNode\.(MeshIP\|MeshIPv6\|NATType\|RoutableNetworks)\b"`) | `collision.go:120`, `daemon.go:378`, `:380`, `:391`, `:397`, `:490`, `:1719`, `dht.go:256`, `:272`, `:285`, `:422`, `:445` |
| `grep -rnE "func \(.*\) Stop\(\)" --include=*.go pkg/discovery pkg/daemon pkg/rpc \| grep -v _test` | 6 | `exchange.go:159`, `dht.go:183`, `lan.go:109`, `gossip.go:121`, `epoch.go:32`, `rpc/server.go:356`; LAN and gossip are restart-safe (#884), exchange and DHT are not, `EpochManager` and the RPC server were not read for restart |

Left out on purpose: `main.go:1498` and `pkg/pilot/report.go:53` write a report file the operator named (not state read again); `pkg/pilot/config.go:18` writes the operator tool's YAML config (not daemon state; not checked who else reads it); `pkg/daemon/systemd.go:156`, `:162` write the secret file and the unit at install time and `install-service` is re-run to repair them (a crash leaves a short file only until the next run; recorded in Out of Scope); `cmd/status-gen/main.go:27` writes `STATUS.md` in a development tool; `pkg/analytics/logger.go:16` appends to a log. `Mesh.Save` as a read-modify-write across two CLI invocations loses an update; that is a lock question, not a torn write, and belongs to the centralized group (see Out of Scope). The teardown that deletes an interface the daemon adopted rather than created stays (Out of Scope).

**ADR-0002 touches the same files.** It governs `pkg/node/*.go`, `pkg/daemon/cache.go`, `pkg/daemon/collision.go`, `pkg/daemon/config.go` and the discovery sources; ADR-0002 T5 persists the collision-resolved mesh IP by calling the existing `saveLocalNode(d.stateFile, d.localNode)`, and its T5 text says the atomic write belongs to this group. The contract between the two is that `saveLocalNode(path string, node *LocalNode) error` keeps its signature and only its body changes (T2), in `helpers.go`, which ADR-0002 does not touch, so T2 may land before or after ADR-0002 T5 and T5 is atomic either way. Everything else here edits files ADR-0002 edits and runs after the named ADR-0002 task, see Inter-task Contracts.

## Existing Primitives Audit

- Temp file and rename, four private copies (`account.go:40-63`, `services.go:63-68`, `promo/store.go:122-139`, `main.go:1583-1590`): reuse the shape, replace the four by `atomicfile.WriteFile` (T3) which adds the fsync, a unique temp name and the directory fsync.
- `LocalNode.endpointMu` (`daemon.go:112-128`): the pattern to extend; one `sync.RWMutex` for the node's mutable fields, accessors named like `GetEndpoint`.
- `PeerStore` as the single owner of peer state with one mutex (ADR-0002, `pkg/node/store.go`): the model for the local node.
- `PeerExchange.Start` re-creating `stopCh` (`exchange.go:149`) and the LAN and gossip loops taking `stopCh` and the connection as arguments (`lan.go:101`, `gossip.go:113`, PR #884): reuse for `listenLoop` (T10); `DHTDiscovery` keeps `parentCtx` so `Start` can derive a fresh context.
- `rpc.IsWritable` creates a temp file with `os.CreateTemp` (`pkg/rpc/server.go:404`): the same call is the temp-file primitive.
- `golang.org/x/sys` is an indirect dependency (`go.mod:45`); not needed: `syscall.Flock` exists on Linux and Darwin, the two targets of `.goreleaser.yml:15-17`.
- `localNodeStateDir` (`daemon.go:350`): the one place for per-interface files, overridable in tests; the lock file lives beside the state file.
- Nothing serialises two daemons today (`grep -rn "Flock\|flock" --include=*.go .` returns nothing at `b8b83f3`).

## Decision

1. **One commit step for files, `atomicfile.WriteFile(path, data, perm)` in `pkg/atomicfile`.** It creates a temp file in the same directory (`os.CreateTemp`, unique name, `perm` applied before the data is written), writes, `Sync`s the file, closes, renames over `path`, then `Sync`s the directory (a directory fsync that the filesystem does not support, `EINVAL` or `ENOTSUP`, is ignored; any other error is returned), and removes its temp file on every failure path. It is stateless: no lock, and no state besides two unexported function variables, `syncFile` and `syncDir`, that tests replace. The four in-place writers call it (T2); the four private copies call it (T3, M6); `TestNoInPlaceStateWrite` fails on a new `os.WriteFile` in a production file that is not on its allowlist. `saveLocalNode` keeps its signature.
2. **Load distinguishes missing from corrupt.** `loadLocalNode` returns `ErrStateMissing` (wraps `os.ErrNotExist`) or `ErrStateCorrupt` (not JSON, empty, or a missing public or private key); `initLocalNode` generates a key only for missing. Corrupt follows the staged path of M1: N renames the file to `<iface>.json.corrupt-<unix>` through `os.Rename`, logs the path and continues; with `RefuseCorruptState = true` (N+1) `initLocalNode` returns an error that names the file. The mesh state loader already returns an error and never regenerates (`mesh.go:60-82`) and `LoadPeerCache` failure only logs (`cache.go:95-100`, the cache is rebuilt by discovery): both stay.
3. **Exclusive claims, `flock.TryLock(path) (release func(), holder string, err error)` in `pkg/flock`.** An exclusive non-blocking `flock` on a lock file; it writes the holder's pid into the file for the message and the kernel drops the lock when the process dies, so a stale lock cannot exist. Two claims: the interface lock `<localNodeStateDir>/<iface>.lock`, taken in `joinCmd` before `NewDaemon` and so before `createRPCServer` (`main.go:422`, `:459`), released at exit; and the socket lock `<socket-path>.lock`, taken inside `rpc.NewServer` before it removes a stale socket (`server.go:70`) and released in `Stop`. A held interface lock exits 1 with the message of M2; a held socket lock makes `createRPCServer` return an error, which `main.go:461` already treats as "continue without RPC" (M3, recommendation a).
4. **Per-caller reply registration.** `pendingReplies` becomes `map[string]map[uint64]chan *PeerInfo` guarded by `pendingMu`; `registerReply(remote)` returns the channel and an `unregister` closure that deletes only that registration (and the address entry when empty); `handleReply` delivers to every registered channel with the existing non-blocking send. No wire change (M5).
5. **The local node is one type, each group of mutable fields under one lock.** `daemon.LocalNode` becomes an alias of `node.LocalNode` (T7, M4); the type gains one `sync.RWMutex` (`stateMu`) for `MeshIP`, `MeshIPv6`, `NATType` and `Introducer`, next to `endpointMu` and `routesMu` (the latter added by PR #895, merged 2026-11-15 `14a89ff`, which also gave `RoutableNetworks` its `GetRoutableNetworks`/`SetRoutableNetworks`; amended 2026-11-15, see M7 above); getters and setters for the four fields, `Identity()` and `NewLocalNode` (T8); the announcement builders read them through one `Snapshot()` that reads each group under its own lock in turn, so an announcement is not assembled from fields read at different moments within a group. The writes listed in the audit move to the setters; `TestLocalNodeFieldsNotWrittenDirectly` fails on a direct assignment from a production file outside `pkg/node`. `WGPubKey`, `WGPrivateKey` and `Hostname` are assigned once in `initLocalNode` or `loadLocalNode` before any goroutine starts and stay plain fields, and the guard test fails on a write anywhere else (M 2026-11-16 08:21: a field with later writes has a lock, a write-once field has one writer function and a test).
6. **Restart-safe loops (T10).** `PeerExchange.listenLoop` takes `stopCh` and `conn` as arguments, as the LAN and gossip loops do; `DHTDiscovery` stores `parentCtx` and `Start` derives a fresh `ctx` and `cancel` under `d.mu` before any loop starts.
7. **Staging.** Only decision 2 changes a default and takes two releases (N moves the file aside and continues; N+1 sets `RefuseCorruptState`). Decisions 1, 4, 5 and 6 change no behaviour a working node relies on. Decision 3 changes what an operator's duplicate `join` does, from destroying the running node to exiting; no workflow works through the old behaviour, so it acts from N (M2 and M3).

What would make this decision fail: (a) a writer that bypasses the helper, caught by the guard test; (b) a loader that treats corruption as absence again, caught by T4's tests; (c) a second join that still unlinks the socket because `createRPCServer` ran before the lock, caught by T5's order test; (d) a field assigned directly, caught by the T8 guard; (e) a flock that does not exclude on the platform in use (NFS, some FUSE mounts): not detectable by a unit test, see Risks.

## Alternatives Considered

- **Fix each writer in place with its own temp and rename.** Rejected: that is how four copies without fsync and with one shared temp name arose; a helper with a guard test is cheaper than reviewing every new writer.
- **A pid file instead of flock.** Rejected: a stale pid file after a crash needs a liveness check, and checking then writing is a race; `flock` is released by the kernel when the process exits.
- **Probe the socket (dial it) and unlink only when nothing answers.** Rejected as the only mechanism: probe-then-listen is not atomic and does nothing for the interface reset; the lock gives one answer for both. Kept as no part of this record.
- **A request id in HELLO and REPLY to route each reply to one caller.** Rejected: a wire change that needs two releases and that old nodes cannot echo; the reply is the same for all callers, so fan-out needs no id (M5).
- **Coalesce a second exchange onto the first one's result (singleflight).** Rejected: a reply that arrived before the second call started is lost to it, and the second caller's own HELLO would never be sent; registration of every caller keeps each caller's retry loop.
- **Make `LocalNode` immutable and swap it with `atomic.Pointer`.** Rejected: 62 read sites would change to snapshots and writers would have to copy the whole struct to change one field; a mutex with accessors follows `endpointMu`.
- **Refuse a corrupt identity at once.** Not rejected, M1 option (a), not chosen; M chose the staged form because no node that starts today is stopped in N.

## Component / Boundary Impact

- `pkg/atomicfile` (new): one reason to change, how a file replaces another; imports only the standard library; every package that persists state imports it. `pkg/flock` (new): one reason to change, exclusive ownership of a path by a process; standard library only.
- `pkg/daemon`: writers and the loader of the local node state use `atomicfile`; `initLocalNode` reads the typed errors; the type `LocalNode` is an alias of `node.LocalNode`. `daemon` still does not import `discovery`.
- `pkg/node`: `LocalNode` gains the lock and accessors (`types.go`); T7 moves `daemon.LocalNode`, with the `ObserveEndpoint` that ADR-0002 T9 adds to it, into this file in place of the unused struct.
- `pkg/rpc`: `NewServer` takes the socket lock before removing anything; `pkg/rpc` gains an import of `pkg/flock`.
- `pkg/discovery`: `exchange.go` gets the reply registry and the loop arguments, `dht.go` the context; both read the local node through its accessors.
- `main.go`: `joinCmd` takes the interface lock; `saveSecretFile` calls the helper.

## Wiring & Contract Changes

| Surface | Change | Producer | Consumer(s) |
|---------|--------|----------|-------------|
| `atomicfile.WriteFile(path, data, perm)` | new | `pkg/atomicfile` | `helpers.go`, `cache.go`, `mesh.go`, `main.go`, `account.go`, `services.go`, `promo/store.go` |
| `saveLocalNode(path, node)` | None — signature unchanged, body now atomic | `pkg/daemon` | `initLocalNode`, ADR-0002 T5 (`collision.go`) |
| `loadLocalNode` error values `ErrStateMissing`, `ErrStateCorrupt`; `RefuseCorruptState` package variable, `false` at N | new | `pkg/daemon` | `initLocalNode` |
| `<iface>.json.corrupt-<unix>` file | new, written only on a corrupt identity at N | `pkg/daemon` | operators |
| `flock.TryLock(path)` | new | `pkg/flock` | `main.go` (`joinCmd`), `pkg/rpc/server.go` |
| `<localNodeStateDir>/<iface>.lock`, `<socket-path>.lock` | new files; zero-length plus a pid line | `pkg/flock` | operators (error text) |
| `rpc.NewServer` | returns an error when the socket lock is held | `pkg/rpc` | `main.go:459` (treats it as "no RPC") |
| `daemon.LocalNode` | alias of `node.LocalNode` | `pkg/daemon/peerstore_compat.go` | discovery, tests |
| `LocalNode.GetMeshIP/SetMeshIP`, `GetMeshIPv6/SetMeshIPv6`, `GetNATType/SetNATType`, `IsIntroducer/SetIntroducer`, `Identity()`, `Snapshot()`, `NewLocalNode(...)` (`GetRoutableNetworks`/`SetRoutableNetworks` exist since PR #895) | new | `pkg/node` | `exchange.go`, `lan.go`, `gossip.go`, `dht.go`, `daemon.go`, `collision.go` |
| `LocalNode.MeshIP`, `MeshIPv6`, `NATType`, `Introducer`, `RoutableNetworks` exported fields | removed (unexported): a SOURCE break for a Go program that imports `pkg/daemon` or `pkg/node` and touches them; no wire, state file, RPC or log change. Decision of the coordinator 2026-11-16 (M: decide small design details yourself): no deprecation release, because `pkg/` has no supported library API (README documents the CLI and the daemon only; no importer found in the checkouts) and a field that stays exported cannot be locked; the release note (T11) says it | `pkg/node` | Go importers (none known) |
| `PeerExchange.registerReply(remote)` | replaces `setPendingReplyChannel` / `clearPendingReplyChannel` / `getPendingReplyChannel` | `pkg/discovery` | `ExchangeWithPeer`, `handleReply` |
| `PeerExchange.listenLoop(stopCh, conn)`, `DHTDiscovery.Start` fresh context | None — implementation-internal only | `pkg/discovery` | — |
| State file formats, announcement JSON, envelope bytes, exit code of a normal `join` | None — nothing changes shape | — | — |

## Inter-task Contracts

| Contract | Producing task | Consuming task(s) | Breaking? |
|----------|----------------|-------------------|-----------|
| `atomicfile.WriteFile` | T1 | T2, T3, T4 | No: additive |
| `saveLocalNode` writes atomically, same signature | T2 | ADR-0002 T5 (either order), T4 | No |
| `TestNoInPlaceStateWrite` and its allowlist | T3 | T11 | No |
| `ErrStateMissing`, `ErrStateCorrupt`, `RefuseCorruptState` | T4 | T11 | No: `false` at N |
| `flock.TryLock` | T5 | T6 | No: additive |
| socket lock in `rpc.NewServer` | T6 | T11 | No: a held lock is a warning in `main.go` |
| one `LocalNode` type: `daemon.LocalNode` (with ADR-0002 T9's `ObserveEndpoint`) moves to `pkg/node`, `daemon.LocalNode` becomes an alias | T7 | T8 | No: alias; runs after ADR-0002 T9 |
| local-node accessors, `Identity()`, `Snapshot()`, `NewLocalNode`; the five fields unexported | T8 | T11 | Source break for Go importers only (see Wiring); no wire or state change |
| `registerReply` | T9 | T11 | No |
| restart-safe `listenLoop` and DHT context | T10 | T11 | No |

Ordering against ADR-0002 (its tasks T1 to T11 run first on its own branch, `task/adr-0002-exec`): T1, T2, T3 (new packages, `helpers.go`, `mesh.go`, `account.go`, `services.go`, `promo/store.go`, `main.go:321`, `main.go:1583`, `cache.go:90`) have no overlap with a hunk ADR-0002 edits except `main.go` (ADR-0002 T10 adds the `--accept-routes` flag near `main.go:333` and `:788`) and `cache.go` (ADR-0002 T2 edits `RestoreFromCache`, a different function), so they may run at any time and are merged one after the other; T4 follows ADR-0002 T5 (both edit `initLocalNode`, T5 adds a `stateFile` field set at `daemon.go:353`); T5 follows ADR-0002 T10 (both edit `main.go`); T7 runs **after** ADR-0002 T9 (M4: ADR-0002 decision 10 puts `ObserveEndpoint` on `daemon.LocalNode`, T7 moves that type with it); T8, T9 and T10 follow ADR-0002 T9 (they edit `exchange.go`, `dht.go`, `lan.go`, `gossip.go`, `pkg/node/types.go` and `daemon.go` hunks that ADR-0002 T2, T3, T4, T7, T8 and T9 edit); T11 is last. Against the other groups: none read in this session; the group that owns liveness (#829) calls `Start` again after a sleep, which T10 makes safe, and it owns what decides to restart.

## Implementation

See `tasks/README.md`. Eleven tasks: T1 the atomic write step, T2 the four in-place writers use it (#870), T3 the four private copies use it and the guard test, T4 a corrupt identity is not regenerated (#870), T5 the interface lock and `joinCmd` (#860), T6 the socket lock in `rpc.NewServer` (#860), T7 one `LocalNode` type, T8 the local node under one lock, T9 reply registration per caller (#855), T10 restart-safe exchange loop and DHT context (#863 remainder), T11 specs, release note and backlog rows.

## Consequences

- **Positive:** a crash cannot truncate the identity, the peer cache, the mesh state or the init secret; a duplicate `join` can no longer unlink the live socket or reset the live interface (of a lock-holding release); a corrupt identity leaves the bad file on disk instead of overwriting it; concurrent exchanges to one address all finish; the local node can be read from any goroutine under `-race`.
- **Negative:** until N+1 a corrupt identity still leads to a new key and mesh IP (the old file is kept aside); two releases for one default. A daemon of an older release does not hold the lock, so the protection starts when the running node is the new release. `fsync` adds one disk flush to each state save (the peer cache every five minutes, the identity at start); not measured here, T2 records the time of one save on the development machine.
- **Neutral:** stale `.tmp-*` files from a crash between create and rename remain in the state directory and are ignored; `Mesh.Save` read-modify-write across two CLI invocations still loses an update (Out of Scope).

## Out of Scope

- A lock around `Mesh.Save` read-modify-write across CLI invocations (two `addnode` runs lose one node): a lost update, not a torn write (deferred: docs/adr/BACKLOG.md).
- The pipeline that pushes configs by SSH, #865, #866, #867, #868: they act on `pkg/ssh` and the apply step, `Mesh.Save` here is only the file write (permanent: boundary: another group owns the centralized pipeline).
- Deleting an interface the daemon adopted instead of created at shutdown (`daemon.go:503-515`): with the lock the second daemon no longer reaches it; whether a restart should leave an adopted interface alone is a lifecycle decision (deferred: docs/adr/BACKLOG.md).
- `install-service` writing `secret.env` and the unit atomically (`pkg/daemon/systemd.go:156`, `:162`): re-running the command repairs a short file (deferred: docs/adr/BACKLOG.md).
- Deciding when to restart discovery after sleep or wake, #829, and peer liveness (permanent: boundary: another group owns liveness; T10 only makes `Start` after `Stop` safe).
- Mesh IP persistence of the collision result: ADR-0002 T5 (permanent: boundary: owned by ADR-0002; this record only changes how `saveLocalNode` writes).
- Admit rules and the `Offer` path: ADR-0002 (permanent: boundary: owned by ADR-0002).
- A database or journal for node state (permanent: boundary: the state is four small files and a rename is atomic on the filesystems in use).
- Locks on network filesystems and Windows (permanent: fact: releases are built for linux and darwin only; citation: file `.goreleaser.yml:15`).

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| `flock` does not exclude on the state directory's filesystem (NFS, some FUSE) | Low | Med | the state directory is `/var/lib/wgmesh`, local by convention; T5 documents it; a lock that cannot be taken because the call fails is an error, not a silent pass |
| An operator relies on running `join` again to restart | Low | Med | it destroyed the running node before; the message names the lock and the pid and says to stop the service first; M2 |
| The directory fsync is slow or unsupported | Low | Low | unsupported is ignored (`EINVAL`, `ENOTSUP`); the writes are rare (identity at start, cache every 5 minutes) |
| Stale `.tmp-*` files fill the state directory | Low | Low | at most one per crash; `CreateTemp` names are distinct; documented in the release note |
| The alias makes a daemon test that builds `LocalNode` with unexported fields fail to compile | Low | Low | T7 step S1 greps tests first; both types have identical fields |
| Moving the writes to setters misses one | Med | Med | the guard test parses non-test files and fails on a direct assignment; mutation-tested in T8 |
| Delivering a reply to every registered caller wakes a caller whose HELLO the reply does not answer (the reply is for the other caller's HELLO) | Low | Low | the reply is the peer's own announcement, valid for both; a caller whose own HELLO was lost gets an answer to the other's, which is the same data |
| Ordering against ADR-0002 is broken (T4, T5, T8 to T10 started early) | Med | Med | the dependency headers say it, T7 and T8 state the ADR-0002 task they follow in Stop Condition; the tasks run on one branch in order |
| An older running daemon is not protected by the new `join` | Med | Low | stated in Consequences and the release note; the first release that ships T5 protects the next |

## Rollback

N is additive. State files keep their format, so an older release reads what the new one wrote; stale `.tmp-*` and `*.corrupt-*` files are ignored by it. Lock files are zero-length (plus one pid line) and an older release does not look at them; deleting `pkg/flock` use returns to the old behaviour with no cleanup. Reverting T4 returns the loader to "any error means absent"; a file moved aside at N stays where it is and can be copied back by hand. Reverting T7 to T10 changes no persisted data. After N+1, setting `RefuseCorruptState` back to `false` is a one-line revert and rewrites nothing.

## Follow-ups

- [ ] N+1: set `RefuseCorruptState = true`, with a release note (deferred: docs/adr/BACKLOG.md)
