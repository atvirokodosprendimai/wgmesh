# Issue #540: Key Rotation Changes Node IP Address

## Summary

When a mesh secret is rotated using `wgmesh rotate-secret`, nodes that restart during or after the rotation derive a different mesh IP address. This occurs because mesh IP derivation (both IPv4 and IPv6) uses the secret as a key derivation input. The current codebase only persists IPv4 addresses across restarts to prevent address changes during secret rotation, but IPv6 addresses are re-derived on every daemon start.

This bug causes:
- Loss of mesh connectivity when nodes restart after secret rotation
- Peer configuration inconsistencies across the mesh
- Potential routing table corruption due to changing peer IP mappings

## Context

### Root Cause Analysis

**IPv4 Addresses (Partially Fixed):**
The daemon code in `pkg/daemon/daemon.go:initLocalNode()` already contains logic to persist IPv4 addresses across restarts:

```go
// Use the persisted mesh IP when it is present and falls within the
// expected subnet. Re-derive only when the field is absent (old state
// file) or the configured subnet has changed.
if node.MeshIP != "" && meshIPInSubnet(node.MeshIP, d.config) {
    // Persisted IP is valid — keep it so that secret rotation does not
    // change the node's address.
} else {
    // Re-derive IP...
    if err := saveLocalNode(stateFile, d.localNode); err != nil {
        log.Printf("Warning: failed to save local node state: %v", err)
    }
}
```

However, this logic only applies when loading from an existing state file. If a node's state file is corrupted, missing, or the node is being joined for the first time after rotation, it will derive a new IP based on the new secret.

**IPv6 Addresses (Not Fixed):**
IPv6 addresses lack this persistence mechanism entirely. The code explicitly re-derives IPv6 addresses on every start:

```go
// Always re-derive IPv6; IPv6 addresses are not globally routable and
// subnet pinning is not applicable for the /64 ULA prefix.
if node.MeshIPv6 == "" {
    d.localNode.MeshIPv6 = crypto.DeriveMeshIPv6(d.config.Keys.MeshPrefixV6, d.localNode.WGPubKey, d.config.Secret)
    if err := saveLocalNode(stateFile, d.localNode); err != nil {
        log.Printf("Warning: failed to save local node state: %v", err)
    }
}
```

The comment "IPv6 addresses are not globally routable" refers to the ULA prefix (fc00::/7), but this does not justify changing the address during secret rotation. IPv6 addresses are used for:
- Health probes and monitoring (`pkg/daemon/daemon.go:936-937`)
- Peer health checks (`pkg/daemon/daemon.go:1122-1123, 1312-1313`)
- WireGuard AllowedIPs configuration (`pkg/daemon/daemon.go:589-604`)

**Derivation Functions:**
All IP derivation functions use `secret` as an input:
- `DeriveMeshIP(meshSubnet, wgPubKey, secret)` - Legacy IPv4 derivation
- `DeriveMeshIPv6(meshPrefixV6, wgPubKey, secret)` - IPv6 ULA derivation
- `DeriveMeshIPInSubnet(subnet, wgPubKey, secret)` - Custom subnet IPv4 derivation

When `secret` changes, all these functions return different results for the same `wgPubKey`.

**Secret Rotation Flow:**
The `wgmesh rotate-secret` command (`main.go:rotateSecretCmd()`) creates a rotation announcement but does not update any local daemon configuration. The operator must manually:
1. Run `wgmesh rotate-secret` to generate the new secret
2. Update the daemon service/systemd unit with the new secret
3. Restart each node

During step 3, nodes derive new addresses based on the new secret, causing mesh disruption.

**Centralized Mode (service.go):**
The `wgmesh service` commands also use `DeriveMeshIPInSubnet()` with the current secret, but there is no persistence mechanism for service-mode deployments. Each service invocation re-derives IPs from the provided secret.

### Affected Components

- `pkg/daemon/daemon.go:initLocalNode()` - IPv4 persistence (partial fix), IPv6 missing persistence
- `pkg/daemon/helpers.go:loadLocalNode()` / `saveLocalNode()` - State persistence functions
- `pkg/crypto/derive.go:DeriveMeshIPv6()` - IPv6 derivation function
- `service.go:deriveMeshIPForService()` - Centralized mode derivation (no persistence)

## Requirements

### R1: IP Address Persistence During Secret Rotation
Mesh IP addresses (both IPv4 and IPv6) MUST remain stable across secret rotation operations. A node's mesh IP address should only change when:
- The node's WireGuard keypair changes (rekey)
- The operator explicitly changes the mesh subnet configuration

### R2: Graceful Dual-Secret Operation
During the grace period after secret rotation (default 24 hours), nodes MUST:
- Accept peers authenticated with either the old or new secret
- Maintain stable IP addresses for all peers regardless of which secret authenticated them
- Store both old and new secrets locally to handle late-arriving peers

### R3: IPv6 Address Persistence
IPv6 mesh addresses MUST be persisted across daemon restarts, similar to IPv4 addresses. The current behavior of re-deriving IPv6 addresses on every start is a bug.

### R4: State File Migration
The solution MUST handle migration scenarios:
- Nodes with existing state files (v1.0.x format)
- Nodes with missing or corrupted state files
- Nodes joining during active rotation (grace period in progress)

### R5: Centralized Mode Compatibility
The solution should account for `wgmesh service` commands that may not benefit from daemon state persistence. Service-mode deployments may need an alternative mechanism or documentation.

## Acceptance Criteria

### AC1: IPv4 Persistence Works During Rotation
- Given a node with persisted IPv4 address `10.42.7.33` derived from old secret
- When the operator rotates the secret and restarts the node
- Then the node MUST retain `10.42.7.33` as its mesh IPv4 address
- AND the state file MUST contain the unchanged IPv4 address

### AC2: IPv6 Persistence Works During Rotation
- Given a node with persisted IPv6 address `fc00::42:7:33` derived from old secret
- When the operator rotates the secret and restarts the node
- Then the node MUST retain `fc00::42:7:33` as its mesh IPv6 address
- AND the state file MUST contain the unchanged IPv6 address

### AC3: First Boot After Rotation
- Given a new node joining the mesh after secret rotation
- When the node starts with the new secret
- Then the node derives its mesh IP from the new secret
- AND all existing nodes (still on old secret during grace period) can connect to it
- AND the new node can connect to all existing nodes

### AC4: Grace Period Acceptance
- Given a node in dual-secret mode (grace period active)
- When a peer authenticates with the old secret
- Then the node MUST accept the peer
- AND MUST use the peer's persisted IP address (not re-derive)

### AC5: Subnet Change Re-derivation
- Given a node with persisted IPv4 address `10.42.7.33` in subnet `10.42.0.0/16`
- When the operator changes `--mesh-subnet` to `10.100.0.0/16` and restarts
- Then the node MUST re-derive its IPv4 address in the new subnet
- AND the new persisted address MUST be saved to the state file

### AC6: Corrupted State Recovery
- Given a node with corrupted or missing state file
- When the node starts after secret rotation
- Then the node MUST derive its mesh IP from the current secret
- AND MUST save the new IP to the state file

### AC7: WireGuard Configuration Stability
- Given a running mesh with N nodes
- When the secret is rotated and all nodes restart
- Then all WireGuard peer configurations MUST use the same IP addresses as before rotation
- AND no AllowedIPs entries should change for existing peers

## Out of Scope

- Changing the IP derivation algorithms (cryptographic stability is required)
- Implementing full gossip-based rotation announcement distribution (exists in design but not implemented)
- Modifying WireGuard key rotation logic (separate concern)
- STUN/endpoint discovery changes (unrelated to IP derivation)
- Hourly epoch rotation for privacy (uses different epoch seed, not mesh secret)
- DHT network ID rotation (uses `GetCurrentAndPreviousNetworkIDs()`, already handles rotation)

## Implementation Notes

### Recommended Approach

1. **Add IPv6 persistence to `initLocalNode()`**: Apply the same persistence logic to IPv6 that currently exists for IPv4.

2. **Add rotation state to Config**: Extend `pkg/daemon/config.go:Config` to track the old secret during grace period:
   ```go
   type Config struct {
       Secret        string
       OldSecret     string        // Set during grace period
       RotationState *crypto.RotationState
       // ... existing fields
   }
   ```

3. **Load rotation state from file**: Create `pkg/daemon/rotation.go` to persist and load rotation state, allowing nodes to remember they're in grace period across restarts.

4. **Derive dual keys during grace period**: In `initLocalNode()`, when `OldSecret` is set:
   - Derive `OldDerivedKeys` from `OldSecret`
   - Accept peers authenticated with either `MembershipKey` or `OldDerivedKeys.MembershipKey`

5. **Document service-mode limitation**: Centralized mode (`wgmesh service`) lacks daemon state persistence. Document that service users should:
   - Use stable secrets (avoid rotation)
   - Or manually pin mesh IPs via `--mesh-ip` flag (requires adding this flag)

### Test Strategy

- Unit tests for `meshIPInSubnet()` with IPv6 addresses
- Table-driven tests for `initLocalNode()` covering:
  - Existing state file with valid IPv4/IPv6
  - Existing state file with invalid subnet
  - Missing state file
  - Corrupted state file
  - Grace period active (dual secret)
- Integration test for full rotation workflow:
  1. Start 2-node mesh with old secret
  2. Rotate secret, restart node A
  3. Verify A retains original IPs
  4. Restart node B
  5. Verify B retains original IPs
  6. Verify A and B reconnect successfully

### Files to Modify

- `pkg/daemon/daemon.go` - Update `initLocalNode()` to persist IPv6
- `pkg/daemon/config.go` - Add `OldSecret` and `RotationState` fields
- `pkg/daemon/rotation.go` - New file for rotation state persistence
- `pkg/daemon/helpers.go` - Extend `localNodeState` to include rotation metadata
- `pkg/crypto/derive.go` - Consider adding `DeriveMeshIPv6InSubnet()` for completeness (optional)
- `main.go` - Update `rotateSecretCmd()` to write rotation state file
- Test files: `pkg/daemon/daemon_test.go`, `pkg/daemon/helpers_test.go`, `pkg/crypto/derive_test.go`

### Complexity Estimate

- **Low to Medium**: The IPv4 persistence logic already exists and works correctly. Extending it to IPv6 is straightforward. Adding dual-secret support requires more careful implementation but is well-scoped.
- **Estimated effort**: 6-10 hours implementation + 4-6 hours testing + 2 hours documentation
- **Risk level**: Low - Changes are additive and isolated to daemon initialization
