# State files, locks and local node: rollout

Decision record: [ADR-0005](adr/ADR-0005-atomic-state/ADR-0005-atomic-state.md). This page is for the operator of a mesh and is the release note for the release that brings it in (N) and the one after it (N+1). N+1 is not done yet: it is a row in [the backlog](adr/BACKLOG.md), not a task.

Four things change in N: state files are written atomically, `wgmesh join` and the RPC socket take exclusive locks, a corrupt identity file is moved aside instead of overwritten, and concurrent peer exchanges to one address all receive the reply. No byte on the wire and no state file format changes, so old and new nodes talk to each other and an older release reads what N wrote.

## What changes and in which release

| Release | What the node does | Switch |
|---|---|---|
| N (the first with this change) | writes state files atomically; takes `<iface>.lock` and `<socket>.lock`; moves a corrupt identity file to `<iface>.json.corrupt-<unix>` and creates a new identity; stops at an unreadable identity file | `daemon.RefuseCorruptState = false` |
| N+1 | refuses to start on a corrupt identity file and leaves the file where it is | `RefuseCorruptState = true` |

`RefuseCorruptState` is a package variable that a release flips; it is not an operator switch. It is `false` in N and nothing sets it.

## Release note for N

1. **State files are written atomically** (`atomicfile.WriteFile`: a temp file in the same directory, fsync, rename, fsync of the directory). The files are the local identity `<iface>.json`, the peer cache `<iface>-peers.json`, the centralized `mesh-state.json` (plain and encrypted), the init secret (`--save-to`), and the account, services, promo and referral files. A crash or power loss leaves the old file or the new one. Differences from the old plain write that an operator can notice:
   - A symlink at a state path is replaced by a regular file; it is not followed.
   - The mode is set to 0600 and the owner becomes the process user on every save: a 0640 file an admin made, or a file re-created by root, changes.
   - The directory must be writable: a user-owned file in a root-owned read-only directory no longer saves.
   - A single-file bind mount of a state file fails with `EBUSY` on the rename. The daemon logs the error and continues; the identity is then not persisted.
   - A leftover `<name>.tmp-*` after `kill -9` or power loss is not swept. A leftover temp file of the identity holds the private key at mode 0600; delete it by hand.
   - On darwin Go's `File.Sync` uses `F_FULLFSYNC` (plain fsync on mounts that refuse it, such as SMB); darwin is not verified.
   - An error of the directory fsync arrives after the rename: the new content is already in place, and the error is returned (M decided 2026-11-16).
2. **The fsync costs a little.** Measured 2026-11-15 on host mars, zfs `rpool/claude`, median of 20 runs, three repeats: `saveLocalNode` 0.06 ms before, 1.44 ms now; `SavePeerCache` with 1000 peers 5.3 ms before, 6.2 ms now. On tmpfs there is no difference. The identity is saved at start and on an address change, the cache every 5 minutes.
3. **A corrupt identity is moved aside** (see "Recovering a moved-aside identity" below). **An unreadable identity file stops the daemon.**
4. **`wgmesh join` takes an interface lock** (see "Messages and what to do").
5. **The RPC socket has a lock** (see "Messages and what to do").
6. **`LocalNode` fields are unexported.** This is a Go source break for a program that imports `pkg/daemon` or `pkg/node`: `LocalNode.MeshIP`, `MeshIPv6`, `NATType`, `Introducer` and `RoutableNetworks` are now behind accessors (`GetMeshIP`/`SetMeshIP`, `GetNATType`/`SetNATType`, `Identity()`, `Snapshot()`, `NewLocalNode`). There is no wire, state file, RPC or log change, and `pkg/` is not a documented library API. `reconcile()` is serialized (the SIGHUP handler and the reconcile loop both call it; a collision of the two could move the node twice), and `Config.LogLevel` is behind accessors (the DHT debug logging raced with the SIGHUP reload).
   Main's `identityMu` accessors (`GetMeshIP`, `GetMeshIPs`, `GetNATType` and the setters) are the same names on the one `node.LocalNode`; only the fields and `identityMu` are gone.
7. **Every concurrent exchange to one address receives the reply.** `ExchangeWithPeer` calls to one address each register their own reply channel and each gets a copy of the reply, so punch, rendezvous and transitive callers may now share one reply (#855). Nothing changes on the wire.
8. **The exchange loop and the DHT layer restart safely** (#863 remainder, #894 item 2). What the code does now:
   - `Stop` waits up to 3 seconds for the exchange listen loop and for the DHT loops; a loop that does not end within that time is logged, and `Stop` returns anyway.
   - `Start` and `Stop` of the DHT layer are serialized: a `Stop` issued during a `Start` waits for it and then stops it.
   - A startup error stops the layers already started (exchange, gossip, LAN), so a failed `Start` leaves nothing listening and may be retried.
   - `Start` after the parent context has ended fails ("DHT discovery parent context cancelled"); a `Start` after a `Stop` gets a fresh context.
   - Each run builds its own LAN and gossip layers.
   - The layer counts as running only after every layer started.

## Messages and what to do

| Message | Meaning | What to do |
|---|---|---|
| `another wgmesh join holds the lock <path> (pid N); stop it first (systemctl stop wgmesh, or brew services stop wgmesh)` | a `join` for this interface is running | stop the service, then start again; do not run `join` a second time to restart |
| `... ; run wgmesh join as root (or as the user that owns /var/lib/wgmesh)` | the lock directory or file cannot be created or opened (permission) | run `join` as root, or as the user that owns `/var/lib/wgmesh` |
| `RPC socket <path> is in use by another daemon (pid N): run the second daemon with --socket-path (or WGMESH_SOCKET) pointing at a different file` | a second daemon picked the same socket path; it runs without RPC and logs a warning | give the second daemon its own `--socket-path`; clients choose a socket through `WGMESH_SOCKET` (only `join` has `--socket-path`) |
| `Local node state file <path> is corrupt and was replaced by a new identity ...` | the identity file was moved aside, a new key and mesh IP were created | see the next section |
| `failed to read local node state file <path>: ...` | the identity file exists but cannot be read (permission, a directory) | fix the permission or the path; at N the daemon stops (the old code generated a new key) |
| `local node state file <path> is corrupt and could not be moved aside` | the move failed | fix the directory, then start again; nothing was overwritten |

Details of the locks:

- The interface lock is `<state dir>/<iface>.lock`, always under `/var/lib/wgmesh` by default whatever `--state-dir` says: the interface is a host resource. `join` takes it before it reads the account file, records a referral, or creates the daemon or the RPC server, so a second `join` exits 1 before it touches the socket or the interface. A `join` whose configuration is invalid no longer saves the account key or records the referral.
- The RPC lock is `<socket path>.lock` beside the socket (`/var/run`, `$XDG_RUNTIME_DIR` or `/tmp`). The unit of the second daemon stays active while one interface has no RPC. `wgmesh status` and the other client commands talk to the first daemon.
- A lock file in the world-writable `/tmp` fallback owned by another user gives `EACCES`; a symlink planted at the lock path is not followed (`O_NOFOLLOW`).
- A failed RPC `Start` releases the lock, `Stop` always logs `RPC server stopped`, and the daemon now logs a failed socket removal. `pkg/rpc` and `pkg/flock` build on unix only.

## Recovering a moved-aside identity

At N an identity file that the daemon calls corrupt (`ErrStateCorrupt`: not JSON, empty, `{}`, or without the public or private key) is moved to `<iface>.json.corrupt-<unix>` (`-1`, `-2` ... when the name is taken; mode forced to 0600; never deleted), and a new identity is created: a new key and a new mesh IP, so peers see a new node. The log names both paths. To go back to the old identity, after you have repaired or checked the old file:

1. Stop the daemon (`systemctl stop wgmesh`, or `brew services stop wgmesh`).
2. Rename the `.corrupt-<unix>` file back to `<iface>.json` (replacing the new one).
3. Start the daemon. Peers see the old key again.

If the old file really is empty or broken, there is nothing to restore: keep the new identity and delete the `.corrupt-*` file when you no longer need it (it may hold a private key).

`wgmesh service` (`deriveMeshIPForService`) still reads `<state dir>/wg0.json` with its own struct and tolerates a file the daemon calls corrupt.

## Limits of the locks

- A lock protects only against a running daemon of a release that takes it. An older daemon holds none, and an older `join` started after N does not look at it: the first release that ships these locks protects the next one.
- The RPC protection is one-sided: an older release without the lock still unlinks a live socket, and a new release started after an old one still unlinks its socket.
- The lock files are never removed and keep the last pid. Removing a lock file while a daemon runs lets a second `join` lock a new file (known limit, backlog).
- `wgmesh install-service` starts the unit with `systemctl start`, which succeeds before a lock-refused `join` exits 1: the unit then restarts every 5 seconds while the install reports success (backlog).
- Docker Desktop bind mounts on macOS may not exclude across the VM (not verified). The state directory is expected on a local filesystem; `flock` may not exclude on NFS or some FUSE mounts.
- A client command cannot pick a socket except through `WGMESH_SOCKET`, and a second daemon without RPC shows only a log warning, not a health or `daemon.status` entry (backlog).

## N+1

N+1 sets `RefuseCorruptState = true`: a corrupt identity file then stops the daemon at start with a message that says to move the file away (a new identity is then created) or to restore a good copy, and the file stays where it is. It needs its own release note and is not done: it is a row in [the backlog](adr/BACKLOG.md). Setting it back to `false` is a one-line revert that rewrites nothing.

## Rollback

N is additive. State files keep their format, so an older release reads what N wrote; stale `.tmp-*` and `*.corrupt-*` files are ignored by it. Lock files are zero-length plus one pid line and an older release does not look at them. Reverting the corrupt-identity handling returns the loader to "any error means absent"; a file moved aside at N stays where it is and can be copied back by hand. The local node, reply registry and restart changes persist no data.
