package wireguard

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// runWG reads the wg path and the timeout from every goroutine that reads
// state; a test changes them with SetWGPath: that must not be a data race.
func TestReadStateWhileWGPathAndTimeoutChange(t *testing.T) {
	dir := fakeWG(t)
	setDump(t, dir, "wg0", dumpWG0)
	path := filepath.Join(dir, "wg")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			restore := SetWGPath(path)
			wgCommandTimeout = 10*time.Second + time.Duration(i)
			restore()
		}
	}()
	for i := 0; i < 20; i++ {
		ReadState("wg0", 0)
	}
	wg.Wait()
}
