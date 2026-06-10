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
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-test-status", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove("/tmp/wgmesh-test-status")

	testSecret := "wgmesh://v1/dGVzdHNlY3JldGZvcnN0YXR1c2pzb250ZXN0aW5nMTIz"

	tests := []struct {
		name        string
		args        []string
		wantKeys    []string
		wantCompact bool
	}{
		{
			name:        "compact json",
			args:        []string{"status", "--secret", testSecret, "--json"},
			wantKeys:    []string{"interface", "network_id", "mesh_subnet", "mesh_ipv6_prefix", "gossip_port", "rendezvous_id", "active_peers", "total_peers", "peers"},
			wantCompact: true,
		},
		{
			name:        "pretty json",
			args:        []string{"status", "--secret", testSecret, "--json", "--pretty"},
			wantKeys:    []string{"interface", "network_id", "mesh_subnet", "mesh_ipv6_prefix", "gossip_port", "rendezvous_id", "active_peers", "total_peers", "peers"},
			wantCompact: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("/tmp/wgmesh-test-status", tt.args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Command failed: %v\nOutput: %s", err, output)
			}

			trimmed := strings.TrimSpace(string(output))

			// Validate JSON parses cleanly.
			var parsed map[string]interface{}
			if jsonErr := json.Unmarshal([]byte(trimmed), &parsed); jsonErr != nil {
				t.Fatalf("Output is not valid JSON: %v\nOutput: %s", jsonErr, trimmed)
			}

			// Check required top-level keys are present.
			for _, key := range tt.wantKeys {
				if _, ok := parsed[key]; !ok {
					t.Errorf("Missing JSON key %q in output: %s", key, trimmed)
				}
			}

			// Compact check: output must be a single line.
			if tt.wantCompact {
				lines := strings.Split(trimmed, "\n")
				if len(lines) != 1 {
					t.Errorf("Expected single-line compact JSON, got %d lines", len(lines))
				}
			}

			// Pretty check: output must have more than one line.
			if !tt.wantCompact {
				lines := strings.Split(trimmed, "\n")
				if len(lines) < 5 {
					t.Errorf("Expected multi-line pretty JSON, got %d lines", len(lines))
				}
			}

			// peers field must be a JSON array (even if empty).
			if peersRaw, ok := parsed["peers"]; ok {
				if _, ok := peersRaw.([]interface{}); !ok {
					t.Errorf("peers field must be a JSON array, got %T", peersRaw)
				}
			}

			// gossip_port must be a number.
			if port, ok := parsed["gossip_port"]; ok {
				if _, ok := port.(float64); !ok {
					t.Errorf("gossip_port must be a number, got %T", port)
				}
			}
		})
	}
}

func TestStatusTextOutputUnchanged(t *testing.T) {
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-test-status-text", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove("/tmp/wgmesh-test-status-text")

	testSecret := "wgmesh://v1/dGVzdHNlY3JldGZvcnN0YXR1c2pzb250ZXN0aW5nMTIz"

	cmd := exec.Command("/tmp/wgmesh-test-status-text", "status", "--secret", testSecret)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, output)
	}

	result := string(output)

	// Text output must contain the human-readable header.
	if !strings.Contains(result, "Mesh Status") {
		t.Errorf("Expected text output to contain 'Mesh Status', got: %s", result)
	}
	if !strings.Contains(result, "Interface:") {
		t.Errorf("Expected text output to contain 'Interface:', got: %s", result)
	}
	if !strings.Contains(result, "Network ID:") {
		t.Errorf("Expected text output to contain 'Network ID:', got: %s", result)
	}

	// Text output must NOT start with '{'.
	trimmed := strings.TrimSpace(result)
	if strings.HasPrefix(trimmed, "{") {
		t.Errorf("Text output must not be JSON, got: %s", trimmed)
	}
}
