---
tldr: JSON state file is the single source of truth for all nodes, keys, and access policy; groups + policies optionally filter full-mesh defaults to per-node allowed peers.
category: core
---

# Mesh state and access control

## Target

Centralized representation of the WireGuard mesh: who is in it, what keys they hold, how they connect, and who can reach whom.

## Behaviour

- The mesh is stored as a JSON file containing all nodes, WireGuard keypairs, SSH access coordinates, network topology, and optional access control config.
- The state file is optionally encrypted with a symmetric password; the raw bytes are base64-encoded ciphertext when encryption is active.
- Each node is identified by its hostname and carries: WireGuard keypair, mesh IP (within the mesh CIDR), SSH host+port, listen port, NAT flag, optional public endpoint, optional routable networks, and actual/FQDN values populated at deploy time.
- Adding a node generates a fresh WireGuard keypair immediately; the private key lives only in the state file.
- **Default (no groups/policies): full mesh.** Every node is a peer of every other node, with unrestricted access to all mesh IPs and routable networks.
- **Access control enabled: deny by default.** A node with no group membership gets zero peers.
  - Groups are named sets of hostnames.
  - Access policies declare reachability between groups (from_groups → to_groups).
  - Policy evaluation is bidirectional: a node gains a peer if any policy has the node's group on either side.
  - `allow_mesh_ips` and `allow_routable_networks` are controlled independently per policy.
  - Routable network access accumulates only from outbound policy direction.
- Access control is opt-in: existing deployments without groups or policies continue to behave as full mesh.

## Design

- `Mesh` struct is the root; serialised as the state file.
- `Node` carries both the WireGuard identity (keys, mesh IP, endpoint) and the management channel (SSH host/port) in one record.
- `GetAllowedPeers(hostname) map[string]*PeerAccess` is the policy computation kernel: iterates all policies, accumulates `PeerAccess` per peer hostname, skips self and nodes absent from the nodes map.
- `FullConfigToConfig` bridges two internal config representations: `FullConfig` (slice, used when writing) and `Config` (map by pubkey, used for diffing against live state).

## Interactions

- `pkg/wireguard` — `generateConfigForNode` consumes `GetAllowedPeers` output to build per-node WireGuard peer lists.
- `pkg/crypto` — optional encryption/decryption of the state file on load/save.
- `pkg/mesh/deploy.go` — orchestrates deployment using the state as input; writes back `ActualHostname`, `FQDN`, `BehindNAT`, `PublicEndpoint` after detection.

## Mapping

> [[pkg/mesh/types.go]]
> [[pkg/mesh/mesh.go]]
> [[pkg/mesh/policy.go]]
