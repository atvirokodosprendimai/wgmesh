package promo

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"strings"

	"golang.org/x/crypto/hkdf"
)

const (
	codeLength     = 13                                          // Total characters (8 payload + 4 checksum + 1 version)
	checksumLength = 4                                           // Characters used for checksum
	versionLength  = 1                                           // Characters used for version
	payloadLength  = codeLength - checksumLength - versionLength // Should be 8

	payloadBytes  = 5 // Bytes to encode (5 bytes → 8 base32 chars)
	checksumBytes = 2 // Bytes for checksum (2 bytes → 4 base32 chars with padding)

	codeVersion = "A" // Current version character (base32: A=0, valid character)
)

// GenerateCode creates a unique promo code for a campaign.
// The code format: [base32(source_id_hash)][checksum][version]
// Length: 12 characters, URL-safe, case-insensitive.
func GenerateCode(campaignID string, seed string) (Code, error) {
	if campaignID == "" {
		return "", fmt.Errorf("campaign ID cannot be empty")
	}
	if seed == "" {
		return "", fmt.Errorf("seed cannot be empty")
	}

	// Derive payload bytes from campaignID + random seed
	salt := make([]byte, 8)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	payload := make([]byte, payloadBytes)
	kdf := hkdf.New(sha256.New, []byte(seed+campaignID), salt, []byte("wgmesh-promo-v1"))
	if _, err := kdf.Read(payload); err != nil {
		return "", fmt.Errorf("hkdf read: %w", err)
	}

	// Base32 encode 5 bytes → 8 characters (without padding)
	encoded := base32.StdEncoding.EncodeToString(payload)
	encoded = encoded[:payloadLength] // Take first 8 chars

	// Calculate CRC-24 checksum of encoded payload
	checksum := crc24Checksum([]byte(encoded))

	// Encode 16-bit checksum (lower 16 bits of 24-bit CRC) to 2 bytes
	checksumByteSlice := make([]byte, checksumBytes)
	checksumByteSlice[0] = byte((checksum >> 8) & 0xFF)
	checksumByteSlice[1] = byte(checksum & 0xFF)

	// Base32 encode 2 bytes → 4 characters (with padding)
	checksumEncodedFull := base32.StdEncoding.EncodeToString(checksumByteSlice)
	checksumEncoded := checksumEncodedFull[:checksumLength] // Take first 4 chars (without padding)

	// Assemble: [payload(8)][checksum(4)][version(1)]
	code := Code(strings.ToUpper(encoded + checksumEncoded + codeVersion))

	return code, nil
}

// ValidateCode verifies the checksum of a promo code.
// Returns the code payload and true if valid, false if invalid.
func ValidateCode(code Code) (string, bool) {
	if len(code) != codeLength {
		return "", false
	}

	// Extract components
	payload := string(code[:payloadLength])
	checksumEncoded := string(code[payloadLength : payloadLength+checksumLength])
	version := code[payloadLength+checksumLength]

	// Check version (compare characters)
	if string(version) != codeVersion {
		return "", false
	}

	// Decode and verify checksum (add correct base32 padding)
	checksumDecoded, err := base32.StdEncoding.DecodeString(checksumEncoded + "====") // Base32 needs 4-char padding
	if err != nil {
		return "", false
	}

	// Reconstruct stored checksum from 2 bytes
	storedChecksum := uint32(checksumDecoded[0])<<8 | uint32(checksumDecoded[1])

	calculatedChecksum := crc24Checksum([]byte(payload))

	// Compare only lower 16 bits
	storedChecksum16 := storedChecksum & 0xFFFF
	calculatedChecksum16 := calculatedChecksum & 0xFFFF
	if storedChecksum16 != calculatedChecksum16 {
		return "", false
	}

	return payload, true
}

// IsValid returns true if the promo code has valid checksum.
func (c Code) IsValid() bool {
	_, valid := ValidateCode(c)
	return valid
}
