# Copilot Instructions for wgmesh

## Project Overview

wgmesh is a Go-based WireGuard mesh network builder with two modes:

1. **Centralized mode** (`pkg/mesh`, `pkg/ssh`): Operator-managed node deployment via SSH
2. **Decentralized mode** (`pkg/daemon`, `pkg/discovery`): Self-discovering mesh using a shared secret

The decentralized mode uses layered discovery:
- Layer 0: GitHub Issues-based registry (`pkg/discovery/registry.go`)
- Layer 1: LAN multicast (`pkg/discovery/lan.go`)
- Layer 2: BitTorrent Mainline DHT (`pkg/discovery/dht.go`)
- Layer 3: In-mesh gossip (`pkg/discovery/gossip.go`)

## Code Style

- **Language**: Go 1.23
- **Formatting**: Standard `gofmt`
- **Error handling**: Always check errors, wrap with context using `fmt.Errorf("context: %w", err)`
- **Concurrency**: Use `sync.Mutex` / `sync.RWMutex`, always test with `-race`
- **Testing**: Standard `testing` package, table-driven tests preferred
- **Dependencies**: Minimize external dependencies, prefer stdlib

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

## Build & Test

```bash
make build      # or: go build
make test       # or: go test ./...
make lint       # or: golangci-lint run (requires golangci-lint to be installed)
make fmt        # or: go fmt ./...
make deps       # or: go mod download && go mod tidy
```

**Note**: If `golangci-lint` is not installed, you can install it from https://golangci-lint.run/usage/install/

**Always run tests with race detector when making concurrency changes:**
```bash
go test -race ./...
```

## When Triaging Issues (Spec-Only Mode)

When asked to triage an issue and write a specification:

1. **Do NOT write implementation code** - only produce a spec document
2. Create the spec file at `specs/issue-{NUMBER}-spec.md`
3. Use this template for the spec:

```markdown
# Specification: Issue #{NUMBER}

## Classification
<!-- One of: fix, feature, refactor, wont-do, needs-info -->

## Deliverables
<!-- What needs to be delivered: code, documentation, or both -->
<!-- Examples: "code", "documentation", "code + documentation" -->

## Problem Analysis
<!-- What is wrong or what is being requested -->

## Proposed Approach
<!-- Step-by-step approach to solve this -->

## Affected Files
<!-- List of files that would need changes (code files and/or documentation files) -->

## Test Strategy
<!-- How to verify the fix/feature works -->

## Estimated Complexity
<!-- One of: low, medium, high -->
```

4. Open as a PR titled: `spec: Issue #{NUMBER} - {brief description}`
5. Target the `main` branch
6. Include only the spec file - no code changes

## Code Examples

### Good Error Handling
```go
// ✅ Good: Check errors and wrap with context
func processConfig(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("reading config file: %w", err)
    }
    // ... process data
    return nil
}
```

### Good Concurrency Pattern
```go
// ✅ Good: Proper mutex usage
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

### Good Testing Pattern
```go
// ✅ Good: Table-driven test
func TestDeriveKey(t *testing.T) {
    tests := []struct {
        name   string
        secret string
        want   int
    }{
        {"valid secret", "test-secret", 32},
        {"empty secret", "", 32},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            key, err := DeriveKey(tt.secret)
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if len(key) != tt.want {
                t.Errorf("got key length %d, want %d", len(key), tt.want)
            }
        })
    }
}
```

## What NOT to Modify

- **DO NOT** modify `.git*` files or Git configuration
- **DO NOT** modify `go.mod`/`go.sum` unless explicitly required for the task
- **DO NOT** remove or disable existing security checks
- **DO NOT** modify WireGuard key generation logic without security review
- **DO NOT** change encryption algorithms or key derivation parameters
- **DO NOT** modify the DHT bootstrap nodes without testing
- **DO NOT** add unnecessary external dependencies

## Dependency Management

- Prefer Go standard library over external dependencies
- Before adding a new dependency, check if stdlib can solve the problem
- **ALWAYS** run `make deps` after modifying `go.mod`
- **ALWAYS** run security checks on new dependencies
- Document why each external dependency is necessary

## Testing Guidelines

- Write tests for all new functionality
- Use table-driven tests for multiple test cases
- Test error paths, not just success paths
- Run `go test -race ./...` for concurrency code
- Aim for >80% code coverage on new code
- Mock external dependencies (network, filesystem)

## Security Considerations

- Never hardcode secrets, keys, or tokens
- Shared secrets use HKDF derivation (pkg/crypto/derive.go)
- All peer communication is encrypted with AES-256-GCM envelopes
- Membership validation uses HMAC tokens
- Always validate input from untrusted sources (peers, CLI args, config files)
- Use constant-time comparison for cryptographic values
