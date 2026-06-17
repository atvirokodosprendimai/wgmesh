# Specification: Issue #746 - Build technical deep-dive: How cloudroof mesh discovery works (L0-L3 architecture) for DevOps evaluators

## Classification
documentation

## Problem Analysis

DevOps evaluators evaluating cloudroof mesh networking need to understand how the decentralized discovery architecture enables self-organizing WireGuard meshes without central orchestration. The current codebase implements a sophisticated 4-layer discovery system (L0-L3), but this technical deep-dive documentation is scattered across multiple files (`pkg/discovery/*.go`, `pkg/daemon/daemon.go`, `pkg/privacy/dandelion.go`) with no unified architectural explanation.

**Knowledge gaps for evaluators:**

1. **Discovery layer responsibilities:** How L0 (GitHub Issues registry), L1 (LAN multicast), L2 (BitTorrent DHT), and L3 (in-mesh gossip) interact and failover
2. **Bootstrapping sequence:** How a new node with only a shared secret discovers its first peer
3. **Security properties:** How peer authentication and envelope encryption prevent mesh poisoning
4. **NAT traversal:** How STUN and Dandelion++ relay enable connections behind NAT/firewalls
5. **Privacy guarantees:** How the Dandelion++ relay protects peer announcement metadata
6. **Operational metrics:** What observability signals indicate healthy discovery

The spec deliverable is a comprehensive technical markdown document that explains the L0-L3 architecture with code references, sequence diagrams (in mermaid syntax), and configuration examples. This is NOT a code change—it is documentation production.

## Proposed Approach

### Phase 1: Discovery layer analysis

Analyze the four discovery layers in `pkg/discovery/`:

1. **L0 - GitHub Issues registry** (`registry.go`): 
   - Read `registry.go` to understand GitHub Issues as a bootstrap peer directory
   - Document: API endpoints, authentication (HMAC tokens), rate limiting, fallback behavior
   - Security: read-only access, HMAC token validation, no write operations needed

2. **L1 - LAN multicast** (`lan.go`):
   - Read `lan.go` to understand local subnet discovery via UDP multicast
   - Document: multicast address (224.0.0.251:5353), packet format, subnet scope
   - Security: same-subnet restriction, envelope encryption

3. **L2 - BitTorrent DHT** (`dht.go`):
   - Read `dht.go` to understand Mainline DHT peer discovery
   - Document: DHT bootstrap nodes, infohash derivation from shared secret, announce interval
   - Security: infohash = HKDF(secret, "dht-infohash"), envelope-encrypted peer records

4. **L3 - In-mesh gossip** (`gossip.go`):
   - Read `gossip.go` to understand peer-to-peer propagation
   - Document: gossip protocol, anti-entropy, duplicate suppression
   - Security: HMAC membership tokens, envelope encryption

### Phase 2: Cross-layer integration analysis

1. **Bootstrapping sequence** (`pkg/daemon/daemon.go`, `exchange.go`):
   - Trace the cold-start flow: shared secret → L0/L1 → L2 → L3
   - Document: timeout per layer, concurrent queries, success criteria

2. **NAT traversal** (`pkg/discovery/stun.go`, `exchange.go`):
   - Read `stun.go` for STUN server interaction for public endpoint discovery
   - Document: STUN protocol, fallback to Dandelion++ relay

3. **Privacy layer** (`pkg/privacy/dandelion.go`):
   - Read `dandelion.go` for announcement relay behavior
   - Document: stem phase, fluff phase, anonymity set

### Phase 3: Security analysis

1. **Cryptography** (`pkg/crypto/derive.go`, `envelope.go`, `membership.go`):
   - Read key derivation: HKDF-SHA256 from shared secret
   - Read envelope encryption: AES-256-GCM with unique nonces
   - Read membership: HMAC-SHA256 tokens

2. **Peer authentication** (`pkg/discovery/exchange.go`):
   - Document how incoming peer announcements are validated
   - Document replay protection and nonce handling

### Phase 4: Documentation structure

Create the markdown document with the following structure:

```markdown
# Cloudroof Mesh Discovery: L0-L3 Technical Deep-Dive

## Executive Summary
- High-level architecture diagram
- Discovery layer responsibilities table
- Security guarantees overview

## Discovery Layer 0: GitHub Issues Registry
- Purpose: Bootstrap peer directory for cold-start
- Mechanism: GitHub Issues as read-only peer store
- API: GitHub endpoints, authentication, rate limiting
- Security: HMAC token validation, read-only access
- Fallback: What happens when GitHub is unreachable
- Code references: pkg/discovery/registry.go

## Discovery Layer 1: LAN Multicast
- Purpose: Same-subnet peer discovery
- Mechanism: UDP multicast on 224.0.0.251:5353
- Packet format: TLV-encoded peer announcement
- Security: Subnet scope, envelope encryption
- Limitations: Does not cross routers
- Code references: pkg/discovery/lan.go

## Discovery Layer 2: BitTorrent DHT
- Purpose: Internet-scale peer discovery
- Mechanism: Mainline DHT with infohash from shared secret
- Bootstrap nodes: Public DHT routers
- Announce interval: 20 minutes
- Security: Infohash derivation, envelope encryption
- NAT traversal: How DHT peers behind NAT are found
- Code references: pkg/discovery/dht.go

## Discovery Layer 3: In-Mesh Gossip
- Purpose: Peer-to-peer propagation within connected mesh
- Mechanism: Push-based gossip with anti-entropy
- Duplicate suppression: Bloom filter or sequence numbers
- Security: HMAC membership tokens, envelope encryption
- Convergence: How gossip achieves consistent peer view
- Code references: pkg/discovery/gossip.go

## Bootstrapping Sequence
- Cold start: New node with only shared secret
- Layer activation order: L0 → L1 → L2 → L3
- Concurrent queries: How layers run in parallel
- Success criteria: Minimum peer count to enable L3
- Timeout behavior: Per-layer fallback
- Mermaid sequence diagram: Cold-start flow

## NAT Traversal
- STUN protocol: Public endpoint discovery
- STUN servers: Hardcoded bootstrap servers
- Fallback: Dandelion++ relay for direct connection failure
- Hole punching: Simultaneous open attempts
- Code references: pkg/discovery/stun.go

## Privacy: Dandelion++ Relay
- Purpose: Hide peer announcement metadata
- Stem phase: Routing through relay nodes
- Fluff phase: Local broadcast
- Anonymity set: Minimum relay count
- Code references: pkg/privacy/dandelion.go

## Cryptography Stack
- Key derivation: HKDF-SHA256 from shared secret
- Envelope encryption: AES-256-GCM
- Membership tokens: HMAC-SHA256
- Nonce handling: Unique nonces per announcement
- Code references: pkg/crypto/derive.go, envelope.go, membership.go

## Security Properties
- Peer authentication: How untrusted peers are rejected
- Mesh poisoning resistance: Envelope encryption prevents injection
- Replay attack prevention: Nonce validation
- Insider threat: What a compromised peer cannot do
- Forward secrecy: Key rotation behavior

## Operational Metrics
- Peer count: Expected peer range per layer
- Convergence time: How long until full mesh view
- Health indicators: Prometheus metrics
- Failure modes: Symptoms of layer failure
- Troubleshooting: Common issues and remediation

## Configuration Examples
- CLI flags for layer control
- Environment variables for bootstrap nodes
- YAML config for production deployments
- Debug logging for discovery troubleshooting

## Performance Characteristics
- Scalability limits: Maximum peers per layer
- Bandwidth usage: Per-peer bandwidth overhead
- Latency: Inter-peer latency distribution
- Resource usage: CPU and memory footprint
```

### Phase 5: Diagram generation

1. Create Mermaid sequence diagrams for:
   - Cold-start bootstrapping flow
   - Layer interaction and failover
   - Dandelion++ relay routing

2. Create Mermaid flowcharts for:
   - Peer announcement validation
   - NAT traversal fallback sequence
   - Gossip propagation and anti-entropy

3. Create architecture diagram showing:
   - L0-L3 layer responsibilities
   - Data flow between layers
   - Security boundaries

### Phase 6: Code reference extraction

For each section, extract relevant code snippets (truncated for readability) with file paths and function names:

```go
// Example from pkg/discovery/dht.go
func (d *DHTDiscoverer) Announce(ctx context.Context, peers []node.Peer) error {
    infohash := deriveInfoHash(d.secret) // Line 45-48
    // ... truncated for documentation
}
```

## Acceptance Criteria

1. **Document created:** `docs/discovery-l0-l3-deep-dive.md` exists with the exact section structure above

2. **L0 (GitHub Issues) documented:**
   - Purpose, mechanism, API endpoints explained
   - Security properties (HMAC token validation) documented
   - Fallback behavior when GitHub unreachable explained
   - Code references to `pkg/discovery/registry.go` provided

3. **L1 (LAN multicast) documented:**
   - Multicast address and port specified
   - Packet format and subnet scope explained
   - Security (subnet restriction, envelope encryption) documented
   - Code references to `pkg/discovery/lan.go` provided

4. **L2 (BitTorrent DHT) documented:**
   - Mainline DHT mechanism explained
   - Infohash derivation from shared secret documented
   - Bootstrap nodes and announce interval specified
   - NAT traversal behavior explained
   - Code references to `pkg/discovery/dht.go` provided

5. **L3 (Gossip) documented:**
   - Gossip protocol and anti-entropy explained
   - Duplicate suppression mechanism documented
   - Security (HMAC tokens, envelope encryption) explained
   - Convergence properties specified
   - Code references to `pkg/discovery/gossip.go` provided

6. **Bootstrapping sequence documented:**
   - Cold-start flow from shared secret to mesh membership explained
   - Layer activation order (L0 → L1 → L2 → L3) specified
   - Concurrent query behavior and timeouts documented
   - Mermaid sequence diagram included

7. **NAT traversal documented:**
   - STUN protocol and server endpoints specified
   - Dandelion++ relay fallback explained
   - Hole punching mechanism documented
   - Code references to `pkg/discovery/stun.go` provided

8. **Dandelion++ privacy documented:**
   - Stem and fluff phases explained
   - Anonymity set requirements specified
   - Code references to `pkg/privacy/dandelion.go` provided

9. **Cryptography documented:**
   - HKDF key derivation from shared secret explained
   - AES-256-GCM envelope encryption documented
   - HMAC membership tokens specified
   - Nonce handling and replay prevention explained
   - Code references to `pkg/crypto/` provided

10. **Security properties documented:**
    - Peer authentication mechanism explained
    - Mesh poisoning resistance documented
    - Insider threat limitations specified
    - Forward secrecy behavior explained

11. **Operational metrics documented:**
    - Expected peer counts per layer specified
    - Convergence time estimates provided
    - Health indicators and Prometheus metrics listed
    - Failure mode symptoms documented
    - Troubleshooting guidance provided

12. **Configuration examples provided:**
    - CLI flags for layer control
    - Environment variables for bootstrap
    - YAML config for production
    - Debug logging examples

13. **Diagrams included:**
    - At least 3 Mermaid sequence/flow diagrams
    - Architecture diagram showing L0-L3 layers
    - All diagrams render correctly in standard Mermaid viewers

14. **Code references included:**
    - Each section references relevant `pkg/discovery/*.go` files
    - Function names and line numbers provided
    - Code snippets truncated for readability (no large dumps)

15. **Public-repo safe:**
    - No hardcoded secrets or API tokens
    - No customer PII or revenue figures
    - Internal references (e.g., specific GitHub repo URLs) sanitized

## Out of scope

1. **Code changes:** This spec is documentation-only. No Go code will be modified.
2. **Performance optimization:** The document explains current behavior, not proposed optimizations.
3. **New features:** The document describes existing L0-L3 architecture, not future enhancements.
4. **WireGuard configuration:** The document focuses on discovery, not WireGuard interface setup (see separate docs).
5. **Centralized mode:** The document covers decentralized discovery only (not `pkg/mesh` SSH-based management).
6. **Testing procedures:** The document explains architecture, not test execution (see `testlab/` for testing).
7. **Deployment guides:** The document explains discovery mechanisms, not production deployment (see ops docs).
8. **API reference:** The document provides architectural overview, not function-by-function API docs (see GoDoc).
9. **Benchmarking results:** The document may describe performance characteristics, but does not include new benchmarks.
10. **Third-party integrations:** The document covers cloudroof-native discovery, not external tool integration.

## Deliverables

1. **Primary deliverable:** Markdown document at `docs/discovery-l0-l3-deep-dive.md` (approximately 2000-3000 lines)

2. **Secondary deliverables (optional):**
   - Mermaid diagram source files in `docs/diagrams/` if diagrams are complex
   - Configuration examples in `docs/examples/` if YAML/JSON snippets are extensive

3. **Review artifacts:**
   - Open PR with draft document for technical review
   - Tag security team for cryptography section review
   - Tag DevOps team for operational metrics section review
