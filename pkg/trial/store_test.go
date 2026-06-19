package trial

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreLoadNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	state, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if state != nil {
		t.Error("expected nil state when file doesn't exist")
	}
}

func TestStoreSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	now := time.Now()
	state := &TrialState{
		MeshID:    "test-mesh-123",
		StartedAt: now,
		ExpiresAt: now.Add(TrialDuration),
		Status:    StatusActive,
	}

	// Save
	err := store.Save(state)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(store.filename); err != nil {
		t.Errorf("trial file doesn't exist: %v", err)
	}

	// Load
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.MeshID != state.MeshID {
		t.Errorf("expected mesh ID %s, got %s", state.MeshID, loaded.MeshID)
	}

	if loaded.Status != state.Status {
		t.Errorf("expected status %s, got %s", state.Status, loaded.Status)
	}

	// Check times are approximately equal (JSON marshaling loses some precision)
	if loaded.StartedAt.Sub(state.StartedAt) > time.Second {
		t.Errorf("time mismatch: started %s vs %s", state.StartedAt, loaded.StartedAt)
	}
}

func TestStoreSaveNil(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	err := store.Save(nil)
	if err == nil {
		t.Error("expected error when saving nil state")
	}
}

func TestStoreUpdateExisting(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Create initial state
	now := time.Now()
	state := &TrialState{
		MeshID:    "test-mesh-123",
		StartedAt: now,
		ExpiresAt: now.Add(TrialDuration),
		Status:    StatusActive,
	}

	if err := store.Save(state); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Update state
	state.Status = StatusUpgraded
	nowUpgraded := time.Now()
	state.UpgradedAt = &nowUpgraded

	if err := store.Save(state); err != nil {
		t.Fatalf("Save updated failed: %v", err)
	}

	// Load and verify
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Status != StatusUpgraded {
		t.Errorf("expected status %s, got %s", StatusUpgraded, loaded.Status)
	}

	if loaded.UpgradedAt == nil {
		t.Error("expected UpgradedAt to be set")
	}
}

func TestStoreDelete(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Create and save state
	state := &TrialState{
		MeshID:    "test-mesh-123",
		StartedAt: time.Now(),
		ExpiresAt: time.Now().Add(TrialDuration),
		Status:    StatusActive,
	}

	if err := store.Save(state); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(store.filename); err != nil {
		t.Errorf("trial file doesn't exist: %v", err)
	}

	// Delete
	if err := store.Delete(); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(store.filename); !os.IsNotExist(err) {
		t.Error("trial file still exists after delete")
	}
}

func TestStoreLoadOrStartNew(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	meshID := "test-mesh-456"

	state, err := store.LoadOrStart(meshID)
	if err != nil {
		t.Fatalf("LoadOrStart failed: %v", err)
	}

	if state == nil {
		t.Fatal("expected non-nil state")
	}

	if state.MeshID != meshID {
		t.Errorf("expected mesh ID %s, got %s", meshID, state.MeshID)
	}

	if state.Status != StatusActive {
		t.Errorf("expected status %s, got %s", StatusActive, state.Status)
	}

	// Verify file was created
	if _, err := os.Stat(store.filename); err != nil {
		t.Errorf("trial file wasn't created: %v", err)
	}
}

func TestStoreLoadOrStartExisting(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	meshID := "test-mesh-789"

	// Create initial trial
	state1, err := store.LoadOrStart(meshID)
	if err != nil {
		t.Fatalf("LoadOrStart failed: %v", err)
	}

	// Load again - should return existing state
	state2, err := store.LoadOrStart(meshID)
	if err != nil {
		t.Fatalf("LoadOrStart (2nd) failed: %v", err)
	}

	if state2.MeshID != state1.MeshID {
		t.Error("mesh ID changed on reload")
	}

	if state2.StartedAt.Sub(state1.StartedAt) > time.Second {
		t.Errorf("StartedAt changed on reload: %s vs %s", state1.StartedAt, state2.StartedAt)
	}

	if state2.ExpiresAt.Sub(state1.ExpiresAt) > time.Second {
		t.Errorf("ExpiresAt changed on reload: %s vs %s", state1.ExpiresAt, state2.ExpiresAt)
	}
}

func TestStoreLoadOrStartMeshIDMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Create trial with one mesh ID
	state1, err := store.LoadOrStart("mesh-1")
	if err != nil {
		t.Fatalf("LoadOrStart failed: %v", err)
	}

	// Try to load with different mesh ID
	_, err = store.LoadOrStart("mesh-2")
	if err == nil {
		t.Error("expected error for mesh ID mismatch")
	}

	// Original state should be unchanged
	state2, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if state2.MeshID != state1.MeshID {
		t.Error("mesh ID was modified after mismatch error")
	}
}

func TestStoreCreatesDirectory(t *testing.T) {
	// Use a non-existent subdirectory
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir", "nested")
	store := NewStore(subDir)

	state := &TrialState{
		MeshID:    "test-mesh",
		StartedAt: time.Now(),
		ExpiresAt: time.Now().Add(TrialDuration),
		Status:    StatusActive,
	}

	err := store.Save(state)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(subDir); err != nil {
		t.Errorf("directory wasn't created: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(store.filename); err != nil {
		t.Errorf("file wasn't created: %v", err)
	}
}

func TestStoreAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	state := &TrialState{
		MeshID:    "test-mesh",
		StartedAt: time.Now(),
		ExpiresAt: time.Now().Add(TrialDuration),
		Status:    StatusActive,
	}

	// Save multiple times to ensure atomic rename works
	for i := 0; i < 5; i++ {
		if err := store.Save(state); err != nil {
			t.Fatalf("Save (iteration %d) failed: %v", i, err)
		}
	}

	// Final load should succeed
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.MeshID != state.MeshID {
		t.Error("mesh ID mismatch after multiple saves")
	}
}
