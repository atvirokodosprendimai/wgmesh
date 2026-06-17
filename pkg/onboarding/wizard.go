package onboarding

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// WizardOptions holds configuration for the onboarding wizard
type WizardOptions struct {
	Secret              string
	InterfaceName       string
	SkipRegistry        bool
	DisableLANDiscovery bool
	PeerTimeout         time.Duration
	Reset               bool
	SkipTo              string
}

// Wizard manages the interactive onboarding process
type Wizard struct {
	checklist *Checklist
	store     *Store
	options   WizardOptions
	storePath string
}

// NewWizard creates a new onboarding wizard
func NewWizard(options WizardOptions, storePath string) (*Wizard, error) {
	if storePath == "" {
		storePath = StorePath
	}

	// Load existing store or create new one
	store, err := LoadStore(storePath)
	if err != nil {
		return nil, fmt.Errorf("loading onboarding state: %w", err)
	}

	// Handle reset flag
	if options.Reset {
		if err := store.Reset(); err != nil {
			return nil, fmt.Errorf("resetting onboarding state: %w", err)
		}
		fmt.Println("Onboarding progress has been reset.")
		fmt.Println()
	}

	// Create checklist
	checklist := NewChecklist(options.Secret, options)

	w := &Wizard{
		checklist: checklist,
		store:     store,
		options:   options,
		storePath: storePath,
	}

	// Restore state from store
	w.restoreState()

	return w, nil
}

// restoreState restores checklist state from the persistent store
func (w *Wizard) restoreState() {
	for _, item := range w.checklist.items {
		if w.store.IsComplete(item.Item.ID) {
			item.Status = "complete"
			item.Completed = &w.store.LastUpdated
		}
	}

	// Find current step
	for idx, item := range w.checklist.items {
		if item.Status != "complete" {
			w.checklist.currentIndex = idx
			break
		}
	}

	// If all complete, set index to end
	if w.checklist.IsComplete() {
		w.checklist.currentIndex = len(w.checklist.items)
	}
}

// Run executes the onboarding wizard
func (w *Wizard) Run() error {
	fmt.Println("wgmesh Onboarding Wizard")
	fmt.Println("========================")
	fmt.Println()

	// Handle skip-to flag
	if w.options.SkipTo != "" {
		return w.skipToStep(w.options.SkipTo)
	}

	// Show current progress
	w.showProgress()

	// Check if already complete
	if w.checklist.IsComplete() {
		fmt.Println("✓ Onboarding complete!")
		fmt.Println()
		fmt.Println("All steps have been completed. Your mesh is ready.")
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  - Monitor peer discovery: wgmesh peers list")
		fmt.Println("  - Check interface status: wg show wg0")
		fmt.Println("  - View mesh status: wgmesh status --secret <SECRET>")
		return nil
	}

	// Run through remaining steps
	for !w.checklist.IsComplete() {
		current := w.checklist.CurrentItem()
		if current == nil {
			break
		}

		// Run validation for current step
		if err := w.runStep(current); err != nil {
			// Step failed - show error and guidance
			fmt.Printf("\n✗ Step failed: %s\n", current.Item.Name)
			fmt.Printf("  Error: %v\n", err)
			fmt.Printf("  %s\n", current.Item.Remediate)
			fmt.Println()

			// Ask if user wants to continue
			if !w.promptContinue() {
				fmt.Println("Onboarding aborted.")
				w.checklist.MarkFailed(err)
				_ = w.saveState()
				return fmt.Errorf("onboarding aborted at step %s: %w", current.Item.ID, err)
			}

			// Skip this step and continue
			w.checklist.Advance()
			continue
		}

		// Step complete
		w.checklist.MarkComplete()
		fmt.Printf("\n✓ %s complete\n", current.Item.Name)

		// Save progress
		if err := w.saveState(); err != nil {
			fmt.Printf("Warning: failed to save progress: %v\n", err)
		}

		// Move to next step
		w.checklist.Advance()

		// Small delay for user to see progress
		time.Sleep(500 * time.Millisecond)
	}

	// All complete
	fmt.Println()
	fmt.Println("✓ Onboarding complete!")
	fmt.Println()
	fmt.Println("Your mesh is ready. Here's what you can do next:")
	fmt.Println("  - Monitor peer discovery: wgmesh peers list")
	fmt.Println("  - Check interface status: wg show wg0")
	fmt.Println("  - View mesh status: wgmesh status --secret <SECRET>")

	return nil
}

// runStep executes a single validation step
func (w *Wizard) runStep(item *ChecklistItemState) error {
	fmt.Printf("\n[%s] %s\n", item.Item.ID, item.Item.Name)
	fmt.Printf("  %s\n", item.Item.Description)
	fmt.Printf("  Validating... ")

	// Mark as running
	item.Status = "running"

	// Run validation
	err := item.Item.Validate()

	return err
}

// showProgress displays current onboarding progress
func (w *Wizard) showProgress() {
	fmt.Printf("Progress: %s\n\n", w.checklist.Progress())

	// Show completed steps
	for _, item := range w.checklist.items {
		if item.Status == "complete" {
			fmt.Printf("  ✓ %s\n", item.Item.Name)
		}
	}

	if w.checklist.CompletedCount() > 0 {
		fmt.Println()
	}
}

// promptContinue asks the user if they want to continue after a failure
func (w *Wizard) promptContinue() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Continue with remaining steps? (y/N): ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

// saveState saves current checklist state to the store
func (w *Wizard) saveState() error {
	// Update store from checklist state
	storeState := w.checklist.ToStore()

	// Copy to existing store
	w.store.CompletedItems = storeState.CompletedItems
	w.store.CurrentStep = storeState.CurrentStep
	w.store.StartedAt = storeState.StartedAt
	w.store.LastUpdated = storeState.LastUpdated

	return w.store.Save(w.storePath)
}

// skipToStep jumps to a specific step (for debugging/recovery)
func (w *Wizard) skipToStep(stepID string) error {
	// Find the step
	found := false
	for i, item := range w.checklist.items {
		if item.Item.ID == stepID {
			w.checklist.currentIndex = i
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("step %s not found", stepID)
	}

	fmt.Printf("Skipping to step: %s\n", stepID)
	return w.Run()
}

// Status returns the current onboarding status
func (w *Wizard) Status() (*WizardStatus, error) {
	return &WizardStatus{
		TotalSteps:     w.checklist.TotalCount(),
		CompletedSteps: w.checklist.CompletedCount(),
		CurrentStep:    w.checklist.CurrentItem().Item.Name,
		Progress:       w.checklist.Progress(),
		StartedAt:      w.store.StartedAt,
		LastUpdated:    w.store.LastUpdated,
		Duration:       w.store.Duration(),
		Items:          w.checklist.items,
	}, nil
}

// WizardStatus represents the current wizard status
type WizardStatus struct {
	TotalSteps     int
	CompletedSteps int
	CurrentStep    string
	Progress       string
	StartedAt      time.Time
	LastUpdated    time.Time
	Duration       time.Duration
	Items          []*ChecklistItemState
}
