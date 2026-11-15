package discovery

import (
	"sync"
	"time"
)

// nonceCacheCapacity bounds the number of nonces the exchange path remembers.
const nonceCacheCapacity = 65536

type nonceEntry struct {
	nonce [12]byte
	at    time.Time
}

// nonceCache remembers the nonces of datagrams that were opened successfully,
// for ttl after they were first seen and for at most capacity entries (oldest
// evicted first), so a datagram replayed inside the acceptance window of
// crypto.OpenEnvelopeRaw is recognised. Its memory is bounded by capacity.
type nonceCache struct {
	mu       sync.Mutex
	ttl      time.Duration
	capacity int
	now      func() time.Time

	seen map[[12]byte]struct{}
	fifo []nonceEntry // entries in first-seen order; fifo[head:] are live
	head int
}

func newNonceCache(ttl time.Duration, capacity int, now func() time.Time) *nonceCache {
	if capacity < 1 {
		capacity = 1
	}
	return &nonceCache{ttl: ttl, capacity: capacity, now: now, seen: make(map[[12]byte]struct{})}
}

// Seen records the nonce and reports whether it was already present. A
// repeat does not refresh the entry: the age is measured from the first sighting.
func (c *nonceCache) Seen(nonce [12]byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.now()

	for c.head < len(c.fifo) && t.Sub(c.fifo[c.head].at) >= c.ttl {
		c.drop()
	}
	if _, ok := c.seen[nonce]; ok {
		return true
	}
	for len(c.seen) >= c.capacity && c.head < len(c.fifo) {
		c.drop()
	}
	c.seen[nonce] = struct{}{}
	c.fifo = append(c.fifo, nonceEntry{nonce: nonce, at: t})
	if c.head > 1024 && c.head*2 > len(c.fifo) {
		c.fifo = append([]nonceEntry(nil), c.fifo[c.head:]...)
		c.head = 0
	}
	return false
}

func (c *nonceCache) drop() {
	delete(c.seen, c.fifo[c.head].nonce)
	c.head++
}

// Len returns the number of remembered nonces.
func (c *nonceCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.seen)
}
