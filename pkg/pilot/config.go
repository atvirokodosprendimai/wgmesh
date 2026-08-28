package pilot

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// saveConfig writes the pilot configuration to a YAML file
func saveConfig(config *Config, path string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// loadConfig reads the pilot configuration from a YAML file
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate loaded config
	if config.PilotID == "" {
		return nil, fmt.Errorf("invalid config: missing pilot_id")
	}

	return &config, nil
}
