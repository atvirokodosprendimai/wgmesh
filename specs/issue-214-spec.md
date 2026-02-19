# Specification: Issue #214

## Classification
fix

## Deliverables
code

## Problem Analysis

The `pkg/crypto/encrypt.go` file implements password-based AES-256-GCM encryption for the vault using PBKDF2 key derivation, but has zero test coverage. No `encrypt_test.go` file exists, which means:

1. The encryption/decryption functionality is untested
2. Error paths (wrong password, corrupted data, invalid input) are not validated
3. Edge cases (empty input, empty password) are not covered
4. There's no verification that round-trip encrypt/decrypt operations work correctly

### Current Implementation Details

From `pkg/crypto/encrypt.go`:
- `Encrypt(plaintext []byte, password string)` encrypts data with AES-256-GCM
  - Generates random 32-byte salt
  - Derives 32-byte key using PBKDF2 (100k iterations, SHA-256)
  - Creates AES cipher block, then GCM mode
  - Generates random nonce
  - Encrypts plaintext using GCM
  - Returns base64-encoded: `[salt][nonce+ciphertext]`

- `Decrypt(encoded string, password string)` decrypts the data
  - Base64 decodes the input
  - Extracts salt (first 32 bytes)
  - Derives key from password + salt using PBKDF2
  - Creates AES cipher + GCM mode
  - Extracts nonce and ciphertext
  - Decrypts and returns plaintext

### Error Conditions to Test

1. **Wrong password**: GCM authentication will fail with "decryption failed (wrong password?)" error
2. **Short ciphertext**: Checks for `len(encrypted) < saltSize` and `len(ciphertext) < nonceSize`
3. **Corrupted base64**: Base64 decoding fails with "failed to decode base64" error
4. **Empty plaintext**: Should encrypt successfully (0-length data is valid)
5. **Empty password**: Should work (though weak security-wise, it's technically valid)

### Existing Test Patterns

From `pkg/crypto/derive_test.go` and `pkg/crypto/membership_test.go`:
- Use table-driven tests with structs containing test cases
- Test function naming: `TestFunctionName` or `TestFunctionName_Scenario`
- Use `t.Parallel()` for independent tests
- Error checking pattern: verify error is not nil, check error message contains expected text
- Deterministic behavior verification (same input → same output)

## Proposed Approach

Create `pkg/crypto/encrypt_test.go` with comprehensive table-driven tests:

### Test 1: Round-trip encryption/decryption
- Test various plaintext inputs (empty, small, large)
- Verify `Decrypt(Encrypt(data, password), password) == data`
- Test with different passwords
- Verify encryption is non-deterministic (same input produces different ciphertext due to random salt/nonce)

### Test 2: Wrong password fails decryption
- Encrypt with one password
- Attempt to decrypt with different password
- Verify error contains "wrong password" or "decryption failed"

### Test 3: Short/invalid ciphertext errors
- Test ciphertext shorter than salt size (< 32 bytes)
- Test ciphertext with salt but no nonce (< 32 + 12 bytes)
- Verify appropriate error messages

### Test 4: Corrupted base64 input
- Pass invalid base64 strings
- Verify "failed to decode base64" error

### Test 5: Edge cases
- Empty plaintext with valid password
- Valid plaintext with empty password
- Very long plaintext (e.g., 10 KB)
- Unicode/binary data

### Test Structure

```go
func TestEncryptDecryptRoundTrip(t *testing.T) {
    tests := []struct {
        name      string
        plaintext []byte
        password  string
    }{
        {"simple text", []byte("hello world"), "password123"},
        {"empty plaintext", []byte(""), "password123"},
        {"binary data", []byte{0x00, 0x01, 0xFF}, "password123"},
        // ... more cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            // Test logic
        })
    }
}
```

Similar structure for error path tests.

## Affected Files

### New Files
- `pkg/crypto/encrypt_test.go` - All test functions

### Existing Files (no changes)
- `pkg/crypto/encrypt.go` - Implementation remains unchanged

## Test Strategy

### Unit Tests
All tests will be in `encrypt_test.go`:

1. **TestEncryptDecryptRoundTrip**: Verify encrypt→decrypt returns original data
2. **TestDecryptWrongPassword**: Verify wrong password fails
3. **TestDecryptShortCiphertext**: Verify short data is rejected
4. **TestDecryptInvalidBase64**: Verify base64 decode errors are handled
5. **TestEncryptEdgeCases**: Empty password, empty plaintext, large data

### Verification Method
- Run `go test ./pkg/crypto -v -run TestEncrypt`
- Run with race detector: `go test -race ./pkg/crypto -run TestEncrypt`
- Verify coverage: `go test -cover ./pkg/crypto`
- Expected coverage increase: ~90%+ for encrypt.go

### Success Criteria
- All 5 test scenarios pass
- Tests use table-driven pattern (matches project style)
- Tests use `t.Parallel()` where applicable
- No changes to production code (`encrypt.go`)
- Tests run successfully with `-race` flag
- Code coverage for `encrypt.go` > 90%

## Estimated Complexity
low

This is straightforward test creation for existing, well-defined functionality. The implementation is already complete and correct; we're just adding missing test coverage following established patterns in the codebase.
