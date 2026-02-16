# Specification: Issue #106

## Classification
fix

## Deliverables
code + documentation

## Problem Analysis

The exchange and gossip listeners process every incoming UDP packet without rate limiting, creating a Denial-of-Service (DoS) vulnerability. An attacker could flood the UDP ports with crafted messages, causing:

1. **Unbounded goroutine spawning**: Each packet to the exchange listener spawns a new goroutine (`pkg/discovery/exchange.go:139`)
2. **CPU exhaustion via decrypt attempts**: Both listeners call `crypto.OpenEnvelope` on every packet, which involves:
   - JSON unmarshaling of the envelope
   - AES cipher initialization
   - GCM decryption attempt
   - JSON unmarshaling of the plaintext
   - Protocol version and timestamp validation
3. **No per-IP throttling**: The only deduplication exists in DHT's `contactPeer` (`pkg/discovery/dht.go:500-509`), which throttles *outbound* contacts, not inbound messages

### Attack Vectors

**Exchange Listener** (`pkg/discovery/exchange.go:115-141`):
- Listens on UDP port 51821 (default)
- Line 139: `go pe.handleMessage(data, remoteAddr)` - spawns goroutine per packet
- No limit on concurrent goroutines
- An attacker sending 10,000 packets/second would spawn 10,000 goroutines/second

**Gossip Listener** (`pkg/discovery/gossip.go:215-251`):
- Tight loop in `listenLoop()` 
- Line 241: `crypto.OpenEnvelope(buf[:n], g.gossipKey)` - expensive decrypt on every packet
- No per-IP rate limiting or global message cap
- Processes packets synchronously in main loop (better than exchange, but still vulnerable)

### Current Mitigations (Insufficient)

1. **Encryption as filter**: Only valid messages with correct gossip key are processed, but decryption itself is expensive
2. **Read timeout**: 1-second read deadlines prevent blocking, but don't limit processing rate
3. **Outbound throttling**: DHT's `contactedPeers` map prevents rapid outbound contacts but doesn't protect inbound

### Security Impact

- **Severity**: Medium-High (P2)
- **Exploitability**: High (UDP flooding is trivial)
- **Impact**: Service degradation/unavailability, CPU/memory exhaustion
- **Scope**: All nodes running decentralized mode are vulnerable

## Proposed Approach

Implement a multi-layered rate limiting strategy:

### Layer 1: Per-IP Token Bucket Rate Limiter

Implement token bucket algorithm with per-source-IP tracking:

1. **Create new package** `pkg/ratelimit/limiter.go`:
   - `RateLimiter` struct with token bucket per IP
   - Default: 10 messages/second per source IP
   - Configurable burst size (e.g., 20 tokens)
   - LRU eviction for IP cache (prevent memory exhaustion)
   - Thread-safe with `sync.RWMutex`

2. **Token bucket algorithm**:
   - Each IP starts with full bucket (e.g., 20 tokens)
   - Tokens refill at constant rate (e.g., 10/second)
   - Incoming message consumes 1 token
   - If bucket empty, message is dropped
   - Log dropped messages at DEBUG level (avoid log flooding)

3. **Configuration options** (via daemon config):
   - `MaxMessagesPerIP`: messages per second per IP (default: 10)
   - `BurstSize`: max tokens in bucket (default: 20)
   - `IPCacheSize`: max IPs to track (default: 1000)
   - `CleanupInterval`: how often to evict stale IPs (default: 5 minutes)

### Layer 2: Bounded Goroutine Pool for Exchange

Prevent unbounded goroutine spawning in exchange listener:

1. **Semaphore-based pool**:
   - Create buffered channel of size 100 (configurable)
   - Acquire token before spawning goroutine
   - Release token when handler completes
   - If pool full, drop packet and log warning

2. **Implementation pattern**:
   ```go
   // In PeerExchange struct
   type PeerExchange struct {
       // ...existing fields...
       handlerPool chan struct{}  // Semaphore for goroutine pool
   }
   
   // In listenLoop()
   select {
   case pe.handlerPool <- struct{}{}:
       // Got token, spawn goroutine
       go func() {
           defer func() { <-pe.handlerPool }()
           pe.handleMessage(data, remoteAddr)
       }()
   default:
       // Pool full, drop packet
       log.Printf("[Exchange] Handler pool full, dropping packet from %s", remoteAddr)
   }
   ```

3. **Configuration**:
   - `MaxConcurrentHandlers`: max concurrent message handlers (default: 100)

### Layer 3: Global Message Rate Cap (Optional Safety Net)

Add simple global rate limiter as last resort:

1. **Simple counter-based limiter**:
   - Track messages processed in current second
   - If over threshold (e.g., 1000/sec globally), drop packet
   - Reset counter every second
   - Prevents catastrophic resource exhaustion even if per-IP limiter bypassed (e.g., distributed attack from many IPs)

2. **Configuration**:
   - `MaxGlobalMessagesPerSecond`: global cap (default: 1000)

### Integration Points

**Exchange** (`pkg/discovery/exchange.go`):
1. Add rate limiter to `PeerExchange` struct
2. Check rate limit before spawning goroutine in `listenLoop()` (line 139)
3. Check goroutine pool availability before spawning
4. Log rate limit violations at DEBUG level

**Gossip** (`pkg/discovery/gossip.go`):
1. Add rate limiter to `MeshGossip` struct
2. Check rate limit before calling `OpenEnvelope()` in `listenLoop()` (line 241)
3. Drop packets that exceed rate limit
4. No goroutine pool needed (synchronous processing)

**Configuration** (`pkg/daemon/config.go`):
1. Add `RateLimiting` struct to `Config`:
   ```go
   type RateLimitConfig struct {
       MaxMessagesPerIP         int
       BurstSize                int
       IPCacheSize              int
       MaxConcurrentHandlers    int
       MaxGlobalMessagesPerSec  int
   }
   ```
2. Set reasonable defaults
3. Allow override via environment variables or config file

### Testing Strategy

Rate limiting should be **observable but transparent** in normal operation:
- Under normal load: rate limiting should not drop any legitimate messages
- Under attack: rate limiting should protect the node while logging violations
- Legitimate peers should not experience service degradation

### Backward Compatibility

- No protocol changes required
- Purely local defensive measure
- Transparent to other peers
- No impact on mesh functionality

### Performance Considerations

1. **Token bucket overhead**: O(1) per message, minimal CPU/memory
2. **IP cache**: LRU eviction prevents unbounded growth
3. **Goroutine pool**: Prevents unbounded concurrency, improves resource usage
4. **Logging**: Rate limit violation logs to prevent log flooding

## Affected Files

### New Files

1. **`pkg/ratelimit/limiter.go`** (NEW - ~200 lines):
   - `RateLimiter` struct with per-IP token buckets
   - `NewRateLimiter(config)` constructor
   - `Allow(ip string) bool` - check if IP can send message
   - `Cleanup()` - periodic LRU eviction of stale IPs
   - Thread-safe with `sync.RWMutex`

2. **`pkg/ratelimit/limiter_test.go`** (NEW - ~150 lines):
   - Test basic rate limiting (allow/deny)
   - Test burst handling
   - Test IP cache eviction
   - Test concurrent access
   - Benchmark token bucket performance

### Modified Files

3. **`pkg/discovery/exchange.go`**:
   - Line ~23-39: Add `rateLimiter *ratelimit.RateLimiter` and `handlerPool chan struct{}` to `PeerExchange` struct
   - Line ~42-50: Initialize rate limiter and handler pool in `NewPeerExchange()`
   - Line ~139: Replace `go pe.handleMessage(...)` with rate-limited version:
     ```go
     // Check rate limit
     if !pe.rateLimiter.Allow(remoteAddr.IP.String()) {
         log.Printf("[Exchange] Rate limit exceeded for %s", remoteAddr)
         continue
     }
     
     // Try to acquire handler pool slot
     select {
     case pe.handlerPool <- struct{}{}:
         go func() {
             defer func() { <-pe.handlerPool }()
             pe.handleMessage(data, remoteAddr)
         }()
     default:
         log.Printf("[Exchange] Handler pool full, dropping packet from %s", remoteAddr)
     }
     ```
   - Line ~85-95: Add cleanup goroutine in `Start()` to periodically clean IP cache

4. **`pkg/discovery/gossip.go`**:
   - Line ~22-36: Add `rateLimiter *ratelimit.RateLimiter` to `MeshGossip` struct
   - Line ~39-48: Initialize rate limiter in `NewMeshGossip()`
   - Line ~51-60: Initialize rate limiter in `NewMeshGossipWithExchange()`
   - Line ~241: Add rate limit check before `OpenEnvelope()`:
     ```go
     // Check rate limit
     if !g.rateLimiter.Allow(remoteAddr.IP.String()) {
         continue  // Silently drop (avoid log flooding)
     }
     
     _, announcement, err := crypto.OpenEnvelope(buf[:n], g.gossipKey)
     ```
   - Add cleanup goroutine in `Start()` to clean IP cache

5. **`pkg/daemon/config.go`**:
   - Line ~30-40: Add `RateLimitConfig` struct:
     ```go
     type RateLimitConfig struct {
         MaxMessagesPerIP         int
         BurstSize                int
         IPCacheSize              int
         MaxConcurrentHandlers    int
         MaxGlobalMessagesPerSec  int
     }
     ```
   - Line ~45-60: Add `RateLimit RateLimitConfig` field to `Config` struct
   - In `NewConfig()`: Initialize with defaults:
     ```go
     RateLimit: RateLimitConfig{
         MaxMessagesPerIP:        10,
         BurstSize:               20,
         IPCacheSize:             1000,
         MaxConcurrentHandlers:   100,
         MaxGlobalMessagesPerSec: 1000,
     }
     ```

6. **`README.md`**:
   - Add new "Security" section documenting rate limiting
   - Explain DoS protection mechanisms
   - Document configuration options

7. **`ENCRYPTION.md`** (if exists) or create new **`SECURITY.md`**:
   - Document rate limiting as security feature
   - Explain attack mitigations
   - Provide guidance on tuning rate limits

## Test Strategy

### Unit Tests

1. **`pkg/ratelimit/limiter_test.go`**:
   - `TestRateLimiterAllow`: Verify messages allowed within rate
   - `TestRateLimiterDeny`: Verify messages denied when rate exceeded
   - `TestRateLimiterBurst`: Verify burst handling
   - `TestRateLimiterRefill`: Verify token refill over time
   - `TestRateLimiterConcurrency`: Test with concurrent goroutines
   - `TestRateLimiterIPCacheEviction`: Verify LRU eviction
   - `BenchmarkRateLimiterAllow`: Measure overhead

2. **`pkg/discovery/exchange_test.go`** (NEW):
   - `TestExchangeRateLimit`: Verify rate limiting in exchange listener
   - `TestExchangeHandlerPool`: Verify goroutine pool limits concurrency
   - `TestExchangeFlood`: Simulate flood attack, verify dropped packets

3. **`pkg/discovery/gossip_test.go`** (extend existing):
   - `TestGossipRateLimit`: Verify rate limiting in gossip listener
   - `TestGossipFlood`: Simulate flood attack, verify dropped packets

### Integration Tests

4. **Manual DoS simulation**:
   ```bash
   # Start a node
   ./wgmesh join --secret test123
   
   # Flood with UDP packets from another terminal
   for i in {1..10000}; do
       echo "flood" | nc -u localhost 51821
   done
   
   # Verify:
   # - Node remains responsive
   # - Logs show rate limit violations
   # - CPU usage stays reasonable
   # - No unbounded goroutine growth (use pprof)
   ```

5. **Distributed attack simulation**:
   - Generate flood from multiple source IPs
   - Verify per-IP rate limiting works
   - Verify global rate cap prevents total resource exhaustion

6. **Legitimate traffic test**:
   - Set up 10-node mesh
   - Verify all nodes discover each other
   - Verify no legitimate messages dropped
   - Verify rate limiting is transparent to normal operation

### Performance Tests

7. **Benchmarks**:
   - Measure rate limiter overhead per message
   - Target: <100ns per Allow() call
   - Verify no memory leaks in IP cache
   - Verify cleanup doesn't impact message processing

8. **Load testing**:
   - Simulate mesh with 100 nodes
   - Verify rate limiting scales appropriately
   - Measure memory usage of IP cache

### Observability

9. **Metrics to monitor**:
   - Rate limit violations per IP (log at DEBUG)
   - Handler pool saturation events (log at WARN)
   - Active goroutines (via pprof)
   - Memory usage of rate limiter
   - Message drop rate

10. **Logging verification**:
    - Verify dropped messages logged at appropriate level
    - Verify log flooding doesn't occur (rate limit the rate limiter logs!)

## Estimated Complexity

**Medium** (6-8 hours)

### Breakdown

1. **Rate limiter implementation** (2-3 hours):
   - Token bucket algorithm: 1 hour
   - Per-IP tracking with LRU: 1 hour
   - Thread-safety and cleanup: 30 minutes
   - Unit tests: 30-60 minutes

2. **Exchange integration** (1.5-2 hours):
   - Add rate limiter: 20 minutes
   - Implement handler pool: 30 minutes
   - Update listenLoop: 30 minutes
   - Testing: 30 minutes

3. **Gossip integration** (1-1.5 hours):
   - Add rate limiter: 20 minutes
   - Update listenLoop: 20 minutes
   - Testing: 30 minutes

4. **Configuration** (30-45 minutes):
   - Add config struct: 15 minutes
   - Set defaults: 15 minutes
   - Documentation: 15 minutes

5. **Testing and validation** (1.5-2 hours):
   - Integration tests: 45 minutes
   - DoS simulation: 30 minutes
   - Performance verification: 30 minutes
   - Documentation: 15 minutes

### Risk Factors

**Low-Medium Risk**:
- Well-defined problem with clear solution
- Token bucket is a well-established pattern
- Changes are localized to discovery package
- No protocol changes required
- Backward compatible

**Potential Issues**:
- Tuning rate limits for different network conditions
- False positives (legitimate peers being rate limited)
- Testing distributed attack scenarios requires multiple nodes
- Need to avoid logging-induced DoS (rate limit the logs)

### Mitigation Strategies

1. **Start with conservative (high) limits** and tune down based on telemetry
2. **Make limits configurable** via environment variables
3. **Add metrics/observability** to monitor rate limit effectiveness
4. **Document expected behavior** in normal vs. attack scenarios
5. **Test with real multi-node mesh** before declaring complete

### Dependencies

- No external dependencies required (use stdlib only)
- `sync` package for thread-safety
- `time` package for token refill
- `container/list` for LRU (if needed)

### Alternative Approaches Considered

1. **Connection-based limits**: TCP would provide per-connection limiting, but mesh uses UDP for performance
2. **Firewall rules**: External to application, harder to configure, less portable
3. **BPF filters**: Linux-specific, requires elevated privileges
4. **Global-only rate limit**: Doesn't protect against distributed attacks from many IPs
5. **Drop crypto entirely**: Would make protocol vulnerable to other attacks

**Chosen approach**: Per-IP token bucket + goroutine pool provides best balance of security, performance, and portability.
