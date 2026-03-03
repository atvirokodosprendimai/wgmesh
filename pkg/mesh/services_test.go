package mesh

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadServicesEmpty(t *testing.T) {
	state, err := LoadServices("/nonexistent/path/services.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(state.Services) != 0 {
		t.Errorf("expected empty services, got %d", len(state.Services))
	}
}

func TestSaveAndLoadServices(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "services.json")

	state := ServiceState{
		Services: map[string]ServiceEntry{
			"ollama": {
				SiteID:       "site_abc123",
				Name:         "ollama",
				Domain:       "ollama.abcdef123456.wgmesh.dev",
				LocalAddr:    ":11434",
				Protocol:     "http",
				RegisteredAt: time.Date(2026, 3, 3, 19, 0, 0, 0, time.UTC),
			},
		},
	}

	if err := SaveServices(path, state); err != nil {
		t.Fatalf("SaveServices failed: %v", err)
	}

	loaded, err := LoadServices(path)
	if err != nil {
		t.Fatalf("LoadServices failed: %v", err)
	}

	if len(loaded.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(loaded.Services))
	}

	entry, ok := loaded.Services["ollama"]
	if !ok {
		t.Fatal("expected 'ollama' service")
	}
	if entry.SiteID != "site_abc123" {
		t.Errorf("expected site_id 'site_abc123', got %q", entry.SiteID)
	}
	if entry.Domain != "ollama.abcdef123456.wgmesh.dev" {
		t.Errorf("expected domain 'ollama.abcdef123456.wgmesh.dev', got %q", entry.Domain)
	}
}

func TestSaveServicesAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "services.json")

	state := ServiceState{
		Services: map[string]ServiceEntry{
			"api": {
				SiteID: "site_xyz",
				Name:   "api",
			},
		},
	}

	if err := SaveServices(path, state); err != nil {
		t.Fatalf("SaveServices failed: %v", err)
	}

	// Verify no .tmp file remains
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should not exist after successful save")
	}

	// Verify file permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected 0600 permissions, got %o", info.Mode().Perm())
	}
}

func TestSaveServicesCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "services.json")

	state := ServiceState{Services: make(map[string]ServiceEntry)}
	if err := SaveServices(path, state); err != nil {
		t.Fatalf("SaveServices failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("file should exist: %v", err)
	}
}
