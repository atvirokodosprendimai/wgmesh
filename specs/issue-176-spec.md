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

1. **Find node's groups**: Collect all groups where this node is a member
2. **Find allowed target groups**: For each policy where node's group is in `from_groups`, collect all groups in `to_groups`
3. **Build peer list**: Add peers only if their group is in allowed target groups
4. **Configure AllowedIPs**: Based on policy settings:
   - `allow_mesh_ips: true` → Include peer's mesh IP in AllowedIPs
   - `allow_routable_networks: true` → Include peer's routable networks in AllowedIPs

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
    
    if access.AllowMeshIP {
        allowedIPs = append(allowedIPs, fmt.Sprintf("%s/32", peer.MeshIP.String()))
    }
    
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
  - Check: No circular references
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

## Notes

- This feature does NOT require changes to the decentralized mode (daemon), only centralized mode
- The access control is enforced at the WireGuard level via AllowedIPs, not via external firewalls
- Policies are statically evaluated at deployment time, not dynamically at runtime
- Initial implementation focuses on manual JSON editing; CLI commands can be added later
- Groups can overlap (a node can be in multiple groups) - policies are evaluated independently
- **RPC-based distributed ACL** is an alternative approach worth considering for future enhancements
