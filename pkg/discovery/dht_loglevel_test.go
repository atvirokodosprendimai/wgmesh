package discovery

import (
	"sync"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

// debugf runs on DHT goroutines while a SIGHUP reload sets the log level on
// the same *daemon.Config. Levels toggled here never reach "debug", so
// nothing is logged; the -race run is the assertion.
func TestDebugfRacingLogLevelReload(t *testing.T) {
	t.Parallel()
	cfg := &daemon.Config{}
	cfg.SetLogLevel("info")
	d := &DHTDiscovery{config: cfg}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			if i%2 == 0 {
				cfg.SetLogLevel("warn")
			} else {
				cfg.SetLogLevel("info")
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			d.debugf("probe %d", i)
		}
	}()
	wg.Wait()
}
