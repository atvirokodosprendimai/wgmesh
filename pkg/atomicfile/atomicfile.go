// Package atomicfile replaces a state file whole or not at all.
//
// WriteFile is the one commit step for files that hold the only copy of
// identity, cache or state: a crash, a full disk, a failed write, sync or
// rename leaves the old file in place.
//
// It is an atomic whole-file replace. There is no locking between writers: two
// writers of one file race and the last rename wins. A directory fsync that
// the filesystem does not support is ignored. A directory fsync that fails
// for another reason is reported after the rename: the new content is already
// at path and the temp file is already gone. After kill -9 or power loss a
// "<name>.tmp-*" file can remain in the directory; nothing sweeps it.
//
// Differences from os.WriteFile that callers must know:
//   - perm is applied exactly to the new file, not filtered by the umask.
//   - a symlink at path is replaced by a regular file, not followed.
//   - the new file is owned by the process user and group, whatever owned the
//     old file.
//   - the directory of path must be writable, not only the file.
package atomicfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
)

// Test seams: tests replace these to observe the order of the commit steps.
// They are atomic because WriteFile runs from several goroutines (cache saver,
// state writers) while a test replaces them.
var (
	syncFile atomic.Pointer[func(*os.File) error]
	syncDir  atomic.Pointer[func(string) error]
)

func init() {
	sf := func(f *os.File) error { return f.Sync() }
	sd := func(dir string) error {
		d, err := os.Open(dir)
		if err != nil {
			return err
		}
		err = d.Sync()
		if cerr := d.Close(); err == nil {
			err = cerr
		}
		return err
	}
	syncFile.Store(&sf)
	syncDir.Store(&sd)
}

// WriteFile replaces path with data, whole or not at all.
//
// The data goes to a temp file in the same directory (so the rename is
// atomic), with perm applied before the first byte is written. The temp file
// is synced and closed, renamed over path, and then the directory is synced so
// the rename itself survives a crash. The target is never opened for writing.
// If a step before the rename fails, the temp file is removed and path keeps
// its old content. If the rename fails, the same holds. If only the final
// directory sync fails, the error is returned but path already holds the new
// content.
func WriteFile(path string, data []byte, perm os.FileMode) (err error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("atomicfile: create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	closed := false
	defer func() {
		if err != nil {
			if !closed {
				tmp.Close()
			}
			os.Remove(tmpName)
		}
	}()

	if err = tmp.Chmod(perm); err != nil {
		return fmt.Errorf("atomicfile: chmod %s: %w", tmpName, err)
	}
	if _, err = tmp.Write(data); err != nil {
		return fmt.Errorf("atomicfile: write %s: %w", tmpName, err)
	}
	if err = (*syncFile.Load())(tmp); err != nil {
		return fmt.Errorf("atomicfile: sync %s: %w", tmpName, err)
	}
	closed = true
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("atomicfile: close %s: %w", tmpName, err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("atomicfile: rename %s to %s: %w", tmpName, path, err)
	}
	// The rename is done: path holds the new content, so a failed directory
	// sync is reported but the temp file is already gone.
	if serr := (*syncDir.Load())(dir); serr != nil &&
		!errors.Is(serr, syscall.EINVAL) && !errors.Is(serr, syscall.ENOTSUP) {
		return fmt.Errorf("atomicfile: sync directory %s: %w", dir, serr)
	}
	return nil
}
