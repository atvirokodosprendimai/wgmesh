# Issue #540: Key rotation changes node IP address

## Classification
bug

## Problem Analysis

### Root Cause
When a mesh secret is rotated using `wgmesh rotate-secret`, nodes that generate a new WireGuard keypair receive a different mesh IP address. This occurs because mesh IP derivation depends on both the secret and the WireGuard public key:

```go
// From pkg/crypto/derive.go:194-206
func DeriveMeshIP(meshSubnet [2]byte, wgPubKey, secret string) string {
    input := wgPubKey + secret
    hash := sha256.Sum256([]byte(input))
    // ...
}
```

When a node generates a new keypair after secret rotation, the combination `(newPublicKey, newSecret)` produces a different hash input, resulting in a different mesh IP. While the code attempts to preserve IPs via state persistence (`pkg/daemon/daemon.go:339-340`), this mitigation fails in two scenarios:

1. **New node joins during rotation**: Node has no prior state, generates fresh keypair → new IP
2. **State file corruption/loss**: Node loses state file, regenerates keypair → new IP  
3. **Rekey during rotation**: Manual or automated rekey generates new keypair → new IP
4. **Subnet change**: Any node in a custom subnet re-derives IP after key change

### Impact
- **Connectivity disruption**: Peers routing to the old IP cannot reach the node
- **Routing table pollution**: Stale peer entries with old IPs linger until timeout
- **Split-brain scenarios**: Node believes it has new IP, peers still reference old IP
- **Management complexity**: Operators must manually track IP changes across nodes

### Current Mitigation Attempts
The code contains logic to preserve IPs across secret rotation (`pkg/daemon/daemon.go:331-366`):

```go
// Try to load existing key from state file
node, err := loadLocalNode(stateFile)
if err == nil && node != nil {
    // Use persisted mesh IP when present and in expected subnet
    if node.MeshIP != "" && meshIPInSubnet(node.MeshIP, d.config) {
        // Persisted IP is valid — keep it so that secret rotation does not
        // change the node's address.
    }
}
```

However, this only works when:
- State file exists and is readable
- Node's keypair has NOT been regenerated
- Subnet configuration hasn't changed

### Why This Happens
The fundamental issue is **key regeneration**: when `initLocalNode()` calls `wireguard.GenerateKeyPair()`, a new `(privateKey, publicKey)` pair is created. Since the public key is an input to IP derivation, any node generating a new key after secret rotation will compute a new IP address.

The state preservation logic assumes the keypair is stable across secret rotation, but the code path for key generation does NOT check if a keypair for the current node identity already exists.

## Proposed Approach

### 1. IP Stabilization via Node Identity Salt

Add a node-unique salt value that persists across key rotations. Modify `DeriveMeshIP` to incorporate this salt, ensuring IP stability regardless of keypair changes:

```go
// In pkg/crypto/derive.go
func DeriveMeshIPWithSalt(meshSubnet [2]byte, wgPubKey, secret, nodeSalt string) string {
    input := wgPubKey + secret + nodeSalt  // Include salt in derivation
    hash := sha256.Sum256([]byte(input))
    // ... rest of derivation logic
}
```

The `nodeSalt` is a UUID or high-entropy random string generated once per node and persisted in the state file.

### 2. State File Enhancement

Add `NodeSalt` to `localNodeState`:

```go
// In pkg/daemon/helpers.go
type localNodeState struct {
    WGPubKey     string `json:"wg_pubkey"`
    WGPrivateKey string `json:"wg_private_key"`
    MeshIP       string `json:"mesh_ip,omitempty"`
    MeshIPv6     string `json:"mesh_ipv6,omitempty"`
    NodeSalt     string `json:"node_salt,omitempty"`  // NEW: IP stability salt
}
```

### 3. Key Generation Logic Update

Modify `initLocalNode()` in `pkg/daemon/daemon.go`:

- **When state file exists**: Reuse existing keypair and salt
- **When state file missing**: Generate new keypair, generate new salt, derive initial IP
- **When keypair rotation requested**: Generate new keypair only, preserve salt and IP

### 4. Grace Period Key Compatibility

During secret rotation grace period, nodes must handle peer announcements with both old and new secrets. The salt ensures IP remains stable regardless of which secret is used for derivation:

```
Node IP = DeriveMeshIP(subnet, pubkey, newSecret, nodeSalt)
          = DeriveMeshIP(subnet, pubkey, oldSecret, nodeSalt)  // Same output
```

This is achieved by making the derivation **secret-agnostic** when salt is present. The salt provides sufficient entropy to prevent collisions while maintaining stability.

### 5. Backward Compatibility

For existing state files without `NodeSalt`:

- Add migration logic in `loadLocalNode()` to generate salt on-the-fly
- Use a deterministic fallback: `NodeSalt = SHA256(wgPubKey)[:16]` for legacy nodes
- Mark for persistence on next save

### 6. Collision Handling Update

Update `DeriveMeshIPWithCollisionCheck()` in `pkg/daemon/collision.go` to accept salt parameter:

```go
func DeriveMeshIPWithCollisionCheck(
    meshSubnet [2]byte, 
    wgPubKey, secret, nodeSalt string,  // Add nodeSalt
    existingIPs map[string]string, 
    customSubnet *net.IPNet,
) string
```

### 7. WireGuard Config Update

When keypair changes, only the `PrivateKey` field in WireGuard config needs updating. The IP address (`Address` field) remains unchanged.

## Acceptance Criteria

1. **IP Stability**: A node that performs key rotation (manual or automated) retains its mesh IP address
2. **Secret Rotation**: A node that rotates from `oldSecret` to `newSecret` retains its mesh IP address
3. **New Nodes**: New nodes joining during active rotation derive stable IPs immediately
4. **Collision Resolution**: IP collision detection/resolution continues to work with salt-based derivation
5. **Backward Compatibility**: Existing nodes without salt in state file upgrade gracefully
6. **State Persistence**: `NodeSalt` is persisted correctly and survives daemon restarts
7. **Test Coverage**: Unit tests for salt-based derivation, integration test for full rotation workflow

## Out of scope

- IPv6 address derivation changes (separate issue, if needed)
- Subnet migration mechanisms (assumes static subnet configuration)
- DNS integration for node discovery (out of scope for this bug fix)
- Rendezvous/Hooks API changes (unrelated to IP derivation)
- Performance optimization of the derivation function (current SHA256-based approach is sufficient)
- UI/CLI changes beyond `rotate-secret` command (no new CLI flags required)
