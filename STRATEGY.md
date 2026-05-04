---
name: wgmesh
last_updated: 2026-05-04
---

# wgmesh Strategy

## Target problem

Operators wiring agent fleets to edge nodes (e.g. exposing an openclaw webserver via cloudroof in minutes) need fast, ad-hoc, secure meshes. Existing tools either treat edge as second-class (Tailscale), feel bolted-on (Cloudflare Tunnel), or "work" in demos and fail at minute-3 of real ad-hoc setup.

## Our approach

wgmesh is autonomous — the mesh discovers, peers, and heals itself with no control plane to host or trust, so ad-hoc agent-to-edge setups complete in minutes and survive past minute-3.

## Who it's for

**Primary:** LLM agent builders running stacks like Hermes / openclaw — devs not devops, force-of-nature tinkerers who don't know ops and don't want to learn. They're hiring wgmesh to wire agents + services across boxes and edge in minutes, without becoming a sysadmin.

## Key metrics

- **Paying customers** — count of active Polar.sh subscriptions. Lagging.
- **Cost coverage ratio** — MRR ÷ monthly cost. Single number for "are we alive."
- **Weekly active meshes** — distinct rendezvous IDs alive in DHT/registry over rolling 7d. Source: chimney.
- **Time-to-mesh (p50)** — median seconds from `wgmesh join` to first handshake on internet path. Source: daemon logs / synthetic probe.
- **Unsolicited positive feedback rate** — count/week of GitHub issues, DMs, Show HN comments tagged positive.

## Tracks

### Mesh autonomy core

Discovery layers (L0 GitHub / L1 LAN multicast / L2 DHT / L3 in-mesh gossip), reconcile loop, secret rotation, NAT survival.

_Why it serves the approach:_ this **is** the autonomy — without a control plane, discovery and self-healing must be load-bearing reliable.

### Edge as first-class

Lighthouse CDN, managed ingress, the openclaw→cloudroof path. The use case Tailscale and Cloudflare both lose.

_Why it serves the approach:_ autonomy extends to the edge node — no bolt-on tunnel, edge is a peer not an afterthought.

### Commercial pipe (Bet A)

Polar.sh billing, landing repositioning at cloudroof.eu, cost tracking, distribution (Show HN, tutorial, stargazer outreach).

_Why it serves the approach:_ revenue without a growth/sales team — the autonomy thesis applied to the company itself.

## Milestones

- **2026-Q2** — first paying customer (any tier, even $5/mo) and `costs.json` populated enough to compute runway.
- **2026-Q3** — ~10 paying customers, dashboard at chimney.beerpub.dev shows them by name, Lighthouse-managed-ingress is self-serve.

## Not working on

- **Hosted control plane for someone else's wgmesh deployment** — we are not Headscale-as-a-service. cloudroof.eu uses wgmesh internally as part of a bundled edge offering; we don't sell a control plane for your mesh.
- **Windows native client** — Linux + Docker covers the homelab market. Two months to ship something worse than what's in the box.
- **Custom WireGuard fork** — every WG fork has regretted it within 18 months. Stay on upstream.
- **Free tier with "limit removed in paid"** — either it's free or it's paid; in-between makes the product feel cheap.
- **Proprietary protocol on top of WG** — breaks interop, which is half the value.

## Marketing

**One-liner:** wgmesh — a WireGuard mesh that just works.

**Key message:** If your reaction to "Tailscale's control plane" is "I'd rather not," wgmesh is the tool. If your reaction is "fine, sounds easy," Tailscale is the tool. No coordination server, no accounts, one secret, EU-first.
