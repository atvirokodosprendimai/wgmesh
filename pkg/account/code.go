package account

import (
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	// CodeFormatVersion is the current version byte for referral codes
	CodeFormatVersion = '1'

	// CodeLength is the total length of a referral code
	CodeLength = 12

	// checksumLength is the length of the CRC24 checksum in base32 characters
	checksumLength = 3

	// hkdfInfoReferral is the HKDF info string for referral code derivation
	hkdfInfoReferral = "wgmesh-referral-v1"
)

var (
	// ErrInvalidCodeFormat is returned when the code format is invalid
	ErrInvalidCodeFormat = errors.New("invalid referral code format")

	// ErrInvalidChecksum is returned when the code checksum is invalid
	ErrInvalidChecksum = errors.New("invalid referral code checksum")

	// ErrInvalidVersion is returned when the code version is not supported
	ErrInvalidVersion = errors.New("unsupported referral code version")
)

// crc24Table is the CRC-24 table for IEEE polynomial
var crc24Table = func() [256]uint32 {
	var table [256]uint32
	poly := uint32(0x864cfb)
	for i := 0; i < 256; i++ {
		crc := uint32(i) << 16
		for j := 0; j < 8; j++ {
			if (crc^poly)&0x800000 != 0 {
				crc = (crc << 1) ^ poly
			} else {
				crc <<= 1
			}
		}
		table[i] = crc & 0xffffff
	}
	return table
}()

// GenerateCode creates a unique referral code for an account
// Format: [base32(account_id_hash)][checksum][version]
// Length: 12 characters, URL-safe, case-insensitive
func GenerateCode(accountID AccountID) (ReferralCode, error) {
	// Derive 8 bytes using HKDF
	var derived [8]byte
	reader := hkdf.New(sha256.New, []byte(string(accountID)), nil, []byte(hkdfInfoReferral))
	if _, err := io.ReadFull(reader, derived[:]); err != nil {
		return "", fmt.Errorf("HKDF derivation failed: %w", err)
	}

	// Encode to base32 (8 bytes → 13 chars, we need only 8)
	encoded := base32.StdEncoding.EncodeToString(derived[:]) // 13 chars
	codePart := encoded[:8]                                  // Take first 8 chars

	// Calculate checksum over code part
	checksum := crc24Checksum([]byte(codePart))

	// Encode checksum to base32 (3 bytes → 5 chars, we need only 3)
	checksumEncoded := base32.StdEncoding.EncodeToString(checksum)[:3]

	// Build final code: codePart (8) + checksum (3) + version (1) = 12 chars
	code := codePart + checksumEncoded + string(CodeFormatVersion)

	return ReferralCode(code), nil
}

// ValidateCode verifies checksum and extracts account ID
func ValidateCode(code ReferralCode) (AccountID, error) {
	// Check length
	if len(code) != CodeLength {
		return "", ErrInvalidCodeFormat
	}

	// Extract parts
	codePart := string(code[:8])       // Base32 hash
	checksumPart := string(code[8:11]) // Base32 checksum
	versionPart := string(code[11])    // Version byte

	// Validate version
	if versionPart != string(CodeFormatVersion) {
		return "", ErrInvalidVersion
	}

	// Validate checksum
	expectedChecksum := crc24Checksum([]byte(codePart))
	expectedChecksumEncoded := base32.StdEncoding.EncodeToString(expectedChecksum)[:3]

	if checksumPart != expectedChecksumEncoded {
		return "", ErrInvalidChecksum
	}

	// The code is valid; return the code itself as the AccountID placeholder
	// The actual account lookup will be done in the store
	return AccountID(code), nil
}

// crc24Checksum calculates a CRC24 checksum for error detection
func crc24Checksum(data []byte) []byte {
	crc := uint32(0xFFFFFF)
	for _, b := range data {
		crc = (crc >> 8) ^ crc24Table[(crc^uint32(b))&0xFF]
	}
	crc ^= 0xFFFFFF

	result := make([]byte, 3)
	result[0] = byte(crc >> 16)
	result[1] = byte(crc >> 8)
	result[2] = byte(crc)
	return result
}
