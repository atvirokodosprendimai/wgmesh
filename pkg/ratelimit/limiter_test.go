package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	config := Config{
		MaxMessagesPerIP: 10,
		BurstSize:        5,
		MaxIPs:           100,
		CleanupInterval:  1 * time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Test that burst size allows initial messages
	ip := "192.168.1.1"
	for i := 0; i < 5; i++ {
		if !rl.Allow(ip) {
			t.Errorf("Message %d should be allowed (within burst)", i+1)
		}
	}

	// Test that burst is exhausted
	if rl.Allow(ip) {
		t.Error("Message 6 should be denied (burst exhausted)")
	}
}

func TestRateLimiterRefill(t *testing.T) {
	config := Config{
		MaxMessagesPerIP: 10,
		BurstSize:        5,
		MaxIPs:           100,
		CleanupInterval:  1 * time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	ip := "192.168.1.1"

	// Exhaust burst
	for i := 0; i < 5; i++ {
		rl.Allow(ip)
	}

	// Should be denied
	if rl.Allow(ip) {
		t.Error("Should be denied after burst exhausted")
	}

	// Wait for 0.2 seconds (should get 2 tokens at 10/sec)
	time.Sleep(200 * time.Millisecond)

	// Should allow 2 more messages
	for i := 0; i < 2; i++ {
		if !rl.Allow(ip) {
			t.Errorf("Message %d after refill should be allowed", i+1)
		}
	}

	// Should be denied again
	if rl.Allow(ip) {
		t.Error("Should be denied after refilled tokens exhausted")
	}
}

func TestRateLimiterMultipleIPs(t *testing.T) {
	config := Config{
		MaxMessagesPerIP: 2,
		BurstSize:        5,
		MaxIPs:           100,
		CleanupInterval:  1 * time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Multiple IPs should have independent limits
	ips := []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}

	for _, ip := range ips {
		for i := 0; i < 5; i++ {
			if !rl.Allow(ip) {
				t.Errorf("IP %s: Message %d should be allowed", ip, i+1)
			}
		}
	}

	// All should be exhausted
	for _, ip := range ips {
		if rl.Allow(ip) {
			t.Errorf("IP %s should be denied after burst exhausted", ip)
		}
	}
}

func TestRateLimiterLRUEviction(t *testing.T) {
	config := Config{
		MaxMessagesPerIP: 10,
		BurstSize:        5,
		MaxIPs:           3, // Small cache to test eviction
		CleanupInterval:  1 * time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Fill cache
	rl.Allow("192.168.1.1")
	rl.Allow("192.168.1.2")
	rl.Allow("192.168.1.3")

	// Add 4th IP - should evict oldest (192.168.1.1)
	rl.Allow("192.168.1.4")

	// 192.168.1.1 should be evicted and re-created
	for i := 0; i < 5; i++ {
		if !rl.Allow("192.168.1.1") {
			t.Errorf("192.168.1.1 should be allowed after eviction (new bucket)")
		}
	}
}

func TestRateLimiterConcurrentAccess(t *testing.T) {
	config := Config{
		MaxMessagesPerIP: 100,
		BurstSize:        1000,
		MaxIPs:           10,
		CleanupInterval:  1 * time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	var wg sync.WaitGroup
	allowed := make([]int64, 10) // Track allowed count per goroutine

	// Spawn multiple goroutines accessing the same IP
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			ip := "192.168.1.1"
			for i := 0; i < 100; i++ {
				if rl.Allow(ip) {
					allowed[goroutineID]++
				}
				time.Sleep(1 * time.Millisecond)
			}
		}(g)
	}

	wg.Wait()

	// Total should be burst size (1000)
	total := int64(0)
	for _, count := range allowed {
		total += count
	}

	if total != 1000 {
		t.Errorf("Expected total allowed messages to be 1000, got %d", total)
	}
}

func TestRateLimiterStats(t *testing.T) {
	config := Config{
		MaxMessagesPerIP: 10,
		BurstSize:        5,
		MaxIPs:           100,
		CleanupInterval:  1 * time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Add some IPs
	rl.Allow("192.168.1.1")
	rl.Allow("192.168.1.2")
	rl.Allow("192.168.1.3")

	stats := rl.Stats()
	if tracked, ok := stats["tracked_ips"].(int); !ok || tracked != 3 {
		t.Errorf("Expected tracked_ips to be 3, got %v", stats["tracked_ips"])
	}
	if max, ok := stats["max_ips"].(int); !ok || max != 100 {
		t.Errorf("Expected max_ips to be 100, got %v", stats["max_ips"])
	}
	if maxSec, ok := stats["max_per_sec"].(int); !ok || maxSec != 10 {
		t.Errorf("Expected max_per_sec to be 10, got %v", stats["max_per_sec"])
	}
	if burst, ok := stats["burst_size"].(int); !ok || burst != 5 {
		t.Errorf("Expected burst_size to be 5, got %v", stats["burst_size"])
	}
}

func TestRateLimiterDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	rl := NewRateLimiter(config)
	defer rl.Stop()

	if config.MaxMessagesPerIP != 10 {
		t.Errorf("Expected default MaxMessagesPerIP to be 10, got %d", config.MaxMessagesPerIP)
	}
	if config.BurstSize != 20 {
		t.Errorf("Expected default BurstSize to be 20, got %d", config.BurstSize)
	}
	if config.MaxIPs != 1000 {
		t.Errorf("Expected default MaxIPs to be 1000, got %d", config.MaxIPs)
	}
	if config.CleanupInterval != 5*time.Minute {
		t.Errorf("Expected default CleanupInterval to be 5 minutes, got %v", config.CleanupInterval)
	}
}

func TestRateLimiterZeroConfig(t *testing.T) {
	config := Config{
		MaxMessagesPerIP: 0,
		BurstSize:        0,
		MaxIPs:           0,
		CleanupInterval:  0,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Should use defaults
	rl.Allow("192.168.1.1")

	stats := rl.Stats()
	if max, ok := stats["max_ips"].(int); !ok || max != 1000 {
		t.Errorf("Expected zero config to use default max_ips, got %v", max)
	}
}

func TestRateLimiterCleanup(t *testing.T) {
	config := Config{
		MaxMessagesPerIP: 10,
		BurstSize:        5,
		MaxIPs:           100,
		CleanupInterval:  100 * time.Millisecond, // Short interval for testing
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Add IPs
	rl.Allow("192.168.1.1")
	rl.Allow("192.168.1.2")

	stats := rl.Stats()
	if tracked, ok := stats["tracked_ips"].(int); !ok || tracked != 2 {
		t.Errorf("Expected 2 tracked IPs initially, got %d", tracked)
	}

	// Wait for cleanup cycles to run
	time.Sleep(400 * time.Millisecond)

	// Cleanup should have run without errors
	// Verify IPs are still tracked (they're active)
	stats = rl.Stats()
	if tracked, ok := stats["tracked_ips"].(int); !ok || tracked < 2 {
		t.Errorf("Expected at least 2 tracked IPs after cleanup, got %d", tracked)
	}
}

func BenchmarkRateLimiterAllow(b *testing.B) {
	config := Config{
		MaxMessagesPerIP: 100,
		BurstSize:        1000,
		MaxIPs:           100,
		CleanupInterval:  1 * time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	ip := "192.168.1.1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow(ip)
	}
}

func BenchmarkRateLimiterAllowParallel(b *testing.B) {
	config := Config{
		MaxMessagesPerIP: 1000,
		BurstSize:        10000,
		MaxIPs:           100,
		CleanupInterval:  1 * time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ip := "192.168.1.1"
		for pb.Next() {
			rl.Allow(ip)
		}
	})
}
