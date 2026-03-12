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

---

## How do mesh secrets work?

**The secret is a passphrase, not a token.** It doesn't carry encoded metadata — every mesh parameter is *derived* from it using HKDF-SHA256.

### Why does any string work?

When you run `wgmesh init --secret`, it generates 32 random bytes (256 bits of entropy) and formats them as a `wgmesh://v1/<base64url>` URI. But the `wgmesh://v1/` prefix is cosmetic — `parseSecret()` strips it before key derivation.

What matters is the raw string underneath. `DeriveKeys()` accepts **any string of 16+ characters** and feeds it into HKDF-SHA256 to produce all mesh parameters. So yes, `MESH_SECRET=myfavoritepizza99` in a `.env` file will form a working mesh.

### What gets derived from the secret

From your one secret string, wgmesh derives **10 independent parameters** using domain-separated HKDF:

| Parameter | What it does |
|-----------|-------------|
| NetworkID | DHT discovery — how peers find each other |
| GossipKey | AES-256-GCM encryption for gossip messages |
| MeshSubnet | Deterministic 10.x.y.0/16 address range |
| IPv6 prefix | ULA /64 for the mesh |
| MulticastID | LAN discovery multicast group |
| PSK | WireGuard PresharedKey between peers |
| GossipPort | Gossip listener port |
| RendezvousID | DHT rendezvous point |
| MembershipKey | HMAC-based membership proofs |
| EpochSeed | Dandelion++ relay rotation |

**This is why different secrets create completely separate meshes.** Everything — the network identity, the subnet, the encryption keys, the discovery channels — is different. Two meshes with different secrets literally cannot find or communicate with each other.

### Why you should still use `wgmesh init --secret`

Auto-generated secrets have 256 bits of entropy from `crypto/rand`. A hand-typed passphrase has much less.

HKDF-SHA256 is **not a password-based KDF** — it has no iterations or memory hardness to slow down brute-force attacks (RFC 5869 §4). If an attacker can observe your mesh traffic (e.g., the DHT NetworkID), they can try candidate secrets offline at full speed.

| Secret type | Brute-force risk |
|-------------|-----------------|
| `wgmesh init --secret` (256 bits) | Infeasible |
| Long random passphrase (80+ bits) | Very hard |
| Short memorable phrase (20-40 bits) | Hours to days |

**Recommendation:** Always use `wgmesh init --secret` to generate your mesh secret. If you must use a custom string, make it long and random.

### The `wgmesh://v1/` prefix

The URI format is optional convenience:

```
wgmesh://v1/dGhpcyBpcyBhIHRlc3Qgc2VjcmV0IGZvcg?network=mynet
```

- `wgmesh://v1/` — stripped before key derivation
- The base64url payload — this is the actual secret
- `?query` params — stripped, reserved for future use

You can use the secret with or without the prefix — both produce the same mesh.
