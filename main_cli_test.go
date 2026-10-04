package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestPilotInitRejectsInvalidFlagValues asserts that a malformed explicit
// --nodes / --duration value fails fast at flag-parse time (exit 1 + usage),
// instead of warn-and-continue that would silently fall back to defaults and
// proceed to initialize a pilot with the wrong shape.
func TestPilotInitRejectsInvalidFlagValues(t *testing.T) {
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer func() { _ = os.Remove("/tmp/wgmesh-test") }()

	tests := []struct {
		name string
		flag string
	}{
		{"nodes", "--nodes"},
		{"duration", "--duration"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("/tmp/wgmesh-test", "pilot", "init", "--org", "acme", "--contact", "a@b.com", tt.flag, "abc")
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected non-zero exit for invalid %s, got success. output: %s", tt.flag, output)
			}
			out := string(output)
			if !strings.Contains(out, "invalid "+tt.flag) {
				t.Errorf("expected 'invalid %s' error, got: %s", tt.flag, out)
			}
			if !strings.Contains(out, "pilot init") {
				t.Errorf("expected usage to be printed, got: %s", out)
			}
			if strings.Contains(out, "Failed to initialize pilot") || strings.Contains(out, "Failed to save pilot") {
				t.Errorf("invalid flag should fail before pilot init/save, got: %s", out)
			}
		})
	}
}
