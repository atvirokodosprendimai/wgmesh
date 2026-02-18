package daemon

import (
	"runtime"
	"testing"
)

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

func TestNewConfigIntroducer(t *testing.T) {
	cfg, err := NewConfig(DaemonOpts{
		Secret:     testConfigSecret,
		Introducer: true,
	})
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	if !cfg.Introducer {
		t.Fatal("expected Introducer to be enabled")
	}
}

func TestNewConfigDisableIPv6(t *testing.T) {
	cfg, err := NewConfig(DaemonOpts{
		Secret:      testConfigSecret,
		DisableIPv6: true,
	})
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	if !cfg.DisableIPv6 {
		t.Fatal("expected DisableIPv6 to be enabled")
	}
}

func TestNewConfigForceRelayAndNoPunching(t *testing.T) {
	cfg, err := NewConfig(DaemonOpts{
		Secret:          testConfigSecret,
		ForceRelay:      true,
		DisablePunching: true,
	})
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	if !cfg.ForceRelay {
		t.Fatal("expected ForceRelay to be enabled")
	}
	if !cfg.DisablePunching {
		t.Fatal("expected DisablePunching to be enabled")
	}
}

func TestNewConfigDefaultInterfaceName(t *testing.T) {
	cfg, err := NewConfig(DaemonOpts{Secret: testConfigSecret})
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	expected := DefaultInterface
	if runtime.GOOS == "darwin" {
		expected = DefaultInterfaceDarwin
	}

	if cfg.InterfaceName != expected {
		t.Errorf("expected interface %s on %s, got %s", expected, runtime.GOOS, cfg.InterfaceName)
	}
}

func TestNewConfigExplicitInterfaceName(t *testing.T) {
	cfg, err := NewConfig(DaemonOpts{
		Secret:        testConfigSecret,
		InterfaceName: "custom0",
	})
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	if cfg.InterfaceName != "custom0" {
		t.Errorf("expected interface custom0, got %s", cfg.InterfaceName)
	}
}
