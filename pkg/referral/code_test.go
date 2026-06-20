package referral

import (
	"testing"
)

func TestGenerateFormat(t *testing.T) {
	code, err := Generate()
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}
	if !Validate(code) {
		t.Errorf("Generated code failed validation: %s", code)
	}
	// Check format: XXXXX-XXXXX → 5 + 1 + 5 = 11 chars.
	if len(code) != 11 {
		t.Errorf("Wrong length: got %d, want 11", len(code))
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		{"ABCDE-FGHIJ", true},        // Valid
		{"abcde-fghij", false},       // Lowercase invalid
		{"AB-DEFGHIJ", false},        // Wrong length first part
		{"ABCDE-FGH", false},         // Wrong length second part
		{"ABC DE-FGHIJ", false},      // Space invalid
		{"", false},                  // Empty
		{"ABCDE-FGHI2", true},        // Digits in base32 set are valid
		{"ABCDE-FGHI1", false},       // '1' is not in base32 set
		{"ABCDE-FGHIJ-KLMNO", false}, // Too many parts
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := Validate(tt.code)
			if got != tt.want {
				t.Errorf("Validate(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestCollisionResistance(t *testing.T) {
	// Generate 10,000 codes and check for duplicates.
	// Monte Carlo test for collision resistance.
	generated := make(map[string]bool)
	iterations := 10000
	for i := 0; i < iterations; i++ {
		code, err := Generate()
		if err != nil {
			t.Fatalf("Generate() failed: %v", err)
		}
		if generated[code] {
			t.Fatalf("Collision detected at iteration %d: %s", i, code)
		}
		generated[code] = true
	}
	// If we get here, no collisions in 10k iterations (acceptable for 10^16 space).
}
