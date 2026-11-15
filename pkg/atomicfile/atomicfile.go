// Package atomicfile replaces a state file whole or not at all.
//
// WriteFile is the one commit step for files that hold the only copy of
// identity, cache or state: a crash, a full disk or a failed rename leaves the
// old file and no temp file behind.
//
// It is an atomic whole-file replace. There is no locking between writers: two
// writers of one file race and the last rename wins. A directory fsync that
// the filesystem does not support is ignored.
package atomicfile

import "os"

// Test seams: tests replace these to observe the order of the commit steps.
var (
	syncFile = func(f *os.File) error { return f.Sync() }
	syncDir  = func(dir string) error { return nil }
)

// WriteFile is a stub until the implementation commit.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	_ = syncFile
	_ = syncDir
	return nil
}
