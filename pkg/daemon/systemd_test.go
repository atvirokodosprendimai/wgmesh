package daemon

import (
	"strings"
	"testing"
)

func TestGenerateSystemdUnit(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:        "test-secret-that-is-long-enough",
		InterfaceName: "wg1",
		ListenPort:    51821,
		BinaryPath:    "/usr/local/bin/wgmesh",
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "wgmesh") {
		t.Error("Unit should contain 'wgmesh'")
	}
	if !strings.Contains(unit, "/usr/local/bin/wgmesh") {
		t.Error("Unit should contain binary path")
	}
	if !strings.Contains(unit, "--interface wg1") {
		t.Error("Unit should contain interface flag")
	}
	if !strings.Contains(unit, "--listen-port 51821") {
		t.Error("Unit should contain listen port flag")
	}
	if !strings.Contains(unit, "[Service]") {
		t.Error("Unit should contain [Service] section")
	}
	if !strings.Contains(unit, "EnvironmentFile") {
		t.Error("Unit should use EnvironmentFile for secret")
	}
	if !strings.Contains(unit, "${WGMESH_SECRET}") {
		t.Error("Unit should reference WGMESH_SECRET env var")
	}
	// Secret should NOT appear directly in the unit file
	if strings.Contains(unit, "test-secret-that-is-long-enough") {
		t.Error("Secret should not appear directly in unit file")
	}
	// NoNewPrivileges should be enabled
	if !strings.Contains(unit, "NoNewPrivileges=yes") {
		t.Error("Unit should have NoNewPrivileges=yes")
	}
}

func TestGenerateSystemdUnitDefaults(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:     "test-secret-that-is-long-enough",
		BinaryPath: "/usr/local/bin/wgmesh",
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	// Default interface and port should not be in args
	if strings.Contains(unit, "--interface wg0") {
		t.Error("Default interface should not be in args")
	}
	if strings.Contains(unit, "--listen-port 51820") {
		t.Error("Default port should not be in args")
	}
}

func TestGenerateSystemdUnitWithRoutes(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:          "test-secret-that-is-long-enough",
		BinaryPath:      "/usr/local/bin/wgmesh",
		AdvertiseRoutes: []string{"192.168.0.0/24", "10.0.0.0/8"},
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "--advertise-routes 192.168.0.0/24,10.0.0.0/8") {
		t.Error("Unit should contain advertise routes")
	}
}

func TestGenerateSystemdUnitWithPrivacy(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:     "test-secret-that-is-long-enough",
		BinaryPath: "/usr/local/bin/wgmesh",
		Privacy:    true,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "--privacy") {
		t.Error("Unit should contain --privacy flag when Privacy is true")
	}
}

func TestGenerateSystemdUnitWithoutPrivacy(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:     "test-secret-that-is-long-enough",
		BinaryPath: "/usr/local/bin/wgmesh",
		Privacy:    false,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if strings.Contains(unit, "--privacy") {
		t.Error("Unit should not contain --privacy flag when Privacy is false")
	}
}

func TestGenerateSystemdUnitWithGossip(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:     "test-secret-that-is-long-enough",
		BinaryPath: "/usr/local/bin/wgmesh",
		Gossip:     true,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "--gossip") {
		t.Error("Unit should contain --gossip flag when Gossip is true")
	}
}

func TestGenerateSystemdUnitWithoutGossip(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:     "test-secret-that-is-long-enough",
		BinaryPath: "/usr/local/bin/wgmesh",
		Gossip:     false,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if strings.Contains(unit, "--gossip") {
		t.Error("Unit should not contain --gossip flag when Gossip is false")
	}
}

func TestGenerateSystemdUnitWithNoLANDiscovery(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:              "test-secret-that-is-long-enough",
		BinaryPath:          "/usr/local/bin/wgmesh",
		DisableLANDiscovery: true,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "--no-lan-discovery") {
		t.Error("Unit should contain --no-lan-discovery flag when DisableLANDiscovery is true")
	}
}

func TestGenerateSystemdUnitWithLANDiscoveryDefault(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:              "test-secret-that-is-long-enough",
		BinaryPath:          "/usr/local/bin/wgmesh",
		DisableLANDiscovery: false,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if strings.Contains(unit, "--no-lan-discovery") {
		t.Error("Unit should not contain --no-lan-discovery when DisableLANDiscovery is false")
	}
}

func TestGenerateSystemdUnitWithIntroducer(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:     "test-secret-that-is-long-enough",
		BinaryPath: "/usr/local/bin/wgmesh",
		Introducer: true,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "--introducer") {
		t.Error("Unit should contain --introducer flag when Introducer is true")
	}
}

func TestGenerateSystemdUnitWithNoIPv6(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:      "test-secret-that-is-long-enough",
		BinaryPath:  "/usr/local/bin/wgmesh",
		DisableIPv6: true,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "--no-ipv6") {
		t.Error("Unit should contain --no-ipv6 flag when DisableIPv6 is true")
	}
}

func TestGenerateSystemdUnitWithForceRelayAndNoPunching(t *testing.T) {
	cfg := SystemdServiceConfig{
		Secret:          "test-secret-that-is-long-enough",
		BinaryPath:      "/usr/local/bin/wgmesh",
		ForceRelay:      true,
		DisablePunching: true,
	}

	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit failed: %v", err)
	}

	if !strings.Contains(unit, "--force-relay") {
		t.Error("Unit should contain --force-relay flag when ForceRelay is true")
	}
	if !strings.Contains(unit, "--no-punching") {
		t.Error("Unit should contain --no-punching flag when DisablePunching is true")
	}
}
