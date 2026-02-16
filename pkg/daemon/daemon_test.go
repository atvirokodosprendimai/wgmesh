package daemon

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDaemonWaitsForGoroutinesOnShutdown(t *testing.T) {
	// Verify that cancelling the daemon context causes Wait() to block
	// until background goroutines (reconcileLoop, statusLoop) exit.
	config := &Config{
		InterfaceName: "wg-test",
		WGListenPort:  51820,
	}
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

func TestDaemonShutdownMethod(t *testing.T) {
	// Test that Shutdown() cancels context and waits for goroutines
	config := &Config{
		InterfaceName: "wg-test",
		WGListenPort:  51820,
	}
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

	done := make(chan struct{})
	go func() {
		d.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// Good
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown() did not return in time")
	}

	// Verify context was cancelled
	select {
	case <-d.ctx.Done():
		// Good
	default:
		t.Error("context was not cancelled after Shutdown()")
	}
}
