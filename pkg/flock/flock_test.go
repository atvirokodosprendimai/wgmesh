//go:build unix

package flock

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const childEnv = "WGMESH_FLOCK_TEST_CHILD"

// bound limits every blocking call in these tests: a regression that makes
// TryLock block (a missing LOCK_NB) fails the test instead of hanging it.
const bound = 10 * time.Second

// TestMain lets the test binary act as the lock-holding child of
// TestTryLockFreedWhenHolderDies. The child exits when its stdin closes.
func TestMain(m *testing.M) {
	if path := os.Getenv(childEnv); path != "" {
		if _, _, err := TryLock(path); err != nil {
			fmt.Fprintln(os.Stderr, "child:", err)
			os.Exit(2)
		}
		fmt.Println("locked")
		_, _ = io.ReadAll(os.Stdin)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// tryLock is TryLock under a bound.
func tryLock(t *testing.T, path string) (release func(), holder string, err error) {
	t.Helper()
	type result struct {
		release func()
		holder  string
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		r, h, e := TryLock(path)
		ch <- result{r, h, e}
	}()
	select {
	case r := <-ch:
		return r.release, r.holder, r.err
	case <-time.After(bound):
		t.Fatalf("TryLock(%s) blocked for %v", path, bound)
		return nil, "", nil
	}
}

func TestTryLockExcludesSecondHolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.lock")
	release, _, err := tryLock(t, path)
	if err != nil {
		t.Fatalf("first TryLock: %v", err)
	}
	defer release()
	_, _, err = tryLock(t, path)
	if err == nil {
		t.Fatal("second TryLock succeeded while the first is held")
	}
	if !errors.Is(err, ErrHeld) {
		t.Errorf("error %v does not match ErrHeld", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the lock path %q", err, path)
	}
}

func TestTryLockReportsHolderPid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "b.lock")
	release, _, err := tryLock(t, path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	_, holder, err := tryLock(t, path)
	if err == nil {
		t.Fatal("second TryLock succeeded")
	}
	if want := strconv.Itoa(os.Getpid()); holder != want {
		t.Errorf("holder = %q, want %q", holder, want)
	}
	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if want := strconv.Itoa(os.Getpid()) + "\n"; string(raw) != want {
		t.Errorf("lock file content = %q, want only the pid line %q", raw, want)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("lock file mode = %v, want 0600", fi.Mode().Perm())
	}
}

// A holder that has locked but not yet written its pid leaves the file empty:
// the contender reports no pid instead of failing.
func TestTryLockEmptyHolderHasNoPid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.lock")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	_, holder, err := tryLock(t, path)
	if !errors.Is(err, ErrHeld) {
		t.Fatalf("error = %v, want ErrHeld", err)
	}
	if holder != "" {
		t.Errorf("holder = %q, want empty", holder)
	}
	if !strings.Contains(err.Error(), "held by another process") {
		t.Errorf("error %q lacks the no-pid wording", err)
	}
}

// A longer pid line left by a dead holder must not survive behind the new one.
func TestTryLockTruncatesStalePid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stale.lock")
	if err := os.WriteFile(path, []byte("123456789012345678\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	release, _, err := tryLock(t, path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := strconv.Itoa(os.Getpid()) + "\n"; string(raw) != want {
		t.Errorf("lock file content = %q, want %q", raw, want)
	}
}

func TestTryLockReleaseAllowsNext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.lock")
	release, _, err := tryLock(t, path)
	if err != nil {
		t.Fatal(err)
	}
	release()
	release2, _, err := tryLock(t, path)
	if err != nil {
		t.Fatalf("TryLock after release: %v", err)
	}
	release2()
}

// A second release must not touch the descriptor number again: after the
// first release another file may have reused it, and a second unlock-and-close
// of a raw descriptor would drop that file's lock. Note: with *os.File inside,
// the sync.Once alone is not what protects this (a closed *os.File reports an
// invalid descriptor); the test guards the behaviour against a raw-fd rewrite.
func TestTryLockReleaseTwiceKeepsReusedDescriptor(t *testing.T) {
	dir := t.TempDir()
	release, _, err := tryLock(t, filepath.Join(dir, "first.lock"))
	if err != nil {
		t.Fatal(err)
	}
	release()
	otherPath := filepath.Join(dir, "other.lock")
	other, err := os.OpenFile(otherPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := syscall.Flock(int(other.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	release() // second call: must be a no-op
	_, _, err = tryLock(t, otherPath)
	if !errors.Is(err, ErrHeld) {
		t.Fatalf("lock of the reused descriptor was dropped by the second release: %v", err)
	}
}

func TestTryLockFreedWhenHolderDies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.lock")
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), childEnv+"="+path)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	// The parent keeps the write end of the child's stdin: the child lives
	// until the pipe closes or the watchdog kills it.
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	watchdog := time.AfterFunc(bound, func() { _ = cmd.Process.Kill() })
	defer watchdog.Stop()
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()

	buf := make([]byte, len("locked\n"))
	if _, err := io.ReadFull(out, buf); err != nil || string(buf) != "locked\n" {
		t.Fatalf("child did not report the lock: %q %v", buf, err)
	}
	_, holder, err := tryLock(t, path)
	if err == nil {
		t.Fatal("TryLock succeeded while the child holds the lock")
	}
	if want := strconv.Itoa(cmd.Process.Pid); holder != want {
		t.Errorf("holder = %q, want child pid %q", holder, want)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	release, _, err := tryLock(t, path)
	if err != nil {
		t.Fatalf("TryLock after the holder died: %v", err)
	}
	release()
}

func TestTryLockReleaseIsSafeConcurrently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "e.lock")
	release, _, err := tryLock(t, path)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); release() }()
	}
	wg.Wait()
	release2, _, err := tryLock(t, path)
	if err != nil {
		t.Fatalf("TryLock after concurrent release: %v", err)
	}
	release2()
}

// A symlink planted at the lock path (a shared directory) is not followed:
// TryLock fails and the target is not truncated or written.
func TestTryLockRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "victim")
	if err := os.WriteFile(target, []byte("precious\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	lock := filepath.Join(dir, "x.lock")
	if err := os.Symlink(target, lock); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	release, _, err := TryLock(lock)
	if err == nil {
		release()
		t.Fatal("TryLock followed a symlink at the lock path")
	}
	got, rerr := os.ReadFile(target)
	if rerr != nil || string(got) != "precious\n" {
		t.Errorf("symlink target changed: %q, %v", got, rerr)
	}
}
