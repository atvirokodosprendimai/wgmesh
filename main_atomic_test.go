package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Words: saveSecretFile replaces the init secret file (wgmesh init --save-to).
// Why: written in place, a crash mid-write leaves a truncated secret; the
// node would then join with a wrong secret or none (#870).
// Invariants: content "<uri>\n", mode 0600, parent directories 0700 unchanged.

func TestSaveSecretFileIsAtomic(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	path := filepath.Join(dir, "secret")
	first := "wgmesh://v1/" + strings.Repeat("a", 43)
	second := "wgmesh://v1/" + strings.Repeat("b", 43)

	if err := saveSecretFile(path, first); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveSecretFile(path, second); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Error("secret file was rewritten in place (same inode); want a temp file renamed over it")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != second+"\n" {
		t.Errorf("content %q err %v, want %q", got, err, second+"\n")
	}
	if after.Mode().Perm() != 0o600 {
		t.Errorf("mode %o, want 600", after.Mode().Perm())
	}
	if di, err := os.Stat(dir); err != nil || di.Mode().Perm() != 0o700 {
		t.Errorf("directory mode: %v %v, want 700", di, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 1 || names[0] != filepath.Base(path) {
		t.Errorf("directory holds %q; want only %q", names, filepath.Base(path))
	}
}

func TestSaveSecretFileKeepsOldFileWhenWriteFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: a read-only directory does not stop root, the failure cannot be provoked")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := saveSecretFile(path, "wgmesh://v1/old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if err := saveSecretFile(path, "wgmesh://v1/new"); err == nil {
		t.Fatal("save into a read-only directory returned nil; want the error")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "wgmesh://v1/old\n" {
		t.Errorf("old secret changed: %q %v", got, err)
	}
}
