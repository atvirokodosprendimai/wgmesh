package discovery

import "time"

// nonceCacheCapacity bounds the number of nonces the exchange path remembers.
const nonceCacheCapacity = 65536

// nonceCache is a STUB for ADR-0001 T3 step S1 (red run); implemented in S3.
type nonceCache struct{}

func newNonceCache(ttl time.Duration, capacity int, now func() time.Time) *nonceCache {
	return &nonceCache{}
}

// Seen records the nonce and reports whether it was already present.
func (c *nonceCache) Seen(nonce [12]byte) bool { return false }

// Len returns the number of remembered nonces.
func (c *nonceCache) Len() int { return 0 }
