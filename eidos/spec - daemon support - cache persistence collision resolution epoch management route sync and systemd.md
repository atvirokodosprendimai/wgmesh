---
status: implemented
compat-dimensions: []
tracking-issue:
since: ""
tldr: Five support subsystems: peer cache survives restarts; collision resolution re-derives IPs deterministically; epoch manager rotates Dandelion++ relays; route sync tracks relay-aware gateways; systemd integration installs the daemon as a hardened service.
category: core
---

# Daemon support subsystems

## Target

The five supporting mechanisms that complement the daemon core: persistence, IP uniqueness, privacy relay rotation, kernel route management, and OS service integration.

## Behaviour

### Peer cache

- The peer store is serialised to `/var/lib/wgmesh/<iface>-peers.json` every 5 minutes and on clean shutdown.
- On startup, cached entries not older than 24 hours are restored into the peer store via the `"cache"` discovery method.
  This allows the node to reconnect to known peers without waiting for a full DHT/gossip rediscovery cycle.
- Cache restores do not update `LastSeen` of a key that is not already in the store — the peer must be re-confirmed by a live discovery source to be treated as active. For a key already in the store a `Restored` offer raises `LastSeen` when the cache time is newer (`merge`); production restores before discovery starts (`RunWithDHTDiscovery`), so the store is empty then.
- Restored entries are offered as `Restored` (not first hand): a tombstone refuses them, and the `accept-routes` list judges their routes. A guessed endpoint is not written to the cache.
- The file is written by `atomicfile.WriteFile` (a temp file in the same directory, fsync, rename, fsync of the directory), so a crash or power loss leaves the old file or the new one, never a half-written one. It is mode 0600 and owned by the process user on every save, a symlink at the path is replaced and not followed, and the directory must be writable. A leftover `<name>.tmp-*` after `kill -9` is not swept. Differences from the old plain write and their numbers: `docs/state-rollout.md`.
- **`last_seen` is `LastContact` (ADR-0004):** the field written for each peer is the newest of `LastSeen` and `LastDataPlane`, never the restore time or the wake grace (a time in the future is written as now), so a peer kept connected by handshakes is saved fresh and a restored peer never heard from keeps its cache time. The field and its type are unchanged. On restore the entry gets `RestoredAt` (it is dialable for `PeerDeadTimeout`) and the log line counts how many restored peers are alive by evidence and how many only suspected.

### Mesh IP collision resolution

- Mesh IPs are derived deterministically from `(shared secret, WireGuard public key)`. Two peers can independently derive the same IP. A claim is **bound** when its IP equals the IP derived from its own key at nonce 0 to 10 (`node.MaxNonce`); rotated, nonce and custom-subnet addresses are legitimate and are never refused for being unbound.
- **Detection:** `node.ResolveClaims(local, peers, derive)` lists the IPs claimed by more than one member, **the local node included** (ADR-0002 R3), sorted by IP then key so every observer computes the same list. `DetectCollisions(ps, local)` wraps it.
- **Claim order:** with `EnforceIPBinding` set, a bound claim beats an unbound one and equal classes fall to the lower public key. While the switch is `false` (this release) the lower public key wins as before, and each collision the bound order would decide differently is counted as `winner-differs` in `wgmesh_admit_warnings_total`.
- **Resolution:** a local loser (`moverIP`) takes the first address of nonce 1 to 10 that no other member claims (the nonce-1 address when all are claimed). For a remote loser `ResolveCollision` derives the nonce-1 address only to log it.
- If the **local node** is the loser: reconfigure the WireGuard interface address at once (so from release N the interface address of a node can change at runtime) and persist the resolved IP in the state file (`saveLocalNode`), so a restart keeps it: the IP is stored, not the nonce. `wgmesh service add` reads the state file `wg0.json` only (known limit, `docs/adr/BACKLOG.md`).
- If a **remote peer** is the loser: only log the expected nonce-1 address; nothing is recorded or changed here. The remote moves only if it runs release N or later code.
- Collision check runs at the end of every reconcile cycle.

### Epoch management (Dandelion++)

- `EpochManager` wraps `pkg/privacy.DandelionRouter` and starts a rotation goroutine that periodically re-selects relay peers for Dandelion++ stem routing.
- The epoch seed is the mesh subnet bytes from `pkg/crypto.DerivedKeys`.
- Epoch rotation runs independently of reconciliation; relay peer selection is available to the reconcile loop via `GetRouter()`.

### Route sync

- Kernel routes of the mesh interface are converged in every reconcile cycle with the same observe, desired state and apply as the peers (`converge.go`): both families are read back (`ip route show dev <iface>` and `ip -6 …` on Linux), one diff is computed and applied one route at a time, and a failed route never skips the others.
- **One owner per network:** a network that several peers advertise has one owner, chosen by `routes.Resolve`, and the route and the owner's AllowedIPs come from that one choice. A peer with a handshake newer than `HandshakeStaleAfter` beats one without; among fresh peers the owner read back from the kernel keeps the network; otherwise the lowest mesh IP wins, compared as a number, then the lowest key. The choice is local to the node. A network this node advertises itself is owned by nobody, so two HA exit nodes each drop the other's default route (`routes.ExcludeOwn`).
- **Relay-aware:** if a peer is relay-routed, its network gateway is set to the relay's mesh IP (not the peer's mesh IP).
- **Both families:** an IPv4 route uses the carrier's mesh IPv4 as gateway, an IPv6 route the carrier's mesh IPv6 (the relay's when the peer is relayed, falling back to the peer's own when the relay has none); a network whose carrier and peer both have no mesh IPv6 is skipped, and with `--no-ipv6` IPv6 networks are not routed.
- A default route (`0.0.0.0/0`, `::/0`) installed with `ip route replace` replaces the host's own default route of that family (same route key); the daemon remembers the host's (device and gateway) from the read before it and puts it back once when the mesh default route is removed.
- A network equal to or inside one this host is attached to is not routed from the first release on macOS; on Linux it is warned about (rule `attached-network`) and, once `EnforceRouteGuard` is on, left out. A supernet is kept.
- Skips peers that are temporarily offline.
- Linux uses `ip`; macOS reads `netstat -rn` and writes with `route` (the formats are not yet verified on a real Mac); no other OS has routes.
- **Forwarding is converged once:** on Linux `ip_forward` is written only when it reads 0 and the iptables `FORWARD ACCEPT` rule for the WireGuard interface (which lets relay traffic pass through) is appended only when `iptables -C` says it is absent. A missing `iptables` is logged once. At release N forwarding is still forced on every Linux node; once `EnforceForwardScope` is on only an introducer or a node with `advertise-routes` does.

### Wake from sleep (ADR-0004)

- The resume loop reads the wall clock every `ResumeCheckInterval` (2 s); a gap above `ResumeGapThreshold` (10 s) is a wake: it counts `wgmesh_resume_detected_total`, calls `PeerStore.NoteResume`, signals one extra reconcile and calls `OnResume` on the discovery layer when that implements `node.Resumer`.
- For `ResumeGrace` (60 s) after a wake the eviction by the health loop and by the mesh probe and the choice of a relay from a stale handshake are skipped, but only while `node.EnforceLiveness` is set; while it is `false` each such case is counted as `resume-grace` in `wgmesh_liveness_disagree_total`.

### Relayed peers kept probe-only (ADR-0004)

- With `daemon.EnforceRelayProbe` set (`false` at release N) a relay-routed peer that has a usable endpoint stays on the interface with its endpoint and no AllowedIPs (`desiredPeerConfig.probeOnly`; `wg set` is sent an empty `allowed-ips`), so a direct handshake can bring it back; the relay carries its traffic meanwhile. The health loop and the reconnect skip relay-routed peers whatever the switch.
- `wgmesh_relay_route_age_seconds{peer}` is the time since this daemon first saw the peer relayed (0 after a restart); it is exported at every release.

### Systemd integration

- Generates a systemd unit file from daemon options. The secret is stored in `/etc/wgmesh/secret.env` (mode 0600) and referenced as `${WGMESH_SECRET}` — it never appears in the process list.
- Unit hardening: `NoNewPrivileges=yes`, `ProtectSystem=full`, `ProtectHome=true`, `ReadWritePaths=/var/lib/wgmesh`.
- `InstallSystemdService`: writes unit + secret env, creates `/var/lib/wgmesh` (required by `ReadWritePaths`), runs `systemctl enable + start`.
- The unit and `secret.env` are still written with a plain `os.WriteFile`, so a crash can leave a short file (re-running `install-service` repairs it); they are the two allowlisted calls of `systemd.go` in `TestNoInPlaceStateWrite`. `install-service` also reports success while the unit crash-loops when the interface lock is held. Both are rows in `docs/adr/BACKLOG.md` (ADR-0005).
- `UninstallSystemdService`: stops, disables, removes unit and secret files.

## Design

- Cache file path: `/var/lib/wgmesh/<iface>-peers.json` (state directory, not config directory).
- Collision nonce: nonces 1 to `node.MaxNonce` are tried, the first address nobody claims is taken.
- State files are written by `atomicfile.WriteFile` and by nothing else: the local identity, the peer cache, `mesh-state.json` (plain and encrypted), the init secret (`--save-to`), the account, services, promo and referral files. `TestNoInPlaceStateWrite` (`pkg/atomicfile/guard_test.go`) parses every production file and fails on a new `os.WriteFile` or `os.Rename` outside the allowlist; `saveLocalNode` keeps its signature and only its body is atomic. The resolved mesh IP of a collision is persisted through it.
- `CommandExecutor` interface (`executor.go`) wraps `os/exec` — injected globally (`cmdExecutor`), replaceable with a mock for testing. The `systemd.go`, `observe.go` and `converge.go` shell-outs use this interface.

## Interactions

- `cache.go` ↔ `PeerStore` — `GetAll()` for save, `Offer(…, Restored)` for restore.
- `collision.go` ↔ `node.ResolveClaims` + `PeerStore.Policy()` + `pkg/crypto.DeriveMeshIP`; `applyInterfaceAddress` is the seam that sets the interface address.
- `epoch.go` ↔ `pkg/privacy.DandelionRouter`.
- `desired.go`, `observe.go`, `converge.go` ↔ `PeerStore` (via reconcile) + relay routes map + `pkg/routes.Resolve` and `pkg/routes.CalculateDiff`; `routes.go` keeps the connected-route restore and the relay table snapshots.
- `systemd.go` ↔ `cmdExecutor` (system commands).

## Mapping

> [[pkg/daemon/cache.go]]
> [[pkg/daemon/collision.go]]
> [[pkg/daemon/epoch.go]]
> [[pkg/daemon/routes.go]]
> [[pkg/daemon/desired.go]]
> [[pkg/daemon/observe.go]]
> [[pkg/daemon/converge.go]]
> [[pkg/daemon/systemd.go]]
> [[pkg/daemon/executor.go]]
