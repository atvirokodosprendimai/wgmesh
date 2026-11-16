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

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// ErrHeld is matched (errors.Is) by the error TryLock returns when another
// holder has the lock.
var ErrHeld = errors.New("lock held")

// HeldError is the error TryLock returns when the lock is taken.
type HeldError struct {
	Path   string
	Holder string // pid line the holder wrote; empty when not yet written
}

func (e *HeldError) Error() string {
	if e.Holder == "" {
		return fmt.Sprintf("lock %s is held by another process", e.Path)
	}
	return fmt.Sprintf("lock %s is held by pid %s", e.Path, e.Holder)
}

// Is makes errors.Is(err, ErrHeld) true.
func (e *HeldError) Is(target error) bool { return target == ErrHeld }

// Known limit: flock locks an inode, not a name. If the lock file is unlinked
// while a holder runs, a later TryLock creates and locks a new inode. The
// directory is root-owned (0700); nothing here can detect that, and a test
// would need a hook between open and flock, which is not worth a seam.
// TryLock takes an exclusive non-blocking flock on path (created 0600) and
// writes the caller's pid into it. When another holder has it, the error
// matches ErrHeld and holder is the pid that holder wrote. release unlocks and
// closes; it is idempotent and safe to call from several goroutines.
func TryLock(path string) (release func(), holder string, err error) {
	// O_NOFOLLOW: a symlink planted at the lock path (shared directory) is
	// refused, not followed and truncated.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, "", fmt.Errorf("open lock %s: %w", path, err)
	}
	// flock can be interrupted by a signal before it decides; retry then.
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err != syscall.EINTR {
			break
		}
	}
	if err != nil {
		defer f.Close()
		if err == syscall.EWOULDBLOCK {
			// Read through the open file. The holder may not have written
			// its pid yet (it truncates and writes after locking): then
			// holder is empty.
			buf := make([]byte, 64)
			n, _ := f.ReadAt(buf, 0)
			holder = strings.TrimSpace(string(buf[:n]))
			return nil, holder, &HeldError{Path: path, Holder: holder}
		}
		return nil, "", fmt.Errorf("flock %s: %w", path, err)
	}
	if err := f.Truncate(0); err != nil {
		_ = f.Close()
		return nil, "", fmt.Errorf("truncate %s: %w", path, err)
	}
	if _, err := f.WriteAt([]byte(strconv.Itoa(os.Getpid())+"\n"), 0); err != nil {
		_ = f.Close()
		return nil, "", fmt.Errorf("write pid to %s: %w", path, err)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			_ = f.Close()
		})
	}, "", nil
}
