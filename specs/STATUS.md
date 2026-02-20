# Spec Audit Status

**Audited:** 2026-02-16

## Directory Structure

```
specs/
  implemented/     # Fully implemented, all GitHub issues CLOSED
  partial/         # Code exists but spec not fully satisfied
  not-implemented/ # No implementation exists
```

```
features/
  archived/        # Design docs for the bootstrap/decentralized mode (all layers implemented)
```

---

## Implemented (3 specs)

### Issue #24 - macOS Binary Builds
- **File:** `specs/implemented/issue-24-spec.md`
- **GitHub:** Issue #24 (CLOSED)
- **Evidence:** `.github/workflows/binary-build.yml` includes `darwin/amd64` and `darwin/arm64` in build matrix (lines 31-36), release job includes both (lines 97-98), Homebrew formula has `on_macos` blocks (lines 136-145)

### Issue #43 - `--version` / `-v` Flags
- **File:** `specs/implemented/issue-43-spec.md`
- **GitHub:** Issue #43 (CLOSED)
- **Evidence:** `main.go:23-29` has early arg loop checking `--version`/`-v` before subcommand switch. `main_test.go` has 4 test functions covering all three forms, priority behavior, exit codes, and format consistency.

### Issue #76 - Default State File Path
- **File:** `specs/implemented/issue-76-spec.md`
- **GitHub:** Issue #76 (CLOSED)
- **Evidence:** `main.go:72` sets default to `/var/lib/wgmesh/mesh-state.json`. `pkg/mesh/mesh.go:79` has `os.MkdirAll(dir, 0700)` in `Save()` for directory auto-creation.

---

## Partially Implemented (2 specs)

### Issue #4 - Gossip vs DHT Discovery Modes
- **File:** `specs/partial/issue-4-spec.md`
- **GitHub:** Issue #4 (CLOSED)
- **What's done:** CLI `--gossip` flag exists (`main.go:244`), GOSSIP_TESTING.md exists
- **What's missing:** README.md has no section explaining discovery layers. GOSSIP_TESTING.md doesn't explicitly clarify that gossip *supplements* (not replaces) DHT. Help text for the flag is minimal.
- **Remaining work:** Documentation-only (~30 min)

### Issue #81 - Node Listing with VPN IPs (`list-peers`)
- **File:** `specs/partial/issue-81-spec.md`
- **GitHub:** Issue #81 (CLOSED)
- **What's done:**
  - `list-peers` subcommand (`main.go:64-66`, implementation at `main.go:598-681`)
  - Dual mode support (`--state` for centralized, `--secret` for decentralized)
  - `Mesh.ListSimple()` in `pkg/mesh/mesh.go:188-200`
  - `Hostname` field in `PeerInfo` struct (`pkg/daemon/peerstore.go:19`)
- **What's missing:**
  - `Hostname` field NOT in `PeerAnnouncement` (`pkg/crypto/envelope.go:23-31`) -- hostnames can't be transmitted over the wire
  - Discovery handlers (gossip, LAN, DHT) don't extract/pass hostname
  - Decentralized mode peers will show truncated pubkeys instead of hostnames
- **Remaining work:** Protocol extension (~3-4 hours, Phase 3 of spec)

---

## Implemented Since Last Audit (1 spec)

### RPC Socket Interface for Querying Running Daemon
- **File:** `specs/not-implemented/rpc-socket-interface-spec.md`
- **GitHub:** No associated issue found
- **Status:** **IMPLEMENTED** — `pkg/rpc/` exists with full Unix socket JSON-RPC server (422 lines), client, protocol, and integration tests
- **Evidence:** `pkg/rpc/server.go` (Unix socket listener, methods: `peers.list`, `peers.get`, `peers.count`, `daemon.status`, `daemon.ping`), `pkg/rpc/client.go`, `pkg/rpc/protocol.go`, `pkg/rpc/server_test.go`, `pkg/rpc/protocol_test.go`, `pkg/rpc/integration_test.go`
- **Note:** Spec file should be moved to `specs/implemented/`

---

## Feature Docs (Archived)

### Bootstrap / Token-Based Mesh Autodiscovery
- **Files:** `features/archived/bootstrap.md`, `features/archived/IMPLEMENTATION_PLAN.md`
- **Status:** Core implementation complete. All discovery layer files exist:
  - `pkg/discovery/registry.go` (Layer 0: GitHub Issue rendezvous)
  - `pkg/discovery/lan.go` (Layer 1: LAN multicast)
  - `pkg/discovery/dht.go` (Layer 2: BitTorrent DHT)
  - `pkg/discovery/gossip.go` (Layer 3: In-mesh gossip)
  - `pkg/privacy/dandelion.go` (Dandelion++ privacy)
  - `pkg/crypto/membership.go` (Membership tokens)
  - `pkg/daemon/epoch.go` (Epoch management)
- **Not implemented (intentionally deferred per plan):**
  - Private floodfill mode (deemed overkill)
  - Garlic bundling (deemed overkill)
  - Unidirectional tunnels (deemed overkill)
  - Secret rotation protocol
  - QR code generation
