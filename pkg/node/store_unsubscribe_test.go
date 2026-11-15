package node

import (
	"sync"
	"testing"
)

// Unsubscribe while Update is notifying must not panic with "send on closed
// channel" (notify sends after releasing the store lock).
func TestUnsubscribeWhileNotifyingDoesNotPanic(t *testing.T) {
	ps := NewPeerStore()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					ps.notify("k", PeerEventUpdated)
				}
			}
		}()
	}

	for i := 0; i < 20000; i++ {
		ch := ps.Subscribe()
		ps.Unsubscribe(ch)
	}
	close(stop)
	wg.Wait()
}
