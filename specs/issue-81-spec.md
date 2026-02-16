# Specification: Issue #81

## Classification
feature

## Deliverables
code + documentation

## Problem Analysis

Currently, users need to SSH into each server to find that server's VPN IP address, which is inconvenient and time-consuming, especially for larger mesh networks. There is no simple way to get an overview of all nodes in the mesh with their hostnames and corresponding VPN IPs.

### Current State

The centralized mesh mode already has a `-list` flag that displays detailed node information:
- Mesh network configuration
- Node hostname (with local and NAT markers)
- Mesh IP
- SSH connection details
- Public key
- Public endpoint (if available)
- Routable networks

However, this output is verbose and not optimized for quickly finding a specific node's VPN IP. The request is for a simplified output format showing just hostname + VPN IP pairs.

### User Experience Issue

The feature request specifically mentions:
- Command should be something like `./wgmesh mesh list`
- Output should be simple hostname + IP pairs (e.g., `node1 10.39.0.1`)
- The mesh system should collect server hostname and FQDN on start

### Technical Context

From the codebase analysis:
- **Centralized mode** uses `pkg/mesh` package
- Node data is stored in `Mesh.Nodes` (map of hostname → Node struct)
- Each `Node` struct already contains:
  - `Hostname` field (already collected during node addition)
  - `MeshIP` field (net.IP, the VPN overlay IP)
- The existing `-list` flag calls `Mesh.List()` method in `pkg/mesh/mesh.go`
- Hostname is specified during node addition: `hostname:mesh_ip:ssh_host[:ssh_port]`

## Proposed Approach

The issue mentions `./wgmesh mesh list`, but the current CLI uses flags (e.g., `-list`), not subcommands for centralized mode. There are two possible interpretations:

### Option A: New Simplified List Format (Recommended)
Add a new flag `-list-simple` or `-list-ips` that produces compact hostname + IP output:
```bash
./wgmesh -list-simple
# Output:
node1 10.39.0.1
node2 10.39.55.15
node3 10.39.100.20
```

**Pros:**
- Minimal code changes
- Maintains consistency with existing CLI patterns
- Preserves existing `-list` functionality
- Easy to pipe to grep/awk for scripting

**Cons:**
- Doesn't match the exact syntax requested (`mesh list`)

### Option B: Add "mesh" Subcommand with "list" Sub-subcommand
Restructure CLI to add `mesh` as a subcommand with sub-operations:
```bash
./wgmesh mesh list        # Simple hostname + IP format
./wgmesh mesh list --full # Detailed format (existing behavior)
```

**Pros:**
- Matches the user's requested syntax exactly
- More intuitive grouping of mesh operations
- Allows future expansion (e.g., `mesh status`, `mesh validate`)

**Cons:**
- Larger refactoring needed
- May confuse existing users
- Breaks backward compatibility with `-list` flag

### Recommended Implementation: Option A + Alias

**Approach:**
1. Add a new flag `-list-simple` that outputs hostname + IP pairs only
2. Keep `-list` for detailed output (backward compatible)
3. Update documentation to show both options
4. Optionally: Add a deprecation notice suggesting users try the new format

### Implementation Steps

1. **Add new flag in `main.go`**:
   - Add `listSimple := flag.Bool("list-simple", false, "List nodes in simple hostname IP format")`
   - Add case handling after flag parsing

2. **Add method to Mesh struct** (`pkg/mesh/mesh.go`):
   - Create `func (m *Mesh) ListSimple()` method
   - Iterate over `m.Nodes` map
   - Print each node as: `fmt.Printf("%s %s\n", hostname, node.MeshIP)`
   - Sort hostnames alphabetically for consistent output

3. **Update main.go logic**:
   - Add case to handle `-list-simple` flag
   - Call `m.ListSimple()` method

4. **Documentation updates**:
   - Update README.md with new flag example
   - Add use case examples for quick IP lookup
   - Document that output is suitable for scripting

### Alternative: Enhance Existing `-list` with Format Flag

Instead of a separate flag, add a `-format` flag:
```bash
./wgmesh -list -format simple   # hostname IP
./wgmesh -list -format detailed # existing output (default)
```

This is cleaner but requires more complex flag handling.

## Affected Files

### Code Changes Required

1. **`main.go`**:
   - Line ~67: Add new flag definition for `-list-simple`
   - Line ~139: Add case handling for the new flag

2. **`pkg/mesh/mesh.go`**:
   - After line 169: Add new `ListSimple()` method (~15 lines)
   - Method should iterate over nodes and print hostname + MeshIP

3. **`README.md`**:
   - Section on centralized mode (around line 180): Add examples for `-list-simple`
   - Add use case showing how to find a specific node's IP

## Test Strategy

### Unit Testing
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

### Manual Testing
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

### Edge Cases
- Empty mesh (no nodes added)
- Single node mesh
- Mesh with special characters in hostname
- Very large mesh (100+ nodes) - verify performance

### Backward Compatibility
- Verify existing `-list` flag still works unchanged
- Verify all other flags continue to work
- Test with encrypted state files

## Estimated Complexity

**low** (1-2 hours)

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
