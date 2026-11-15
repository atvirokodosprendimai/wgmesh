# Release notes: next release (draft)

For operators. No dates are set: "next release" and "the release after" are the only schedule. PR numbers are merged PRs on `main`.

## Upgrade actions

- Centralized `wgmesh -deploy`: the first run after upgrading needs every node's SSH host key in `~/.ssh/known_hosts` (PR #876; before, host keys were ignored).
- Add a key with `ssh-keyscan -H <host> >> ~/.ssh/known_hosts` after checking its fingerprint.
- Use another file with `WGMESH_SSH_KNOWN_HOSTS`. `WGMESH_SSH_INSECURE_HOST_KEY=1` turns verification off for labs and prints a warning.
- Decentralized systemd nodes: the unit no longer carries the secret in `ExecStart` (#876). The secret is read from `/etc/wgmesh/secret.env` (mode 0600) as `WGMESH_SECRET`.
- Existing nodes keep the old unit until it is rewritten: run `wgmesh install-service --secret <SECRET>` again, then `systemctl restart wgmesh` (install runs `enable` and `start`, which does not restart a running unit).
- Docker Compose examples pass `WGMESH_SECRET` from `MESH_SECRET`, and compose fails at once when `MESH_SECRET` is unset (#882).

## Behaviour changes

- Envelope message type is authenticated and replays are refused, staged (ADR-0001, `docs/envelope-rollout.md`, PR #877). This release is N.
- N: readers open both forms. GOODBYE is the only message sealed in the new form; all other writers are unchanged (`pkg/discovery/exchange.go`). A node older than N cannot open the new GOODBYE and removes that peer through its health check a few minutes later.
- A peer's own announcement with an empty route list now withdraws its routes (#881).
- `PeerStore.Unsubscribe` no longer closes the subscriber channel (#881). Library users must not wait for the channel to close.
- Centralized re-deploy ignores a runtime-learned endpoint when none is desired (#883).
- A node keeps its persisted mesh IP when no `--mesh-subnet` is set, also after `--mesh-subnet` is dropped (#880, #887).
- `service add` registers that persisted IP (#880).
- A network a node advertises itself is never routed through the mesh (#879, #885). This covers HA shared subnets and the default routes of HA exit nodes (each drops the other's `0.0.0.0/0`).
- A decentralized node restores its connected LAN route when it removes such a via route (#885).
- LAN discovery logs a warning when `--listen-port` equals 51830 (#886).
- IPv4-only start when IPv6 is disabled (#878).
- Fixes: centralized deploy deadlock (#842), `wg show dump` keepalive parse (#846), `rotate-secret` now fails with an honest error instead of faking success (#847).
- Hardening in #876: probe reads and request size are bounded; advertised routes and exchange announcements are validated.

## Staged plan ahead

- Envelope N+1 (the release after): all writers seal the new form. Nodes older than N cannot open the new packets, so upgrade every node to N first (`docs/adr/BACKLOG.md`).
- Envelope N+2 (the release after N+1): legacy envelopes are refused, and nodes older than N+1 are cut off. Watch `LegacyEnvelopeOpens` before this step.
- ADR-0002, admit step (`docs/adr/ADR-0002-admit-step/`, Accepted 2026-11-15): mesh IP and route binding is warned at N.
- IP binding is enforced at N+1 and route binding at N+2 (`accept-routes`; an unset list stays open).
- ADR-0002 is accepted, not built: its tasks are not in this release.

## Known limits

- #861 (open): LAN discovery uses UDP 51830. A WireGuard `--listen-port` of 51830 collides with it; this release only warns.
- #853 (open): HELLO, REPLY and ANNOUNCE carry the full active peer list in one UDP datagram, so large peer lists do not fit.
- #859 (open): advertised routes with several advertisers (duplicate subnets, default route) can flap every reconcile; failover is not reliable.
