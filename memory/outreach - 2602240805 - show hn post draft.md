---
tldr: Draft Show HN post for wgmesh — autonomous pipeline + WireGuard CDN
status: draft
---

# Show HN: Draft

**Title:**
Show HN: I built a zero-infra anycast CDN on WireGuard mesh — with an AI pipeline that specs, codes, and ships itself

---

wgmesh is a WireGuard-based mesh network that routes traffic across peers using DHT discovery and NAT traversal.
No cloud vendor, no managed control plane — you bring the nodes, the mesh finds them.
Traffic is distributed across whichever peers are healthy and closest, which is roughly anycast behavior without BGP or a network vendor's blessing.

The interesting part is the pipeline that builds it.
Every GitHub issue can become a shipped feature without me touching a keyboard:

1. GitHub issue describes what's needed
2. A Copilot agent writes a spec (architecture decisions, interface contracts, edge cases)
3. Goose (Block's agentic coding tool) reads the spec and implements it
4. The pipeline opens a PR, runs CI, and auto-merges when green

I didn't set out to build an autonomous SDLC.
It emerged from trying to ship faster as a solo developer.
The tradeoffs are real: the pipeline makes confident mistakes, spec quality determines implementation quality, and you still have to review the diffs.
But for a project in this stage it compresses weeks of iteration into hours.

The live dashboard at chimney.beerpub.dev shows the pipeline running: current deploys, blue/green swap state, eBPF telemetry via coroot-node-agent.

What's actually working:
- WireGuard peer mesh with DHT-based discovery
- NAT traversal without a STUN server
- Blue/green deploys via Docker Compose with automatic ancillary service startup
- Coroot-based observability (eBPF metrics, OTLP traces via Clickhouse)
- The spec/code/ship pipeline on GitHub

What isn't done yet:
- Formal anycast routing (right now it's load balancing, not true anycast)
- Multi-region failover with real SLA guarantees
- A polished onboarding path for adding your own edge node

I'm opening two early sponsor tiers: $5/mo founding member (listed on the dashboard, Discord/Matrix access, votes on roadmap) and $20/mo edge node beta (early access to run a node in the mesh when that's ready).
These are honest early backer tiers, not a product you're buying.

Code: github.com/atvirokodosprendimai/wgmesh
Live dashboard: chimney.beerpub.dev

Happy to answer questions on the mesh architecture, the pipeline design, or the tradeoffs of AI-driven SDLC in practice.

---

## Where to post

- https://news.ycombinator.com/submit (Show HN — title + URL = chimney.beerpub.dev or github.com/atvirokodosprendimai/wgmesh)
- Post on a weekday between 8–11am US Eastern for best visibility
