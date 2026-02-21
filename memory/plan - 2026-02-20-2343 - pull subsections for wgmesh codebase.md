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

- [x] **Action B** — `/eidos:pull pkg/daemon/` (multi-pass — expanded below)
  => Overview: [[pull - 2602211113 - daemon package overview.md]]
  => Sub-actions B.1–B.5 below replace this action.

- [x] **Action B.1** — daemon core lifecycle
  => [[spec - daemon lifecycle - secret-derived identity with interface setup and hot-reload.md]]

- [x] **Action B.2** — reconciliation loop & relay routing
  => [[spec - daemon reconciliation - peer config sync with relay routing fallback.md]]

- [x] **Action B.3** — peer store
  => [[spec - peer store - thread-safe registry with endpoint ranking and pub-sub.md]]

- [x] **Action B.4** — health monitoring & mesh probing
  => [[spec - daemon health - dual signal health monitoring with eviction and temporary offline.md]]

- [x] **Action B.5** — support subsystems
  => [[spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md]]

- [x] **Action C** — `/eidos:pull pkg/discovery/` (multi-pass — expanded below)
  Pull discovery subsystem: DHT, LAN multicast, gossip, STUN, peer exchange protocol, rendezvous/hole-punching.
  => Overview: [[pull - 2602211143 - discovery package overview.md]]
  => Sub-actions C.1–C.6 below replace this action.

- [x] **Action C.1** — LAN discovery + STUN/NAT detection (`lan.go`, `stun.go`)
  => [[spec - discovery lan and stun - multicast peer discovery and nat type detection.md]]

- [x] **Action C.2** — In-mesh gossip (`gossip.go`)
  => [[spec - discovery gossip - in-mesh udp peer broadcast.md]]

- [ ] **Action C.3** — Peer exchange protocol — peer advertisement (`exchange.go` hello/reply)

- [ ] **Action C.4** — Peer exchange rendezvous + hole-punching (`exchange.go` rendezvous)

- [ ] **Action C.5** — DHT announce, query, persistence, IPv6 sync (`dht.go` core)

- [ ] **Action C.6** — DHT rendezvous + hole-punching + GitHub registry (`dht.go` rendezvous + `registry.go`)

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
