package atomicfile

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// WriteFile is called from the peer cache saver, the node state writer and the
// service commands at once; a test replaces the sync seams meanwhile.
func TestWriteFileWhileSyncSeamsChange(t *testing.T) {
	dir := t.TempDir()
	origFile, origDir := syncFile, syncDir
	t.Cleanup(func() { syncFile, syncDir = origFile, origDir })

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			syncFile = func(f *os.File) error { return origFile(f) }
			syncDir = func(d string) error { return origDir(d) }
		}
	}()
	for i := 0; i < 50; i++ {
		if err := WriteFile(filepath.Join(dir, "t"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
}
