package trial

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DefaultTrialFileName is the default filename for trial state
	DefaultTrialFileName = "trial.json"
)

// Store handles persistent storage of trial state
type Store struct {
	stateDir string
	filename string
}

// NewStore creates a new trial store
func NewStore(stateDir string) *Store {
	return &Store{
		stateDir: stateDir,
		filename: filepath.Join(stateDir, DefaultTrialFileName),
	}
}

// Load loads trial state from disk
// Returns nil, nil if the file doesn't exist (trial not started yet)
func (s *Store) Load() (*TrialState, error) {
	data, err := os.ReadFile(s.filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read trial file: %w", err)
	}

	var state TrialState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse trial file: %w", err)
	}

	return &state, nil
}

// Save saves trial state to disk atomically
func (s *Store) Save(state *TrialState) error {
	if state == nil {
		return fmt.Errorf("cannot save nil state")
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal trial state: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(s.stateDir, 0700); err != nil {
		return fmt.Errorf("create directory %s: %w", s.stateDir, err)
	}

	// Write atomically
	tmpPath := s.filename + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, s.filename); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// Delete removes the trial state file
func (s *Store) Delete() error {
	if err := os.Remove(s.filename); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete trial file: %w", err)
	}
	return nil
}

// LoadOrStart loads existing trial state or creates a new one
func (s *Store) LoadOrStart(meshID string) (*TrialState, error) {
	state, err := s.Load()
	if err != nil {
		return nil, fmt.Errorf("load trial: %w", err)
	}

	if state != nil {
		// Verify mesh ID matches
		if state.MeshID != meshID {
			return nil, fmt.Errorf("mesh ID mismatch: existing=%s, new=%s", state.MeshID, meshID)
		}
		return state, nil
	}

	// Create new trial
	state, err = StartTrial(meshID)
	if err != nil {
		return nil, fmt.Errorf("start trial: %w", err)
	}

	// Save it
	if err := s.Save(state); err != nil {
		return nil, fmt.Errorf("save trial: %w", err)
	}

	return state, nil
}
