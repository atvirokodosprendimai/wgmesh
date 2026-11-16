package daemon

import (
	"sync"
	"testing"
)

// SetDHTDiscoveryFactory is public: a call while Run reads the factory must not
// race. Run under -race.
func TestDHTDiscoveryFactoryIsSafeUnderConcurrency(t *testing.T) {
	old := GetDHTDiscoveryFactory()
	t.Cleanup(func() { SetDHTDiscoveryFactory(old) })

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			SetDHTDiscoveryFactory(nil)
		}()
		go func() {
			defer wg.Done()
			_ = GetDHTDiscoveryFactory()
		}()
	}
	wg.Wait()
}
