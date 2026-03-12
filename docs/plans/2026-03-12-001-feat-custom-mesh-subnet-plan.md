---
title: "feat: Custom mesh subnet support"
type: feat
status: completed
date: 2026-03-12
---

# feat: Custom mesh subnet support

## Overview

Allow users to specify a custom mesh subnet (any valid CIDR, e.g. `192.168.100.0/24`, `172.16.0.0/12`) instead of the hardcoded `10.X.Y.0/16` derivation.
Applies to both decentralized (daemon) and centralized (SSH deploy) modes.

## Problem Statement

Currently:
- **Decentralized mode**: mesh subnet is deterministically derived from the shared secret via HKDF → always `10.X.Y.0/16`. No user control.
- **Centralized mode**: default network is hardcoded to `10.99.0.0/16` in `pkg/mesh/mesh.go:29`. Users can manually assign IPs, but the CIDR frame is fixed.
- Users who need specific ranges (e.g. `192.168.100.0/24` to avoid conflicts with existing infrastructure) cannot configure this.

## Proposed Solution

Add a `--mesh-subnet` flag (decentralized) and `--network` flag (centralized init) that accepts any valid CIDR.
IP derivation adapts to fit within the specified subnet's host space.

### Key Design Decisions

1. **Derived subnet becomes the default, not the only option** — if `--mesh-subnet` is not specified, behavior is unchanged (backward compatible).
2. **IP derivation generalizes to any CIDR** — hash mod host-space-size, mapped into the subnet.
3. **Subnet stored in daemon state** — persisted to `/var/lib/wgmesh/<iface>.json` so restarts use the same subnet.
4. **All peers must use the same subnet** — enforced by including it in gossip announcements; mismatch = warning log.

## Technical Approach

### Phase 1: Core — Generalized IP derivation (`pkg/crypto`)

**Files to modify:**
- `pkg/crypto/derive.go` — new `DeriveMeshIPInSubnet(subnet net.IPNet, wgPubKey, secret string) string`

**Logic:**
```go
func DeriveMeshIPInSubnet(subnet net.IPNet, wgPubKey, secret string) (string, error) {
    ones, bits := subnet.Mask.Size()
    hostBits := bits - ones
    if hostBits < 2 {
        return "", fmt.Errorf("subnet /%d too small (need at least /30)", ones)
    }
    maxHosts := (1 << hostBits) - 2 // exclude network and broadcast

    hash := sha256.Sum256([]byte(wgPubKey + secret))
    hostNum := binary.BigEndian.Uint32(hash[:4]) % uint32(maxHosts)
    hostNum += 1 // skip network address (.0)

    ip := make(net.IP, len(subnet.IP))
    copy(ip, subnet.IP)
    // Add hostNum to the network address
    for i := len(ip) - 1; i >= 0 && hostNum > 0; i-- {
        sum := uint32(ip[i]) + (hostNum & 0xFF)
        ip[i] = byte(sum & 0xFF)
        hostNum = (hostNum >> 8) + (sum >> 8)
    }
    return ip.String(), nil
}
```

**Collision resolution** (`pkg/daemon/collision.go`):
- `DeriveMeshIPWithNonce` must also use the generalized function.
- Replace hardcoded `10.%d.%d.%d` format with subnet-aware derivation.

**Tests:**
- Derive IP in `/24` → result within `x.x.x.1` – `x.x.x.254`
- Derive IP in `/16` → result within range
- Derive IP in `/28` → result within 14-host range
- Subnet too small (`/31`, `/32`) → error
- Determinism: same inputs → same output
- IPv6 subnet support (optional, Phase 3)

### Phase 2: Decentralized mode — CLI flag + config propagation

**Files to modify:**
- `main.go` — add `--mesh-subnet` flag to `join` and `status` subcommands
- `pkg/daemon/config.go` — add `MeshSubnet net.IPNet` field to `Config` struct (rename existing `Keys.MeshSubnet` usage)
- `pkg/daemon/daemon.go:313,328` — use new `DeriveMeshIPInSubnet` instead of `DeriveMeshIP`
- `pkg/daemon/collision.go:78,99,109` — pass subnet from config
- `service.go:454,464` — use new derivation
- `main.go:517` — update status display

**Config persistence:**
- Add `mesh_subnet` field to `/var/lib/wgmesh/<iface>.json` state file
- On join: if `--mesh-subnet` provided, store it; otherwise derive as before

**Backward compatibility:**
- No `--mesh-subnet` flag → derive `MeshSubnet` from secret as today → `10.X.Y.0/16`
- With `--mesh-subnet 192.168.100.0/24` → use specified CIDR
- Interface address mask matches the specified CIDR prefix length (currently hardcoded `/16` at `collision.go:104`)

### Phase 3: Centralized mode — init flag + state file

**Files to modify:**
- `pkg/mesh/mesh.go:29` — use user-provided network or default `10.99.0.0/16`
- `main.go` (centralized flags) — add `--network` flag to `-init`
- `pkg/mesh/deploy.go:253` — use prefix length from state instead of hardcoded `/16`

**State file change:**
```json
{
  "interface_name": "wg0",
  "network": "192.168.100.0/24",
  ...
}
```
Already stored as string in `Mesh.Network` — just needs validation on init.

### Phase 4: Subnet agreement enforcement (decentralized)

**Optional but recommended:**
- Include mesh subnet CIDR in gossip peer announcements
- On receiving a peer with mismatched subnet → log warning (don't reject — soft enforcement)
- `wgmesh status` shows configured subnet

## Acceptance Criteria

- [x] `wgmesh join --secret <S> --mesh-subnet 192.168.100.0/24` assigns IPs within `192.168.100.1–254`
- [x] `wgmesh join --secret <S>` (no flag) works exactly as before — no regression
- [x] `wgmesh -init -state mesh.json --network 172.16.5.0/24` creates state with custom network
- [x] Collision resolution works within custom subnets
- [x] Interface address uses correct prefix length (not hardcoded `/16`)
- [x] `wgmesh status` displays the configured subnet
- [x] All existing tests pass
- [x] New unit tests for `DeriveMeshIPInSubnet` cover `/24`, `/16`, `/28`, edge cases
- [ ] Subnet persisted across daemon restarts (deferred — requires state file schema change)

## Dependencies & Risks

**Risks:**
- **All peers must agree on subnet** — if one peer joins with `--mesh-subnet 192.168.100.0/24` and another without, they'll derive different IPs → can't communicate. Mitigation: gossip-based subnet advertisement + clear docs.
- **Small subnets increase collision probability** — a `/28` has only 14 hosts. Mitigation: collision resolution already exists, just needs to work within the subnet.
- **Breaking change for existing meshes** — if users upgrade and add `--mesh-subnet`, they get new IPs. Mitigation: document clearly, make it opt-in only.

**Dependencies:**
- None — self-contained feature, no external deps needed.

## Files Reference

| File | Role | Change |
|------|------|--------|
| `pkg/crypto/derive.go:155-175` | IP derivation | Add `DeriveMeshIPInSubnet`, keep `DeriveMeshIP` as wrapper |
| `pkg/crypto/derive_test.go` | Tests | Add subnet-aware derivation tests |
| `pkg/daemon/config.go:24-38` | Daemon config | Add `CustomSubnet *net.IPNet` field |
| `pkg/daemon/daemon.go:313,328` | IP assignment | Use subnet-aware derivation |
| `pkg/daemon/collision.go:66-83,99,109` | Collision resolution | Generalize to work with any subnet |
| `pkg/mesh/mesh.go:21-36` | Centralized init | Accept custom network CIDR |
| `pkg/mesh/deploy.go:253` | WG config | Use prefix length from state |
| `main.go` | CLI flags | Add `--mesh-subnet` and `--network` flags |
| `service.go:454,464` | Service registration | Use subnet-aware derivation |
