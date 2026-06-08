# Issue #510: Add `wgmesh status --json` output format for programmatic consumption

## Summary

Add JSON output format support to the `wgmesh status` CLI command via a `--json` flag, enabling programmatic consumption of mesh network status information by monitoring tools, automation scripts, and GUI dashboards. The JSON output will contain the same information currently displayed in human-readable text format, structured as a stable JSON schema.

## Context

The current `wgmesh status` command outputs status information in human-readable text format only:

```bash
$ wgmesh status --secret <SECRET>
Mesh Status
===========
Interface: wg0
Network ID: 3a8f9c2b1e4d
Mesh Subnet: 10.142.0.0/16
Mesh IPv6 Prefix: fdc8:1a2b:3c4d::/64
Gossip Port: 57321
Rendezvous ID: 9e4f2a8b1c3d
Service Status: active

(Run 'wg show' to see connected peers)
```

This format is unsuitable for automated monitoring, alerting systems, and GUI integrations that require structured, machine-readable data. Programmatic consumers need:

1. **Monitoring integration**: Prometheus exporters, health check endpoints
2. **Automation**: CI/CD pipelines testing mesh connectivity
3. **GUI dashboards**: Network management UIs displaying real-time mesh status
4. **Log aggregation**: Structured logging for centralized analysis

The JSON format will provide a stable contract for tooling while maintaining backward compatibility with existing text output.

## Requirements

### 1. CLI Flag Addition

Add a `--json` flag to the `status` subcommand:

```bash
wgmesh status --secret <SECRET> --json
```

- When `--json` is set: output valid JSON to stdout
- When `--json` is NOT set: output current human-readable format (default behavior unchanged)
- Flag should be a boolean switch (no value required)

### 2. JSON Schema Design

The JSON output MUST include all fields currently displayed in text format:

```json
{
  "interface": "wg0",
  "network_id": "3a8f9c2b1e4d",
  "mesh_subnet": "10.142.0.0/16",
  "mesh_subnet_custom": false,
  "mesh_ipv6_prefix": "fdc8:1a2b:3c4d::/64",
  "gossip_port": 57321,
  "rendezvous_id": "9e4f2a8b1c3d",
  "service_status": "active"
}
```

**Field Specifications:**

| Field | Type | Description | Source |
|-------|------|-------------|--------|
| `interface` | string | WireGuard interface name | `cfg.InterfaceName` |
| `network_id` | string | Hex string (16 hex chars, first 8 bytes of NetworkID) | `cfg.Keys.NetworkID[:8]` formatted as `%x` |
| `mesh_subnet` | string | CIDR notation (e.g., "10.142.0.0/16") | Derived from `cfg.Keys.MeshSubnet[0]` or `cfg.CustomSubnet` |
| `mesh_subnet_custom` | boolean | True if user-provided subnet, false if derived | `cfg.CustomSubnet != nil` |
| `mesh_ipv6_prefix` | string | IPv6 ULA prefix in CIDR (e.g., "fdc8:1a2b:3c4d::/64") | `formatIPv6Prefix(cfg.Keys.MeshPrefixV6)` |
| `gossip_port` | number | UDP port for gossip protocol (0-65535) | `cfg.Keys.GossipPort` |
| `rendezvous_id` | string | Hex string (16 hex chars) | `cfg.Keys.RendezvousID` formatted as `%x` |
| `service_status` | string | Service state or "unknown" | `daemon.ServiceStatus()` result |

**Service Status Values:** `"active"`, `"inactive"`, `"unknown"`, or any systemctl is-active output.

### 3. Implementation Details

**Location:** Modify `statusCmd()` function in `main.go` (lines 525-573).

**Changes Required:**

1. Add flag parsing for `--json` boolean flag
2. Derive keys and validate configuration (same as current implementation)
3. Branch output logic based on JSON flag:
   - **JSON path**: Marshal structured data using `encoding/json`, write to stdout
   - **Text path**: Execute existing `fmt.Printf` statements (no changes)
4. Keep error messages on stderr regardless of output format

**Key Constraints:**

- Use standard library `encoding/json` package only (no external JSON dependencies)
- Ensure JSON output is always valid and parseable even on partial errors
- Maintain backward compatibility: default behavior must remain text output
- Hex strings MUST be lowercase (consistent with `%x` format specifier)

### 4. Error Handling

JSON output mode must handle errors gracefully:

1. **Configuration errors** (e.g., invalid secret): Output error message to stderr, exit with code 1 (same as current behavior)
2. **JSON marshaling errors**: Should never occur with static schema; if they do, log to stderr and exit 1
3. **Service status failures**: If `daemon.ServiceStatus()` returns error, set `service_status` to `"unknown"` (do not fail entire command)

## Acceptance Criteria

### 1. Command-Line Interface

- [ ] `wgmesh status --secret <SECRET> --json` produces valid JSON output
- [ ] `wgmesh status --secret <SECRET>` (without `--json`) produces text output unchanged
- [ ] `--json` flag is optional and defaults to `false`
- [ ] Flag appears in help output when available

### 2. JSON Schema Validation

- [ ] JSON output validates against the specified schema
- [ ] All required fields are present in all successful executions
- [ ] Hex strings are lowercase and properly formatted
- [ ] CIDR notation is valid for subnet fields
- [ ] Port numbers are within valid range (0-65535)

### 3. Field Content Verification

- [ ] `interface` matches the actual WireGuard interface name
- [ ] `network_id` is hex string of first 8 bytes of NetworkID
- [ ] `mesh_subnet` shows derived subnet OR custom subnet correctly
- [ ] `mesh_subnet_custom` is `true` when custom subnet used, `false` otherwise
- [ ] `mesh_ipv6_prefix` is properly formatted IPv6 CIDR
- [ ] `gossip_port` is integer between 0-65535
- [ ] `rendezvous_id` is hex string of RendezvousID
- [ ] `service_status` reflects actual systemctl state or "unknown"

### 4. Backward Compatibility

- [ ] Existing scripts using text output continue to work unchanged
- [ ] Error messages and exit codes remain consistent
- [ ] No breaking changes to command-line arguments or flag behavior

### 5. Test Coverage

- [ ] Unit test for JSON marshaling with mock configuration
- [ ] Unit test verifying all fields are correctly populated
- [ ] Integration test comparing text vs JSON output for same secret
- [ ] Test error handling paths (invalid secret, systemctl failures)

### 6. Edge Cases

- [ ] JSON output works correctly on macOS (utun interfaces) and Linux (wg interfaces)
- [ ] Custom mesh subnet is correctly flagged in JSON output
- [ ] Service status "unknown" is emitted when systemctl command fails
- [ ] Hex formatting is consistent across different network configurations

## Out of Scope

The following features are explicitly **out of scope** for this specification:

1. **Peer Information**: Connected peers are NOT included (refer to `wg show` command, consider future peer query via RPC)
2. **WireGuard Status**: WireGuard interface statistics, handshakes, transfer bytes (use `wg show` for this)
3. **Pretty-Printing**: No JSON pretty-print option (standard compact JSON only)
4. **Output Redirection**: No support for writing to files (use shell redirection: `> status.json`)
5. **Schema Versioning**: No `$schema` or version field in JSON (contract defined by code)
6. **Additional Formats**: No YAML, TOML, or other output formats
7. **Live Monitoring**: No streaming/watch mode (single snapshot only)
8. **Performance Metrics**: No CPU, memory, or network statistics

### Future Enhancements (Not Part of This Issue)

Potential future improvements that should NOT be implemented now:
- Adding `--pretty` flag for indented JSON output
- Including WireGuard peer information from RPC query
- Adding uptime, health check results, or daemon version
- Filtering specific fields with `--fields` flag
- Schema versioning and backward compatibility guarantees
