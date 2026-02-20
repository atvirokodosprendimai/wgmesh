# Plan — pull subsections for wgmesh codebase

**Date:** 2026-02-20
**Status:** pending

---

## Claim
Extract eidos specs for all major subsystems of wgmesh.

## Phases

### Phase 1 — Core subsystems (independent, can be done in any order)

- [ ] **Action A** — `/eidos:pull pkg/mesh/ pkg/wireguard/ pkg/ssh/`
  Pull centralized mesh management: state file, SSH deployment, group/policy access control, diff-based WireGuard config, NAT detection.

- [ ] **Action B** — `/eidos:pull pkg/daemon/`
  Pull daemon core: reconciliation loop, peer store, health probing, relay routing, epoch/membership, IP collision resolution, WireGuard interface lifecycle.

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
