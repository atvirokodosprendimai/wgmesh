package mesh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot walks up from the test's directory to the folder holding go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}

// ADR-0006 task T6: the documents describe the centralized deploy as built and
// name the two later flips. The test checks names, not meaning; review owns the
// meaning. A missing file fails its assertions.
func TestSpecNamesCentralized(t *testing.T) {
	root := repoRoot(t)
	const (
		deploySpec = "eidos/spec - centralized SSH deploy - diff-based persistent WireGuard config pushed via SSH.md"
		stateSpec  = "eidos/spec - mesh state and access control - centralized node registry with policy-filtered peer configuration.md"
		guide      = "docs/centralized-mode.md"
		note       = "docs/centralized-rollout.md"
		backlog    = "docs/adr/BACKLOG.md"
		readme     = "README.md"
	)
	wanted := []struct {
		file  string
		names []string
	}{
		{deploySpec, []string{"public_endpoint", "PersistFailureFatal", "ssh.Cmd", "connected route"}},
		{stateSpec, []string{"StrictState", "class A", "class B"}},
		{guide, []string{"centralized-rollout.md", "public_endpoint", "connected route", "StrictState", "PersistFailureFatal"}},
		{note, []string{"StrictState", "PersistFailureFatal", "public_endpoint", "class A", "class B", "N+2", "connected route", "Warnings:"}},
		{backlog, []string{"ADR-0006", "#894", "StrictState", "PersistFailureFatal", "N+2", "-only"}},
		{readme, []string{"centralized-rollout.md"}},
	}
	for _, w := range wanted {
		raw, err := os.ReadFile(filepath.Join(root, w.file))
		if err != nil {
			t.Errorf("%s: cannot read: %v", w.file, err)
			continue
		}
		text := string(raw)
		for _, name := range w.names {
			if !strings.Contains(text, name) {
				t.Errorf("%s does not name %q", w.file, name)
			}
		}
	}

	// The old NAT sentence said a node whose public IP differs from its SSH
	// host gets no public endpoint, without the all-private rule and the
	// operator override. A line that still says it must carry the condition.
	raw, err := os.ReadFile(filepath.Join(root, deploySpec))
	if err != nil {
		t.Errorf("%s: cannot read: %v", deploySpec, err)
		return
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "given no public endpoint") && !strings.Contains(line, "private") {
			t.Errorf("%s keeps the old NAT sentence without the private-address condition: %q", deploySpec, line)
		}
	}
	if !strings.Contains(string(raw), "private IP") {
		t.Errorf("%s does not state the all-private endpoint rule", deploySpec)
	}
}
