package mesh

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Words: Mesh.Save replaces the centralized mesh state, plain or encrypted,
// which is the only copy of the topology.
// Why: written in place, a crash mid-write leaves a truncated file that Load
// cannot read (#870).
// Invariants: format, mode 0600 and path unchanged; both paths go through the
// same write; a failed save returns the error and keeps the old file.

func meshWithPad(filler byte) *Mesh {
	return &Mesh{
		InterfaceName: "wg0",
		Network:       "10.99.0.0/16",
		ListenPort:    51820,
		Nodes:         map[string]*Node{},
		LocalHostname: strings.Repeat(string(filler), 64<<10),
	}
}

func TestMeshSaveIsAtomic(t *testing.T) {
	for _, tc := range []struct {
		name     string
		password string
		writes   int
	}{
		{"plain", "", 300},
		// PBKDF2 makes each encrypted save slow, so fewer iterations.
		{"encrypted", "test-password", 40},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := encryptionPassword
			SetEncryptionPassword(tc.password)
			t.Cleanup(func() { SetEncryptionPassword(old) })

			dir := t.TempDir()
			path := filepath.Join(dir, "mesh-state.json")
			meshes := [2]*Mesh{meshWithPad('a'), meshWithPad('b')}
			if err := meshes[0].Save(path); err != nil {
				t.Fatal(err)
			}

			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := meshes[1].Save(path); err != nil {
				t.Fatal(err)
			}
			after, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if os.SameFile(before, after) {
				t.Fatal("state file was rewritten in place (same inode); want a temp file renamed over it")
			}
			if after.Mode().Perm() != 0o600 {
				t.Errorf("mode %o, want 600", after.Mode().Perm())
			}

			var (
				wg      sync.WaitGroup
				mu      sync.Mutex
				failure string
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
					m, err := Load(path)
					startOnce.Do(func() { close(started) })
					if err == nil && len(m.LocalHostname) != 64<<10 {
						err = fmt.Errorf("state is incomplete (local_hostname %d bytes)", len(m.LocalHostname))
					}
					if err != nil {
						mu.Lock()
						if failure == "" {
							failure = err.Error()
						}
						mu.Unlock()
						return
					}
				}
			}()
			<-started
			var werr error
			for i := 0; i < tc.writes; i++ {
				mu.Lock()
				bad := failure != ""
				mu.Unlock()
				if bad {
					break
				}
				if werr = meshes[i%2].Save(path); werr != nil {
					break
				}
			}
			close(done)
			wg.Wait()
			if werr != nil {
				t.Fatalf("save: %v", werr)
			}
			mu.Lock()
			if failure != "" {
				t.Fatalf("a reader saw a partial or unreadable state while saving: %s", failure)
			}
			mu.Unlock()

			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				if strings.Contains(e.Name(), ".tmp-") {
					t.Errorf("temp file left behind: %s", e.Name())
				}
			}
			if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
				t.Errorf("final mode/stat: %v %v", fi, err)
			}
		})
	}
}

func TestMeshSaveKeepsOldFileWhenWriteFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: a read-only directory does not stop root, the failure cannot be provoked")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "mesh-state.json")
	if err := meshWithPad('a').Save(path); err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if err := meshWithPad('b').Save(path); err == nil {
		t.Fatal("save into a read-only directory returned nil; want the error")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, old) {
		t.Error("old state file changed after a failed save")
	}
}
