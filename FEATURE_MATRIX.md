# Feature Matrix

Two views of the same underlying inventory.

1. **Competitive matrix** — wgmesh against the tools people actually consider when they shop for "WireGuard mesh."
2. **Internal feature inventory** — what wgmesh ships today, by mode, with status.

---

## 1. Competitive Matrix

Scoring legend: ✅ shipped · 🟡 partial / limited · 🔧 self-hostable but bring-your-own · ❌ no.

The "honest" column is where wgmesh is behind. Don't hide it on the landing page; the people we want as users will catch it.

| Capability | wgmesh | Tailscale | Headscale | Netbird | ZeroTier | Innernet | Nebula | EasyTier |
|---|---|---|---|---|---|---|---|---|
| **Coordination server required** | ❌ none (DHT) | ✅ SaaS | 🔧 self-host | ✅ SaaS or self-host | ✅ Earth roots | ✅ central server | 🔧 lighthouses | 🔧 trackers |
| **Bootstrap UX** | one secret | account + login | account + login | account + login | network ID + auth | invitation files | cert authority + config | name + secret |
| **NAT traversal** | ✅ DHT-driven | ✅ DERP relays | 🟡 needs DERP | ✅ Coturn/STUN | ✅ Earth relays | 🟡 limited | ✅ lighthouses | ✅ public relays |
| **Works fully offline (LAN)** | ✅ multicast | ❌ needs control plane | ❌ needs control plane | ❌ needs control plane | ❌ needs Earth | ❌ needs server | 🟡 needs lighthouse | 🟡 needs tracker |
| **Routable subnets** | ✅ both modes | ✅ subnet routers | ✅ | ✅ | ✅ bridging | ✅ CIDRs | ✅ unsafe_routes | ✅ |
| **ACLs / segmentation** | ✅ centralized only | ✅ HuJSON ACLs | ✅ | ✅ groups + rules | ✅ flow rules | ✅ CIDR-based | ✅ groups | 🟡 |
| **Magic DNS / name resolution** | ❌ | ✅ | ✅ | ✅ | 🟡 | ❌ | ❌ | 🟡 |
| **Mobile clients (iOS/Android)** | ❌ | ✅ | ✅ via Tailscale app | ✅ | ✅ | ❌ | ❌ | ❌ |
| **Web dashboard / GUI** | 🟡 chimney (read-only) | ✅ | 🟡 | ✅ | ✅ | ❌ | ❌ | 🟡 |
| **SSO / OIDC / SAML** | ❌ | ✅ | 🟡 OIDC | ✅ | ✅ | ❌ | ❌ | ❌ |
| **Audit logs** | ❌ | ✅ | 🟡 | ✅ | ✅ | ❌ | 🟡 | ❌ |
| **Open source** | ✅ MIT | 🟡 client only | ✅ BSD-3 | ✅ BSD-3 | ✅ BSL-1.1 | ✅ MIT | ✅ MIT | ✅ Apache-2 |
| **Self-hostable end-to-end** | ✅ no server at all | ❌ | ✅ | ✅ | 🟡 | ✅ | ✅ | ✅ |
| **Encrypted state at rest** | ✅ AES-256-GCM | n/a | 🟡 DB-level | 🟡 DB-level | n/a | 🟡 | 🟡 | ❌ |
| **EU-based / GDPR-friendly** | ✅ Hetzner, deSEC | ❌ US | depends on host | depends on host | ❌ US | depends on host | depends on host | depends on host |
| **Vendor lock-in risk** | low | high | low | medium | high | low | low | medium |
| **Time-to-mesh (LAN)** | <5s | ~30s | ~30s | ~30s | ~30s | manual | manual | ~30s |
| **Time-to-mesh (Internet)** | 15–60s | 5–10s | 5–10s | 5–10s | 5–10s | manual | 10–30s | 30–60s |
| **CLI-only operation** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Maintained by** | solo + AI agents | YC company | community | YC company | ZeroTier Inc. | Tonari | Defined Networking | community |

### Where wgmesh wins

- **No coordination server.** This is the thing nobody else has. The cost of running Tailscale-the-protocol at home is "trust Tailscale's control plane or run Headscale." wgmesh removes the question.
- **Works offline.** A LAN with no internet still meshes. None of the SaaS tools do this.
- **One secret, no accounts.** No OIDC dance, no email verification, no admin console. Useful for ad-hoc, ephemeral, or air-gapped setups.
- **Encrypted state file.** Vault-friendly out of the box for centralized mode.
- **EU-first.** Hetzner, deSEC, Migadu — matters for buyers who care about data residency.

### Where wgmesh loses (don't pretend otherwise)

- **No mobile.** Tailscale on an iPhone is the killer feature for road warriors. wgmesh can't do it yet.
- **No SSO.** Means it's not enterprise-shaped. Means your buyer is a homelabber, not a CISO.
- **No magic DNS.** People grow to love `ssh node1` instead of `ssh 10.99.0.1`. We don't do that yet.
- **No managed offering.** Tailscale's free tier solves "I just want it to work." We don't have that path.
- **Smaller community.** Tailscale has tens of thousands of GitHub stars, Discord channels, blog posts. We have ~200 stars and a Show HN draft.
- **DHT discovery is slower.** 15–60s vs Tailscale's 5–10s. The control-server tools win on time-to-first-connection.

### Positioning that holds up

> wgmesh is for people who'd rather operate a mesh than rent one. If your reaction to "Tailscale's control plane" is "I'd rather not," wgmesh is the tool. If your reaction is "fine, sounds easy," Tailscale is the tool.

---

## 2. Internal Feature Inventory

Two operating modes, two surface areas. Status: ✅ shipped · 🟡 partial · 🔧 in-flight · 📋 spec'd not built · ❌ not started.

### Centralized mode (`pkg/mesh`, `pkg/ssh`)

| Feature | Status | Notes |
|---|---|---|
| `init / add / remove / list / deploy` CLI | ✅ | The original surface |
| State file (`mesh-state.json`) | ✅ | JSON, auto-created at `/var/lib/wgmesh/` |
| Encrypted state (`--encrypt`) | ✅ | AES-256-GCM + PBKDF2 100k iters, vault-friendly |
| Diff-based `wg set` updates | ✅ | No interface restart |
| SSH-based config push | ✅ | Tries agent, then `id_rsa`, `id_ed25519`, `id_ecdsa` |
| **SSH host key verification** | ❌ | `InsecureIgnoreHostKey` — gap in security posture |
| Auto-install of WireGuard on remote | ✅ | apt/yum/dnf detection |
| NAT detection (compare SSH host vs public IP) | ✅ | |
| `routable_networks` (subnet routing) | ✅ | Both direct and via-peer routes |
| Group-based access control | ✅ | Issue #176; policies + `AllowedIPs` enforcement |
| systemd persistence (`wg-quick@wg0`) | ✅ | |
| Custom mesh subnet | ✅ | Default `10.99.0.0/16` |

### Decentralized mode (`pkg/daemon`, `pkg/discovery`, `pkg/crypto`)

| Feature | Status | Notes |
|---|---|---|
| `init --secret / join / status / qr` | ✅ | |
| `wgmesh://v1/<base64>` token format | ✅ | |
| HKDF-derived keys (`pkg/crypto/derive.go`) | ✅ | network_id, gossip_key, mesh_subnet, multicast_id, PSK |
| Deterministic mesh IP from pubkey | ✅ | Collision resolution by lexicographic pubkey |
| 5-second reconcile loop | ✅ | `pkg/daemon/daemon.go` |
| SIGHUP hot reload | ✅ | `advertise-routes`, `log-level` |
| Persistent peer cache | ✅ | Survives daemon restart |
| Discovery L0: GitHub Issue rendezvous | ✅ | `pkg/discovery/registry.go` |
| Discovery L1: LAN multicast | ✅ | `239.192.x.x` derived from secret |
| Discovery L2: BitTorrent DHT | ✅ | `anacrolix/dht/v2`, hourly infohash rotation |
| Discovery L3: In-mesh gossip | ✅ | `--gossip` flag |
| Encrypted peer exchange | ✅ | AES-256-GCM envelopes |
| Dandelion++ stem-fluff routing | ✅ | `pkg/privacy/dandelion.go` |
| Membership tokens | ✅ | `pkg/crypto/membership.go` |
| Epoch management | ✅ | `pkg/daemon/epoch.go` |
| Relay routing fallback | 🟡 | Spec exists, partially wired |
| RPC: Unix socket JSON-RPC | ✅ | `peers.list/get/count`, `daemon.status/ping` |
| `wgmesh peers list` table | 🟡 | Truncated pubkeys (#178), no hostname (#81 phase 3) |
| `test-peer` connectivity probe | ✅ | |
| Hostname in peer announcements | 🟡 | Field exists in `PeerInfo`, not in wire format |
| **ACLs / segmentation** | ❌ | Centralized has it, decentralized doesn't |
| **Secret rotation protocol** | 📋 | In `features/archived/bootstrap.md`, deferred |
| **STUN integration** | 📋 | Open question in archived plan |
| **TURN/relay fallback for CGNAT** | 📋 | Open question |
| **IPv6 mesh subnet** | ❌ | |
| **DNS / name resolution** | ❌ | No magic-DNS equivalent |

### Operational surface

| Item | Status | Notes |
|---|---|---|
| Linux builds (amd64, arm64, armv7) | ✅ | goreleaser on `v*.*.*` |
| Darwin builds (amd64, arm64) | ✅ | Issue #24 |
| `.deb` / `.rpm` packages | ✅ | |
| Homebrew formula | ✅ | |
| Docker image (`ghcr.io/...`) | ✅ | |
| Docker Compose recipe | ✅ | `DOCKER-COMPOSE.md` |
| Nix package | 🔧 | Plan in `memory/plan - 2603012134 - distributable packages...` |
| systemd unit generation | 🟡 | Centralized only |
| `wgmesh install-service` | 📋 | Phase 4 of original plan |

### Adjacent / business surface

| Item | Status | Notes |
|---|---|---|
| Chimney dashboard | ✅ | Extracted to its own repo; live at chimney.beerpub.dev |
| Lighthouse CDN control plane | ✅ | Extracted; not yet productized |
| `lighthouse-go` SDK | ✅ | `wgmesh service` subcommand uses it |
| `wgmesh service add` CLI | 📋 | Issue #372, in triage |
| Autonomous AI dev pipeline | ✅ | Goose builds, Copilot reviews, auto-merge |
| Landing page (cloudroof.eu) | 🟡 | Placeholder; needs repositioning per first-customer brainstorm |
| Polar.sh billing | 🔧 | Issue #376, needs-human |
| Cost tracking | 🔧 | Issue #373, `costs.json` has nulls |
| GitHub Sponsors tiers | ✅ | Contributor $5 / Edge Node $20 / Mesh Operator $100 |
| First paying customer | ❌ | The thing this whole thing is about |

---

## What this matrix tells us

Three things stand out when you read both halves together.

The first is that **the technical product is much more complete than the commercial product**. The internal inventory has a lot of green; the business surface has almost none. The gap to first paying customer is not "build more features," it's "wire up Polar.sh, ship the Show HN, find one human."

The second is that **the competitive losses cluster in one place: the surfaces a non-technical buyer sees**. No mobile, no SSO, no GUI, no magic DNS. If the ICP is homelabbers and small ops teams who already think in terminals, this matters less. If the ICP becomes "the CTO of a 50-person company," every red cell becomes a deal-blocker.

The third is that **the differentiator is structural, not feature-based**. You can't add "no coordination server" to Tailscale; that's the architecture. Everyone else's roadmap can copy our checkboxes; nobody else's roadmap can copy the absence of a control plane. That's the moat. Defend it in marketing.
