# Plan — pull subsections for wgmesh codebase

**Date:** 2026-02-20
**Status:** pending

---

## Claim
Extract eidos specs for all major subsystems of wgmesh.

## Phases

### Phase 1 — Core subsystems (independent, can be done in any order)

- [x] **Action A** — `/eidos:pull pkg/mesh/ pkg/wireguard/ pkg/ssh/`
  Pull centralized mesh management: state file, SSH deployment, group/policy access control, diff-based WireGuard config, NAT detection.
  => [[spec - mesh state and access control - centralized node registry with policy-filtered peer configuration.md]]
  => [[spec - centralized SSH deploy - diff-based persistent WireGuard config pushed via SSH.md]]
  => [[pull - 2602211106 - mesh state access control and centralized SSH deploy.md]]

- [ ] **Action B** — `/eidos:pull pkg/daemon/` (multi-pass — expanded below)
  => Overview: [[pull - 2602211113 - daemon package overview.md]]
  => Sub-actions B.1–B.5 below replace this action.

- [ ] **Action B.1** — daemon core lifecycle
  Pull `Daemon` struct, `NewDaemon`, `Run`, `Shutdown`, `initLocalNode`, `setupWireGuard`, `teardownWireGuard`, signal handling, SIGHUP hot-reload.
  Files: `daemon.go` (lifecycle sections), `helpers.go` (interface ops), `config.go`

- [ ] **Action B.2** — reconciliation loop & relay routing
  Pull `reconcileLoop`, `reconcile`, `buildDesiredPeerConfigs`, `applyDesiredPeerConfigs`, `shouldRelayPeer`, `selectRelayForPeer`.
  Files: `daemon.go` (reconcile + relay sections)

- [ ] **Action B.3** — peer store
  Pull `PeerStore`, `PeerInfo`, endpoint ranking, subscription/notification, peer lifecycle timeouts.
  Files: `peerstore.go`

- [ ] **Action B.4** — health monitoring & mesh probing
  Pull `healthMonitorLoop`, `meshProbeLoop`, `checkPeerHealth`, TCP probe sessions, `temporaryOffline` eviction.
  Files: `daemon.go` (health + probe sections)

- [ ] **Action B.5** — support subsystems
  Pull peer cache persistence, IP collision resolution, epoch management, route sync, systemd integration.
  Files: `cache.go`, `collision.go`, `epoch.go`, `routes.go`, `systemd.go`

- [ ] **Action C** — `/eidos:pull pkg/discovery/`
  Pull discovery subsystem: DHT, LAN multicast, gossip, STUN, peer exchange protocol, rendezvous/hole-punching.

- [ ] **Action D** — `/eidos:pull pkg/crypto/`
  Pull crypto & identity: key derivation from shared secret, envelope encryption, membership proofs, secret rotation.

- [ ] **Action E** — `/eidos:pull pkg/privacy/`
  Pull privacy layer: Dandelion++ stem/fluff routing.

- [ ] **Action F** — `/eidos:pull pkg/rpc/`
  Pull RPC subsystem: unix socket JSON-RPC server/client, peer query API.

### Phase 2 — Auxiliary services

- [ ] **Action G** — `/eidos:pull pkg/lighthouse/ cmd/lighthouse/`
  Pull Lighthouse control plane: REST API, DNS management, XDS, health, rate limiting.

- [ ] **Action H** — `/eidos:pull cmd/chimney/`
  Pull Chimney dashboard server: GitHub API proxy, Dragonfly/in-memory cache, static serving.

### Phase 3 — Entry point (depends on B, C, D)

- [ ] **Action I** — `/eidos:pull main.go`
  Pull CLI entry point: dual-mode dispatch, subcommand structure, RPC server wiring.

---

## Notes
- Overview doc: `memory/pull - 2026-02-20-2343 - wgmesh codebase overview.md`
- Existing specs in `specs/` are issue-linked (older format) — check before writing to avoid duplication
- Infrastructure/CI (`.github/workflows/`, `deploy/`, `testlab/`) omitted — not product spec territory
