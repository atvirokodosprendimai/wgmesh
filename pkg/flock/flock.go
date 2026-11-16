//go:build unix

// Package flock is an exclusive, non-blocking, per-file process lock.
//
// Words: TryLock claims a lock file for the life of the process and reports
// who holds it when the claim fails.
// Why: a second `wgmesh join` for a running interface unlinked the live RPC
// socket and reset the interface (#860); the kernel drops a flock when its
// holder dies, so a stale lock cannot exist.
// Invariants: the file content is the holder's pid line only; release is
// idempotent and safe from several goroutines; only the standard library;
// no package state.
package flock

import "errors"

// ErrHeld is matched (errors.Is) by the error TryLock returns when another
// holder has the lock.
var ErrHeld = errors.New("lock held")

// TryLock is a stub: it always succeeds.
func TryLock(path string) (release func(), holder string, err error) {
	return func() {}, "", nil
}
