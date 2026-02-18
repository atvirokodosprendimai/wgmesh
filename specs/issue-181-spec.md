# Specification: Issue #181

## Classification
feature

## Deliverables
code

## Problem Analysis

Currently, the daemon's periodic status output (decentralized mode) and the `wgmesh peers list` RPC command output are not very informative. 

### Current Status Output (Decentralized Mode)

In `pkg/daemon/daemon.go` (lines 1324-1347), the daemon prints:
```
[Status] Active peers: 7
  - hostname (10.42.0.2) route=direct via [lan,dht] endpoint=192.168.1.5:51820
  - abc123... (10.42.0.3) route=relay:node1 via [gossip] endpoint=1.2.3.4:51820
```

While this shows some useful information, it lacks:
- **Public key** (truncated format only shown when hostname is missing)
- **External IP** (only the endpoint is shown, which may be a local IP)
- **Structured table format** (easier to scan and parse)

### Current RPC Response (`peers.list`)

The RPC `peers.list` command (via `wgmesh peers list`) displays a table format in `main.go` (lines 859-898):
```
PUBLIC KEY                               MESH IP         ENDPOINT                  LAST SEEN  DISCOVERED VIA
--------------------------------------------------------------------------------------------------------------------
abc123def456...                          10.42.0.2       192.168.1.5:51820        2m ago     lan,dht
```

This is better formatted but still missing:
- **Hostname** (not included in RPC PeerInfo struct in `pkg/rpc/protocol.go:40-48`)
- **External IP** distinction from endpoint

### Issue Request

The user wants a more informative table format that includes:
1. **Hostname** - human-readable identifier
2. **Public key** - WireGuard public key (maybe truncated)
3. **VPN IP** (Mesh IP) - the IP within the mesh network
4. **External IP** - the public endpoint
5. **Discovery info** - how the peer was discovered

This should be:
- Displayed in the periodic daemon status output (every X time)
- Returned in the RPC response for `peers.list`

## Proposed Approach

### Phase 1: Enhance RPC PeerInfo Structure

Modify `pkg/rpc/protocol.go` to include missing fields in `PeerInfo`:

```go
type PeerInfo struct {
    PubKey           string   `json:"pubkey"`
    Hostname         string   `json:"hostname,omitempty"`        // NEW
    MeshIP           string   `json:"mesh_ip"`
    Endpoint         string   `json:"endpoint"`
    LastSeen         string   `json:"last_seen"`
    DiscoveredVia    []string `json:"discovered_via"`
    RoutableNetworks []string `json:"routable_networks,omitempty"`
}
```

Update `pkg/rpc/server.go` (lines 261-281) to populate the `Hostname` field from `peer.Hostname`.

### Phase 2: Enhance CLI Table Output

Modify `main.go` `handlePeersList()` function (lines 835-899) to:

1. **Add hostname column** to the table format
2. **Reorder columns** for better readability:
   ```
   HOSTNAME         PUBLIC KEY (truncated)    MESH IP         ENDPOINT            LAST SEEN   DISCOVERED VIA
   ```
3. **Improve formatting**:
   - Keep public key truncated (first 8-12 chars + "...")
   - Add hostname with fallback to pubkey if hostname is empty
   - Maintain endpoint display as-is (external IP:port)

Example output:
```
HOSTNAME         PUBKEY           MESH IP         ENDPOINT                  LAST SEEN   DISCOVERED VIA
----------------------------------------------------------------------------------------------------------
node-1           abc123de...      10.42.0.2       192.168.1.5:51820        2m ago      lan,dht
server-prod      def456ab...      10.42.0.3       203.0.113.10:51820       5m ago      gossip
(unknown)        789xyz01...      10.42.0.4       198.51.100.20:51820      1h ago      dht
```

### Phase 3: Enhance Daemon Status Output

Modify `pkg/daemon/daemon.go` `printStatus()` function (lines 1313-1348) to print a structured table instead of the current log format:

1. **Add table header** before the peer loop
2. **Format each peer as a table row** with consistent column widths
3. **Include all requested fields**: hostname, pubkey (truncated), mesh IP, endpoint, discovery methods

Example output:
```
[Status] Active peers: 7

HOSTNAME         PUBKEY           MESH IP         ENDPOINT                  ROUTE          DISCOVERED VIA
----------------------------------------------------------------------------------------------------------
node-1           abc123de...      10.42.0.2       192.168.1.5:51820        direct-lan     lan,dht
server-prod      def456ab...      10.42.0.3       203.0.113.10:51820       relay:node-1   gossip
(unknown)        789xyz01...      10.42.0.4       198.51.100.20:51820      direct         dht
```

### Implementation Details

#### Truncation Strategy
- Public keys: Show first 8 characters + "..." (total 11 chars)
- Hostname fallback: Use "(unknown)" or truncated pubkey if hostname is empty
- Column widths: Use fixed widths with `Printf` format strings

#### Backward Compatibility
- RPC: Adding `hostname` field to JSON response is backward compatible (clients ignoring unknown fields will continue to work)
- Logging: Changing log format may affect log parsers, but since this is a status display (not structured logging), impact should be minimal

## Affected Files

### Code Changes

1. **`pkg/rpc/protocol.go`** (line 41):
   - Add `Hostname string` field to `PeerInfo` struct

2. **`pkg/rpc/server.go`** (lines 270-277):
   - Populate `Hostname` field in `handlePeersList()` when creating PeerInfo

3. **`main.go`** (lines 859-898):
   - Reorder and enhance table columns in `handlePeersList()`
   - Add hostname column with fallback logic
   - Adjust column widths and headers

4. **`pkg/daemon/daemon.go`** (lines 1324-1347):
   - Replace log-based output with table format in `printStatus()`
   - Add table header
   - Format peer rows with consistent columns

### No Test Changes Required

The existing test infrastructure:
- `pkg/rpc/integration_test.go` tests RPC `peers.list` functionality
- Tests verify response structure but don't enforce specific fields
- Adding `hostname` field to JSON response is backward compatible
- Table formatting is presentation layer and doesn't require unit tests

Manual testing will verify the enhanced output format.

## Test Strategy

### Manual Testing - RPC Enhancement

1. **Start daemon** in decentralized mode:
   ```bash
   sudo wgmesh join --secret "test-secret-123"
   ```

2. **Query peers via RPC**:
   ```bash
   wgmesh peers list
   ```
   
3. **Verify output**:
   - Hostname column is present
   - Hostnames are displayed when available
   - Fallback to truncated pubkey when hostname is missing
   - Table is properly aligned
   - All existing columns (mesh IP, endpoint, discovered via) still present

### Manual Testing - Daemon Status Output

1. **Start daemon with debug logging**:
   ```bash
   sudo wgmesh join --secret "test-secret-123"
   ```

2. **Wait for status output** (daemon prints status periodically, typically every 30-60 seconds)

3. **Verify output**:
   - Table header is printed
   - Each peer is a properly formatted table row
   - All requested columns are present
   - Column alignment is consistent
   - "ROUTE" column shows direct/direct-lan/relay info
   - Discovery methods are displayed

### Testing with Different Scenarios

1. **Empty peer list**: Verify graceful handling when no peers are active
2. **Mix of hostname/no-hostname**: Verify fallback logic works correctly
3. **Long hostnames**: Verify truncation or wrapping doesn't break table alignment
4. **Various discovery methods**: Verify comma-separated display of multiple discovery layers

## Estimated Complexity

**low** (1-2 hours)

### Rationale

- **Straightforward changes**: Mostly formatting and presentation logic
- **No protocol changes**: Just adding an optional field to existing RPC response
- **No new functionality**: All data already exists in `PeerInfo` struct
- **Low risk**: Changes are isolated to output formatting
- **No external dependencies**: Uses only standard library `fmt` package
- **No breaking changes**: Backward compatible additions only

### Time Breakdown

- RPC protocol enhancement: 10 minutes
- RPC server modification: 10 minutes
- CLI table output enhancement: 20-30 minutes
- Daemon status table formatting: 20-30 minutes
- Manual testing and verification: 20-30 minutes
- Total: ~80-110 minutes
