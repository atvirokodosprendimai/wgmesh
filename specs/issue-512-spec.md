# Specification: Issue #512

## Classification
feature

## Deliverables
code

## Problem Analysis

The daemon currently performs partial validation inside `NewConfig()` in `pkg/daemon/config.go`:

- Interface name is validated via `ifname.Validate` (path traversal, length).
- Mesh subnet is validated as IPv4, minimum `/30`.

However, the following validations are absent and can cause late-stage crashes or silent misconfigurations:

1. **Empty secret** — `DeriveKeys("")` succeeds (all nodes with no secret share the same mesh), so no error is surfaced if `--secret` is omitted or empty after URI parsing.
2. **Port range** — `WGListenPort` accepts any integer; values outside `1–65535` are not rejected.
3. **AdvertiseRoutes CIDR validation** — routes supplied via `--advertise-routes` are split by comma but never parsed with `net.ParseCIDR`. A typo like `10.0./24` silently enters the reconcile loop.
4. **No pre-flight `wgmesh validate` command** — operators cannot validate a config without actually starting the daemon.

There is no config-file format for the daemon (config comes entirely from CLI flags / env vars). The `wgmesh validate <config-file>` acceptance criterion therefore requires defining a KEY=VALUE config file format consistent with the existing `.reload` file convention (`pkg/daemon/config.go:LoadReloadFile`), then adding a parser and a CLI subcommand that reads that file and runs all validations.

## Implementation Tasks

### Task 1: Extend `pkg/daemon/validate.go` with `ValidateOpts`

Open `pkg/daemon/validate.go`. The file currently contains only `ValidateInterfaceName`.

Add the following exported function after `ValidateInterfaceName`:

```go
// ValidateOpts checks DaemonOpts for correctness before key derivation.
// It returns a non-nil error whose message names the offending field and
// describes the expected format.
func ValidateOpts(opts DaemonOpts) error {
    if strings.TrimSpace(opts.Secret) == "" {
        return fmt.Errorf("field \"secret\": required; provide a wgmesh://v1/<base64> URI or a passphrase")
    }

    if opts.WGListenPort != 0 && (opts.WGListenPort < 1 || opts.WGListenPort > 65535) {
        return fmt.Errorf("field \"listen-port\": %d is out of range; must be 0 (default 51820) or 1–65535", opts.WGListenPort)
    }

    for i, r := range opts.AdvertiseRoutes {
        r = strings.TrimSpace(r)
        if r == "" {
            continue
        }
        if _, _, err := net.ParseCIDR(r); err != nil {
            return fmt.Errorf("field \"advertise-routes[%d]\": %q is not a valid CIDR; expected format e.g. 192.168.1.0/24", i, r)
        }
    }

    if opts.MeshSubnet != "" {
        if _, _, err := net.ParseCIDR(opts.MeshSubnet); err != nil {
            return fmt.Errorf("field \"mesh-subnet\": %q is not a valid CIDR; expected format e.g. 10.99.0.0/16", opts.MeshSubnet)
        }
    }

    return nil
}
```

Add the required imports to `pkg/daemon/validate.go`:

```go
import (
    "fmt"
    "net"
    "strings"

    "github.com/atvirokodosprendimai/wgmesh/pkg/ifname"
)
```

### Task 2: Call `ValidateOpts` at the top of `NewConfig`

Open `pkg/daemon/config.go`. In `NewConfig`, insert a call to `ValidateOpts` as the very first statement before the `parseSecret` call:

```go
func NewConfig(opts DaemonOpts) (*Config, error) {
    if err := ValidateOpts(opts); err != nil {
        return nil, err
    }

    // Parse secret from URI format if needed
    secret := parseSecret(opts.Secret)
    // ... rest unchanged ...
}
```

This ensures that the validation runs on every code path that constructs a config (daemon start, `status`, `install-service`, etc.).

### Task 3: Define the config file format and add `ParseConfigFile`

Open `pkg/daemon/config.go`. Add the following function after `LoadReloadFile`:

```go
// ParseConfigFile reads a KEY=VALUE config file and returns a DaemonOpts.
// Supported keys (case-insensitive):
//
//   secret            wgmesh://v1/<base64> URI or raw passphrase
//   listen-port       integer 1–65535
//   interface         WireGuard interface name
//   mesh-subnet       IPv4 CIDR (e.g. 10.99.0.0/16)
//   advertise-routes  comma-separated CIDR list
//   log-level         debug|info|warn|error
//
// Lines starting with '#' and empty lines are ignored.
// Unknown keys are silently skipped.
func ParseConfigFile(path string) (DaemonOpts, error) {
    f, err := os.Open(path)
    if err != nil {
        return DaemonOpts{}, fmt.Errorf("open config file: %w", err)
    }
    defer f.Close()

    var opts DaemonOpts
    sc := bufio.NewScanner(f)
    lineNo := 0
    for sc.Scan() {
        lineNo++
        line := strings.TrimSpace(sc.Text())
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        key, val, ok := strings.Cut(line, "=")
        if !ok {
            return DaemonOpts{}, fmt.Errorf("config file line %d: expected KEY=VALUE, got %q", lineNo, line)
        }
        key = strings.TrimSpace(strings.ToLower(key))
        val = strings.TrimSpace(val)
        switch key {
        case "secret":
            opts.Secret = val
        case "listen-port":
            port, err := strconv.Atoi(val)
            if err != nil {
                return DaemonOpts{}, fmt.Errorf("config file line %d: field \"listen-port\": %q is not an integer", lineNo, val)
            }
            opts.WGListenPort = port
        case "interface":
            opts.InterfaceName = val
        case "mesh-subnet":
            opts.MeshSubnet = val
        case "advertise-routes":
            if val == "" {
                opts.AdvertiseRoutes = []string{}
            } else {
                parts := strings.Split(val, ",")
                routes := make([]string, 0, len(parts))
                for _, p := range parts {
                    if r := strings.TrimSpace(p); r != "" {
                        routes = append(routes, r)
                    }
                }
                opts.AdvertiseRoutes = routes
            }
        case "log-level":
            opts.LogLevel = val
        }
    }
    if err := sc.Err(); err != nil {
        return DaemonOpts{}, fmt.Errorf("read config file: %w", err)
    }
    return opts, nil
}
```

Add `"strconv"` to the existing imports in `pkg/daemon/config.go` (it already imports `"bufio"` and `"strings"` — only `"strconv"` is new).

### Task 4: Add `validateCmd` to `main.go`

Open `main.go`. Add the following function (place it near `statusCmd` for readability):

```go
// validateCmd implements "wgmesh validate <config-file>".
// Exits 0 on valid config, 1 on any error (parse or validation).
func validateCmd() {
    if len(os.Args) < 3 {
        fmt.Fprintln(os.Stderr, "Error: config file path required")
        fmt.Fprintln(os.Stderr, "Usage: wgmesh validate <config-file>")
        os.Exit(1)
    }

    path := os.Args[2]

    opts, err := daemon.ParseConfigFile(path)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    if err := daemon.ValidateOpts(opts); err != nil {
        fmt.Fprintf(os.Stderr, "Invalid config: %v\n", err)
        os.Exit(1)
    }

    fmt.Printf("Config %q is valid.\n", path)
}
```

### Task 5: Wire `validate` into the main switch

Open `main.go`. In the `switch os.Args[1]` block in `main()`, add before the closing brace:

```go
case "validate":
    validateCmd()
    return
```

Also add `validate` to the usage string in `printUsage()`. Locate the existing `SUBCOMMANDS (decentralized mode):` block and append:

```
  validate <config-file>        Validate a config file; exits 0 if valid, 1 if invalid
```

### Task 6: Write unit tests for `ValidateOpts`

Create a new file `pkg/daemon/validate_test.go`:

```go
package daemon

import (
    "strings"
    "testing"
)

func TestValidateOpts_EmptySecret(t *testing.T) {
    err := ValidateOpts(DaemonOpts{Secret: ""})
    if err == nil {
        t.Fatal("expected error for empty secret, got nil")
    }
    if !strings.Contains(err.Error(), "secret") {
        t.Errorf("error should mention field \"secret\", got: %v", err)
    }
}

func TestValidateOpts_WhitespaceOnlySecret(t *testing.T) {
    err := ValidateOpts(DaemonOpts{Secret: "   "})
    if err == nil {
        t.Fatal("expected error for whitespace-only secret, got nil")
    }
}

func TestValidateOpts_ValidSecret(t *testing.T) {
    err := ValidateOpts(DaemonOpts{Secret: testConfigSecret})
    if err != nil {
        t.Fatalf("unexpected error for valid secret: %v", err)
    }
}

func TestValidateOpts_PortZeroIsDefault(t *testing.T) {
    // Port 0 means "use default", must not error.
    err := ValidateOpts(DaemonOpts{Secret: testConfigSecret, WGListenPort: 0})
    if err != nil {
        t.Fatalf("unexpected error for port 0: %v", err)
    }
}

func TestValidateOpts_PortBoundaries(t *testing.T) {
    tests := []struct {
        port    int
        wantErr bool
    }{
        {1, false},
        {51820, false},
        {65535, false},
        {-1, true},
        {0, false},      // 0 = default
        {65536, true},
        {99999, true},
    }
    for _, tt := range tests {
        err := ValidateOpts(DaemonOpts{Secret: testConfigSecret, WGListenPort: tt.port})
        if (err != nil) != tt.wantErr {
            t.Errorf("port %d: got err=%v, wantErr=%v", tt.port, err, tt.wantErr)
        }
        if err != nil && !strings.Contains(err.Error(), "listen-port") {
            t.Errorf("port %d: error should mention field \"listen-port\", got: %v", tt.port, err)
        }
    }
}

func TestValidateOpts_ValidAdvertiseRoutes(t *testing.T) {
    err := ValidateOpts(DaemonOpts{
        Secret:          testConfigSecret,
        AdvertiseRoutes: []string{"10.0.0.0/8", "192.168.1.0/24"},
    })
    if err != nil {
        t.Fatalf("unexpected error for valid routes: %v", err)
    }
}

func TestValidateOpts_InvalidAdvertiseRoute(t *testing.T) {
    err := ValidateOpts(DaemonOpts{
        Secret:          testConfigSecret,
        AdvertiseRoutes: []string{"10.0.0.0/8", "not-a-cidr"},
    })
    if err == nil {
        t.Fatal("expected error for invalid CIDR route, got nil")
    }
    if !strings.Contains(err.Error(), "advertise-routes") {
        t.Errorf("error should mention field \"advertise-routes\", got: %v", err)
    }
}

func TestValidateOpts_ValidMeshSubnet(t *testing.T) {
    err := ValidateOpts(DaemonOpts{
        Secret:     testConfigSecret,
        MeshSubnet: "10.50.0.0/16",
    })
    if err != nil {
        t.Fatalf("unexpected error for valid mesh subnet: %v", err)
    }
}

func TestValidateOpts_InvalidMeshSubnet(t *testing.T) {
    err := ValidateOpts(DaemonOpts{
        Secret:     testConfigSecret,
        MeshSubnet: "not-a-cidr",
    })
    if err == nil {
        t.Fatal("expected error for invalid mesh subnet, got nil")
    }
    if !strings.Contains(err.Error(), "mesh-subnet") {
        t.Errorf("error should mention field \"mesh-subnet\", got: %v", err)
    }
}

func TestValidateOpts_EmptyAdvertiseRoutesSkipped(t *testing.T) {
    // Empty strings in the slice must not cause errors.
    err := ValidateOpts(DaemonOpts{
        Secret:          testConfigSecret,
        AdvertiseRoutes: []string{"", "  ", "10.0.0.0/8"},
    })
    if err != nil {
        t.Fatalf("unexpected error when advertise-routes contains empty strings: %v", err)
    }
}
```

### Task 7: Write unit tests for `ParseConfigFile`

Add a second test file `pkg/daemon/configfile_test.go`:

```go
package daemon

import (
    "os"
    "path/filepath"
    "testing"
)

func writeConfigFile(t *testing.T, content string) string {
    t.Helper()
    dir := t.TempDir()
    path := filepath.Join(dir, "wgmesh.conf")
    if err := os.WriteFile(path, []byte(content), 0600); err != nil {
        t.Fatalf("failed to write temp config: %v", err)
    }
    return path
}

func TestParseConfigFile_ValidFull(t *testing.T) {
    path := writeConfigFile(t, `
# wgmesh config
secret=wgmesh://v1/dGVzdA
listen-port=51821
interface=wg1
mesh-subnet=10.50.0.0/16
advertise-routes=192.168.1.0/24,10.0.0.0/8
log-level=debug
`)
    opts, err := ParseConfigFile(path)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if opts.Secret != "wgmesh://v1/dGVzdA" {
        t.Errorf("secret: got %q", opts.Secret)
    }
    if opts.WGListenPort != 51821 {
        t.Errorf("listen-port: got %d", opts.WGListenPort)
    }
    if opts.InterfaceName != "wg1" {
        t.Errorf("interface: got %q", opts.InterfaceName)
    }
    if opts.MeshSubnet != "10.50.0.0/16" {
        t.Errorf("mesh-subnet: got %q", opts.MeshSubnet)
    }
    if len(opts.AdvertiseRoutes) != 2 {
        t.Errorf("advertise-routes: got %v", opts.AdvertiseRoutes)
    }
    if opts.LogLevel != "debug" {
        t.Errorf("log-level: got %q", opts.LogLevel)
    }
}

func TestParseConfigFile_CommentsAndBlankLines(t *testing.T) {
    path := writeConfigFile(t, `
# This is a comment
secret=mysecret

# Another comment
log-level=info
`)
    opts, err := ParseConfigFile(path)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if opts.Secret != "mysecret" {
        t.Errorf("secret: got %q", opts.Secret)
    }
    if opts.LogLevel != "info" {
        t.Errorf("log-level: got %q", opts.LogLevel)
    }
}

func TestParseConfigFile_MissingFile(t *testing.T) {
    _, err := ParseConfigFile("/nonexistent/path/wgmesh.conf")
    if err == nil {
        t.Fatal("expected error for missing file, got nil")
    }
}

func TestParseConfigFile_BadPortNotInteger(t *testing.T) {
    path := writeConfigFile(t, "secret=s\nlisten-port=abc\n")
    _, err := ParseConfigFile(path)
    if err == nil {
        t.Fatal("expected error for non-integer port, got nil")
    }
}

func TestParseConfigFile_MalformedLine(t *testing.T) {
    path := writeConfigFile(t, "secret=ok\nthisisnotakeyvalue\n")
    _, err := ParseConfigFile(path)
    if err == nil {
        t.Fatal("expected error for malformed line, got nil")
    }
}

func TestParseConfigFile_UnknownKeysIgnored(t *testing.T) {
    path := writeConfigFile(t, "secret=ok\nunknown-key=whatever\n")
    opts, err := ParseConfigFile(path)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if opts.Secret != "ok" {
        t.Errorf("secret: got %q", opts.Secret)
    }
}

func TestParseConfigFile_EmptyAdvertiseRoutes(t *testing.T) {
    path := writeConfigFile(t, "secret=s\nadvertise-routes=\n")
    opts, err := ParseConfigFile(path)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(opts.AdvertiseRoutes) != 0 {
        t.Errorf("expected empty AdvertiseRoutes, got %v", opts.AdvertiseRoutes)
    }
}
```

## Affected Files

| File | Change |
|------|--------|
| `pkg/daemon/validate.go` | Add `ValidateOpts(opts DaemonOpts) error` with imports |
| `pkg/daemon/validate_test.go` | **New** — unit tests for each validation rule |
| `pkg/daemon/config.go` | Call `ValidateOpts` at top of `NewConfig`; add `ParseConfigFile`; add `"strconv"` import |
| `pkg/daemon/configfile_test.go` | **New** — unit tests for `ParseConfigFile` |
| `main.go` | Add `validateCmd()` function; wire `"validate"` case in `main()`; update `printUsage()` |

No changes to `go.mod`/`go.sum` — all additions use stdlib only.

## Test Strategy

Run after implementation:

```bash
go test ./pkg/daemon/... -run TestValidateOpts
go test ./pkg/daemon/... -run TestParseConfigFile
go test ./pkg/daemon/... -race
go test ./...
```

Manual end-to-end verification:

```bash
# Valid config — exits 0
cat > /tmp/wgmesh.conf <<'EOF'
secret=wgmesh://v1/dGVzdHNlY3JldA
listen-port=51820
mesh-subnet=10.99.0.0/16
advertise-routes=192.168.10.0/24
EOF
wgmesh validate /tmp/wgmesh.conf
# Expected: "Config \"/tmp/wgmesh.conf\" is valid."  exit 0

# Missing secret — exits 1
cat > /tmp/bad.conf <<'EOF'
listen-port=51820
EOF
wgmesh validate /tmp/bad.conf
# Expected: "Invalid config: field \"secret\": required..."  exit 1

# Bad port — exits 1
cat > /tmp/badport.conf <<'EOF'
secret=wgmesh://v1/dGVzdA
listen-port=99999
EOF
wgmesh validate /tmp/badport.conf
# Expected: "Invalid config: field \"listen-port\": 99999 is out of range..."  exit 1

# Bad route CIDR — exits 1
cat > /tmp/badroute.conf <<'EOF'
secret=wgmesh://v1/dGVzdA
advertise-routes=not-a-cidr
EOF
wgmesh validate /tmp/badroute.conf
# Expected: "Invalid config: field \"advertise-routes[0]\": ..."  exit 1
```

## Estimated Complexity
low
