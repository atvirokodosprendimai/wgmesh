package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

const systemdUnitTemplate = `[Unit]
Description=WireGuard Mesh Network (wgmesh)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/wgmesh/secret.env
ExecStart=/bin/sh -c 'exec {{.ExecStart}}'
Restart=always
RestartSec=5
LimitNOFILE=65535

# Security hardening
NoNewPrivileges=yes
ProtectSystem=full
ProtectHome=true
ReadWritePaths=/var/lib/wgmesh

[Install]
WantedBy=multi-user.target
`

// SystemdServiceConfig holds configuration for generating the systemd service
type SystemdServiceConfig struct {
	Secret              string
	InterfaceName       string
	ListenPort          int
	AdvertiseRoutes     []string
	Privacy             bool
	Gossip              bool
	DisableLANDiscovery bool
	Introducer          bool
	BinaryPath          string
}

// GenerateSystemdUnit generates a systemd unit file for wgmesh
func GenerateSystemdUnit(cfg SystemdServiceConfig) (string, error) {
	if cfg.BinaryPath == "" {
		// Find wgmesh binary
		path, err := exec.LookPath("wgmesh")
		if err != nil {
			path, err = filepath.Abs(os.Args[0])
			if err != nil {
				return "", fmt.Errorf("could not determine wgmesh binary path: %w", err)
			}
		}
		cfg.BinaryPath = path
	}

	// Build ExecStart command - use env var for secret to avoid exposing in process list
	args := []string{cfg.BinaryPath, "join", "--secret", "${WGMESH_SECRET}"}

	if cfg.InterfaceName != "" && cfg.InterfaceName != DefaultInterface {
		args = append(args, "--interface", cfg.InterfaceName)
	}
	if cfg.ListenPort != 0 && cfg.ListenPort != DefaultWGPort {
		args = append(args, "--listen-port", fmt.Sprintf("%d", cfg.ListenPort))
	}
	if len(cfg.AdvertiseRoutes) > 0 {
		args = append(args, "--advertise-routes", strings.Join(cfg.AdvertiseRoutes, ","))
	}
	if cfg.Privacy {
		args = append(args, "--privacy")
	}
	if cfg.Gossip {
		args = append(args, "--gossip")
	}
	if cfg.DisableLANDiscovery {
		args = append(args, "--no-lan-discovery")
	}
	if cfg.Introducer {
		args = append(args, "--introducer")
	}

	data := struct {
		ExecStart string
	}{
		ExecStart: strings.Join(args, " "),
	}

	tmpl, err := template.New("systemd").Parse(systemdUnitTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// InstallSystemdService installs and enables the wgmesh systemd service
func InstallSystemdService(cfg SystemdServiceConfig) error {
	unit, err := GenerateSystemdUnit(cfg)
	if err != nil {
		return fmt.Errorf("failed to generate unit file: %w", err)
	}

	// Write secret to environment file with restricted permissions
	secretDir := "/etc/wgmesh"
	if err := os.MkdirAll(secretDir, 0700); err != nil {
		return fmt.Errorf("failed to create secret directory (run as root?): %w", err)
	}

	// Quote the secret value for safe systemd environment file parsing
	escapedSecret := strings.ReplaceAll(cfg.Secret, `\`, `\\`)
	escapedSecret = strings.ReplaceAll(escapedSecret, `"`, `\"`)
	secretEnv := fmt.Sprintf("WGMESH_SECRET=\"%s\"\n", escapedSecret)
	secretPath := filepath.Join(secretDir, "secret.env")
	if err := os.WriteFile(secretPath, []byte(secretEnv), 0600); err != nil {
		return fmt.Errorf("failed to write secret file (run as root?): %w", err)
	}

	// Write unit file
	unitPath := "/etc/systemd/system/wgmesh.service"
	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		return fmt.Errorf("failed to write unit file (run as root?): %w", err)
	}

	// Reload systemd
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable service
	if err := exec.Command("systemctl", "enable", "wgmesh.service").Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	// Start service
	if err := exec.Command("systemctl", "start", "wgmesh.service").Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

// UninstallSystemdService stops and removes the wgmesh systemd service
func UninstallSystemdService() error {
	// Stop service
	exec.Command("systemctl", "stop", "wgmesh.service").Run()

	// Disable service
	exec.Command("systemctl", "disable", "wgmesh.service").Run()

	// Remove unit file
	unitPath := "/etc/systemd/system/wgmesh.service"
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove unit file: %w", err)
	}

	// Remove secret environment file
	secretDir := "/etc/wgmesh"
	secretPath := filepath.Join(secretDir, "secret.env")
	if err := os.Remove(secretPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove secret file: %w", err)
	}

	// Attempt to remove secret directory (ignore errors; it may not be empty or may not exist)
	_ = os.Remove(secretDir)

	// Reload systemd
	exec.Command("systemctl", "daemon-reload").Run()

	return nil
}

// ServiceStatus returns the status of the wgmesh systemd service
func ServiceStatus() (string, error) {
	cmd := exec.Command("systemctl", "is-active", "wgmesh.service")
	output, err := cmd.Output()
	if err != nil {
		return "inactive", nil
	}
	return strings.TrimSpace(string(output)), nil
}
