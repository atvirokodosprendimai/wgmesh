package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Words: saveLocalNode and SavePeerCache replace a state file that is the only
// copy of the node identity / the peer cache.
// Why: written in place, a crash mid-write leaves a truncated file; the loader
// then treats it as absent and a new key and mesh IP are generated (#870).
// Invariants: format, field names, mode 0600 and path unchanged; a failed save
// returns an error and leaves the old file byte-identical.

// replacedByRename fails the test unless save puts a new file (a new inode) at
// path: an in-place os.WriteFile keeps the inode, a temp file + rename does not.
// This is the deterministic check; the reader below is the probabilistic one.
func replacedByRename(t *testing.T, path string, save func() error) {
	t.Helper()
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := save(); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Fatalf("%s was rewritten in place (same inode); want a temp file renamed over it", path)
	}
}

// readWhileSaving runs save(i) for i in [0, writes) while a goroutine reads
// path in a loop; it fails if any read errors or check rejects the bytes read.
// The number of saves is fixed, so there is no wall-clock deadline.
func readWhileSaving(t *testing.T, path string, writes int, save func(i int) error, check func([]byte) error) {
	t.Helper()
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
			got, err := os.ReadFile(path)
			startOnce.Do(func() { close(started) })
			if err == nil {
				err = check(got)
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
	for i := 0; i < writes; i++ {
		mu.Lock()
		bad := failure != ""
		mu.Unlock()
		if bad {
			break
		}
		if werr = save(i); werr != nil {
			break
		}
	}
	close(done)
	wg.Wait()
	if werr != nil {
		t.Fatalf("save: %v", werr)
	}
	mu.Lock()
	defer mu.Unlock()
	if failure != "" {
		t.Fatalf("a reader saw a partial or unreadable file while saving: %s", failure)
	}
}

// onlyEntries fails the test unless dir holds exactly the want names: a leftover
// temp file is caught whatever its name, so the check does not depend on the
// temp-name prefix.
func onlyEntries(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{}
	for _, w := range want {
		wanted[w] = true
	}
	var unexpected []string
	seen := map[string]bool{}
	for _, e := range entries {
		seen[e.Name()] = true
		if !wanted[e.Name()] {
			unexpected = append(unexpected, e.Name())
		}
	}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("expected entry %q is missing from %s", w, dir)
		}
	}
	if len(unexpected) > 0 {
		t.Errorf("unexpected entries in %s: %q; want only %q", dir, unexpected, want)
	}
}

func requireMode0600(t *testing.T, path string) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %o, want 600", fi.Mode().Perm())
	}
}

func TestSaveLocalNodeIsAtomic(t *testing.T) {
	const size = 64 << 10
	pad := [2]string{strings.Repeat("a", size), strings.Repeat("b", size)}
	path := filepath.Join(t.TempDir(), "wg0.json")
	node := func(i int) *LocalNode {
		return &LocalNode{WGPubKey: "pub", WGPrivateKey: "priv", MeshIP: pad[i%2]}
	}
	if err := saveLocalNode(path, node(0)); err != nil {
		t.Fatal(err)
	}

	replacedByRename(t, path, func() error { return saveLocalNode(path, node(1)) })
	requireMode0600(t, path)

	readWhileSaving(t, path, 300,
		func(i int) error { return saveLocalNode(path, node(i)) },
		func(b []byte) error {
			var s localNodeState
			if err := json.Unmarshal(b, &s); err != nil {
				return fmt.Errorf("%d bytes do not parse: %w", len(b), err)
			}
			if s.WGPubKey != "pub" || len(s.MeshIP) != size {
				return fmt.Errorf("parsed state is incomplete (%d bytes, mesh_ip %d)", len(b), len(s.MeshIP))
			}
			return nil
		})

	got, err := loadLocalNode(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.WGPrivateKey != "priv" {
		t.Errorf("private key %q after saves", got.WGPrivateKey)
	}
	requireMode0600(t, path)
	onlyEntries(t, filepath.Dir(path), filepath.Base(path))
}

func TestSaveLocalNodeKeepsOldFileWhenWriteFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: a read-only directory does not stop root, the failure cannot be provoked")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "wg0.json")
	if err := saveLocalNode(path, &LocalNode{WGPubKey: "old", WGPrivateKey: "old-priv"}); err != nil {
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

	if err := saveLocalNode(path, &LocalNode{WGPubKey: "new", WGPrivateKey: "new-priv"}); err == nil {
		t.Fatal("save into a read-only directory returned nil; want the error")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, old) {
		t.Errorf("old file changed after a failed save:\n%s", got)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	onlyEntries(t, dir, filepath.Base(path))
}

// peerStoreOf returns a store with n peers whose hostnames are padded with
// the filler byte, so two stores differ in every entry and are large.
func peerStoreOf(n int, filler byte) *PeerStore {
	ps := NewPeerStore()
	host := strings.Repeat(string(filler), 256)
	for i := 0; i < n; i++ {
		ps.Update(&PeerInfo{
			WGPubKey: fmt.Sprintf("key-%04d", i),
			Hostname: host,
			MeshIP:   fmt.Sprintf("10.0.%d.%d", i/250, i%250+1),
			Endpoint: fmt.Sprintf("192.0.2.%d:51820", i%250+1),
		}, "test")
	}
	return ps
}

func TestSavePeerCacheIsAtomic(t *testing.T) {
	// 1000 is the peer store's cap (node.DefaultMaxPeers).
	const peers = 1000
	dir := t.TempDir()
	old := localNodeStateDir
	localNodeStateDir = dir
	t.Cleanup(func() { localNodeStateDir = old })

	stores := [2]*PeerStore{peerStoreOf(peers, 'a'), peerStoreOf(peers, 'b')}
	const iface = "wgtest"
	path := CacheFilePath(iface)
	if filepath.Dir(path) != dir {
		t.Fatalf("cache path %s is not under the test directory %s", path, dir)
	}
	if err := SavePeerCache(iface, stores[0]); err != nil {
		t.Fatal(err)
	}

	replacedByRename(t, path, func() error { return SavePeerCache(iface, stores[1]) })
	requireMode0600(t, path)

	readWhileSaving(t, path, 200,
		func(i int) error { return SavePeerCache(iface, stores[i%2]) },
		func(b []byte) error {
			var c PeerCache
			if err := json.Unmarshal(b, &c); err != nil {
				return fmt.Errorf("%d bytes do not parse: %w", len(b), err)
			}
			if len(c.Peers) != peers {
				return fmt.Errorf("cache holds %d peers, want %d", len(c.Peers), peers)
			}
			return nil
		})

	c, err := LoadPeerCache(iface)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Peers) != peers {
		t.Errorf("loaded %d peers, want %d", len(c.Peers), peers)
	}
	requireMode0600(t, path)
	onlyEntries(t, dir, filepath.Base(path))
}
