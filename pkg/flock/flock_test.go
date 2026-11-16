//go:build unix

package flock

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const childEnv = "WGMESH_FLOCK_TEST_CHILD"

// TestMain lets the test binary act as the lock-holding child of
// TestTryLockFreedWhenHolderDies.
func TestMain(m *testing.M) {
	if path := os.Getenv(childEnv); path != "" {
		if _, _, err := TryLock(path); err != nil {
			fmt.Fprintln(os.Stderr, "child:", err)
			os.Exit(2)
		}
		fmt.Println("locked")
		time.Sleep(time.Minute) // killed by the parent long before this
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestTryLockExcludesSecondHolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.lock")
	release, _, err := TryLock(path)
	if err != nil {
		t.Fatalf("first TryLock: %v", err)
	}
	defer release()
	_, _, err = TryLock(path)
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
	release, _, err := TryLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	_, holder, err := TryLock(path)
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

func TestTryLockReleaseAllowsNext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.lock")
	release, _, err := TryLock(path)
	if err != nil {
		t.Fatal(err)
	}
	release()
	release() // idempotent
	release2, _, err := TryLock(path)
	if err != nil {
		t.Fatalf("TryLock after release: %v", err)
	}
	release2()
}

func TestTryLockFreedWhenHolderDies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.lock")
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), childEnv+"="+path)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	buf := make([]byte, 7)
	if _, err := out.Read(buf); err != nil || string(buf) != "locked\n"[:7] {
		t.Fatalf("child did not report the lock: %q %v", buf, err)
	}
	_, holder, err := TryLock(path)
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
	release, _, err := TryLock(path)
	if err != nil {
		t.Fatalf("TryLock after the holder died: %v", err)
	}
	release()
}

func TestTryLockReleaseIsSafeConcurrently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "e.lock")
	release, _, err := TryLock(path)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); release() }()
	}
	wg.Wait()
	release2, _, err := TryLock(path)
	if err != nil {
		t.Fatalf("TryLock after concurrent release: %v", err)
	}
	release2()
}
