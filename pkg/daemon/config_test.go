package daemon

import (
	"reflect"
	"runtime"
	"strings"
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
	// Use an OS-appropriate custom name (macOS requires utunN pattern).
	customName := "custom0"
	if runtime.GOOS == "darwin" {
		customName = "utun99"
	}

	cfg, err := NewConfig(DaemonOpts{
		Secret:        testConfigSecret,
		InterfaceName: customName,
	})
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	if cfg.InterfaceName != customName {
		t.Errorf("expected interface %s, got %s", customName, cfg.InterfaceName)
	}
}

func TestNewConfigRejectsInvalidInterfaceName(t *testing.T) {
	_, err := NewConfig(DaemonOpts{
		Secret:        testConfigSecret,
		InterfaceName: "../evil",
	})
	if err == nil {
		t.Fatal("expected error for path-traversal interface name")
	}
}

// TestConfig_NoPunchingFlag verifies that the --no-punching flag is correctly
// parsed and stored in the daemon Config.
func TestConfig_NoPunchingFlag(t *testing.T) {
	cfg, err := NewConfig(DaemonOpts{
		Secret:          testConfigSecret,
		DisablePunching: true,
	})
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	if !cfg.DisablePunching {
		t.Fatal("expected DisablePunching to be true when --no-punching is set")
	}

	// Verify combining with ForceRelay is also valid (NAT environments may want both)
	cfg2, err := NewConfig(DaemonOpts{
		Secret:          testConfigSecret,
		DisablePunching: true,
		ForceRelay:      true,
	})
	if err != nil {
		t.Fatalf("NewConfig with ForceRelay+DisablePunching failed: %v", err)
	}
	if !cfg2.DisablePunching || !cfg2.ForceRelay {
		t.Fatal("expected both DisablePunching and ForceRelay to be set")
	}
}

func TestGenerateSecret(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error: %v", err)
	}
	// base64url of 128 bytes = 171 characters (no padding)
	const wantLen = 171
	if len(secret) != wantLen {
		t.Errorf("GenerateSecret() secret length = %d, want %d", len(secret), wantLen)
	}
	// Must not contain padding characters or non-base64url chars
	if strings.ContainsAny(secret, "=+/") {
		t.Errorf("GenerateSecret() secret contains non-base64url characters: %q", secret)
	}
	// Two calls must produce different secrets
	secret2, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() second call error: %v", err)
	}
	if secret == secret2 {
		t.Error("GenerateSecret() returned identical secrets on two consecutive calls")
	}
}

func TestNormalizeRoutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{"nil stays nil", nil, nil, false},
		{"valid", []string{"10.0.0.0/8", "192.168.1.0/24"}, []string{"10.0.0.0/8", "192.168.1.0/24"}, false},
		{"whitespace trimmed", []string{" 10.0.0.0/8 "}, []string{"10.0.0.0/8"}, false},
		{"host bits canonicalised", []string{"192.168.1.10/24"}, []string{"192.168.1.0/24"}, false},
		{"host route kept", []string{"10.0.0.5/32"}, []string{"10.0.0.5/32"}, false},
		{"empty and trailing comma dropped", []string{"10.0.0.0/8", "", " "}, []string{"10.0.0.0/8"}, false},
		{"only empty", []string{""}, []string{}, false},
		{"duplicates removed", []string{"10.0.0.0/8", "10.1.2.3/8", "10.0.0.0/8"}, []string{"10.0.0.0/8"}, false},
		{"ipv6", []string{"fd00:1::5/64", "2001:db8::/32"}, []string{"fd00:1::/64", "2001:db8::/32"}, false},
		{"bare ipv4 rejected", []string{"192.168.1.0"}, nil, true},
		{"bare ipv6 rejected", []string{"fd00::1"}, nil, true},
		{"garbage rejected", []string{"10.0.0.0/8", "bogus"}, nil, true},
		{"bad prefix rejected", []string{"10.0.0.0/33"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeRoutes(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeRoutes(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("normalizeRoutes(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNewConfigAdvertiseRoutes(t *testing.T) {
	t.Parallel()
	cfg, err := NewConfig(DaemonOpts{Secret: testConfigSecret, AdvertiseRoutes: []string{"192.168.1.10/24", ""}})
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}
	if want := []string{"192.168.1.0/24"}; !reflect.DeepEqual(cfg.AdvertiseRoutes, want) {
		t.Errorf("AdvertiseRoutes = %v, want %v", cfg.AdvertiseRoutes, want)
	}

	_, err = NewConfig(DaemonOpts{Secret: testConfigSecret, AdvertiseRoutes: []string{"192.168.1.0"}})
	if err == nil || !strings.Contains(err.Error(), "advertise-routes") {
		t.Fatalf("NewConfig with bare IP route: err = %v, want advertise-routes error", err)
	}
}
