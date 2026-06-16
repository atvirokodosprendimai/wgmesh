package account

import (
	"strings"
	"testing"
)

func TestGenerateCode(t *testing.T) {
	tests := []struct {
		name      string
		accountID AccountID
		wantLen   int
	}{
		{
			name:      "valid account ID",
			accountID: AccountID("550e8400-e29b-41d4-a716-446655440000"),
			wantLen:   CodeLength,
		},
		{
			name:      "another account ID",
			accountID: AccountID("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
			wantLen:   CodeLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := GenerateCode(tt.accountID)
			if err != nil {
				t.Fatalf("GenerateCode() error = %v", err)
			}
			if len(code) != tt.wantLen {
				t.Errorf("GenerateCode() len = %d, want %d", len(code), tt.wantLen)
			}
		})
	}
}

func TestGenerateCodeUnique(t *testing.T) {
	// Generate 1000 codes and verify no collisions
	codes := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		accountID := AccountID("test-account-" + string(rune(i)))
		code, err := GenerateCode(accountID)
		if err != nil {
			t.Fatalf("GenerateCode() error = %v", err)
		}
		if codes[string(code)] {
			t.Errorf("Code collision detected: %s", code)
		}
		codes[string(code)] = true
	}
}

func TestGenerateCodeDeterministic(t *testing.T) {
	accountID := AccountID("550e8400-e29b-41d4-a716-446655440000")

	code1, err1 := GenerateCode(accountID)
	code2, err2 := GenerateCode(accountID)

	if err1 != nil || err2 != nil {
		t.Fatalf("GenerateCode() error = %v, %v", err1, err2)
	}

	if code1 != code2 {
		t.Error("GenerateCode() is not deterministic")
	}
}

func TestValidateCode(t *testing.T) {
	accountID := AccountID("550e8400-e29b-41d4-a716-446655440000")
	code, err := GenerateCode(accountID)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	// Validate the generated code
	validatedID, err := ValidateCode(code)
	if err != nil {
		t.Errorf("ValidateCode() error = %v", err)
	}

	// The validated ID should be the code itself (since we can't reverse HKDF)
	if validatedID != AccountID(code) {
		t.Errorf("ValidateCode() returned unexpected ID")
	}
}

func TestValidateCodeChecksum(t *testing.T) {
	accountID := AccountID("550e8400-e29b-41d4-a716-446655440000")
	code, err := GenerateCode(accountID)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	// Test single-character errors (typo detection)
	tests := []struct {
		name    string
		modify  func(ReferralCode) ReferralCode
		wantErr bool
	}{
		{
			name: "first char error",
			modify: func(c ReferralCode) ReferralCode {
				if c[0] == 'A' {
					return ReferralCode("B" + string(c[1:]))
				}
				return ReferralCode("A" + string(c[1:]))
			},
			wantErr: true,
		},
		{
			name: "middle char error",
			modify: func(c ReferralCode) ReferralCode {
				mid := len(c) / 2
				if c[mid] == 'A' {
					return ReferralCode(string(c[:mid]) + "B" + string(c[mid+1:]))
				}
				return ReferralCode(string(c[:mid]) + "A" + string(c[mid+1:]))
			},
			wantErr: true,
		},
		{
			name: "last checksum char error",
			modify: func(c ReferralCode) ReferralCode {
				// Modify the last checksum character (index 10)
				if c[10] == 'A' {
					return ReferralCode(string(c[:10]) + "B" + string(c[11:]))
				}
				return ReferralCode(string(c[:10]) + "A" + string(c[11:]))
			},
			wantErr: true,
		},
		{
			name: "no error",
			modify: func(c ReferralCode) ReferralCode {
				return c
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modifiedCode := tt.modify(code)
			_, err := ValidateCode(modifiedCode)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCodeInvalidLength(t *testing.T) {
	tests := []struct {
		name    string
		code    ReferralCode
		wantErr error
	}{
		{
			name:    "too short",
			code:    "ABC123",
			wantErr: ErrInvalidCodeFormat,
		},
		{
			name:    "too long",
			code:    ReferralCode(strings.Repeat("A", 20)),
			wantErr: ErrInvalidCodeFormat,
		},
		{
			name:    "empty",
			code:    "",
			wantErr: ErrInvalidCodeFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateCode(tt.code)
			if err != tt.wantErr {
				t.Errorf("ValidateCode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCodeInvalidVersion(t *testing.T) {
	// Create a valid code and change the version
	accountID := AccountID("550e8400-e29b-41d4-a716-446655440000")
	code, err := GenerateCode(accountID)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	// Change version byte
	invalidCode := ReferralCode(string(code[:11]) + "X")

	_, err = ValidateCode(invalidCode)
	if err != ErrInvalidVersion {
		t.Errorf("ValidateCode() error = %v, wantErr %v", err, ErrInvalidVersion)
	}
}

func TestCodeURLSafe(t *testing.T) {
	accountID := AccountID("550e8400-e29b-41d4-a716-446655440000")
	code, err := GenerateCode(accountID)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	// Check that code only contains URL-safe characters (base32 + version digit)
	for i, c := range code {
		isLastChar := (i == len(code)-1)
		if isLastChar {
			// Version byte can be a digit
			if c < '0' || c > '9' {
				t.Errorf("Version byte contains invalid character: %c", c)
			}
		} else {
			// Base32 characters
			if !((c >= 'A' && c <= 'Z') || (c >= '2' && c <= '7')) {
				t.Errorf("Code contains non-URL-safe character: %c at position %d", c, i)
			}
		}
	}
}

func TestCodeCaseInsensitive(t *testing.T) {
	accountID := AccountID("550e8400-e29b-41d4-a716-446655440000")
	code, err := GenerateCode(accountID)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	// Convert to lowercase and validate
	lowerCode := ReferralCode(strings.ToLower(string(code)))
	_, err = ValidateCode(lowerCode)
	if err == nil {
		t.Error("ValidateCode() should reject lowercase code")
	}

	// Note: Base32 is case-sensitive, so lowercase codes should fail
	// This test verifies that the validation is working correctly
}
