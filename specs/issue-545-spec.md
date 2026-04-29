# Specification: Issue #545

## Classification
documentation

## Deliverables
documentation

## Problem Analysis

wgmesh's README and website prominently advertise "No central server, no coordination server,
no single point of failure." Users reading this reasonably conclude that decentralized mode
is fully peer-to-peer with zero reliance on any external infrastructure. In reality, decentralized
mode contacts two categories of external servers at startup:

1. **BitTorrent DHT bootstrap nodes** (defined in `pkg/discovery/dht.go`):
   ```go
   var DHTBootstrapNodes = []string{
       "router.bittorrent.com:6881",
       "router.utorrent.com:6881",
       "dht.transmissionbt.com:6881",
       "dht.libtorrent.org:25401",
   }
   ```
   These are contacted over UDP to bootstrap entry into the BitTorrent Mainline DHT network,
   which is how peers find each other across the internet. Without these (or reachable DHT peers
   already cached from a previous run), remote peer discovery will not work.

2. **STUN servers** (defined in `pkg/discovery/stun.go`):
   ```go
   var DefaultSTUNServers = []string{
       "stun.l.google.com:19302",
       "stun1.l.google.com:19302",
       "stun.cloudflare.com:3478",
   }
   ```
   These are contacted over UDP to detect the node's NAT type and server-reflexive (public)
   endpoint address. STUN is used exclusively in decentralized mode (via
   `DHTDiscovery.discoverExternalEndpoint()` and `DHTDiscovery.stunRefreshLoop()`).
   Centralized mode does not use STUN.

The "No central server" claim is accurate in the sense that there is no *wgmesh-specific*
infrastructure to host — but third-party public DHT and STUN servers are contacted. This nuance
is not documented anywhere. The issue asks for an explicit, user-visible statement about these
connections.

Three documentation files need updating:

- `README.md` — the primary entry point; the `## Security Considerations` section or the
  `## Usage → Decentralized Mode` section is the right place for a concise disclosure.
- `docs/quickstart.md` — the step-by-step guide; the "What happens after `join`" list already
  mentions "DHT (BitTorrent Mainline)" but does not name the bootstrap servers or STUN.
- `docs/FAQ.md` — the natural place for a dedicated "Does wgmesh contact external servers?"
  Q&A entry.

## Implementation Tasks

### Task 1: Add external-server disclosure to `README.md`

In `README.md`, locate the `## Security Considerations` section. Its current content starts with:

```markdown
## Security Considerations

- **Centralized mode**: Keys stored in `mesh-state.json` — use `--encrypt` for AES-256-GCM encryption. See [ENCRYPTION.md](ENCRYPTION.md).
- **Decentralized mode**: Each node stores its keypair in `/var/lib/wgmesh/{interface}.json` with `0600` permissions.
- WireGuard traffic is encrypted end-to-end.
```

Insert the following new bullet point **immediately before** the line `- WireGuard traffic is encrypted end-to-end.`:

```markdown
- **Decentralized mode contacts external servers**: when running `wgmesh join`, the daemon
  connects to public BitTorrent DHT bootstrap nodes (`router.bittorrent.com:6881`,
  `router.utorrent.com:6881`, `dht.transmissionbt.com:6881`, `dht.libtorrent.org:25401`) over
  UDP to enter the DHT network for peer discovery, and to public STUN servers
  (`stun.l.google.com:19302`, `stun1.l.google.com:19302`, `stun.cloudflare.com:3478`) over
  UDP to detect the node's public IP and NAT type. These are well-known, freely available
  third-party services; no wgmesh-specific servers are involved. Centralized mode does not
  use DHT or STUN.
```

The resulting `## Security Considerations` block must look exactly like this (preserve all pre-existing bullets; the three bullets above and below the insertion are shown for context):

```markdown
## Security Considerations

- **Centralized mode**: Keys stored in `mesh-state.json` — use `--encrypt` for AES-256-GCM encryption. See [ENCRYPTION.md](ENCRYPTION.md).
- **Decentralized mode**: Each node stores its keypair in `/var/lib/wgmesh/{interface}.json` with `0600` permissions.
- **Decentralized mode contacts external servers**: when running `wgmesh join`, the daemon
  connects to public BitTorrent DHT bootstrap nodes (`router.bittorrent.com:6881`,
  `router.utorrent.com:6881`, `dht.transmissionbt.com:6881`, `dht.libtorrent.org:25401`) over
  UDP to enter the DHT network for peer discovery, and to public STUN servers
  (`stun.l.google.com:19302`, `stun1.l.google.com:19302`, `stun.cloudflare.com:3478`) over
  UDP to detect the node's public IP and NAT type. These are well-known, freely available
  third-party services; no wgmesh-specific servers are involved. Centralized mode does not
  use DHT or STUN.
- WireGuard traffic is encrypted end-to-end.
- **SSH authentication**: The tool tries the SSH agent first (`SSH_AUTH_SOCK`), then `~/.ssh/id_rsa`, `~/.ssh/id_ed25519`, and `~/.ssh/id_ecdsa`.
- The tool currently uses `InsecureIgnoreHostKey` for SSH — consider implementing proper host key verification for production.
```

---

### Task 2: Update `docs/quickstart.md` — expand the "What happens after `join`" list

In `docs/quickstart.md`, locate the `### What happens after \`join\`` subsection. Its current content is:

```markdown
### What happens after `join`

1. wgmesh derives a deterministic mesh subnet and WireGuard PSK from your secret.
2. A WireGuard keypair is generated for this node and stored in `/var/lib/wgmesh/wg0.json`.
3. The daemon announces itself on three discovery channels simultaneously:
   - **DHT (BitTorrent Mainline)** — finds peers across the internet
   - **LAN multicast** — instantly finds peers on the same local network
   - **GitHub Issues registry** — bootstraps cold-start discovery
4. As peers are found, WireGuard configuration is applied live (no interface restarts).
5. NAT traversal (UDP hole-punching) is attempted for peers behind NAT.
```

Replace the entire subsection with the following (changes: expand the DHT bullet to name the
bootstrap servers, add a STUN bullet, and add a network-notice paragraph at the end):

```markdown
### What happens after `join`

1. wgmesh derives a deterministic mesh subnet and WireGuard PSK from your secret.
2. A WireGuard keypair is generated for this node and stored in `/var/lib/wgmesh/wg0.json`.
3. The daemon announces itself on three discovery channels simultaneously:
   - **DHT (BitTorrent Mainline)** — contacts public bootstrap nodes
     (`router.bittorrent.com:6881`, `router.utorrent.com:6881`,
     `dht.transmissionbt.com:6881`, `dht.libtorrent.org:25401`) over UDP to enter the
     global DHT network and find remote peers.
   - **LAN multicast** — instantly finds peers on the same local network (no external
     connections required).
   - **GitHub Issues registry** — bootstraps cold-start discovery via the GitHub API.
4. The daemon queries public STUN servers (`stun.l.google.com:19302`,
   `stun1.l.google.com:19302`, `stun.cloudflare.com:3478`) over UDP to detect the
   node's public IP address and NAT type.
5. As peers are found, WireGuard configuration is applied live (no interface restarts).
6. NAT traversal (UDP hole-punching) is attempted for peers behind NAT.

> **Note — external network connections:** decentralized mode contacts third-party DHT and
> STUN servers at startup and periodically during operation (see steps 3 and 4 above). No
> wgmesh-specific servers are used. All peer-to-peer WireGuard traffic is encrypted
> end-to-end and never passes through these servers. See
> [FAQ — Does wgmesh contact external servers?](FAQ.md#does-wgmesh-contact-external-servers)
> for details.
```

---

### Task 3: Add a FAQ entry in `docs/FAQ.md`

In `docs/FAQ.md`, append the following section at the **end of the file** (after the last `---`
separator, or after the last Q&A block if there is no trailing separator):

```markdown
---

## Does wgmesh contact external servers?

**Yes — in decentralized mode only.** Centralized mode (SSH deployment) makes no external
connections beyond the SSH sessions you configure.

When you run `wgmesh join`, the daemon contacts two categories of external servers:

### DHT bootstrap nodes

To discover remote peers, wgmesh joins the BitTorrent Mainline DHT network. On startup it
contacts these well-known bootstrap nodes over **UDP**:

| Server | Port | Operator |
|---|---|---|
| `router.bittorrent.com` | 6881 | BitTorrent Inc. |
| `router.utorrent.com` | 6881 | BitTorrent Inc. |
| `dht.transmissionbt.com` | 6881 | Transmission project |
| `dht.libtorrent.org` | 25401 | libtorrent project |

Once bootstrapped, the daemon communicates directly with other DHT peers (including other
wgmesh nodes) without going back to the bootstrap nodes. The bootstrap nodes are contacted
again only on daemon restart or after a long disconnection.

### STUN servers

To detect the node's public IP address and NAT type, wgmesh sends a short UDP probe to:

| Server | Port | Operator |
|---|---|---|
| `stun.l.google.com` | 19302 | Google |
| `stun1.l.google.com` | 19302 | Google |
| `stun.cloudflare.com` | 3478 | Cloudflare |

STUN probes are sent on startup and periodically refreshed. The probe is a single small UDP
packet; the server replies with your public IP and port. No connection data, secrets, or
WireGuard keys are sent to STUN servers.

### What these servers do NOT see

- Your WireGuard private key or mesh secret
- The content of any WireGuard traffic (encrypted end-to-end between peers)
- Your peer list or mesh topology (this is exchanged directly between wgmesh nodes
  over encrypted channels)

The DHT bootstrap nodes see only that a DHT client is bootstrapping — the same information
any BitTorrent client reveals. The STUN servers see only your IP and port, which is the
minimum required for NAT traversal.

### Can I use custom servers?

Not yet via a flag, but you can change the defaults at compile time by modifying:

- `pkg/discovery/dht.go` — `DHTBootstrapNodes` slice
- `pkg/discovery/stun.go` — `DefaultSTUNServers` slice

A future release may expose these as runtime configuration options.
```

---

## Affected Files

- **Modified:** `README.md` — add external-server disclosure bullet in `## Security Considerations`
- **Modified:** `docs/quickstart.md` — expand "What happens after `join`" list to name servers,
  add STUN step, add network-notice callout block
- **Modified:** `docs/FAQ.md` — append new "Does wgmesh contact external servers?" Q&A entry

No code files are changed. No Go packages are touched. No new dependencies.

## Test Strategy

No automated tests required for documentation. Verify manually:

1. `README.md` renders without broken Markdown. Confirm the new bullet appears inside
   `## Security Considerations` between the "Decentralized mode keypair" bullet and the
   "WireGuard traffic" bullet.
2. `docs/quickstart.md` renders correctly. Confirm the numbered list under
   "What happens after `join`" now has 6 items and includes the STUN step. Confirm the
   `> **Note**` callout block appears immediately after the list.
3. The FAQ link `FAQ.md#does-wgmesh-contact-external-servers` in `docs/quickstart.md`
   resolves to the new FAQ section (GitHub auto-generates anchors from headings).
4. `docs/FAQ.md` renders the new section with a correctly formatted table (4 columns line up).
5. All existing content in all three files is preserved unchanged.

## Estimated Complexity
low

**Reasoning:** Pure documentation. Three targeted edits to existing Markdown files — one new
bullet in README.md (~10 lines), expanded list + callout in quickstart.md (~20 lines replaced),
and one new FAQ entry (~55 lines appended). No code changes, no dependency updates, no build
pipeline changes. Estimated effort: 30–45 minutes.
