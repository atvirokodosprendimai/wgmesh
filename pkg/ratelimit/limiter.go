package ratelimit

import (
	"sync"
	"time"
)

// RateLimiter implements a per-IP token bucket rate limiter with LRU eviction.
type RateLimiter struct {
	mu sync.RWMutex

	// Configuration
	maxMessagesPerSec int // Tokens added per second per IP
	burstSize         int // Maximum tokens in bucket per IP
	maxIPs            int // Maximum number of IPs to track
	cleanupInterval   time.Duration

	// State
	buckets   map[string]*tokenBucket // IP -> token bucket
	ipList    []string                // Ordered list of IPs for LRU eviction
	ipIndex   map[string]int          // IP -> position in ipList
	stopCh    chan struct{}
	cleanedUp bool
}

// tokenBucket represents a single IP's token bucket state.
type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
}

// Config holds configuration for the rate limiter.
type Config struct {
	MaxMessagesPerIP int           // Messages per second per IP (default: 10)
	BurstSize        int           // Max tokens in bucket (default: 20)
	MaxIPs           int           // Maximum IPs to track (default: 1000)
	CleanupInterval  time.Duration // IP cache cleanup interval (default: 5 minutes)
}

// DefaultConfig returns the default rate limiter configuration.
func DefaultConfig() Config {
	return Config{
		MaxMessagesPerIP: 10,
		BurstSize:        20,
		MaxIPs:           1000,
		CleanupInterval:  5 * time.Minute,
	}
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(config Config) *RateLimiter {
	if config.MaxMessagesPerIP <= 0 {
		config.MaxMessagesPerIP = 10
	}
	if config.BurstSize <= 0 {
		config.BurstSize = 20
	}
	if config.MaxIPs <= 0 {
		config.MaxIPs = 1000
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 5 * time.Minute
	}

	rl := &RateLimiter{
		maxMessagesPerSec: config.MaxMessagesPerIP,
		burstSize:         config.BurstSize,
		maxIPs:            config.MaxIPs,
		cleanupInterval:   config.CleanupInterval,
		buckets:           make(map[string]*tokenBucket),
		ipList:            make([]string, 0, config.MaxIPs),
		ipIndex:           make(map[string]int),
		stopCh:            make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a message from the given IP should be allowed.
// Returns true if the message is allowed, false if rate limited.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Get or create bucket for this IP
	bucket, exists := rl.buckets[ip]
	if !exists {
		// Create new bucket with full tokens
		bucket = &tokenBucket{
			tokens:     float64(rl.burstSize),
			lastRefill: now,
		}

		// Add to LRU tracking
		rl.addIP(ip, bucket)

		// Consume one token
		bucket.tokens -= 1.0
		bucket.lastRefill = now
		return true
	}

	// Update LRU position
	rl.touchIP(ip)

	// Refill tokens based on elapsed time
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	if elapsed > 0 {
		tokensToAdd := elapsed * float64(rl.maxMessagesPerSec)
		bucket.tokens += tokensToAdd

		// Cap at burst size
		if bucket.tokens > float64(rl.burstSize) {
			bucket.tokens = float64(rl.burstSize)
		}
	}

	// Check if we have enough tokens
	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		bucket.lastRefill = now
		return true
	}

	// Update lastRefill even when denied to track inactivity
	bucket.lastRefill = now
	return false
}

// addIP adds an IP to the LRU tracking structures.
func (rl *RateLimiter) addIP(ip string, bucket *tokenBucket) {
	// Evict oldest if we're at capacity
	if len(rl.ipList) >= rl.maxIPs {
		if len(rl.ipList) > 0 {
			oldest := rl.ipList[0]
			delete(rl.buckets, oldest)
			delete(rl.ipIndex, oldest)
			rl.ipList = rl.ipList[1:]
		}
	}

	// Add new IP
	rl.ipList = append(rl.ipList, ip)
	rl.ipIndex[ip] = len(rl.ipList) - 1
	rl.buckets[ip] = bucket
}

// touchIP updates the LRU position for an IP.
func (rl *RateLimiter) touchIP(ip string) {
	idx, exists := rl.ipIndex[ip]
	if !exists {
		return
	}

	// Move to end of list (most recently used)
	rl.ipList = append(rl.ipList[:idx], rl.ipList[idx+1:]...)
	rl.ipList = append(rl.ipList, ip)
	rl.ipIndex[ip] = len(rl.ipList) - 1
}

// cleanupLoop periodically removes stale IPs from the cache.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.cleanup()
		}
	}
}

// cleanup removes IPs with empty buckets (fully consumed and not refilled).
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	newIPList := make([]string, 0, len(rl.ipList))

	for _, ip := range rl.ipList {
		bucket, exists := rl.buckets[ip]
		if !exists {
			continue
		}

		// Check if IP has been inactive (no successful messages) for longer than cleanup interval
		timeSinceLastRefill := now.Sub(bucket.lastRefill)

		// Also check if bucket is full (meaning it's been idle long enough to refill)
		isFull := bucket.tokens >= float64(rl.burstSize)-0.001 // small epsilon for floating point

		// Remove IPs that haven't had a successful message in > cleanup interval
		// and are at or near full bucket (inactive)
		if timeSinceLastRefill > rl.cleanupInterval && isFull {
			delete(rl.buckets, ip)
			delete(rl.ipIndex, ip)
		} else {
			newIPList = append(newIPList, ip)
		}
	}

	rl.ipList = newIPList
}

// Stop stops the cleanup goroutine.
func (rl *RateLimiter) Stop() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if !rl.cleanedUp {
		close(rl.stopCh)
		rl.cleanedUp = true
	}
}

// Stats returns statistics about the rate limiter state.
func (rl *RateLimiter) Stats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return map[string]interface{}{
		"tracked_ips":      len(rl.ipList),
		"max_ips":          rl.maxIPs,
		"max_per_sec":      rl.maxMessagesPerSec,
		"burst_size":       rl.burstSize,
		"cleanup_interval": rl.cleanupInterval.String(),
	}
}
