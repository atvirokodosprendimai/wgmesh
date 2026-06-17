package onboarding

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StorePath is the default path for onboarding state
const StorePath = "/var/lib/wgmesh/onboarding.json"

// Store represents the persisted onboarding state
type Store struct {
	CompletedItems []string  `json:"completed_items"`
	CurrentStep    string    `json:"current_step,omitempty"`
	StartedAt      time.Time `json:"started_at"`
	LastUpdated    time.Time `json:"last_updated"`

	mu sync.RWMutex
}

// LoadStore loads the onboarding state from disk
func LoadStore(path string) (*Store, error) {
	if path == "" {
		path = StorePath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No existing state, return empty store
			return &Store{
				CompletedItems: []string{},
				StartedAt:      time.Now(),
				LastUpdated:    time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("reading onboarding state: %w", err)
	}

	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing onboarding state: %w", err)
	}

	return &s, nil
}

// Save writes the onboarding state to disk
func (s *Store) Save(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if path == "" {
		path = StorePath
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	s.LastUpdated = time.Now()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding onboarding state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing onboarding state: %w", err)
	}

	return nil
}

// IsComplete checks if a given step ID is marked complete
func (s *Store) IsComplete(stepID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, id := range s.CompletedItems {
		if id == stepID {
			return true
		}
	}
	return false
}

// MarkComplete marks a step as complete
func (s *Store) MarkComplete(stepID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already complete
	for _, id := range s.CompletedItems {
		if id == stepID {
			return nil
		}
	}

	s.CompletedItems = append(s.CompletedItems, stepID)
	s.LastUpdated = time.Now()
	return nil
}

// SetCurrentStep sets the current step ID
func (s *Store) SetCurrentStep(stepID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.CurrentStep = stepID
	s.LastUpdated = time.Now()
	return nil
}

// Reset clears all onboarding progress
func (s *Store) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.CompletedItems = []string{}
	s.CurrentStep = ""
	s.StartedAt = time.Now()
	s.LastUpdated = time.Now()
	return nil
}

// Delete removes the onboarding state file
func Delete(path string) error {
	if path == "" {
		path = StorePath
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting onboarding state: %w", err)
	}

	return nil
}

// Duration returns the time since onboarding started
func (s *Store) Duration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.LastUpdated.Sub(s.StartedAt)
}

// Progress returns a formatted progress string
func (s *Store) Progress(totalSteps int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	completed := len(s.CompletedItems)
	if totalSteps == 0 {
		return fmt.Sprintf("%d steps complete", completed)
	}

	barWidth := 20
	filled := (completed * barWidth) / totalSteps

	bar := "["
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += "="
		} else {
			bar += "-"
		}
	}
	bar += "]"

	return fmt.Sprintf("%s %d/%d steps complete", bar, completed, totalSteps)
}
