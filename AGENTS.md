# AGENTS.md - Coding Agent Guide for wgmesh

This document provides essential context for agentic coding tools (Claude Code, Copilot, Goose) operating in this repository.

## Project Overview

wgmesh is a Go-based WireGuard mesh network builder with two operational modes:

1. **Centralized mode** (`pkg/mesh`, `pkg/ssh`): Operator-managed node deployment via SSH
2. **Decentralized mode** (`pkg/daemon`, `pkg/discovery`): Self-discovering mesh using a shared secret

### Discovery Layers (Decentralized Mode)
- Layer 0: GitHub Issues-based registry (`pkg/discovery/registry.go`)
- Layer 1: LAN multicast (`pkg/discovery/lan.go`)
- Layer 2: BitTorrent Mainline DHT (`pkg/discovery/dht.go`)
- Layer 3: In-mesh gossip (`pkg/discovery/gossip.go`)

## Project Structure

```
pkg/
├── crypto/      # Key derivation (HKDF), AES-256-GCM, membership tokens
├── daemon/      # Decentralized daemon mode, peer store, reconciliation
├── discovery/   # Peer discovery layers (DHT, gossip, LAN, registry)
├── privacy/     # Dandelion++ announcement relay
├── mesh/        # Centralized mode data structures and operations
├── ssh/         # SSH client and remote WireGuard operations
└── wireguard/   # WireGuard config parsing, diffing, key generation
```

## Build & Test Commands

```bash
# Build
make build        # or: go build -o wgmesh
go build -o wgmesh .

# Test all
make test         # or: go test ./...
go test ./...

# Test single package
go test ./pkg/crypto/...

# Test single file (run specific test)
go test ./pkg/crypto -run TestDeriveKeys
go test ./pkg/daemon -run TestPeerStore -v

# Test with race detector (REQUIRED for concurrency changes)
go test -race ./...
go test -race ./pkg/daemon/...

# Coverage
go test -cover ./...
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

# Format
make fmt          # or: go fmt ./...
go fmt ./...

# Lint
make lint         # or: golangci-lint run
golangci-lint run

# Dependencies
make deps         # or: go mod download && go mod tidy
```

## Code Style

### Language & Formatting
- Go 1.23
- Standard `gofmt` formatting
- Line length: keep under 120 characters
- Tabs for indentation (Go standard)

### Import Organization
```go
import (
    // Standard library
    "crypto/sha256"
    "fmt"
    "net"
    "sync"
    
    // External packages
    "golang.org/x/crypto/hkdf"
    
    // Internal packages
    "github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)
```

### Naming Conventions
- **Packages**: lowercase, single word preferred (e.g., `crypto`, `mesh`, `daemon`)
- **Types**: PascalCase for exported, camelCase for unexported
- **Functions**: PascalCase for exported, camelCase for unexported
- **Constants**: PascalCase or UPPER_SNAKE_CASE for exported
- **Interfaces**: typically end with `-er` (e.g., `Discoverer`, `PeerStore`)

### Error Handling
Always check errors and wrap with context using `fmt.Errorf("context: %w", err)`. Never silently ignore errors.

### Concurrency
Use `sync.Mutex`/`sync.RWMutex` for thread-safe access. Always test with `-race`:
```go
type PeerStore struct {
    mu    sync.RWMutex
    peers map[string]*Peer
}

func (ps *PeerStore) GetPeer(id string) (*Peer, bool) {
    ps.mu.RLock()
    defer ps.mu.RUnlock()
    peer, ok := ps.peers[id]
    return peer, ok
}
```

## Testing Standards

- Test files: `*_test.go` in same package directory
- Test functions: `TestFunctionName` or `TestFunctionName_Scenario`
- Use `t.Parallel()` for independent tests
- Mock external dependencies (network, filesystem, SSH)
- Test error paths, not just success paths
- Aim for >80% coverage on new code
- Always test concurrency code with `-race` flag

### Table-Driven Tests (Preferred)
```go
func TestDeriveKey(t *testing.T) {
    tests := []struct {
        name    string
        secret  string
        wantLen int
        wantErr bool
    }{
        {"valid secret", "test-secret-long-enough", 32, false},
        {"short secret", "short", 0, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            key, err := DeriveKey(tt.secret)
            if (err != nil) != tt.wantErr {
                t.Errorf("DeriveKey() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Security Considerations

- NEVER hardcode secrets, keys, or tokens
- NEVER disable or weaken existing security checks
- NEVER modify encryption algorithms or key derivation parameters without review
- Use constant-time comparison for cryptographic values
- Validate all input from untrusted sources (peers, CLI args, config files)
- Key derivation: HKDF-SHA256 (`pkg/crypto/derive.go`)
- Encryption: AES-256-GCM with unique nonces (`pkg/crypto/envelope.go`)
- Authentication: HMAC (`pkg/crypto/membership.go`)

## What NOT to Modify

- `.git*` files or Git configuration
- `go.mod`/`go.sum` unless explicitly required
- WireGuard key generation logic without security review
- DHT bootstrap nodes without testing
- Prefer Go stdlib over external dependencies; run `go mod tidy` after changes

## Specialized Agents

This repo defines specialized agents in `.github/AGENTS.md`:
- **docs_agent**: Documentation only (*.md files)
- **test_agent**: Test files only (*_test.go)
- **security_agent**: Crypto and security code review
- **refactor_agent**: Code structure improvements

Invoke the appropriate agent for specialized work.
