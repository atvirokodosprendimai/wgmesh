package daemon

import (
	"io"
	"net"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)

// TestIsTunFdMode verifies the isTunFdMode helper.
func TestIsTunFdMode(t *testing.T) {
	tests := []struct {
		name  string
		tunFd int
		want  bool
	}{
		{"zero fd = normal mode", 0, false},
		{"positive fd = tun mode", 5, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{config: &Config{TunFd: tt.tunFd}}
			if got := d.isTunFdMode(); got != tt.want {
				t.Errorf("isTunFdMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSetupWireGuardFromFd verifies that setupWireGuardFromFd stores the
// base64-encoded private key in localNode and does not attempt to create or
// configure a system WireGuard interface.
func TestSetupWireGuardFromFd(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}

	d := &Daemon{
		config: &Config{
			TunFd:         5,
			TunPrivateKey: key,
		},
	}

	if err := d.setupWireGuardFromFd(); err != nil {
		t.Fatalf("setupWireGuardFromFd() error: %v", err)
	}

	if d.localNode == nil {
		t.Fatal("localNode is nil after setupWireGuardFromFd")
	}
	if d.localNode.WGPrivateKey == "" {
		t.Error("localNode.WGPrivateKey is empty after setupWireGuardFromFd")
	}
}

// TestSetupWireGuardFromFd_BadKey verifies that an invalid key length is rejected.
func TestSetupWireGuardFromFd_BadKey(t *testing.T) {
	d := &Daemon{
		config: &Config{
			TunFd:         5,
			TunPrivateKey: []byte{1, 2, 3}, // too short
		},
	}
	if err := d.setupWireGuardFromFd(); err == nil {
		t.Error("expected error for short TunPrivateKey, got nil")
	}
}

// TestTeardownWireGuardTunFdMode verifies that teardownWireGuard is a no-op
// (does not call deleteInterface) when in TunFd mode.
func TestTeardownWireGuardTunFdMode(t *testing.T) {
	// Replace cmdExecutor with a recorder to assert no wg/ip commands are run.
	oldExecutor := cmdExecutor
	defer func() { cmdExecutor = oldExecutor }()

	rec := &recordingExecutor{}
	cmdExecutor = rec

	d := &Daemon{
		config: &Config{
			InterfaceName: "wg0",
			TunFd:         5,
		},
	}
	d.teardownWireGuard()

	if len(rec.commands) != 0 {
		t.Errorf("teardownWireGuard in TunFd mode ran unexpected commands: %v", rec.commands)
	}
}

// recordingExecutor captures all commands issued through cmdExecutor.
type recordingExecutor struct {
	commands [][]string
}

func (r *recordingExecutor) LookPath(file string) (string, error) {
	return "", nil
}

func (r *recordingExecutor) Command(name string, args ...string) Command {
	r.commands = append(r.commands, append([]string{name}, args...))
	return &noopCmd{}
}

// noopCmd is a Cmd that does nothing.
type noopCmd struct{}

func (n *noopCmd) Output() ([]byte, error)         { return nil, nil }
func (n *noopCmd) CombinedOutput() ([]byte, error) { return nil, nil }
func (n *noopCmd) Run() error                      { return nil }
func (n *noopCmd) Start() error                    { return nil }
func (n *noopCmd) Wait() error                     { return nil }
func (n *noopCmd) SetStdin(_ io.Reader)            {}
func (n *noopCmd) SetStdout(_ io.Writer)           {}
func (n *noopCmd) SetStderr(_ io.Writer)           {}

// TestSetupWireGuard_CallsFdModeWhenTunFdSet verifies that setupWireGuard
// delegates to setupWireGuardFromFd when TunFd is set.
func TestSetupWireGuard_CallsFdModeWhenTunFdSet(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}

	d := &Daemon{
		config: &Config{
			TunFd:         5,
			TunPrivateKey: key,
		},
	}

	if err := d.setupWireGuard(); err != nil {
		t.Fatalf("setupWireGuard() error: %v", err)
	}

	if d.localNode == nil {
		t.Fatal("localNode is nil after setupWireGuard with TunFd")
	}
	if d.localNode.WGPrivateKey == "" {
		t.Error("localNode.WGPrivateKey is empty after setupWireGuard with TunFd")
	}
}

// TestNewConfig_WithTunFd verifies that NewConfig properly validates and
// stores TunFd and TunPrivateKey fields.
func TestNewConfig_WithTunFd(t *testing.T) {
	tests := []struct {
		name        string
		tunFd       int
		tunKey      []byte
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid fd and key",
			tunFd:   5,
			tunKey:  make([]byte, 32),
			wantErr: false,
		},
		{
			name:        "invalid key length",
			tunFd:       5,
			tunKey:      make([]byte, 16),
			wantErr:     true,
			errContains: "TunPrivateKey must be exactly 32 bytes when TunFd is set",
		},
		{
			name:        "missing key with fd",
			tunFd:       5,
			tunKey:      nil,
			wantErr:     true,
			errContains: "TunPrivateKey must be exactly 32 bytes when TunFd is set",
		},
		{
			name:    "no fd = normal mode",
			tunFd:   0,
			tunKey:  nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DaemonOpts{
				Secret:        "test-secret-long-enough-for-derivation",
				TunFd:         tt.tunFd,
				TunPrivateKey: tt.tunKey,
			}

			cfg, err := NewConfig(opts)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewConfig() expected error containing %q, got nil", tt.errContains)
				} else if tt.errContains != "" && err.Error() != tt.errContains {
					t.Errorf("NewConfig() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewConfig() unexpected error: %v", err)
			}

			if cfg.TunFd != tt.tunFd {
				t.Errorf("Config.TunFd = %d, want %d", cfg.TunFd, tt.tunFd)
			}

			if tt.tunFd != 0 {
				if len(cfg.TunPrivateKey) != 32 {
					t.Errorf("Config.TunPrivateKey length = %d, want 32", len(cfg.TunPrivateKey))
				}
			} else {
				if cfg.TunPrivateKey != nil {
					t.Errorf("Config.TunPrivateKey = %v, want nil when TunFd is 0", cfg.TunPrivateKey)
				}
			}
		})
	}
}

// TestConfig_PrefixLen_WithCustomSubnet verifies that PrefixLen returns the
// correct prefix length for custom subnets in TunFd mode.
func TestConfig_PrefixLen_WithCustomSubnet(t *testing.T) {
	keys, err := crypto.DeriveKeys("test-secret-long-enough")
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}

	cfg := &Config{
		Keys:          keys,
		CustomSubnet:  mustParseCIDR("10.0.0.0/24"),
		TunFd:         5,
		TunPrivateKey: make([]byte, 32),
	}

	if got := cfg.PrefixLen(); got != 24 {
		t.Errorf("Config.PrefixLen() = %d, want 24", got)
	}
}

// TestConfig_PrefixLen_DefaultSubnet verifies that PrefixLen returns the
// default prefix length when no custom subnet is set.
func TestConfig_PrefixLen_DefaultSubnet(t *testing.T) {
	keys, err := crypto.DeriveKeys("test-secret-long-enough")
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}

	cfg := &Config{
		Keys:          keys,
		CustomSubnet:  nil,
		TunFd:         5,
		TunPrivateKey: make([]byte, 32),
	}

	if got := cfg.PrefixLen(); got != 16 {
		t.Errorf("Config.PrefixLen() = %d, want 16 (default)", got)
	}
}

// mustParseCIDR parses a CIDR string or panics. Used in tests.
func mustParseCIDR(cidr string) *net.IPNet {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	return ipnet
}
