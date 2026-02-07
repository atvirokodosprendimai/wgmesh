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
