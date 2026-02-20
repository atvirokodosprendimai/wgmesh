// Package ratelimit provides per-IP token bucket rate limiting for UDP listeners.
//
// The IPRateLimiter maintains one token bucket per source IP and a fixed-size
// LRU-style cache to bound memory use. It is safe for concurrent use.
package ratelimit

import (
	"container/list"
	"sync"
	"time"
)

const (
	// DefaultRate is the default allowed messages per second per source IP.
	DefaultRate = 10
	// DefaultBurst is the default burst size (token bucket depth) per source IP.
	DefaultBurst = 20
	// DefaultMaxIPs is the maximum number of source IPs tracked simultaneously.
	// When the cache is full the least-recently-used entry is evicted.
	DefaultMaxIPs = 4096
)

// bucket is a token bucket for a single source IP.
type bucket struct {
	tokens   float64
	lastFill time.Time
}

// entry is a cached bucket with its IP key.
type entry struct {
	ip  string
	bkt *bucket
}

// IPRateLimiter rate-limits incoming messages on a per-source-IP basis using
// token buckets. An LRU eviction policy keeps memory bounded.
type IPRateLimiter struct {
	mu      sync.Mutex
	rate    float64 // tokens per second
	burst   float64 // maximum token depth
	maxIPs  int
	buckets map[string]*list.Element
	lru     *list.List
}

// New creates a new IPRateLimiter with the given rate, burst, and maximum
// number of tracked IPs.
func New(rate, burst float64, maxIPs int) *IPRateLimiter {
	if rate <= 0 {
		rate = DefaultRate
	}
	if burst <= 0 {
		burst = DefaultBurst
	} else if burst < 1 {
		burst = 1
	}
	if maxIPs <= 0 {
		maxIPs = DefaultMaxIPs
	}
	return &IPRateLimiter{
		rate:    rate,
		burst:   burst,
		maxIPs:  maxIPs,
		buckets: make(map[string]*list.Element, maxIPs),
		lru:     list.New(),
	}
}

// NewDefault creates an IPRateLimiter with DefaultRate, DefaultBurst, and DefaultMaxIPs.
func NewDefault() *IPRateLimiter {
	return New(DefaultRate, DefaultBurst, DefaultMaxIPs)
}

// consume is the internal token bucket logic. Must be called with l.mu held.
// It returns whether the request is allowed, the remaining whole tokens after
// consumption, and how long to wait before the next token is available (only
// meaningful when allowed == false).
func (l *IPRateLimiter) consume(key string, now time.Time) (allowed bool, remaining float64, retryAfter time.Duration) {
	elem, exists := l.buckets[key]
	if exists {
		bkt := elem.Value.(*entry).bkt
		// Refill tokens based on elapsed time
		elapsed := now.Sub(bkt.lastFill).Seconds()
		bkt.tokens += elapsed * l.rate
		if bkt.tokens > l.burst {
			bkt.tokens = l.burst
		}
		bkt.lastFill = now
		l.lru.MoveToFront(elem)

		if bkt.tokens < 1 {
			wait := (1 - bkt.tokens) / l.rate
			return false, 0, time.Duration(wait * float64(time.Second))
		}
		bkt.tokens--
		return true, bkt.tokens, 0
	}

	// New key: evict LRU entry if at capacity
	if l.lru.Len() >= l.maxIPs {
		oldest := l.lru.Back()
		if oldest != nil {
			l.lru.Remove(oldest)
			delete(l.buckets, oldest.Value.(*entry).ip)
		}
	}

	// Start with burst-1 tokens (consumed one for this message)
	bkt := &bucket{tokens: l.burst - 1, lastFill: now}
	e := &entry{ip: key, bkt: bkt}
	elem = l.lru.PushFront(e)
	l.buckets[key] = elem
	return true, l.burst - 1, 0
}

// Allow returns true if the message from the given IP should be processed.
// It consumes one token from the source IP's bucket. Returns false if the
// bucket is empty (rate limit exceeded).
func (l *IPRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	allowed, _, _ := l.consume(ip, time.Now())
	return allowed
}

// Reserve attempts to consume a token for the given key.
// It returns whether the request is allowed, the number of whole tokens
// remaining in the bucket after this request, and how long to wait before
// retrying (only meaningful when allowed == false).
func (l *IPRateLimiter) Reserve(key string) (allowed bool, remaining int, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ok, rem, after := l.consume(key, time.Now())
	return ok, int(rem), after
}

// Burst returns the configured burst size (maximum token depth).
func (l *IPRateLimiter) Burst() int {
	return int(l.burst)
}

// Rate returns the configured refill rate in tokens per second.
func (l *IPRateLimiter) Rate() float64 {
	return l.rate
}

// Reset clears all state. Useful for testing.
func (l *IPRateLimiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buckets = make(map[string]*list.Element, l.maxIPs)
	l.lru.Init()
}
