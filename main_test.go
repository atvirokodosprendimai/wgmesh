package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

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
