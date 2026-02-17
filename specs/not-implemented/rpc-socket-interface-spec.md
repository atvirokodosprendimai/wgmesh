# Specification: RPC Interface for Querying Running Daemon

## Classification
feature

## Deliverables
code + tests + documentation

## Problem Analysis

When `wgmesh join --secret <key>` is running as a daemon (in decentralized mode), there's currently no way to query it for runtime information such as:
- List of DHT-discovered peers
- Current mesh status
- Active peer connections
- Network statistics

Users need a way to interact with a running daemon without restarting it or accessing internal state files directly.

### Current State

The daemon (`pkg/daemon/daemon.go`) has:
- `PeerStore` with `GetAll()` and `GetActive()` methods that return discovered peers
- DHT discovery layer that populates the peer store
- No external interface for querying this information

### Use Case Example

```bash
# Terminal 1: Start daemon
$ wgmesh join --secret "wgmesh://v1/K7x2..."
Initializing mesh node with DHT discovery...
Mesh IP: 10.42.0.5
[running in background]

# Terminal 2: Query the daemon
$ wgmesh peers list
WGPubKey                         MeshIP        Endpoint           LastSeen    Via
abc123...                        10.42.0.1     1.2.3.4:51820     2s ago      dht,gossip
def456...                        10.42.0.2     5.6.7.8:51820     5s ago      lan,dht
...

$ wgmesh peers count
Active peers: 15
Total discovered: 23
```

## Proposed Approach

Implement a Unix domain socket-based RPC interface at `/var/run/wgmesh.sock` (or `/run/wgmesh.sock`) that allows querying the running daemon.

### Architecture

```
┌─────────────────────────────────────────────────┐
│  wgmesh join (daemon process)                   │
│  ┌───────────────────────────────────────────┐  │
│  │  Daemon                                   │  │
│  │  ├─ PeerStore (discovered peers)          │  │
│  │  ├─ DHT Discovery                         │  │
│  │  └─ RPC Server (Unix socket listener)    │◄─┼─── /var/run/wgmesh.sock
│  └───────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
                                                 ▲
                                                 │
┌────────────────────────────────────────────────┼─┐
│  wgmesh peers list (CLI command)               │ │
│  ┌──────────────────────────────────────────┐  │ │
│  │  RPC Client                               │  │ │
│  │  ├─ Connect to Unix socket               │──┘ │
│  │  ├─ Send request                          │    │
│  │  └─ Format and display response           │    │
│  └──────────────────────────────────────────┘    │
└──────────────────────────────────────────────────┘
```

### Protocol Design

Use a simple line-delimited JSON-RPC protocol for ease of implementation and debugging:

**Request format:**
```json
{"jsonrpc":"2.0","method":"peers.list","params":{},"id":1}
```

**Response format:**
```json
{
  "jsonrpc":"2.0",
  "result":{
    "peers":[
      {
        "pubkey":"abc123...",
        "mesh_ip":"10.42.0.1",
        "endpoint":"1.2.3.4:51820",
        "last_seen":"2024-01-15T10:30:00Z",
        "discovered_via":["dht","gossip"]
      }
    ]
  },
  "id":1
}
```

### Supported RPC Methods

1. **`peers.list`**: Return all active peers
   - Returns: Array of `PeerInfo` objects
   - Filters: Only peers seen within `PeerDeadTimeout`

2. **`peers.get`**: Get specific peer by public key
   - Params: `{"pubkey": "abc123..."}`
   - Returns: Single `PeerInfo` object or null

3. **`peers.count`**: Get peer statistics
   - Returns: `{"active": 15, "total": 23, "dead": 8}`

4. **`daemon.status`**: Get daemon status
   - Returns: Mesh IP, local pubkey, uptime, interface name

5. **`daemon.ping`**: Health check
   - Returns: `{"pong": true, "version": "v1.0.0"}`

### Implementation Plan

#### 1. Create RPC Server Package (`pkg/rpc/`)

**File: `pkg/rpc/server.go`**
```go
package rpc

import (
    "context"
    "net"
    "os"
)

type Server struct {
    socketPath string
    listener   net.Listener
    daemon     DaemonInterface
}

type DaemonInterface interface {
    GetPeerStore() PeerStoreInterface
    GetLocalNode() LocalNodeInfo
    GetUptime() time.Duration
}

func NewServer(socketPath string, daemon DaemonInterface) (*Server, error)
func (s *Server) Start(ctx context.Context) error
func (s *Server) Stop() error
func (s *Server) handleConnection(conn net.Conn)
func (s *Server) handleRequest(req *Request) *Response
```

**File: `pkg/rpc/protocol.go`**
```go
package rpc

type Request struct {
    JSONRPC string                 `json:"jsonrpc"`
    Method  string                 `json:"method"`
    Params  map[string]interface{} `json:"params"`
    ID      interface{}            `json:"id"`
}

type Response struct {
    JSONRPC string      `json:"jsonrpc"`
    Result  interface{} `json:"result,omitempty"`
    Error   *Error      `json:"error,omitempty"`
    ID      interface{} `json:"id"`
}

type Error struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}
```

**File: `pkg/rpc/client.go`**
```go
package rpc

type Client struct {
    socketPath string
    conn       net.Conn
}

func NewClient(socketPath string) (*Client, error)
func (c *Client) Call(method string, params map[string]interface{}) (interface{}, error)
func (c *Client) Close() error
```

#### 2. Integrate into Daemon (`pkg/daemon/daemon.go`)

Add RPC server to the daemon:

```go
type Daemon struct {
    // ... existing fields
    rpcServer *rpc.Server
}

func (d *Daemon) Run() error {
    // ... existing initialization

    // Start RPC server
    socketPath := "/var/run/wgmesh.sock"
    d.rpcServer = rpc.NewServer(socketPath, d)
    go d.rpcServer.Start(d.ctx)
    defer d.rpcServer.Stop()

    // ... rest of Run()
}
```

Implement `DaemonInterface`:

```go
func (d *Daemon) GetPeerStore() *PeerStore {
    return d.peerStore
}

func (d *Daemon) GetLocalNode() LocalNodeInfo {
    return LocalNodeInfo{
        PubKey: d.localNode.WGPubKey,
        MeshIP: d.localNode.MeshIP,
    }
}

func (d *Daemon) GetUptime() time.Duration {
    return time.Since(d.startTime)
}
```

#### 3. Add CLI Commands (`main.go`)

Add new subcommand `peers`:

```go
case "peers":
    peersCmd()
    return
```

Implement `peersCmd()`:

```go
func peersCmd() {
    fs := flag.NewFlagSet("peers", flag.ExitOnError)
    fs.Parse(os.Args[2:])

    if fs.NArg() < 1 {
        fmt.Fprintln(os.Stderr, "Usage: wgmesh peers <list|count|get>")
        os.Exit(1)
    }

    action := fs.Arg(0)

    client, err := rpc.NewClient("/var/run/wgmesh.sock")
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to connect to daemon: %v\n", err)
        fmt.Fprintln(os.Stderr, "Is wgmesh daemon running? (wgmesh join)")
        os.Exit(1)
    }
    defer client.Close()

    switch action {
    case "list":
        result, err := client.Call("peers.list", nil)
        // Format and display
    case "count":
        result, err := client.Call("peers.count", nil)
        // Format and display
    case "get":
        if fs.NArg() < 2 {
            fmt.Fprintln(os.Stderr, "Usage: wgmesh peers get <pubkey>")
            os.Exit(1)
        }
        pubkey := fs.Arg(1)
        result, err := client.Call("peers.get", map[string]interface{}{"pubkey": pubkey})
        // Format and display
    }
}
```

#### 4. Socket Path Configuration

Support different socket paths based on:
- Default: `/var/run/wgmesh.sock` (requires root)
- Fallback: `$XDG_RUNTIME_DIR/wgmesh.sock` (for non-root)
- Override: `--socket-path` flag or `WGMESH_SOCKET` env var

#### 5. Security Considerations

1. **Socket permissions**: Set socket to `0600` (owner-only access)
2. **User authentication**: Verify connecting UID matches daemon UID
3. **Rate limiting**: Prevent DoS via excessive requests
4. **Input validation**: Validate all RPC method names and parameters

### File Structure

```
pkg/
├── rpc/
│   ├── server.go        # RPC server implementation
│   ├── client.go        # RPC client implementation
│   ├── protocol.go      # Request/response structures
│   ├── handlers.go      # RPC method handlers
│   ├── server_test.go   # Server tests
│   └── client_test.go   # Client tests
├── daemon/
│   └── daemon.go        # Add RPC server integration
main.go                  # Add `peers` subcommand
```

## Affected Files

**New files:**
- `pkg/rpc/server.go`
- `pkg/rpc/client.go`
- `pkg/rpc/protocol.go`
- `pkg/rpc/handlers.go`
- `pkg/rpc/server_test.go`
- `pkg/rpc/client_test.go`

**Modified files:**
- `pkg/daemon/daemon.go` (add RPC server lifecycle)
- `pkg/daemon/peerstore.go` (possibly add interface methods)
- `main.go` (add `peers` subcommand)
- `README.md` (document new commands)

## Test Strategy

1. **Unit tests:**
   - RPC protocol serialization/deserialization
   - RPC method handlers with mock peer store
   - Client request/response handling

2. **Integration tests:**
   - Start daemon with RPC server
   - Connect client and perform queries
   - Verify correct data is returned
   - Test error conditions (daemon not running, invalid requests)

3. **Manual testing:**
   - Start daemon: `wgmesh join --secret <key>`
   - Query peers: `wgmesh peers list`
   - Verify output matches peer store state
   - Test with multiple concurrent clients

## Error Handling

1. **Socket already in use:** Exit with error if socket exists (daemon already running)
2. **Permission denied:** Suggest running with appropriate privileges
3. **Daemon not running:** Client should show helpful error message
4. **Invalid RPC method:** Return JSON-RPC error with code -32601
5. **Invalid params:** Return JSON-RPC error with code -32602

## Estimated Complexity

**Medium-High**

This feature requires:
- New package implementation (RPC protocol)
- Daemon lifecycle integration
- CLI command additions
- Comprehensive testing

Estimated effort: 2-3 days for implementation + testing

## Alternative Approaches Considered

1. **HTTP REST API:** More complex, requires port allocation, less suitable for local IPC
2. **gRPC:** Heavier dependency, overkill for simple local IPC
3. **Shared memory / file-based:** Brittle, requires complex locking, no request/response pattern
4. **D-Bus:** Platform-specific (Linux), additional dependency

**Chosen approach (Unix socket + JSON-RPC)** is simpler, well-supported in Go stdlib, and suitable for local IPC.

## Future Enhancements

1. Add WebSocket support for real-time peer updates
2. Implement subscription mechanism for event notifications
3. Add administrative commands (force peer refresh, manual peer add/remove)
4. Support remote RPC over TCP with authentication (optional)

## References

- JSON-RPC 2.0 Spec: https://www.jsonrpc.org/specification
- Go `net` package Unix socket examples
- Docker daemon API (similar architecture)
- systemd socket activation (future enhancement)
