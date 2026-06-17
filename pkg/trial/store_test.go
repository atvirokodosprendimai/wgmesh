package trial

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStoreCreateGet(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "trials.json")

	store, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	trial := &Trial{
		ID:        "test-id-1",
		Email:     "test@example.com",
		Source:    "test",
		CreatedAt: time.Now(),
		Status:    "pending",
	}

	// Test Create
	if err := store.Create(trial); err != nil {
		t.Errorf("Create() error = %v", err)
	}

	// Test GetByID
	got, err := store.GetByID(trial.ID)
	if err != nil {
		t.Errorf("GetByID() error = %v", err)
	}
	if got.Email != trial.Email {
		t.Errorf("GetByID() got = %v, want %v", got.Email, trial.Email)
	}

	// Test GetByEmail
	got, err = store.GetByEmail(trial.Email)
	if err != nil {
		t.Errorf("GetByEmail() error = %v", err)
	}
	if got.ID != trial.ID {
		t.Errorf("GetByEmail() got = %v, want %v", got.ID, trial.ID)
	}
}

func TestFileStoreExistsDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "trials.json")

	store, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	trial := &Trial{
		ID:        "test-id-1",
		Email:     "test@example.com",
		Source:    "test",
		CreatedAt: time.Now(),
		Status:    "pending",
	}

	store.Create(trial)

	// Test exists
	if !store.Exists(trial.Email) {
		t.Error("Exists() should return true for existing email")
	}

	// Test not exists
	if store.Exists("other@example.com") {
		t.Error("Exists() should return false for non-existing email")
	}
}

func TestFileStoreUpdateStatus(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "trials.json")

	store, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	trial := &Trial{
		ID:        "test-id-1",
		Email:     "test@example.com",
		Source:    "test",
		CreatedAt: time.Now(),
		Status:    "pending",
	}

	store.Create(trial)

	// Update status
	if err := store.UpdateStatus(trial.ID, "active"); err != nil {
		t.Errorf("UpdateStatus() error = %v", err)
	}

	// Verify
	got, _ := store.GetByID(trial.ID)
	if got.Status != "active" {
		t.Errorf("Status = %v, want %v", got.Status, "active")
	}
}

func TestFileStoreEmailTracking(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "trials.json")

	store, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	trial := &Trial{
		ID:        "test-id-1",
		Email:     "test@example.com",
		Source:    "test",
		CreatedAt: time.Now(),
		Status:    "pending",
	}

	store.Create(trial)

	// Test mark sent
	if err := store.MarkEmailSent(trial.ID, "test_email"); err != nil {
		t.Errorf("MarkEmailSent() error = %v", err)
	}

	// Test check sent
	sent, err := store.EmailSent(trial.ID, "test_email")
	if err != nil {
		t.Errorf("EmailSent() error = %v", err)
	}
	if !sent {
		t.Error("EmailSent() should return true for sent email")
	}

	// Test not sent
	sent, err = store.EmailSent(trial.ID, "other_email")
	if err != nil {
		t.Errorf("EmailSent() error = %v", err)
	}
	if sent {
		t.Error("EmailSent() should return false for unsent email")
	}
}

func TestFileStoreAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "trials.json")

	store, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// Create multiple trials
	for i := 0; i < 10; i++ {
		trial := &Trial{
			ID:        "test-id-" + string(rune(i)),
			Email:     "test@example.com",
			Source:    "test",
			CreatedAt: time.Now(),
			Status:    "pending",
		}
		store.Create(trial)
	}

	// Reload store to verify persistence
	store2, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileStore() reload error = %v", err)
	}

	trials, _ := store2.GetPendingNurture()
	if len(trials) != 10 {
		t.Errorf("Got %d trials, want 10", len(trials))
	}
}

func TestFileStoreGetExpiring(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "trials.json")

	store, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// Create an old trial (13 days ago - should be expiring)
	oldTrial := &Trial{
		ID:        "old-id",
		Email:     "old@example.com",
		Source:    "test",
		CreatedAt: time.Now().Add(-13 * 24 * time.Hour),
		Status:    "pending",
	}
	store.Create(oldTrial)

	// Create a recent trial (should not be expiring)
	recentTrial := &Trial{
		ID:        "recent-id",
		Email:     "recent@example.com",
		Source:    "test",
		CreatedAt: time.Now().Add(-1 * 24 * time.Hour),
		Status:    "pending",
	}
	store.Create(recentTrial)

	// Get expiring trials (those expiring within 1 day from now)
	expiring, err := store.GetExpiring(time.Now().Add(1 * 24 * time.Hour))
	if err != nil {
		t.Errorf("GetExpiring() error = %v", err)
	}

	if len(expiring) != 1 {
		t.Errorf("Got %d expiring trials, want 1", len(expiring))
	}

	if len(expiring) > 0 && expiring[0].ID != oldTrial.ID {
		t.Errorf("Got expiring trial ID %s, want %s", expiring[0].ID, oldTrial.ID)
	}
}

func TestFileStorePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "trials.json")

	// Create store and add trial
	store, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	trial := &Trial{
		ID:        "test-id-1",
		Email:     "test@example.com",
		Source:    "test",
		CreatedAt: time.Now(),
		Status:    "pending",
	}

	if err := store.Create(trial); err != nil {
		t.Errorf("Create() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(storePath); os.IsNotExist(err) {
		t.Error("Store file was not created")
	}

	// Reload and verify
	store2, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("NewFileStore() reload error = %v", err)
	}

	got, err := store2.GetByID(trial.ID)
	if err != nil {
		t.Errorf("GetByID() after reload error = %v", err)
	}
	if got.Email != trial.Email {
		t.Errorf("GetByID() after reload got = %v, want %v", got.Email, trial.Email)
	}
}
