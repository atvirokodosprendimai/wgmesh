package daemon

import (
	"sync"
	"testing"
	"time"
)

// Run records the start time while the RPC goroutine reads uptime; under
// -race a bare startTime is reported here.
func TestGetUptimeConcurrentWithStart(t *testing.T) {
	d := &Daemon{}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if d.GetUptime() < 0 {
					t.Error("negative uptime")
				}
			}
		}()
	}
	for j := 0; j < 200; j++ {
		d.markStarted()
	}
	wg.Wait()
	time.Sleep(time.Millisecond)
	if up := d.GetUptime(); up <= 0 || up > time.Minute {
		t.Fatalf("GetUptime() = %v after markStarted, want a small positive duration", up)
	}
}
