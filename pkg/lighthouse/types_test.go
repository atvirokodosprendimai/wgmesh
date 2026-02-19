package lighthouse

import (
	"strings"
	"testing"
)

func TestGenerateID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prefix string
	}{
		{"org prefix", "org"},
		{"site prefix", "site"},
		{"key prefix", "key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GenerateID(tt.prefix)
			if !strings.HasPrefix(id, tt.prefix+"_") {
				t.Errorf("GenerateID(%q) = %q, want prefix %q_", tt.prefix, id, tt.prefix)
			}
			// 12 random bytes = 24 hex chars + prefix + underscore
			expectedLen := len(tt.prefix) + 1 + 24
			if len(id) != expectedLen {
				t.Errorf("GenerateID(%q) length = %d, want %d", tt.prefix, len(id), expectedLen)
			}
		})
	}

	// Uniqueness
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateID("test")
		if ids[id] {
			t.Errorf("GenerateID produced duplicate: %s", id)
		}
		ids[id] = true
	}
}

func TestGenerateAPIKey(t *testing.T) {
	t.Parallel()

	key := GenerateAPIKey()
	if !strings.HasPrefix(key, "cr_") {
		t.Errorf("GenerateAPIKey() = %q, want cr_ prefix", key)
	}
	// "cr_" + 64 hex chars (32 bytes)
	if len(key) != 3+64 {
		t.Errorf("GenerateAPIKey() length = %d, want %d", len(key), 3+64)
	}
}

func TestHashKey(t *testing.T) {
	t.Parallel()

	key := "cr_abc123def456"
	hash1 := HashKey(key)
	hash2 := HashKey(key)

	if hash1 != hash2 {
		t.Error("HashKey not deterministic")
	}

	// Different keys produce different hashes
	hash3 := HashKey("cr_different_key")
	if hash1 == hash3 {
		t.Error("Different keys produced same hash")
	}

	// Hash is hex-encoded SHA-256 (64 chars)
	if len(hash1) != 64 {
		t.Errorf("HashKey length = %d, want 64", len(hash1))
	}
}

func TestPrefixFromKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rawKey  string
		wantPfx string
	}{
		{"standard key", "cr_abcdefghijklmnop", "abcdefgh"},
		{"without prefix", "abcdefghijklmnop", "abcdefgh"},
		{"short key", "cr_abc", "abc"},
		{"exactly 8", "cr_12345678", "12345678"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PrefixFromKey(tt.rawKey)
			if got != tt.wantPfx {
				t.Errorf("PrefixFromKey(%q) = %q, want %q", tt.rawKey, got, tt.wantPfx)
			}
		})
	}
}
