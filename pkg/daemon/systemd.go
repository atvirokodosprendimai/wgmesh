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
ExecStart={{.ExecStart}}
Restart=always
RestartSec=5
LimitNOFILE=65535

# Security hardening
NoNewPrivileges=no
ProtectSystem=full
ProtectHome=true
ReadWritePaths=/var/lib/wgmesh

[Install]
WantedBy=multi-user.target
`

// SystemdServiceConfig holds configuration for generating the systemd service
type SystemdServiceConfig struct {
	Secret          string
	InterfaceName   string
	ListenPort      int
	AdvertiseRoutes []string
	Privacy         bool
	BinaryPath      string
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

	// Build ExecStart command
	args := []string{cfg.BinaryPath, "join", "--secret", fmt.Sprintf("%q", cfg.Secret)}

	if cfg.InterfaceName != "" && cfg.InterfaceName != DefaultInterface {
		args = append(args, "--interface", cfg.InterfaceName)
	}
	if cfg.ListenPort != 0 && cfg.ListenPort != DefaultWGPort {
		args = append(args, "--listen-port", fmt.Sprintf("%d", cfg.ListenPort))
	}
	if len(cfg.AdvertiseRoutes) > 0 {
		args = append(args, "--advertise-routes", strings.Join(cfg.AdvertiseRoutes, ","))
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
