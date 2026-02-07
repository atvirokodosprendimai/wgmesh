# Bootstrap Feature Implementation Plan

## Executive Summary

This document outlines the implementation plan for the token-based mesh autodiscovery feature described in `features/bootstrap.md`. The feature enables decentralized WireGuard mesh formation where nodes discover each other automatically using only a shared secret.

**Target UX:**
```bash
# Generate a mesh secret (once)
wgmesh init --secret
# outputs: wgmesh://v1/K7x2mP9qR4...

# On every node
wgmesh join --secret "wgmesh://v1/K7x2mP9qR4..."
# Mesh forms automatically in ~60 seconds
```

## Current Implementation Status

### ✅ Completed Components

1. **Core Daemon Infrastructure** (`pkg/daemon/`)
   - [x] `daemon.go` - Main daemon loop with reconciliation
   - [x] `config.go` - Configuration management
   - [x] `peerstore.go` - Thread-safe peer store
   - [x] `routes.go` - Route management
   - [x] `helpers.go` - Utility functions

2. **Key Derivation** (`pkg/crypto/`)
   - [x] `derive.go` - HKDF-based key derivation from secret
   - [x] `envelope.go` - Message encryption/decryption
   - [x] `encrypt.go` - AES-256-GCM encryption
   - [x] Support for: NetworkID, GossipKey, MeshSubnet, PSK, GossipPort

3. **DHT Discovery** (`pkg/discovery/`)
   - [x] `dht.go` - BitTorrent Mainline DHT integration
   - [x] `exchange.go` - Encrypted peer exchange protocol
   - [x] `init.go` - Discovery layer initialization
   - [x] DHT announce/query mechanism
   - [x] Peer exchange with encryption

4. **CLI Commands** (`main.go`)
   - [x] `join` - Join mesh using secret
   - [x] `init` - Generate new secret
   - [x] `status` - Show mesh status
   - [x] `test-peer` - Test peer exchange

5. **WireGuard Integration** (`pkg/wireguard/`)
   - [x] Key generation and management
   - [x] Configuration parsing and diffing
   - [x] Online updates via `wg set`
   - [x] Persistent configuration

### 🚧 Partially Implemented

1. **Privacy Features**
   - ⚠️ Basic encryption present, but no rotating infohash
   - ⚠️ No membership token validation
   - ⚠️ No Dandelion++ implementation

2. **Discovery Layers**
   - ✅ Layer 2 (DHT) - Implemented
   - ❌ Layer 0 (Registry) - **Not implemented**
   - ❌ Layer 1 (LAN) - **Not implemented**
   - ❌ Layer 3 (Gossip) - **Not implemented**

### ❌ Missing Components

1. **Registry Rendezvous** (`pkg/discovery/registry.go`)
   - GitHub Issue-based peer discovery
   - First-node bootstrap support
   - Encrypted peer list storage

2. **LAN Multicast Discovery** (`pkg/discovery/lan.go`)
   - UDP multicast announcements
   - Sub-second local discovery
   - Offline operation support

3. **In-Mesh Gossip** (`pkg/discovery/gossip.go`)
   - UDP gossip over WireGuard tunnels
   - Transitive peer discovery
   - Fast convergence

4. **Privacy Enhancements** (`pkg/privacy/`)
   - Rotating DHT infohash (hourly)
   - Membership token validation
   - Dandelion++ announcement relay
   - Epoch-based relay rotation

5. **Advanced Features**
   - QR code generation
   - Secret rotation protocol
   - Mesh IP collision resolution
   - Systemd service installation
   - Persistent peer cache

## Implementation Phases

### Phase 1: Foundation & Registry Bootstrap (Weeks 1-3)

**Goal:** Enable first-node bootstrap without DHT exposure. Complete the missing discovery layers.

#### 1.1 Registry Rendezvous Discovery

**Files to create:**
- `pkg/discovery/registry.go` - GitHub Issue-based discovery
- `pkg/discovery/registry_test.go` - Unit tests

**Key features:**
```go
type RendezvousRegistry struct {
    SearchTerm string        // Derived from secret
    GossipKey  []byte        
    IssueURL   string        // Cached after first find/create
    client     *http.Client
}

func (r *RendezvousRegistry) FindOrCreate(myInfo PeerInfo) ([]PeerInfo, error)
func (r *RendezvousRegistry) UpdatePeerList(peers []PeerInfo) error
func (r *RendezvousRegistry) decryptPeerList(body string) []PeerInfo
func (r *RendezvousRegistry) createIssue(myInfo PeerInfo, token string) error
```

**Implementation tasks:**
- [ ] Implement GitHub Issue search via API
- [ ] Implement encrypted peer list format
- [ ] Add fallback for nodes without GITHUB_TOKEN
- [ ] Add retry logic for API failures
- [ ] Integration with daemon bootstrap sequence

**Dependencies:**
- No new Go dependencies (uses standard `net/http`)
- Requires `GITHUB_TOKEN` environment variable (optional)

**Success criteria:**
- First node can create registry entry
- Second node can find and decrypt peer list
- Works behind corporate firewalls (HTTPS only)

#### 1.2 Enhanced Key Derivation

**Files to modify:**
- `pkg/crypto/derive.go` - Add missing derivations

**New derivations needed:**
```go
type DerivedKeys struct {
    // Existing
    NetworkID    [20]byte
    GossipKey    [32]byte
    MeshSubnet   [2]byte
    PSK          [32]byte
    
    // NEW - Add these
    RendezvousID [8]byte   // For GitHub Issue search term
    MembershipKey [32]byte // For token generation/validation
    EpochSeed    [32]byte  // For relay peer rotation
}

// NEW - Add rotating infohash support
func CurrentNetworkIDs(secret string) (current, previous [20]byte)
```

**Implementation tasks:**
- [ ] Add `RendezvousID` derivation (SHA256(secret || "rv")[0:8])
- [ ] Add `MembershipKey` derivation (HKDF)
- [ ] Add `EpochSeed` derivation (HKDF)
- [ ] Implement time-based rotating network IDs
- [ ] Update existing code to use rotating IDs

**Testing:**
- [ ] Verify deterministic derivation (same secret → same keys)
- [ ] Test infohash rotation at hour boundaries
- [ ] Verify backward compatibility

#### 1.3 Membership Token Authentication

**Files to create:**
- `pkg/crypto/membership.go` - Token generation/validation
- `pkg/crypto/membership_test.go` - Unit tests

**Implementation:**
```go
func GenerateMembershipToken(membershipKey []byte, myPubkey []byte) []byte
func ValidateMembershipToken(membershipKey []byte, theirPubkey, token []byte) bool
```

**Integration points:**
- [ ] Add token to peer exchange HELLO message
- [ ] Validate token in exchange.go HandlePeerExchange()
- [ ] Silent reject for invalid tokens (no oracle)

**Success criteria:**
- Nodes with secret can exchange peers
- Nodes without secret get silent rejection
- Clock skew tolerance (±1 hour)

#### 1.4 Bootstrap Discovery Chain

**Files to modify:**
- `pkg/daemon/daemon.go` - Add registry to bootstrap sequence

**Implementation:**
```go
func (d *Daemon) Bootstrap() []PeerInfo {
    var peers []PeerInfo
    
    // Layer 0: Registry (NEW)
    if registry := d.NewRegistryDiscovery(); registry != nil {
        if rp, err := registry.FindOrCreate(d.localNode.ToPeerInfo()); err == nil {
            peers = append(peers, rp...)
        }
    }
    
    // Layer 1: LAN (TODO: Phase 1.5)
    
    // Layer 2: DHT (existing)
    if len(peers) == 0 && d.dhtDiscovery != nil {
        peers = append(peers, d.dhtDiscovery.GetPeers()...)
    }
    
    return deduplicate(peers)
}
```

**Tasks:**
- [ ] Implement fallback chain logic
- [ ] Add retry with exponential backoff
- [ ] Implement peer deduplication
- [ ] Add metrics/logging for discovery sources

**Milestone 1.4:** First node creates registry, second node discovers it via registry before DHT.

### Phase 1.5: LAN Multicast Discovery (Week 4)

**Goal:** Enable sub-second discovery on local networks.

**Files to create:**
- `pkg/discovery/lan.go` - UDP multicast implementation
- `pkg/discovery/lan_test.go` - Unit tests

**Key features:**
```go
type LANDiscovery struct {
    config       *daemon.Config
    gossipKey    []byte
    multicastAddr *net.UDPAddr
    conn         *net.UDPConn
    peerStore    *daemon.PeerStore
}

func (l *LANDiscovery) Start() error
func (l *LANDiscovery) Stop() error
func (l *LANDiscovery) Announce() error
func (l *LANDiscovery) Listen() error
```

**Implementation tasks:**
- [ ] Multicast group: `239.192.0.0/16` + derived discriminator
- [ ] Encrypted announcement format
- [ ] 5-second announcement interval
- [ ] IPv4 and IPv6 support (link-local ff02::1)
- [ ] Integration with daemon bootstrap

**Dependencies:**
- `golang.org/x/net/ipv4` - Already used elsewhere
- `golang.org/x/net/ipv6` - For IPv6 multicast

**Success criteria:**
- Two nodes on same LAN discover each other in <5 seconds
- Works offline (no internet required)
- Only nodes with correct secret can decrypt

**Testing:**
- [ ] Unit tests with mock UDP connections
- [ ] Integration test with two local nodes
- [ ] Test IPv4 and IPv6 multicast
- [ ] Test encrypted announcement parsing

### Phase 2: Privacy Enhancements (Weeks 5-7)

**Goal:** Implement privacy features from bootstrap.md Section "Privacy-Enhanced Architecture"

#### 2.1 Rotating DHT Infohash

**Files to modify:**
- `pkg/discovery/dht.go` - Update to query both current and previous hour

**Implementation:**
```go
func (d *DHTDiscovery) getCurrentInfohashes() (current, previous [20]byte) {
    return crypto.CurrentNetworkIDs(d.config.Secret)
}

func (d *DHTDiscovery) announceLoop() {
    // Announce only to current infohash
    current, _ := d.getCurrentInfohashes()
    d.server.Announce(current, ...)
}

func (d *DHTDiscovery) queryLoop() {
    // Query both current and previous (hour boundary tolerance)
    current, previous := d.getCurrentInfohashes()
    peers1 := d.server.GetPeers(current)
    peers2 := d.server.GetPeers(previous)
    return append(peers1, peers2...)
}
```

**Tasks:**
- [ ] Modify DHT announce to use current hour infohash
- [ ] Modify DHT query to check both hours
- [ ] Add logging for infohash rotation
- [ ] Test hour boundary transitions

**Success criteria:**
- Nodes announce to current hour infohash
- Nodes can discover peers in previous hour (transition period)
- Observer cannot correlate across hour boundaries

#### 2.2 Epoch-Based Relay Selection

**Files to create:**
- `pkg/daemon/epoch.go` - Epoch management
- `pkg/daemon/epoch_test.go` - Unit tests

**Implementation:**
```go
type Epoch struct {
    ID         uint64
    RelayPeers []daemon.PeerInfo
    StartedAt  time.Time
    Duration   time.Duration // 10 minutes default
}

func (d *Daemon) RotateEpoch() {
    // Select 2 relay peers using epoch_seed + epoch_id
    allPeers := d.peerStore.GetAll()
    seed := hmac(d.keys.EpochSeed, epochID)
    d.epoch.RelayPeers = deterministicShuffle(allPeers, seed)[:2]
    d.epoch.ID++
}
```

**Tasks:**
- [ ] Implement epoch rotation timer (10 minutes)
- [ ] Deterministic peer selection using HMAC
- [ ] Handle meshes with <2 peers gracefully
- [ ] Store current epoch in daemon state

#### 2.3 Dandelion++ Announcement Relay

**Files to create:**
- `pkg/privacy/dandelion.go` - Stem/fluff implementation
- `pkg/privacy/dandelion_test.go` - Unit tests

**Key types:**
```go
type DandelionAnnounce struct {
    OriginPubkey     []byte
    OriginMeshIP     string
    OriginEndpoint   string
    RoutableNetworks []string
    HopCount         uint8
    Timestamp        int64
    Nonce            []byte
}

func (d *Daemon) HandleDandelionAnnounce(msg DandelionAnnounce)
func (d *Daemon) ShouldFluff(hopCount uint8) bool // 10% probability
func (d *Daemon) RelayToStem(msg DandelionAnnounce)
func (d *Daemon) FluffToDHT(msg DandelionAnnounce)
```

**Implementation tasks:**
- [ ] Implement stem phase (relay through mesh)
- [ ] Implement fluff phase (announce to DHT or broadcast)
- [ ] 10% fluff probability per hop
- [ ] Max 4 hops before forced fluff
- [ ] Integration with DHT announce

**Success criteria:**
- New node announcement goes through relay
- Observer sees relay IP, not origin IP
- First node bootstraps directly (no relays available)

**Testing:**
- [ ] Unit test stem probability
- [ ] Integration test with 3+ nodes
- [ ] Verify origin IP not visible in DHT

### Phase 3: In-Mesh Gossip & Convergence (Weeks 8-9)

**Goal:** Fast mesh convergence via in-mesh peer exchange.

**Files to create:**
- `pkg/discovery/gossip.go` - In-mesh gossip protocol
- `pkg/discovery/gossip_test.go` - Unit tests

**Key features:**
```go
type MeshGossip struct {
    config    *daemon.Config
    localNode *daemon.LocalNode
    peerStore *daemon.PeerStore
    gossipKey []byte
    port      uint16 // Derived from secret
}

func (g *MeshGossip) Start() error
func (g *MeshGossip) Stop() error
func (g *MeshGossip) GossipLoop()
func (g *MeshGossip) ExchangeWithRandomPeer()
```

**Implementation tasks:**
- [ ] UDP socket on derived gossip port
- [ ] Random peer selection every 10 seconds
- [ ] Full peer list exchange
- [ ] Encryption with gossip_key
- [ ] Integration with peerstore
- [ ] Transitive discovery support

**Success criteria:**
- Node C discovers Node B through Node A
- Full mesh convergence in <30 seconds after any connection
- Routable networks propagate quickly

**Testing:**
- [ ] Unit tests for gossip messages
- [ ] Integration test: 3 nodes, verify transitive discovery
- [ ] Test routable_networks propagation

### Phase 4: Advanced Features (Weeks 10-12)

#### 4.1 QR Code Generation

**Files to modify:**
- `main.go` - Add `qr` subcommand

**Implementation:**
```bash
wgmesh qr --secret "wgmesh://v1/..."
# Prints QR code to terminal (UTF-8 box drawing)
```

**Dependencies:**
- `github.com/skip2/go-qrcode` - QR code generation

**Tasks:**
- [ ] Implement `qrCmd()` in main.go
- [ ] UTF-8 terminal output
- [ ] PNG file output option
- [ ] Integration with init command

#### 4.2 Mesh IP Collision Detection & Resolution

**Files to create:**
- `pkg/daemon/collision.go` - Collision detection/resolution
- `pkg/daemon/collision_test.go` - Unit tests

**Implementation:**
```go
func (p *PeerStore) DetectCollision() ([]PeerInfo, bool)
func (d *Daemon) ResolveCollision(peers []PeerInfo)
func DeterministicWinner(peer1, peer2 PeerInfo) PeerInfo
```

**Tasks:**
- [ ] Detect: two different pubkeys, same mesh IP
- [ ] Resolution: lexicographically lower pubkey wins
- [ ] Loser re-derives mesh IP with nonce++
- [ ] Gossip collision resolution decisions

#### 4.3 Systemd Service Installation

**Files to create:**
- `pkg/daemon/systemd.go` - Service generation
- `install-service` subcommand in main.go

**Implementation:**
```bash
wgmesh install-service --secret "..." [options]
# Creates /etc/systemd/system/wgmesh.service
# Enables and starts service
```

**Tasks:**
- [ ] Generate systemd unit file
- [ ] Handle service options (routes, port, etc)
- [ ] Enable and start service
- [ ] Status checking command
- [ ] Uninstall command

#### 4.4 Persistent Peer Cache

**Files to create:**
- `pkg/daemon/cache.go` - Peer cache persistence

**Implementation:**
```go
func (d *Daemon) LoadPeerCache() error
func (d *Daemon) SavePeerCache() error
```

**Tasks:**
- [ ] Store known peers in JSON file
- [ ] Load on daemon startup
- [ ] Periodic save (every 5 minutes)
- [ ] Merge with discovered peers
- [ ] Cache expiration (24 hours)

**Benefits:**
- Faster startup (no re-discovery needed)
- Works offline if peers haven't changed
- Reduces DHT traffic

#### 4.5 Secret Rotation Protocol

**Files to create:**
- `pkg/crypto/rotation.go` - Secret rotation logic
- `rotate-secret` subcommand in main.go

**Implementation:**
```bash
wgmesh rotate-secret --current "old" --new "new" --grace 24h
```

**Tasks:**
- [ ] Generate rotation announcement
- [ ] Broadcast via gossip
- [ ] Maintain dual-secret mode during grace period
- [ ] Coordinated switchover
- [ ] Update persistent config

## Testing Strategy

### Unit Tests

**Coverage target:** 80% for all new code

**Key test files:**
- `pkg/discovery/registry_test.go` - Registry discovery
- `pkg/discovery/lan_test.go` - LAN multicast
- `pkg/discovery/gossip_test.go` - In-mesh gossip
- `pkg/crypto/membership_test.go` - Token validation
- `pkg/privacy/dandelion_test.go` - Stem/fluff logic
- `pkg/daemon/epoch_test.go` - Epoch rotation
- `pkg/daemon/collision_test.go` - IP collision resolution

**Testing approach:**
- Mock network connections where possible
- Use table-driven tests for crypto functions
- Test edge cases (empty meshes, single node, etc)

### Integration Tests

**Test scenarios:**

1. **Two-Node Bootstrap**
   ```bash
   # Node 1
   wgmesh join --secret "test" --interface wg-test1
   
   # Node 2 (different network)
   wgmesh join --secret "test" --interface wg-test2
   
   # Verify: mesh forms in <60s
   ```

2. **LAN Discovery**
   ```bash
   # Both nodes on same network
   # Verify: discovery in <5s
   ```

3. **Three-Node Transitive Discovery**
   ```bash
   # Node A knows B, B knows C
   # Verify: A discovers C via gossip
   ```

4. **Privacy Mode**
   ```bash
   wgmesh join --secret "test" --privacy
   # Verify: No direct DHT announcement
   ```

5. **Collision Resolution**
   ```bash
   # Force two nodes to same mesh IP
   # Verify: One re-derives, mesh stabilizes
   ```

### Manual Testing Checklist

- [ ] First-node bootstrap (creates registry entry)
- [ ] Second-node discovery via registry
- [ ] LAN discovery (same subnet)
- [ ] DHT discovery (different networks)
- [ ] Mesh convergence (3+ nodes)
- [ ] Rotating infohash at hour boundary
- [ ] Privacy mode (Dandelion++)
- [ ] Service installation/restart
- [ ] QR code generation
- [ ] Secret rotation

## Dependencies & Prerequisites

### New Go Dependencies

```go
// Already in use
github.com/anacrolix/dht/v2        // DHT implementation
golang.org/x/crypto/hkdf           // Key derivation
golang.org/x/net/ipv4              // Multicast

// New for Phase 1
// (none - uses stdlib)

// New for Phase 4
github.com/skip2/go-qrcode         // QR code generation
```

**Dependency update required:**
- Update go.mod if adding go-qrcode

### System Prerequisites

**Development:**
- Go 1.23+
- WireGuard tools (`wg` command)
- Root/sudo access for testing

**Runtime:**
- Linux kernel with WireGuard support
- UDP port access (for DHT and gossip)
- Optional: GITHUB_TOKEN for registry creation

### Build & Test Commands

```bash
# Build
go build -o wgmesh

# Run tests
go test ./...

# Run with coverage
go test -cover ./...

# Integration tests (requires root)
sudo go test -tags=integration ./...

# Lint
golangci-lint run
```

## Milestones & Deliverables

### Milestone 1: Registry Bootstrap (End of Week 3)
**Deliverables:**
- [x] Registry discovery implementation
- [x] Enhanced key derivation
- [x] Membership token authentication
- [x] Bootstrap chain integration
- [x] Unit tests (80% coverage)
- [x] Documentation updates

**Success criteria:**
- First node creates registry entry
- Second node discovers via registry
- Works behind firewalls

### Milestone 2: Complete Discovery (End of Week 4)
**Deliverables:**
- [x] LAN multicast discovery
- [x] All discovery layers operational
- [x] Integration tests for each layer
- [x] Performance benchmarks

**Success criteria:**
- LAN discovery in <5 seconds
- DHT discovery in <60 seconds
- Offline LAN mesh works

### Milestone 3: Privacy Features (End of Week 7)
**Deliverables:**
- [x] Rotating DHT infohash
- [x] Epoch-based relay selection
- [x] Dandelion++ implementation
- [x] Privacy mode CLI flag
- [x] Security documentation

**Success criteria:**
- Origin IP not visible in DHT
- Topology inference limited
- Membership tokens enforced

### Milestone 4: Gossip & Convergence (End of Week 9)
**Deliverables:**
- [x] In-mesh gossip protocol
- [x] Transitive peer discovery
- [x] Fast convergence tests
- [x] Performance optimization

**Success criteria:**
- 10-node mesh converges in <90 seconds
- Routable networks propagate quickly
- Dead peer cleanup works

### Milestone 5: Production Ready (End of Week 12)
**Deliverables:**
- [x] QR code generation
- [x] Collision resolution
- [x] Systemd service support
- [x] Persistent peer cache
- [x] Secret rotation
- [x] Complete documentation
- [x] Production deployment guide

**Success criteria:**
- Feature-complete per bootstrap.md
- 80%+ test coverage
- Production deployment tested
- Security audit complete

## Open Questions & Design Decisions

### 1. Registry Implementation Choice

**Question:** Use GitHub Issues or implement custom registry server?

**Options:**
- **A) GitHub Issues API** (current plan)
  - ✅ Zero infrastructure
  - ✅ Works behind firewalls
  - ✅ Free, anonymous search
  - ❌ Rate limits (60 req/hour unauthenticated)
  - ❌ Requires GITHUB_TOKEN to create issues

- **B) Custom registry server**
  - ✅ No rate limits
  - ✅ Full control
  - ❌ Infrastructure cost
  - ❌ Single point of failure
  - ❌ Against decentralization goal

- **C) GitLab/Gitea alternatives**
  - ✅ More permissive rate limits
  - ❌ Less ubiquitous than GitHub

**Decision:** Proceed with GitHub Issues (Option A). Rate limits are acceptable for typical usage. Users requiring high-frequency updates can:
1. Provide GITHUB_TOKEN
2. Use LAN/DHT discovery primarily
3. Self-host registry alternative

### 2. Default Privacy Level

**Question:** Should `--privacy` be default or opt-in?

**Arguments for default:**
- Better security posture
- No reason not to enable
- Protects users who don't understand risks

**Arguments for opt-in:**
- Simpler debugging (direct DHT visible)
- Faster initial discovery (no relay hops)
- Less complexity by default

**Decision:** **Opt-in for Phase 2**, **default for Phase 5**. Rationale:
1. Phase 2: Opt-in allows testing and debugging
2. After stability proven, make default in Phase 5
3. Add `--no-privacy` flag for debugging

### 3. DHT Port Sharing

**Question:** Share WireGuard's listen port or use separate port?

**Options:**
- **A) Separate port** (current implementation)
  - ✅ Simpler implementation
  - ✅ No protocol multiplexing needed
  - ❌ One more port to open
  - ❌ Two ports in documentation

- **B) Shared port**
  - ✅ One port to configure
  - ❌ Complex multiplexing
  - ❌ Risk of breaking WireGuard

**Decision:** Separate port (Option A). Simplicity and safety trump convenience. DHT port is derived from secret, so no manual config needed.

### 4. Maximum Mesh Size

**Question:** What's the target maximum mesh size?

**Current design:** /16 subnet = 65,534 addresses
- Collision probability at 100 nodes: ~0.08%
- Collision probability at 1000 nodes: ~7.5%

**Options:**
- **A) Support up to 100 nodes** (current plan)
  - ✅ Collision rare
  - ✅ Simpler implementation
  - ❌ Limits large deployments

- **B) Support 1000+ nodes**
  - ✅ More flexible
  - ❌ Requires IP allocation protocol
  - ❌ Gossip scaling issues

**Decision:** Target 100 nodes in Phase 1-4. For Phase 5+, if demand exists:
- Implement DHCP-like IP allocation
- Consensus-based assignment via gossip
- Bump to /8 or support multiple /16 subnets

### 5. STUN Integration

**Question:** Add STUN for NAT endpoint detection?

**Current:** Nodes detect public endpoint via SSH host or DHT peer IP

**With STUN:**
- ✅ More accurate endpoint detection
- ✅ Works without SSH
- ✅ Standard protocol
- ❌ Additional dependency
- ❌ Requires public STUN server

**Decision:** Add STUN in Phase 4 as optional enhancement:
```bash
wgmesh join --secret "..." --stun stun.l.google.com:19302
```

Use `pion/stun` library. Fallback to current method if STUN unavailable.

### 6. Garlic Bundling

**Question:** Implement garlic routing (I2P-style message bundling)?

**From bootstrap.md:**
> "Honest assessment: This is probably overkill unless threat model includes nation-state adversary"

**Decision:** **NOT implementing in initial release**. Rationale:
- WireGuard already encrypts all traffic
- Adds latency (100-500ms batching)
- Complex implementation
- Marginal benefit for VPN use case
- Can add later if demand exists

Document as "Future Enhancement" rather than planned feature.

### 7. Floodfill Mode

**Question:** Implement private floodfill (I2P-style)?

**From bootstrap.md:**
> "Hybrid mode: Can run both public DHT and private floodfill"

**Decision:** **NOT implementing in initial release**. Rationale:
- Registry + DHT provides sufficient discovery
- Adds significant complexity
- Requires node election logic
- Sybil attack concerns
- Can add in Phase 5+ if needed

Document as "Future Enhancement".

### 8. Backward Compatibility

**Question:** Should daemon mode read/write `mesh-state.json`?

**Use case:** Mixed deployment where some nodes use centralized SSH management, others use autodiscovery.

**Decision:** **No** for initial release. Rationale:
- Different operational models
- Risk of state corruption
- Cleaner separation of concerns

Future enhancement: `--import mesh-state.json` to seed peer cache.

## Risk Assessment & Mitigation

### Technical Risks

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|------------|
| DHT unreliable in production | Medium | High | Layer 0 (registry) and Layer 1 (LAN) provide fallback |
| GitHub API rate limits | Medium | Medium | Cache peers, use LAN/DHT primarily |
| Mesh IP collisions frequent | Low | High | Implement collision detection and resolution |
| Gossip doesn't scale to 100 nodes | Low | Medium | Implement exponential backoff, selective gossip |
| Privacy features degrade performance | Medium | Low | Make opt-in, optimize critical paths |
| NAT traversal fails | High | High | Document STUN configuration, consider TURN relay |

### Security Risks

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|------------|
| Secret leaked | Medium | Critical | Implement secret rotation, document key management |
| Sybil attack on mesh | Low | High | Membership tokens, rate limiting, future: PoW |
| Traffic analysis despite encryption | Low | Medium | Implement Dandelion++, document limitations |
| DHT poisoning attack | Low | Medium | Membership token validation |
| Registry spam/DoS | Low | Low | GitHub's abuse detection |

### Operational Risks

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|------------|
| Complex debugging | High | Medium | Add verbose logging, diagnostic commands |
| User misconfiguration | High | Medium | Sensible defaults, validation, good error messages |
| Service crashes | Medium | Medium | Systemd restart policies, graceful recovery |
| State corruption | Low | High | Atomic writes, backups, state validation |

## Documentation Plan

### User Documentation

1. **README.md updates**
   - Add decentralized mode section
   - Update quick start guide
   - Add privacy features explanation

2. **Tutorial: Your First Mesh**
   - Step-by-step with screenshots
   - Troubleshooting common issues

3. **Privacy Guide**
   - Threat model explanation
   - When to use privacy features
   - Limitations and non-goals

4. **Deployment Guide**
   - Production best practices
   - Systemd service setup
   - Monitoring and health checks

### Developer Documentation

1. **ARCHITECTURE.md**
   - System overview
   - Component interactions
   - State management

2. **CONTRIBUTING.md**
   - Development setup
   - Testing guidelines
   - Code review process

3. **API.md**
   - Protocol specifications
   - Wire formats
   - Versioning strategy

### Implementation Notes

Each phase should update:
- Code comments (godoc)
- Unit test documentation
- Integration test scenarios
- CHANGELOG.md

## Success Metrics

### Functional Metrics

- [ ] First node bootstrap time: <10 seconds
- [ ] Second node discovery time: <60 seconds
- [ ] LAN discovery time: <5 seconds
- [ ] 10-node mesh convergence: <90 seconds
- [ ] Mesh stability: >99% uptime over 24 hours

### Quality Metrics

- [ ] Test coverage: >80%
- [ ] Zero critical security issues (CodeQL)
- [ ] <10 known bugs at release
- [ ] Documentation complete for all features

### Performance Metrics

- [ ] CPU usage idle: <1%
- [ ] Memory usage: <50MB per node
- [ ] DHT traffic: <100KB/hour per node
- [ ] Gossip traffic: <10KB/minute per node

## Timeline Summary

```
Week 1-3:   Phase 1 - Registry, Key Derivation, Membership Tokens
Week 4:     Phase 1.5 - LAN Multicast Discovery
Week 5-7:   Phase 2 - Privacy Enhancements
Week 8-9:   Phase 3 - In-Mesh Gossip
Week 10-12: Phase 4 - Advanced Features
Week 13:    Testing, Documentation, Release Prep
```

**Total estimated time:** 13 weeks (3.25 months)

**Team requirements:**
- 1 Senior Go developer (full-time)
- 1 Security reviewer (part-time, Weeks 5-7)
- 1 Technical writer (part-time, Week 12-13)

## Next Steps

1. **Review and approve** this implementation plan
2. **Set up project tracking** (GitHub Projects or Jira)
3. **Create Phase 1 work items** with detailed tasks
4. **Begin implementation** of registry discovery
5. **Schedule weekly progress reviews**

## Appendix: File Structure

```
wgmesh/
├── main.go                           # CLI entry point
├── features/
│   ├── bootstrap.md                  # Feature specification
│   └── IMPLEMENTATION_PLAN.md        # This document
├── pkg/
│   ├── crypto/
│   │   ├── derive.go                # ✅ Key derivation (needs enhancement)
│   │   ├── envelope.go              # ✅ Message encryption
│   │   ├── membership.go            # ❌ NEW - Token validation
│   │   └── rotation.go              # ❌ NEW - Secret rotation
│   ├── daemon/
│   │   ├── daemon.go                # ✅ Main daemon loop
│   │   ├── config.go                # ✅ Configuration
│   │   ├── peerstore.go             # ✅ Peer management
│   │   ├── epoch.go                 # ❌ NEW - Epoch rotation
│   │   ├── collision.go             # ❌ NEW - IP collision
│   │   ├── cache.go                 # ❌ NEW - Peer cache
│   │   └── systemd.go               # ❌ NEW - Service management
│   ├── discovery/
│   │   ├── dht.go                   # ✅ DHT discovery
│   │   ├── exchange.go              # ✅ Peer exchange
│   │   ├── registry.go              # ❌ NEW - Registry discovery
│   │   ├── lan.go                   # ❌ NEW - LAN multicast
│   │   └── gossip.go                # ❌ NEW - In-mesh gossip
│   ├── privacy/
│   │   └── dandelion.go             # ❌ NEW - Dandelion++
│   ├── mesh/                        # ✅ Existing centralized mode
│   ├── ssh/                         # ✅ Existing SSH client
│   └── wireguard/                   # ✅ Existing WG management
└── test/
    ├── integration/                 # ❌ NEW - Integration tests
    └── fixtures/                    # ❌ NEW - Test data

Legend:
✅ Implemented
❌ Not implemented (planned)
⚠️  Partially implemented
```

## References

- [bootstrap.md](./bootstrap.md) - Complete feature specification
- [DevSwarm](https://github.com/HackrsValv/devswarm) - Registry inspiration
- [Dandelion++ BIP-156](https://github.com/bitcoin/bips/blob/master/bip-0156.mediawiki) - Privacy reference
- [anacrolix/dht](https://github.com/anacrolix/dht) - DHT library
- [WireGuard Protocol](https://www.wireguard.com/protocol/) - WireGuard spec

---

**Document Version:** 1.0  
**Last Updated:** 2026-02-07  
**Status:** Draft for Review  
**Author:** GitHub Copilot Implementation Planning Agent
