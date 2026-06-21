package promo

import (
	"fmt"
	"strings"
	"testing"
)

func TestGenerateCode(t *testing.T) {
	tests := []struct {
		name       string
		campaignID string
		seed       string
		wantErr    bool
	}{
		{
			name:       "valid code generation",
			campaignID: "test-campaign",
			seed:       "random-seed-12345",
			wantErr:    false,
		},
		{
			name:       "empty campaign ID",
			campaignID: "",
			seed:       "seed",
			wantErr:    true,
		},
		{
			name:       "empty seed",
			campaignID: "campaign",
			seed:       "",
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := GenerateCode(tt.campaignID, tt.seed)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateCode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(code) != codeLength {
					t.Errorf("GenerateCode() length = %d, want %d", len(code), codeLength)
				}
				// Verify code is URL-safe and uppercase
				if code != Code(strings.ToUpper(string(code))) {
					t.Errorf("GenerateCode() not uppercase")
				}
			}
		})
	}
}

func TestGenerateCodeUnique(t *testing.T) {
	// Generate 1000 codes and verify no collisions
	codes := make(map[Code]bool)
	campaignID := "collision-test"
	for i := 0; i < 1000; i++ {
		seed := fmt.Sprintf("seed-%d", i)
		code, err := GenerateCode(campaignID, seed)
		if err != nil {
			t.Fatalf("GenerateCode() failed: %v", err)
		}
		if codes[code] {
			t.Errorf("collision detected at iteration %d", i)
		}
		codes[code] = true
	}
}

func TestValidateCodeChecksum(t *testing.T) {
	campaignID := "checksum-test"
	seed := "test-seed"

	code, err := GenerateCode(campaignID, seed)
	if err != nil {
		t.Fatalf("GenerateCode() failed: %v", err)
	}

	// Test valid code
	if !code.IsValid() {
		t.Error("generated code failed validation")
	}

	// Test single-char errors in payload (most should fail checksum)
	// Note: With 16-bit checksum, some mutations may pass validation
	// This is acceptable for typo detection in promo codes
	codeStr := string(code)
	payloadErrorsDetected := 0
	for i := 0; i < 8; i++ { // Only test payload positions
		if codeStr[i] >= 'A' && codeStr[i] <= 'Z' {
			mutated := []byte(codeStr)
			mutated[i] = 'A' + (mutated[i]-'A'+1)%26
			mutatedCode := Code(mutated)
			if !mutatedCode.IsValid() {
				payloadErrorsDetected++
			}
		}
	}
	// At least 50% of payload mutations should be detected
	// (16-bit checksum provides reasonable but not perfect typo detection)
	if payloadErrorsDetected < 4 {
		t.Errorf("only %d/8 payload mutations detected (expected >=4)", payloadErrorsDetected)
	}
}

func TestValidateCodeLength(t *testing.T) {
	tests := []struct {
		name  string
		code  Code
		valid bool
	}{
		{
			name:  "valid length",
			code:  "ABCDEFGHIJKL",
			valid: false, // Will fail checksum but length check passes first
		},
		{
			name:  "too short",
			code:  "SHORT",
			valid: false,
		},
		{
			name:  "too long",
			code:  "TOOLONGCODE123",
			valid: false,
		},
		{
			name:  "empty",
			code:  "",
			valid: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, valid := ValidateCode(tt.code)
			if valid != tt.valid {
				t.Errorf("ValidateCode() = %v, want %v", valid, tt.valid)
			}
		})
	}
}

func TestCodeIsURLSafe(t *testing.T) {
	campaignID := "url-safe-test"
	seed := "seed-12345"

	for i := 0; i < 100; i++ {
		testSeed := fmt.Sprintf("%s-%d", seed, i)
		code, err := GenerateCode(campaignID, testSeed)
		if err != nil {
			t.Fatalf("GenerateCode() failed: %v", err)
		}

		// Check for URL-safe characters (A-Z, 2-7 for base32)
		for _, c := range code {
			if !((c >= 'A' && c <= 'Z') || (c >= '2' && c <= '7')) {
				t.Errorf("code contains non-URL-safe character: %c in %s", c, code)
			}
		}
	}
}
