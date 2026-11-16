package main

import (
	"path/filepath"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/flock"
)

// Words: createRPCServer returns an untyped nil daemon.RPCServer when
// rpc.NewServer fails.
// Why: `return rpc.NewServer(config)` put a nil *rpc.Server inside the
// interface, so a caller testing `s != nil` took a failed server for a live
// one and called Stop on it.
// Invariants: on error the interface value itself is nil.
func TestCreateRPCServerReturnsUntypedNilOnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.sock")
	release, _, err := flock.TryLock(path + ".lock")
	if err != nil {
		t.Fatalf("TryLock: %v", err)
	}
	defer release()

	s, err := createRPCServer(nil, path)
	if err == nil {
		t.Fatal("createRPCServer on an owned socket path succeeded")
	}
	if s != nil {
		t.Errorf("createRPCServer returned a non-nil interface (%T) with error %v", s, err)
	}
}
