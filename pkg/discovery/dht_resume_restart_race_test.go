package discovery

import (
	"context"
	"sync"
	"testing"
)

// OnResume's goroutine reads the run's context while a restart replaces it
// under d.mu (beginRun does the same two assignments). The read must go through
// the lock or the captured copy: under -race a bare d.ctx read fails here.
func TestOnResumeGoroutineReadsContextUnderRestart(t *testing.T) {
	d, _ := resumeDHT(t)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			d.OnResume()
			d.resumeWG.Wait()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			d.mu.Lock()
			d.cancel()
			d.ctx, d.cancel = context.WithCancel(d.parentCtx)
			d.mu.Unlock()
		}
	}()
	wg.Wait()
}
