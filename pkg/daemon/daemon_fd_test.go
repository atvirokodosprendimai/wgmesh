package daemon

import (
	"io"
	"testing"
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
	orig := cmdExecutor
	defer func() { cmdExecutor = orig }()
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
// It implements the CommandExecutor interface.
type recordingExecutor struct {
	commands [][]string
}

func (r *recordingExecutor) LookPath(file string) (string, error) {
	return file, nil
}

func (r *recordingExecutor) Command(name string, args ...string) Command {
	r.commands = append(r.commands, append([]string{name}, args...))
	return &noopCmd{}
}

// noopCmd is a Cmd that does nothing. It implements the Command interface.
type noopCmd struct{}

func (n *noopCmd) Output() ([]byte, error)         { return nil, nil }
func (n *noopCmd) CombinedOutput() ([]byte, error) { return nil, nil }
func (n *noopCmd) Run() error                      { return nil }
func (n *noopCmd) Start() error                    { return nil }
func (n *noopCmd) Wait() error                     { return nil }
func (n *noopCmd) SetStdin(_ io.Reader)            {}
func (n *noopCmd) SetStdout(_ io.Writer)           {}
func (n *noopCmd) SetStderr(_ io.Writer)           {}
