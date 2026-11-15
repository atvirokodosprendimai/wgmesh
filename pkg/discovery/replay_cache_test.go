package discovery

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func nonceN(i uint32) [12]byte {
	var n [12]byte
	binary.BigEndian.PutUint32(n[:4], i)
	return n
}

func TestNonceCacheRejectsRepeat(t *testing.T) {
	t.Parallel()
	clk := &fakeClock{t: time.Unix(1800000000, 0)}
	c := newNonceCache(time.Minute, 10, clk.now)

	if c.Seen(nonceN(1)) {
		t.Error("a nonce seen for the first time was reported as repeated")
	}
	if !c.Seen(nonceN(1)) {
		t.Error("the second sighting of a nonce was not reported as repeated")
	}
	if c.Seen(nonceN(2)) {
		t.Error("a different nonce was reported as repeated")
	}
}

// OpenEnvelopeRaw accepts a timestamp up to MaxMessageAge in the future, so a
// datagram first seen now stays replayable by its own timestamp for up to
// twice MaxMessageAge; the cache must refuse it for that long.
func TestNonceCacheExpires(t *testing.T) {
	t.Parallel()
	clk := &fakeClock{t: time.Unix(1800000000, 0)}
	ttl := 2 * crypto.MaxMessageAge
	c := newNonceCache(ttl, 10, clk.now)

	if c.Seen(nonceN(7)) {
		t.Fatal("first sighting reported as repeated")
	}
	clk.t = clk.t.Add(crypto.MaxMessageAge + time.Second)
	if !c.Seen(nonceN(7)) {
		t.Error("a repeat after MaxMessageAge+1s was accepted, but a future-dated packet is still valid then")
	}
	clk.t = clk.t.Add(ttl)
	if c.Seen(nonceN(7)) {
		t.Error("a nonce older than the TTL was still refused")
	}
}

func TestNonceCacheStaysUnderCap(t *testing.T) {
	t.Parallel()
	clk := &fakeClock{t: time.Unix(1800000000, 0)}
	const capacity = 100
	c := newNonceCache(time.Hour, capacity, clk.now)

	for i := uint32(0); i < capacity+50; i++ {
		c.Seen(nonceN(i))
	}
	if got := c.Len(); got != capacity {
		t.Errorf("Len() = %d after %d inserts, want exactly the cap %d", got, capacity+50, capacity)
	}
	if !c.Seen(nonceN(capacity + 49)) {
		t.Error("the newest nonce was evicted")
	}
	if c.Seen(nonceN(0)) {
		t.Error("the oldest nonce was still remembered after the cap was exceeded")
	}
}
