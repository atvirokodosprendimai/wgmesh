package wireguard

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/crypto/curve25519"
)

// GenerateKeyPair generates a new WireGuard private/public key pair using the
// `wg genkey` and `wg pubkey` commands.
func GenerateKeyPair() (privateKey, publicKey string, err error) {
	privCmd := exec.Command(wgPath, "genkey")
	var privOut bytes.Buffer
	privCmd.Stdout = &privOut

	if err := privCmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	privateKey = strings.TrimSpace(privOut.String())

	pubCmd := exec.Command(wgPath, "pubkey")
	pubCmd.Stdin = strings.NewReader(privateKey)
	var pubOut bytes.Buffer
	pubCmd.Stdout = &pubOut

	if err := pubCmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}

	publicKey = strings.TrimSpace(pubOut.String())

	return privateKey, publicKey, nil
}

// GenerateKeyPairBytes generates a WireGuard keypair without external dependencies.
// Returns (privateKey, publicKey, error) as raw byte slices (32 bytes each).
func GenerateKeyPairBytes() ([]byte, []byte, error) {
	privateKey := make([]byte, 32)
	publicKey := make([]byte, 32)

	// Generate random private key using crypto/rand
	if _, err := rand.Read(privateKey); err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Derive public key from private key using Curve25519
	var pubKey [32]byte
	curve25519.ScalarBaseMult(&pubKey, (*[32]byte)(privateKey))
	copy(publicKey, pubKey[:])

	return privateKey, publicKey, nil
}

// PrivateKeyToPublicKey derives a public key from a private key byte slice.
func PrivateKeyToPublicKey(privateKey []byte) ([]byte, error) {
	if len(privateKey) != 32 {
		return nil, fmt.Errorf("private key must be 32 bytes, got %d", len(privateKey))
	}

	var publicKey [32]byte
	curve25519.ScalarBaseMult(&publicKey, (*[32]byte)(privateKey))
	return publicKey[:], nil
}

// ParseKey converts a base64-encoded WireGuard key to a 32-byte slice.
func ParseKey(key string) ([]byte, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}

	// Try standard base64 decoding first
	keyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		// Try base64url (with or without padding)
		// Add padding if needed
		for len(key)%4 != 0 {
			key += "="
		}
		keyBytes, err = base64.URLEncoding.DecodeString(key)
		if err != nil {
			return nil, fmt.Errorf("failed to decode key: %w", err)
		}
	}

	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("invalid key length: expected 32 bytes, got %d", len(keyBytes))
	}

	return keyBytes, nil
}

// FormatKey converts a 32-byte key to base64 WireGuard format (standard base64).
func FormatKey(keyBytes []byte) (string, error) {
	if len(keyBytes) != 32 {
		return "", fmt.Errorf("invalid key length: expected 32 bytes, got %d", len(keyBytes))
	}
	return base64.StdEncoding.EncodeToString(keyBytes), nil
}
