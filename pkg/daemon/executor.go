package daemon

import (
	"io"
	"os/exec"
)

// CommandExecutor abstracts command execution for testing
type CommandExecutor interface {
	// LookPath searches for an executable in PATH
	LookPath(file string) (string, error)
	// Command creates a new command with the given name and arguments
	Command(name string, args ...string) Command
}

// Command abstracts a command that can be executed
type Command interface {
	// CombinedOutput runs the command and returns its combined stdout and stderr
	CombinedOutput() ([]byte, error)
	// Output runs the command and returns its standard output
	Output() ([]byte, error)
	// Run runs the command and waits for it to complete
	Run() error
	// Start starts the command without waiting for it to complete
	Start() error
	// SetStdin sets the standard input for the command
	SetStdin(stdin io.Reader)
}

// RealCommandExecutor is the production implementation that uses os/exec
type RealCommandExecutor struct{}

// LookPath searches for an executable in PATH
func (r *RealCommandExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// Command creates a new command with the given name and arguments
func (r *RealCommandExecutor) Command(name string, args ...string) Command {
	return &RealCommand{cmd: exec.Command(name, args...)}
}

// RealCommand wraps exec.Cmd
type RealCommand struct {
	cmd *exec.Cmd
}

// CombinedOutput runs the command and returns its combined stdout and stderr
func (r *RealCommand) CombinedOutput() ([]byte, error) {
	return r.cmd.CombinedOutput()
}

// Output runs the command and returns its standard output
func (r *RealCommand) Output() ([]byte, error) {
	return r.cmd.Output()
}

// Run runs the command and waits for it to complete
func (r *RealCommand) Run() error {
	return r.cmd.Run()
}

// Start starts the command without waiting for it to complete
func (r *RealCommand) Start() error {
	return r.cmd.Start()
}

// SetStdin sets the standard input for the command
func (r *RealCommand) SetStdin(stdin io.Reader) {
	r.cmd.Stdin = stdin
}
