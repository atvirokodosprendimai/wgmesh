package referral

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"strings"
)

// ErrNotFound is returned by Store implementations when a referral record
// cannot be located. It is defined in the public interface package so that
// backend implementations (which live in a private repository) and callers can
// agree on a sentinel "not found" value without importing the backend.
var ErrNotFound = errors.New("referral: not found")

// Generate creates a new share code in the format XXXXX-XXXXX using
// cryptographically secure randomness. base32 (RFC 4648) is used for
// human-readability and collision resistance.
func Generate() (string, error) {
	// 10 random bytes → 16 base32 chars → use 10 for the XXXXX-XXXXX format.
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	// Base32 encode without padding.
	encoded := base32.StdEncoding.EncodeToString(buf)
	encoded = strings.TrimRight(encoded, "=")

	// Take first 10 chars, insert a dash at position 5.
	if len(encoded) < 10 {
		// Fallback: repeat the prefix to reach 10 chars. This branch is
		// effectively unreachable because 10 random bytes always yield 16
		// base32 characters, but it keeps the function defensive.
		encoded = encoded + encoded[:10-len(encoded)]
	}

	return encoded[:5] + "-" + encoded[5:10], nil
}

// Validate checks whether a code is in the valid XXXXX-XXXXX format using only
// the base32 character set (A–Z and 2–7).
func Validate(code string) bool {
	parts := strings.Split(code, "-")
	if len(parts) != 2 {
		return false
	}
	if len(parts[0]) != 5 || len(parts[1]) != 5 {
		return false
	}
	// Must use only the base32 character set.
	for _, part := range parts {
		for _, r := range part {
			if !((r >= 'A' && r <= 'Z') || (r >= '2' && r <= '7')) {
				return false
			}
		}
	}
	return true
}
