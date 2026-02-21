# Pull — wgmesh codebase overview

**Date:** 2026-02-20
**Mode:** multi-pass overview

---

## What the system is

wgmesh is a WireGuard mesh network builder with two completely distinct operational modes that share a binary.

---

## Major subsections / concerns

### 1. Dual-mode CLI entry point
**Files:** `main.go`
The single binary dispatches between two paradigms at startup: flag-based "centralized mode" (SSH deployment operator tool) and subcommand-based "decentralized mode" (long-running daemon). RPC server setup also lives here.

---

### 2. Centralized mesh management
**Files:** `pkg/mesh/`, `pkg/wireguard/`, `pkg/ssh/`
Operator-centric, SSH-based workflow. Manages a `mesh-state.json` file; deploys WireGuard configs to remote Ubuntu nodes via SSH. Supports group/policy access control, NAT detection, diff-based incremental updates, optional AES-256-GCM encrypted state.

---

### 3. Daemon core (decentralized mode)
**Files:** `pkg/daemon/`
The long-running node process. Owns the reconciliation loop, peer store, WireGuard interface lifecycle, health probing (TCP probe over a side-channel port), relay routing, epoch-based membership, IP collision resolution, and systemd service integration.

---

### 4. Discovery subsystem
**Files:** `pkg/discovery/`
Multiple peer-discovery mechanisms that feed the daemon: BitTorrent mainline DHT (primary), LAN multicast, in-mesh gossip, STUN endpoint detection, direct encrypted peer exchange, and rendezvous/NAT hole-punching introducer protocol.

---

### 5. Crypto & identity layer
**Files:** `pkg/crypto/`
Derives all mesh keys (WireGuard keypair, gossip key, network ID, mesh subnet, rendezvous ID) deterministically from a single shared secret. Provides envelope encryption for peer exchange messages, membership proofs, and secret rotation announcements.

---

### 6. Privacy layer
**Files:** `pkg/privacy/`
Dandelion++ protocol: message propagation with stem/fluff phases to obscure the origin of peer announcements.

---

### 7. RPC subsystem
**Files:** `pkg/rpc/`
Unix socket JSON-RPC server/client for querying a running daemon (`peers list`, `peers count`, `peers get`). Decoupled via a callback interface.

---

### 8. Lighthouse (control plane server)
**Files:** `pkg/lighthouse/`, `cmd/lighthouse/`
Separate server binary. REST API for peer registry, DNS management (Hetzner DNS), XDS (Envoy control plane), health reporting. Rate-limited, token-auth'd. Appears to serve `cloudroof.eu`.

---

### 9. Chimney (dashboard origin server)
**Files:** `cmd/chimney/`
Standalone HTTP server for the project dashboard. Serves static HTML and proxies GitHub REST API with server-side caching in Dragonfly (Redis-compatible) + in-memory fallback. Unrelated to mesh operation.

---

### 10. Infrastructure & CI/CD
**Files:** `.github/workflows/`, `deploy/`, `testlab/`
GitHub Actions for builds, Docker images, Chimney/Lighthouse deploy, DNS updates, auto-merge. Hetzner cloud provisioning scripts. Vagrant/Lima testlab for local mesh testing.

---

### 11. Auxiliary packages
**Files:** `pkg/routes/`, `pkg/ratelimit/`, `pkg/proxy/`
Route computation helpers, IP rate limiter (used by Lighthouse API), and a simple proxy.

---

## Existing specs

The project has a `specs/` directory with issue-linked spec files (implemented/partial/not-implemented). These are older, issue-focused specs — not the eidos format. They should be reviewed when writing eidos specs to avoid duplication.

---

## How subsections relate

```
shared secret
     │
     ▼
 pkg/crypto  ──────────────────────────────────────────────────────────┐
     │                                                                  │
     ▼                                                                  │
pkg/daemon ◄── pkg/discovery (DHT, LAN, gossip, STUN, exchange, RDZ) │
     │                                                                  │
     │── pkg/privacy (Dandelion++)                                      │
     │── pkg/wireguard (interface config/apply)                         │
     │── pkg/rpc (unix socket query API)                                │
     │                                                                  │
pkg/mesh ◄── pkg/ssh ◄── pkg/wireguard (centralized SSH deploy)       │
     │                                                                  │
cmd/lighthouse ◄── pkg/lighthouse ◄── pkg/ratelimit                   │
cmd/chimney (standalone, github proxy + dashboard)                     │
```

---

## Recommended subsection breakdown for full pull

| # | Subsection | Files |
|---|-----------|-------|
| A | Centralized mesh management | `pkg/mesh/`, `pkg/wireguard/`, `pkg/ssh/` |
| B | Daemon core | `pkg/daemon/` |
| C | Discovery subsystem | `pkg/discovery/` |
| D | Crypto & identity | `pkg/crypto/` |
| E | Privacy layer | `pkg/privacy/` |
| F | RPC subsystem | `pkg/rpc/` |
| G | Lighthouse control plane | `pkg/lighthouse/`, `cmd/lighthouse/` |
| H | Chimney dashboard server | `cmd/chimney/` |
| I | CLI entry point | `main.go` |
