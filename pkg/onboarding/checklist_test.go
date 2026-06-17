package onboarding

import (
	"fmt"
	"testing"
	"time"
)

func TestChecklist_SequentialCompletion(t *testing.T) {
	secret := "test-secret-that-is-long-enough-for-validation"
	opts := WizardOptions{
		Secret:              secret,
		InterfaceName:       "wg0",
		SkipRegistry:        true,
		DisableLANDiscovery: true,
		PeerTimeout:         1 * time.Minute,
	}

	checklist := NewChecklist(secret, opts)

	if checklist == nil {
		t.Fatal("NewChecklist returned nil")
	}

	// Verify all items are initialized
	if len(checklist.items) != len(ChecklistSteps) {
		t.Errorf("Expected %d items, got %d", len(ChecklistSteps), len(checklist.items))
	}

	// Verify initial state
	if checklist.CompletedCount() != 0 {
		t.Errorf("Expected 0 completed items, got %d", checklist.CompletedCount())
	}

	if checklist.TotalCount() != len(ChecklistSteps) {
		t.Errorf("Expected %d total items, got %d", len(ChecklistSteps), checklist.TotalCount())
	}

	// Test completion tracking
	current := checklist.CurrentItem()
	if current == nil {
		t.Fatal("Expected current item, got nil")
	}

	if current.Status != "pending" {
		t.Errorf("Expected pending status, got %s", current.Status)
	}

	// Mark first item as complete
	checklist.MarkComplete()
	if checklist.CompletedCount() != 1 {
		t.Errorf("Expected 1 completed item, got %d", checklist.CompletedCount())
	}

	// Advance to next item
	checklist.Advance()
	if checklist.currentIndex != 1 {
		t.Errorf("Expected index 1, got %d", checklist.currentIndex)
	}
}

func TestChecklist_Progress(t *testing.T) {
	secret := "test-secret-that-is-long-enough"
	opts := WizardOptions{
		Secret:              secret,
		InterfaceName:       "wg0",
		SkipRegistry:        true,
		DisableLANDiscovery: true,
	}

	checklist := NewChecklist(secret, opts)

	// Test initial progress
	progress := checklist.Progress()
	if progress == "" {
		t.Error("Expected progress string, got empty")
	}

	// Test progress after completing some items
	for i := 0; i < 3; i++ {
		checklist.MarkComplete()
		checklist.Advance()
	}

	progress = checklist.Progress()
	if progress == "" {
		t.Error("Expected progress string after completion, got empty")
	}

	// Verify progress format
	if len(progress) < 10 {
		t.Errorf("Progress string too short: %s", progress)
	}
}

func TestChecklist_MarkFailed(t *testing.T) {
	secret := "test-secret-that-is-long-enough"
	opts := WizardOptions{
		Secret:              secret,
		InterfaceName:       "wg0",
		SkipRegistry:        true,
		DisableLANDiscovery: true,
	}

	checklist := NewChecklist(secret, opts)

	// Mark current item as failed
	err := fmt.Errorf("test error")
	checklist.MarkFailed(err)

	current := checklist.CurrentItem()
	if current == nil {
		t.Fatal("Expected current item after failure, got nil")
	}

	if current.Status != "failed" {
		t.Errorf("Expected failed status, got %s", current.Status)
	}

	if current.Error == "" {
		t.Error("Expected error message, got empty")
	}

	if current.Error != "test error" {
		t.Errorf("Expected 'test error', got '%s'", current.Error)
	}
}

func TestChecklist_ToStore(t *testing.T) {
	secret := "test-secret-that-is-long-enough"
	opts := WizardOptions{
		Secret:              secret,
		InterfaceName:       "wg0",
		SkipRegistry:        true,
		DisableLANDiscovery: true,
	}

	checklist := NewChecklist(secret, opts)

	// Complete first 2 items
	checklist.MarkComplete()
	checklist.Advance()
	checklist.MarkComplete()

	store := checklist.ToStore()

	if store == nil {
		t.Fatal("ToStore returned nil")
	}

	if len(store.CompletedItems) != 2 {
		t.Errorf("Expected 2 completed items, got %d", len(store.CompletedItems))
	}

	if store.StartedAt.IsZero() {
		t.Error("Expected StartedAt to be set")
	}

	if store.LastUpdated.IsZero() {
		t.Error("Expected LastUpdated to be set")
	}
}

func TestChecklist_IsComplete(t *testing.T) {
	secret := "test-secret-that-is-long-enough"
	opts := WizardOptions{
		Secret:              secret,
		InterfaceName:       "wg0",
		SkipRegistry:        true,
		DisableLANDiscovery: true,
	}

	checklist := NewChecklist(secret, opts)

	// Initially not complete
	if checklist.IsComplete() {
		t.Error("Expected IsComplete to return false initially")
	}

	// Complete all items
	for i := 0; i < checklist.TotalCount(); i++ {
		checklist.MarkComplete()
		if i < checklist.TotalCount()-1 {
			checklist.Advance()
		}
	}

	// Now should be complete
	if !checklist.IsComplete() {
		t.Error("Expected IsComplete to return true after completing all items")
	}
}
