---
tldr: Roadmap to first paying customer — homelab LLM operator using wgmesh as AI service gateway with managed ingress
---

# First Customer

Turn wgmesh from a working open-source project into a product with its first paying customer within 3 months. The customer is a homelab LLM operator who runs local AI services (Ollama, Open WebUI, ComfyUI, etc.) and needs to expose them to external clients over HTTPS without port forwarding, Cloudflare tunnels, or managing a VPN server.

## Target

wgmesh has all the core networking implemented — decentralized discovery, encrypted mesh, multi-arch builds, CI/CD. What's missing is the product layer: a managed ingress proxy that turns "nodes on a mesh" into "services reachable from the internet," plus enough trust signals and polish that someone will pay for it.

The first customer comes from personal network, which lowers the trust bar but still requires the product to work reliably and be worth paying for.

## Behaviour

### The pitch

"Share a secret, expose your AI. Run `wgmesh join` on your GPU box, register your services, and they're live at `https://ollama.yourmesh.wgmesh.dev` — no port forwarding, no Cloudflare, no ops."

### What the customer gets

- One-command mesh join (already works)
- Service registration: declare what's running on a node (`wgmesh service add ollama :11434`)
- Managed ingress: Lighthouse edge proxies route `https://<service>.<mesh>.wgmesh.dev` to the right mesh node over WireGuard
- TLS termination at the edge (automatic certs via Let's Encrypt / Caddy)
- Simple auth: API key or mesh membership required to reach services through ingress
- Status visibility: `wgmesh status` shows connected nodes, registered services, and ingress URLs

### What the operator does

1. Signs up (gets a mesh account + API key)
2. Runs `wgmesh join --secret <secret> --account <api-key>` on each machine
3. Registers services: `wgmesh service add ollama :11434`
4. Services appear at managed URLs immediately
5. Shares URLs (with auth tokens) with whoever needs access

## Design

### Phase 1: Service registry + CLI (weeks 1-3)

Add service registration to the daemon:
- `wgmesh service add <name> <host:port>` — registers a local service
- `wgmesh service list` — shows registered services on this node
- `wgmesh service remove <name>` — deregisters
- Services stored in local daemon state and announced via gossip
- RPC extension: `services.list`, `services.get` methods

### Phase 2: Managed ingress via Lighthouse (weeks 3-6)

Evolve Lighthouse from CDN control plane into the managed ingress product:
- Lighthouse learns about services via mesh gossip or API registration
- Edge proxies (Caddy) get dynamic upstreams from Lighthouse via xDS or REST config
- Route `https://<service>.<mesh-id>.wgmesh.dev` → mesh node running that service
- TLS termination at edge with automatic cert provisioning
- Auth layer: API key header or mesh token validation before proxying
- Health checks: Lighthouse monitors service availability through the mesh
- Failover: if a service runs on multiple nodes, route to healthiest

### Phase 3: Account + billing scaffold (weeks 5-8)

Minimal account system — just enough to charge:
- Account creation (API key issuance)
- Mesh ownership (which account owns which mesh)
- Usage tracking: bandwidth through ingress, number of services, number of nodes
- Stripe integration for billing (or manual invoicing for customer #1)
- No web dashboard required — account management via CLI or direct support

### Phase 4: Trust + polish (weeks 6-10)

Make it feel like a product someone should pay for:
- Landing page at wgmesh.dev explaining the AI gateway use case
- Quickstart guide specifically for "expose Ollama in 5 minutes"
- `wgmesh` install one-liner (`curl -fsSL https://wgmesh.dev/install | sh`)
- Status page showing edge proxy uptime
- Comparison page: wgmesh vs Tailscale Funnel vs Cloudflare Tunnel vs ngrok
- Clean `--help` output, clear error messages, graceful failure modes

### Phase 5: Dogfood + harden (weeks 8-12)

Run it yourself, then hand it to the customer:
- Dogfood the full flow on own infrastructure (already partially done via Chimney)
- Fix reliability issues discovered during dogfooding
- Write runbook for common failure modes
- Set up monitoring/alerting for the managed ingress
- Onboard first customer with direct support

## Verification

- Can join a mesh and register a service in under 2 minutes on a fresh Linux box
- Service is reachable at `https://<service>.<mesh>.wgmesh.dev` within 60 seconds of registration
- Ingress handles service going offline gracefully (returns 503, not connection timeout)
- Customer's Ollama is accessible from their phone/laptop outside the home network
- First invoice sent and paid

## Friction

- **DNS wildcard**: `*.wgmesh.dev` wildcard DNS + certs needed for per-mesh subdomains. Caddy handles this but needs DNS challenge provider.
- **Edge location**: Single edge proxy region means latency for geographically distant customers. Acceptable for customer #1 from personal network. Multi-region comes later.
- **NAT traversal reliability**: Ingress proxy needs reliable connectivity to mesh nodes behind NAT. The mesh's existing STUN + keepalive should handle this, but edge cases (strict CGNAT) may need relay fallback.
- **Pricing uncertainty**: Deferring pricing means customer #1 pricing will be ad-hoc. Need to track costs (compute, bandwidth, DNS) to inform future pricing.
- **Single-tenant assumption**: Account system is minimal. Multi-tenant isolation, rate limiting per account, and abuse prevention are deferred.

## Interactions

- Depends on existing mesh networking (daemon, discovery layers, gossip)
- Evolves Lighthouse (`cmd/lighthouse/`, `pkg/lighthouse/`) into the ingress product
- Builds on Chimney deployment patterns (`deploy/chimney/`) for edge proxy infrastructure
- Service registration extends the RPC interface (`pkg/rpc/`)
- Gossip protocol (`pkg/discovery/gossip.go`) carries service announcements

## Mapping

> [[cmd/lighthouse/main.go]]
> [[pkg/lighthouse/api.go]]
> [[pkg/lighthouse/xds.go]]
> [[pkg/rpc/server.go]]
> [[pkg/discovery/gossip.go]]
> [[pkg/daemon/daemon.go]]
> [[deploy/chimney/bluegreen.sh]]

## Boundaries

Explicitly out of scope for first-customer milestone:
- Mobile clients (iOS/Android)
- Web management dashboard (CLI + direct support is enough)
- Multi-region edge (single edge location)
- Custom domains (only `*.wgmesh.dev` subdomains)
- IPv6
- Secret rotation protocol
- Public marketplace / self-serve signup

## Future

{[!] Multi-region edge proxies — deploy to 2-3 locations for latency}
{[!] Web dashboard for service and mesh management}
{[!] Self-serve signup and onboarding flow}
{[!] Custom domain support with automatic TLS}
{[?] Usage-based pricing model vs flat rate}
{[?] Relay fallback for nodes behind strict CGNAT}
{[?] Service mesh features — load balancing, circuit breaking, retries}
