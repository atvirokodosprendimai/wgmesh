package mesh

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AccountConfig stores Lighthouse API credentials for service operations.
type AccountConfig struct {
	APIKey        string `json:"api_key"`
	LighthouseURL string `json:"lighthouse_url,omitempty"`
}

// LoadAccount reads account credentials from disk.
// Returns an error if the file does not exist (account not configured).
func LoadAccount(path string) (AccountConfig, error) {
	var cfg AccountConfig

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, fmt.Errorf("no account configured (file not found: %s)", path)
		}
		return cfg, fmt.Errorf("read account file: %w", err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse account file: %w", err)
	}

	if cfg.APIKey == "" {
		return cfg, fmt.Errorf("account file exists but api_key is empty")
	}

	return cfg, nil
}

// SaveAccount writes account credentials to disk atomically.
func SaveAccount(path string, cfg AccountConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal account config: %w", err)
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
