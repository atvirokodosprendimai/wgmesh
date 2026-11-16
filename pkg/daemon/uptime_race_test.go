package daemon

import (
	"sync"
	"testing"
)

// The RPC server calls GetUptime from one goroutine per connection; the first
// call sets the start time when Run did not (RunWithDHTDiscovery does not).
func TestGetUptimeConcurrentFirstCalls(t *testing.T) {
	d, err := NewDaemon(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				d.GetUptime()
			}
		}()
	}
	wg.Wait()
}
