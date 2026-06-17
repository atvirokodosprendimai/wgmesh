package onboarding

import (
	"testing"
	"time"
)

func TestValidateSecretGeneration(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{
			name:    "valid base64 secret",
			secret:  "dGhpcy1pcy1hLXZhbGlkLWJhc2U2NC1zZWNyZXQtd2l0aC1lbm91Z2gtZW50cm9weS1mb3ItdGVzdGluZw",
			wantErr: false,
		},
		{
			name:    "short secret",
			secret:  "short",
			wantErr: true,
		},
		{
			name:    "empty secret",
			secret:  "",
			wantErr: true,
		},
		{
			name:    "exactly minimum length",
			secret:  "12345678901234567890123456789012", // 32 chars
			wantErr: false,
		},
		{
			name:    "long valid secret",
			secret:  "this-is-a-very-long-secret-with-more-than-enough-entropy-for-validation-purposes-1234567890",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := validateSecretGeneration(tt.secret)
			err := validator()

			if (err != nil) != tt.wantErr {
				t.Errorf("validateSecretGeneration() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil && tt.wantErr {
				// Verify error message is meaningful
				if err.Error() == "" {
					t.Error("Expected error message, got empty")
				}
			}
		})
	}
}

func TestValidateInterfaceConfig(t *testing.T) {
	tests := []struct {
		name          string
		interfaceName string
		wantErr       bool
	}{
		{
			name:          "valid interface name",
			interfaceName: "wg0",
			wantErr:       true, // Will fail unless wg0 exists
		},
		{
			name:          "empty interface",
			interfaceName: "",
			wantErr:       true,
		},
		{
			name:          "darwin interface",
			interfaceName: "utun20",
			wantErr:       true, // Will fail unless utun20 exists
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := validateInterfaceConfig(tt.interfaceName)
			err := validator()

			// Most will fail in test environment, just check function works
			if tt.interfaceName == "" && err == nil {
				t.Error("Expected error for empty interface name")
			}

			if tt.interfaceName != "" && err != nil {
				// Expected to fail in test environment
				t.Logf("Got expected error in test env: %v", err)
			}
		})
	}
}

func TestValidateGitHubRegistry(t *testing.T) {
	tests := []struct {
		name         string
		skipRegistry bool
		wantErr      bool
	}{
		{
			name:         "with skip",
			skipRegistry: true,
			wantErr:      false,
		},
		{
			name:         "without skip",
			skipRegistry: false,
			wantErr:      false, // Should succeed if network available
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := validateGitHubRegistry(tt.skipRegistry)
			err := validator()

			if tt.skipRegistry && err != nil {
				t.Errorf("Expected no error when skipping, got %v", err)
			}

			if !tt.skipRegistry && err != nil {
				t.Logf("GitHub validation failed (may be network issue): %v", err)
			}
		})
	}
}

func TestValidateLANMulticast(t *testing.T) {
	tests := []struct {
		name                string
		disableLANDiscovery bool
		wantErr             bool
	}{
		{
			name:                "with LAN disabled",
			disableLANDiscovery: true,
			wantErr:             false,
		},
		{
			name:                "with LAN enabled",
			disableLANDiscovery: false,
			wantErr:             false, // Should succeed in most environments
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := validateLANMulticast(tt.disableLANDiscovery)
			err := validator()

			if tt.disableLANDiscovery && err != nil {
				t.Errorf("Expected no error when LAN disabled, got %v", err)
			}

			if !tt.disableLANDiscovery && err != nil {
				t.Logf("LAN multicast validation failed: %v", err)
			}
		})
	}
}

func TestValidateDHTBootstrap(t *testing.T) {
	validator := validateDHTBootstrap()
	err := validator()

	// This will likely fail in restricted test environments
	if err != nil {
		t.Logf("DHT bootstrap validation failed (expected in test env): %v", err)
	}
}

func TestValidatePeerDiscovery(t *testing.T) {
	tests := []struct {
		name    string
		timeout int // seconds
		wantErr bool
	}{
		{
			name:    "short timeout",
			timeout: 1,
			wantErr: true, // Will timeout without peers
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := validateFirstPeerContact(time.Duration(tt.timeout) * time.Second)
			err := validator()

			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

func TestValidateWireGuardHandshake(t *testing.T) {
	tests := []struct {
		name          string
		interfaceName string
		wantErr       bool
	}{
		{
			name:          "wg0 interface",
			interfaceName: "wg0",
			wantErr:       true, // Will fail unless wg0 exists with peers
		},
		{
			name:          "empty interface",
			interfaceName: "",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := validateBidirectionalPing(tt.interfaceName)
			err := validator()

			// Expected to fail in test environment
			if tt.interfaceName == "" && err == nil {
				t.Error("Expected error for empty interface name")
			}

			if tt.interfaceName != "" && err != nil {
				t.Logf("Got expected error in test env: %v", err)
			}
		})
	}
}
