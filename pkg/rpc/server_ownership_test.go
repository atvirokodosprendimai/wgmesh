package rpc

// Words: NewServer owns its socket path (lock <path>.lock) before it removes
// anything; Stop releases the lock after the socket file is gone.
// Why: the default socket path has no interface in it, so a second daemon
// unlinked the first daemon's live socket (#860, ADR-0005 M3).
// Invariants: a leftover socket nobody holds is still removed; the lock is
// freed by Stop (any number of calls, from any goroutine) and by a failed
// NewServer.

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/flock"
)

// shortSocketPath returns a socket path in a short temp dir (the platform
// limit for a unix socket path is about 104 bytes).
func shortSocketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "rpc")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "w.sock")
}

func ownershipConfig(path string) ServerConfig {
	return ServerConfig{
		SocketPath:    path,
		Version:       "test",
		GetPeers:      func() []*PeerData { return nil },
		GetPeer:       func(string) (*PeerData, bool) { return nil, false },
		GetPeerCounts: func() (int, int, int) { return 0, 0, 0 },
		GetStatus:     func() *StatusData { return &StatusData{} },
	}
}

func startServer(t *testing.T, path string) *Server {
	t.Helper()
	s, err := NewServer(ownershipConfig(path))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := s.Start(); err != nil {
		_ = s.Stop()
		t.Fatalf("Start: %v", err)
	}
	return s
}

func dialOK(t *testing.T, path string) {
	t.Helper()
	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", path, err)
	}
	_ = conn.Close()
}

// lockIsFree reports whether nobody holds <path>.lock.
func lockIsFree(t *testing.T, path string) bool {
	t.Helper()
	release, _, err := flock.TryLock(path + ".lock")
	if err != nil {
		return false
	}
	release()
	return true
}

func TestNewServerRefusesOwnedSocket(t *testing.T) {
	path := shortSocketPath(t)
	first := startServer(t, path)
	defer first.Stop()

	second, err := NewServer(ownershipConfig(path))
	if err == nil {
		_ = second.Stop()
		t.Fatal("second NewServer on an owned socket path succeeded")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the socket path %s", err, path)
	}
	if pid := strconv.Itoa(os.Getpid()); !strings.Contains(err.Error(), pid) {
		t.Errorf("error %q does not name the holder pid %s", err, pid)
	}
}

func TestNewServerLeavesLiveSocketAlone(t *testing.T) {
	path := shortSocketPath(t)
	first := startServer(t, path)
	defer first.Stop()

	if second, err := NewServer(ownershipConfig(path)); err == nil {
		_ = second.Stop()
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("first server's socket is gone after a refused second NewServer: %v", err)
	}
	dialOK(t, path)
}

func TestStopReleasesSocketLock(t *testing.T) {
	path := shortSocketPath(t)
	first := startServer(t, path)
	if err := first.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := first.Stop(); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("socket file still there after Stop: %v", err)
	}
	again := startServer(t, path)
	defer again.Stop()
	dialOK(t, path)
}

// Pin: one daemon on a host behaves as before.
func TestNewServerRemovesStaleUnownedSocket(t *testing.T) {
	path := shortSocketPath(t)
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	_ = l.Close() // leaves the socket file, nobody listens or holds a lock
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stale socket was not left behind: %v", err)
	}

	s := startServer(t, path)
	defer s.Stop()
	dialOK(t, path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("socket mode %v, want 0600", info.Mode().Perm())
	}
}

// A NewServer that fails after it took the lock frees the lock. A non-empty
// directory at the socket path makes os.Remove fail and the path "not a
// socket", which is after the lock.
func TestNewServerErrorReleasesLock(t *testing.T) {
	path := shortSocketPath(t)
	if err := os.MkdirAll(filepath.Join(path, "child"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if s, err := NewServer(ownershipConfig(path)); err == nil {
		_ = s.Stop()
		t.Fatal("NewServer on a non-empty directory path succeeded")
	}
	if !lockIsFree(t, path) {
		t.Fatal("failed NewServer left the socket lock held")
	}
}

// Stop is called from several goroutines (run under -race); the lock is free
// afterwards.
func TestStopIsSafeConcurrently(t *testing.T) {
	path := shortSocketPath(t)
	s := startServer(t, path)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Stop()
		}()
	}
	wg.Wait()
	if !lockIsFree(t, path) {
		t.Fatal("socket lock still held after Stop")
	}
}

// Start fails when the socket path is too long for sun_path (about 108 bytes
// on linux, 104 on macOS) while the lock file beside it is fine, so NewServer
// succeeds and Start fails after the lock was taken. The lock must then be
// free: the caller defers Stop only on a successful Start.
func TestStartFailureReleasesLock(t *testing.T) {
	path := filepath.Join(filepath.Dir(shortSocketPath(t)), strings.Repeat("a", 120)+".sock")
	s, err := NewServer(ownershipConfig(path))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := s.Start(); err == nil {
		_ = s.Stop()
		t.Fatal("Start on an over-long socket path succeeded")
	}
	if !lockIsFree(t, path) {
		t.Fatal("failed Start left the socket lock held")
	}
}
