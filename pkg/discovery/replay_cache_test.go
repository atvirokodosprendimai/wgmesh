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

const testType = crypto.MessageTypeAnnounce

func TestNonceCacheRejectsRepeat(t *testing.T) {
	t.Parallel()
	clk := &fakeClock{t: time.Unix(1800000000, 0)}
	c := newNonceCache(time.Minute, 10, clk.now)

	if c.Seen(nonceN(1), testType) {
		t.Error("a nonce seen for the first time was reported as repeated")
	}
	if !c.Seen(nonceN(1), testType) {
		t.Error("the second sighting of a nonce was not reported as repeated")
	}
	if c.Seen(nonceN(2), testType) {
		t.Error("a different nonce was reported as repeated")
	}
}

func TestNonceCacheKeysOnMessageType(t *testing.T) {
	t.Parallel()
	clk := &fakeClock{t: time.Unix(1800000000, 0)}
	c := newNonceCache(time.Minute, 10, clk.now)

	c.Seen(nonceN(1), crypto.MessageTypeReply)
	if c.Seen(nonceN(1), crypto.MessageTypeAnnounce) {
		t.Error("the same nonce under another message type was reported as repeated")
	}
	if !c.Seen(nonceN(1), crypto.MessageTypeReply) {
		t.Error("the same nonce and type was not reported as repeated")
	}
}

// OpenEnvelopeRaw accepts a timestamp up to MaxMessageAge in the future, so a
// datagram first seen now stays replayable by its own timestamp for up to
// twice MaxMessageAge; the cache must refuse it for exactly that long, counted
// from the first sighting.
func TestNonceCacheExpires(t *testing.T) {
	t.Parallel()
	t0 := time.Unix(1800000000, 0)
	clk := &fakeClock{t: t0}
	c := newNonceCache(nonceCacheTTL, 10, clk.now)

	if c.Seen(nonceN(7), testType) {
		t.Fatal("first sighting reported as repeated")
	}
	clk.t = t0.Add(crypto.MaxMessageAge + time.Second)
	if !c.Seen(nonceN(7), testType) {
		t.Error("a repeat after MaxMessageAge+1s was accepted, but a future-dated packet is still valid then")
	}
	clk.t = t0.Add(nonceCacheTTL)
	if !c.Seen(nonceN(7), testType) {
		t.Error("a repeat exactly at the TTL was accepted: the entry is kept for the whole TTL, and a repeat must not refresh it")
	}
	clk.t = t0.Add(nonceCacheTTL + time.Nanosecond)
	if c.Seen(nonceN(7), testType) {
		t.Error("a nonce older than the TTL was still refused")
	}
}

func TestNonceCacheStaysUnderCap(t *testing.T) {
	t.Parallel()
	clk := &fakeClock{t: time.Unix(1800000000, 0)}
	const capacity = 100
	c := newNonceCache(time.Hour, capacity, clk.now)

	for i := uint32(0); i < capacity+50; i++ {
		c.Seen(nonceN(i), testType)
	}
	if got := c.Len(); got != capacity {
		t.Errorf("Len() = %d after %d inserts, want exactly the cap %d", got, capacity+50, capacity)
	}
	if !c.Seen(nonceN(capacity+49), testType) {
		t.Error("the newest nonce was evicted")
	}
	if c.Seen(nonceN(0), testType) {
		t.Error("the oldest nonce was still remembered after the cap was exceeded")
	}
}

// The FIFO slice is compacted once more than 1024 dead slots pile up; the
// live set must survive that and the slice must not keep growing.
func TestNonceCacheCompactsItsQueue(t *testing.T) {
	t.Parallel()
	clk := &fakeClock{t: time.Unix(1800000000, 0)}
	const capacity, inserts = 10, 5000
	c := newNonceCache(time.Hour, capacity, clk.now)

	for i := uint32(0); i < inserts; i++ {
		clk.t = clk.t.Add(time.Second)
		c.Seen(nonceN(i), testType)
	}
	if got := c.Len(); got != capacity {
		t.Errorf("Len() = %d, want %d", got, capacity)
	}
	if !c.Seen(nonceN(inserts-1), testType) {
		t.Error("the newest nonce is gone after compaction")
	}
	if c.Seen(nonceN(0), testType) {
		t.Error("the oldest nonce survived compaction")
	}
	if n := len(c.fifo); n > 2*1024+2*capacity {
		t.Errorf("queue holds %d slots for %d live entries: compaction did not run", n, capacity)
	}
}

func TestNonceCacheClampsCapacity(t *testing.T) {
	t.Parallel()
	clk := &fakeClock{t: time.Unix(1800000000, 0)}
	c := newNonceCache(time.Hour, 0, clk.now)

	if c.Seen(nonceN(1), testType) || !c.Seen(nonceN(1), testType) {
		t.Error("a cache built with capacity 0 does not behave as capacity 1")
	}
}
