# Specification: Issue #578

## Classification
feature

## Deliverables
code + documentation

## Problem Analysis

`wgmesh join --secret <SECRET>` uses a single shared string for two orthogonal
purposes:

1. **Network topology derivation** — The secret is fed into `DeriveKeys()` (HKDF-SHA256)
   to produce the DHT infohash (`NetworkID`), gossip encryption key (`GossipKey`), mesh
   subnet, pre-shared key (PSK), gossip port, rendezvous ID, membership key, and epoch
   seed.  Every node in the same mesh uses the same secret and therefore derives the same
   values.

2. **Per-node WireGuard identity** — Each node auto-generates its own Curve25519 keypair
   via `wireguard.GenerateKeyPair()` on first start, persists it in
   `/var/lib/wgmesh/<iface>.json`, and uses the public key to deterministically derive its
   mesh IP address.

The issue asks for a way to **supply the WireGuard keypair externally** (via files) instead
of having the daemon auto-generate it.  The motivations are:

- **Larger/enterprise deployments** want to pre-provision node identities through an
  existing key management or PKI system.
- Operators may need to know the node's WireGuard public key **before** the node starts
  (e.g., for firewall allowlists, DNS registrations, or audits).
- Organizations that treat private key material as secrets under strict lifecycle controls
  (generation, rotation, revocation) cannot accept auto-generated, unmanaged keys.

The `--secret` flag remains required for network-level parameters; the new flags
(`--private-key`, `--public-key`) only control the node's WireGuard identity.

### Scope

The primary change target is `wgmesh join`.  The `install-service` subcommand (which
generates a systemd unit that calls `wgmesh join`) must also accept and thread through the
same flags so that the systemd unit references the key files.

## Proposed Approach

Add two optional flags to `wgmesh join` and `wgmesh install-service`:

| Flag | Type | Purpose |
|------|------|---------|
| `--private-key <path>` | string (file path) | Path to file containing a WireGuard private key (base64, 44 chars) |
| `--public-key <path>` | string (file path) | Optional path to file containing the corresponding public key; if omitted the public key is derived from the private key via `wg pubkey` |

When `--private-key` is provided:
1. The daemon reads the file, trims whitespace, validates the key format (44-char base64url
   or standard base64), and derives the public key if `--public-key` is absent.
2. If both are provided, the public key is validated to match the private key.
3. The keypair overrides auto-generation in `initLocalNode()` **and** overrides any
   keypair already persisted in the state file (so a key rotation is possible by providing
   a new file).
4. The overriding keypair is persisted back to the state file for subsequent restarts
   that don't use the flag.

When `--private-key` is **not** provided the behaviour is unchanged (auto-generate or
reuse persisted keypair).

## Implementation Tasks

### Task 1: Add `DerivePublicKey` to `pkg/wireguard/keys.go`

File: `pkg/wireguard/keys.go`

Add a new exported function **after** `GenerateKeyPair`:

```go
// DerivePublicKey derives the WireGuard public key from a private key using
// the `wg pubkey` command.  privateKey must be a trimmed base64 string (44
// characters for a 32-byte Curve25519 key).
func DerivePublicKey(privateKey string) (string, error) {
	cmd := exec.Command(wgPath, "pubkey")
	cmd.Stdin = strings.NewReader(privateKey + "\n")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to derive public key: %w", err)
	}
	return strings.TrimSpace(out.String()), nil
}
```

No changes to `GenerateKeyPair`.

### Task 2: Add `PrivateKey` / `PublicKey` fields to `DaemonOpts` and `Config` in `pkg/daemon/config.go`

File: `pkg/daemon/config.go`

**2a — Extend `Config` struct** (add two fields after `CustomSubnet`):

```go
type Config struct {
	Secret          string
	Keys            *crypto.DerivedKeys
	InterfaceName   string
	WGListenPort    int
	AdvertiseRoutes []string
	LogLevel        string
	Privacy         bool
	Gossip          bool
	LANDiscovery    bool
	Introducer      bool
	DisableIPv6     bool
	ForceRelay      bool
	DisablePunching bool
	CustomSubnet    *net.IPNet
	// OverridePrivateKey, when non-empty, is used as the node's WireGuard
	// private key instead of auto-generating one.
	OverridePrivateKey string
	// OverridePublicKey is the corresponding public key for OverridePrivateKey.
	// It is always populated by NewConfig when OverridePrivateKey is set.
	OverridePublicKey string
}
```

**2b — Extend `DaemonOpts` struct** (add two fields after `MeshSubnet`):

```go
type DaemonOpts struct {
	Secret              string
	InterfaceName       string
	WGListenPort        int
	AdvertiseRoutes     []string
	LogLevel            string
	Privacy             bool
	Gossip              bool
	DisableLANDiscovery bool
	Introducer          bool
	DisableIPv6         bool
	ForceRelay          bool
	DisablePunching     bool
	MeshSubnet          string
	// PrivateKey, when non-empty, is a raw WireGuard private key (base64).
	// If provided the daemon uses it instead of auto-generating a keypair.
	PrivateKey string
	// PublicKey is the corresponding WireGuard public key (base64).
	// If empty and PrivateKey is set, NewConfig derives it with DerivePublicKey.
	PublicKey string
}
```

**2c — Load and validate the keypair inside `NewConfig`**

Add the following block near the end of `NewConfig`, just before the `return &Config{…}` statement:

```go
// Validate and resolve the override WireGuard keypair when provided.
var overridePrivKey, overridePubKey string
if opts.PrivateKey != "" {
	overridePrivKey = strings.TrimSpace(opts.PrivateKey)
	if overridePrivKey == "" {
		return nil, fmt.Errorf("private key is empty after trimming whitespace")
	}
	if opts.PublicKey != "" {
		overridePubKey = strings.TrimSpace(opts.PublicKey)
		// Derive expected public key and compare to guard against mismatches.
		derived, err := wireguard.DerivePublicKey(overridePrivKey)
		if err != nil {
			return nil, fmt.Errorf("validating private key: %w", err)
		}
		if derived != overridePubKey {
			return nil, fmt.Errorf("public key does not match private key")
		}
	} else {
		var err error
		overridePubKey, err = wireguard.DerivePublicKey(overridePrivKey)
		if err != nil {
			return nil, fmt.Errorf("deriving public key from private key: %w", err)
		}
	}
}
```

Then propagate these fields in the returned `Config`:

```go
return &Config{
	Secret:             secret,
	Keys:               keys,
	InterfaceName:      ifaceName,
	WGListenPort:       listenPort,
	AdvertiseRoutes:    opts.AdvertiseRoutes,
	LogLevel:           logLevel,
	Privacy:            opts.Privacy,
	Gossip:             opts.Gossip,
	LANDiscovery:       !opts.DisableLANDiscovery,
	Introducer:         opts.Introducer,
	DisableIPv6:        opts.DisableIPv6,
	ForceRelay:         opts.ForceRelay,
	DisablePunching:    opts.DisablePunching,
	CustomSubnet:       customSubnet,
	OverridePrivateKey: overridePrivKey,
	OverridePublicKey:  overridePubKey,
}, nil
```

Note: `NewConfig` needs to import `wireguard` package.  Add to the import block in
`pkg/daemon/config.go`:

```go
"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
```

### Task 3: Use the override keypair in `pkg/daemon/daemon.go` — `initLocalNode()`

File: `pkg/daemon/daemon.go`, function `initLocalNode()` (currently around line 322).

**3a — Override existing state-file keypair**

When `d.config.OverridePrivateKey` is set and the state file already exists, replace the
persisted keypair with the override **before** the mesh IP check.  Insert the following
block immediately after the `d.localNode = node` assignment:

```go
// If an override keypair was provided at startup, always use it — even if
// the state file already has a different keypair (supports key rotation).
if d.config.OverridePrivateKey != "" {
	d.localNode.WGPrivateKey = d.config.OverridePrivateKey
	d.localNode.WGPubKey     = d.config.OverridePublicKey
	// Force re-derivation of mesh IP because the public key changed.
	d.localNode.MeshIP   = ""
	d.localNode.MeshIPv6 = ""
}
```

**3b — Use the override keypair for new nodes (no state file)**

Replace the `GenerateKeyPair()` call block:

```go
// Generate new keypair
privateKey, publicKey, err := wireguard.GenerateKeyPair()
if err != nil {
    return fmt.Errorf("failed to generate keypair: %w", err)
}
```

with:

```go
// Use override keypair if provided; otherwise generate a new one.
var privateKey, publicKey string
if d.config.OverridePrivateKey != "" {
    privateKey = d.config.OverridePrivateKey
    publicKey  = d.config.OverridePublicKey
} else {
    var err error
    privateKey, publicKey, err = wireguard.GenerateKeyPair()
    if err != nil {
        return fmt.Errorf("failed to generate keypair: %w", err)
    }
}
```

### Task 4: Add `--private-key` / `--public-key` flags to `joinCmd()` in `main.go`

File: `main.go`, function `joinCmd()` (currently around line 275).

**4a — Declare new flags** (add after the existing `pprofAddr` and `metricsAddr` flags,
before `fs.Parse`):

```go
privateKeyFile := fs.String("private-key", "", "Path to file containing WireGuard private key (base64); omit to auto-generate")
publicKeyFile  := fs.String("public-key",  "", "Path to file containing WireGuard public key (base64); derived from --private-key if omitted")
```

**4b — Read key files after `fs.Parse`**

Add the following block right after the existing `WGMESH_SECRET_FILE` fallback block (after
line 317 in the original file):

```go
// Read WireGuard keypair from files if provided.
var privateKeyContent, publicKeyContent string
if *privateKeyFile != "" {
    b, err := os.ReadFile(*privateKeyFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error reading private key file %s: %v\n", *privateKeyFile, err)
        os.Exit(1)
    }
    privateKeyContent = strings.TrimSpace(string(b))
}
if *publicKeyFile != "" {
    b, err := os.ReadFile(*publicKeyFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error reading public key file %s: %v\n", *publicKeyFile, err)
        os.Exit(1)
    }
    publicKeyContent = strings.TrimSpace(string(b))
}
```

**4c — Pass the key strings into `DaemonOpts`**

In the `daemon.NewConfig(daemon.DaemonOpts{…})` call, add:

```go
PrivateKey: privateKeyContent,
PublicKey:  publicKeyContent,
```

### Task 5: Add `--private-key` / `--public-key` flags to `installServiceCmd()` in `main.go`

File: `main.go`, function `installServiceCmd()` (currently around line 673).

**5a — Declare new flags** (add after `meshSubnet` flag, before `fs.Parse`):

```go
privateKeyFile := fs.String("private-key", "", "Path to file containing WireGuard private key (base64)")
publicKeyFile  := fs.String("public-key",  "", "Path to file containing WireGuard public key (base64); derived if omitted")
```

**5b — Add fields to `SystemdServiceConfig`**

File: `pkg/daemon/systemd.go`, struct `SystemdServiceConfig`:

Add two fields (after `MeshSubnet`):

```go
// PrivateKeyFile is the path to the WireGuard private key file, forwarded
// as --private-key in the systemd ExecStart command.
PrivateKeyFile string
// PublicKeyFile is the path to the WireGuard public key file, forwarded
// as --public-key in the systemd ExecStart command.
PublicKeyFile string
```

**5c — Append the flags in `GenerateSystemdUnit`**

In `GenerateSystemdUnit`, after the existing `if cfg.MeshSubnet != ""` block, add:

```go
if cfg.PrivateKeyFile != "" {
    args = append(args, "--private-key", shellQuoteSystemd(cfg.PrivateKeyFile))
}
if cfg.PublicKeyFile != "" {
    args = append(args, "--public-key", shellQuoteSystemd(cfg.PublicKeyFile))
}
```

**5d — Populate fields in `installServiceCmd`**

In the `daemon.SystemdServiceConfig{…}` literal in `installServiceCmd`, add:

```go
PrivateKeyFile: *privateKeyFile,
PublicKeyFile:  *publicKeyFile,
```

The private key content is **not** loaded or validated by `installServiceCmd`; the
validation happens at daemon start when `wgmesh join` runs from the systemd unit.

### Task 6: Update `README.md`

File: `README.md`, section **"Common `join` options"** (the code block showing
`wgmesh join` flags).

Append the following two lines to the example block (before the closing ` ``` `).  The existing
block in `README.md` looks like:

```bash
wgmesh join \
  --secret "wgmesh://v1/<your-secret>" \
  --advertise-routes "192.168.10.0/24,10.0.0.0/8" \
  --listen-port 51820 \
  --interface wg0 \
  --log-level debug \
  --gossip
```

Add the two new flag lines so the block becomes:

```bash
wgmesh join \
  --secret "wgmesh://v1/<your-secret>" \
  --advertise-routes "192.168.10.0/24,10.0.0.0/8" \
  --listen-port 51820 \
  --interface wg0 \
  --log-level debug \
  --gossip \
  --private-key /etc/wgmesh/node.key \
  --public-key  /etc/wgmesh/node.pub
```

Also add a short paragraph after the code block explaining when to use the flags:

```
> **Note:** `--private-key` and `--public-key` are optional.  Use them when you
> manage WireGuard node identities externally (e.g., via a PKI or key management
> system).  If omitted, the daemon generates and persists a keypair automatically.
```

### Task 7: Write tests in `pkg/wireguard/keys_test.go` and `pkg/daemon/config_test.go`

**7a — `pkg/wireguard/keys_test.go`**

If the file does not yet exist, create it.  Add a test for `DerivePublicKey`:

```go
func TestDerivePublicKey(t *testing.T) {
    priv, pub, err := GenerateKeyPair()
    if err != nil {
        t.Skipf("wg binary not available: %v", err)
    }
    derived, err := DerivePublicKey(priv)
    if err != nil {
        t.Fatalf("DerivePublicKey() error: %v", err)
    }
    if derived != pub {
        t.Errorf("DerivePublicKey() = %q, want %q", derived, pub)
    }
}
```

**7b — `pkg/daemon/config_test.go`**

Add a table-driven test for the override keypair path in `NewConfig`.  The test stubs out
the `wireguard.DerivePublicKey` call by using a valid keypair generated from `GenerateKeyPair`
(skip if `wg` is unavailable):

```go
func TestNewConfig_OverrideKeypair(t *testing.T) {
    // Requires wg binary; skip gracefully if unavailable.
    priv, pub, err := wireguard.GenerateKeyPair()
    if err != nil {
        t.Skipf("wg binary not available: %v", err)
    }

    tests := []struct {
        name       string
        privKey    string
        pubKey     string
        wantErrStr string
    }{
        {
            name:    "private key only — public key derived",
            privKey: priv,
            pubKey:  "",
        },
        {
            name:    "both keys match",
            privKey: priv,
            pubKey:  pub,
        },
        {
            name:       "public key mismatch",
            privKey:    priv,
            pubKey:     "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", // wrong
            wantErrStr: "public key does not match private key",
        },
        {
            name:       "empty private key after trim",
            privKey:    "   ",
            pubKey:     "",
            wantErrStr: "private key is empty after trimming whitespace",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cfg, err := NewConfig(DaemonOpts{
                Secret:     "test-secret-long-enough",
                PrivateKey: tt.privKey,
                PublicKey:  tt.pubKey,
            })
            if tt.wantErrStr != "" {
                if err == nil {
                    t.Fatalf("NewConfig() expected error containing %q, got nil", tt.wantErrStr)
                }
                if !strings.Contains(err.Error(), tt.wantErrStr) {
                    t.Fatalf("NewConfig() error = %q, want to contain %q", err.Error(), tt.wantErrStr)
                }
                return
            }
            if err != nil {
                t.Fatalf("NewConfig() unexpected error: %v", err)
            }
            if cfg.OverridePrivateKey != strings.TrimSpace(tt.privKey) {
                t.Errorf("cfg.OverridePrivateKey = %q, want %q", cfg.OverridePrivateKey, strings.TrimSpace(tt.privKey))
            }
            if cfg.OverridePublicKey != pub {
                t.Errorf("cfg.OverridePublicKey = %q, want %q", cfg.OverridePublicKey, pub)
            }
        })
    }
}
```

## Affected Files

| File | Change |
|------|--------|
| `pkg/wireguard/keys.go` | Add `DerivePublicKey(privateKey string) (string, error)` |
| `pkg/daemon/config.go` | Add `OverridePrivateKey`/`OverridePublicKey` to `Config`; add `PrivateKey`/`PublicKey` to `DaemonOpts`; validate/resolve override keypair in `NewConfig`; add `wireguard` import |
| `pkg/daemon/daemon.go` | Use override keypair in `initLocalNode()` (both existing-state and new-node paths) |
| `pkg/daemon/systemd.go` | Add `PrivateKeyFile`/`PublicKeyFile` to `SystemdServiceConfig`; emit flags in `GenerateSystemdUnit` |
| `main.go` | Add `--private-key` / `--public-key` flags to `joinCmd()` and `installServiceCmd()`; thread through to `DaemonOpts` / `SystemdServiceConfig` |
| `README.md` | Document the new flags in the `join` usage section |
| `pkg/wireguard/keys_test.go` | Add `TestDerivePublicKey` |
| `pkg/daemon/config_test.go` | Add `TestNewConfig_OverrideKeypair` |

## Test Strategy

1. `go build ./...` — must compile without errors.
2. `go test ./pkg/wireguard/...` — `TestDerivePublicKey` must pass (skipped gracefully if
   `wg` binary is absent).
3. `go test ./pkg/daemon/...` — `TestNewConfig_OverrideKeypair` must pass (skipped
   gracefully if `wg` binary is absent).
4. `go vet ./...` — no issues.
5. **Manual smoke test (requires `wg` binary):**

```bash
# Generate a keypair
wg genkey | tee /tmp/test.key | wg pubkey > /tmp/test.pub

# Join with explicit keys
wgmesh join \
  --secret "$(wgmesh init --secret 2>&1 | tail -1)" \
  --private-key /tmp/test.key \
  --public-key  /tmp/test.pub \
  --no-lan-discovery \
  --log-level debug
# Expected: daemon starts using /tmp/test.key, not auto-generating a key.

# Mismatch test
echo "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" > /tmp/wrong.pub
wgmesh join \
  --secret "$(wgmesh init --secret 2>&1 | tail -1)" \
  --private-key /tmp/test.key \
  --public-key /tmp/wrong.pub
# Expected: error "public key does not match private key"; exit 1.
```

## Estimated Complexity
medium
