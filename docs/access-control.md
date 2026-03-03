# Access Control and Network Segmentation

wgmesh supports group-based access control in centralized mode, allowing you to segment your mesh network and control which nodes can communicate with each other.

## Overview

Without access control, every node in the mesh can communicate with every other node (full mesh topology). With access control enabled:
- Nodes are organized into **groups**
- **Policies** define which groups can communicate
- WireGuard `AllowedIPs` filtering enforces the policies
- Nodes can only reach peers that have at least one policy connecting their groups

## Defining Groups

Groups are defined in the `/var/lib/wgmesh/mesh-state.json` file:

```json
{
  "groups": {
    "production": {
      "description": "Production environment nodes",
      "members": ["web1", "web2", "app1", "db1"]
    },
    "staging": {
      "description": "Staging environment",
      "members": ["web3", "app2", "db2"]
    },
    "database": {
      "description": "Database servers",
      "members": ["db1", "db2"]
    }
  }
}
```

- `description`: Optional human-readable description
- `members`: List of node hostnames that belong to this group

## Defining Access Policies

Policies define communication rules between groups:

```json
{
  "access_policies": [
    {
      "name": "prod-to-db",
      "description": "Allow production nodes to access databases",
      "from_groups": ["production"],
      "to_groups": ["database"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    },
    {
      "name": "prod-internal",
      "description": "Production nodes can talk to each other",
      "from_groups": ["production"],
      "to_groups": ["production"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    },
    {
      "name": "staging-isolated",
      "description": "Staging is isolated from production",
      "from_groups": ["staging"],
      "to_groups": ["staging"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    }
  ]
}
```

Policy fields:
- `name`: Unique policy identifier
- `description`: Optional description
- `from_groups`: Groups initiating the connection
- `to_groups`: Groups being accessed
- `allow_mesh_ips`: Allow access to mesh IPs (direct node-to-node communication)
- `allow_routable_networks`: Allow access to networks behind target nodes

## How Policy Evaluation Works

For each node, wgmesh evaluates policies to determine which peers to configure:

1. **Find node's groups**: Collect all groups where this node is a member
2. **Determine relevant policies**:
   - **Outbound policies**: Policies where the node's groups appear in `from_groups`
   - **Inbound policies**: Policies where the node's groups appear in `to_groups`
3. **Build peer list**: For every policy that connects the node's groups to other groups, add all members of those other groups as WireGuard peers
4. **Configure AllowedIPs**:
   - Always include mesh IP (`/32`) for handshakes when peer is configured
   - Include mesh IP as destination if `allow_mesh_ips: true` in outbound policy
   - Include `routable_networks` if `allow_routable_networks: true` in outbound policy
5. **Deny-by-default**: If no policies connect the node's groups to another node's groups, no peer configuration is created

**Important**: Policies are evaluated bidirectionally for peer configuration. If any policy relates two nodes' groups (in either direction), both nodes will have each other configured as peers. However, the `AllowedIPs` settings are directional based on outbound policies.

## Example: Three-Tier Architecture

**Scenario**: Production web and app servers need to reach databases, but staging must be isolated.

```json
{
  "groups": {
    "web": {
      "members": ["web1", "web2"]
    },
    "app": {
      "members": ["app1"]
    },
    "db": {
      "members": ["db1"]
    },
    "staging": {
      "members": ["web3", "app2", "db2"]
    }
  },
  "access_policies": [
    {
      "name": "web-to-app",
      "from_groups": ["web"],
      "to_groups": ["app"],
      "allow_mesh_ips": true,
      "allow_routable_networks": false
    },
    {
      "name": "app-to-db",
      "from_groups": ["app"],
      "to_groups": ["db"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    },
    {
      "name": "db-to-app",
      "from_groups": ["db"],
      "to_groups": ["app"],
      "allow_mesh_ips": true,
      "allow_routable_networks": false
    },
    {
      "name": "staging-isolated",
      "from_groups": ["staging"],
      "to_groups": ["staging"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    }
  ]
}
```

**Result**:
- `web1` can reach `app1` but not `db1` directly (must go through `app1`)
- `app1` can reach `web1`, `web2`, and `db1`
- `db1` can reach `app1` (for responses) but not `web1` or `web2`
- `web3`, `app2`, `db2` can only reach each other (isolated from production)
- Staging cannot reach production and vice versa

## Example: Hub-and-Spoke

**Scenario**: Field offices should only communicate with HQ, not each other.

```json
{
  "groups": {
    "hq": {
      "members": ["hq1", "hq2"]
    },
    "office": {
      "members": ["office1", "office2", "office3"]
    }
  },
  "access_policies": [
    {
      "name": "office-to-hq",
      "from_groups": ["office"],
      "to_groups": ["hq"],
      "allow_mesh_ips": true,
      "allow_routable_networks": true
    },
    {
      "name": "hq-to-office",
      "from_groups": ["hq"],
      "to_groups": ["office"],
      "allow_mesh_ips": true,
      "allow_routable_networks": false
    }
  ]
}
```

**Result**:
- `office1` can reach `hq1` and `hq2` (and their routable networks)
- `office2` cannot reach `office1` or `office3`
- `hq1` can reach all offices (for responses to connections initiated from offices)
- Offices are isolated from each other for security

## Viewing Access Control Configuration

Use the `-list` command to see groups, policies, and memberships:

```bash
wgmesh -list
```

Output:
```
Mesh Network: 10.99.0.0/16
Interface: wg0
Listen Port: 51820

Access Control: Enabled

Groups:
  production (2 members):
    - web1
    - web2

  staging (1 member):
    - web3

Policies:
  prod-internal:
    From: production
    To: production
    Mesh IPs: Yes
    Routable Networks: Yes

Nodes:
  web1:
    Mesh IP: 10.99.0.1
    Groups: [production]
    SSH: 192.168.1.10:22

  web2:
    Mesh IP: 10.99.0.2
    Groups: [production]
    SSH: 192.168.1.11:22

  web3:
    Mesh IP: 10.99.0.3
    Groups: [staging]
    SSH: 192.168.1.12:22
```

## Validation and Warnings

When you run `wgmesh -deploy`, wgmesh validates the access control configuration:

**Validation checks**:
- All group members must exist as nodes
- All group names in policies must exist
- Policies must have at least one `from_groups` and one `to_groups`
- No duplicate policy names
- No duplicate members within a group

**Warnings**:
- Groups defined without policies: "Groups exist but no access policies defined. Nodes in groups will have no peers."
- Nodes not in any group: "Node X is not a member of any group (when groups are defined)."

## Backward Compatibility

**No groups or policies defined**: Full mesh topology (all nodes connect to all nodes)

**Groups defined but no policies**: Deny-by-default behavior. Nodes in groups will have no peers configured.

**Backward compatible**: Existing meshes without groups continue to work exactly as before.

## Best Practices

1. **Always use policies with groups**: If you define groups, define at least one policy that references them
2. **Be explicit with policy direction**: Consider both `allow_mesh_ips` and `allow_routable_networks` flags
3. **Test isolated networks**: Verify that nodes that shouldn't communicate truly cannot reach each other
4. **Document your architecture**: Add clear descriptions to groups and policies
5. **Use descriptive names**: Choose group and policy names that reflect their purpose
6. **Plan for bidirectional access**: Remember that peer configuration is symmetric, but `AllowedIPs` are directional

## Common Patterns

**Allow all access within a group**:
```json
{
  "name": "group-internal",
  "from_groups": ["groupname"],
  "to_groups": ["groupname"],
  "allow_mesh_ips": true,
  "allow_routable_networks": true
}
```

**Allow mesh access but not routable networks**:
```json
{
  "name": "limited-access",
  "from_groups": ["client"],
  "to_groups": ["server"],
  "allow_mesh_ips": true,
  "allow_routable_networks": false
}
```

**Return traffic only (no initiated connections)**:
```json
{
  "name": "read-only-response",
  "from_groups": ["server"],
  "to_groups": ["client"],
  "allow_mesh_ips": true,
  "allow_routable_networks": false
}
```

This configures the peer (for WireGuard handshakes) but doesn't allow the server to initiate new connections to the client's routable networks.
