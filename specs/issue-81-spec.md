# Specification: Issue #81

## Classification
feature

## Deliverables
code + documentation

## Problem Analysis

Currently, users need to SSH into each server to find that server's VPN IP address, which is inconvenient and time-consuming, especially for larger mesh networks. There is no simple way to get an overview of all nodes in the mesh with their hostnames and corresponding VPN IPs.

### Current State - Centralized Mode

The centralized mesh mode already has a `-list` flag that displays detailed node information:
- Mesh network configuration
- Node hostname (with local and NAT markers)
- Mesh IP
- SSH connection details
- Public key
- Public endpoint (if available)
- Routable networks

However, this output is verbose and not optimized for quickly finding a specific node's VPN IP. The request is for a simplified output format showing just hostname + VPN IP pairs.

### Current State - Decentralized Mode (DHT)

The decentralized daemon mode (`join` command) uses DHT-based peer discovery:
- Peers stored in `PeerStore` (in-memory, keyed by WireGuard public key)
- Each `PeerInfo` contains: WGPubKey, MeshIP, Endpoint, RoutableNetworks, LastSeen, DiscoveredVia
- **Important limitation:** DHT mode does NOT currently collect or store hostnames
- The `status` command shows basic mesh configuration but doesn't list peers
- The daemon's `printStatus()` logs peers as: `pubkey_prefix... (mesh_ip) via [discovery_methods]`

### User Experience Issue

The feature request specifically mentions:
- Command should be something like `./wgmesh mesh list`
- Output should be simple hostname + IP pairs (e.g., `node1 10.39.0.1`)
- The mesh system should collect server hostname and FQDN on start
- Should work with **both** centralized mode (state.json) and decentralized mode (DHT)

### Technical Context

**Centralized mode** (`pkg/mesh`):
- Node data stored in `Mesh.Nodes` (map of hostname → Node struct)
- Each `Node` struct already contains:
  - `Hostname` field (already collected during node addition)
  - `MeshIP` field (net.IP, the VPN overlay IP)
- The existing `-list` flag calls `Mesh.List()` method in `pkg/mesh/mesh.go`
- Hostname is specified during node addition: `hostname:mesh_ip:ssh_host[:ssh_port]`

**Decentralized mode** (`pkg/daemon`):
- Peers stored in `PeerStore.peers` (map of pubkey → PeerInfo)
- `PeerInfo` struct does NOT have a hostname field
- `PeerAnnouncement` (used in DHT exchange) does NOT include hostname
- Would need to extend the announcement protocol to share hostnames
- Each node can get its own hostname via `os.Hostname()`

## Proposed Approach

The solution must support **both** centralized mode (mesh-state.json) and decentralized mode (DHT). However, DHT mode currently lacks hostname information, so we need to extend the protocol.

### Overall Strategy

1. **Centralized mode:** Add simple list output from existing mesh state
2. **Decentralized mode:** Extend peer announcement protocol to include hostname, then add list capability
3. **Unified CLI:** Provide consistent `-list-simple` flag for both modes

### Part 1: Centralized Mode Implementation

Add a new flag `-list-simple` for compact hostname + IP output:
```bash
./wgmesh -list-simple
# Output:
node1 10.39.0.1
node2 10.39.55.15
node3 10.39.100.20
```

**Implementation:**
1. Add `-list-simple` flag in `main.go`
2. Create `Mesh.ListSimple()` method in `pkg/mesh/mesh.go`
3. Iterate over `m.Nodes` map, print hostname + MeshIP
4. Sort hostnames alphabetically for consistent output

### Part 2: Decentralized Mode Implementation

**Challenge:** DHT mode doesn't collect hostnames. Need to extend the protocol.

**Solution - Extend Peer Announcement:**

1. **Add Hostname field to `PeerAnnouncement`** (`pkg/crypto/envelope.go`):
   ```go
   type PeerAnnouncement struct {
       WGPubKey         string
       MeshIP           string
       WGEndpoint       string
       RoutableNetworks []string
       KnownPeers       []KnownPeer
       Timestamp        int64
       Hostname         string  // NEW: optional hostname/FQDN
   }
   ```

2. **Add Hostname field to `PeerInfo`** (`pkg/daemon/peerstore.go`):
   ```go
   type PeerInfo struct {
       WGPubKey         string
       MeshIP           string
       Endpoint         string
       RoutableNetworks []string
       LastSeen         time.Time
       DiscoveredVia    []string
       Latency          *time.Duration
       Hostname         string  // NEW: hostname from peer announcement
   }
   ```

3. **Collect local hostname in daemon** (`pkg/daemon/daemon.go`):
   - Call `os.Hostname()` during daemon initialization
   - Store in `LocalNode` struct (add Hostname field)
   - Include in all outgoing peer announcements

4. **Update announcement handlers** to extract and store hostname:
   - `pkg/discovery/gossip.go` - gossip protocol handler
   - `pkg/discovery/lan.go` - LAN multicast handler
   - `pkg/discovery/dht.go` - DHT announcement handler
   - When processing announcements, store hostname in PeerStore

5. **Extend status command** to list peers:
   ```bash
   ./wgmesh status --secret <SECRET> --list-simple
   # Output:
   node1 10.99.1.5
   node2 10.99.2.10
   # (or if hostname not available)
   abc123de... 10.99.1.5
   ```

6. **Add `ListSimple()` method to Daemon**:
   - Get active peers from PeerStore
   - Print hostname (or pubkey prefix if no hostname) + MeshIP
   - Sort by MeshIP for consistent output

### Part 3: CLI Integration

**Centralized mode:**
```bash
./wgmesh -list-simple              # reads mesh-state.json
```

**Decentralized mode:**
```bash
./wgmesh status --secret <SECRET> --list-simple   # queries running daemon
```

Or add a new subcommand:
```bash
./wgmesh peers --secret <SECRET>   # list active peers in simple format
```

### Backward Compatibility

- Keep existing `-list` flag unchanged (detailed output)
- Keep existing `status` command output unchanged (add optional flag)
- Hostname field in announcements is optional (for compatibility with older nodes)
- If hostname not available, fall back to showing pubkey prefix

### Implementation Phases

**Phase 1 - Centralized mode (simpler, can implement first):**
- Add `-list-simple` flag
- Add `Mesh.ListSimple()` method
- Update documentation

**Phase 2 - Decentralized mode (requires protocol extension):**
- Extend PeerAnnouncement with Hostname field
- Update daemon to collect and share hostname
- Update all discovery layers to handle hostname
- Add list capability to status/peers command
- Update documentation

This phased approach allows partial delivery if time-constrained.

## Affected Files

### Phase 1: Centralized Mode (Lower Complexity)

1. **`main.go`**:
   - Line ~67: Add new flag definition for `-list-simple`
   - Line ~139: Add case handling for the new flag

2. **`pkg/mesh/mesh.go`**:
   - After line 169: Add new `ListSimple()` method (~15 lines)
   - Method should iterate over nodes and print hostname + MeshIP

3. **`README.md`**:
   - Section on centralized mode (around line 180): Add examples for `-list-simple`
   - Add use case showing how to find a specific node's IP

### Phase 2: Decentralized Mode (Higher Complexity)

4. **`pkg/crypto/envelope.go`**:
   - Line ~23-31: Add `Hostname string` field to `PeerAnnouncement` struct
   - Update JSON serialization (Go handles this automatically)

5. **`pkg/daemon/peerstore.go`**:
   - Line ~14-22: Add `Hostname string` field to `PeerInfo` struct
   - Update `AddOrUpdate()` method to accept hostname parameter

6. **`pkg/daemon/daemon.go`**:
   - Line ~44-50: Add `Hostname string` field to `LocalNode` struct
   - In `NewDaemon()`: Call `os.Hostname()` to get local hostname
   - Update announcement creation to include hostname
   - Add new `ListSimple()` method to output peers in simple format

7. **`pkg/discovery/gossip.go`**:
   - Update announcement handling to extract hostname from PeerAnnouncement
   - Pass hostname to PeerStore when adding peers

8. **`pkg/discovery/lan.go`**:
   - Update LAN discovery announcement handling to extract hostname
   - Pass hostname to PeerStore when adding peers

9. **`pkg/discovery/dht.go`**:
   - Update DHT announcement handling to extract hostname
   - Pass hostname to PeerStore when adding peers

10. **`main.go`** (for decentralized mode):
    - Lines ~373-413: Extend `statusCmd()` to support `--list-simple` flag
    - Or add new `peersCmd()` subcommand for listing peers
    - Connect to running daemon via PeerStore or add RPC mechanism

11. **`README.md`**:
    - Add examples for listing peers in decentralized mode
    - Document that hostname sharing is automatic via DHT

## Test Strategy

### Phase 1: Centralized Mode Testing

#### Unit Testing
1. **Test `ListSimple()` method**:
   - Create test mesh with multiple nodes
   - Call `ListSimple()` and capture stdout
   - Verify output format: `hostname IP\n` for each node
   - Verify sorting (if implemented)

2. **Integration test**:
   - Create mesh state file with known nodes
   - Run `./wgmesh -list-simple`
   - Parse output and verify all nodes present
   - Verify IP addresses match expected values

#### Manual Testing
1. **Create test mesh**:
   ```bash
   ./wgmesh -init -network 10.99.0.0/16
   ./wgmesh -add node1:10.99.0.1:192.168.1.10
   ./wgmesh -add node2:10.99.0.2:192.168.1.11
   ./wgmesh -add node3:10.99.0.3:192.168.1.12
   ```

2. **Verify new command**:
   ```bash
   ./wgmesh -list-simple
   # Expected output:
   # node1 10.99.0.1
   # node2 10.99.0.2
   # node3 10.99.0.3
   ```

3. **Compare with existing command**:
   ```bash
   ./wgmesh -list  # Should show detailed output unchanged
   ```

4. **Test with encryption**:
   ```bash
   ./wgmesh --encrypt -list-simple
   # Should prompt for password and work correctly
   ```

5. **Test scripting use case**:
   ```bash
   # Find specific node's IP
   ./wgmesh -list-simple | grep node2
   
   # Extract just IPs
   ./wgmesh -list-simple | awk '{print $2}'
   ```

#### Edge Cases
- Empty mesh (no nodes added)
- Single node mesh
- Mesh with special characters in hostname
- Very large mesh (100+ nodes) - verify performance

#### Backward Compatibility
- Verify existing `-list` flag still works unchanged
- Verify all other flags continue to work
- Test with encrypted state files

### Phase 2: Decentralized Mode Testing

#### Unit Testing
1. **Test PeerAnnouncement with hostname**:
   - Serialize announcement with hostname to JSON
   - Deserialize and verify hostname field preserved
   - Test with empty/missing hostname (backward compatibility)

2. **Test PeerStore hostname handling**:
   - Add peer with hostname
   - Retrieve peer and verify hostname stored
   - Update peer with different hostname
   - Test with missing hostname (should handle gracefully)

3. **Test Daemon.ListSimple()**:
   - Create daemon with mock PeerStore
   - Add peers with hostnames
   - Call ListSimple() and verify output format
   - Test with mix of peers with/without hostnames

#### Integration Testing
1. **Test hostname collection**:
   ```bash
   ./wgmesh join --secret test123
   # Verify daemon collects hostname via os.Hostname()
   # Verify hostname included in announcements
   ```

2. **Test multi-node DHT mesh**:
   - Start 3 nodes with same secret on different machines/containers
   - Wait for peer discovery
   - Run list command on each node
   - Verify each node sees other nodes' hostnames + IPs

3. **Test hostname propagation**:
   - Start node A with hostname "nodeA"
   - Start node B with hostname "nodeB"
   - Verify they discover each other
   - Check that node A's PeerStore shows "nodeB"
   - Check that node B's PeerStore shows "nodeA"

4. **Test backward compatibility**:
   - Run old node (without hostname support)
   - Run new node (with hostname support)
   - Verify they can still communicate
   - Verify new node handles missing hostname gracefully

#### Manual Testing
1. **List peers in decentralized mode**:
   ```bash
   ./wgmesh status --secret test123 --list-simple
   # Expected output:
   # host1 10.99.1.5
   # host2 10.99.2.10
   # host3 10.99.3.15
   ```

2. **Test with systemd service**:
   ```bash
   ./wgmesh install-service --secret test123
   systemctl start wgmesh
   ./wgmesh status --secret test123 --list-simple
   ```

3. **Test discovery mechanisms**:
   - Verify hostname shared via gossip protocol
   - Verify hostname shared via LAN multicast
   - Verify hostname shared via DHT
   - Check that DiscoveredVia still works correctly

#### Edge Cases
- Node with no hostname (os.Hostname() fails)
- Node with FQDN vs simple hostname
- Node with special characters in hostname
- Very long hostnames (>255 chars)
- Hostname changes during runtime (rare but possible)
- Network partitions and rejoins

#### Performance Testing
- Large mesh (50+ nodes) with hostname sharing
- Measure announcement size increase (hostname adds ~20-50 bytes)
- Verify no significant performance degradation
- Check memory usage with many peers

### Cross-Mode Testing
- Verify centralized and decentralized modes work independently
- Test documentation examples for both modes
- Ensure CLI help text is clear about mode differences

## Estimated Complexity

### Overall: **medium** (4-6 hours for both phases)

### Phase 1: Centralized Mode - **low** (1-2 hours)

**Justification:**
- Very small code change: one new flag, one new method (~20 lines total)
- Simple functionality: iterate map and print formatted output
- No complex logic or error handling needed
- Existing infrastructure (flag parsing, mesh loading) already in place
- Straightforward testing
- Minimal documentation updates

**Breakdown:**
- Implementation: 30 minutes
- Testing: 30 minutes
- Documentation: 15 minutes
- Review and refinement: 15 minutes

### Phase 2: Decentralized Mode - **medium** (3-4 hours)

**Justification:**
- Protocol extension required (add Hostname field to PeerAnnouncement)
- Multiple discovery layers need updates (gossip, LAN, DHT)
- Need to handle backward compatibility with nodes not sending hostname
- More complex testing (multi-node, network scenarios)
- Additional error handling for missing/invalid hostnames
- Need to coordinate between daemon, peerstore, and discovery layers

**Breakdown:**
- Protocol extension (envelope.go): 20 minutes
- Daemon changes (collect hostname, update LocalNode): 30 minutes
- PeerStore updates: 20 minutes
- Discovery layer updates (3 files): 60 minutes
- CLI integration (status command extension): 30 minutes
- Unit tests: 45 minutes
- Integration tests: 45 minutes
- Documentation: 20 minutes
- Review and refinement: 30 minutes

### Implementation Recommendation

**Suggested approach:**
1. Implement Phase 1 first (centralized mode) - delivers immediate value
2. Review and merge Phase 1
3. Implement Phase 2 (decentralized mode) - adds DHT support
4. Review and merge Phase 2

This allows incremental delivery and reduces risk. Users of centralized mode get the feature sooner, while decentralized mode users wait for the more complex protocol extension.

### Risk Factors

**Low risk (Phase 1):**
- No protocol changes
- No backward compatibility concerns
- Isolated changes to centralized mode only

**Medium risk (Phase 2):**
- Protocol change requires careful handling for backward compatibility
- Need to ensure old and new versions can coexist in same mesh
- Multiple discovery layers must be updated consistently
- More testing scenarios (distributed, multi-node)

### Alternative: DHT-Only Without Hostname Collection

If collecting hostnames in DHT proves too complex, an alternative is to:
- Display pubkey prefix instead: `abc123de... 10.99.1.5`
- Document that centralized mode shows hostnames, DHT shows pubkeys
- Users can maintain their own hostname mapping if needed

This reduces Phase 2 from medium to low complexity (1-2 hours) but provides less user value.
