# FAQ

## Can I use a custom WireGuard interface name?

**Yes on Linux, with restrictions. On macOS, only `utunN` names work.**

By default, wgmesh uses `wg0` on Linux and `utun20` on macOS.
You can override this with the `--interface` flag:

```bash
wgmesh join --secret <secret> --interface cloudroof0
```

### Naming rules

**Linux:**
- Must start with a letter (`a-z`, `A-Z`)
- Can contain letters, digits, underscores, and hyphens
- Maximum 15 characters (kernel IFNAMSIZ limit)
- Examples: `wg0`, `cloudroof0`, `mesh-1`, `corp_vpn`

**macOS:**
- Must follow the `utunN` pattern (e.g. `utun20`, `utun99`)
- This is a `wireguard-go` requirement — macOS WireGuard only creates utun interfaces

### What gets rejected

| Input | Reason |
|-------|--------|
| `0wg` | Must start with a letter |
| `-custom` | Must start with a letter |
| `this-is-way-too-long` | Exceeds 15-character limit (Linux) |
| `foo/bar` | Path separators not allowed |
| `wg0;evil` | Shell metacharacters not allowed |
| `cloudroof0` (on macOS) | Must use `utunN` pattern |

### How the name is used

The interface name appears in several places:
- WireGuard device name visible in `ip link` / `ifconfig`
- State file: `/var/lib/wgmesh/<name>.json`
- Peer cache: `/var/lib/wgmesh/<name>-peers.json`
- Systemd unit: `--interface <name>` in ExecStart (if not default)

The interface name is **not hot-reloadable** — changing it requires a daemon restart.

### Systemd service

When you install the systemd service with a custom interface:

```bash
wgmesh install-service --secret <secret> --interface mesh0
```

The generated unit file includes `--interface mesh0` in ExecStart.
Only one wgmesh service can run per host (the unit name is `wgmesh.service`, not parameterised by interface).
