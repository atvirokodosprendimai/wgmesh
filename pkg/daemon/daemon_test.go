package daemon

import (
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)

func testConfig(t *testing.T) *Config {
	t.Helper()
	keys, err := crypto.DeriveKeys("test-secret-for-daemon-tests")
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}
	return &Config{
		InterfaceName: "wg-test",
		WGListenPort:  51820,
		Keys:          keys,
	}
}

func TestDaemonWaitsForGoroutinesOnShutdown(t *testing.T) {
	// Verify that cancelling the daemon context causes Wait() to block
	// until background goroutines (reconcileLoop, statusLoop) exit.
	config := testConfig(t)
	d, err := NewDaemon(config)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	// We need a peerStore for reconcile to work
	d.peerStore = NewPeerStore()

	// Track whether goroutines have exited
	var reconcileExited atomic.Bool
	var statusExited atomic.Bool

	// Start goroutines the same way Run() does
	d.wg.Add(2)
	go func() {
		defer d.wg.Done()
		d.reconcileLoop()
		reconcileExited.Store(true)
	}()
	go func() {
		defer d.wg.Done()
		d.statusLoop()
		statusExited.Store(true)
	}()

	// Give goroutines time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	d.cancel()

	// Wait must return (not hang)
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good — goroutines exited
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for goroutines to exit after context cancellation")
	}

	if !reconcileExited.Load() {
		t.Error("reconcileLoop did not exit after context cancellation")
	}
	if !statusExited.Load() {
		t.Error("statusLoop did not exit after context cancellation")
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"", slog.LevelInfo},
		{"invalid", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLogLevel(tt.input)
			if got != tt.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestConfigureLoggingDoesNotPanic(t *testing.T) {
	// Verify that configuring logging with various levels doesn't panic
	for _, level := range []string{"debug", "info", "warn", "error", ""} {
		configureLogging(level)
	}
}

func TestDaemonShutdownMethod(t *testing.T) {
	// Test that Shutdown() cancels context, causing goroutines to exit.
	// Callers wait for Run() to return; here we simulate with wg.Wait().
	config := testConfig(t)
	d, err := NewDaemon(config)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	d.peerStore = NewPeerStore()

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.reconcileLoop()
	}()

	time.Sleep(50 * time.Millisecond)

	// Shutdown only cancels context — does not block
	d.Shutdown()

	// Verify context was cancelled
	select {
	case <-d.ctx.Done():
		// Good
	default:
		t.Fatal("context was not cancelled after Shutdown()")
	}

	// Simulate Run()'s wait — goroutines should exit promptly
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good — goroutines exited
	case <-time.After(5 * time.Second):
		t.Fatal("goroutines did not exit after Shutdown()")
	}
}
