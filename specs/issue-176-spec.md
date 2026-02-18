# Specification: Issue #176

## Classification
feature

## Deliverables
code + documentation

## Problem Analysis

Currently, wgmesh operates as a "flat network" in centralized mode where all nodes can reach all other nodes through the WireGuard mesh. Every node is configured as a peer to every other node, and the `AllowedIPs` configuration permits traffic from all mesh IPs and routable networks.

This creates a security and operational challenge:
- No ability to segment the network into groups (e.g., production vs staging)
- No way to restrict which nodes can communicate with which resources
- All nodes have full mesh connectivity regardless of their purpose or trust level
- Networks behind nodes (via `routable_networks`) are accessible to all mesh members

### Current Architecture (Centralized Mode)

From `pkg/mesh/deploy.go`, the `generateConfigForNode()` function creates a full mesh:
- Each node gets **all other nodes** as WireGuard peers
- `AllowedIPs` includes the mesh IP (`/32`) of every peer plus all their `routable_networks`
- There's no filtering mechanism to limit which peers a node should connect to

Example: If we have 5 nodes (A, B, C, D, E), every node gets 4 peers configured with full access to all networks.

### Business Use Cases

Organizations need network segmentation for:
1. **Environment isolation**: Dev/staging/prod nodes shouldn't all interconnect
2. **Security boundaries**: Database nodes shouldn't be reachable from all nodes
3. **Compliance**: PCI/HIPAA networks require restricted access
4. **Multi-tenant**: Different customers/projects on same mesh infrastructure
5. **Least privilege**: Nodes should only access resources they need

## Proposed Approach

Implement a **group-based access control** system where nodes can be assigned to one or more groups, and access policies define which groups can communicate with which other groups.

### Design Principles

1. **Backward compatible**: Existing meshes without groups continue to work as full mesh
2. **Declarative**: Groups and policies defined in mesh state file
3. **WireGuard native**: Uses AllowedIPs filtering (no external firewall needed)
4. **Simple first**: Start with basic group membership and access rules
5. **Centralized control**: Operator defines policies, deployment enforces them

### Data Model

Extend the mesh state file (`/var/lib/wgmesh/mesh-state.json`) with:

```json
{
  "interface_name": "wg0",
  "network": "10.99.0.0/16",
  "listen_port": 51820,
  "local_hostname": "control-node",
  "groups": {
    "production": {
      "description": "Production environment nodes",
      "members": ["node1", "node2"]
    },
    "staging": {
      "description": "Staging environment",
      "members": ["node3", "node4"]
    },
    "database": {
      "description": "Database servers",
      "members": ["node5"]
    }
  },
  "access_policies": [
    {
      "name": "prod-to-db",
      "description": "Allow production nodes to access database",
      "from_groups": ["production"],
      "to_groups": ["database"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    },
    {
      "name": "staging-isolated",
      "description": "Staging can only talk within staging",
      "from_groups": ["staging"],
      "to_groups": ["staging"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    }
  ],
  "nodes": {
    "node1": {
      "hostname": "node1",
      "mesh_ip": "10.99.0.1",
      "routable_networks": ["192.168.10.0/24"],
      ...
    },
    ...
  }
}
```

### Policy Evaluation Algorithm

For each node, when generating WireGuard configuration:

1. **Find node's groups**: Collect all groups where this node is a member.
2. **Determine relevant policies**:
   - **Outbound policies**: Policies where any of the node's groups appear in `from_groups`.
   - **Inbound policies**: Policies where any of the node's groups appear in `to_groups`.
3. **Build symmetric peer list**:
   - For every policy (outbound or inbound) that links the node's groups to some other groups, add all members of those other groups as WireGuard peers on this node.
   - This ensures that whenever any policy relates two nodes' groups (in either direction), both nodes have each other configured as peers, satisfying WireGuard's requirement that both sides know each other's public keys.
4. **Configure AllowedIPs (directional access)** for each peer:
   - **Baseline for handshakes**: If any policy exists between the node's groups and the peer's groups (in either direction), include the peer's mesh IP (`/32`) in `AllowedIPs` so that WireGuard handshakes and basic packet acceptance work.
   - **Outbound mesh access**: If there is at least one outbound policy where the node's groups are in `from_groups` and the peer's groups are in `to_groups` with `allow_mesh_ips: true`, include the peer's mesh IP (`/32`) as an allowed destination for application traffic.
   - **Outbound routable networks**: If there is at least one outbound policy where the node's groups are in `from_groups` and the peer's groups are in `to_groups` with `allow_routable_networks: true`, include the peer's `routable_networks` in `AllowedIPs`.
5. **Deny-by-default**: If no relevant policies (outbound or inbound) exist between the node's groups and a remote node's groups, do not configure that remote node as a peer and do not add any of its IPs to `AllowedIPs`.

### Default Behavior

If no groups/policies are defined:
- **Current behavior**: Full mesh (all nodes peer with all nodes)
- **Rationale**: Backward compatibility, zero-config for simple deployments

If groups exist but no policies:
- **Deny-by-default**: Nodes in groups don't connect unless policy allows
- **Warning**: CLI warns if groups exist without policies

### Implementation Changes

#### 1. Data Structures (`pkg/mesh/types.go`)

```go
type Group struct {
    Description string   `json:"description,omitempty"`
    Members     []string `json:"members"` // hostnames
}

type AccessPolicy struct {
    Name                  string   `json:"name"`
    Description           string   `json:"description,omitempty"`
    FromGroups            []string `json:"from_groups"`
    ToGroups              []string `json:"to_groups"`
    AllowMeshIPs          bool     `json:"allow_mesh_ips"`
    AllowRoutableNetworks bool     `json:"allow_routable_networks"`
}

type Mesh struct {
    InterfaceName string                  `json:"interface_name"`
    Network       string                  `json:"network"`
    ListenPort    int                     `json:"listen_port"`
    Nodes         map[string]*Node        `json:"nodes"`
    LocalHostname string                  `json:"local_hostname"`
    Groups        map[string]*Group       `json:"groups,omitempty"`
    AccessPolicies []*AccessPolicy        `json:"access_policies,omitempty"`
    mu            sync.RWMutex            `json:"-"`
}
```

#### 2. Policy Evaluation (`pkg/mesh/policy.go` - NEW FILE)

```go
package mesh

// GetNodeGroups returns all groups that a node belongs to
func (m *Mesh) GetNodeGroups(hostname string) []string

// GetAllowedPeers returns the list of peer hostnames this node can connect to
func (m *Mesh) GetAllowedPeers(hostname string) map[string]*PeerAccess

type PeerAccess struct {
    AllowMeshIP          bool
    AllowRoutableNetworks bool
}

// ValidateGroups checks for group definition errors
func (m *Mesh) ValidateGroups() error

// ValidatePolicies checks for policy errors
func (m *Mesh) ValidatePolicies() error
```

#### 3. Deployment Changes (`pkg/mesh/deploy.go`)

Modify `generateConfigForNode()`:
- Check if groups/policies are defined
- If yes: Use policy engine to determine allowed peers
- If no: Use current full-mesh logic
- Filter AllowedIPs based on policy permissions

```go
func (m *Mesh) generateConfigForNode(node *Node) *WireGuardConfig {
    config := &WireGuardConfig{
        Interface: WGInterface{
            PrivateKey: node.PrivateKey,
            Address:    fmt.Sprintf("%s/16", node.MeshIP.String()),
            ListenPort: node.ListenPort,
        },
        Peers: make([]WGPeer, 0),
    }

    // Check if access control is enabled
    if len(m.Groups) > 0 || len(m.AccessPolicies) > 0 {
        // Use policy-based peer selection
        allowedPeers := m.GetAllowedPeers(node.Hostname)
        for peerHostname, access := range allowedPeers {
            peer := m.Nodes[peerHostname]
            peerConfig := m.buildPeerConfig(peer, access)
            config.Peers = append(config.Peers, peerConfig)
        }
    } else {
        // Default: full mesh (current behavior)
        for peerHostname, peer := range m.Nodes {
            if peerHostname == node.Hostname {
                continue
            }
            peerConfig := m.buildPeerConfigFullAccess(peer)
            config.Peers = append(config.Peers, peerConfig)
        }
    }

    return config
}

func (m *Mesh) buildPeerConfig(peer *Node, access *PeerAccess) WGPeer {
    allowedIPs := []string{}
    
    // Always include mesh /32 for handshakes when peer is configured
    allowedIPs = append(allowedIPs, fmt.Sprintf("%s/32", peer.MeshIP.String()))
    
    // Add routable networks only if policy permits
    if access.AllowRoutableNetworks {
        allowedIPs = append(allowedIPs, peer.RoutableNetworks...)
    }
    
    peerConfig := WGPeer{
        PublicKey:  peer.PublicKey,
        AllowedIPs: allowedIPs,
    }
    
    if peer.PublicEndpoint != "" {
        peerConfig.Endpoint = peer.PublicEndpoint
    }
    
    peerConfig.PersistentKeepalive = 5
    
    return peerConfig
}
```

#### 4. CLI Commands (Optional - Manual State Editing is Primary)

For initial implementation, **editing JSON directly is acceptable**. Future enhancement could add CLI commands:

```bash
# Future: Add nodes to groups
wgmesh group add production node1 node2
wgmesh group create staging --description "Staging environment"
wgmesh group list

# Future: Manage policies
wgmesh policy add prod-to-db --from production --to database
wgmesh policy list
wgmesh policy remove staging-isolated
```

**For MVP**: Users edit `/var/lib/wgmesh/mesh-state.json` directly to add groups and policies.

#### 5. Validation

Add validation on:
- `-init`: Accept new state format
- `-list`: Display group memberships and policies
- `-deploy`: Validate groups and policies before deployment
  - Check: All members exist as nodes
  - Check: All group names in policies exist
  - Check: No duplicate policy names
  - Check: Policies have at least one from_group and one to_group
  - Check: Policies match at least one node via their referenced groups
  - Check: No duplicate member entries within a group
  - Warn: Groups without policies
  - Warn: Nodes not in any group (when groups exist)

### Example Scenario

**Setup**: 3 environments (prod, staging, db), need staging isolated, prod can access db

```json
{
  "groups": {
    "prod": {"members": ["web1", "web2"]},
    "staging": {"members": ["web3"]},
    "db": {"members": ["db1"]}
  },
  "access_policies": [
    {
      "name": "prod-to-db",
      "from_groups": ["prod"],
      "to_groups": ["db"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    },
    {
      "name": "prod-internal",
      "from_groups": ["prod"],
      "to_groups": ["prod"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    },
    {
      "name": "staging-isolated",
      "from_groups": ["staging"],
      "to_groups": ["staging"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    },
    {
      "name": "db-to-prod",
      "from_groups": ["db"],
      "to_groups": ["prod"],
      "allow_mesh_ips": true,
      "allow_routable_networks": false
    }
  ]
}
```

**Result**:
- `web1` and `web2` can talk to each other and to `db1`
- `web3` can only talk to itself (isolated staging)
- `db1` can talk back to `web1` and `web2` (bidirectional)
- `web3` **cannot** reach `db1` (not allowed by any policy)

## Affected Files

### New Files
- `pkg/mesh/policy.go` - Policy evaluation logic (150-200 lines)
- `pkg/mesh/policy_test.go` - Unit tests for policy engine (200-300 lines)

### Modified Files
- `pkg/mesh/types.go` - Add Group and AccessPolicy structs (~40 lines added)
- `pkg/mesh/deploy.go` - Modify generateConfigForNode() to use policies (~50 lines modified)
- `pkg/mesh/mesh.go` - Add validation methods (~50 lines added)
- `README.md` - Document groups and access policies feature (~100-150 lines added)

### Documentation Files
- `README.md` - Add "Access Control" section with examples
- Potentially `docs/ACCESS_CONTROL.md` - Detailed guide (optional)

## Test Strategy

### Unit Tests

1. **Policy evaluation tests** (`pkg/mesh/policy_test.go`):
   - Test GetNodeGroups() with various group configurations
   - Test GetAllowedPeers() with different policies
   - Test policy validation (invalid groups, missing members, etc.)
   - Test allow_mesh_ips and allow_routable_networks flags

2. **Configuration generation tests** (`pkg/mesh/deploy_test.go`):
   - Test peer list filtering based on policies
   - Test AllowedIPs configuration with different policy settings
   - Test backward compatibility (no groups = full mesh)

### Integration Tests

1. **Manual testing**:
   - Create a 4-node mesh with 2 groups
   - Define policies for cross-group access
   - Deploy and verify WireGuard configs on each node
   - Test actual connectivity (ping, curl) between nodes
   - Verify isolation (blocked traffic doesn't work)

2. **Validation testing**:
   - Invalid group names in policies (should error)
   - Nodes not in any group (should warn)
   - Groups without policies (should warn)
   - Empty groups (should warn)

### Test Scenarios

**Scenario 1: Basic isolation**
- Groups: A={node1, node2}, B={node3, node4}
- Policy: A can only talk to A, B can only talk to B
- Verify: node1 cannot reach node3

**Scenario 2: Hub-and-spoke**
- Groups: hub={node1}, spoke={node2, node3, node4}
- Policy: spoke→hub allowed, spoke→spoke denied
- Verify: node2 can reach node1, but not node3

**Scenario 3: Routable networks**
- node1 has routable_networks=["192.168.1.0/24"]
- Policy allows mesh_ips but not routable_networks
- Verify: node2 can ping node1's mesh IP, but not 192.168.1.0/24

**Scenario 4: Backward compatibility**
- Create mesh without groups/policies
- Verify: Full mesh connectivity (current behavior)

## Estimated Complexity

**Medium**

### Rationale

- **Data model changes**: Straightforward addition of groups and policies to existing Mesh struct
- **Policy engine**: Moderate complexity - group membership resolution and access evaluation
- **Configuration generation**: Modify existing function, need to handle both code paths (with/without policies)
- **Testing**: Requires comprehensive testing for policy evaluation and network isolation
- **No external dependencies**: Pure Go implementation, uses existing WireGuard AllowedIPs mechanism
- **Backward compatible**: Must preserve current behavior when groups not used

### Estimated Effort

- Design & data model: 1-2 hours
- Policy evaluation logic: 3-4 hours
- Integration with deployment: 2-3 hours
- Unit tests: 2-3 hours
- Integration/manual testing: 2-3 hours
- Documentation: 1-2 hours

**Total: 11-17 hours (approximately 2-3 days)**

### Risks

1. **AllowedIPs edge cases**: Need to ensure correct CIDR handling and no overlaps
2. **Policy conflicts**: What if policies are contradictory? (Resolved: policies are additive, not subtractive)
3. **Validation complexity**: Need robust validation to catch configuration errors early
4. **Testing isolation**: Requires actual multi-node setup to verify network isolation works

### Future Enhancements (Out of Scope for MVP)

1. **CLI commands**: `wgmesh group add/remove`, `wgmesh policy add/remove`
2. **RBAC**: Role-based access control with more granular permissions
3. **Time-based policies**: Allow access only during certain hours
4. **IP-based policies**: Allow specific IPs/ports instead of all-or-nothing
5. **Audit logging**: Log policy evaluation decisions
6. **Dynamic updates**: Change policies without redeploying all nodes
7. **Policy inheritance**: Nested groups, policy templates
8. **Deny rules**: Explicit deny (currently only allow rules)

## Alternative Approach: RPC-Based Distributed ACL Management

### Overview

An alternative to the centralized SSH deployment approach is to leverage the existing RPC infrastructure (currently used in decentralized mode) to enable distributed ACL management. This would allow any node in the mesh to manage and deploy access policies to all other nodes.

### Motivation

The RPC interface (`pkg/rpc/`) currently supports:
- `peers.list` - List all active peers
- `peers.get` - Get specific peer information
- `peers.count` - Get peer statistics
- `daemon.status` - Get daemon status
- `daemon.ping` - Health check

This could be extended to support ACL management operations, enabling a more distributed and dynamic approach to access control.

### Proposed RPC Methods for ACL Management

Add new RPC methods to the server:

1. **`acl.groups.list`** - List all defined groups
2. **`acl.groups.get`** - Get group details (members)
3. **`acl.groups.set`** - Create/update a group
4. **`acl.groups.delete`** - Remove a group
5. **`acl.policies.list`** - List all access policies
6. **`acl.policies.get`** - Get policy details
7. **`acl.policies.set`** - Create/update an access policy
8. **`acl.policies.delete`** - Remove a policy
9. **`acl.apply`** - Apply current ACL configuration to WireGuard

### Architecture for RPC-Based ACL

```
┌─────────────────────────────────────────────┐
│  Node A (any node in mesh)                  │
│  ┌───────────────────────────────────────┐  │
│  │  wgmesh acl group add prod node1 node2│  │
│  │  (CLI command)                         │  │
│  └───────────────────────────────────────┘  │
│         │                                    │
│         ▼                                    │
│  ┌───────────────────────────────────────┐  │
│  │  RPC Client                            │  │
│  │  - Connect to local RPC socket         │  │
│  │  - Send acl.groups.set request         │  │
│  └───────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────┐
│  Node B, C, D, ... (all nodes)              │
│  ┌───────────────────────────────────────┐  │
│  │  RPC Server                            │  │
│  │  - Receive acl.groups.set              │  │
│  │  - Update local ACL state              │  │
│  │  - Propagate to other peers (gossip)   │  │
│  └───────────────────────────────────────┘  │
│         │                                    │
│         ▼                                    │
│  ┌───────────────────────────────────────┐  │
│  │  ACL Engine                            │  │
│  │  - Evaluate policies                   │  │
│  │  - Reconfigure WireGuard peers         │  │
│  │  - Update AllowedIPs                   │  │
│  └───────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
```

### Data Synchronization

To ensure ACL consistency across all nodes:

1. **State storage**: Each node stores ACL configuration locally (e.g., `/var/lib/wgmesh/acl-state.json`)
2. **Gossip protocol**: Use existing gossip layer (`pkg/discovery/gossip.go`) to propagate ACL changes
3. **Versioning**: Include version numbers/timestamps to resolve conflicts
4. **Consensus**: Use last-write-wins or implement a simple consensus mechanism

### Example Usage (RPC-Based Approach)

```bash
# On any node in the mesh
wgmesh acl group create prod --members node1,node2
wgmesh acl group create db --members node3

wgmesh acl policy add prod-to-db \
  --from prod \
  --to db \
  --allow-mesh \
  --allow-networks

# Changes propagate to all nodes via RPC + gossip
# Each node reconfigures its WireGuard peers
```

### Advantages of RPC Approach

1. **Distributed management**: Any authorized node can manage ACLs
2. **Real-time updates**: Changes apply immediately without SSH deployment
3. **No central controller**: Doesn't require operator workstation to be online
4. **Consistent with decentralized mode**: Uses same RPC infrastructure
5. **Better for dynamic environments**: Cloud auto-scaling, ephemeral nodes

### Disadvantages of RPC Approach

1. **Complexity**: Requires distributed state management and synchronization
2. **Authentication**: Need to secure RPC calls (who can modify ACLs?)
3. **Conflict resolution**: Multiple simultaneous changes need coordination
4. **Testing**: More complex to test distributed state consistency
5. **Debugging**: Harder to troubleshoot when state diverges across nodes

### Hybrid Approach

A practical middle ground:

1. **Initial deployment**: Use centralized SSH deployment (simpler, more reliable)
2. **Runtime updates**: Add RPC methods for runtime ACL modifications
3. **State of record**: Centralized state file remains authoritative
4. **Periodic sync**: Operator can push canonical state via SSH deployment

This combines the simplicity of centralized management with the flexibility of distributed updates.

### Implementation Considerations

If pursuing RPC-based ACL:

1. **Security**: 
   - Authenticate RPC calls (verify caller is authorized)
   - Encrypt RPC payloads (use existing crypto layer)
   - Audit log all ACL changes

2. **State management**:
   - Use CRDTs or vector clocks for conflict-free state merging
   - Implement rollback mechanism for bad ACL changes
   - Version all ACL configurations

3. **Backward compatibility**:
   - RPC-based ACL should coexist with SSH deployment
   - Allow reading centralized state file and converting to distributed state
   - Support migration path from centralized to distributed ACL

4. **New files needed**:
   - `pkg/acl/state.go` - ACL state management
   - `pkg/acl/sync.go` - State synchronization via gossip
   - `pkg/rpc/acl_handlers.go` - RPC handlers for ACL operations
   - Add RPC methods to `pkg/rpc/server.go`

### Recommendation

**For MVP**: Implement the centralized SSH deployment approach (original proposal)
- Simpler to implement and test
- More predictable and debuggable
- Sufficient for most use cases
- Can add RPC-based management later as enhancement

**For future**: Consider RPC-based approach as Phase 2
- Builds on working centralized implementation
- Adds distributed management capabilities
- Better supports dynamic cloud environments
- Requires careful design of state synchronization

## Implementation Phases

### Phase 1: Centralized SSH Deployment ACL (MVP)

**Objective**: Add group-based access control to centralized mode using SSH deployment, with policies stored in mesh state file.

#### Phase 1.1: Core Data Model & Validation (3-4 hours)

**Tasks**:
1. Extend `pkg/mesh/types.go`:
   - Add `Group` struct with `Description` and `Members` fields
   - Add `AccessPolicy` struct with all policy fields
   - Add `Groups` and `AccessPolicies` fields to `Mesh` struct
   - Ensure JSON marshaling/unmarshaling works correctly

2. Create `pkg/mesh/policy.go`:
   - Implement `ValidateGroups()` - check for invalid group definitions
   - Implement `ValidatePolicies()` - verify group references exist
   - Add helper functions for group/policy lookups

3. Update `pkg/mesh/mesh.go`:
   - Keep `Load()` and `Save()` permissive (no strict validation that would block incremental edits)
   - Expose hooks/helpers so CLI commands (e.g., `-deploy`, optionally `-list`) can run strict validation and emit warning logs for common misconfigurations

**Deliverables**:
- Data structures support groups and policies
- Validation logic is implemented and used by deploy-time commands to prevent invalid configurations while allowing incremental state edits
- Clear error and warning messages for misconfigurations when strict validation is invoked

**Tests**:
- Unit tests for JSON marshaling with groups/policies
- Validation tests for invalid group names, missing members, etc.
- Tests for overlapping group memberships (allowed)

#### Phase 1.2: Policy Evaluation Engine (4-5 hours)

**Tasks**:
1. Implement in `pkg/mesh/policy.go`:
   - `GetNodeGroups(hostname string) []string` - return groups for a node
   - `GetAllowedPeers(hostname string) map[string]*PeerAccess` - evaluate policies
   - `PeerAccess` struct to track allowed access levels per peer

2. Policy evaluation logic:
   - For given node, find all groups it belongs to
   - For each policy where node's group is in `from_groups`:
     - Collect all nodes in `to_groups`
     - Record access permissions (mesh IPs, routable networks)
   - Handle nodes in multiple groups (union of permissions)
   - Handle bidirectional policies correctly

3. Edge cases:
   - Node in no groups (when groups exist) → no peers
   - Node in multiple groups → merge permissions
   - Self-referencing policies (group can talk to itself)

**Deliverables**:
- Policy engine correctly evaluates allowed peers
- Handles all edge cases and multi-group scenarios
- Clear permission model (allow-only, no deny rules in MVP)

**Tests**:
- Unit tests for policy evaluation with various scenarios
- Test multi-group memberships
- Test bidirectional and self-referencing policies
- Test nodes not in any group

#### Phase 1.3: WireGuard Configuration Integration (3-4 hours)

**Tasks**:
1. Modify `pkg/mesh/deploy.go`:
   - Update `generateConfigForNode()` to check for groups/policies
   - If groups exist, use `GetAllowedPeers()` instead of full mesh
   - Build peer configs with filtered AllowedIPs based on `PeerAccess`
   - Maintain backward compatibility (no groups = full mesh)

2. Implement helper functions:
   - `buildPeerConfig(peer *Node, access *PeerAccess) WGPeer`
   - `buildPeerConfigFullAccess(peer *Node) WGPeer` (for backward compat)

3. Route configuration:
   - Update `collectAllRoutesForNode()` to respect policies
   - Only include routes for networks we have policy access to

**Deliverables**:
- WireGuard configs reflect ACL policies
- AllowedIPs correctly filtered per policy
- Backward compatible with existing meshes

**Tests**:
- Integration tests generating configs with policies
- Verify AllowedIPs match policy permissions
- Test backward compatibility without groups

#### Phase 1.4: CLI Enhancements & Documentation (2-3 hours)

**Tasks**:
1. Update `-list` command:
   - Display group memberships for each node
   - Show access policies summary
   - Warn if nodes exist without group membership (when groups defined)

2. Add validation to `-deploy`:
   - Run `ValidateGroups()` and `ValidatePolicies()` before deployment
   - Show clear error messages for invalid configurations
   - Provide deployment preview showing ACL changes

3. Documentation:
   - Add "Access Control" section to README.md
   - Include example JSON configurations
   - Document policy evaluation rules
   - Provide common use case examples (prod/staging isolation, hub-and-spoke)

**Deliverables**:
- Enhanced CLI shows ACL status clearly
- Comprehensive documentation with examples
- User-friendly error messages

**Tests**:
- Manual testing of CLI output with groups/policies
- Verify documentation examples work correctly

#### Phase 1.5: Testing & Validation (2-3 hours)

**Tasks**:
1. Integration testing:
   - Create 4-node test mesh with 2 groups
   - Deploy with various policy configurations
   - Verify actual connectivity matches policies (ping tests)
   - Test policy changes and redeployment

2. Test scenarios:
   - Basic isolation (group A cannot reach group B)
   - Hub-and-spoke (spokes can reach hub, not each other)
   - Partial network access (mesh IPs yes, routable networks no)
   - Backward compatibility (no groups/policies)

3. Error handling:
   - Test with malformed JSON
   - Test with invalid group references
   - Test with circular dependencies (if applicable)

**Deliverables**:
- Comprehensive test suite
- Real-world validation of network isolation
- Documented test procedures

**Tests**:
- Automated integration tests where possible
- Manual multi-node deployment tests
- Connectivity verification scripts

**Phase 1 Total Estimate**: 14-19 hours (2-3 days)

### Phase 2: RPC-Based Distributed ACL Management

**Objective**: Enable distributed ACL management via RPC, allowing any node to manage and deploy policies across the mesh.

**Prerequisites**: Phase 1 completed and stable

#### Phase 2.1: ACL State Management (4-5 hours)

**Tasks**:
1. Create `pkg/acl/state.go`:
   - `ACLState` struct to hold groups and policies with versioning
   - Version/timestamp fields for conflict resolution
   - Methods: `Load()`, `Save()`, `Merge()`, `Validate()`

2. Create `pkg/acl/store.go`:
   - Persistent storage for ACL state (e.g., `/var/lib/wgmesh/acl-state.json`)
   - Thread-safe access (mutex protection)
   - Atomic updates to prevent corruption

3. Versioning strategy:
   - Use vector clocks or Lamport timestamps
   - Last-write-wins for simple conflict resolution
   - Optional: implement CRDT for conflict-free merging

**Deliverables**:
- ACL state can be stored and loaded persistently
- Version tracking enables conflict detection
- Thread-safe state management

**Tests**:
- Unit tests for state save/load
- Concurrent access tests (race detector)
- Version comparison and merge tests

#### Phase 2.2: RPC Protocol Extensions (5-6 hours)

**Tasks**:
1. Extend `pkg/rpc/protocol.go`:
   - Add request/response types for ACL operations
   - Define `ACLGroupInfo`, `ACLPolicyInfo` structs
   - Add error codes for ACL-specific errors

2. Add RPC handlers in `pkg/rpc/server.go`:
   - `acl.groups.list` - return all groups
   - `acl.groups.get` - get group details
   - `acl.groups.set` - create/update group
   - `acl.groups.delete` - remove group
   - `acl.policies.list` - return all policies
   - `acl.policies.get` - get policy details
   - `acl.policies.set` - create/update policy
   - `acl.policies.delete` - remove policy
   - `acl.apply` - trigger ACL application to WireGuard

3. Implement in `pkg/rpc/acl_handlers.go`:
   - Handler implementations for each RPC method
   - Validation of input parameters
   - Error handling and response formatting

**Deliverables**:
- RPC methods for ACL management implemented
- Proper error handling and validation
- Protocol documentation

**Tests**:
- Unit tests for each RPC handler
- Test invalid inputs and error cases
- Test concurrent RPC calls

#### Phase 2.3: Gossip-Based State Propagation (6-8 hours)

**Tasks**:
1. Create `pkg/acl/sync.go`:
   - Implement gossip protocol for ACL state distribution
   - Use existing `pkg/discovery/gossip.go` as reference
   - State synchronization on startup and periodic refresh

2. Gossip message types:
   - `ACLStateAnnouncement` - announce state version
   - `ACLStateRequest` - request full state from peer
   - `ACLStateResponse` - send full state to peer
   - `ACLDelta` - incremental updates (optimization)

3. Synchronization logic:
   - On startup: request state from all known peers
   - Periodically: announce state version
   - On version mismatch: request and merge newer state
   - Apply merged state to local WireGuard config

**Deliverables**:
- ACL state propagates across all mesh nodes
- Eventual consistency achieved via gossip
- Handles network partitions gracefully

**Tests**:
- Multi-node gossip simulation tests
- Network partition and recovery tests
- State convergence time measurements

#### Phase 2.4: WireGuard Dynamic Reconfiguration (4-5 hours)

**Tasks**:
1. Create `pkg/acl/apply.go`:
   - `ApplyACLToWireGuard()` - reconfigure WireGuard based on ACL
   - Use existing `pkg/wireguard/config.go` for config generation
   - Calculate diff between current and desired config
   - Apply changes using `wg set` commands

2. Integration with daemon:
   - Listen for ACL state changes
   - Automatically trigger reconfiguration
   - Rate limiting to prevent flapping
   - Rollback mechanism on failure

3. Edge cases:
   - Handle nodes joining/leaving during ACL updates
   - Preserve existing connections where possible
   - Log all ACL-driven config changes

**Deliverables**:
- ACL changes apply to WireGuard automatically
- Minimal disruption to existing connections
- Audit trail of ACL-driven changes

**Tests**:
- Test ACL changes trigger reconfig
- Verify connections preserved when allowed
- Test rollback on configuration errors

#### Phase 2.5: CLI Commands for Distributed ACL (3-4 hours)

**Tasks**:
1. Add new CLI commands to `main.go`:
   - `wgmesh acl group create <name> --members <node1,node2>`
   - `wgmesh acl group delete <name>`
   - `wgmesh acl group list`
   - `wgmesh acl policy add <name> --from <group1> --to <group2> [--allow-mesh] [--allow-networks]`
   - `wgmesh acl policy delete <name>`
   - `wgmesh acl policy list`
   - `wgmesh acl status` - show current ACL state and sync status

2. Implement `aclCmd()` function:
   - Parse subcommands and flags
   - Connect to local RPC socket
   - Send appropriate RPC requests
   - Format and display responses

3. User experience:
   - Clear output formatting (tables for lists)
   - Progress indicators for state propagation
   - Confirmation prompts for destructive operations

**Deliverables**:
- User-friendly CLI for ACL management
- Works from any node in the mesh
- Clear feedback on operation status

**Tests**:
- CLI integration tests
- Test all commands with various inputs
- Error message clarity tests

#### Phase 2.6: Security & Authentication (5-6 hours)

**Tasks**:
1. RPC authentication:
   - Implement token-based authentication for RPC calls
   - Use shared secret derived from mesh secret (for decentralized mode)
   - Or use PKI with node public keys (for centralized mode)

2. Authorization:
   - Define ACL admin role/capability
   - Only authorized nodes can modify ACL state
   - Read-only access for status queries

3. Audit logging:
   - Log all ACL changes with timestamp and originator
   - Store audit log persistently
   - Optional: forward audit logs to central syslog

4. Encryption:
   - Encrypt ACL state in gossip messages
   - Use existing crypto primitives from `pkg/crypto/`

**Deliverables**:
- Secure RPC communication
- Authorization for ACL modifications
- Comprehensive audit trail

**Tests**:
- Test unauthorized access attempts
- Verify audit logs capture all changes
- Test encryption of gossip messages

#### Phase 2.7: Integration Testing & Documentation (4-5 hours)

**Tasks**:
1. End-to-end testing:
   - Deploy multi-node mesh with RPC-based ACL
   - Test ACL changes from different nodes
   - Verify state propagation and convergence
   - Test network partitions and recovery

2. Performance testing:
   - Measure ACL state propagation time
   - Test with 10, 50, 100 nodes
   - Identify bottlenecks and optimize

3. Documentation:
   - Add "Distributed ACL Management" section to README
   - Document RPC protocol for ACL operations
   - Provide migration guide from Phase 1 to Phase 2
   - Include troubleshooting guide

4. Migration path:
   - Tool to convert centralized mesh state to distributed ACL state
   - Backward compatibility considerations
   - Rollback procedures

**Deliverables**:
- Fully tested distributed ACL system
- Comprehensive documentation
- Migration tools and guides

**Tests**:
- Large-scale mesh simulations
- Performance benchmarks
- Migration scenario tests

**Phase 2 Total Estimate**: 31-39 hours (5-7 days)

### Combined Implementation Timeline

**Phase 1 (MVP)**: 14-19 hours (2-3 days)
- Core ACL functionality for centralized mode
- Manual JSON editing for policies
- SSH deployment applies policies

**Phase 2 (Distributed)**: 31-39 hours (5-7 days)
- RPC-based ACL management
- Gossip-based state synchronization
- Dynamic WireGuard reconfiguration
- Full CLI for ACL operations

**Total**: 45-58 hours (7-10 days of development time)

### Development Dependencies

**Phase 1 depends on**:
- Existing mesh deployment infrastructure
- WireGuard configuration generation

**Phase 2 depends on**:
- Phase 1 complete and stable
- Existing RPC infrastructure (`pkg/rpc/`)
- Existing gossip protocol (`pkg/discovery/gossip.go`)
- Existing crypto primitives (`pkg/crypto/`)

### Risk Mitigation

**Phase 1 risks**:
- Risk: Policy evaluation logic errors → **Mitigation**: Comprehensive unit tests, test scenarios
- Risk: Breaking backward compatibility → **Mitigation**: Feature flag, extensive testing without groups
- Risk: User confusion with JSON editing → **Mitigation**: Clear documentation, validation with helpful errors

**Phase 2 risks**:
- Risk: State synchronization conflicts → **Mitigation**: Vector clocks, last-write-wins, thorough testing
- Risk: Security vulnerabilities in RPC → **Mitigation**: Authentication, authorization, encryption, audit logging
- Risk: Network partitions causing divergent state → **Mitigation**: Gossip protocol, eventual consistency, partition detection
- Risk: Performance degradation with many nodes → **Mitigation**: Performance testing, optimization, rate limiting

## Notes

- This feature does NOT require changes to the decentralized mode (daemon), only centralized mode
- The access control is enforced at the WireGuard level via AllowedIPs, not via external firewalls
- Policies are statically evaluated at deployment time, not dynamically at runtime
- Initial implementation focuses on manual JSON editing; CLI commands can be added later
- Groups can overlap (a node can be in multiple groups) - policies are evaluated independently
- **RPC-based distributed ACL** is an alternative approach worth considering for future enhancements
