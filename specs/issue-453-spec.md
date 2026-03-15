# Specification: Issue #453

## Classification
feature

## Deliverables
documentation

## Problem Analysis

wgmesh is currently in a "Foundation" stage where the core decentralized daemon mode and most
discovery layers exist in code, but there is no single document that defines what "working
end-to-end" means, what the MVP feature set is, or what constitutes an exit from the Foundation
stage into a shippable product.

The codebase already implements:
- **Decentralized mode**: `pkg/daemon/` — reconcile loop, peer store, collision detection, relay,
  health checks, epoch management, systemd integration, hot reload via SIGHUP
- **Discovery layers**: `pkg/discovery/` — L0 GitHub registry (`registry.go`), L1 LAN multicast
  (`lan.go`), L2 BitTorrent DHT (`dht.go`), L3 in-mesh gossip (`gossip.go`), STUN
  (`stun.go`), encrypted peer exchange (`exchange.go`)
- **Cryptography**: `pkg/crypto/` — HKDF key derivation (`derive.go`), AES-256-GCM envelopes
  (`envelope.go`), HMAC membership tokens (`membership.go`)
- **Centralized mode**: `pkg/mesh/`, `pkg/ssh/`, `pkg/wireguard/` — SSH deploy, diff-based
  WireGuard apply, state file encryption
- **RPC**: `pkg/rpc/` — Unix socket JSON-RPC server with `peers.list`, `peers.get`,
  `peers.count`, `daemon.status`, `daemon.ping`
- **CDN / ingress**: `service.go` — `wgmesh service add/list/remove` backed by the
  `lighthouse-go` SDK pointing at a Lighthouse API
- **CLI**: `main.go` subcommands: `version`, `join`, `init`, `status`, `test-peer`, `qr`,
  `install-service`, `uninstall-service`, `rotate-secret`, `mesh`, `peers`, `service`

What is **missing** is a formal definition of:
1. Which of those features must be fully working for MVP (vs. nice-to-have)
2. Concrete, testable exit criteria for each component
3. Epic issue structure to track remaining work
4. A canonical definition of "working end-to-end"

## Implementation Tasks

### Task 1: Create `docs/mvp.md` — MVP scope definition document

**File to create**: `docs/mvp.md`

**Content requirements** (write every section listed below; do not skip any):

```markdown
---
title: wgmesh MVP — Scope and Exit Criteria
type: specification
status: active
date: <today>
---

# wgmesh MVP — Scope and Exit Criteria
```

#### Section 1: Definition of "Working End-to-End"

Write a section titled `## Definition of "Working End-to-End"` that states the following
two scenarios must both pass without manual WireGuard configuration:

**Scenario A — Two nodes, same LAN**
1. Run `wgmesh init --secret` on one machine; copy the printed URI.
2. Run `wgmesh join --secret <URI>` on both machines (as root).
3. Within **10 seconds** both machines can `ping` each other's mesh IP.
4. `wgmesh peers list` on either machine shows the remote peer with status `active`.
5. After rebooting both machines, the mesh re-forms within **60 seconds** without any
   manual intervention.

**Scenario B — Two nodes, different networks (internet, no LAN path)**
1. Use the same secret on both nodes (no shared LAN).
2. Within **90 seconds** both machines can `ping` each other's mesh IP.
3. `wgmesh peers list` shows the remote peer discovered via `dht` or `gossip`.
4. After rebooting both machines, the mesh re-forms within **120 seconds**.

#### Section 2: Core Feature Scope

Write a section titled `## Core Feature Scope` with two subsections:

**`### Must-Have (MVP block)`** — list each item as a checkbox `- [ ]`:

1. `wgmesh init --secret` prints a `wgmesh://v1/<base64>` URI (already implemented in
   `main.go:initCmd`; verify it works without error).
2. `wgmesh join --secret <URI>` starts the daemon, creates the WireGuard interface, derives
   a mesh IP, and begins discovery (already implemented; verify end-to-end).
3. LAN multicast discovery (Layer 1, `pkg/discovery/lan.go`) — peers on the same /24
   mesh within 10 seconds.
4. DHT discovery (Layer 2, `pkg/discovery/dht.go`) — peers across the internet within
   90 seconds.
5. Reconcile loop (`pkg/daemon/daemon.go`) applies `wg set` diffs every 5 seconds with
   no WireGuard interface restart.
6. WireGuard config persists across daemon restart (`pkg/wireguard/persist.go`) so the
   mesh re-forms on reboot.
7. `wgmesh peers list` (via RPC, `pkg/rpc/`) returns all active peers with pubkey, mesh
   IP, endpoint, last-seen, and discovered-via fields.
8. `wgmesh install-service` / `wgmesh uninstall-service` generates and installs a
   systemd unit that starts `wgmesh join` at boot.
9. `wgmesh status --secret <URI>` locally derives and prints mesh parameters (subnet,
   multicast group, DHT infohash, gossip port, rendezvous ID) **without starting the
   daemon** — this is a pure local computation from the secret; no running daemon is
   required.
10. Centralized mode baseline: `wgmesh -init`, `-add`, `-deploy` over SSH still work
    with at least two nodes.
11. **NAT traversal**: At least one node behind NAT can join a mesh containing one
    node with a public IP.

**`### Out-of-Scope for MVP`** — list each item as a bullet `-`:

- Secret rotation (`rotate-secret`) — protocol design incomplete (see `specs/STATUS.md`)
- Mesh IP collision resolution under high load (>100 nodes)
- `wgmesh service add` / Lighthouse CDN ingress — depends on external Lighthouse API
  availability; treat as post-MVP
- QR code generation (`qr` subcommand) — cosmetic; does not affect mesh correctness
- In-mesh gossip (Layer 3, `pkg/discovery/gossip.go`) — optional optimisation layer;
  DHT alone satisfies MVP convergence requirement
- GitHub registry (Layer 0, `pkg/discovery/registry.go`) — requires GitHub token and
  external API; optional bootstrap mechanism
- macOS / darwin support — `utun` interface handling; Linux-only for MVP
- `wgmesh peers` hostname display (Issue #81 partial spec) — cosmetic; pubkey display
  is sufficient for MVP

#### Section 3: WireGuard Integration Requirements

Write a section titled `## WireGuard Integration Requirements`:

List the following requirements, each as a checkbox `- [ ]`:

1. `wg` CLI tool (from the `wireguard-tools` package) must be present at
   `/usr/bin/wg` or on `$PATH`. The daemon detects its absence and exits with a clear
   error message.
2. A WireGuard kernel module (or `wireguard-go` userspace) must be loadable. The daemon
   checks `wg show wg0` at startup; if it fails with a kernel-module-missing error it
   prints actionable guidance.
3. Key generation uses `wg genkey` and `wg pubkey` CLI tools (from `wireguard-tools`).
   Keys are stored at `/var/lib/wgmesh/<interface>.json` with `0600` permissions.
   The daemon loads an existing keypair from this file on restart, so the WireGuard
   identity is stable across reboots.
4. Live updates use `wg set <iface> peer <pubkey> endpoint ... allowed-ips ... persistent-keepalive ...`
   so the interface is never torn down during reconciliation.
5. Persistent config for **centralized mode**: `wg-quick`-compatible `.conf` is written to
   `/etc/wireguard/<interface>.conf` (see `pkg/wireguard/persist.go`) and the
   `wg-quick@<iface>` systemd service is enabled so the interface comes up on reboot.
   For **decentralized mode**: persistence is via the keypair file
   (`/var/lib/wgmesh/<iface>.json`) plus the systemd unit generated by
   `wgmesh install-service`, which restarts `wgmesh join` at boot — no `wg-quick` is used.
6. IPv4-only mesh IPs are supported in MVP. IPv6 mesh IPs are out of scope.
7. `AllowedIPs` for each peer is set to `<mesh_ip>/32` plus any explicitly advertised
   routes (via `--advertise-routes`).
8. `PersistentKeepalive = 25` seconds is applied on all peers to maintain NAT mappings.

#### Section 4: Lighthouse CDN Control Plane Integration

Write a section titled `## Lighthouse CDN Control Plane Integration`:

State that Lighthouse integration is **post-MVP** but describe the integration contract
so it can be tracked as a separate epic:

1. `wgmesh service add <name> <local-addr> --secret <URI> --account <cr_...>` registers
   a service with the Lighthouse API via the `lighthouse-go` SDK.
2. The Lighthouse API assigns a public HTTPS domain `<name>.<meshID>.wgmesh.dev` and
   routes inbound HTTPS traffic to the node's mesh IP and local port.
3. The mesh ID used is `keys.MeshID()` — the first 12 hex characters of the derived
   `NetworkID` (see `pkg/crypto/derive.go:MeshID()`).
4. Integration exit criteria (post-MVP, not required for Foundation exit):
   - `wgmesh service add` succeeds and returns a live HTTPS URL.
   - Health checks pass when the local origin is reachable on the mesh.
   - `wgmesh service list` shows registered services with their URLs.
   - `wgmesh service remove` deregisters the service and the domain stops routing.

#### Section 5: Foundation Exit Criteria Checklist

Write a section titled `## Foundation Exit Criteria` with the following checklist items.
An item is checked only when a corresponding GitHub issue is **closed** and CI is green.

```markdown
### Connectivity
- [ ] Two nodes on a LAN mesh within 10 seconds (Scenario A passing in testlab)
- [ ] Two nodes across the internet mesh within 90 seconds (Scenario B passing in testlab)
- [ ] A node behind NAT can join a mesh with a public-IP node (NAT traversal working)
- [ ] Mesh re-forms after daemon restart without manual intervention

### Persistence
- [ ] WireGuard config survives daemon restart (keypair + peers persisted)
- [ ] `wgmesh install-service` + reboot: mesh re-forms within 120 seconds

### Observability
- [ ] `wgmesh peers list` returns all active peers with correct fields
- [ ] Daemon logs show discovery source and endpoint for each peer
- [ ] `wgmesh status` prints derived mesh parameters without side effects

### Centralized Mode Baseline
- [ ] `wgmesh -init / -add / -deploy` still works with two nodes via SSH
- [ ] Diff-based WireGuard updates apply without interface restart

### Release Readiness
- [ ] `wgmesh version` prints correct semver from `-ldflags`
- [ ] Pre-built Linux amd64 and arm64 binaries published on GitHub Releases
- [ ] Docker image builds and runs `wgmesh join` successfully
- [ ] README Quick Start commands execute without modification on a clean Ubuntu 24.04 VM
```

#### Section 6: Epic Issues to Create

Write a section titled `## Epic Issues to Create` that instructs engineers to open the
following GitHub issues (one issue per epic) against the `atvirokodosprendimai/wgmesh`
repository:

| Epic title | Scope summary |
|---|---|
| **[EPIC] Decentralized mode end-to-end validation** | Automated testlab test that runs Scenario A and Scenario B and asserts convergence times. Use existing `testlab/cloud/` infrastructure. |
| **[EPIC] Persistence and reboot survival** | Verify `pkg/wireguard/persist.go` writes `/etc/wireguard/wg0.conf`; verify systemd unit from `install-service` re-forms mesh after `systemctl reboot`. |
| **[EPIC] NAT traversal hardening** | STUN (`pkg/discovery/stun.go`) endpoint detection, NAT hole-punching in `pkg/daemon/daemon.go`, fallback relay when direct path fails. |
| **[EPIC] Centralized mode regression tests** | Table-driven integration tests for `wgmesh -init / -add / -remove / -deploy` against a mock SSH server (see `pkg/ssh/`). |
| **[EPIC] Release pipeline and binary distribution** | GoReleaser config (`.goreleaser.yml`) produces amd64/arm64 Linux + Docker; `make release` runs cleanly on a clean checkout. |
| **[EPIC] README Quick Start validation** | CI job that spins up a clean Ubuntu 24.04 container, runs every command in the README Quick Start section verbatim, and asserts zero non-zero exit codes. |
| **[EPIC] Lighthouse CDN integration (post-MVP)** | `wgmesh service add/list/remove` end-to-end against a staging Lighthouse API; HTTPS domain resolves and routes traffic. |

---

### Task 2: Update `specs/STATUS.md` to reference the new MVP doc

**File to edit**: `specs/STATUS.md`

At the **bottom** of the file, append the following section (after the last `---` separator):

```markdown
---

## MVP Scope and Foundation Exit Criteria

### Issue #453 — Define wgmesh MVP specification
- **File:** `docs/mvp.md`
- **Status:** SPEC WRITTEN — no implementation code
- **Summary:** Defines "working end-to-end" (two-node LAN + internet scenarios), core
  must-have feature list, WireGuard integration requirements, Lighthouse post-MVP scope,
  Foundation exit criteria checklist, and epics to open.
```

---

## Affected Files

| File | Action |
|---|---|
| `docs/mvp.md` | **CREATE** — new MVP specification document |
| `specs/STATUS.md` | **EDIT** — append Issue #453 entry at the bottom |

No package files (`*.go`) are modified.

---

## Test Strategy

This deliverable is documentation only. Verification steps:

1. `docs/mvp.md` exists and renders correctly on GitHub (headings, checkboxes, table).
2. All YAML front matter fields (`title`, `type`, `status`, `date`) are present.
3. `specs/STATUS.md` contains the new `## MVP Scope and Foundation Exit Criteria` section.
4. Run `go build ./...` — no build errors (no Go files changed).
5. Manually review each checklist item in `docs/mvp.md` against the current codebase:
   - Confirm that "Must-Have" items reference real, existing files/functions.
   - Confirm that "Out-of-Scope" items are genuinely not required for basic mesh formation.

---

## Estimated Complexity

low

### Rationale

- Documentation-only change: create one Markdown file, append to one existing file.
- No Go code changes; no build or test impact.
- All technical facts are derived directly from reading the existing source tree.
- Estimated time: 1–2 hours.
