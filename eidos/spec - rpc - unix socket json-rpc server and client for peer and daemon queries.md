---
tldr: JSON-RPC 2.0 over a Unix domain socket (0600 permissions); server exposes five methods for peer listing and daemon status via injected callbacks; client is synchronous with an atomic request ID counter.
category: core
---

# RPC — Unix socket JSON-RPC server and client

## Target

The local control interface for the running wgmesh daemon: a Unix domain socket server that
exposes peer and daemon state, and a matching client used by the CLI.

## Behaviour

### Transport

- **Socket**: Unix domain socket, line-delimited JSON (each request/response terminated by `\n`).
- **Permissions**: `chmod 0600` on the socket file — only the socket owner can connect.
- **Socket path resolution** (`GetSocketPath()`), tried in order:
  1. `$WGMESH_SOCKET` environment variable.
  2. `/var/run/wgmesh.sock` — if `/var/run` is writable (root or appropriate capability).
  3. `$XDG_RUNTIME_DIR/wgmesh.sock` — for non-root systemd sessions.
  4. `/tmp/wgmesh.sock` — last resort.
- Existing socket at the target path is removed on server start (handles stale sockets from prior crashes). Fails if the path exists but is not a socket.
- On `Stop()`: cancels context, closes listener, removes socket file.

### Protocol

JSON-RPC 2.0. Each connection is persistent — multiple request/response pairs on one socket.
Server reads lines, dispatches, writes response, repeats.

### Methods

| Method | Params | Result |
|---|---|---|
| `peers.list` | — | `{peers: [{pubkey, mesh_ip, endpoint, last_seen (RFC3339), discovered_via, routable_networks}]}` |
| `peers.get` | `{pubkey: string}` | Single `PeerInfo` or error if not found |
| `peers.count` | — | `{active, total, dead}` |
| `daemon.status` | — | `{mesh_ip, pubkey, uptime, interface, version}` |
| `daemon.ping` | — | `{pong: true, version}` |

Unknown methods return error code `-32601` (method not found).
`peers.get` with missing/invalid `pubkey` returns `-32602` (invalid params).

### Server construction

`NewServer(ServerConfig)` requires four callback functions injected at construction:
- `GetPeers() []*PeerData` — all peers.
- `GetPeer(pubKey string) (*PeerData, bool)` — single peer lookup.
- `GetPeerCounts() (active, total, dead int)` — aggregate counts.
- `GetStatus() *StatusData` — daemon identity and uptime.

The server is agnostic to the daemon's internals; callbacks decouple it from `pkg/daemon`.

### Client

`NewClient(socketPath)` dials the socket and returns a `Client`.
`Call(method, params) (interface{}, error)`: atomic auto-incrementing request ID, sends
one newline-terminated JSON request, reads one newline-terminated response, returns the
`result` field or an error wrapping the `error` field.
Client is synchronous — one in-flight call at a time on a single connection.

## Design

- Line-delimited JSON (NDJSON) over Unix socket: no framing overhead, trivially shell-testable
  with `echo '{"jsonrpc":"2.0","method":"daemon.ping","id":1}' | nc -U /var/run/wgmesh.sock`.
- Callback injection means the server can be unit-tested without a real daemon.
- `0600` socket permissions prevent other users from querying peer lists or daemon state on
  a shared host.
- Synchronous client is sufficient for CLI use (one command → one request → print result).

## Interactions

- `pkg/daemon.Daemon` — wires `GetPeers`, `GetPeer`, `GetPeerCounts`, `GetStatus` callbacks.
- CLI subcommands (`peers`, `status`, `ping`) — use `Client.Call`.
- `main.go` — constructs and starts the server as part of daemon startup.

## Mapping

> [[pkg/rpc/protocol.go]]
> [[pkg/rpc/server.go]]
> [[pkg/rpc/client.go]]
