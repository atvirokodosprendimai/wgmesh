# Specification: Issue #561

## Classification
fix

## Deliverables
code + documentation

## Problem Analysis

`wgmesh init --secret` currently generates **32 random bytes** (256 bits of entropy) and encodes them as a `wgmesh://v1/<base64url>` URI via `GenerateSecret()` in `pkg/daemon/config.go`.

While 256 bits is cryptographically sufficient for key derivation, the issue requests increasing the generated secret to **128 bytes** to further reduce any chance of two independent mesh deployments deriving the same network parameters. The base64url-encoded 128-byte secret will be a 171-character string (no padding), still human-passable via copy-paste and `wgmesh join --secret "..."`.

No other part of the secret pipeline changes: `parseSecret()` / `normalizeSecret()` strip the URI prefix, `DeriveKeys()` accepts any string ≥ 16 characters, and all key derivation (HKDF-SHA256) is unchanged.

## Implementation Tasks

### Task 1: Increase generated secret size in `pkg/daemon/config.go`

File: `pkg/daemon/config.go`, function `GenerateSecret()` (currently lines 148–160).

Change the byte-slice allocation from **32** to **128**:

```go
// Before
func GenerateSecret() (string, error) {
	// Generate 32 random bytes
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode as base64url (no padding for cleaner URLs)
	secret := base64.RawURLEncoding.EncodeToString(b)

	return secret, nil
}

// After
func GenerateSecret() (string, error) {
	// Generate 128 random bytes
	b := make([]byte, 128)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode as base64url (no padding for cleaner URLs)
	secret := base64.RawURLEncoding.EncodeToString(b)

	return secret, nil
}
```

Only the literal `32` → `128` and the comment change. No other code in this file changes.

### Task 2: Update `docs/FAQ.md`

File: `docs/FAQ.md`, section "How do mesh secrets work?" → "Why does any string work?".

Find the sentence:

```
When you run `wgmesh init --secret`, it generates 32 random bytes (256 bits of entropy) and formats them as a `wgmesh://v1/<base64url>` URI.
```

Replace it with:

```
When you run `wgmesh init --secret`, it generates 128 random bytes (1024 bits of entropy) and formats them as a `wgmesh://v1/<base64url>` URI.
```

### Task 3: Update `docs/quickstart.md`

File: `docs/quickstart.md`, the inline note in the Step 2 section.

Find the sentence:

```
Generated secrets have 256 bits of entropy from `crypto/rand`; short passphrases are significantly weaker.
```

Replace it with:

```
Generated secrets have 1024 bits of entropy from `crypto/rand`; short passphrases are significantly weaker.
```

### Task 4: Add / update test in `pkg/daemon/config_test.go`

Locate (or create) the test for `GenerateSecret`. Find any test that asserts the encoded secret length is consistent with 32-byte input (base64url of 32 bytes = 43 characters) and update it to expect the new length (base64url of 128 bytes = 171 characters, no padding).

If a test like `TestGenerateSecret` already exists, update the expected-length assertion:

```go
// Before (if present)
if len(secret) != 43 {
    t.Errorf("expected 43-char base64url secret, got %d", len(secret))
}

// After
if len(secret) != 171 {
    t.Errorf("expected 171-char base64url secret, got %d", len(secret))
}
```

If no such test exists, add a table-driven test:

```go
func TestGenerateSecret(t *testing.T) {
    secret, err := GenerateSecret()
    if err != nil {
        t.Fatalf("GenerateSecret() error: %v", err)
    }
    // base64url of 128 bytes = 171 characters (no padding)
    const wantLen = 171
    if len(secret) != wantLen {
        t.Errorf("GenerateSecret() secret length = %d, want %d", len(secret), wantLen)
    }
    // Must not contain padding characters
    if strings.ContainsAny(secret, "=+/") {
        t.Errorf("GenerateSecret() secret contains non-base64url characters: %q", secret)
    }
    // Two calls must produce different secrets
    secret2, err := GenerateSecret()
    if err != nil {
        t.Fatalf("GenerateSecret() second call error: %v", err)
    }
    if secret == secret2 {
        t.Error("GenerateSecret() returned identical secrets on two consecutive calls")
    }
}
```

### Task 5: Verify

Run the following and confirm all pass with no errors:

```bash
go build ./...
go test ./pkg/daemon/...
go vet ./...
```

## Affected Files

- `pkg/daemon/config.go` — change `32` → `128` in `GenerateSecret()`
- `pkg/daemon/config_test.go` — update/add test asserting length 171
- `docs/FAQ.md` — update entropy description from "32 random bytes (256 bits)" to "128 random bytes (1024 bits)"
- `docs/quickstart.md` — update "256 bits of entropy" to "1024 bits of entropy"

## Test Strategy

1. `go test ./pkg/daemon/...` — the updated `TestGenerateSecret` must pass with expected length 171.
2. `go build ./...` — ensures no compilation errors.
3. Manual smoke test: run `go run . init --secret` and verify the printed `wgmesh://v1/<token>` URI payload is 171 characters long (excluding the `wgmesh://v1/` prefix).

## Estimated Complexity
low
