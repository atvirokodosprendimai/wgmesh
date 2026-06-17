package onboarding

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_LoadAndSave(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "test-onboarding.json")

	// Create a new store
	store := &Store{
		CompletedItems: []string{"secret_generation", "interface_config"},
		CurrentStep:    "github_registry",
		StartedAt:      time.Now().Add(-1 * time.Hour),
		LastUpdated:    time.Now(),
	}

	// Save store
	err := store.Save(testPath)
	if err != nil {
		t.Fatalf("Failed to save store: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Fatal("Store file was not created")
	}

	// Load store
	loadedStore, err := LoadStore(testPath)
	if err != nil {
		t.Fatalf("Failed to load store: %v", err)
	}

	// Verify loaded data
	if len(loadedStore.CompletedItems) != 2 {
		t.Errorf("Expected 2 completed items, got %d", len(loadedStore.CompletedItems))
	}

	if loadedStore.CurrentStep != "github_registry" {
		t.Errorf("Expected current step 'github_registry', got '%s'", loadedStore.CurrentStep)
	}

	if len(loadedStore.CompletedItems) != len(store.CompletedItems) {
		t.Error("Completed items count mismatch")
	}
}

func TestStore_LoadNonExistent(t *testing.T) {
	// Try to load a non-existent store
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "does-not-exist.json")

	store, err := LoadStore(testPath)
	if err != nil {
		t.Fatalf("Failed to load non-existent store: %v", err)
	}

	if store == nil {
		t.Fatal("Expected empty store, got nil")
	}

	if len(store.CompletedItems) != 0 {
		t.Errorf("Expected 0 completed items for new store, got %d", len(store.CompletedItems))
	}
}

func TestStore_MarkComplete(t *testing.T) {
	store := &Store{
		CompletedItems: []string{},
		StartedAt:      time.Now(),
		LastUpdated:    time.Now(),
	}

	// Mark first step as complete
	err := store.MarkComplete("secret_generation")
	if err != nil {
		t.Fatalf("Failed to mark complete: %v", err)
	}

	if len(store.CompletedItems) != 1 {
		t.Errorf("Expected 1 completed item, got %d", len(store.CompletedItems))
	}

	if !store.IsComplete("secret_generation") {
		t.Error("Expected secret_generation to be complete")
	}

	// Mark again (should not duplicate)
	err = store.MarkComplete("secret_generation")
	if err != nil {
		t.Fatalf("Failed to mark complete again: %v", err)
	}

	if len(store.CompletedItems) != 1 {
		t.Errorf("Expected 1 completed item after duplicate, got %d", len(store.CompletedItems))
	}
}

func TestStore_SetCurrentStep(t *testing.T) {
	store := &Store{
		CompletedItems: []string{},
		StartedAt:      time.Now(),
		LastUpdated:    time.Now(),
	}

	err := store.SetCurrentStep("github_registry")
	if err != nil {
		t.Fatalf("Failed to set current step: %v", err)
	}

	if store.CurrentStep != "github_registry" {
		t.Errorf("Expected current step 'github_registry', got '%s'", store.CurrentStep)
	}
}

func TestStore_Reset(t *testing.T) {
	store := &Store{
		CompletedItems: []string{"step1", "step2", "step3"},
		CurrentStep:    "step4",
		StartedAt:      time.Now().Add(-1 * time.Hour),
		LastUpdated:    time.Now(),
	}

	err := store.Reset()
	if err != nil {
		t.Fatalf("Failed to reset: %v", err)
	}

	if len(store.CompletedItems) != 0 {
		t.Errorf("Expected 0 completed items after reset, got %d", len(store.CompletedItems))
	}

	if store.CurrentStep != "" {
		t.Errorf("Expected empty current step after reset, got '%s'", store.CurrentStep)
	}

	if !store.StartedAt.After(time.Now().Add(-1 * time.Minute)) {
		t.Error("Expected StartedAt to be updated after reset")
	}
}

func TestStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "test-delete.json")

	// Create a file
	store := &Store{
		CompletedItems: []string{"step1"},
		StartedAt:      time.Now(),
		LastUpdated:    time.Now(),
	}

	err := store.Save(testPath)
	if err != nil {
		t.Fatalf("Failed to save store: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Fatal("Store file was not created")
	}

	// Delete file
	err = Delete(testPath)
	if err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(testPath); !os.IsNotExist(err) {
		t.Fatal("Store file still exists after delete")
	}

	// Delete non-existent file (should not error)
	err = Delete(testPath)
	if err != nil {
		t.Errorf("Expected no error when deleting non-existent file, got %v", err)
	}
}

func TestStore_Duration(t *testing.T) {
	store := &Store{
		CompletedItems: []string{},
		StartedAt:      time.Now().Add(-1 * time.Hour),
		LastUpdated:    time.Now(),
	}

	duration := store.Duration()
	if duration < 59*time.Minute || duration > 61*time.Minute {
		t.Errorf("Expected duration around 1 hour, got %v", duration)
	}
}

func TestStore_Progress(t *testing.T) {
	store := &Store{
		CompletedItems: []string{"step1", "step2", "step3"},
		StartedAt:      time.Now(),
		LastUpdated:    time.Now(),
	}

	progress := store.Progress(7)
	if progress == "" {
		t.Fatal("Expected progress string, got empty")
	}

	// Progress should contain "3/7"
	if len(progress) < 5 {
		t.Errorf("Progress string too short: %s", progress)
	}
}
