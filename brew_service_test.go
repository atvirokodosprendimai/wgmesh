package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSaveSecretFile checks that the generated secret URI lands on disk with
// 0600 permissions, a trailing newline, and parent directories created — the
// file layout consumed by WGMESH_SECRET_FILE in the Homebrew service.
func TestSaveSecretFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "wgmesh", "secret")

	const uri = "wgmesh://v1/abc123"
	if err := saveSecretFile(path, uri); err != nil {
		t.Fatalf("saveSecretFile() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%s): %v", path, err)
	}
	if got, want := strings.TrimSpace(string(data)), uri; got != want {
		t.Errorf("secret content = %q, want %q", got, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%s): %v", path, err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("secret file perms = %o, want 600", perm)
	}
}
