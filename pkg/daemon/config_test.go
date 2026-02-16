package daemon

import "testing"

const testConfigSecret = "wgmesh-test-secret-long-enough-for-key-derivation"

func TestNewConfigLANDiscoveryDefaultEnabled(t *testing.T) {
	cfg, err := NewConfig(DaemonOpts{Secret: testConfigSecret})
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	if !cfg.LANDiscovery {
		t.Fatal("expected LANDiscovery to be enabled by default")
	}
}

func TestNewConfigDisableLANDiscovery(t *testing.T) {
	cfg, err := NewConfig(DaemonOpts{
		Secret:              testConfigSecret,
		DisableLANDiscovery: true,
	})
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	if cfg.LANDiscovery {
		t.Fatal("expected LANDiscovery to be disabled")
	}
}
