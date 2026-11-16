package mesh

import (
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

// List must not take m.mu twice: a recursive RLock deadlocks as soon as a
// writer queues between the two acquisitions. The window is tiny, so this
// hammers List against a writer; with the nested RLock it hangs within a
// few iterations, without it the loop finishes.
// Not parallel: it swaps os.Stdout to silence List.
func TestListDoesNotNestReadLock(t *testing.T) {
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	stdout := os.Stdout
	os.Stdout = devNull
	defer func() { os.Stdout = stdout }()

	m := &Mesh{
		Network: "10.99.0.0/16",
		Nodes:   map[string]*Node{},
		Groups:  map[string]*Group{"web": {Members: []string{}}},
	}
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("node%d", i)
		m.Nodes[name] = &Node{Hostname: name, MeshIP: net.ParseIP("10.99.0.1")}
		m.Groups["web"].Members = append(m.Groups["web"].Members, name)
	}

	done := make(chan struct{})
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			m.List()
		}
		close(done)
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			m.mu.Lock()
			m.mu.Unlock() //nolint:staticcheck // empty critical section: only queues a writer
		}
	}()

	select {
	case <-done:
		close(stop)
		wg.Wait()
	case <-time.After(10 * time.Second):
		t.Fatal("List deadlocked: nested RLock with a queued writer")
	}
}
