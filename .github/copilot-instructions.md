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
make lint       # or: go vet ./...
make fmt        # or: gofmt -w .
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

## Problem Analysis
<!-- What is wrong or what is being requested -->

## Proposed Approach
<!-- Step-by-step approach to solve this -->

## Affected Files
<!-- List of files that would need changes -->

## Test Strategy
<!-- How to verify the fix/feature works -->

## Estimated Complexity
<!-- One of: low, medium, high -->
```

4. Open as a PR titled: `spec: Issue #{NUMBER} - {brief description}`
5. Target the `main` branch
6. Include only the spec file - no code changes

## Security Considerations

- Never hardcode secrets, keys, or tokens
- Shared secrets use HKDF derivation (pkg/crypto/derive.go)
- All peer communication is encrypted with AES-256-GCM envelopes
- Membership validation uses HMAC tokens
