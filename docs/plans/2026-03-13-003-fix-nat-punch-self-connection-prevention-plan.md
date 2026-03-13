---
title: "fix: NAT punch self-connection — node tries to hole-punch to its own public IP"
type: fix
status: completed
date: 2026-03-13
---

# fix: NAT punch self-connection — node tries to hole-punch to its own public IP

## Problem

A netcup node (`198.51.100.1`) repeatedly attempts to NAT-punch to its own public IP, producing 40 HELLO timeouts:

```
time=2026-03-12T19:00:37.595Z level=INFO msg="[NAT] Punch timeout with 198.51.100.1:52790 after 40 HELLO attempts"
```

The node wastes ~4 seconds per self-punch attempt and pollutes logs. If the node has symmetric NAT or is behind CGNAT, the self-punch consumes introducer resources too.

## Root Cause

Three self-connection prevention gaps in the discovery layer:

### Gap 1: `ExchangeWithPeer()` doesn't check target endpoint against self

**File:** `pkg/discovery/exchange.go:437`

When `ExchangeWithPeer()` is called, it sends HELLO to a remote address without checking whether that address is the node's own public endpoint. If a peer's endpoint resolves to the local node's own IP (e.g., through gossip or transitive discovery), it will attempt and timeout.

### Gap 2: `getKnownPeers()` doesn't filter self from advertisements

**File:** `pkg/discovery/exchange.go:1032`

When advertising known peers to other nodes, `getKnownPeers()` iterates `peerStore.GetActive()` without filtering out `localNode.WGPubKey`. If the local node is somehow in its own peer store (via transitive gossip), its endpoint gets advertised, creating a loop where other nodes (or itself via another gossip round) might try to connect self to self.

### Gap 3: `controlEndpointForPeer()` doesn't validate against self endpoint

**File:** `pkg/discovery/dht.go:1124`

The function that resolves a peer's control endpoint for punching only checks if the endpoint is valid and public — not whether it matches the local node's own endpoint.

### Existing self-prevention (partial)

These checks exist but don't cover the gaps above:
- `handleHello()` (exchange.go:293) — discards incoming HELLO from self (by WGPubKey)
- `selectRendezvousIntroducers()` (dht.go:1255) — skips self as introducer
- `updateTransitivePeers()` (exchange.go:916) — skips self when parsing transitive peers
- `tryTransitivePeersWithBackoff()` (dht.go:1040) — skips self

## Proposed Solution

### Fix 1: Self-endpoint guard in `ExchangeWithPeer()`

Before sending HELLO, compare the target address against the local node's public endpoint:

```go
// pkg/discovery/exchange.go — ExchangeWithPeer(), before HELLO loop
func (pe *PeerExchange) ExchangeWithPeer(remoteAddr *net.UDPAddr, ...) error {
    // Prevent punching to own public endpoint
    ownEndpoint := pe.localNode.GetEndpoint()
    if ownEndpoint != "" {
        ownHost, _, _ := net.SplitHostPort(ownEndpoint)
        if ownHost == remoteAddr.IP.String() {
            log.Printf("[NAT] Skipping punch to own public IP %s", remoteAddr.IP)
            return fmt.Errorf("refusing to punch to own endpoint")
        }
    }
    // ... existing HELLO loop
}
```

### Fix 2: Self-filter in `getKnownPeers()`

```go
// pkg/discovery/exchange.go — getKnownPeers()
func (pe *PeerExchange) getKnownPeers() []crypto.KnownPeer {
    active := pe.peerStore.GetActive()
    var peers []crypto.KnownPeer
    for _, p := range active {
        if p.WGPubKey == "" || p.WGPubKey == pe.localNode.WGPubKey {
            continue  // don't advertise self
        }
        peers = append(peers, p)
    }
    return peers
}
```

### Fix 3: Self-endpoint check in `controlEndpointForPeer()`

```go
// pkg/discovery/dht.go — controlEndpointForPeer()
func (d *DHTDiscovery) controlEndpointForPeer(peer crypto.KnownPeer) string {
    // ... existing endpoint resolution ...

    // Don't return own endpoint
    ownEndpoint := d.localNode.GetEndpoint()
    if ownEndpoint != "" {
        ownHost, _, _ := net.SplitHostPort(ownEndpoint)
        resolvedHost, _, _ := net.SplitHostPort(endpoint)
        if ownHost == resolvedHost {
            log.Printf("[NAT] Refusing to use own endpoint %s for peer %s", endpoint, peer.WGPubKey[:8])
            return ""
        }
    }

    return endpoint
}
```

## Acceptance Criteria

- [x] Node does not attempt NAT punching to its own public IP
- [x] `getKnownPeers()` excludes self from advertised peer list
- [x] `controlEndpointForPeer()` rejects endpoints matching own public IP
- [x] Existing self-prevention checks remain in place (defense in depth)
- [x] Unit tests for self-connection prevention in each fixed function
- [x] No regression in legitimate NAT punching between different nodes

## Context

### Why the IP check (not just WGPubKey check)?

The WGPubKey check covers the case where self is explicitly in the peer store. But the IP check covers edge cases where:
- A peer behind the same NAT gateway has the same public IP (though different port)
- Endpoint IP is known but WGPubKey mapping is stale or missing

Both checks together provide defense in depth.

### Note on same-NAT peers

Two nodes behind the same NAT gateway will share a public IP but have different ports. The self-check should compare IP only (not IP:port) for the self case, but must NOT block same-NAT peers (different WGPubKey, same IP, different port). The WGPubKey check is primary; the IP check is supplementary for when the peer is definitively self.

**Revised approach**: Use WGPubKey as the primary self-check. Only use IP comparison as a log warning, not a hard block, to avoid false positives with same-NAT peers.

## Sources

- Log evidence: `[NAT] Punch timeout with 198.51.100.1:52790 after 40 HELLO attempts` (2026-03-12)
- `pkg/discovery/exchange.go` — `ExchangeWithPeer()`, `getKnownPeers()`, `handleHello()`
- `pkg/discovery/dht.go` — `controlEndpointForPeer()`, `selectRendezvousIntroducers()`, `tryTransitivePeersWithBackoff()`
- `pkg/discovery/stun.go` — STUN public IP detection
