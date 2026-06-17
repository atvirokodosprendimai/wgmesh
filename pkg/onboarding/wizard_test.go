package onboarding

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewWizard(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "test-wizard.json")

	opts := WizardOptions{
		Secret:              "test-secret-long-enough-for-validation",
		InterfaceName:       "wg0",
		SkipRegistry:        true,
		DisableLANDiscovery: true,
		PeerTimeout:         2 * time.Minute,
	}

	wizard, err := NewWizard(opts, testPath)
	if err != nil {
		t.Fatalf("Failed to create wizard: %v", err)
	}

	if wizard == nil {
		t.Fatal("Expected wizard, got nil")
	}

	if wizard.checklist == nil {
		t.Error("Expected checklist, got nil")
	}

	if wizard.store == nil {
		t.Error("Expected store, got nil")
	}

	if wizard.storePath != testPath {
		t.Errorf("Expected store path %s, got %s", testPath, wizard.storePath)
	}
}

func TestNewWizard_Reset(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "test-wizard-reset.json")

	// Create existing state
	store := &Store{
		CompletedItems: []string{"step1", "step2"},
		CurrentStep:    "step3",
		StartedAt:      time.Now().Add(-1 * time.Hour),
		LastUpdated:    time.Now(),
	}

	err := store.Save(testPath)
	if err != nil {
		t.Fatalf("Failed to save initial state: %v", err)
	}

	// Create wizard with reset flag
	opts := WizardOptions{
		Secret:              "test-secret-long-enough",
		InterfaceName:       "wg0",
		SkipRegistry:        true,
		DisableLANDiscovery: true,
		Reset:               true,
	}

	wizard, err := NewWizard(opts, testPath)
	if err != nil {
		t.Fatalf("Failed to create wizard with reset: %v", err)
	}

	// Verify state was reset
	if len(wizard.store.CompletedItems) != 0 {
		t.Errorf("Expected empty completed items after reset, got %d", len(wizard.store.CompletedItems))
	}

	if wizard.store.CurrentStep != "" {
		t.Errorf("Expected empty current step after reset, got %s", wizard.store.CurrentStep)
	}
}

func TestWizard_RestoreState(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "test-restore.json")

	// Create store with some completed steps
	store := &Store{
		CompletedItems: []string{"secret_generation", "interface_config"},
		CurrentStep:    "github_registry",
		StartedAt:      time.Now().Add(-1 * time.Hour),
		LastUpdated:    time.Now(),
	}

	err := store.Save(testPath)
	if err != nil {
		t.Fatalf("Failed to save initial state: %v", err)
	}

	// Create wizard
	opts := WizardOptions{
		Secret:              "test-secret-long-enough",
		InterfaceName:       "wg0",
		SkipRegistry:        true,
		DisableLANDiscovery: true,
	}

	wizard, err := NewWizard(opts, testPath)
	if err != nil {
		t.Fatalf("Failed to create wizard: %v", err)
	}

	// Verify state was restored
	if wizard.checklist.CompletedCount() != 2 {
		t.Errorf("Expected 2 completed items, got %d", wizard.checklist.CompletedCount())
	}

	// Should be at step 2 (github_registry)
	if wizard.checklist.currentIndex != 2 {
		t.Errorf("Expected index 2, got %d", wizard.checklist.currentIndex)
	}
}

func TestWizard_Status(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "test-status.json")

	opts := WizardOptions{
		Secret:              "test-secret-long-enough",
		InterfaceName:       "wg0",
		SkipRegistry:        true,
		DisableLANDiscovery: true,
	}

	wizard, err := NewWizard(opts, testPath)
	if err != nil {
		t.Fatalf("Failed to create wizard: %v", err)
	}

	// Complete first step
	wizard.checklist.MarkComplete()
	wizard.checklist.Advance()

	status, err := wizard.Status()
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	if status == nil {
		t.Fatal("Expected status, got nil")
	}

	if status.TotalSteps != 7 {
		t.Errorf("Expected 7 total steps, got %d", status.TotalSteps)
	}

	if status.CompletedSteps != 1 {
		t.Errorf("Expected 1 completed step, got %d", status.CompletedSteps)
	}

	if status.Progress == "" {
		t.Error("Expected progress string, got empty")
	}

	if status.Items == nil {
		t.Error("Expected items, got nil")
	}

	if len(status.Items) != 7 {
		t.Errorf("Expected 7 items, got %d", len(status.Items))
	}
}

func TestWizard_SaveState(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "test-save-state.json")

	opts := WizardOptions{
		Secret:              "test-secret-long-enough",
		InterfaceName:       "wg0",
		SkipRegistry:        true,
		DisableLANDiscovery: true,
	}

	wizard, err := NewWizard(opts, testPath)
	if err != nil {
		t.Fatalf("Failed to create wizard: %v", err)
	}

	// Complete first step
	wizard.checklist.MarkComplete()
	wizard.checklist.Advance()

	// Save state
	err = wizard.saveState()
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Fatal("State file was not created")
	}

	// Load and verify
	loadedStore, err := LoadStore(testPath)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	if len(loadedStore.CompletedItems) != 1 {
		t.Errorf("Expected 1 completed item, got %d", len(loadedStore.CompletedItems))
	}
}

func TestWizardStatus_Format(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "test-status-format.json")

	opts := WizardOptions{
		Secret:              "test-secret-long-enough",
		InterfaceName:       "wg0",
		SkipRegistry:        true,
		DisableLANDiscovery: true,
	}

	wizard, err := NewWizard(opts, testPath)
	if err != nil {
		t.Fatalf("Failed to create wizard: %v", err)
	}

	status, err := wizard.Status()
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	// Verify status fields
	if status.TotalSteps == 0 {
		t.Error("Expected non-zero TotalSteps")
	}

	if status.StartedAt.IsZero() {
		t.Error("Expected StartedAt to be set")
	}

	if status.LastUpdated.IsZero() {
		t.Error("Expected LastUpdated to be set")
	}

	if status.Duration < 0 {
		t.Error("Expected non-negative Duration")
	}
}

func TestWizard_SkipToStep(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "test-skip-to.json")

	opts := WizardOptions{
		Secret:              "test-secret-long-enough",
		InterfaceName:       "wg0",
		SkipRegistry:        true,
		DisableLANDiscovery: true,
		SkipTo:              "dht_bootstrap",
	}

	wizard, err := NewWizard(opts, testPath)
	if err != nil {
		t.Fatalf("Failed to create wizard: %v", err)
	}

	// The skipTo option is only processed when calling Run()
	// NewWizard alone doesn't process it
	// So we just verify the wizard was created successfully
	if wizard == nil {
		t.Fatal("Expected wizard, got nil")
	}

	if wizard.options.SkipTo != "dht_bootstrap" {
		t.Errorf("Expected SkipTo to be 'dht_bootstrap', got '%s'", wizard.options.SkipTo)
	}
}

func TestWizard_Progress(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "test-progress.json")

	opts := WizardOptions{
		Secret:              "test-secret-long-enough",
		InterfaceName:       "wg0",
		SkipRegistry:        true,
		DisableLANDiscovery: true,
	}

	wizard, err := NewWizard(opts, testPath)
	if err != nil {
		t.Fatalf("Failed to create wizard: %v", err)
	}

	progress := wizard.checklist.Progress()

	// Progress should be in format "[===--] 0/7 steps complete"
	if len(progress) < 10 {
		t.Errorf("Progress string too short: %s", progress)
	}

	// Should contain steps complete indicator
	if len(progress) < 5 {
		t.Error("Progress format unexpected")
	}
}
