package atomicfile

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestWriteFileReplacesWhole(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("old content that is longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("content = %q, want %q", got, "new")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestWriteFileKeepsOldFileWhenRenameFails(t *testing.T) {
	dir := t.TempDir()
	// A non-empty directory at path makes the rename fail.
	path := filepath.Join(dir, "target")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(path, "keep")
	if err := os.WriteFile(inner, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("new"), 0o600); err == nil {
		t.Fatal("WriteFile returned nil, want the rename error")
	}
	if got, err := os.ReadFile(inner); err != nil || string(got) != "old" {
		t.Fatalf("old content = %q, %v; want it untouched", got, err)
	}
	if names := dirNames(t, dir); !reflect.DeepEqual(names, []string{"target"}) {
		t.Fatalf("directory holds %v, want only [target]", names)
	}
}

func TestWriteFileLeavesNoTempFile(t *testing.T) {
	okDir := t.TempDir()
	if err := WriteFile(filepath.Join(okDir, "target"), []byte("data"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if names := dirNames(t, okDir); !reflect.DeepEqual(names, []string{"target"}) {
		t.Fatalf("after success the directory holds %v, want only [target]", names)
	}

	failDir := t.TempDir()
	failPath := filepath.Join(failDir, "target")
	if err := os.MkdirAll(filepath.Join(failPath, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(failPath, []byte("data"), 0o600); err == nil {
		t.Fatal("WriteFile returned nil, want an error")
	}
	if names := dirNames(t, failDir); !reflect.DeepEqual(names, []string{"target"}) {
		t.Fatalf("after failure the directory holds %v, want only [target]", names)
	}
}

func TestWriteFileSyncsFileThenDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	origFile, origDir := syncFile, syncDir
	t.Cleanup(func() { syncFile, syncDir = origFile, origDir })

	var events []string
	syncFile = func(f *os.File) error {
		events = append(events, "file")
		if f.Name() == path {
			t.Error("syncFile got the target, want the temp file")
		}
		if got, _ := os.ReadFile(f.Name()); string(got) != "new" {
			t.Errorf("temp content at file sync = %q, want the data already written", got)
		}
		if got, _ := os.ReadFile(path); string(got) != "old" {
			t.Errorf("target at file sync = %q, want old (rename not yet done)", got)
		}
		return origFile(f)
	}
	syncDir = func(d string) error {
		events = append(events, "dir")
		if d != dir {
			t.Errorf("syncDir dir = %q, want %q", d, dir)
		}
		if got, _ := os.ReadFile(path); string(got) != "new" {
			t.Errorf("target at dir sync = %q, want new (rename already done)", got)
		}
		return origDir(d)
	}

	if err := WriteFile(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if want := []string{"file", "dir"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("sync order = %v, want %v", events, want)
	}
}

func TestConcurrentReaderNeverSeesPartialFile(t *testing.T) {
	const (
		writes  = 300
		size    = 1 << 20
		timeout = 60 * time.Second
	)
	a := bytes.Repeat([]byte{'a'}, size)
	b := bytes.Repeat([]byte{'b'}, size)

	path := filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(path, a, 0o600); err != nil {
		t.Fatal(err)
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		failure string
		reads   int
	)
	done := make(chan struct{})
	started := make(chan struct{})
	var startOnce sync.Once
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			got, err := os.ReadFile(path)
			startOnce.Do(func() { close(started) })
			mu.Lock()
			reads++
			if failure == "" {
				switch {
				case err != nil:
					failure = "read: " + err.Error()
				case !bytes.Equal(got, a) && !bytes.Equal(got, b):
					failure = "read a partial or mixed file"
				}
			}
			bad := failure != ""
			mu.Unlock()
			if bad {
				return
			}
		}
	}()

	// Start writing only after the reader has read the initial file once.
	<-started
	deadline := time.Now().Add(timeout)
	last := a
	var werr error
	for i := 0; i < writes; i++ {
		if time.Now().After(deadline) {
			werr = os.ErrDeadlineExceeded
			break
		}
		last = a
		if i%2 == 1 {
			last = b
		}
		if werr = WriteFile(path, last, 0o600); werr != nil {
			break
		}
	}
	close(done)
	wg.Wait()

	if werr != nil {
		t.Fatalf("write loop: %v", werr)
	}
	mu.Lock()
	defer mu.Unlock()
	if failure != "" {
		t.Fatal(failure)
	}
	if reads == 0 {
		t.Fatal("reader never ran")
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, last) {
		t.Fatalf("final content is not the last write (err %v)", err)
	}
}
