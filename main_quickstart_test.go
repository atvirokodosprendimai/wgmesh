package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestQuickstartHelp verifies the `wgmesh quickstart --help` dispatch exists
// and prints the usage for the trial subcommand.
func TestQuickstartHelp(t *testing.T) {
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-quickstart-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove("/tmp/wgmesh-quickstart-test")

	cmd := exec.Command("/tmp/wgmesh-quickstart-test", "quickstart", "--help")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	// flag.ExitOnError causes exit code 0 for explicit --help.
	if err := cmd.Run(); err != nil {
		t.Fatalf("quickstart --help failed: %v, output: %s", err, out.String())
	}

	got := out.String()
	if !strings.Contains(got, "Usage: wgmesh quickstart") {
		t.Errorf("expected quickstart usage header, got:\n%s", got)
	}
	if !strings.Contains(got, "trial mesh") {
		t.Errorf("expected trial mesh description in help, got:\n%s", got)
	}
}

// TestQuickstartDryRun exercises the CLI dispatch path for the quickstart
// subcommand without starting a daemon. It verifies the deterministic success
// line ("trial mesh joined") is printed and the embedded public trial secret
// is referenced.
func TestQuickstartDryRun(t *testing.T) {
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-quickstart-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove("/tmp/wgmesh-quickstart-test")

	cmd := exec.Command("/tmp/wgmesh-quickstart-test", "quickstart", "--dry-run")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("quickstart --dry-run failed: %v, output: %s", err, out.String())
	}

	got := out.String()
	if !strings.Contains(got, "trial mesh joined") {
		t.Errorf("expected deterministic success line 'trial mesh joined', got:\n%s", got)
	}
	if !strings.Contains(got, DefaultTrialSecret) {
		t.Errorf("expected embedded public trial secret in output, got:\n%s", got)
	}
	if !strings.Contains(got, "wgmesh quickstart") {
		t.Errorf("expected quickstart references in output, got:\n%s", got)
	}
}

// TestQuickstartEnvSecret verifies that WGMESH_TRIAL_SECRET overrides the
// embedded default trial secret in the printed command.
func TestQuickstartEnvSecret(t *testing.T) {
	buildCmd := exec.Command("go", "build", "-o", "/tmp/wgmesh-quickstart-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v", err)
	}
	defer os.Remove("/tmp/wgmesh-quickstart-test")

	override := "wgmesh://v1/custom-rotated-trial-secret-123"
	cmd := exec.Command("/tmp/wgmesh-quickstart-test", "quickstart", "--dry-run")
	cmd.Env = append(os.Environ(), "WGMESH_TRIAL_SECRET="+override)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("quickstart --dry-run failed: %v, output: %s", err, out.String())
	}

	got := out.String()
	if !strings.Contains(got, override) {
		t.Errorf("expected override trial secret in output, got:\n%s", got)
	}
	if strings.Contains(got, DefaultTrialSecret) {
		t.Errorf("default secret should not appear when overridden, got:\n%s", got)
	}
}
