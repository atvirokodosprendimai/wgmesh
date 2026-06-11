package wireguard

import (
	"testing"
)

func TestGenerateKeyPairBytes(t *testing.T) {
	privKey, pubKey, err := GenerateKeyPairBytes()
	if err != nil {
		t.Fatalf("GenerateKeyPairBytes() failed: %v", err)
	}

	// Check key lengths
	if len(privKey) != 32 {
		t.Errorf("private key length: got %d, want 32", len(privKey))
	}
	if len(pubKey) != 32 {
		t.Errorf("public key length: got %d, want 32", len(pubKey))
	}

	// Verify that the public key can be derived from the private key
	derivedPubKey, err := PrivateKeyToPublicKey(privKey)
	if err != nil {
		t.Fatalf("PrivateKeyToPublicKey() failed: %v", err)
	}

	if string(derivedPubKey) != string(pubKey) {
		t.Errorf("derived public key does not match original public key")
	}
}

func TestPrivateKeyToPublicKey(t *testing.T) {
	tests := []struct {
		name       string
		privateKey []byte
		wantErr    bool
	}{
		{
			name:       "valid 32-byte key",
			privateKey: make([]byte, 32),
			wantErr:    false,
		},
		{
			name:       "invalid length - too short",
			privateKey: make([]byte, 16),
			wantErr:    true,
		},
		{
			name:       "invalid length - too long",
			privateKey: make([]byte, 64),
			wantErr:    true,
		},
		{
			name:       "empty key",
			privateKey: []byte{},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pubKey, err := PrivateKeyToPublicKey(tt.privateKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("PrivateKeyToPublicKey() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(pubKey) != 32 {
				t.Errorf("public key length: got %d, want 32", len(pubKey))
			}
		})
	}
}

func TestParseKey(t *testing.T) {
	// Generate a valid 32-byte key for testing
	privKey, _, _ := GenerateKeyPairBytes()
	validKey, _ := FormatKey(privKey)

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "valid base64 key",
			key:     validKey,
			wantErr: false,
		},
		{
			name:    "empty key",
			key:     "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			key:     "   ",
			wantErr: true,
		},
		{
			name:    "invalid base64",
			key:     "not-valid-base64!!!",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyBytes, err := ParseKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseKey() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(keyBytes) != 32 {
				t.Errorf("key length: got %d, want 32", len(keyBytes))
			}
		})
	}
}

func TestFormatKey(t *testing.T) {
	tests := []struct {
		name       string
		keyBytes   []byte
		wantErr    bool
		checkValid bool
	}{
		{
			name:       "valid 32-byte key",
			keyBytes:   make([]byte, 32),
			wantErr:    false,
			checkValid: true,
		},
		{
			name:     "invalid length - too short",
			keyBytes: make([]byte, 16),
			wantErr:  true,
		},
		{
			name:     "invalid length - too long",
			keyBytes: make([]byte, 64),
			wantErr:  true,
		},
		{
			name:     "empty key",
			keyBytes: []byte{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyStr, err := FormatKey(tt.keyBytes)
			if (err != nil) != tt.wantErr {
				t.Errorf("FormatKey() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if tt.checkValid {
					// Verify we can parse it back
					parsed, err := ParseKey(keyStr)
					if err != nil {
						t.Errorf("FormatKey() produced invalid key: %v", err)
					}
					if string(parsed) != string(tt.keyBytes) {
						t.Errorf("round-trip failed: got %v, want %v", parsed, tt.keyBytes)
					}
				}
			}
		})
	}
}
