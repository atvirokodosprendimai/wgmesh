package daemon

import (
	"sync"
	"testing"
)

// Discovery goroutines read the log level from the Config (DHTDiscovery.debugf)
// while a SIGHUP reload replaces it: the Config must hand it out under its own
// lock, since the daemon's configMu is not visible to them.
func TestConfigLogLevelReadWhileReloaded(t *testing.T) {
	d := newMinimalDaemon(t)
	d.config.SetLogLevel("info")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			lvl := "debug"
			if i%2 == 0 {
				lvl = "warn"
			}
			d.reloadConfig(DaemonOpts{LogLevel: lvl})
		}
	}()
	for i := 0; i < 200; i++ {
		_ = d.config.GetLogLevel()
	}
	wg.Wait()
	if got := d.config.GetLogLevel(); got != "debug" && got != "warn" {
		t.Fatalf("log level = %q after reloads", got)
	}
}
