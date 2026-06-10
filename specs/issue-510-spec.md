# Issue #510: Add `wgmesh status --json` output format for programmatic consumption

## Classification
feature

## Problem Analysis

The `wgmesh status --secret <SECRET>` command currently outputs status information in a human-readable text format only. For automation, monitoring integrations, and programmatic consumption, JSON output is needed.

Current state:
- `statusCmd()` in `main.go` (lines 541-620) already has a `--json` flag defined
- The `StatusOutput` struct already exists with proper JSON tags
- The JSON encoding logic is already implemented (lines 595-600)
- However, this implementation was incomplete or removed - the flag exists but may not be properly wired

The existing `StatusOutput` struct contains:
- `Interface`: WireGuard interface name
- `NetworkID`: First 8 bytes of network ID
- `MeshSubnet`: IPv4 mesh subnet CIDR
- `MeshIPv6Prefix`: IPv6 prefix representation
- `GossipPort`: UDP gossip port
- `RendezvousID`: DHT rendezvous ID
- `ServiceStatus`: Optional systemd service status

Additionally, there's an RPC-based daemon status system (`pkg/rpc/server.go`) that provides richer status information via `daemon.status` RPC call, including:
- `MeshIP`: Assigned mesh IP address
- `PubKey`: WireGuard public key
- `Uptime`: Daemon uptime duration
- `Interface`: WireGuard interface name
- `Version`: wgmesh version

The problem is that users need programmatic access to status information for:
- CI/CD pipeline integration
- Monitoring systems (Prometheus, Nagios, etc.)
- Health check scripts
- Automated deployment verification
- Mesh management tools

## Proposed Approach

### 1. Verify and Complete JSON Output Implementation

Verify that the existing `--json` flag in `statusCmd()` is functional. The code at lines 595-600 should already handle JSON output:

```go
if *jsonOutput {
    enc := json.NewEncoder(os.Stdout)
    enc.SetIndent("", "  ")
    if err := enc.Encode(output); err != nil {
        fmt.Fprintf(os.Stderr, "Failed to encode JSON: %v\n", err)
        os.Exit(1)
    }
}
```

If this is working, no changes are needed to `statusCmd()`. If not, debug why JSON output is not being triggered properly.

### 2. Enhance StatusOutput Struct

Add fields to `StatusOutput` struct for parity with RPC daemon status:

```go
type StatusOutput struct {
    Interface      string        `json:"interface"`
    NetworkID      string        `json:"network_id"`
    MeshSubnet     string        `json:"mesh_subnet"`
    MeshIPv6Prefix string        `json:"mesh_ipv6_prefix"`
    GossipPort     int           `json:"gossip_port"`
    RendezvousID   string        `json:"rendezvous_id"`
    ServiceStatus  string        `json:"service_status,omitempty"`
    
    // New fields for programmatic consumption
    Version        string        `json:"version"`
    MeshIP         string        `json:"mesh_ip,omitempty"`
    PubKey         string        `json:"pubkey,omitempty"`
    Uptime         time.Duration `json:"uptime,omitempty"`
}
```

### 3. Populate Additional Fields

In `statusCmd()`, populate the new fields:

```go
// Add version
output.Version = versionOutput()

// Try to get richer status from daemon RPC if available
if client := getRPCClient(); client != nil {
    status, err := client.DaemonStatus()
    if err == nil && status != nil {
        output.MeshIP = status.MeshIP
        output.PubKey = status.PubKey
        output.Uptime = status.Uptime
        // Override interface with daemon's view if available
        if status.Interface != "" {
            output.Interface = status.Interface
        }
    }
}
```

Note: `getRPCClient()` would be a helper to connect to the daemon socket (similar to existing patterns in `peersCmd()`).

### 4. Update Text Output

Update the text output format to include new fields for consistency:

```go
fmt.Printf("Version: %s\n", output.Version)
if output.MeshIP != "" {
    fmt.Printf("Mesh IP: %s\n", output.MeshIP)
    fmt.Printf("Public Key: %s\n", output.PubKey)
    fmt.Printf("Uptime: %s\n", formatDuration(output.Uptime))
}
```

### 5. Add JSON Schema Documentation

Create inline JSON schema documentation in the struct comments:

```go
// StatusOutput defines the JSON structure for status output.
// When using --json flag, output is a JSON object with these fields.
// All fields are always present in JSON output (optional fields are omitted if empty).
type StatusOutput struct { ... }
```

### 6. Update Help Text

Ensure the `--json` flag usage is documented in help output and CLI documentation.

## Acceptance Criteria

1. **JSON Output Works**: `wgmesh status --secret <SECRET> --json` outputs valid JSON
2. **JSON Structure Valid**: Output matches `StatusOutput` struct with proper field names (snake_case in JSON)
3. **Text Output Preserved**: Default behavior (no `--json` flag) remains unchanged
4. **New Fields Populated**: When daemon is running, JSON includes `mesh_ip`, `pubkey`, `uptime`, and `version`
5. **Graceful Degradation**: When daemon is not running, JSON output still works with derived fields only
6. **Error Handling**: JSON encoding errors are properly reported to stderr with non-zero exit code
7. **Test Coverage**: Add tests for JSON output format validation

Example JSON output:
```json
{
  "interface": "wg0",
  "network_id": "a1b2c3d4e5f6g7h8",
  "mesh_subnet": "10.42.0.0/16",
  "mesh_ipv6_prefix": "fd00:42::/32",
  "gossip_port": 54321,
  "rendezvous_id": "1a2b3c4d",
  "service_status": "active (running)",
  "version": "0.1.0",
  "mesh_ip": "10.42.0.1",
  "pubkey": "abcdef1234567890...",
  "uptime": "2h30m45s"
}
```

## Out of Scope

- Implementing a new RPC protocol or changing existing RPC structures
- Modifying `pkg/rpc/server.go` or `pkg/rpc/protocol.go` (only client-side consumption)
- Adding YAML, TOML, or other output formats
- Creating a separate `status-json` subcommand
- Implementing filtering or querying capabilities (e.g., `--json-paths` for jq-like queries)
- Modifying the daemon's `GetStatus()` implementation
- WebSocket or streaming status updates
- Authentication or authorization changes for status access
