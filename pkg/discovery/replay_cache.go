package discovery

import (
	"sync"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)

const (
	// nonceCacheCapacity bounds the number of nonces the exchange path remembers.
	nonceCacheCapacity = 65536

	// nonceCacheTTL is twice MaxMessageAge: crypto.OpenEnvelopeRaw accepts a
	// timestamp up to MaxMessageAge in the future, so a datagram first seen now
	// can still be replayed that long after its own timestamp goes stale.
	nonceCacheTTL = 2 * crypto.MaxMessageAge
)

// wallClock is time.Now without the monotonic reading, so ages are measured on
// the same wall clock as the timestamp check in crypto.OpenEnvelopeRaw.
func wallClock() time.Time { return time.Now().Round(0) }

// nonceKey is a nonce together with the message type it arrived as. Legacy
// envelopes do not authenticate their type, so keying on the nonce alone would
// let someone re-send a captured packet under another type and use up the
// nonce before the real packet arrives.
type nonceKey struct {
	nonce [12]byte
	typ   string
}

type nonceEntry struct {
	key nonceKey
	at  time.Time
}

// nonceCache remembers the nonces of datagrams that were opened successfully,
// for ttl after they were first seen and for at most capacity entries (oldest
// evicted first), so a datagram replayed inside the acceptance window of
// crypto.OpenEnvelopeRaw is recognised. Its memory is bounded by capacity.
type nonceCache struct {
	// mu exists because handleMessage runs in one goroutine per datagram.
	mu       sync.Mutex
	ttl      time.Duration
	capacity int
	now      func() time.Time

	seen map[nonceKey]struct{}
	fifo []nonceEntry // entries in first-seen order; fifo[head:] are live
	head int
}

func newNonceCache(ttl time.Duration, capacity int, now func() time.Time) *nonceCache {
	if capacity < 1 {
		capacity = 1
	}
	return &nonceCache{ttl: ttl, capacity: capacity, now: now, seen: make(map[nonceKey]struct{})}
}

// Seen records the nonce and message type and reports whether the pair was
// already present. A repeat does not refresh the entry: the age is measured
// from the first sighting, and an entry is kept for exactly ttl.
func (c *nonceCache) Seen(nonce [12]byte, messageType string) bool {
	key := nonceKey{nonce: nonce, typ: messageType}
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.now()

	for c.head < len(c.fifo) && t.Sub(c.fifo[c.head].at) > c.ttl {
		c.drop()
	}
	if _, ok := c.seen[key]; ok {
		return true
	}
	for len(c.seen) >= c.capacity && c.head < len(c.fifo) {
		c.drop()
	}
	c.seen[key] = struct{}{}
	c.fifo = append(c.fifo, nonceEntry{key: key, at: t})
	if c.head > 1024 && c.head*2 > len(c.fifo) {
		c.fifo = append([]nonceEntry(nil), c.fifo[c.head:]...)
		c.head = 0
	}
	return false
}

// drop forgets the oldest live entry. The caller holds mu and has checked that
// head < len(fifo); there is no bounds check here.
func (c *nonceCache) drop() {
	delete(c.seen, c.fifo[c.head].key)
	c.head++
}

// Len returns the number of remembered entries (a test seam).
func (c *nonceCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.seen)
}

// ReplayedDatagrams returns how many datagrams the nonce cache has dropped as
// replays since the exchange started.
func (pe *PeerExchange) ReplayedDatagrams() uint64 { return pe.replayDrops.Load() }
