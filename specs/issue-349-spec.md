# Spec: Issue #349 - Zero-Trust Edge Architecture with Caching Modes

## Classification

**Security Enhancement** — Hardening multi-tenant CDN against compromised/malicious edges.

## Deliverables

Code + Documentation

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

### Threat Model (assume edge is NOT trusted)
1. **Traffic interception** — edge reads tenant HTTP traffic
2. **Traffic misrouting** — edge routes gontis.com → wrong origin
3. **Selective dropping** — edge drops some requests
4. **Response tampering** — edge modifies origin response

## Proposed Approach

### Mode 1: Zero-Trust (default)
- Origin terminates TLS
- Edge forwards encrypted bytes only
- No caching at edge
- Highest security, lowest performance

### Mode 2: Trusted Edge
- Edge terminates TLS
- Can cache responses
- Edge can read/modify traffic
- Similar to Cloudflare/Fastly trust model

### Mode 3: Hybrid (Encrypted Cache)
- Origin encrypts responses with cache key
- Edge caches encrypted blobs
- Can cache but cannot read content
- Research: OblivCDN (2025)

### TPM Attestation
- Edge registers with lighthouse via TPM 2.0
- Lighthouse verifies PCR quotes
- Short-lived certificates issued

### Tenant Canary Verification
- Tenant exposes /health/verify endpoint
- Tenant periodically checks routing correctness
- Detects misrouted traffic

## Affected Files

- `pkg/attestation/tpm.go` — TPM quote generation
- `pkg/attestation/verify.go` — Attestation verification
- `pkg/edge/identity.go` — Edge identity management
- `pkg/lighthouse/attest.go` — Lighthouse attestation endpoints
- `pkg/tenant/verify.go` — Tenant verification client
- `main.go` — Add edge attest command

## Test Strategy

1. Unit tests for TPM quote generation/verification
2. Integration tests for attestation flow
3. Tenant verification tests
4. Negative tests (stolen keys, modified software, replay)
5. Formal verification with Tamarin Prover

## Estimated Complexity

High
