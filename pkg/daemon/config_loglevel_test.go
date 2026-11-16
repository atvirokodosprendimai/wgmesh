package daemon

import (
	"sync"
	"testing"
)

// Config.LogLevel is written by the SIGHUP reload goroutine and read by the
// DHT goroutines (debugf), so every access goes through GetLogLevel and
// SetLogLevel. Run under -race: 8 goroutines set and get at once.
func TestConfigLogLevelConcurrent(t *testing.T) {
	t.Parallel()
	c := &Config{}
	c.SetLogLevel("info")

	const workers = 8
	const rounds = 500
	levels := []string{"debug", "info", "warn", "error"}
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				if w%2 == 0 {
					c.SetLogLevel(levels[(w+i)%len(levels)])
					continue
				}
				got := c.GetLogLevel()
				ok := false
				for _, l := range levels {
					if got == l {
						ok = true
					}
				}
				if !ok {
					t.Errorf("GetLogLevel = %q, not one of the set levels", got)
					return
				}
			}
		}(w)
	}
	wg.Wait()
}
