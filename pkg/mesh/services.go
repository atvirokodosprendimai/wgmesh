package mesh

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ServiceEntry tracks a single registered service.
type ServiceEntry struct {
	SiteID       string    `json:"site_id"`
	Name         string    `json:"name"`
	Domain       string    `json:"domain"`
	LocalAddr    string    `json:"local_addr"`
	Protocol     string    `json:"protocol"`
	RegisteredAt time.Time `json:"registered_at"`
}

// ServiceState holds all locally registered services.
type ServiceState struct {
	Services map[string]ServiceEntry `json:"services"`
}

// LoadServices reads the service state from disk.
// Returns an empty state if the file does not exist.
func LoadServices(path string) (ServiceState, error) {
	state := ServiceState{Services: make(map[string]ServiceEntry)}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, fmt.Errorf("read services file: %w", err)
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("parse services file: %w", err)
	}

	if state.Services == nil {
		state.Services = make(map[string]ServiceEntry)
	}

	return state, nil
}

// SaveServices writes the service state to disk atomically (write tmp + rename).
func SaveServices(path string, state ServiceState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal services: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}
