# Running wgmesh as a macOS service (Homebrew / launchd)

Since v0.3, the Homebrew formula ships a **launchd service block**: `brew services
start wgmesh` runs the mesh daemon as a root LaunchDaemon that starts at boot and
restarts if it crashes.

This is the macOS counterpart of the Linux systemd flow
(`wgmesh install-service` / `packaging/wgmesh.service`).

## Prerequisites

- macOS (Intel or Apple Silicon) with Homebrew installed
- The formula installs `wireguard-go` and `wireguard-tools` automatically
  (both are required on macOS: `wireguard-go` creates the `utun` interface,
  `wg` configures it)

## Install

```bash
brew install atvirokodosprendimai/tap/wgmesh
```

The install step creates the two writable directories under the Homebrew prefix:

| Path | Purpose |
|------|---------|
| `$(brew --prefix)/etc/wgmesh` | secret file (read by the daemon) |
| `$(brew --prefix)/var/wgmesh` | account + services state |
| `/var/lib/wgmesh` | node identity (WireGuard keypair, created by the root daemon) |
| `$(brew --prefix)/var/log/wgmesh.log` | daemon logs |

## Bootstrap (one time)

Generate a mesh secret and save it where the service expects it:

```bash
wgmesh init --secret --save-to "$(brew --prefix)/etc/wgmesh/secret"
```

`--save-to` writes the `wgmesh://v1/...` URI to the file with `0600`
permissions (owner-only), creating the parent directory if needed. The file is
read on every daemon start via the `WGMESH_SECRET_FILE` environment variable —
there is no re-join needed across restarts.

## Start

```bash
brew services start wgmesh
```

Because wgmesh must create the `utun` interface and install routes, the service
runs **as root** (`require_root` in the formula). Homebrew will prompt for your
password the first time; the resulting plist is a LaunchDaemon in
`/Library/LaunchDaemons`.

Verify:

```bash
brew services list                 # wgmesh should show "started"
wgmesh peers list                  # requires the daemon RPC socket
tail -f "$(brew --prefix)/var/log/wgmesh.log"
```

## Manage

```bash
brew services stop wgmesh          # stop (launchd kills the daemon)
brew services restart wgmesh       # restart
brew uninstall wgmesh              # stop + remove binary and dirs
```

## Troubleshooting

- **`wireguard-go not found in PATH`** — the formula depends on
  `wireguard-go`; if you installed wgmesh another way, run
  `brew install wireguard-go wireguard-tools`.
- **`Error reading secret file`** — the secret file is missing or unreadable.
  Re-run the bootstrap step above.
- **Daemon exits immediately at boot** — check the log file; the most common
  cause is a missing secret or a stale `/var/lib/wgmesh` state file. Remove
  `/var/lib/wgmesh/<iface>.json` only if you want a fresh identity.
- **`Operation not permitted`** — the service must run as root; do not run
  `wgmesh join` manually as your user while the service is managing the
  interface.

## Security notes

- The secret file lives in the Homebrew prefix, user-owned, with `0600`
  permissions — it never appears in the process list or in the plist.
- The daemon runs as root (required for `utun` + routing); the formula
  installs no privileged helper and creates no world-writable state.
