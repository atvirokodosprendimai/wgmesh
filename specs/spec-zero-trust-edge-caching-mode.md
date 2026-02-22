# Spec: Zero-Trust Edge Architecture with TPM Attestation

## Classification

**Security Enhancement** — Hardening multi-tenant CDN against compromised/malicious edges.

## Problem Analysis

### Current Trust Model
```
Tenant → Edge (trusted by default) → Origin
```
Edge is trusted to:
- Route traffic correctly to tenant origin
- Not snoop on tenant traffic
- Not misroute traffic to wrong tenant

If edge is compromised or misconfigured:
- Tenant A traffic → routes to Tenant B origin
- Tenant traffic gets sniffed/modified
- No cryptographic protection against edge

### Threat Model (assume edge is NOT trusted)
1. **Traffic interception** — edge reads tenant HTTP traffic
2. **Traffic misrouting** — edge routes gontis.com → wrong origin
3. **Selective dropping** — edge drops some requests
4. **Response tampering** — edge modifies origin response

## Proposed Approach

### Layer 1: End-to-End Encryption (primary defense)

```
User → [TLS: user ↔ tenant origin] → Edge → [WireGuard: edge ↔ origin] → Origin
```

**Edge cannot decrypt traffic** — TLS terminates at origin, not edge.

```
┌──────────────────────────────────────────────────────────────────┐
│ Flow:                                                            │
│                                                                   │
│  1. User connects to edge (HTTPS gontis.com)                   │
│  2. Edge proxies to origin via WireGuard mesh                  │
│  3. TLS handshake: user ↔ origin (edge sees only encrypted)    │
│  4. Edge forwards encrypted bytes, cannot read/modify           │
└──────────────────────────────────────────────────────────────────┘
```

**Requirement:** Origin must have valid TLS certificate for domain.

### Layer 2: TPM-Based Edge Identity & Attestation

Even with E2E encryption, edge can:
- Drop traffic (DoS)
- Misroute to wrong origin IP (but TLS fails if wrong cert)
- Selective forwarding based on user/headers

**TPM attestation flow:**
```
┌──────────────────────────────────────────────────────────────────┐
│ Edge Bootstrap:                                                  │
│                                                                   │
│ 1. Edge boots with TPM 2.0                                      │
│ 2. TPM measures: UEFI → kernel → OS → wgmesh binary → config   │
│ 3. Edge generates identity key in TPM (AK - Attestation Key)    │
│ 4. Edge registers with lighthouse:                               │
│    - TPM EK certificate (proves hardware)                       │
│    - PCR quotes (proves software state)                         │
│    - WireGuard pubkey                                            │
│ 5. Lighthouse verifies:                                         │
│    - TPM EK certificate valid                                    │
│    - PCRs match expected values (known-good config)             │
│    - Issue edge certificate (signed, short-lived)                │
└──────────────────────────────────────────────────────────────────┘
```

**Attestation prevents:**
- Stolen edge identity (requires TPM to sign)
- Modified software (PCRs won't match)
- Replay attacks (nonce in attestation)

### Layer 3: Continuous Verification

```
┌──────────────────────────────────────────────────────────────────┐
│ Runtime Monitoring:                                              │
│                                                                   │
│ - Origin periodically attests to lighthouse:                     │
│   "I am gontis.com, my mesh IP is X, here is TLS cert hash"    │
│                                                                   │
│ - Lighthouse verifies:                                           │
│   - Origin pubkey matches registration                           │
│   - TLS cert hash matches expected                               │
│   - Origin is reachable (health check)                           │
│                                                                   │
│ - If mismatch:                                                   │
│   - Alert tenant (traffic may be misrouted)                      │
│   - Revoke edge trust                                            │
└──────────────────────────────────────────────────────────────────┘
```

### Layer 4: Tenant-Side Verification (falsifiable)

```
┌──────────────────────────────────────────────────────────────────┐
│ Tenant monitoring:                                               │
│                                                                   │
│ - Tenant runs "canary" endpoint: /health/verify                 │
│   Returns: signed token with origin mesh IP + timestamp         │
│                                                                   │
│ - Tenant periodically checks:                                    │
│   1. Resolve gontis.com → get edge IP(s)                        │
│   2. Connect to edge, make request to /health/verify            │
│   3. Verify token signature + origin IP matches expected         │
│                                                                   │
│ - If verification fails: tenant alerts → investigating          │
└──────────────────────────────────────────────────────────────────┘
```

This is Popper-style: tenant can *falsify* that traffic reaches correct origin.

## Architecture

### Components

1. **TPM Attestation Service** (`pkg/attestation/`)
   - TPM 2.0 quote generation and verification
   - PCR measurement collection
   - EK certificate validation

2. **Edge Identity Manager** (`pkg/edge/identity.go`)
   - Generates WireGuard keys in TPM (if supported) or software
   - Handles registration with lighthouse
   - Rotates short-lived certificates

3. **Lighthouse Attestation Verification** (`pkg/lighthouse/attest.go`)
   - Verifies edge TPM quotes
   - Maintains registry of attested edges
   - Issues short-lived routing authorization

4. **Tenant Verification Client** (`pkg/tenant/verify.go`)
   - Canary endpoint implementation
   - Periodic verification checks
   - Alerting on failure

### Data Structures

```go
// EdgeAttestation represents edge's attested state
type EdgeAttestation struct {
    EdgeID          string            // Unique edge identifier
    TPM_EK_Cert     []byte            // TPM Endorsement Key certificate
    PCRs            map[int][]byte    // Platform Configuration Registers
    WireGuardPubKey string            // Edge's WireGuard public key
    MeshIP          string            // Edge's IP in cloudroof mesh
    AttestedAt      time.Time         // Attestation timestamp
    ValidUntil      time.Time         // Expiration
    Signature       []byte            // TPM signature over attestation
}

// TenantCanaryResponse for verification
type TenantCanaryResponse struct {
    Domain      string    `json:"domain"`
    OriginMeshIP string   `json:"origin_mesh_ip"`
    Timestamp   int64     `json:"timestamp"`
    Nonce       string    `json:"nonce"`
    Signature   []byte    `json:"signature"` // Signed by origin's mesh key
}
```

### API Endpoints

**Lighthouse:**
- `POST /v1/edges/register` — Edge registers with TPM attestation
- `POST /v1/edges/attest` — Periodic re-attestation
- `GET /v1/edges/status` — List attested edges
- `DELETE /v1/edges/{id}` — Revoke edge trust

**Origin (tenant):**
- `GET /.well-known/tenant-verify` — Canary endpoint for verification

## Test Strategy

1. **Unit tests for TPM quote generation/verification**
   - Mock TPM interface for testing
   - PCR mismatch detection
   - Certificate chain validation

2. **Integration tests for attestation flow**
   - Simulated edge registration
   - Attestation verification
   - Edge revocation

3. **Tenant verification tests**
   - Canary token generation
   - Verification success/failure cases
   - Timing attack resistance

4. **Negative tests**
   - Stolen keys rejected
   - Modified software detected
   - Replay attacks prevented

5. **Formal Verification with Tamarin Prover**
   - Model attestation protocol security properties
   - Verify edge cannot forge attestation
   - Verify tenant can detect misrouting
   - Find attacks on routing protocol

## Affected Files

- `pkg/attestation/tpm.go` — NEW: TPM quote generation
- `pkg/attestation/verify.go` — NEW: Attestation verification
- `pkg/edge/identity.go` — NEW: Edge identity management
- `pkg/lighthouse/attest.go` — NEW: Lighthouse attestation endpoints
- `pkg/tenant/verify.go` — NEW: Tenant verification client
- `main.go` — Add `edge attest` command

## Estimated Complexity

**Medium-High** — Requires TPM 2.0 integration, attestation protocol design, and distributed verification system.

## Security Properties

| Threat | Zero-Trust Mode | Trusted Mode | Hybrid Mode |
|--------|-----------------|--------------|--------------|
| Edge reads tenant traffic | ❌ No (E2E TLS) | ⚠️ Yes | ❌ No |
| Edge misroutes traffic | TLS fails if wrong origin | Possible | TLS fails |
| Edge caches responses | ❌ No | ✅ Yes | ✅ Encrypted only |
| Performance | Lower | Highest | Medium |
| Use case | Sensitive data | Static content | Sensitive + cache |

## Trade-offs

- **TPM availability:** Not all VPS have TPM 2.0 — software attestation fallback
- **Performance:** Attestation adds latency — cache and batch verification
- **Complexity:** More components = more attack surface in attestation system itself
- **User burden:** Tenant must run verification checks (can be automated)

## Caching Modes: Tenant Choice

```
┌─────────────────────────────────────────────────────────────────────┐
│                    TENANT CACHING MODE MENU                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  MODE 1: Zero-Trust (default for security-sensitive tenants)       │
│  ─────────────────────────────────────────────────────────────────  │
│  • E2E TLS: client ↔ origin (edge cannot decrypt)                 │
│  • Edge only forwards encrypted bytes                              │
│  • NO caching at edge                                             │
│  • Use case: banking, health data, sensitive APIs                 │
│                                                                     │
│  MODE 2: Trusted Edge (performance-optimized)                      │
│  ─────────────────────────────────────────────────────────────────  │
│  • TLS termination at edge (Keyless SSL style)                    │
│  • Edge can cache responses                                        │
│  • Edge CAN read/modify traffic                                    │
│  • Similar to Cloudflare Keyless SSL                              │
│  • Use case: static content, public APIs, media                    │
│                                                                     │
│  MODE 3: Hybrid (best of both worlds)                             │
│  ─────────────────────────────────────────────────────────────────  │
│  • Origin encrypts responses with cache-key-K                      │
│  • Edge caches E(K, response) - encrypted blobs                    │
│  • Edge can cache but CANNOT read content                          │
│  • Client decrypts with K                                          │
│  • Inspired by OblivCDN research (2025)                          │
│  • Use case: sensitive but cachable content                       │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Prior Art Research

### Encrypted Caching

| Solution | Edge Can Read | Caching | Status |
|----------|---------------|---------|--------|
| Keyless SSL (Cloudflare) | Yes (session key) | Yes | Production |
| OblivCDN (2025) | No | Yes | Academic paper, no open source |
| OCDN (2017, Princeton) | No | Yes | Academic paper |
| mTLS + proxy | No | Hard | Standard practice |

**Key insight:** Mode 3 (hybrid) is an active research area. OblivCDN (ASIA CCS '25) shows it's practical with ORAM primitives (256MB video in 5.6s), but no open source implementation exists.

### TPM Attestation

| Solution | Description | Status |
|----------|-------------|--------|
| RFC 9683 | TPM-based Network Device Remote Integrity Verification | IETF Standard (Dec 2024) |
| RFC 9684 | CHARRA - Challenge-Response-Based Remote Attestation | IETF Standard (Dec 2024) |
| Keylime | Open source TPM attestation for Linux | Active project |

**Key insight:** TPM attestation is well-standardized. We can use existing libraries (go-tpm, Keylime) rather than building from scratch.

## Formal Verification with Tamarin Prover

We will use **Tamarin Prover** (https://tamarin-prover.com/) to formally verify security properties:

### What to Verify

1. **Attestation Protocol**
   - Edge cannot forge valid attestation without TPM
   - Lighthouse can detect tampered PCRs
   - Short-lived certificates prevent replay

2. **Canary Verification Protocol**
   - Tenant can detect misrouted traffic
   - Edge cannot predict/forge canary tokens
   - Freshness (nonces) prevent replay

3. **Routing Integrity**
   - Edge cannot route tenant A traffic to tenant B without detection
   - Origin identity verification prevents impersonation

### Tamarin Workflow

```
1. Model protocol in Tamarin's symbolic language
2. Define security properties (lemmas):
   - "edge_attestation_authenticity"
   - "tenant_canary_verification"  
   - "routing_integrity"
3. Tamarin finds attacks or proves properties
4. Fix any attacks found
5. Use proven model as reference for implementation
```

### Resources

- Tamarin Book (2025): "Modeling and Analyzing Security Protocols with Tamarin"
- Existing models: TLS 1.3, WireGuard, Privacy Pass (reference implementations)

## Implementation Approach

Given prior art, we implement:

1. **Mode 1 (E2E TLS)**: Origin terminates TLS, edge forwards encrypted bytes
2. **Mode 2 (Keyless)**: Use TLS key exchange with origin, edge caches plaintext (trust model)
3. **Mode 3 (Hybrid)**: Future work - requires custom ORAM-like implementation or research partnership

### Mode Details

#### Mode 1: Zero-Trust (E2E TLS)

```
User --[TLS]--> Edge --[WireGuard]--> Origin terminates TLS
```

- Tenant provides TLS cert on origin
- Edge sees only encrypted bytes
- Full isolation, zero trust, no caching

#### Mode 2: Trusted Edge (TLS termination)

```
User --[TLS]--> Edge terminates TLS --> Cache --> Origin
```

- Edge manages TLS (Let's Encrypt or tenant-provided cert)
- Edge can cache responses
- Edge can log/analyze/modify traffic
- Same trust model as Cloudflare/Fastly

#### Mode 3: Encrypted Cache (Hybrid)

```
User --[TLS]--> Edge
                │
                ├─ Cache hit: return E(K, response)
                │
                └─ Cache miss: forward to origin
                                    │
                                    ▼
                              Origin encrypts with K
                                    │
                                    ▼
                              Edge stores E(K, response)
```

- Origin encrypts responses before sending
- Edge caches encrypted blobs
- Edge has cache-key-K but NOT origin's app key
- Can cache but can't read content
- Requires origin to support encrypted responses

### Implementation

```go
// TenantConfig specifies caching mode
type TenantConfig struct {
    Domain        string    `json:"domain"`
    CachingMode   string    `json:"caching_mode"` // "zero-trust" | "trusted" | "hybrid"
    CacheKey      string    `json:"cache_key,omitempty"` // For hybrid mode
    TLSCertPath   string    `json:"tls_cert_path,omitempty"` // For trusted mode
}
```

### Trade-off Summary

| Mode | Caching | Edge Can Read | Security | Performance |
|------|---------|---------------|----------|-------------|
| Zero-Trust | ❌ | ❌ | Highest | Lowest |
| Trusted | ✅ | ✅ | Lowest | Highest |
| Hybrid | ✅ | ❌ | High | Medium |
