# Release notes: v0.3.0-rc3 (pre-release)

For operators of existing nodes. This is a release candidate: test it on a few nodes before the rest. "Release N" is this release; "the release after" is N+1, and N+2 follows it. No dates are set. PR numbers are merged PRs on `main`. Each feature has a rollout page in `docs/` with the full detail; this page keeps what an operator must know before upgrading.

## Upgrade actions

- Upgrade every node to this release before any later release in the staged plan below. Old and new nodes talk to each other at N: no wire format changes.
- Centralized `wgmesh -deploy`: the first run after upgrading needs every node's SSH host key in `~/.ssh/known_hosts` (PR #876; before, host keys were ignored).
- Add a key with `ssh-keyscan -H <host> >> ~/.ssh/known_hosts` after checking its fingerprint.
- Use another file with `WGMESH_SSH_KNOWN_HOSTS`. `WGMESH_SSH_INSECURE_HOST_KEY=1` turns verification off for labs and prints a warning.
- A host recorded in `known_hosts` only by its Ed25519 (or another non-ECDSA) key now connects: the client offers the key types `known_hosts` holds for that host first (#896). `ssh-rsa` and DSA keys never bring back SHA-1 signatures.
- Centralized `-deploy` validates the state file before the first connection. A class A value (a character that would break a root shell or the wg-quick file) is refused at once; class B findings only print `Warning: state:` lines. Read them now: they become refusals in the release after (`docs/centralized-rollout.md`).
- Decentralized systemd nodes: the unit no longer carries the secret in `ExecStart` (#876). The secret is read from `/etc/wgmesh/secret.env` (mode 0600) as `WGMESH_SECRET`.
- Existing nodes keep the old unit until it is rewritten: run `wgmesh install-service --secret <SECRET>` again, then `systemctl restart wgmesh` (install runs `enable` and `start`, which does not restart a running unit).
- deb and rpm upgrades keep the service running from now on: preremove stops and disables it only on real removal, postinstall runs `systemctl try-restart wgmesh.service` (#892).
- CAVEAT for the first upgrade: the package you upgrade FROM runs its own old preremove, which still stops and disables the service. After upgrading from a version without #892, run `systemctl enable --now wgmesh` once. Verified on Debian 13 only.
- `wgmesh join` takes an exclusive lock `/var/lib/wgmesh/<iface>.lock`: a second `join` for one interface exits 1. Restart with `systemctl restart wgmesh`, never by starting a second `join`. A second daemon on one RPC socket needs its own `--socket-path`.
- State files are now written atomically: a symlink at a state path is replaced by a regular file, the mode is reset to 0600 on every save, and the state directory must be writable. A single-file bind mount of a state file fails with `EBUSY` and the identity is then not persisted: mount the directory (`docs/state-rollout.md`).
- Delete leftover `<name>.tmp-*` files in the state directory by hand after a crash: they are not swept, and an identity temp file holds the private key.
- A corrupt identity file `<iface>.json` is moved to `<iface>.json.corrupt-<unix>` and a NEW identity is created (new key, new mesh IP). An unreadable file stops the daemon. To go back, stop the daemon, rename the `.corrupt-*` file back, start (`docs/state-rollout.md`).
- Alerts or log greps on `Failed to configure peer`, `Failed to remove obsolete peer`, `Failed to sync peer routes` or `Failed to install relay FORWARD rule` stop matching: the texts are replaced (`docs/kernel-state.md`, Log lines).
- `--advertise-routes` takes a bare IP as a host route again, `/32` or `/128`, with a warning (#893). A unit with a bare IP crash-looped between #849 and this release.
- `wgmesh` with no subcommand or an unknown first word prints usage on stderr and exits 2. Before, it fell into the legacy centralized mode. `--version` and `-v` count only as the first argument. Arguments that start with `-` still reach the legacy flags (ADR-0007).
- `status`, `qr`, `install-service` and `wgmesh service` read the secret the way `join` does: flag, then `WGMESH_SECRET`, then `WGMESH_SECRET_FILE` (#891).
- Docker Compose examples pass `WGMESH_SECRET` from `MESH_SECRET`, and compose fails at once when `MESH_SECRET` is unset (#882).
- Go programs that import `pkg/daemon` or `pkg/node`: the exported `LocalNode` fields (`MeshIP`, `MeshIPv6`, `NATType`, `Introducer`, `RoutableNetworks`) are gone, use the accessors. `pkg/` is not a documented library API.

## Behaviour changes

- Envelope message type is authenticated and replays are refused, staged (ADR-0001, `docs/envelope-rollout.md`, PR #877).
- N: readers open both forms. GOODBYE is the only message sealed in the new form; all other writers are unchanged (`pkg/discovery/exchange.go`). A node older than N cannot open the new GOODBYE and removes that peer through its health check a few minutes later.
- Admit step (ADR-0002, `docs/admit-rollout.md`): every claim about a peer passes one decision before the peer store. At N it only counts what N+1 and N+2 would refuse (`wgmesh_admit_warnings_total{rule}`, needs `--metrics`); both switches are off.
- A node that loses a mesh IP collision with a remote peer re-derives its mesh IP and changes the address of its WireGuard interface while running. This is not behind a switch and is expected only where two members derived one address.
- The resolved mesh IP is written to the state file and survives a restart. A node keeps its persisted mesh IP only while it lies inside the current network: the explicit `--mesh-subnet`, or else the `/16` the secret derives (#890). After a secret change that moves the `/16`, or after `--mesh-subnet` is dropped, an address outside it is re-derived and the new address is saved. Before this change any `10.0.0.0/8` address was kept (#880, #887), which split a mesh into two `/16`s after a secret change. `service add` registers it, but reads `wg0.json` only.
- `--accept-routes 10.1.0.0/16,...` (also in the reload file, signalled with SIGHUP) is new: a list of networks this node approves. Unset means open, as before. At N a network the list would drop is counted (`route-not-approved`) and kept.
- A withdrawn peer (GOODBYE, health eviction) is not brought back by a mention for 5 minutes (tombstone, memory only, a restart forgets it).
- A guessed endpoint (sender IP and default port) ranks below the peer cache and is not announced or cached.
- The local endpoint changes on STUN alone only on the third identical answer in a row (about two minutes at the 60 s refresh).
- One kernel state (ADR-0003, `docs/kernel-state.md`): every 5-second tick the daemon reads the kernel, computes one desired state (peers, routes, addresses, forwarding) and writes only the difference. A failed item never skips the others. A steady node writes and logs nothing.
- For a network several peers advertise, one owner is chosen per node: a fresh handshake (150 s) wins, then the owner the kernel already holds, then the lowest mesh IP. Failover takes at most 150 s plus one tick after the owner's last handshake; this is read from the constants, not measured.
- IPv6 routes on the mesh interface are now managed: an IPv6 `via` route someone added on it is removed when not desired.
- An advertised default route replaces the host's own default route of that family (`ip route replace`). The daemon logs `host-default:` and restores the host default once when the mesh default is removed. It is remembered in memory only: after a restart with the mesh default already installed, restore it by hand.
- Four kernel switches are off at N (`EnforceForwardScope`, `EnforceRouteGuard`, `EnforceAddressRemoval`, `EnforceRevertOnExit`). At N a stale address inside the mesh network is only warned about (`wgmesh_kernel_warnings_total{rule}`) and kept; at the release after it is removed. Host-added addresses of the mesh interface's family are lost when a node loses an IP collision (the family is flushed).
- A graceful exit logs what this run added to forwarding (`revert-on-exit:`); it reverts it only after `EnforceRevertOnExit` is on.
- Liveness (ADR-0004, `docs/liveness-rollout.md`): the node records its own evidence (WireGuard handshakes, mesh probe replies) and detects a wake from sleep (gap above 10 s: one extra reconcile, endpoint refresh, announce). At N the old `LastSeen` rules still decide; `wgmesh_liveness_disagree_total{site}` counts where the evidence would decide differently. `EnforceLiveness` and `EnforceRelayProbe` are off.
- The mesh probe loop already uses the evidence at N and visits Suspect peers. The health loop skips relay-routed peers.
- The peer cache field `last_seen` now holds the newest measured contact. An older version still reads the file, and after a rollback a peer kept only by handshakes looks announced-fresh to it.
- Centralized deploy (ADR-0006, `docs/centralized-rollout.md`): every value sent to a node is quoted for the shell; a node that fails (phase `detect`, `connect`, `install`, `apply`, `routes`, `persist`) no longer stops the run. The others are configured and the error lists failed and configured nodes. Exit code stays 1. Run `-deploy` again after fixing the node.
- Endpoint resolution in `-deploy`: a written `public_endpoint` wins; an all-private mesh gets `<ssh host>:<listen port>` per node without asking for the public IP; a detected IP equal to the SSH host is the endpoint; otherwise the node is behind NAT. A config file that cannot be written is a warning at N.
- `-deploy` restores the connected route (`Restored connected route: <net> dev <iface>`) when it removes an own-network route left by a release before #879 (#894 item 1). It needs a local address in that network. Upgrade the control node first.
- Bounded growth (ADR-0007): the DHT contact map has a 60-second window and a hard cap of 8192 distinct addresses (past it new addresses are skipped); the maps keyed by public key are swept against the peer store; the `temporaryOffline` map is swept once a minute. A swept peer that returns restarts its rendezvous backoff.
- Every `wg` call made through `pkg/wireguard` has a 10-second deadline; one `wg show <iface> dump` per 5-second tick is shared by the probe and health loops (up to 5 seconds old). The fork count of about 90 to about 12 a minute is computed from call sites, not measured.
- Race audit (#901, #904, #895): shared daemon, local node and routable-networks state is read and written under a lock or atomically. The start time keeps its monotonic reading, so an NTP step no longer changes the uptime.
- A peer's own announcement with an empty route list now withdraws its routes (#881).
- `PeerStore.Unsubscribe` no longer closes the subscriber channel (#881). Library users must not wait for the channel to close.
- Centralized re-deploy ignores a runtime-learned endpoint when none is desired (#883).
- A network a node advertises itself is never routed through the mesh (#879, #885). This covers HA shared subnets and the default routes of HA exit nodes (each drops the other's `0.0.0.0/0`).
- A decentralized node restores its connected LAN route when it removes such a via route (#885).
- Concurrent exchanges to one address all receive the reply (#855). A failed discovery start stops the layers already started and may be retried; `Stop` waits up to 3 seconds per loop.
- LAN discovery logs a warning when `--listen-port` equals 51830 (#886).
- IPv4-only start when IPv6 is disabled (#878).
- Fixes: centralized deploy deadlock (#842), `wg show dump` keepalive parse (#846), `rotate-secret` now fails with an honest error instead of faking success (#847).
- Hardening in #876: probe reads and request size are bounded; advertised routes and exchange announcements are validated.

## Staged plan ahead

All switches below are Go package variables compiled into the binary, `false` in this release. No flag, file or signal changes them on a running node; only a new build does. Flip one only when every node runs the release before it, and read its counter on every node first ("zero since process start on every node", with `--metrics`).

- Envelope N+1 (the release after): all writers seal the new form. Nodes older than N cannot open the new packets, so upgrade every node to N first (`docs/adr/BACKLOG.md`).
- Envelope N+2: legacy envelopes are refused, and nodes older than N+1 are cut off. Watch `LegacyEnvelopeOpens` before this step.
- ADR-0002 admit: N+1 enforces IP binding (`EnforceIPBinding`; gate `ip-held-by-bound`, `winner-differs` zero); N+2 enforces route binding (`EnforceRouteBinding`; gate the three route rules zero). An unset `accept-routes` stays open.
- ADR-0003 kernel: N+1 flips the four kernel switches (gates: `wgmesh_kernel_warnings_total` zero for the rule).
- ADR-0004 liveness: N+1 `EnforceLiveness`, N+2 `EnforceRelayProbe`. Upgrade every node to N first: a node before N judges by `LastSeen` alone.
- ADR-0005 state: N+1 sets `RefuseCorruptState`: a corrupt identity file then stops the daemon and stays where it is.
- ADR-0006 centralized: N+1 sets `mesh.StrictState` (class B findings refuse); N+2 sets `mesh.PersistFailureFatal` (an unwritable config file fails the node).

## Not verified

- No real mesh run: unit and script tests only. The 10-server test is the first real one.
- macOS: the route code and the `netstat` parser are built from the man page and hand-written fixtures. Not run on a real Mac. From this release a Mac gets route writes for the first time and no switch holds them back; roll back by installing the previous binary.
- The public mainline DHT: the cap of 8192 addresses and the sweeps are not measured against it.
- The mixed-reachability window at N+2 (about 3 reconcile cycles, about 15 s), the wake time after sleep and the keepalive cost of probe-only peers are from the code, not measured.
- Linux package upgrade tested on Debian 13 only. rpm is not run.
- darwin `F_FULLFSYNC` in the atomic file write: Go uses it, not verified on a Mac.

## Open human proofs

- ADR-0003 T6: the macOS route sign-off on a real Mac.
- ADR-0004 T4 S6: `pmset sleepnow`, then the time to the first successful ping to a peer.
- ADR-0004 T6 S2: `wg set <iface> peer <key> allowed-ips ""` on macOS userspace WireGuard (clears the list and keeps the endpoint on Linux, run there).

## Rollback and downgrade

- Everything at N is wire compatible: an older node talks to N nodes during the whole rollout, and state files keep their format (an older release ignores `.tmp-*`, `.corrupt-*` and the lock files).
- Go back by installing the previous binary or package. Kernel state is runtime state; the previous daemon re-applies its own.
- What a downgrade does not undo: a mesh IP that a collision moved is kept in the state file; the interface address is not changed back. A moved-aside identity stays aside; copy it back by hand.
- Do not run N and an older release on one host with the same state directory: the older one takes no lock.
- After a later release flips a switch, go back by setting it `false` in the next release. Nothing persisted needs rewriting, but an address removed at N+1 stays gone until you add it by hand.

## Known limits

- #861 (open): LAN discovery uses UDP 51830. A WireGuard `--listen-port` of 51830 collides with it; this release only warns.
- #853 (open): HELLO, REPLY and ANNOUNCE carry the full active peer list in one UDP datagram, so large peer lists do not fit.
- #859 (open): advertised routes with several advertisers (duplicate subnets, default route) can flap every reconcile; failover is not reliable. ADR-0003 chooses one owner per node, but this is not verified on a real mesh.
- The grinding limit: a member who holds the secret can generate key pairs until one derives to a victim's address (IP binding ties an address to a key and the secret, not to the key's owner).
- A `wg` call without a deadline remains in the 37 `ip`, `iptables` and `systemctl` calls.
- `install-service` does not validate `--accept-routes` or `--advertise-routes`: a bad list is first refused when the service starts, and `install-service` reports success while the unit restarts every 5 seconds.
- The lock files are never removed. NFS, some FUSE mounts and Docker Desktop bind mounts on macOS may not exclude.
- Other known limits are in the Known limits sections of each rollout page and in `docs/adr/BACKLOG.md`.
