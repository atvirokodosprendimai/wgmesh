package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"wgmesh": main,
	})
}

func TestScript(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:           "testdata/script",
		UpdateScripts: os.Getenv("WGMESH_UPDATE_GOLDEN") != "",
	})
}

func TestVersionFlag(t *testing.T) {
	// Build the binary for testing
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove("/tmp/wgmesh-test")

	tests := []struct {
		name string
		args []string
	}{
		{"version subcommand", []string{"version"}},
		{"--version flag", []string{"--version"}},
		{"-v flag", []string{"-v"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("/tmp/wgmesh-test", tt.args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Command failed: %v, output: %s", err, output)
			}

			result := strings.TrimSpace(string(output))
			if !strings.HasPrefix(result, "wgmesh ") {
				t.Errorf("Expected output to start with 'wgmesh ', got: %s", result)
			}

			// Verify it has a version after "wgmesh "
			parts := strings.Split(result, " ")
			if len(parts) < 2 {
				t.Errorf("Expected output format 'wgmesh <version>', got: %s", result)
			}
		})
	}
}

func TestVersionFlagPriority(t *testing.T) {
	// Build the binary for testing
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove("/tmp/wgmesh-test")

	tests := []struct {
		name string
		args []string
	}{
		{"version with other flags", []string{"--version", "--help"}},
		{"version with subcommand", []string{"-v", "join"}},
		{"version with init flag", []string{"--version", "-init"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("/tmp/wgmesh-test", tt.args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Command failed: %v, output: %s", err, output)
			}

			result := strings.TrimSpace(string(output))
			if !strings.HasPrefix(result, "wgmesh ") {
				t.Errorf("Expected version output, got: %s", result)
			}
			// Ensure it doesn't show help or try to run other commands
			if strings.Contains(result, "SUBCOMMANDS") || strings.Contains(result, "FLAGS") {
				t.Errorf("Version flag should not show help, got: %s", result)
			}
			if strings.Contains(result, "Usage:") {
				t.Errorf("Version flag should not show usage, got: %s", result)
			}
			if strings.Contains(result, "Error:") {
				t.Errorf("Version flag should not show errors, got: %s", result)
			}
		})
	}
}

func TestVersionFlagExitCode(t *testing.T) {
	// Build the binary for testing
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove("/tmp/wgmesh-test")

	tests := []struct {
		name string
		args []string
	}{
		{"version subcommand exit code", []string{"version"}},
		{"--version flag exit code", []string{"--version"}},
		{"-v flag exit code", []string{"-v"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("/tmp/wgmesh-test", tt.args...)
			err := cmd.Run()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					t.Errorf("Command exited with non-zero code %d, output: %s",
						exitErr.ExitCode(), exitErr.Stderr)
				} else {
					t.Errorf("Command failed: %v", err)
				}
			}
		})
	}
}

func TestVersionFormatConsistency(t *testing.T) {
	// Build the binary for testing
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove("/tmp/wgmesh-test")

	// Get output from all three forms
	var outputs []string

	for _, args := range [][]string{{"version"}, {"--version"}, {"-v"}} {
		cmd := exec.Command("/tmp/wgmesh-test", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Command failed: %v, output: %s", err, output)
		}
		outputs = append(outputs, strings.TrimSpace(string(output)))
	}

	// All three should produce identical output
	for i := 1; i < len(outputs); i++ {
		if outputs[i] != outputs[0] {
			t.Errorf("Version output inconsistency:\n  %s = %s\n  %s = %s",
				[]string{"version", "--version", "-v"}[0], outputs[0],
				[]string{"version", "--version", "-v"}[i], outputs[i])
		}
	}
}

func TestStatusJSONOutput(t *testing.T) {
	// Build the binary for testing
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove("/tmp/wgmesh-test")

	// Test secret (long enough for key derivation)
	testSecret := "wgmesh://v1/SGVsbG8gV29ybGQhIFRoaXMgaXMgYSB0ZXN0IHNlY3JldCB0aGF0IGlzIGxvbmcgZW5vdWdoIGZvciB0aGUga2V5IGRlcml2YXRpb24u"

	cmd := exec.Command("/tmp/wgmesh-test", "status", "--secret", testSecret, "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %v, output: %s", err, output)
	}

	// Parse JSON output
	var status StatusOutput
	if err := json.Unmarshal(output, &status); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	// Verify required fields are present
	if status.Interface == "" {
		t.Error("Interface field is empty")
	}
	if status.NetworkID == "" {
		t.Error("NetworkID field is empty")
	}
	if status.MeshSubnet == "" {
		t.Error("MeshSubnet field is empty")
	}
	if status.MeshIPv6Prefix == "" {
		t.Error("MeshIPv6Prefix field is empty")
	}
	if status.GossipPort == 0 {
		t.Error("GossipPort field is zero")
	}
	if status.RendezvousID == "" {
		t.Error("RendezvousID field is empty")
	}

	// Verify format expectations
	if !strings.Contains(status.MeshSubnet, "/") {
		t.Errorf("MeshSubnet should be CIDR format, got: %s", status.MeshSubnet)
	}
	if !strings.Contains(status.MeshIPv6Prefix, "/") {
		t.Errorf("MeshIPv6Prefix should contain CIDR, got: %s", status.MeshIPv6Prefix)
	}
}

func TestStatusTextOutput(t *testing.T) {
	// Build the binary for testing
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove("/tmp/wgmesh-test")

	// Test secret
	testSecret := "wgmesh://v1/SGVsbG8gV29ybGQhIFRoaXMgaXMgYSB0ZXN0IHNlY3JldCB0aGF0IGlzIGxvbmcgZW5vdWdoIGZvciB0aGUga2V5IGRlcml2YXRpb24u"

	cmd := exec.Command("/tmp/wgmesh-test", "status", "--secret", testSecret)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %v, output: %s", err, output)
	}

	outputStr := string(output)

	// Verify text format output contains expected headers and fields
	expectedStrings := []string{
		"Mesh Status",
		"===========",
		"Interface:",
		"Network ID:",
		"Mesh Subnet:",
		"Mesh IPv6 Prefix:",
		"Gossip Port:",
		"Rendezvous ID:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(outputStr, expected) {
			t.Errorf("Expected output to contain %q, but it was missing", expected)
		}
	}

	// Verify it does NOT contain JSON markers
	if strings.Contains(outputStr, "{") || strings.Contains(outputStr, "}") {
		t.Error("Text output should not contain JSON braces")
	}
}

func TestStatusJSONWithoutSecret(t *testing.T) {
	// Build the binary for testing
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove("/tmp/wgmesh-test")

	// Test that --json without secret fails
	cmd := exec.Command("/tmp/wgmesh-test", "status", "--json")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected error when --secret is missing, but command succeeded")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "--secret is required") {
		t.Errorf("Expected error message about missing secret, got: %s", outputStr)
	}
	// Should go to stderr, not stdout
	if strings.Contains(outputStr, "{") {
		t.Error("Should not output JSON when secret is missing")
	}
}

func TestStatusCustomSubnet(t *testing.T) {
	// Build the binary for testing
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove("/tmp/wgmesh-test")

	// Test secret
	testSecret := "wgmesh://v1/SGVsbG8gV29ybGQhIFRoaXMgaXMgYSB0ZXN0IHNlY3JldCB0aGF0IGlzIGxvbmcgZW5vdWdoIGZvciB0aGUga2V5IGRlcml2YXRpb24u"
	customSubnet := "192.168.100.0/24"

	tests := []struct {
		name     string
		args     []string
		wantJSON bool
	}{
		{"custom subnet JSON", []string{"status", "--secret", testSecret, "--mesh-subnet", customSubnet, "--json"}, true},
		{"custom subnet text", []string{"status", "--secret", testSecret, "--mesh-subnet", customSubnet}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("/tmp/wgmesh-test", tt.args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Command failed: %v, output: %s", err, output)
			}

			outputStr := string(output)

			if tt.wantJSON {
				var status StatusOutput
				if err := json.Unmarshal(output, &status); err != nil {
					t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
				}
				if status.MeshSubnet != customSubnet {
					t.Errorf("Expected custom subnet %s, got %s", customSubnet, status.MeshSubnet)
				}
			} else {
				if !strings.Contains(outputStr, customSubnet) {
					t.Errorf("Expected text output to contain custom subnet %s", customSubnet)
				}
			}
		})
	}
}

func TestStatusBackwardCompatibility(t *testing.T) {
	// Build the binary for testing
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove("/tmp/wgmesh-test")

	// Test secret
	testSecret := "wgmesh://v1/SGVsbG8gV29ybGQhIFRoaXMgaXMgYSB0ZXN0IHNlY3JldCB0aGF0IGlzIGxvbmcgZW5vdWdoIGZvciB0aGUga2V5IGRlcml2YXRpb24u"

	// Test that default behavior (no --json) produces text output
	cmd := exec.Command("/tmp/wgmesh-test", "status", "--secret", testSecret)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %v, output: %s", err, output)
	}

	outputStr := string(output)

	// Should contain text format markers
	if !strings.Contains(outputStr, "Mesh Status") {
		t.Error("Default output should be text format with 'Mesh Status' header")
	}
	if !strings.Contains(outputStr, "===========") {
		t.Error("Default output should be text format with separator")
	}

	// Should NOT be JSON
	if strings.Contains(outputStr, "\"interface\":") {
		t.Error("Default output should not be JSON format")
	}
}
