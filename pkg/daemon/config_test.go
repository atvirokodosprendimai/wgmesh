package daemon

import (
	"runtime"
	"testing"
)

func TestNewConfig_DefaultInterfaceName(t *testing.T) {
	// Generate a test secret
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("Failed to generate secret: %v", err)
	}

	// Create config with no interface name specified
	cfg, err := NewConfig(DaemonOpts{
		Secret: secret,
	})
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Verify the interface name is set to the correct default
	expectedInterface := DefaultInterface
	if runtime.GOOS == "darwin" {
		expectedInterface = DefaultInterfaceDarwin
	}

	if cfg.InterfaceName != expectedInterface {
		t.Errorf("Expected interface name %s on %s, got %s", expectedInterface, runtime.GOOS, cfg.InterfaceName)
	}
}

func TestNewConfig_ExplicitInterfaceName(t *testing.T) {
	// Generate a test secret
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("Failed to generate secret: %v", err)
	}

	// Create config with explicit interface name
	explicitInterface := "custom-interface"
	cfg, err := NewConfig(DaemonOpts{
		Secret:        secret,
		InterfaceName: explicitInterface,
	})
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Verify the explicit interface name is used
	if cfg.InterfaceName != explicitInterface {
		t.Errorf("Expected interface name %s, got %s", explicitInterface, cfg.InterfaceName)
	}
}
