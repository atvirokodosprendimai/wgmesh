---
title: "NAT punch attempts self-connection when node's public IP matches punch target"
category: logic-errors
date: 2026-03-13
tags: [nat, hole-punching, self-connection, peer-exchange, dht, cgnat]
modules: [pkg/discovery/exchange.go, pkg/discovery/dht.go]
---

## Problem

A netcup node repeatedly attempted NAT hole punching to its own public IP (`198.51.100.1:52790`), logging `[NAT] Punch timeout ... after 40 HELLO attempts`. The node was effectively trying to connect to itself, wasting resources and never succeeding.

## Root Cause

Three gaps in self-connection prevention across the discovery subsystem:

1. **`ExchangeWithPeer`** — no check whether the punch target's IP matches the node's own public IP. The exchange proceeds unconditionally.
2. **`getKnownPeers`** — no filter excluding the local node from the peer list shared with remote peers. Also no filter for peers with empty WGPubKey.
3. **`controlEndpointForPeer` (DHT)** — no WGPubKey self-check, allowing the DHT layer to return the node's own endpoint as a valid peer endpoint.

The underlying pattern: **IP addresses are not reliable self-identifiers** because same-NAT and CGNAT peers share the same public IP. The safe self-identifier is `WGPubKey`.

## Solution

### 1. Warning (not block) on IP match in ExchangeWithPeer (exchange.go)

```go
ownEndpoint := pe.localNode.GetEndpoint()
if ownEndpoint != "" {
    ownHost, _, splitErr := net.SplitHostPort(ownEndpoint)
    if splitErr == nil && ownHost == remoteAddr.IP.String() {
        log.Printf("[NAT] Warning: punch target %s matches own public IP (possible self-connection)", addrStr)
    }
}
```

This is a **warning only** — hard-blocking by IP would break same-NAT/CGNAT peers that legitimately share a public IP.

### 2. Self-filter and empty-pubkey filter in getKnownPeers (exchange.go)

```go
if p.WGPubKey == "" || p.WGPubKey == pe.localNode.WGPubKey {
    continue
}
```

### 3. WGPubKey self-check in controlEndpointForPeer (dht.go)

```go
if peer.WGPubKey == d.localNode.WGPubKey {
    return ""
}
```

## Prevention

- **Use WGPubKey (not IP) as the canonical self-identifier** in peer-to-peer filtering. IPs are shared under NAT/CGNAT; WGPubKey is globally unique.
- **IP-based checks should warn, not block** — they're useful for diagnostics but unreliable for access control in NAT environments.
- **Filter empty identifiers** alongside self-checks — both are invalid peers.
