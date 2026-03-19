# Specification: Issue #349

## Classification
feature

## Deliverables
code

## Problem Analysis

Multi-tenant CDN services registered via `wgmesh service add` currently support only a single implicit security/caching model (TLS auto-terminated at the edge). This issue requests three selectable modes per service/domain:

- **zero-trust** (default): Origin terminates TLS; edge cannot decrypt traffic; no caching.
- **trusted-edge**: Edge terminates TLS and may cache responses (Cloudflare-style).
- **hybrid**: Origin encrypts response payloads; edge caches opaque encrypted blobs without reading content.

Additionally, the issue requires:

- **TPM edge attestation** (FR5): Edge nodes register with Lighthouse via a TPM attestation flow and receive short-lived certificates.
- **Tenant canary verification** (FR6): Tenants can verify that routing is going to the correct edge node.
- **Formal verification** (NFR3): Security properties of the new modes modelled in Tamarin Prover.

### Scope within this repository (`wgmesh`)

This repository owns:

1. The `wgmesh service add/list/remove` CLI (`service.go`).
2. `pkg/mesh/services.go` — `ServiceEntry` struct and persistence.
3. `formal/wgmesh.spthy` — Tamarin security model.
4. The `lighthouse-go` SDK call-site (`service.go:CreateSite`).

TPM attestation and server-side caching logic live in the **lighthouse** and **lighthouse-go** repos. This spec covers only the changes needed in the `wgmesh` repository.

### Current state of affected code

**`pkg/mesh/services.go` — `ServiceEntry`** (lines 11–21):
```go
type ServiceEntry struct {
    SiteID       string    `json:"site_id"`
    Name         string    `json:"name"`
    Domain       string    `json:"domain"`
    LocalAddr    string    `json:"local_addr"`
    Protocol     string    `json:"protocol"`
    RegisteredAt time.Time `json:"registered_at"`
}
```
There is no `CachingMode` field.

**`service.go` — `serviceAddCmd`** (lines ~150–190):
```go
site, err := client.CreateSite(lighthouse.CreateSiteRequest{
    Domain: domain,
    Origin: lighthouse.Origin{ ... },
    TLS:    "auto",
})
```
The call passes `TLS: "auto"` only. No caching mode is sent.

**`formal/wgmesh.spthy`** — five lemmas cover key derivation, message authentication, membership, rotation, and post-rotation secrecy. No lemma covers edge traffic confidentiality (zero-trust) or canary routing correctness.

## Implementation Tasks

### Task 1: Add `CachingMode` type and field to `pkg/mesh/services.go`

**File:** `pkg/mesh/services.go`

Add a string type constant block and extend `ServiceEntry`:

```go
// CachingMode controls TLS termination and caching behaviour at the edge.
type CachingMode string

const (
    // CachingModeZeroTrust is the default. Origin terminates TLS; edge
    // cannot decrypt traffic and performs no caching.
    CachingModeZeroTrust  CachingMode = "zero-trust"
    // CachingModeTrustedEdge allows the edge to terminate TLS and cache
    // responses (Cloudflare-style operation).
    CachingModeTrustedEdge CachingMode = "trusted-edge"
    // CachingModeHybrid allows the edge to cache opaque encrypted blobs
    // produced by the origin; the edge cannot read plaintext content.
    CachingModeHybrid     CachingMode = "hybrid"
)

// DefaultCachingMode is used when the caller does not specify a mode.
const DefaultCachingMode = CachingModeZeroTrust
```

Add the field to `ServiceEntry`:

```go
type ServiceEntry struct {
    SiteID       string      `json:"site_id"`
    Name         string      `json:"name"`
    Domain       string      `json:"domain"`
    LocalAddr    string      `json:"local_addr"`
    Protocol     string      `json:"protocol"`
    CachingMode  CachingMode `json:"caching_mode,omitempty"`  // NEW
    RegisteredAt time.Time   `json:"registered_at"`
}
```

Add a validation helper used by both CLI and unit tests:

```go
// ValidCachingModes lists all accepted caching mode strings.
var ValidCachingModes = []CachingMode{
    CachingModeZeroTrust,
    CachingModeTrustedEdge,
    CachingModeHybrid,
}

// ParseCachingMode validates and normalises a user-supplied caching mode string.
// Returns DefaultCachingMode if input is empty.
// Returns an error if the value is not a known mode.
func ParseCachingMode(s string) (CachingMode, error) {
    if s == "" {
        return DefaultCachingMode, nil
    }
    for _, m := range ValidCachingModes {
        if CachingMode(s) == m {
            return m, nil
        }
    }
    return "", fmt.Errorf("unknown caching mode %q: must be one of zero-trust, trusted-edge, hybrid", s)
}
```

Add required import `"fmt"` if not already present.

---

### Task 2: Add `--caching-mode` flag to `wgmesh service add` in `service.go`

**File:** `service.go`

**Step 2a — Register the flag** in `serviceAddCmd()` immediately after the existing flags (after line with `healthInterval`):

```go
cachingModeStr := fs.String("caching-mode", "", "Edge caching mode: zero-trust (default), trusted-edge, hybrid")
```

**Step 2b — Parse and validate** after `fs.Parse(os.Args[3:])` and before the name/addr validation block:

```go
cachingMode, err := mesh.ParseCachingMode(*cachingModeStr)
if err != nil {
    fmt.Fprintln(os.Stderr, "Error:", err)
    os.Exit(1)
}
```

**Step 2c — Pass to Lighthouse SDK** in the `client.CreateSite(...)` call. The `lighthouse.CreateSiteRequest` struct (defined in the `lighthouse-go` SDK) must already have or gain a `CachingMode string` field. Until the SDK exports that field, set the TLS field using a mapping:

```go
tlsMode := "auto"
switch cachingMode {
case mesh.CachingModeZeroTrust:
    tlsMode = "passthrough"
case mesh.CachingModeTrustedEdge:
    tlsMode = "auto"
case mesh.CachingModeHybrid:
    tlsMode = "auto"
}

site, err := client.CreateSite(lighthouse.CreateSiteRequest{
    Domain:      domain,
    Origin:      lighthouse.Origin{ ... },
    TLS:         tlsMode,
    CachingMode: string(cachingMode),  // requires lighthouse-go SDK update
})
```

**Note:** The `lighthouse.CreateSiteRequest.CachingMode` field does not currently exist in the SDK. Until `github.com/atvirokodosprendimai/lighthouse-go` is updated, omit the `CachingMode` field from the SDK call and only persist it locally. Add a `// TODO(issue-349): pass CachingMode to SDK once lighthouse-go exports the field` comment so it is easy to complete.

**Step 2d — Persist to local state**. Update the `state.Services[name]` assignment to include `CachingMode`:

```go
state.Services[name] = mesh.ServiceEntry{
    SiteID:       site.ID,
    Name:         name,
    Domain:       domain,
    LocalAddr:    localAddr,
    Protocol:     *protocol,
    CachingMode:  cachingMode,    // NEW
    RegisteredAt: time.Now().UTC(),
}
```

**Step 2e — Include mode in the success output** so users can confirm the selection:

```go
fmt.Printf("  Mode:   %s\n", cachingMode)
```
This line should appear after the existing `fmt.Printf("  Origin: ...")` line.

**Step 2f — Update the usage string** printed in `serviceCmd()`:

```
add <name> <local-addr>    Register a service for managed ingress
  --caching-mode <mode>      Edge security/caching mode (zero-trust|trusted-edge|hybrid)
```

---

### Task 3: Update `wgmesh service list` to display `CachingMode`

**File:** `service.go`

In `serviceListCmd()`, the tabwriter output loop currently prints `SiteID`, `Name`, `Domain`, `LocalAddr`, `Protocol`. Add a `MODE` column.

**Step 3a — Update the table header line:**

Change:
```go
fmt.Fprintln(w, "NAME\tDOMAIN\tPROTOCOL\tLOCAL ADDR\tSTATUS")
```
To:
```go
fmt.Fprintln(w, "NAME\tDOMAIN\tPROTOCOL\tMODE\tLOCAL ADDR\tSTATUS")
```

**Step 3b — Update each row** to include the caching mode. For entries loaded from Lighthouse (which may not yet return the field), fall back to `"zero-trust"` if the field is empty:

```go
mode := string(entry.CachingMode)
if mode == "" {
    mode = string(mesh.DefaultCachingMode)
}
fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
    name, entry.Domain, entry.Protocol, mode, entry.LocalAddr, status)
```

---

### Task 4: Extend Tamarin model in `formal/wgmesh.spthy` for zero-trust edge confidentiality

**File:** `formal/wgmesh.spthy`

Add a new protocol block and two lemmas after the existing `L5` lemma and before the executability section.

#### 4a — New rules (add between the existing `Node_Accept_Rotation` rule and the lemmas section):

```
/*
 * ================================================================
 * ZERO-TRUST EDGE — edge cannot decrypt origin traffic
 * ================================================================
 *
 * In zero-trust mode the origin encrypts its response under a key
 * shared only with the tenant client (tenantKey). The edge node
 * (EdgeNode) relays the ciphertext without possessing tenantKey.
 *
 * Invariant: EdgeNode never learns tenantKey.
 */

rule Tenant_Init_ZeroTrust:
  [ Fr(~tenantKey) ]
  -->
  [ !TenantKey(~tenantKey), Out(pk(~tenantKey)) ]

rule Origin_Encrypt_Response:
  [ !TenantKey(tk), Fr(~resp), Fr(~nonce) ]
  -->
  [ OriginBlob(senc(~resp, tk), ~nonce)
  , !OriginResponseSent(~resp) ]

rule Edge_Relay_Blob:
  // Edge relays the ciphertext to the network; it never decrypts.
  [ OriginBlob(blob, nonce) ]
  --[ EdgeRelayed(blob, nonce) ]->
  [ Out(<blob, nonce>) ]

rule Client_Decrypt_Response:
  [ In(<blob, nonce>), !TenantKey(tk) ]
  --[ ClientReceived(sdec(blob, tk), nonce) ]->
  [ ]
```

#### 4b — New lemma `L6 — Zero-trust edge confidentiality`:

```
/*
 * L6 — Zero-trust edge cannot read origin traffic
 *
 * An edge node relays the ciphertext but never learns tenantKey,
 * so the adversary (who models the edge) cannot decrypt origin responses.
 *
 * Expected: VERIFIED
 */
lemma zero_trust_edge_confidentiality:
  "All blob nonce #i.
      EdgeRelayed(blob, nonce) @ i
    ==> not (Ex resp #j. K(resp) @ j
             & ClientReceived(resp, nonce) @ j)"
```

#### 4c — New executability lemma:

```
lemma exec_zero_trust_relay [exists-trace]:
  "Ex blob nonce #i. EdgeRelayed(blob, nonce) @ i"
```

**Important:** The Tamarin model uses abstract symbolic crypto. `senc`/`sdec` are the builtin symmetric encryption operators already present in `builtins: symmetric-encryption`. No new builtins are needed.

---

### Task 5: Unit tests for `ParseCachingMode` and `ServiceEntry` round-trip in `pkg/mesh/services_test.go`

**File:** `pkg/mesh/services_test.go`

Add the following table-driven tests after the existing `TestSaveServicesCreatesDirectory` test:

```go
func TestParseCachingMode(t *testing.T) {
    tests := []struct {
        input   string
        want    CachingMode
        wantErr bool
    }{
        {"", CachingModeZeroTrust, false},
        {"zero-trust", CachingModeZeroTrust, false},
        {"trusted-edge", CachingModeTrustedEdge, false},
        {"hybrid", CachingModeHybrid, false},
        {"invalid", "", true},
        {"ZERO-TRUST", "", true}, // case-sensitive
    }
    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            got, err := ParseCachingMode(tt.input)
            if (err != nil) != tt.wantErr {
                t.Fatalf("ParseCachingMode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
            }
            if !tt.wantErr && got != tt.want {
                t.Errorf("ParseCachingMode(%q) = %q, want %q", tt.input, got, tt.want)
            }
        })
    }
}

func TestServiceEntryCachingModeRoundTrip(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "services.json")

    for _, mode := range ValidCachingModes {
        t.Run(string(mode), func(t *testing.T) {
            state := ServiceState{
                Services: map[string]ServiceEntry{
                    "svc": {
                        SiteID:      "site_test",
                        Name:        "svc",
                        Domain:      "svc.abc123.wgmesh.dev",
                        LocalAddr:   ":8080",
                        Protocol:    "http",
                        CachingMode: mode,
                    },
                },
            }
            if err := SaveServices(path, state); err != nil {
                t.Fatalf("SaveServices: %v", err)
            }
            loaded, err := LoadServices(path)
            if err != nil {
                t.Fatalf("LoadServices: %v", err)
            }
            entry := loaded.Services["svc"]
            if entry.CachingMode != mode {
                t.Errorf("CachingMode round-trip: got %q, want %q", entry.CachingMode, mode)
            }
        })
    }
}
```

---

### Task 6: Update help text in `service.go`

**File:** `service.go`

In the `serviceCmd()` `--help` / usage block, update the description for `service add`:

```
add <name> <local-addr>    Register a service for managed ingress
  --caching-mode zero-trust|trusted-edge|hybrid
                             Security/caching mode (default: zero-trust)
                             zero-trust:   origin terminates TLS, no caching
                             trusted-edge: edge terminates TLS, can cache
                             hybrid:       edge caches encrypted origin blobs
```

---

## Affected Files

| File | Change |
|------|--------|
| `pkg/mesh/services.go` | Add `CachingMode` type, constants, `DefaultCachingMode`, `ValidCachingModes`, `ParseCachingMode`, extend `ServiceEntry` |
| `pkg/mesh/services_test.go` | Add `TestParseCachingMode` and `TestServiceEntryCachingModeRoundTrip` |
| `service.go` | Add `--caching-mode` flag, validate, persist mode, update list output and help text |
| `formal/wgmesh.spthy` | Add rules `Tenant_Init_ZeroTrust`, `Origin_Encrypt_Response`, `Edge_Relay_Blob`, `Client_Decrypt_Response` and lemmas `L6`, `exec_zero_trust_relay` |

**Out of scope for this repository** (tracked in their respective repos):
- `lighthouse-go` SDK: add `CachingMode string` field to `CreateSiteRequest` and `Site` types.
- `lighthouse` service: implement per-site caching mode enforcement, TPM attestation endpoint, and canary challenge/response API.

---

## Test Strategy

### Unit tests (automated, `go test ./pkg/mesh/...`)

- `TestParseCachingMode`: covers all valid modes, empty input (→ zero-trust default), invalid strings, and case-sensitivity.
- `TestServiceEntryCachingModeRoundTrip`: saves and loads each mode through `SaveServices`/`LoadServices`, confirming JSON serialisation round-trip.

### CLI smoke test (manual)

```bash
# Default mode (zero-trust)
wgmesh service add myapi :8080 --secret wgmesh://v1/... --account cr_xxx
# Expected output includes: Mode:   zero-trust

# Explicit trusted-edge
wgmesh service add myapi :8080 --secret wgmesh://v1/... --account cr_xxx \
    --caching-mode trusted-edge
# Expected output includes: Mode:   trusted-edge

# Invalid mode
wgmesh service add myapi :8080 --secret wgmesh://v1/... --account cr_xxx \
    --caching-mode bogus
# Expected: error message listing valid modes, exit code 1

# List shows MODE column
wgmesh service list --secret wgmesh://v1/...
# Expected: table includes MODE column with the stored value
```

### Tamarin model verification

```bash
tamarin-prover --prove formal/wgmesh.spthy
```

Expected: all lemmas VERIFIED, including the new `zero_trust_edge_confidentiality` and `exec_zero_trust_relay`.

---

## Estimated Complexity

**medium** (~3–4 hours)

### Rationale

- `pkg/mesh/services.go` changes are additive (new type + one field + pure helper); backward-compatible JSON (`omitempty`).
- `service.go` changes are additive flag + output lines only.
- Tamarin extension requires understanding the existing rule structure but is self-contained within the symbolic model.
- TPM attestation (FR5) and canary verification (FR6) are left for the lighthouse/lighthouse-go repos and are explicitly out of scope here.
- No new external dependencies; no `go.mod` changes.
