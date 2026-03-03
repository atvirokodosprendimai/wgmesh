package mesh

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAccountNotFound(t *testing.T) {
	_, err := LoadAccount("/nonexistent/account.json")
	if err == nil {
		t.Fatal("expected error for missing account file")
	}
}

func TestSaveAndLoadAccount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.json")

	cfg := AccountConfig{
		APIKey:        "cr_abc123def456",
		LighthouseURL: "https://lighthouse.abcdef123456.wgmesh.dev",
	}

	if err := SaveAccount(path, cfg); err != nil {
		t.Fatalf("SaveAccount failed: %v", err)
	}

	loaded, err := LoadAccount(path)
	if err != nil {
		t.Fatalf("LoadAccount failed: %v", err)
	}

	if loaded.APIKey != cfg.APIKey {
		t.Errorf("expected api_key %q, got %q", cfg.APIKey, loaded.APIKey)
	}
	if loaded.LighthouseURL != cfg.LighthouseURL {
		t.Errorf("expected lighthouse_url %q, got %q", cfg.LighthouseURL, loaded.LighthouseURL)
	}
}

func TestLoadAccountEmptyKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.json")

	if err := os.WriteFile(path, []byte(`{"api_key":""}`), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadAccount(path)
	if err == nil {
		t.Fatal("expected error for empty api_key")
	}
}

func TestSaveAccountPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.json")

	cfg := AccountConfig{APIKey: "cr_test"}
	if err := SaveAccount(path, cfg); err != nil {
		t.Fatalf("SaveAccount failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected 0600 permissions, got %o", info.Mode().Perm())
	}
}
