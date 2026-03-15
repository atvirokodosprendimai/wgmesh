# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test

```bash
make build                          # go build -o wgmesh
make test                           # go test ./...
make lint                           # golangci-lint run
make fmt                            # go fmt ./...
make deps                           # go mod download && go mod tidy

go test ./pkg/crypto/...            # single package
go test ./pkg/daemon -run TestPeerStore -v  # single test
go test -race ./...                 # REQUIRED for any concurrency changes
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

Version injected via ldflags: `-X main.version={{.Version}}`.

## Architecture

Go 1.25 module: `github.com/atvirokodosprendimai/wgmesh`

### Two Operating Modes

**Centralized** (`pkg/mesh`, `pkg/ssh`, `pkg/wireguard`): Operator manages `mesh-state.json` with node topology. CLI pushes WireGuard configs via SSH using `wg show dump` -> parse -> diff -> `wg set` (live, no-restart updates). State file optionally AES-256-GCM + PBKDF2 encrypted.

**Decentralized** (`pkg/daemon`, `pkg/discovery`, `pkg/crypto`): All cryptographic parameters derived from a single shared secret via HKDF into `DerivedKeys` (see `pkg/crypto/derive.go`). Mesh IPs are deterministic from pubkey + secret. Daemon runs a 5-second reconcile loop: reads PeerStore, applies WireGuard config diffs.

### Discovery Layers (Decentralized Mode)

- **L0 — GitHub Registry** (`discovery/registry.go`): Bootstraps by finding/creating GitHub Issues tagged `wgmesh-{rendezvousID}`
- **L1 — LAN Multicast** (`discovery/lan.go`): UDP multicast on deterministic group from `keys.MulticastID`
- **L2 — BitTorrent DHT** (`discovery/dht.go`): Announces on `keys.NetworkID` infohash, hourly rotation for privacy; drives NAT hole-punching
- **L3 — In-Mesh Gossip** (`discovery/gossip.go`): Encrypted UDP broadcasts over established tunnels for transitive discovery

All wire messages use `crypto.SealEnvelope`/`OpenEnvelope` (AES-256-GCM with derived gossip key).

### Key Decoupling Pattern

The `daemon` package doesn't import `discovery`. Instead, `pkg/discovery/init.go` registers a factory via `daemon.SetDHTDiscoveryFactory()` in its `init()`. The main binary triggers this with `_ "github.com/.../pkg/discovery"`. The daemon only knows the `DiscoveryLayer` interface (`Start()/Stop()`).

### Extracted Repos

- **chimney** — GitHub API caching proxy / dashboard server → [github.com/atvirokodosprendimai/chimney](https://github.com/atvirokodosprendimai/chimney)
- **lighthouse** — CDN control plane with REST API, Dragonfly store, xDS/Envoy sync → [github.com/atvirokodosprendimai/lighthouse](https://github.com/atvirokodosprendimai/lighthouse)
- **lighthouse-go** — Go client SDK for the Lighthouse API → [github.com/atvirokodosprendimai/lighthouse-go](https://github.com/atvirokodosprendimai/lighthouse-go)

The `wgmesh service` subcommand imports `lighthouse-go` SDK to talk to the Lighthouse API.

### RPC

JSON-RPC 2.0 over Unix socket (`pkg/rpc/`). Methods: `peers.list`, `peers.get`, `peers.count`, `daemon.status`, `daemon.ping`. Server wired with callback closures from daemon.

### Secret Format

`wgmesh://v1/<base64url-encoded-32-bytes>` — parsed by `daemon.parseSecret()` and `service.normalizeSecret()`.

### Hot Reload

Daemon watches for `SIGHUP`, reads `/var/lib/wgmesh/{iface}.reload` (KEY=VALUE) for `advertise-routes` and `log-level`.

## Code Conventions

- **Imports**: three groups separated by blank lines — stdlib, external, internal
- **Errors**: always wrap with context: `fmt.Errorf("context: %w", err)`
- **Concurrency**: `sync.RWMutex` with `defer` unlock. PeerStore notifies subscribers outside the lock to prevent deadlock
- **Testing**: table-driven, `t.Parallel()` for independent tests, mock via `CommandExecutor` interface
- **CLI tests** (`main_test.go`): build a binary to `/tmp/wgmesh-test`, exec and verify output/exit codes

## What NOT to Modify Without Review

- Encryption algorithms or key derivation parameters (`pkg/crypto/`)
- WireGuard key generation logic
- DHT bootstrap nodes
- `go.mod`/`go.sum` unless the task requires it — always `go mod tidy` after changes

## Spec-Only Triage Mode

When triaging issues (not implementing): create `specs/issue-{NUMBER}-spec.md` with Classification, Deliverables, Problem Analysis, Proposed Approach, Affected Files, Test Strategy, Estimated Complexity. Open as PR titled `spec: Issue #{NUMBER} - {description}`. No code changes.

## CI/CD

- GoReleaser on `v*.*.*` tags: Linux/Darwin (amd64/arm64/arm7), `.deb`/`.rpm`, Homebrew
- Docker: `ghcr.io/atvirokodosprendimai/wgmesh`
- Goose automated implementation triggered by `approved-for-build` label on spec PRs

## Eidos Specs

Design specs live in `eidos/`. Plans and decisions live in `memory/`. See the eidos system prompt for workflow details.
