package crypto

import (
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		plaintext []byte
		password  string
	}{
		{
			name:      "simple text",
			plaintext: []byte("hello world"),
			password:  "password123",
		},
		{
			name:      "empty plaintext",
			plaintext: []byte(""),
			password:  "password123",
		},
		{
			name:      "binary data",
			plaintext: []byte{0x00, 0x01, 0xFF, 0x7F, 0x80},
			password:  "password123",
		},
		{
			name:      "unicode text",
			plaintext: []byte("Hello 世界 🌍"),
			password:  "password123",
		},
		{
			name:      "long plaintext",
			plaintext: []byte(strings.Repeat("This is a test of the encryption system. ", 100)),
			password:  "password123",
		},
		{
			name:      "different password 1",
			plaintext: []byte("test data"),
			password:  "another-password",
		},
		{
			name:      "different password 2",
			plaintext: []byte("test data"),
			password:  "yet-another-password!@#$",
		},
		{
			name:      "empty password",
			plaintext: []byte("test data"),
			password:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Encrypt the plaintext
			encrypted, err := Encrypt(tt.plaintext, tt.password)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			// Decrypt the ciphertext
			decrypted, err := Decrypt(encrypted, tt.password)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			// Verify the decrypted data matches the original plaintext
			if string(decrypted) != string(tt.plaintext) {
				t.Errorf("Decrypted data does not match original\nGot:  %v\nWant: %v", decrypted, tt.plaintext)
			}
		})
	}
}

func TestEncryptDecryptNonDeterministic(t *testing.T) {
	t.Parallel()

	plaintext := []byte("same data")
	password := "test-password"

	// Encrypt twice with same inputs
	encrypted1, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("First Encrypt failed: %v", err)
	}

	encrypted2, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Second Encrypt failed: %v", err)
	}

	// Encrypted data should be different due to random salt/nonce
	if encrypted1 == encrypted2 {
		t.Error("Encrypted data should be different due to random salt/nonce")
	}

	// But both should decrypt to the same plaintext
	decrypted1, err := Decrypt(encrypted1, password)
	if err != nil {
		t.Fatalf("First Decrypt failed: %v", err)
	}

	decrypted2, err := Decrypt(encrypted2, password)
	if err != nil {
		t.Fatalf("Second Decrypt failed: %v", err)
	}

	if string(decrypted1) != string(plaintext) {
		t.Errorf("First decrypted data does not match original\nGot:  %v\nWant: %v", decrypted1, plaintext)
	}
	if string(decrypted2) != string(plaintext) {
		t.Errorf("Second decrypted data does not match original\nGot:  %v\nWant: %v", decrypted2, plaintext)
	}
}

func TestDecryptWrongPassword(t *testing.T) {
	t.Parallel()

	plaintext := []byte("secret message")
	encryptPassword := "correct-password"
	wrongPassword := "wrong-password"

	// Encrypt with correct password
	encrypted, err := Encrypt(plaintext, encryptPassword)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Attempt to decrypt with wrong password
	decrypted, err := Decrypt(encrypted, wrongPassword)
	if err == nil {
		t.Error("Decrypt with wrong password should fail, but it succeeded")
	}

	// Verify error message contains expected text
	if !strings.Contains(err.Error(), "wrong password") && !strings.Contains(err.Error(), "decryption failed") {
		t.Errorf("Error message should mention wrong password or decryption failed, got: %v", err)
	}

	// No plaintext should be returned
	if decrypted != nil {
		t.Errorf("Decrypted data should be nil on error, got: %v", decrypted)
	}

	// Verify that correct password still works
	decryptedCorrect, err := Decrypt(encrypted, encryptPassword)
	if err != nil {
		t.Fatalf("Decrypt with correct password failed: %v", err)
	}

	if string(decryptedCorrect) != string(plaintext) {
		t.Errorf("Decrypted data with correct password does not match original\nGot:  %v\nWant: %v", decryptedCorrect, plaintext)
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	tests := []struct {
		name        string
		encoded     string
		errContains string
	}{
		{
			name:        "not base64",
			encoded:     "this is not base64!!!",
			errContains: "failed to decode base64",
		},
		{
			name:        "invalid characters",
			encoded:     "YWJj!!!@@@",
			errContains: "failed to decode base64",
		},
		{
			name:        "empty string",
			encoded:     "",
			errContains: "too short",
		},
		{
			name:        "whitespace only",
			encoded:     "   ",
			errContains: "failed to decode base64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Decrypt(tt.encoded, "test-password")
			if err == nil {
				t.Error("Decrypt with invalid base64 should fail, but it succeeded")
			}

			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Error message should contain %q, got: %v", tt.errContains, err)
			}
		})
	}
}

func TestDecryptShortCiphertext(t *testing.T) {
	tests := []struct {
		name        string
		encoded     string
		errContains string
	}{
		{
			name:        "shorter than salt (10 bytes)",
			encoded:     "YWJjZGVmZ2hp", // 10 bytes decoded
			errContains: "too short",
		},
		{
			name:        "shorter than salt (20 bytes)",
			encoded:     "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXo=", // 20 bytes decoded
			errContains: "too short",
		},
		{
			name:        "exactly salt size but no room for nonce",
			encoded:     strings.Repeat("a", 44), // 32 bytes = salt size
			errContains: "too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Decrypt(tt.encoded, "test-password")
			if err == nil {
				t.Error("Decrypt with short ciphertext should fail, but it succeeded")
			}

			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Error message should contain %q, got: %v", tt.errContains, err)
			}
		})
	}
}

func TestEncryptEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		plaintext []byte
		password  string
	}{
		{
			name:      "very large plaintext (10KB)",
			plaintext: []byte(strings.Repeat("x", 10240)),
			password:  "password123",
		},
		{
			name:      "all zero bytes",
			plaintext: make([]byte, 1024),
			password:  "password123",
		},
		{
			name: "all ones bytes",
			plaintext: func() []byte {
				b := make([]byte, 1024)
				for i := range b {
					b[i] = 0xFF
				}
				return b
			}(),
			password: "password123",
		},
		{
			name:      "mixed content",
			plaintext: []byte("Text\x00Binary\xff\U0001F600Emoji"),
			password:  "password123",
		},
		{
			name:      "password with special characters",
			plaintext: []byte("test data"),
			password:  "p@ssw0rd!#$%^&*()",
		},
		{
			name:      "very long password",
			plaintext: []byte("test data"),
			password:  strings.Repeat("a", 1000),
		},
		{
			name:      "empty password with data",
			plaintext: []byte("test data"),
			password:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Encrypt
			encrypted, err := Encrypt(tt.plaintext, tt.password)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			// Decrypt
			decrypted, err := Decrypt(encrypted, tt.password)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			// Verify match
			if len(decrypted) != len(tt.plaintext) {
				t.Errorf("Decrypted length %d does not match plaintext length %d", len(decrypted), len(tt.plaintext))
			}

			if string(decrypted) != string(tt.plaintext) {
				t.Errorf("Decrypted data does not match original")
			}
		})
	}
}
