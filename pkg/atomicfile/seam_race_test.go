package atomicfile

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func setFile(f func(*os.File) error) { syncFile.Store(&f) }
func setDir(f func(string) error)    { syncDir.Store(&f) }

// WriteFile is called from the peer cache saver, the node state writer and the
// service commands at once; a test replaces the sync seams meanwhile.
func TestWriteFileWhileSyncSeamsChange(t *testing.T) {
	dir := t.TempDir()
	origFile, origDir := *syncFile.Load(), *syncDir.Load()
	t.Cleanup(func() { syncFile.Store(&origFile); syncDir.Store(&origDir) })

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			setFile(func(f *os.File) error { return origFile(f) })
			setDir(func(d string) error { return origDir(d) })
		}
	}()
	defer wg.Wait()
	for i := 0; i < 50; i++ {
		if err := WriteFile(filepath.Join(dir, "t"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
