---
tldr: LAN multicast announces encrypted peer info every 5s on a mesh-derived multicast group; STUN detects external endpoint and classifies NAT as cone or symmetric using a shared socket.
category: core
---

# Discovery — LAN multicast and STUN/NAT detection

## Target

The two local-network discovery mechanisms: UDP multicast broadcast for peers on the same LAN,
and RFC 5389 STUN queries used to learn external endpoints and classify NAT behaviour.

## Behaviour

### LAN multicast discovery

- Joins and sends on `239.192.X.Y:51830`, where X.Y are the first two bytes of `MulticastID`
  derived from the mesh secret — every mesh network gets its own multicast group.
- Announces every 5 seconds (immediately on start).
  Announcements carry: WireGuard public key, mesh IP, mesh IPv6, gossip port, introducer flag,
  routable networks, hostname, NAT type.
  Known peers are deliberately omitted to keep the packet small.
- Announcements are envelope-encrypted with the mesh gossip key (`crypto.SealEnvelope`).
  A receiver without the secret cannot decrypt or spoof announcements.
- Inbound: decrypt → skip own public key → resolve endpoint → deliver to peer store as `"lan"`.
- Endpoint resolution: if the advertised address is `0.0.0.0`, the sender's actual UDP source IP
  is substituted (NAT-safe fallback).
- Stops gracefully on context cancel: listener socket is closed, announce ticker stopped.

### STUN (RFC 5389)

- Custom implementation — no external STUN library.
- `STUNQuery`: sends a Binding Request to a single STUN server, validates magic cookie and
  transaction ID, returns the server-reflexive (external) IP and port.
  Optional `localPort` binds the socket to a specific port (useful for port-preserving NATs).
  Prefers `XOR-MAPPED-ADDRESS`; falls back to `MAPPED-ADDRESS`.
- `DiscoverExternalEndpoint`: tries servers from `DefaultSTUNServers` in order, returns the
  first success. Default servers: two Google STUN + Cloudflare.
- `DetectNATType`: determines whether the node is behind cone or symmetric NAT.
  Uses a **single shared UDP socket** for both queries — mandatory so both measurements use
  the same source port.
  - Same external IP:port from both servers → `cone` (endpoint-independent mapping; hole-punching reliable)
  - Different IP or port → `symmetric` (per-destination mapping; relay needed for direct connection)
  - Only one server responds → `unknown` (still returns the successful result; does not error)
  - Both fail → error returned.

### NAT type values

| Value | Meaning |
|---|---|
| `cone` | Hole-punching works reliably |
| `symmetric` | Relay routing required; hole-punching unreliable |
| `unknown` | Inconclusive (one STUN server unreachable) |

## Design

- The multicast group address encodes mesh membership: nodes on different meshes use different groups and cannot decrypt each other's announcements even if they overlap on the same LAN.
- Separate send/listen sockets for multicast: listener joins the group; sender dials directly.
  This avoids receiving one's own multicast on some OS configurations.
- STUN transaction IDs are random per request.
  Responses are validated against both the transaction ID and the expected sender IP to resist spoofed responses.
- Shared socket in `DetectNATType` is the key design choice: if separate sockets were used,
  a cone NAT could assign different external ports to each, producing a false `symmetric` result.

## Interactions

- `pkg/crypto.CreateAnnouncement`, `SealEnvelope`, `OpenEnvelope` — encrypt/decrypt LAN packets.
- `pkg/daemon.PeerStore.Update` — deliver discovered peers.
- `pkg/daemon.Config.Keys.MulticastID` — mesh-specific multicast group derivation.
- `DHTDiscovery.Start()` calls `DiscoverExternalEndpoint` + `DetectNATType` on startup to populate local endpoint and NAT type.
- `daemon.PeerInfo.NATType` — NAT type is published in announcements and stored per peer.

## Mapping

> [[pkg/discovery/lan.go]]
> [[pkg/discovery/stun.go]]
