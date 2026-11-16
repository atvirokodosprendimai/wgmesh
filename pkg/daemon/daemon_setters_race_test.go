package daemon

import (
	"sync"
	"testing"
)

type stubLayer struct{}

func (stubLayer) Start() error { return nil }
func (stubLayer) Stop() error  { return nil }

// SetDHTDiscovery and SetRPCServer can be called while Run reads the same
// fields; concurrent calls must not race. Run under -race.
func TestSettersAreSafeUnderConcurrency(t *testing.T) {
	d, err := NewDaemon(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			d.SetDHTDiscovery(stubLayer{})
		}()
		go func() {
			defer wg.Done()
			d.SetRPCServer(stubLayer{})
		}()
	}
	wg.Wait()
}
