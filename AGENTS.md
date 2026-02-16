# AGENTS.md — wgmesh

## Project

WireGuard mesh VPN builder in Go. Two modes: centralized (SSH deploy via `pkg/mesh`, `pkg/ssh`) and decentralized (secret-based autodiscovery via `pkg/daemon`, `pkg/discovery`). CLI entry point is `main.go` at the repo root.

## Build & Test Commands

```bash
make build                          # go build -o wgmesh
make test                           # go test ./...
make fmt                            # go fmt ./...
make lint                           # golangci-lint run (install: https://golangci-lint.run)
make deps                           # go mod download && go mod tidy

# Single package
go test ./pkg/crypto/...
go test ./pkg/daemon/...

# Single test by name
go test ./pkg/daemon/ -run TestPeerStoreUpdate
go test ./pkg/crypto/ -run TestDeriveKeys

# With race detector (required for concurrency changes)
go test -race ./...
go test -race ./pkg/daemon/ -run TestPeerStoreUpdate

# Verbose output
go test -v ./pkg/mesh/ -run TestListSimple

# Coverage
go test -cover ./...
go test -coverprofile=cover.out ./... && go tool cover -html=cover.out
```

CI validates: `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -l .`

## Code Style

**Go version**: 1.23. **CGO**: Disabled (`CGO_ENABLED=0`).

### Formatting

`gofmt` only. No goimports, no custom formatter. Run `make fmt` before committing.

### Imports

Two groups separated by a blank line: stdlib first, then everything else (external + internal mixed).

```go
import (
    "crypto/sha256"
    "fmt"
    "net"

    "golang.org/x/crypto/hkdf"
    "github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)
```

Alias only on conflict: `cryptorand "crypto/rand"` alongside `"math/rand"`.

### Naming

- **Functions/types**: CamelCase with descriptive verbs — `DeriveKeys`, `SealEnvelope`, `DetectCollisions`
- **Acronyms**: ALL CAPS in identifiers — `WGPubKey`, `MeshIP`, `DHT`, `PSK`, `SSHHost`, `FQDN`, `URI`
- **Constants**: CamelCase — `MinSecretLength`, `MaxStemHops`, `FluffProbability`
- **Receivers**: Short — `ps` for PeerStore, `d` for Daemon, `m` for Mesh, `cfg` for Config
- **Test loop var**: `tt` — `for _, tt := range tests { t.Run(tt.name, ...)`
- **JSON tags**: snake_case — `json:"wg_pubkey"`, `json:"mesh_ip"`

### Error Handling

Return errors, don't log them (except in daemon/main layer). Wrap with context using `%w`:

```go
// Library code: wrap and return
return fmt.Errorf("failed to derive gossip key: %w", err)

// Validation: plain error, no wrapping
return fmt.Errorf("secret must be at least %d characters", MinSecretLength)

// CLI layer: log to stderr and exit
fmt.Fprintf(os.Stderr, "Failed to read config: %v\n", err)
os.Exit(1)
```

No custom error types. No sentinel errors. All errors are wrapped stdlib errors.

### Logging

stdlib `log.Printf` with `[Component]` prefix tags:

```go
log.Printf("[Gossip] In-mesh gossip started on port %d", port)
log.Printf("[DHT] Discovered %d peers", count)
```

User-facing CLI output uses `fmt.Printf`/`fmt.Println`. Errors use `fmt.Fprintf(os.Stderr, ...)`.

### Testing

stdlib `testing` only — no testify, no gomock. Table-driven tests are the dominant pattern:

```go
func TestDeriveKeys_MinLength(t *testing.T) {
    tests := []struct {
        name    string
        secret  string
        wantErr bool
    }{
        {"valid", "abcdefghijklmnop", false},
        {"too short", "abc", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := DeriveKeys(tt.secret)
            if (err != nil) != tt.wantErr {
                t.Errorf("DeriveKeys(%q) error = %v, wantErr %v", tt.secret, err, tt.wantErr)
            }
        })
    }
}
```

- Name tests `TestFunctionName` or `TestFunctionName_Scenario`
- Use `t.Helper()` in test helper functions
- Use `t.Skip()` for OS-gated tests: `t.Skip("Test only runs on darwin")`
- Hand-write mocks using interfaces with function fields — no code generation
- Assertions via `t.Fatalf`/`t.Errorf`, string matching via `strings.Contains()`
- No build tags — all tests run with `go test ./...`

### Comments

Exported types and functions get godoc comments. Crypto code has inline "why" comments:

```go
// DerivedKeys holds all keys and parameters derived from a shared secret.
type DerivedKeys struct { ... }

// network_id = SHA256(secret)[0:20] -- DHT infohash (20 bytes)
```

### Dependencies

Minimal. Three direct deps: `anacrolix/dht/v2`, `golang.org/x/crypto`, `golang.org/x/term`. Prefer stdlib. Always run `make deps` after modifying go.mod. Document why any new external dep is necessary.

## Package Layout

```
main.go                  # CLI entry point, subcommand routing
pkg/
  crypto/                # HKDF key derivation, AES-256-GCM envelopes, membership tokens
  daemon/                # Decentralized daemon: config, peerstore, collision, cache, systemd
  discovery/             # DHT, gossip, LAN multicast, registry, peer exchange
  privacy/               # Dandelion++ relay, epoch management
  mesh/                  # Centralized mode: state file, node management, deploy
  ssh/                   # SSH client, remote WireGuard operations
  wireguard/             # WG config parsing, diffing, key gen, apply, persist
```

One concern per file. Test files co-located (`_test.go` in same package).

## Instruction File Precedence

This repo has multiple instruction files. When guidance conflicts, follow this priority order (highest first):

1. **Package-level `CLAUDE.md`** (e.g., `pkg/crypto/CLAUDE.md`) — package-specific rules override general rules for work within that package
2. **Root `AGENTS.md`** (this file) — canonical project-wide conventions
3. **`.github/AGENTS.md`** — Copilot agent persona definitions; scoped to their declared roles
4. **`.github/copilot-instructions.md`** — Copilot-specific context and the spec-writing template
5. **Root `CLAUDE.md`** — Claude Code session context (auto-generated memory)

Package-level files supplement but never contradict root-level conventions. If a package `CLAUDE.md` conflicts with this file, this file wins on style/process rules; the package file wins on domain-specific guidance (e.g., "this package uses X pattern").

## Do Not Modify

- Encryption algorithms or key derivation parameters without security review
- WireGuard key generation logic
- DHT bootstrap nodes without testing
- `.git*` files or git configuration
- `go.mod`/`go.sum` unless the task explicitly requires it

## Copilot Agent Personas

Four specialized agents are defined in `.github/AGENTS.md`:
- **docs_agent** — Markdown only, American English, no code changes
- **test_agent** — `*_test.go` only, table-driven, mock via interfaces
- **security_agent** — `pkg/crypto/`, `pkg/privacy/`, crypto review checklist
- **refactor_agent** — Small incremental changes, no public API breaks

See `.github/copilot-instructions.md` for the spec-writing template used by the CI triage workflow.
