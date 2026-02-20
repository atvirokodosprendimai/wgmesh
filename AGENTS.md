# AGENTS.md - Coding Agent Guide for wgmesh

wgmesh is a Go 1.23 WireGuard mesh network builder with two operational modes:

1. **Centralized mode** (`pkg/mesh`, `pkg/ssh`): Operator-managed node deployment via SSH
2. **Decentralized mode** (`pkg/daemon`, `pkg/discovery`): Self-discovering mesh using a shared secret

Discovery layers (decentralized): GitHub Issues registry (L0), LAN multicast (L1), BitTorrent DHT (L2), in-mesh gossip (L3).

## Project Structure

```
main.go              # CLI entry point (both modes, subcommands: join, init, status, qr, etc.)
cmd/
├── chimney/         # GitHub API proxy service
└── lighthouse/      # CDN control plane service
pkg/
├── crypto/          # HKDF key derivation, AES-256-GCM envelopes, HMAC membership tokens
├── daemon/          # Decentralized daemon: peer store, reconciliation loop, health, relay
├── discovery/       # Peer discovery: DHT, gossip, LAN multicast, registry, STUN, exchange
├── lighthouse/      # Lighthouse API client, auth, store, xDS sync
├── mesh/            # Centralized mode data structures, policy engine
├── privacy/         # Dandelion++ announcement relay
├── ratelimit/       # Token bucket rate limiter
├── routes/          # Route management and diffing
├── rpc/             # RPC protocol, client, server
├── ssh/             # SSH client and remote WireGuard operations
└── wireguard/       # WireGuard config parsing, diffing, key generation
```

## Build & Test Commands

```bash
make build                    # or: go build -o wgmesh .
make test                     # or: go test ./...
make fmt                      # or: go fmt ./...
make lint                     # or: golangci-lint run
make deps                     # or: go mod download && go mod tidy

# Test single package
go test ./pkg/crypto/...

# Test single test by name
go test ./pkg/crypto -run TestDeriveKeys
go test ./pkg/daemon -run TestPeerStore -v

# Test with race detector (REQUIRED for any concurrency changes)
go test -race ./...
go test -race ./pkg/daemon/...

# Coverage
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

## Code Style

### Formatting & Language
- Go 1.23, module path: `github.com/atvirokodosprendimai/wgmesh`
- Standard `gofmt` formatting, tabs for indentation
- Line length: under 120 characters

### Import Organization
Three groups separated by blank lines: stdlib, external, internal.
```go
import (
    "fmt"
    "sync"

    "golang.org/x/crypto/hkdf"

    "github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)
```

### Naming Conventions
- **Packages**: lowercase, single word (`crypto`, `daemon`, `mesh`)
- **Exported types/functions**: PascalCase; unexported: camelCase
- **Constants**: PascalCase or UPPER_SNAKE_CASE for exported
- **Interfaces**: end with `-er` suffix (`Discoverer`, `PeerStore`)

### Error Handling
Always check and wrap with context. Never silently ignore errors.
```go
data, err := os.ReadFile(path)
if err != nil {
    return fmt.Errorf("reading config: %w", err)
}
```

### Concurrency
Use `sync.Mutex`/`sync.RWMutex` for thread-safe access. Always test with `-race`. Use `defer mu.Unlock()` / `defer mu.RUnlock()` pattern.

## Testing Standards

- Test files: `*_test.go` in same package directory
- Test functions: `TestFunctionName` or `TestFunctionName_Scenario`
- Use `t.Parallel()` for independent tests; table-driven tests preferred
- Test error paths, not just success paths
- Mock external dependencies (network, filesystem, SSH)
- Aim for >80% coverage on new code; always run `-race` for concurrency code

Table-driven test pattern:
```go
tests := []struct {
    name string; secret string; wantErr bool
}{
    {"valid secret", "test-secret-long-enough", false},
    {"short secret", "short", true},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        _, err := DeriveKey(tt.secret)
        if (err != nil) != tt.wantErr {
            t.Errorf("DeriveKey() error = %v, wantErr %v", err, tt.wantErr)
        }
    })
}
```

## Security Rules

- NEVER hardcode secrets, keys, or tokens
- NEVER disable or weaken existing security checks
- NEVER modify encryption algorithms or key derivation parameters without review
- Use constant-time comparison for cryptographic values (`crypto/subtle`)
- Validate all input from untrusted sources (peers, CLI args, config files)
- Key derivation: HKDF-SHA256 (`pkg/crypto/derive.go`)
- Encryption: AES-256-GCM with unique nonces (`pkg/crypto/envelope.go`)
- Authentication: HMAC (`pkg/crypto/membership.go`)

## What NOT to Modify

- `.git*` files or Git configuration
- `go.mod`/`go.sum` unless explicitly required
- WireGuard key generation logic without security review
- DHT bootstrap nodes without testing
- Prefer Go stdlib over external deps; run `go mod tidy` after any dependency change

## Copilot Triage (Spec-Only Mode)

When triaging an issue, do NOT write implementation code. Create a spec at `specs/issue-{NUMBER}-spec.md` with: Classification, Deliverables, Problem Analysis, Proposed Approach, Affected Files, Test Strategy, Estimated Complexity. Open as PR titled `spec: Issue #{NUMBER} - {description}`.

## Specialized Agents (`.github/AGENTS.md`)

- **docs_agent**: Markdown files only
- **test_agent**: Test files only (`*_test.go`)
- **security_agent**: Crypto and security code review
- **refactor_agent**: Code structure improvements

## CI/CD

Docker images: `ghcr.io`, built on push to main and tags (`v*.*.*`). Goose automated implementation triggered by `approved-for-build` label on spec PRs. Always check GitHub Actions for errors after pushing.
