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

// TryLock takes an exclusive non-blocking flock on path (created 0600) and
// writes the caller's pid into it. When another holder has it, the error
// matches ErrHeld and holder is the pid that holder wrote. release unlocks and
// closes; it is idempotent and safe to call from several goroutines.
func TryLock(path string) (release func(), holder string, err error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, "", fmt.Errorf("open lock %s: %w", path, err)
	}
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err != syscall.EINTR {
			break
		}
	}
	if err != nil {
		defer f.Close()
		if err == syscall.EWOULDBLOCK {
			raw, _ := os.ReadFile(path)
			holder = strings.TrimSpace(string(raw))
			return nil, holder, &HeldError{Path: path, Holder: holder}
		}
		return nil, "", fmt.Errorf("flock %s: %w", path, err)
	}
	if err := f.Truncate(0); err == nil {
		_, err = f.WriteAt([]byte(strconv.Itoa(os.Getpid())+"\n"), 0)
		if err != nil {
			_ = f.Close()
			return nil, "", fmt.Errorf("write pid to %s: %w", path, err)
		}
	} else {
		_ = f.Close()
		return nil, "", fmt.Errorf("truncate %s: %w", path, err)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			_ = f.Close()
		})
	}, "", nil
}
