# wgmeshbuilder: Token-Based Mesh Autodiscovery

## Problem Statement

Currently wgmeshbuilder requires a centralized operator who manually `--add` nodes, maintains `mesh-state.json`, and runs `--deploy` over SSH. The goal is to enable a fully decentralized mode where any node with the same shared secret automatically discovers peers and forms a WireGuard mesh within ~60 seconds, with zero pre-configuration beyond the secret itself.

**Target UX:**

```bash
# Generate a mesh secret (once, by anyone)
wgmesh init --secret
# outputs: wgmesh://K7x2mP9qR4... (also renderable as QR code)

# On every node that should join:
wgmesh join --secret "K7x2mP9qR4..."
# That's it. Mesh forms automatically.
```

---

## Architecture Overview

The system uses a layered discovery approach with optional privacy enhancements. Each layer operates independently and results are merged, meaning the mesh works on a LAN with no internet, on the internet with no LAN, or both.

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Shared Secret                                │
│               (only input, encodes everything)                       │
└───────┬─────────────┬──────────────┬───────────────┬────────────────┘
        │             │              │               │
  ┌─────▼─────┐ ┌─────▼─────┐ ┌─────▼──────┐ ┌──────▼───────┐
  │ Layer 0:  │ │ Layer 1:  │ │ Layer 2:   │ │ Layer 3:     │
  │ Registry  │ │ LAN       │ │ BitTorrent │ │ In-mesh      │
  │ Rendezvous│ │ Multicast │ │ DHT        │ │ Gossip       │
  │ (GitHub)  │ │ (local)   │ │ (global)   │ │ (post-conn)  │
  └─────┬─────┘ └─────┬─────┘ └─────┬──────┘ └──────┬───────┘
        │             │              │               │
        │             │      ┌───────▼───────┐       │
        │             │      │ PRIVACY LAYER │       │
        │             │      │ (optional)    │       │
        │             │      │               │       │
        │             │      │ • Dandelion++ │       │
        │             │      │ • Rotating ID │       │
        │             │      │ • Membership  │       │
        │             │      │   Tokens      │       │
        │             │      │ • Epoch Relay │       │
        │             │      └───────┬───────┘       │
        │             │              │               │
  ┌─────▼─────────────▼──────────────▼───────────────▼────────┐
  │              Peer Merge & Diff Engine                      │
  │        (existing wg set / route diff logic)                │
  └────────────────────────────┬──────────────────────────────┘
                               │
                        ┌──────▼──────┐
                        │  WireGuard  │
                        │  wg0 iface  │
                        └─────────────┘
```

**Discovery Priority:**

```
1. Registry (Layer 0)  — First-node friendly, works behind firewalls
2. LAN (Layer 1)       — Instant if peers on same network  
3. DHT (Layer 2)       — Public internet fallback
4. Gossip (Layer 3)    — Fast convergence once any connection made
```

**Bootstrap scenarios:**

| Scenario | Discovery Path |
|----------|----------------|
| First node ever | Registry (create) → wait for others |
| Second node, same LAN | LAN finds first → done |
| Second node, different network | Registry (read) → connect to first |
| Node behind corporate firewall | Registry only (DHT blocked) |
| Offline LAN mesh | LAN only (no internet needed) |
| Privacy-sensitive | Registry + Dandelion (no direct DHT announce) |

### Privacy Mode Architecture

With `--privacy` flag enabled, the data flow changes:

```
┌─────────────────────────────────────────────────────────────────────┐
│  NEW NODE JOINS (with --privacy)                                     │
└─────────────────────────────────────────────────────────────────────┘

 ┌──────────┐                                              ┌──────────┐
 │ New Node │                                              │   DHT    │
 │  (you)   │                                              │ (public) │
 └────┬─────┘                                              └────▲─────┘
      │                                                         │
      │ 1. Derive rotating network_id (hourly)                  │
      │                                                         │
      │ 2. Query DHT (current + prev hour infohash)             │
      │────────────────────────────────────────────────────────▶│
      │                                                         │
      │◀────────────────────────────────────────────────────────│
      │    IP:port list returned                                │
      │                                                         │
      │ 3. Connect to peer, send membership token               │
      │───────────────────────────▶┌───────────┐                │
      │                            │ Mesh Peer │                │
      │◀───────────────────────────│     A     │                │
      │    Encrypted peer list     └─────┬─────┘                │
      │    (if token valid)              │                      │
      │                                  │                      │
      │ 4. Dandelion STEM: send announce │                      │
      │─────────────────────────────────▶│                      │
      │                                  │                      │
      │                            ┌─────▼─────┐                │
      │                            │ Mesh Peer │                │
      │                            │     B     │                │
      │                            └─────┬─────┘                │
      │                                  │ 90% continue stem    │
      │                                  │ 10% FLUFF            │
      │                            ┌─────▼─────┐                │
      │                            │ Mesh Peer │                │
      │                            │     C     │                │
      │                            └─────┬─────┘                │
      │                                  │                      │
      │                                  │ FLUFF: announce to   │
      │                                  │ DHT using C's IP     │
      │                                  └─────────────────────▶│
      │                                                         │
      │    DHT observer sees: "C announced"                     │
      │    Not: "New Node announced"                            │
      │                                                         │

┌─────────────────────────────────────────────────────────────────────┐
│  What external observer learns:                                      │
│                                                                      │
│  WITHOUT privacy:                                                    │
│    • Your IP directly visible in DHT                                 │
│    • Timestamp of when you joined                                    │
│    • Can enumerate all mesh members over time                        │
│                                                                      │
│  WITH privacy (--privacy):                                           │
│    • Some IP announced (relay's, not yours)                          │
│    • Infohash changes hourly (can't correlate across hours)          │
│    • Unauthenticated queries get no response                         │
│    • Topology inference limited to 10-minute epochs                  │
│                                                                      │
│  WITH maximum privacy (--privacy --no-public-dht --garlic):          │
│    • Zero DHT exposure (floodfill-only mode)                         │
│    • Traffic patterns obscured by garlic bundling                    │
│    • Fixed-size padded messages prevent size correlation             │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Key Derivation from Shared Secret

Everything is derived from the single secret. No other configuration needed.

```
secret (arbitrary string, min 16 chars)
  │
  ├─ network_id    = SHA256(secret)[0:20]          → DHT infohash (20 bytes)
  ├─ rendezvous_id = SHA256(secret || "rv")[0:8]   → Public registry search term (8 bytes hex)
  ├─ gossip_key    = HKDF(secret, salt="wgmesh-gossip-v1", 32 bytes)  → symmetric encryption for peer exchange
  ├─ mesh_subnet   = HKDF(secret, salt="wgmesh-subnet-v1", 2 bytes)   → deterministic /16 (10.x.y.0/16)
  ├─ multicast_id  = HKDF(secret, salt="wgmesh-mcast-v1", 4 bytes)    → multicast group discriminator
  └─ psk           = HKDF(secret, salt="wgmesh-wg-psk-v1", 32 bytes)  → WireGuard PresharedKey for all peer pairs
```

**Mesh IP allocation:** Each node's mesh IP is derived deterministically from its WG public key:

```
mesh_ip = mesh_subnet_base + uint16(SHA256(wg_pubkey || secret)[0:2])
```

Collision probability is low for small meshes (<100 nodes in a /16). On collision detection (duplicate mesh IP with different pubkey), the node with the lexicographically lower pubkey wins, the other re-derives with a nonce.

---

## Layer 0: Public Registry Rendezvous (Bootstrap Magic)

**Purpose:** Solve the first-node bootstrap problem without exposing IPs to public DHT. Uses GitHub/GitLab issue search as a zero-infrastructure encrypted meeting point.

**The insight (from DevSwarm):** Git forges provide free, anonymous, searchable storage. The secret can derive a search term that acts as a rendezvous point.

**Mechanism:**

```
1. Derive search term:  rendezvous_id = hex(SHA256(secret || "rv")[0:8])
                        search_term = "wgmesh-" + rendezvous_id
                        Example: "wgmesh-a1b2c3d4"

2. First node joins:
   - Search GitHub Issues in wgmesh-registry/public for search_term
   - Not found → Create issue with encrypted peer info as body
   - Title: "wgmesh-a1b2c3d4" (searchable)
   - Body: Encrypt(gossip_key, {pubkey, endpoint, mesh_ip, timestamp})

3. Subsequent nodes:
   - Search finds the issue
   - Decrypt body → get bootstrap peer(s)
   - Append own info to issue body (edit via API)
   - Connect to discovered peers
   - Once connected, use in-mesh gossip for further discovery

4. Steady state:
   - Issue body contains encrypted list of all peers (or recent subset)
   - Nodes update their entry periodically
   - Old entries (>24h without update) are pruned
```

**Why this is maximum magic:**
- First node is discoverable without public DHT announcement
- Works behind corporate firewalls (just HTTPS to GitHub)
- No repo creation needed (use shared public registry)
- Search is anonymous (no auth required)
- Secret alone derives everything
- Encrypted — observer sees search term but can't decrypt peer list

**Implementation:**

```go
const (
    RegistryRepo = "wgmesh-registry/public"  // Shared public repo
    RegistryAPI  = "https://api.github.com"
)

type RendezvousRegistry struct {
    SearchTerm string
    GossipKey  []byte
    IssueURL   string  // Cached after first find/create
}

func NewRendezvous(secret string) *RendezvousRegistry {
    hash := sha256.Sum256([]byte(secret + "rv"))
    return &RendezvousRegistry{
        SearchTerm: fmt.Sprintf("wgmesh-%x", hash[:8]),
        GossipKey:  hkdf(secret, "wgmesh-gossip-v1", 32),
    }
}

func (r *RendezvousRegistry) FindOrCreate(myInfo PeerInfo) ([]PeerInfo, error) {
    // Search for existing meeting point (no auth required)
    searchURL := fmt.Sprintf("%s/search/issues?q=%s+repo:%s", 
        RegistryAPI, r.SearchTerm, RegistryRepo)
    
    resp, _ := http.Get(searchURL)
    var result struct {
        Items []struct {
            Number int    `json:"number"`
            Title  string `json:"title"`
            Body   string `json:"body"`
        } `json:"items"`
    }
    json.NewDecoder(resp.Body).Decode(&result)
    
    if len(result.Items) > 0 {
        // Found existing meeting point
        r.IssueURL = fmt.Sprintf("%s/repos/%s/issues/%d", 
            RegistryAPI, RegistryRepo, result.Items[0].Number)
        
        // Decrypt peer list from body
        peers := r.decryptPeerList(result.Items[0].Body)
        
        // Add ourselves (requires auth, optional)
        if token := os.Getenv("GITHUB_TOKEN"); token != "" {
            r.appendPeer(myInfo, token)
        }
        
        return peers, nil
    }
    
    // First node: create meeting point (requires auth)
    token := os.Getenv("GITHUB_TOKEN")
    if token == "" {
        // Fall back to DHT if no token
        return nil, ErrNoAuthForCreate
    }
    
    return nil, r.createIssue(myInfo, token)
}

func (r *RendezvousRegistry) decryptPeerList(body string) []PeerInfo {
    // Body format: base64(Encrypt(gossip_key, json([PeerInfo, ...])))
    encrypted, _ := base64.StdEncoding.DecodeString(body)
    decrypted := decrypt(r.GossipKey, encrypted)
    
    var peers []PeerInfo
    json.Unmarshal(decrypted, &peers)
    return peers
}
```

**Fallback chain:**

```go
func (n *Node) Bootstrap() []PeerInfo {
    var peers []PeerInfo
    
    // Layer 0: Public registry (works behind firewalls, first-node friendly)
    if rv := n.rendezvous.FindOrCreate(n.myInfo()); rv != nil {
        peers = append(peers, rv...)
    }
    
    // Layer 1: LAN multicast (instant if peers on same network)
    peers = append(peers, n.lan.Discover(3*time.Second)...)
    
    // Layer 2: DHT (public internet, may expose IP)
    if len(peers) == 0 {
        peers = append(peers, n.dht.GetPeers(n.currentInfohash())...)
    }
    
    return deduplicate(peers)
}
```

**Registry repo structure:**

The shared public repo (`wgmesh-registry/public`) is just a place to create issues. Anyone can:
- Search issues (no auth)
- Read issue bodies (no auth)
- Create issues (needs GitHub account)
- Edit own issues (needs auth)

Each mesh gets its own issue. The issue title is the search term. The body is encrypted peer data.

```
wgmesh-registry/public
├── Issues:
│   ├── #1: "wgmesh-a1b2c3d4" (body: encrypted peer list for mesh A)
│   ├── #2: "wgmesh-e5f6g7h8" (body: encrypted peer list for mesh B)
│   └── ...
```

**Privacy properties:**
- Issue title reveals only that a mesh with that hash exists
- Body is encrypted — observer can't see who's in the mesh
- No IP addresses visible (unlike DHT)
- GitHub sees the encrypted blob but can't decrypt
- Searching doesn't reveal your IP to other mesh members

**When to skip registry:**
- `--no-registry` flag for pure DHT/LAN discovery
- Mesh already has peers (just use gossip)
- Private mesh with bootstrap peer provided via `--bootstrap`

---

## Layer 1: LAN Multicast Discovery

**Purpose:** Sub-second discovery on local networks. Works offline, no internet required.

**Mechanism:**

- Bind UDP socket to a well-known multicast group (e.g., `239.192.77.69:51821`)
- Periodically (every 5s) broadcast an encrypted announcement:
  ```
  ANNOUNCE = Encrypt(gossip_key, {
    wg_pubkey:    base64,
    mesh_ip:      "10.x.y.z",
    wg_endpoint:  "192.168.1.50:51820",
    routable_nets: ["192.168.10.0/24"],
    timestamp:    unix_epoch,
    nonce:        random_bytes(12)
  })
  ```
- Only nodes with the same secret can decrypt — others see random bytes
- On receiving a valid announcement: add/update peer via `wg set`

**Go implementation notes:**
- Use `golang.org/x/net/ipv4` for multicast group join
- Also send on IPv6 link-local multicast `ff02::1` for dual-stack environments
- Include a protocol version byte so future changes don't break old nodes

**Time to mesh (LAN):** 1-5 seconds.

---

## Layer 2: BitTorrent Mainline DHT (Internet Discovery)

**Purpose:** Global peer discovery with zero infrastructure. The public BitTorrent DHT has millions of nodes and has been running for ~20 years.

**Why this is the right choice:**
- No server to run, no account to create, no API to pay for
- Proven at planetary scale (tens of millions of concurrent nodes)
- Go library exists: `github.com/anacrolix/dht/v2`
- Bootstrap nodes are well-known and highly available (`router.bittorrent.com:6881`, `router.utorrent.com:6881`, `dht.transmissionbt.com:6881`)

**Mechanism:**

```
1. Bootstrap into Mainline DHT (UDP, standard BEP 5 protocol)
2. announce_peer(network_id, wg_listen_port)
   - This publishes our IP:port on the DHT under our network_id
3. get_peers(network_id) → returns list of IP:port pairs
   - These are other nodes that announced the same network_id
4. For each discovered IP:port, initiate encrypted peer exchange (see below)
5. Re-announce every 15 minutes (DHT tokens rotate every 5-10 min)
6. Re-query get_peers every 30 seconds until mesh is stable, then every 60s
```

**Encrypted Peer Exchange Protocol (runs over UDP or TCP to discovered peers):**

After DHT gives us an IP:port, we don't yet know the node's WG pubkey. We need a small side-channel handshake:

```
Client → Server:  HELLO || nonce_c(12) || Encrypt(gossip_key, nonce_c, {
                    protocol: "wgmesh-v1",
                    wg_pubkey: <our pubkey>,
                    mesh_ip: <our mesh ip>,
                    wg_endpoint: <our best guess at public endpoint>,
                    routable_networks: [...],
                    timestamp: <unix>
                  })

Server → Client:  REPLY || nonce_s(12) || Encrypt(gossip_key, nonce_s, {
                    protocol: "wgmesh-v1",
                    wg_pubkey: <their pubkey>,
                    mesh_ip: <their mesh ip>,
                    wg_endpoint: <their endpoint>,
                    routable_networks: [...],
                    known_peers: [<list of other peers they know about>],
                    timestamp: <unix>
                  })
```

The `known_peers` field enables transitive discovery — even if the DHT is slow, once two nodes connect they share their full peer lists, accelerating mesh convergence.

**Security considerations:**
- The DHT infohash (network_id) is publicly visible. An observer can see that *some* IPs are participating in *some* network, but cannot determine it's WireGuard or decrypt the peer exchange.
- To mitigate: rotate network_id periodically by including a time component: `SHA256(secret || floor(unix_time / 3600))`. Nodes check both current and previous hour's IDs during transition.
- DHT announce requires outbound UDP. Nodes behind strict corporate firewalls or CGNAT may not be able to announce, but can still query `get_peers` and initiate connections to public nodes.

**Time to mesh (internet):** 15-60 seconds typical. First `get_peers` response usually arrives within 5-15s.

---

## Layer 3: In-Mesh Gossip

**Purpose:** Once WireGuard tunnels are up, use them for faster, more reliable peer exchange.

**Mechanism:**
- Each node runs a small gossip protocol *inside* the WG mesh (over the wg0 interface)
- UDP port derived from secret: `gossip_port = 51821 + (uint16(HKDF(secret, "gossip-port")) % 1000)`
- Every 10 seconds, pick a random known peer and exchange full peer lists
- Encrypted with gossip_key (belt-and-suspenders; WG already encrypts, but this authenticates mesh membership at the application layer)

**Why this layer matters:**
- DHT has inherent lag (minutes). Gossip converges in seconds once any two nodes are connected.
- Handles the "third node joins" case: Node C finds Node A via DHT. Node A tells Node C about Node B via gossip. Node C connects to Node B without needing a separate DHT lookup.
- Handles routable_networks changes propagating quickly across the mesh.

---

## Daemon Mode Implementation

The existing wgmeshbuilder has two modes: imperative CLI (`--add`, `--deploy` over SSH) and the new daemon mode.

### New CLI Surface

```bash
# Initialize and print a new mesh secret
wgmesh init --secret
# Output: wgmesh://K7x2mP9qR4sT8vW1xY3zA5bC7dE9fG0hI2jK4lM6n

# Join a mesh (runs as daemon)
wgmesh join --secret "K7x2mP9qR4..."

# Join with additional options
wgmesh join --secret "K7x2mP9qR4..." \
  --advertise-routes "192.168.10.0/24,10.0.0.0/8" \
  --listen-port 51820 \
  --interface wg0 \
  --log-level debug

# Show mesh status
wgmesh status --secret "K7x2mP9qR4..."

# Generate QR code for the secret
wgmesh qr --secret "K7x2mP9qR4..."

# Existing SSH-based mode still works for centralized management
wgmesh --add node1:10.99.0.1:192.168.1.10
wgmesh --deploy
```

### Daemon Loop (pseudo-code)

```go
func RunDaemon(secret string, opts DaemonOpts) {
    // Derive all keys/params from secret
    cfg := DeriveConfig(secret)

    // Generate or load WG keypair from local state
    localNode := LoadOrCreateLocalNode(cfg)

    // Start WG interface
    EnsureWGInterface(cfg.InterfaceName, localNode)

    // Start all discovery layers concurrently
    peers := NewPeerStore()

    go LANMulticastDiscovery(cfg, localNode, peers)
    go DHTDiscovery(cfg, localNode, peers)
    go InMeshGossip(cfg, localNode, peers)

    // Main reconciliation loop
    ticker := time.NewTicker(5 * time.Second)
    for range ticker.C {
        desired := peers.GetAll()
        current := ReadCurrentWGConfig(cfg.InterfaceName)
        diff := ComputeDiff(current, desired)  // reuse existing diff logic

        if diff.HasChanges() {
            ApplyWGChanges(cfg.InterfaceName, diff)   // wg set commands
            ApplyRouteChanges(cfg.InterfaceName, diff) // ip route commands
            UpdatePersistentConfig(cfg, desired)       // write wg0.conf for reboot survival
        }
    }
}
```

---

## Peer Store Design

Central data structure that all three discovery layers write into. The reconciliation loop reads from it.

```go
type PeerInfo struct {
    WGPubKey         string
    MeshIP           string
    Endpoint         string            // best known endpoint (ip:port)
    RoutableNetworks []string
    LastSeen         time.Time
    DiscoveredVia    []string          // ["lan", "dht", "gossip"]
    Latency          *time.Duration    // measured via WG handshake
}

type PeerStore struct {
    mu    sync.RWMutex
    peers map[string]*PeerInfo  // keyed by WG pubkey
}

// Merge logic: newest timestamp wins for mutable fields (endpoint, routable_networks)
// A peer is considered dead after 5 minutes of no updates from any layer
// Dead peers are removed from WG config after 10 minutes grace period
```

---

## Security Model

### Threat: Secret Compromise
If the shared secret leaks, an attacker can join the mesh. Mitigations:
- WireGuard PSK (derived from secret) provides forward secrecy per-session
- Implement `wgmesh rotate-secret` that coordinates a secret rotation across the mesh via in-mesh gossip (all nodes switch simultaneously after a grace period)
- Consider supporting short-lived secrets: `wgmesh join --secret "..." --expires 24h`

### Threat: DHT Surveillance
An observer watching the DHT can see which IPs announce the same infohash. Mitigations:
- Rotate infohash hourly (include time component in derivation)
- Use Tor-style onion routing through the DHT (overkill for most cases)
- For high-security deployments, disable DHT layer and use LAN + pre-seeded peers only

### Threat: Replay Attacks on Peer Exchange
An attacker captures and replays encrypted peer exchange messages. Mitigations:
- Timestamp in every message, reject messages older than 60 seconds
- Nonce ensures each message is unique
- Gossip_key + AES-256-GCM provides authentication

### Threat: Mesh IP Collision
Two nodes derive the same mesh IP. Mitigations:
- Detection via gossip (two different pubkeys claiming same IP)
- Deterministic resolution: lower pubkey (lexicographic) wins
- Loser re-derives with nonce++ until unique
- /16 gives 65534 usable addresses; collision probability < 0.1% for meshes under 50 nodes

---

## Privacy-Enhanced Architecture (Blockchain-Inspired)

This section describes optional privacy enhancements drawn from Bitcoin (Dandelion++), I2P (garlic routing, floodfill), and anonymity network research. These features hide mesh topology and membership from external observers.

### Design Goals

1. **Membership Privacy:** External observers cannot enumerate mesh members
2. **Topology Privacy:** Even mesh members have limited view of full topology
3. **Origin Privacy:** Observers cannot determine which node originated a message
4. **Temporal Unlinkability:** Observations at time T1 cannot be correlated with T2

### Enhanced Key Derivation

```
secret (arbitrary string, min 16 chars)
  │
  ├─ network_id(t)    = SHA256(secret || floor(unix/3600))[0:20]   → Rotating DHT infohash
  ├─ prev_network_id  = SHA256(secret || floor(unix/3600) - 1)[0:20] → Previous hour (overlap)
  │
  ├─ membership_key   = HKDF(secret, "wgmesh-membership-v1", 32)   → For membership proofs
  ├─ gossip_key       = HKDF(secret, "wgmesh-gossip-v1", 32)       → Symmetric encryption
  ├─ epoch_seed       = HKDF(secret, "wgmesh-epoch-v1", 32)        → Peer rotation entropy
  │
  ├─ mesh_subnet      = HKDF(secret, "wgmesh-subnet-v1", 2)        → /16 allocation
  ├─ psk              = HKDF(secret, "wgmesh-wg-psk-v1", 32)       → WireGuard PresharedKey
  │
  └─ dandelion_prob   = 0.10  (fixed: 10% chance to fluff per hop)
```

### Feature 1: Rotating DHT Infohash (Temporal Unlinkability)

**Problem:** Static infohash = permanent surveillance target. Observer builds member list over time.

**Solution:** Derive infohash from secret + current hour. Nodes announce/query both current and previous hour to handle boundary transitions.

```go
func CurrentInfohashes(secret string) [][]byte {
    now := time.Now().Unix()
    currentHour := now / 3600
    
    return [][]byte{
        sha256(secret + fmt.Sprint(currentHour))[:20],     // Current
        sha256(secret + fmt.Sprint(currentHour - 1))[:20], // Previous (2-hour window)
    }
}

// Announce to current infohash only
// Query both infohashes (catches peers who haven't rotated yet)
```

**Operational behavior:**
- At minute 0 of each hour, new infohash becomes active
- Nodes announce to new infohash immediately
- Nodes query both old and new for 60 minutes
- Observer must maintain continuous surveillance to track membership

**Complexity:** Low. Just add time component to existing derivation.

---

### Feature 2: Membership Proof Tokens (Authenticated Queries)

**Problem:** Anyone can query the DHT and see which IPs are participating, even without the secret.

**Solution:** Require cryptographic proof of mesh membership before responding to peer exchange requests.

```go
// Token proves: "I know the secret and I'm asking at approximately this time"
func GenerateMembershipToken(membershipKey []byte, myPubkey []byte) []byte {
    hourBucket := time.Now().Unix() / 3600
    message := append(myPubkey, []byte(fmt.Sprint(hourBucket))...)
    return hmac.New(sha256.New, membershipKey).Sum(message)[:16]
}

func ValidateMembershipToken(membershipKey []byte, theirPubkey, token []byte) bool {
    hourBucket := time.Now().Unix() / 3600
    
    // Check current and previous hour (clock skew tolerance)
    for _, bucket := range []int64{hourBucket, hourBucket - 1} {
        message := append(theirPubkey, []byte(fmt.Sprint(bucket))...)
        expected := hmac.New(sha256.New, membershipKey).Sum(message)[:16]
        if hmac.Equal(expected, token) {
            return true
        }
    }
    return false
}
```

**Peer exchange flow:**

```
Attacker (no secret):
  1. Queries DHT → gets IP:port list
  2. Connects to mesh node
  3. Sends: HELLO {pubkey: X, token: <garbage>}
  4. Mesh node validates token → FAILS
  5. Mesh node closes connection silently (no error = no oracle)

Legitimate node:
  1. Queries DHT → gets IP:port list  
  2. Connects to mesh node
  3. Sends: HELLO {pubkey: X, token: HMAC(...)}
  4. Mesh node validates token → OK
  5. Mesh node responds with encrypted peer list
```

**What attacker learns:** Only that IPs are listening on that port. No WG pubkeys, no mesh IPs, no routing info.

**Complexity:** Low. Standard HMAC, fits in existing handshake.

---

### Feature 3: Dandelion++ Announcement Relay (Origin Privacy)

**Problem:** When a new node announces to DHT, its IP is directly visible. Observer sees: "IP X just joined mesh Y."

**Solution:** Adopt Bitcoin's Dandelion++ protocol. New nodes relay their announcement through existing mesh peers before it reaches the DHT.

**Two phases:**

1. **Stem Phase:** Announcement hops through 1-4 mesh peers privately
2. **Fluff Phase:** Final relay publishes to DHT (or floods via gossip)

```
                    ┌─────────────────────────────────────────────┐
                    │              STEM PHASE                      │
                    │   (private, only mesh peers see)             │
                    └─────────────────────────────────────────────┘
                                        │
New Node ──secret──▶ Peer A ──secret──▶ Peer B ──secret──▶ Peer C
                                                              │
                                                              │ 10% chance
                                                              │ each hop
                                                              ▼
                    ┌─────────────────────────────────────────────┐
                    │              FLUFF PHASE                     │
                    │   (public DHT or mesh-wide gossip)           │
                    └─────────────────────────────────────────────┘
                                        │
                                        ▼
                              DHT.Announce(Peer C's IP, ...)
                                        
Observer sees: "Peer C announced" — but C is just the relay, not the origin.
```

**Implementation:**

```go
type DandelionAnnounce struct {
    OriginPubkey     []byte    // WG pubkey of actual new node
    OriginMeshIP     string    // Mesh IP of actual new node
    OriginEndpoint   string    // NAT-detected endpoint of origin
    RoutableNetworks []string
    HopCount         uint8     // Incremented each hop
    Timestamp        int64
    Nonce            []byte
}

func (n *Node) HandleDandelionAnnounce(msg DandelionAnnounce, fromPeer PeerInfo) {
    // Validate: is this from a mesh member?
    if !n.peerStore.Has(fromPeer.Pubkey) {
        return // Drop silently
    }
    
    // Add origin to our peer store (we now know about them)
    n.peerStore.AddOrUpdate(PeerInfo{
        Pubkey:   msg.OriginPubkey,
        MeshIP:   msg.OriginMeshIP,
        Endpoint: msg.OriginEndpoint,
        // ...
    })
    
    // Decide: continue stem or start fluff?
    if rand.Float64() < 0.10 || msg.HopCount >= 4 {
        // FLUFF: announce to DHT using OUR endpoint as the visible IP
        // Origin's IP never touches DHT
        n.dht.Announce(n.currentInfohash(), n.endpoint)
        
        // Also flood via in-mesh gossip
        n.gossip.Broadcast(msg)
    } else {
        // STEM: forward to one random peer
        msg.HopCount++
        nextPeer := n.epoch.RelayPeers[rand.Intn(len(n.epoch.RelayPeers))]
        n.SendViaTunnel(nextPeer, msg)
    }
}
```

**First node bootstrap:** When mesh is empty, first node must announce directly (no peers to relay through). Subsequent joiners use Dandelion.

**Epoch-based relay selection:** Each node picks 2 "Dandelion relay" peers per epoch (10 min). All stem traffic goes through these specific peers, preventing graph-learning attacks.

```go
type Epoch struct {
    ID            uint64
    RelayPeers    []PeerInfo  // Exactly 2 peers for Dandelion stem
    StartedAt     time.Time
    Duration      time.Duration // 10 minutes default
}

func (n *Node) RotateEpoch() {
    allPeers := n.peerStore.GetAll()
    if len(allPeers) < 2 {
        n.epoch.RelayPeers = allPeers
    } else {
        // Deterministic but unpredictable selection
        seed := hmac.New(sha256.New, n.epochSeed).Sum(
            []byte(fmt.Sprint(n.epoch.ID + 1)),
        )
        rng := rand.New(rand.NewSource(int64(binary.BigEndian.Uint64(seed))))
        rng.Shuffle(len(allPeers), func(i, j int) {
            allPeers[i], allPeers[j] = allPeers[j], allPeers[i]
        })
        n.epoch.RelayPeers = allPeers[:2]
    }
    n.epoch.ID++
    n.epoch.StartedAt = time.Now()
}
```

**Formal guarantees (from Dandelion++ paper):**
- With 15% adversary nodes, expected anonymity set ≈ 4 possible originators
- Near-optimal among routing-only solutions (no encryption overhead)
- 10% fluff probability → average 10 hops before fluff (geometric distribution)

**Complexity:** Medium. Requires epoch management and relay selection logic.

---

### Feature 4: I2P-Style Private Floodfill (Alternative to Public DHT)

**Problem:** Public BitTorrent DHT inherently exposes participating IPs.

**Solution:** Replace public DHT with mesh-internal "floodfill" nodes that replicate the peer registry among themselves. New nodes discover floodfills via LAN broadcast or a single bootstrap hint.

```
                    ┌─────────────────────────────────────────────┐
                    │         MESH-INTERNAL FLOODFILL              │
                    │   (never touches public infrastructure)      │
                    └─────────────────────────────────────────────┘

Mesh nodes:   A ── B ── C ── [D*] ── E ── F ── [G*] ── H

              [D*] and [G*] are floodfill nodes (volunteer, high uptime)
              
New node joins:
  1. LAN broadcast → discovers D*
  2. Queries D* with membership token
  3. D* returns encrypted peer list
  4. D* replicates new node info to G*
  5. Zero public DHT exposure
```

**Floodfill election:**
```go
// Nodes self-elect as floodfill based on:
// - Uptime > 1 hour
// - Bandwidth > threshold
// - Mesh has < 3 floodfills visible to this node

func (n *Node) ShouldBecomeFloodfill() bool {
    if n.uptime < 1*time.Hour {
        return false
    }
    if n.measuredBandwidth < 1_000_000 { // 1 Mbps
        return false
    }
    knownFloodfills := n.peerStore.CountByRole(RoleFloodfill)
    return knownFloodfills < 3
}
```

**Floodfill replication protocol:**
```go
// Floodfills sync peer registries via encrypted gossip
type FloodfillSync struct {
    Peers     []PeerInfo  // Full peer list
    Timestamp int64
    Signature []byte      // Signed by floodfill's WG key
}

// On receiving new peer (via Dandelion or direct):
func (ff *FloodfillNode) ReplicatePeer(newPeer PeerInfo) {
    for _, otherFF := range ff.knownFloodfills {
        ff.SendViaTunnel(otherFF, FloodfillSync{
            Peers:     []PeerInfo{newPeer},
            Timestamp: time.Now().Unix(),
        })
    }
}
```

**Hybrid mode:** Can run both public DHT and private floodfill. DHT for initial bootstrap (find first floodfill), then switch to floodfill-only for privacy.

**Complexity:** High. Requires floodfill election, replication protocol, failure handling.

---

### Feature 5: Garlic Bundling for Gossip Messages (Traffic Analysis Resistance)

**Problem:** Even with WireGuard encryption, message sizes and timing patterns can reveal topology. "Node A sends 200-byte messages to B every 30s" → A and B are directly connected.

**Solution (from I2P):** Bundle multiple gossip messages into single encrypted "garlic" packets. Each message ("clove") is encrypted to its final recipient; intermediate nodes can't read contents.

```
Node A wants to send updates to B, C, D:

Traditional:
  A → B: {update for B}
  A → C: {update for C}  
  A → D: {update for D}
  
  Observer sees: 3 messages from A, can infer A's connections

Garlic bundled:
  A → random relay R: {
    Clove1: Encrypt(B.pubkey, {update for B, next_hop: B}),
    Clove2: Encrypt(C.pubkey, {update for C, next_hop: C}),
    Clove3: Encrypt(D.pubkey, {update for D, next_hop: D}),
  }
  
  R peels cloves, forwards each to next_hop
  Observer sees: 1 large message from A to R, can't infer A's actual connections
```

**Implementation:**

```go
type GarlicBulb struct {
    Cloves []GarlicClove
}

type GarlicClove struct {
    EncryptedPayload []byte  // Encrypted to final recipient
    NextHop          []byte  // Pubkey of next relay (or final dest)
}

func (n *Node) SendGarlicBundled(messages []OutgoingMessage) {
    bulb := GarlicBulb{}
    
    for _, msg := range messages {
        // Build onion layers: final dest can decrypt innermost
        clove := GarlicClove{
            EncryptedPayload: Encrypt(msg.Dest.Pubkey, msg.Payload),
            NextHop:          msg.Dest.Pubkey,
        }
        bulb.Cloves = append(bulb.Cloves, clove)
    }
    
    // Send entire bulb to random relay
    relay := n.peerStore.RandomPeer()
    n.SendViaTunnel(relay, bulb)
}

func (n *Node) HandleGarlicBulb(bulb GarlicBulb) {
    for _, clove := range bulb.Cloves {
        if bytes.Equal(clove.NextHop, n.pubkey) {
            // We're the final destination
            payload := Decrypt(n.privkey, clove.EncryptedPayload)
            n.ProcessGossipMessage(payload)
        } else {
            // Forward to next hop
            nextPeer := n.peerStore.GetByPubkey(clove.NextHop)
            if nextPeer != nil {
                n.SendViaTunnel(nextPeer, clove)
            }
        }
    }
}
```

**When to bundle:**
- Batch gossip updates (instead of sending immediately, queue for 100-500ms)
- Pad to fixed sizes (1KB, 2KB, 4KB) to prevent size-based fingerprinting
- Add chaff cloves (encrypted garbage to random peers) for traffic uniformity

**Complexity:** High. Requires message queuing, batching logic, padding, relay selection.

**Honest assessment:** This is probably overkill unless threat model includes nation-state adversary doing traffic analysis on encrypted VPN traffic. WireGuard already encrypts everything; garlic adds protection against *timing and size correlation* only.

---

### Feature 6: Unidirectional Tunnels (I2P-Style)

**Problem:** Bidirectional connections reveal both endpoints. If attacker compromises one node, they see all its peers.

**Solution (from I2P):** Separate inbound and outbound paths. Messages from A→B go through different relays than B→A.

```
A's outbound tunnel:  A → R1 → R2 → R3 (outbound endpoint)
B's inbound tunnel:   (inbound gateway) R4 → R5 → R6 → B

A sends to B:
  A → R1 → R2 → R3 → [R3 forwards to R4] → R4 → R5 → R6 → B

Compromise R3: attacker sees messages leave toward R4, but doesn't know B
Compromise R4: attacker sees messages arrive from R3, but doesn't know A
```

**Complexity:** Very high. Requires tunnel building, maintenance, path selection.

**Honest assessment:** Extreme overkill for VPN mesh. This is Tor/I2P-level anonymity infrastructure. Only consider if building an anonymity network, not a VPN.

---

### Privacy Feature Summary

| Feature | Protection | Complexity | Recommendation |
|---------|------------|------------|----------------|
| Registry rendezvous | First-node bootstrap without DHT | Low | **Phase 1: Include** |
| Rotating infohash | Temporal unlinkability | Low | **Phase 1: Include** |
| Membership tokens | Block unauthenticated queries | Low | **Phase 1: Include** |
| Epoch-based relay rotation | Limit topology mapping window | Low | **Phase 1: Include** |
| Dandelion++ announcements | Hide new joiner IP | Medium | **Phase 2: Include** |

**Removed from scope (overkill for VPN use case):**
- Private floodfill (replaced by simpler registry approach)
- Garlic bundling (WireGuard already encrypts; marginal benefit)
- Unidirectional tunnels (Tor/I2P-level; not needed)

---

### Privacy-Enhanced Join Flow

```
1. New node starts with just the secret
   └─ Derives: rotating network_id, membership_key, gossip_key, epoch_seed, ...

2. Bootstrap (find first peer)
   ├─ LAN: UDP multicast with membership token → instant if peers on LAN
   └─ WAN: Query DHT for current + previous hour infohashes

3. Connect to discovered peer
   ├─ TCP/UDP to peer's IP:port
   ├─ Send: HELLO {my_pubkey, membership_token}
   ├─ Peer validates token (HMAC check)
   └─ Peer responds: {their_pubkey, mesh_ip, peer_list, ...} encrypted with gossip_key

4. Dandelion announcement (if peers exist)
   ├─ Create DandelionAnnounce message
   ├─ Send to epoch relay peer (stem phase)
   ├─ Relay forwards with 90% prob, fluffs with 10%
   └─ Eventually reaches DHT via some relay's IP (not ours)

5. Configure WireGuard
   ├─ wg set wg0 peer <pubkey> endpoint <ip:port> allowed-ips <mesh_ip>/32
   └─ Apply routes for routable_networks

6. Start gossip daemon
   ├─ Receive peer updates via in-mesh gossip
   ├─ Rotate epoch every 10 minutes
   └─ Re-announce via Dandelion every 15 minutes

7. Steady state
   ├─ DHT queries use rotating infohash
   ├─ All announcements go through Dandelion relays
   ├─ Epoch rotation limits topology inference window
   └─ Membership tokens block unauthorized probes
```

---

### Privacy Mode CLI Options

```bash
# Standard mode (public DHT, no Dandelion)
wgmesh join --secret "..."

# Privacy mode (rotating infohash + membership tokens + Dandelion)
wgmesh join --secret "..." --privacy

# Maximum privacy (private floodfill, no public DHT)
wgmesh join --secret "..." --privacy --no-public-dht --bootstrap-peer "1.2.3.4:51820"

# Paranoid mode (garlic bundling, traffic padding)
wgmesh join --secret "..." --privacy --garlic --pad-traffic
```

Configuration in mesh config:

```yaml
privacy:
  rotate_infohash: true          # Hourly DHT key rotation
  membership_tokens: true        # Require auth for peer exchange
  dandelion:
    enabled: true
    stem_probability: 0.90       # 90% chance to continue stem
    max_hops: 4
  epoch_duration: 10m            # Relay peer rotation interval
  
  # Advanced (Phase 3+)
  floodfill:
    enabled: false               # Use private floodfill instead of public DHT
    min_uptime: 1h
    min_bandwidth: 1mbps
  garlic:
    enabled: false
    batch_delay: 200ms
    pad_to_sizes: [1024, 2048, 4096]
```

---

## Wire Format: QR Code / Token

The QR code / token encodes a URI:

```
wgmesh://v1/<base64url-encoded-secret>
```

That's it. Everything else is derived.

**Optional extensions** (appended as query params, all optional):

```
wgmesh://v1/<secret>?routes=192.168.10.0/24&port=51820&name=my-mesh
```

**QR code generation:**

```bash
echo "wgmesh://v1/$(head -c 32 /dev/urandom | base64url)" | qrencode -t UTF8
```

---

## File Structure Changes

```
wgmeshbuilder/
├── main.go                          # Add: join, status, qr subcommands
├── pkg/
│   ├── mesh/                        # Existing: centralized mesh management
│   │   ├── types.go
│   │   ├── mesh.go
│   │   └── deploy.go
│   ├── daemon/                      # NEW: decentralized daemon mode
│   │   ├── daemon.go               # Main daemon loop + reconciliation
│   │   ├── config.go               # Config parsing + defaults
│   │   ├── peerstore.go            # Thread-safe peer store with merge logic
│   │   └── epoch.go                # Epoch management + relay peer rotation
│   ├── discovery/                   # NEW: all discovery layers
│   │   ├── registry.go             # Layer 0: GitHub Issue rendezvous (NEW)
│   │   ├── lan.go                  # Layer 1: UDP multicast
│   │   ├── dht.go                  # Layer 2: BitTorrent Mainline DHT
│   │   ├── gossip.go               # Layer 3: in-mesh gossip
│   │   └── exchange.go             # Encrypted peer exchange protocol
│   ├── crypto/                      # NEW: key derivation + message encryption
│   │   ├── derive.go              # HKDF-based derivation from secret
│   │   ├── envelope.go            # Encrypt/decrypt gossip messages (AES-256-GCM)
│   │   └── membership.go          # Membership token generation/validation
│   ├── privacy/                     # NEW: blockchain-inspired privacy features
│   │   ├── dandelion.go           # Dandelion++ stem/fluff relay
│   │   └── epoch.go               # Privacy-aware epoch management
│   ├── wireguard/                   # Existing: WG config management
│   │   ├── keys.go
│   │   ├── config.go
│   │   ├── apply.go
│   │   └── convert.go
│   └── ssh/                         # Existing: remote SSH operations
│       ├── client.go
│       └── wireguard.go
└── mesh-state.json                  # Existing (for centralized mode)
```

---

## New Go Dependencies

```
github.com/anacrolix/dht/v2    # Mainline DHT implementation
golang.org/x/net/ipv4           # Multicast group management
golang.org/x/crypto/hkdf        # Key derivation
github.com/skip2/go-qrcode      # QR code generation (optional, for wgmesh qr)
```

---

## Phased Implementation Plan

### Phase 1: Foundation (daemon mode + bootstrap + LAN discovery)
- [ ] Enhanced key derivation from secret (`pkg/crypto/derive.go`)
  - Rotating network_id (hourly)
  - Rendezvous ID for registry search term
  - Membership key for tokens
  - Epoch seed for relay rotation
- [ ] **Public registry rendezvous** (`pkg/discovery/registry.go`) ← NEW
  - GitHub Issue search/create
  - Encrypted peer list in issue body
  - Fallback chain: registry → LAN → DHT
- [ ] Membership token generation/validation (`pkg/crypto/membership.go`)
- [ ] Encrypted message envelope (`pkg/crypto/envelope.go`)
- [ ] PeerStore with merge logic (`pkg/daemon/peerstore.go`)
- [ ] Epoch management + relay peer rotation (`pkg/daemon/epoch.go`)
- [ ] Daemon main loop + reconciliation using existing diff engine (`pkg/daemon/daemon.go`)
- [ ] LAN multicast discovery with membership tokens (`pkg/discovery/lan.go`)
- [ ] CLI: `wgmesh join --secret`, `wgmesh status`, `wgmesh init --secret`
- **Milestone:** First node creates registry, second node finds it. LAN discovery works. No DHT needed for basic operation.

### Phase 2: Internet discovery (DHT + Dandelion++)
- [ ] DHT bootstrap + announce + get_peers (`pkg/discovery/dht.go`)
- [ ] Rotating infohash queries (current + previous hour)
- [ ] Encrypted peer exchange with membership token validation (`pkg/discovery/exchange.go`)
- [ ] Transitive peer sharing (known_peers in exchange)
- [ ] Dandelion++ stem/fluff relay (`pkg/privacy/dandelion.go`)
  - Epoch-based relay selection (2 peers per epoch)
  - 10% fluff probability per hop
  - Max 4 hops before forced fluff
- [ ] `--privacy` CLI flag to enable Dandelion++ by default
- **Milestone:** Two nodes on different networks mesh within 60 seconds; new joiner IP not directly visible in DHT

### Phase 3: Convergence + enhanced privacy
- [ ] In-mesh gossip protocol (`pkg/discovery/gossip.go`)
- [ ] Routable networks propagation via gossip
- [ ] Dead peer detection + cleanup
- [ ] Private floodfill mode (`pkg/privacy/floodfill.go`)
  - Floodfill self-election logic
  - Peer registry replication between floodfills
  - `--no-public-dht` mode
- [ ] QR code generation (`wgmesh qr`)
- **Milestone:** 10-node mesh fully converges within 90 seconds; optional zero-DHT-exposure mode

### Phase 4: Hardening + advanced privacy
- [ ] Secret rotation protocol
- [ ] Mesh IP collision resolution
- [ ] Systemd unit generation (`wgmesh install-service`)
- [ ] Persistent peer cache (survive daemon restart without re-discovering)
- [ ] Metrics/health endpoint
- [ ] Garlic bundling for gossip (`pkg/privacy/garlic.go`)
  - Message batching (100-500ms delay)
  - Size padding (1KB/2KB/4KB buckets)
  - Chaff clove injection
- [ ] `--garlic` and `--pad-traffic` CLI flags
- **Milestone:** Production-ready with optional traffic analysis resistance

---

## Open Questions for Discussion

### Core Architecture

1. **DHT port sharing with WireGuard:** The DHT needs a UDP port. Should it share WG's listen port (complex, requires multiplexing) or use a separate port (simpler, one more port to open)?

2. **STUN integration:** Should we add STUN (e.g., `stun.l.google.com:19302`) for nodes behind NAT to discover their public endpoint? This would improve the quality of endpoint information shared via DHT. The `pion/stun` Go library makes this straightforward.

3. **TURN/relay fallback:** For nodes behind strict CGNAT where direct WG connections fail, should there be a relay mode? EasyTier does this, but it adds significant complexity and requires relay infrastructure.

4. **Backwards compatibility:** Should daemon mode be able to read/write the existing `mesh-state.json` format? This would allow mixed-mode operation where some nodes are managed centrally and others self-discover.

5. **Maximum mesh size:** The /16 subnet and collision-based IP allocation work well up to ~100 nodes. For larger meshes, consider explicit DHCP-like allocation via gossip consensus. Is >100 nodes a target?

### Privacy Features

6. **Default privacy level:** Should `--privacy` (rotating infohash + membership tokens + Dandelion++) be the default, or opt-in? Arguments for default: better security posture, no reason not to. Arguments for opt-in: simpler debugging, faster initial discovery.

7. **Dandelion++ parameters:** The spec uses 10% fluff probability (from Bitcoin). For smaller meshes, this might be too aggressive (too many hops = latency). Should we auto-tune based on mesh size? E.g., 20% for <10 nodes, 10% for 10-50, 5% for >50?

8. **Floodfill election criteria:** How to prevent Sybil attacks on floodfill election? Currently based on uptime + bandwidth. Should we add proof-of-work, stake, or social vouching?

9. **Garlic bundling latency:** Batching messages for 100-500ms adds latency. For WireGuard VPN (latency-sensitive), is this acceptable? Should we skip garlic for control plane and only use it for gossip?

10. **Threat model documentation:** Who are we protecting against? Current design handles:
    - Passive DHT observer (rotating infohash + membership tokens)
    - Passive mesh observer (Dandelion++ + epoch rotation)
    - Active DHT attacker (membership tokens block queries)
    
    But does NOT handle:
    - Global passive adversary (traffic analysis on all links)
    - Active adversary controlling mesh nodes (Sybil attack)
    - Quantum adversary (X25519 is not post-quantum)
    
    Should these be documented as explicit non-goals?

---

## References & Prior Art

### Mesh VPN / Peer Discovery

| Project | Approach | Relevance |
|---------|----------|-----------|
| [EasyTier](https://github.com/EasyTier/EasyTier) | Shared name+secret, OSPF routing, public relay nodes | Closest to target UX; uses relay servers though |
| [wgautomesh](https://git.deuxfleurs.fr/Deuxfleurs/wgautomesh) | Gossip + LAN broadcast, shared secret | Layer 1 + 3 reference implementation |
| [wiresmith](https://github.com/svenstaro/wiresmith) | Consul KV for peer registry | KV-based discovery pattern |
| [wireguard-dynamic](https://github.com/segator/wireguard-dynamic) | Token → KV store → auto-join | Simple token-based UX |
| [STUNMESH-go](https://fosdem.org/2026/schedule/event/YQWEDC-stunmesh-go_building_p2p_wireguard_mesh_without_self-hosted_infrastructure/) | STUN + Curve25519 + pluggable KV | STUN integration pattern |
| [NetBird](https://github.com/netbirdio/netbird) | WebRTC ICE + STUN + Signal server | Full NAT traversal reference |

### Self-Distributing / Git-Based Coordination

| Project | Approach | Relevance |
|---------|----------|-----------|
| [DevSwarm](https://github.com/HackrsValv/devswarm) | Git forks as replication, CI/CD as coordination, GitHub API for discovery | **Layer 0 registry inspiration** |

### DHT / P2P Infrastructure

| Project | Approach | Relevance |
|---------|----------|-----------|
| [anacrolix/dht](https://github.com/anacrolix/dht) | Go Mainline DHT implementation | Direct dependency candidate |
| [KadNode](https://github.com/whoizit/KadNode) | Mainline DHT for P2P DNS | DHT-as-rendezvous pattern |
| [Weaveworks Mesh](https://github.com/weaveworks/mesh) | Gossip with shared-secret auth | Go gossip library |

### Privacy / Anonymity Networks

| Project | Technique | Relevance |
|---------|-----------|-----------|
| [Dandelion++ (BIP-156)](https://github.com/bitcoin/bips/blob/master/bip-0156.mediawiki) | Stem/fluff transaction relay | Origin privacy for announcements |
| [Grin Dandelion](https://docs.grin.mw/wiki/miscellaneous/dandelion/) | Dandelion++ implementation in Rust | Reference implementation |
| [I2P NetDB](https://geti2p.net/en/docs/how/network-database) | Floodfill-based distributed database | Private peer registry pattern |
| [I2P Garlic Routing](https://geti2p.net/en/docs/how/garlic-routing) | Message bundling + layered encryption | Traffic analysis resistance |
| [Tor](https://www.torproject.org/) | Onion routing, directory authorities | Anonymity network reference |
| [libp2p-onion-routing](https://github.com/hashmatter/libp2p-onion-routing) | Onion routing over DHT | DHT privacy prototype |

### Academic Papers

| Paper | Year | Key Contribution |
|-------|------|------------------|
| Dandelion: Redesigning Bitcoin for Anonymity | 2017 | Formal anonymity guarantees for P2P gossip |
| Dandelion++: Lightweight Cryptocurrency Networking | 2018 | Defenses against active adversaries |
| Deanonymisation of Clients in Bitcoin P2P Network | 2014 | Attack motivating Dandelion |
| I2P: A Scalable Framework for Anonymous Communication | 2003 | Garlic routing, unidirectional tunnels |
| Kademlia: A P2P Information System | 2002 | DHT XOR distance metric |
| Practical Attacks Against the I2P Network | 2013 | I2P threat model analysis |
