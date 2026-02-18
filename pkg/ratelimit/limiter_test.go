package ratelimit

import (
	"fmt"
	"testing"
	"time"
)

func TestAllowUnderLimit(t *testing.T) {
	t.Parallel()
	l := New(10, 5, 100)

	// First 5 messages should all be allowed (burst=5)
	for i := 0; i < 5; i++ {
		if !l.Allow("1.2.3.4") {
			t.Errorf("message %d should be allowed (under burst)", i)
		}
	}
}

func TestAllowExceedsBurst(t *testing.T) {
	t.Parallel()
	l := New(10, 5, 100)

	// Consume the burst
	for i := 0; i < 5; i++ {
		l.Allow("1.2.3.4")
	}

	// Next message must be denied
	if l.Allow("1.2.3.4") {
		t.Error("message beyond burst should be denied")
	}
}

func TestAllowDifferentIPsIndependent(t *testing.T) {
	t.Parallel()
	l := New(10, 2, 100)

	// Exhaust IP A
	l.Allow("10.0.0.1")
	l.Allow("10.0.0.1")
	if l.Allow("10.0.0.1") {
		t.Error("10.0.0.1 should be rate limited")
	}

	// IP B should still be allowed
	if !l.Allow("10.0.0.2") {
		t.Error("10.0.0.2 should not be rate limited (different IP)")
	}
}

func TestAllowRefillOverTime(t *testing.T) {
	t.Parallel()
	// 100 tokens/sec, burst=1 — exhausted immediately, refills after 10ms
	l := New(100, 1, 100)

	if !l.Allow("1.2.3.4") {
		t.Fatal("first message should be allowed")
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("second message should be denied (bucket empty)")
	}

	// Wait long enough for one token to refill (1/100 sec = 10ms, add margin)
	time.Sleep(20 * time.Millisecond)

	if !l.Allow("1.2.3.4") {
		t.Error("message should be allowed after refill period")
	}
}

func TestLRUEviction(t *testing.T) {
	t.Parallel()
	maxIPs := 5
	l := New(10, 10, maxIPs)

	// Fill to capacity
	for i := 0; i < maxIPs; i++ {
		ip := fmt.Sprintf("10.0.0.%d", i+1)
		l.Allow(ip)
	}

	l.mu.Lock()
	if l.lru.Len() != maxIPs {
		t.Errorf("expected %d tracked IPs, got %d", maxIPs, l.lru.Len())
	}
	l.mu.Unlock()

	// Adding one more should evict the LRU entry
	l.Allow("192.168.1.1")

	l.mu.Lock()
	if l.lru.Len() != maxIPs {
		t.Errorf("after eviction: expected %d tracked IPs, got %d", maxIPs, l.lru.Len())
	}
	l.mu.Unlock()
}

func TestAllowConcurrentSafety(t *testing.T) {
	t.Parallel()
	l := NewDefault()

	done := make(chan struct{})
	for g := 0; g < 50; g++ {
		go func(id int) {
			ip := fmt.Sprintf("10.0.%d.1", id%10)
			for i := 0; i < 100; i++ {
				l.Allow(ip)
			}
			done <- struct{}{}
		}(g)
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}

func TestReset(t *testing.T) {
	t.Parallel()
	l := New(10, 1, 100)

	// Exhaust
	l.Allow("1.2.3.4")
	if l.Allow("1.2.3.4") {
		t.Fatal("should be rate limited before reset")
	}

	l.Reset()

	// After reset tokens replenish
	if !l.Allow("1.2.3.4") {
		t.Error("should be allowed after reset")
	}
}
