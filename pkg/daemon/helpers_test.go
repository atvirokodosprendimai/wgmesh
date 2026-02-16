package daemon

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"
)

// MockCommandExecutor is a mock implementation of CommandExecutor for testing
type MockCommandExecutor struct {
	lookPathFunc func(file string) (string, error)
	commandFunc  func(name string, args ...string) Command
}

func (m *MockCommandExecutor) LookPath(file string) (string, error) {
	if m.lookPathFunc != nil {
		return m.lookPathFunc(file)
	}
	return "", errors.New("command not found")
}

func (m *MockCommandExecutor) Command(name string, args ...string) Command {
	if m.commandFunc != nil {
		return m.commandFunc(name, args...)
	}
	return &MockCommand{}
}

// MockCommand is a mock implementation of Command for testing
type MockCommand struct {
	combinedOutputFunc func() ([]byte, error)
	outputFunc         func() ([]byte, error)
	runFunc            func() error
	startFunc          func() error
	waitFunc           func() error
	stdin              io.Reader
	stdout             io.Writer
	stderr             io.Writer
}

func (m *MockCommand) CombinedOutput() ([]byte, error) {
	if m.combinedOutputFunc != nil {
		return m.combinedOutputFunc()
	}
	return []byte{}, nil
}

func (m *MockCommand) Output() ([]byte, error) {
	if m.outputFunc != nil {
		return m.outputFunc()
	}
	return []byte{}, nil
}

func (m *MockCommand) Run() error {
	if m.runFunc != nil {
		return m.runFunc()
	}
	return nil
}

func (m *MockCommand) Start() error {
	if m.startFunc != nil {
		return m.startFunc()
	}
	return nil
}

func (m *MockCommand) Wait() error {
	if m.waitFunc != nil {
		return m.waitFunc()
	}
	return nil
}

func (m *MockCommand) SetStdin(stdin io.Reader) {
	m.stdin = stdin
}

func (m *MockCommand) SetStdout(stdout io.Writer) {
	m.stdout = stdout
}

func (m *MockCommand) SetStderr(stderr io.Writer) {
	m.stderr = stderr
}

// Helper to save and restore cmdExecutor
func withMockExecutor(t *testing.T, mock *MockCommandExecutor, fn func()) {
	t.Helper()
	oldExecutor := cmdExecutor
	cmdExecutor = mock
	defer func() {
		cmdExecutor = oldExecutor
	}()
	fn()
}

// TestInterfaceExists_Darwin_Mock tests the interfaceExists function darwin path using mocks
func TestInterfaceExists_Darwin_Mock(t *testing.T) {
	tests := []struct {
		name           string
		interfaceName  string
		ifconfigResult error
		expected       bool
	}{
		{
			name:           "interface exists",
			interfaceName:  "utun0",
			ifconfigResult: nil,
			expected:       true,
		},
		{
			name:           "interface does not exist",
			interfaceName:  "utun99",
			ifconfigResult: errors.New("ifconfig: interface utun99 does not exist"),
			expected:       false,
		},
	}

	// Only run on darwin since the function uses runtime.GOOS internally
	if runtime.GOOS != "darwin" {
		t.Skip("Test only runs on darwin")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockCommandExecutor{
				commandFunc: func(name string, args ...string) Command {
					if name == "ifconfig" && len(args) == 1 && args[0] == tt.interfaceName {
						return &MockCommand{
							runFunc: func() error {
								return tt.ifconfigResult
							},
						}
					}
					return &MockCommand{
						runFunc: func() error {
							return errors.New("unexpected command")
						},
					}
				},
			}

			withMockExecutor(t, mock, func() {
				result := interfaceExists(tt.interfaceName)
				if result != tt.expected {
					t.Errorf("interfaceExists(%s) = %v, want %v", tt.interfaceName, result, tt.expected)
				}
			})
		})
	}
}

// TestCommandExecutorInterface validates the command executor interface can be mocked
func TestCommandExecutorInterface(t *testing.T) {
	mock := &MockCommandExecutor{
		lookPathFunc: func(file string) (string, error) {
			if file == "wireguard-go" {
				return "/usr/local/bin/wireguard-go", nil
			}
			return "", errors.New("not found")
		},
		commandFunc: func(name string, args ...string) Command {
			return &MockCommand{
				combinedOutputFunc: func() ([]byte, error) {
					return []byte("success"), nil
				},
				runFunc: func() error {
					return nil
				},
			}
		},
	}

	// Test LookPath
	path, err := mock.LookPath("wireguard-go")
	if err != nil {
		t.Errorf("LookPath(wireguard-go) unexpected error: %v", err)
	}
	if path != "/usr/local/bin/wireguard-go" {
		t.Errorf("LookPath(wireguard-go) = %s, want /usr/local/bin/wireguard-go", path)
	}

	_, err = mock.LookPath("nonexistent")
	if err == nil {
		t.Error("LookPath(nonexistent) expected error, got nil")
	}

	// Test Command
	cmd := mock.Command("test", "arg1", "arg2")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("Command CombinedOutput() unexpected error: %v", err)
	}
	if string(output) != "success" {
		t.Errorf("Command CombinedOutput() = %s, want success", string(output))
	}

	err = cmd.Run()
	if err != nil {
		t.Errorf("Command Run() unexpected error: %v", err)
	}
}

// TestCreateInterface_Darwin tests the createInterface function for darwin
func TestCreateInterface_Darwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Test only runs on darwin")
	}

	tests := []struct {
		name           string
		interfaceName  string
		lookPathErr    error
		wireguardGoErr error
		ifconfigResult error
		expectError    bool
		errorContains  string
	}{
		{
			name:           "success - interface created",
			interfaceName:  "utun0",
			lookPathErr:    nil,
			wireguardGoErr: nil,
			ifconfigResult: nil, // interface exists immediately
			expectError:    false,
		},
		{
			name:          "error - wireguard-go not found",
			interfaceName: "utun0",
			lookPathErr:   errors.New("executable file not found in $PATH"),
			expectError:   true,
			errorContains: "wireguard-go not found in PATH",
		},
		{
			name:           "error - wireguard-go fails",
			interfaceName:  "utun0",
			lookPathErr:    nil,
			wireguardGoErr: errors.New("permission denied"),
			expectError:    true,
			errorContains:  "failed to start wireguard-go",
		},
		{
			name:           "error - interface not created after polling",
			interfaceName:  "utun0",
			lookPathErr:    nil,
			wireguardGoErr: nil,
			ifconfigResult: errors.New("interface does not exist"), // interface never appears
			expectError:    true,
			errorContains:  "was not created on macOS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockCommandExecutor{
				lookPathFunc: func(file string) (string, error) {
					if file == "wireguard-go" {
						return "/usr/local/bin/wireguard-go", tt.lookPathErr
					}
					return "", errors.New("command not found")
				},
				commandFunc: func(name string, args ...string) Command {
					if name == "wireguard-go" && len(args) == 1 && args[0] == tt.interfaceName {
						return &MockCommand{
							startFunc: func() error {
								return tt.wireguardGoErr
							},
						}
					}
					if name == "ifconfig" && len(args) == 1 && args[0] == tt.interfaceName {
						return &MockCommand{
							runFunc: func() error {
								return tt.ifconfigResult
							},
						}
					}
					return &MockCommand{
						startFunc: func() error {
							return errors.New("unexpected command")
						},
					}
				},
			}

			withMockExecutor(t, mock, func() {
				err := createInterface(tt.interfaceName)
				if tt.expectError {
					if err == nil {
						t.Errorf("createInterface(%s) expected error, got nil", tt.interfaceName)
					} else if !strings.Contains(err.Error(), tt.errorContains) {
						t.Errorf("createInterface(%s) error = %v, want error containing %q", tt.interfaceName, err, tt.errorContains)
					}
				} else {
					if err != nil {
						t.Errorf("createInterface(%s) unexpected error: %v", tt.interfaceName, err)
					}
				}
			})
		})
	}
}

// TestSetInterfaceAddress_Darwin tests the setInterfaceAddress function for darwin
func TestSetInterfaceAddress_Darwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Test only runs on darwin")
	}

	tests := []struct {
		name              string
		interfaceName     string
		address           string
		ifconfigOutput    []byte
		ifconfigErr       error
		routeAddOutput    []byte
		routeAddErr       error
		routeChangeOutput []byte
		routeChangeErr    error
		expectError       bool
		errorContains     string
	}{
		{
			name:           "success - address and route added",
			interfaceName:  "utun0",
			address:        "10.0.0.1/24",
			ifconfigOutput: []byte(""),
			ifconfigErr:    nil,
			routeAddOutput: []byte(""),
			routeAddErr:    nil,
			expectError:    false,
		},
		{
			name:              "success - address exists, route exists",
			interfaceName:     "utun0",
			address:           "10.0.0.1/24",
			ifconfigOutput:    []byte("File exists"),
			ifconfigErr:       errors.New("exit status 1"),
			routeAddOutput:    []byte("File exists"),
			routeAddErr:       errors.New("exit status 1"),
			routeChangeOutput: []byte(""),
			routeChangeErr:    nil,
			expectError:       false,
		},
		{
			name:          "error - invalid address format",
			interfaceName: "utun0",
			address:       "invalid",
			expectError:   true,
			errorContains: "invalid address format",
		},
		{
			name:          "error - IPv6 not supported",
			interfaceName: "utun0",
			address:       "fd00::1/64",
			expectError:   true,
			errorContains: "only IPv4 addresses are supported",
		},
		{
			name:           "error - ifconfig fails",
			interfaceName:  "utun0",
			address:        "10.0.0.1/24",
			ifconfigOutput: []byte("permission denied"),
			ifconfigErr:    errors.New("exit status 1"),
			expectError:    true,
			errorContains:  "failed to set address",
		},
		{
			name:           "error - route add fails (not file exists)",
			interfaceName:  "utun0",
			address:        "10.0.0.1/24",
			ifconfigOutput: []byte(""),
			ifconfigErr:    nil,
			routeAddOutput: []byte("network unreachable"),
			routeAddErr:    errors.New("exit status 1"),
			expectError:    true,
			errorContains:  "failed to add route",
		},
		{
			name:              "error - route change fails",
			interfaceName:     "utun0",
			address:           "10.0.0.1/24",
			ifconfigOutput:    []byte(""),
			ifconfigErr:       nil,
			routeAddOutput:    []byte("File exists"),
			routeAddErr:       errors.New("exit status 1"),
			routeChangeOutput: []byte("not in table"),
			routeChangeErr:    errors.New("exit status 1"),
			expectError:       true,
			errorContains:     "failed to update route",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockCommandExecutor{
				commandFunc: func(name string, args ...string) Command {
					if name == "ifconfig" && len(args) >= 2 && args[0] == tt.interfaceName {
						return &MockCommand{
							combinedOutputFunc: func() ([]byte, error) {
								return tt.ifconfigOutput, tt.ifconfigErr
							},
						}
					}
					if name == "route" && len(args) >= 3 {
						if args[1] == "add" {
							return &MockCommand{
								combinedOutputFunc: func() ([]byte, error) {
									return tt.routeAddOutput, tt.routeAddErr
								},
							}
						}
						if args[1] == "change" {
							return &MockCommand{
								combinedOutputFunc: func() ([]byte, error) {
									return tt.routeChangeOutput, tt.routeChangeErr
								},
							}
						}
					}
					return &MockCommand{
						combinedOutputFunc: func() ([]byte, error) {
							return []byte{}, fmt.Errorf("unexpected command: %s %v", name, args)
						},
					}
				},
			}

			withMockExecutor(t, mock, func() {
				err := setInterfaceAddress(tt.interfaceName, tt.address)
				if tt.expectError {
					if err == nil {
						t.Errorf("setInterfaceAddress(%s, %s) expected error, got nil", tt.interfaceName, tt.address)
					} else if !strings.Contains(err.Error(), tt.errorContains) {
						t.Errorf("setInterfaceAddress(%s, %s) error = %v, want error containing %q", tt.interfaceName, tt.address, err, tt.errorContains)
					}
				} else {
					if err != nil {
						t.Errorf("setInterfaceAddress(%s, %s) unexpected error: %v", tt.interfaceName, tt.address, err)
					}
				}
			})
		})
	}
}

// TestSetInterfaceUp_Darwin tests the setInterfaceUp function for darwin
func TestSetInterfaceUp_Darwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Test only runs on darwin")
	}

	tests := []struct {
		name          string
		interfaceName string
		ifconfigErr   error
		expectError   bool
	}{
		{
			name:          "success",
			interfaceName: "utun0",
			ifconfigErr:   nil,
			expectError:   false,
		},
		{
			name:          "error - ifconfig fails",
			interfaceName: "utun0",
			ifconfigErr:   errors.New("permission denied"),
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockCommandExecutor{
				commandFunc: func(name string, args ...string) Command {
					if name == "ifconfig" && len(args) == 2 && args[0] == tt.interfaceName && args[1] == "up" {
						return &MockCommand{
							combinedOutputFunc: func() ([]byte, error) {
								if tt.ifconfigErr != nil {
									return []byte(tt.ifconfigErr.Error()), tt.ifconfigErr
								}
								return []byte{}, nil
							},
						}
					}
					return &MockCommand{
						combinedOutputFunc: func() ([]byte, error) {
							return []byte{}, errors.New("unexpected command")
						},
					}
				},
			}

			withMockExecutor(t, mock, func() {
				err := setInterfaceUp(tt.interfaceName)
				if tt.expectError && err == nil {
					t.Errorf("setInterfaceUp(%s) expected error, got nil", tt.interfaceName)
				} else if !tt.expectError && err != nil {
					t.Errorf("setInterfaceUp(%s) unexpected error: %v", tt.interfaceName, err)
				}
			})
		})
	}
}

// TestSetInterfaceDown_Darwin tests the setInterfaceDown function for darwin
func TestSetInterfaceDown_Darwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Test only runs on darwin")
	}

	tests := []struct {
		name          string
		interfaceName string
		ifconfigErr   error
	}{
		{
			name:          "success",
			interfaceName: "utun0",
			ifconfigErr:   nil,
		},
		{
			name:          "ignores errors",
			interfaceName: "utun0",
			ifconfigErr:   errors.New("interface not found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockCommandExecutor{
				commandFunc: func(name string, args ...string) Command {
					if name == "ifconfig" && len(args) == 2 && args[0] == tt.interfaceName && args[1] == "down" {
						return &MockCommand{
							runFunc: func() error {
								return tt.ifconfigErr
							},
						}
					}
					return &MockCommand{
						runFunc: func() error {
							return errors.New("unexpected command")
						},
					}
				},
			}

			withMockExecutor(t, mock, func() {
				// setInterfaceDown should never return an error on darwin
				err := setInterfaceDown(tt.interfaceName)
				if err != nil {
					t.Errorf("setInterfaceDown(%s) unexpected error: %v", tt.interfaceName, err)
				}
			})
		})
	}
}
