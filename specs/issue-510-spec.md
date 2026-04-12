# Specification: Issue #510

## Classification
feature

## Deliverables
code

## Problem Analysis

`wgmesh status --secret <SECRET>` currently prints a fixed human-readable block:

```
Mesh Status
===========
Interface: wg0
Network ID: deadbeefcafebabe
Mesh Subnet: 10.47.0.0/16
Mesh IPv6 Prefix: fd47::/64
Gossip Port: 51821
Rendezvous ID: aabbccdd

Service Status: active (running)

(Run 'wg show' to see connected peers)
```

There is no machine-readable output option, making it impossible to integrate with monitoring
tools or automation scripts without fragile text parsing.

The fix is to add `--json` and `--pretty` flags to `statusCmd()` in `main.go` that emit a
structured JSON document containing all data the text path already shows, plus peer data
fetched from the running daemon via the existing RPC socket, plus per-peer WireGuard
handshake and transfer statistics from `wg show`.

The default text output must remain unchanged. Backward compatibility is a hard requirement.

## Implementation Tasks

### Task 1: Add `StatusOutput` struct to `main.go`

Insert the following struct definitions **before** the `statusCmd()` function in `main.go`
(around line 520). These are used only for JSON serialization; they are unexported helpers
inside `package main`.

```go
// statusPeerOutput is the JSON representation of a single peer in the status output.
type statusPeerOutput struct {
	PubKey           string   `json:"pubkey"`
	Hostname         string   `json:"hostname,omitempty"`
	MeshIP           string   `json:"mesh_ip"`
	Endpoint         string   `json:"endpoint"`
	LastSeen         string   `json:"last_seen"`          // RFC 3339
	DiscoveredVia    []string `json:"discovered_via"`
	RoutableNetworks []string `json:"routable_networks,omitempty"`
	LatencyMs        *float64 `json:"latency_ms,omitempty"`
	LastHandshake    *string  `json:"last_handshake,omitempty"` // RFC 3339; nil if no handshake yet
	RxBytes          *uint64  `json:"rx_bytes,omitempty"`
	TxBytes          *uint64  `json:"tx_bytes,omitempty"`
}

// statusOutput is the top-level JSON document emitted by `wgmesh status --json`.
type statusOutput struct {
	Interface    string             `json:"interface"`
	NetworkID    string             `json:"network_id"`    // first 8 bytes, hex-encoded
	MeshSubnet   string             `json:"mesh_subnet"`
	MeshIPv6     string             `json:"mesh_ipv6_prefix"`
	GossipPort   uint16             `json:"gossip_port"`
	RendezvousID string             `json:"rendezvous_id"` // hex-encoded
	ServiceStatus string            `json:"service_status"` // empty string when daemon is not running as a service
	DaemonMeshIP string             `json:"daemon_mesh_ip,omitempty"` // local node mesh IP from RPC; omitted when daemon not reachable
	DaemonPubKey string             `json:"daemon_pubkey,omitempty"`  // local node WG public key from RPC
	UptimeSeconds float64           `json:"uptime_seconds,omitempty"` // daemon uptime in seconds; 0 when daemon not reachable
	ActivePeers  int                `json:"active_peers"`
	TotalPeers   int                `json:"total_peers"`
	Peers        []statusPeerOutput `json:"peers"`
}
```

### Task 2: Modify `statusCmd()` in `main.go` to accept `--json` and `--pretty` flags

Replace the existing `statusCmd()` function (lines 521–567 in `main.go`) with the following
implementation. Key behavioral rules:

- `--json` alone: emit compact (single-line) JSON to stdout, then exit 0.
- `--json --pretty`: emit pretty-printed JSON (two-space indent) to stdout, then exit 0.
- `--pretty` without `--json`: ignored (no effect, text output as usual).
- Without `--json`: existing text output path runs unchanged.
- All JSON errors on `json.Marshal` / `json.MarshalIndent` must be handled with
  `fmt.Fprintf(os.Stderr, ...)` + `os.Exit(1)`.

```go
func statusCmd() {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	secret    := fs.String("secret", "", "Mesh secret (required)")
	iface     := fs.String("interface", "", "WireGuard interface name (default: wg0 on non-macOS, utun20 on macOS)")
	meshSubnet := fs.String("mesh-subnet", "", "Custom mesh subnet CIDR (e.g. 192.168.100.0/24)")
	jsonOut   := fs.Bool("json", false, "Output status as JSON")
	pretty    := fs.Bool("pretty", false, "Pretty-print JSON output (requires --json)")
	fs.Parse(os.Args[2:])

	if *secret == "" {
		fmt.Fprintln(os.Stderr, "Error: --secret is required")
		fmt.Fprintln(os.Stderr, "Usage: wgmesh status --secret <SECRET>")
		os.Exit(1)
	}

	cfg, err := daemon.NewConfig(daemon.DaemonOpts{
		Secret:        *secret,
		InterfaceName: *iface,
		MeshSubnet:    *meshSubnet,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create config: %v\n", err)
		os.Exit(1)
	}

	if !*jsonOut {
		// ── existing text path (unchanged) ──────────────────────────────────
		fmt.Printf("Mesh Status\n")
		fmt.Printf("===========\n")
		fmt.Printf("Interface: %s\n", cfg.InterfaceName)
		fmt.Printf("Network ID: %x\n", cfg.Keys.NetworkID[:8])
		if cfg.CustomSubnet != nil {
			fmt.Printf("Mesh Subnet: %s (custom)\n", cfg.CustomSubnet)
		} else {
			fmt.Printf("Mesh Subnet: 10.%d.0.0/16\n", cfg.Keys.MeshSubnet[0])
		}
		fmt.Printf("Mesh IPv6 Prefix: %s\n", formatIPv6Prefix(cfg.Keys.MeshPrefixV6))
		fmt.Printf("Gossip Port: %d\n", cfg.Keys.GossipPort)
		fmt.Printf("Rendezvous ID: %x\n", cfg.Keys.RendezvousID)
		fmt.Println()

		svcStatus, svcErr := daemon.ServiceStatus()
		if svcErr == nil {
			fmt.Printf("Service Status: %s\n", svcStatus)
		}

		fmt.Println()
		fmt.Println("(Run 'wg show' to see connected peers)")
		return
	}

	// ── JSON path ────────────────────────────────────────────────────────────
	out := buildStatusOutput(cfg)

	var data []byte
	if *pretty {
		data, err = json.MarshalIndent(out, "", "  ")
	} else {
		data, err = json.Marshal(out)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}
```

### Task 3: Add `buildStatusOutput()` helper to `main.go`

Add the following helper function immediately after the new `statusCmd()` function. It
assembles the `statusOutput` value by combining data from three sources:

1. **Config keys** – always available (derived from secret, no network required).
2. **RPC daemon** – optional; silently skipped when the daemon is not running.
3. **WireGuard kernel interface** – optional; silently skipped when `wg` is unavailable or
   the interface does not yet exist.

```go
// buildStatusOutput assembles the JSON status document from config, RPC, and wg stats.
func buildStatusOutput(cfg *daemon.Config) statusOutput {
	// 1. Static fields derived from the secret / config.
	meshSubnet := fmt.Sprintf("10.%d.0.0/16", cfg.Keys.MeshSubnet[0])
	if cfg.CustomSubnet != nil {
		meshSubnet = cfg.CustomSubnet.String()
	}

	out := statusOutput{
		Interface:    cfg.InterfaceName,
		NetworkID:    fmt.Sprintf("%x", cfg.Keys.NetworkID[:8]),
		MeshSubnet:   meshSubnet,
		MeshIPv6:     formatIPv6Prefix(cfg.Keys.MeshPrefixV6),
		GossipPort:   cfg.Keys.GossipPort,
		RendezvousID: fmt.Sprintf("%x", cfg.Keys.RendezvousID),
		Peers:        []statusPeerOutput{},
	}

	// 2. Service status (systemd / launchd) — best-effort.
	if svc, err := daemon.ServiceStatus(); err == nil {
		out.ServiceStatus = svc
	}

	// 3. Daemon RPC — best-effort. Connect to the running daemon socket.
	socketPath := getRPCSocketPath()
	client, rpcErr := rpc.NewClient(socketPath)
	if rpcErr == nil {
		defer client.Close()

		// daemon.status → local node info + uptime.
		if result, err := client.Call("daemon.status", nil); err == nil {
			if m, ok := result.(map[string]interface{}); ok {
				if v, ok := m["mesh_ip"].(string); ok {
					out.DaemonMeshIP = v
				}
				if v, ok := m["pubkey"].(string); ok {
					out.DaemonPubKey = v
				}
				if v, ok := m["uptime"].(float64); ok {
					// uptime comes back as nanoseconds (time.Duration serialized)
					out.UptimeSeconds = v / 1e9
				}
			}
		}

		// peers.list → peer list.
		if result, err := client.Call("peers.list", nil); err == nil {
			if m, ok := result.(map[string]interface{}); ok {
				if peersRaw, ok := m["peers"].([]interface{}); ok {
					for _, pRaw := range peersRaw {
						p, ok := pRaw.(map[string]interface{})
						if !ok {
							continue
						}
						sp := statusPeerOutput{
							DiscoveredVia: []string{},
						}
						if v, ok := p["pubkey"].(string); ok {
							sp.PubKey = v
						}
						if v, ok := p["hostname"].(string); ok {
							sp.Hostname = v
						}
						if v, ok := p["mesh_ip"].(string); ok {
							sp.MeshIP = v
						}
						if v, ok := p["endpoint"].(string); ok {
							sp.Endpoint = v
						}
						if v, ok := p["last_seen"].(string); ok {
							sp.LastSeen = v
						}
						if v, ok := p["latency_ms"].(float64); ok {
							sp.LatencyMs = &v
						}
						if vs, ok := p["discovered_via"].([]interface{}); ok {
							for _, s := range vs {
								if str, ok := s.(string); ok {
									sp.DiscoveredVia = append(sp.DiscoveredVia, str)
								}
							}
						}
						if vs, ok := p["routable_networks"].([]interface{}); ok {
							for _, s := range vs {
								if str, ok := s.(string); ok {
									sp.RoutableNetworks = append(sp.RoutableNetworks, str)
								}
							}
						}
						out.Peers = append(out.Peers, sp)
					}
				}
			}
		}

		// peers.count → active/total counters.
		if result, err := client.Call("peers.count", nil); err == nil {
			if m, ok := result.(map[string]interface{}); ok {
				if v, ok := m["active"].(float64); ok {
					out.ActivePeers = int(v)
				}
				if v, ok := m["total"].(float64); ok {
					out.TotalPeers = int(v)
				}
			}
		}
	}

	// 4. WireGuard per-peer stats (handshakes + transfer) — best-effort.
	//    Enrich the peers already collected from the RPC list.
	handshakes, _ := wireguard.GetLatestHandshakes(cfg.InterfaceName)
	transfers, _  := wireguard.GetPeerTransfers(cfg.InterfaceName)

	for i := range out.Peers {
		pubkey := out.Peers[i].PubKey
		if ts, ok := handshakes[pubkey]; ok && ts > 0 {
			s := time.Unix(ts, 0).UTC().Format(time.RFC3339)
			out.Peers[i].LastHandshake = &s
		}
		if t, ok := transfers[pubkey]; ok {
			rx := t.RxBytes
			tx := t.TxBytes
			out.Peers[i].RxBytes = &rx
			out.Peers[i].TxBytes = &tx
		}
	}

	return out
}
```

### Task 4: Add required imports to `main.go`

Ensure the following packages are present in the import block of `main.go`. Several are
already imported; add only those that are missing:

```
"encoding/json"                                           // already present (used by service.go; confirm in main.go)
"time"                                                    // for time.Unix / time.RFC3339

"github.com/atvirokodosprendimai/wgmesh/pkg/rpc"         // already imported
"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"   // add this import
```

`wireguard.GetLatestHandshakes` and `wireguard.GetPeerTransfers` are already defined in
`pkg/wireguard/apply.go` and require no new dependencies.

### Task 5: Add unit tests to `main_test.go`

Append the following test function to `main_test.go`. The test builds the binary exactly
like the existing `TestVersionFlag` test, then invokes `wgmesh status --json` with a
well-known test secret and verifies the JSON output.

```go
func TestStatusJSONOutput(t *testing.T) {
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-test-status", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove("/tmp/wgmesh-test-status")

	testSecret := "wgmesh://v1/dGVzdHNlY3JldGZvcnN0YXR1c2pzb250ZXN0aW5nMTIz"

	tests := []struct {
		name        string
		args        []string
		wantKeys    []string // top-level JSON keys that must be present
		wantCompact bool     // true → output must be a single line
	}{
		{
			name:        "compact json",
			args:        []string{"status", "--secret", testSecret, "--json"},
			wantKeys:    []string{"interface", "network_id", "mesh_subnet", "mesh_ipv6_prefix", "gossip_port", "rendezvous_id", "active_peers", "total_peers", "peers"},
			wantCompact: true,
		},
		{
			name:        "pretty json",
			args:        []string{"status", "--secret", testSecret, "--json", "--pretty"},
			wantKeys:    []string{"interface", "network_id", "mesh_subnet", "mesh_ipv6_prefix", "gossip_port", "rendezvous_id", "active_peers", "total_peers", "peers"},
			wantCompact: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("/tmp/wgmesh-test-status", tt.args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Command failed: %v\nOutput: %s", err, output)
			}

			trimmed := strings.TrimSpace(string(output))

			// Validate JSON parses cleanly.
			var parsed map[string]interface{}
			if jsonErr := json.Unmarshal([]byte(trimmed), &parsed); jsonErr != nil {
				t.Fatalf("Output is not valid JSON: %v\nOutput: %s", jsonErr, trimmed)
			}

			// Check required top-level keys are present.
			for _, key := range tt.wantKeys {
				if _, ok := parsed[key]; !ok {
					t.Errorf("Missing JSON key %q in output: %s", key, trimmed)
				}
			}

			// Compact check: output must be a single line.
			if tt.wantCompact {
				lines := strings.Split(trimmed, "\n")
				if len(lines) != 1 {
					t.Errorf("Expected single-line compact JSON, got %d lines", len(lines))
				}
			}

			// Pretty check: output must have more than one line.
			if !tt.wantCompact {
				lines := strings.Split(trimmed, "\n")
				if len(lines) < 5 {
					t.Errorf("Expected multi-line pretty JSON, got %d lines", len(lines))
				}
			}

			// peers field must be a JSON array (even if empty).
			if peersRaw, ok := parsed["peers"]; ok {
				if _, ok := peersRaw.([]interface{}); !ok {
					t.Errorf("peers field must be a JSON array, got %T", peersRaw)
				}
			}

			// gossip_port must be a number.
			if port, ok := parsed["gossip_port"]; ok {
				if _, ok := port.(float64); !ok {
					t.Errorf("gossip_port must be a number, got %T", port)
				}
			}
		})
	}
}

func TestStatusTextOutputUnchanged(t *testing.T) {
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-test-status-text", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove("/tmp/wgmesh-test-status-text")

	testSecret := "wgmesh://v1/dGVzdHNlY3JldGZvcnN0YXR1c2pzb250ZXN0aW5nMTIz"

	cmd := exec.Command("/tmp/wgmesh-test-status-text", "status", "--secret", testSecret)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	result := string(output)

	// Text output must contain the human-readable header.
	if !strings.Contains(result, "Mesh Status") {
		t.Errorf("Expected text output to contain 'Mesh Status', got: %s", result)
	}
	if !strings.Contains(result, "Interface:") {
		t.Errorf("Expected text output to contain 'Interface:', got: %s", result)
	}
	if !strings.Contains(result, "Network ID:") {
		t.Errorf("Expected text output to contain 'Network ID:', got: %s", result)
	}

	// Text output must NOT start with '{'.
	trimmed := strings.TrimSpace(result)
	if strings.HasPrefix(trimmed, "{") {
		t.Errorf("Text output must not be JSON, got: %s", trimmed)
	}
}
```

The test file already imports `"os"`, `"os/exec"`, `"strings"`, and `"testing"`. Add
`"encoding/json"` to the import block if not already present.

## Affected Files

| File | Change |
|------|--------|
| `main.go` | Add `statusPeerOutput` and `statusOutput` structs; add `--json` and `--pretty` flags to `statusCmd()`; add `buildStatusOutput()` helper; add `wireguard` import |
| `main_test.go` | Add `TestStatusJSONOutput` and `TestStatusTextOutputUnchanged`; add `encoding/json` import |

No changes to `pkg/` packages. No new external dependencies. No changes to `go.mod`.

## Test Strategy

1. `go test ./... -run TestStatus` — runs both new tests; verifies compact JSON, pretty JSON,
   and that text output is unchanged.
2. Manual smoke test (daemon not running):
   ```bash
   go build -o /tmp/wgmesh .
   /tmp/wgmesh status --secret "wgmesh://v1/dGVzdHNlY3JldGZvcnN0YXR1c2pzb250ZXN0aW5nMTIz" --json | python3 -m json.tool
   /tmp/wgmesh status --secret "wgmesh://v1/dGVzdHNlY3JldGZvcnN0YXR1c2pzb250ZXN0aW5nMTIz" --json --pretty
   /tmp/wgmesh status --secret "wgmesh://v1/dGVzdHNlY3JldGZvcnN0YXR1c2pzb250ZXN0aW5nMTIz"
   ```
   - First command: one line, valid JSON, all required keys present, `peers` is `[]`.
   - Second command: multi-line, same keys, same values.
   - Third command: text output identical to pre-change behavior.
3. `go test -race ./...` — verify no race conditions introduced.

## Estimated Complexity
low

**Reasoning:** All required data sources (`daemon.NewConfig`, `daemon.ServiceStatus`,
`rpc.NewClient`, `wireguard.GetLatestHandshakes`, `wireguard.GetPeerTransfers`) already
exist and are already imported or trivially importable. The change is confined to `main.go`
and `main_test.go`. The JSON path is additive; the text path is a copy of the existing code
with a `return` appended. No new packages, no schema migrations, no protocol changes.
Estimated implementation effort: 1–2 hours.
