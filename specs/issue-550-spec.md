# Specification: Issue #550

## Classification
documentation

## Deliverables
documentation

## Problem Analysis

wgmesh and cloudroof.eu are proven-functional mesh networking products, but the sales pipeline is
idle due to a lack of identified, qualified prospects. The product's key value propositions are:

- **Zero-server decentralized mesh**: nodes discover each other via DHT, LAN multicast, and GitHub
  Issues registry — no coordination server to host or trust
- **NAT traversal out of the box**: UDP hole-punching, no firewall rules required on most nodes
- **WireGuard-native encryption**: industry-standard crypto, not a bespoke protocol
- **Two deployment modes**: decentralized (`wgmesh join`) for dynamic fleets, centralized
  (SSH-based) for operator-managed infrastructure

The gap is not product-readiness but prospect identification. The deliverable is a single Markdown
document at `docs/customer-research.md` that:

1. Catalogues 20+ specific customer segments / personas who have an acute need for mesh networking
2. Documents each segment's current pain points and what alternatives they likely use today
3. Provides reusable outreach message templates, one per segment
4. Defines concrete pilot/trial qualification criteria an operator can use to qualify a lead
5. Lists case-study interview questions to run with the first paying or pilot customers

## Implementation Tasks

### Task 1: Create `docs/customer-research.md`

Create the file `docs/customer-research.md` with the following exact content:

````markdown
# Target Customer Research & Outreach Strategy

This document identifies qualified prospects for wgmesh / cloudroof.eu mesh networking solutions,
catalogues their pain points, and provides ready-to-send outreach templates, pilot qualification
criteria, and early-adopter case study questions.

---

## Customer Segments and Personas

### Segment 1 — Remote-First Engineering Teams (5–50 engineers)

**Who they are:** Software companies that went fully remote. Engineers work from home or co-working
spaces across multiple countries. The team runs staging, CI runners, internal tools (Grafana,
Vault, internal APIs) on a mix of VPS providers and home servers.

**Pain points:**
- Using a shared VPN (Tailscale, Cloudflare WARP, or OpenVPN) that requires a coordinator they
  must either pay for or self-host
- Adding a new team member means manual VPN provisioning (creating an account, distributing a
  config) — takes 30–60 minutes
- VPN coordinator is a single point of failure: if it goes down, no remote access
- Compliance and data-sovereignty teams worry about a US SaaS VPN (Tailscale) touching metadata

**Current alternatives:** Tailscale (free tier for small teams), Cloudflare WARP, WireGuard with
manual key distribution

**Why wgmesh fits:** Share one secret → any new machine joins automatically, no coordinator SPoF,
keys never leave the mesh

**Example personas:**
- "Platform engineer at a 15-person YC-backed startup who has to onboard 3 contractors per month"
- "Solo DevOps at a 40-person company who manages all infra but has no time to babysit VPN issues"

---

### Segment 2 — Self-Hosted Infrastructure Operators (Homelab / Prosumers)

**Who they are:** Technically sophisticated individuals running Proxmox, TrueNAS, or Kubernetes at
home plus a VPS or two in the cloud. They want to expose home services (Jellyfin, Nextcloud,
Vaultwarden) to themselves and trusted family members.

**Pain points:**
- Tailscale free tier has device count limits (100 devices as of 2024) and requires account sign-in
  on every device
- WireGuard with manual configs breaks every time an endpoint IP changes (residential DHCP)
- Cloudflare Tunnel works for HTTP only; doesn't help for SSH, gaming, or raw TCP

**Current alternatives:** Tailscale (free/plus tier), Headscale (self-hosted Tailscale coordinator),
manual WireGuard configs, ZeroTier

**Why wgmesh fits:** No account, no coordinator server, works with dynamic IPs via DHT, open source
and auditable

**Example personas:**
- "r/homelab power user with 12 nodes across home, Hetzner VPS, and a Raspberry Pi at parents'
  house"

---

### Segment 3 — IoT / Edge Computing Deployments

**Who they are:** Companies that deploy firmware or agents on devices at customer premises — POS
terminals, industrial sensors, retail kiosks, solar inverter monitors. They need a secure
back-channel from each device to their cloud without exposing a public management port.

**Pain points:**
- Devices sit behind CG-NAT or strict firewalls; direct SSH or VPN server connection fails
- Cloud VPN concentrators (AWS Client VPN, Azure VPN Gateway) cost $0.05–0.10/hr per connection
  and need complex certificate infrastructure
- ZeroTier and Tailscale require an internet-reachable coordinator or planet servers

**Current alternatives:** AWS Client VPN, Azure VPN Gateway, ZeroTier Business, custom WireGuard +
dynamic DNS

**Why wgmesh fits:** DHT-based discovery punches through CG-NAT without a coordinator; mesh IP is
deterministic from device identity; no per-connection cloud cost

**Example personas:**
- "CTO of a 20-person cleantech startup deploying solar monitoring hardware at 500 sites, needs
  remote access to each device without paying cloud VPN fees"

---

### Segment 4 — Multi-Cloud / Hybrid Infrastructure Teams

**Who they are:** Engineering teams running workloads across AWS, GCP, Azure, and on-premises. They
need east-west connectivity between services without going through the public internet or paying for
cloud transit.

**Pain points:**
- AWS Transit Gateway, VPC Peering, and Azure VNet Peering are region-locked and expensive at scale
- Cross-cloud connectivity requires managed VPN or Direct Connect / ExpressRoute — complex and costly
- Adding a new cloud account or region means updating every existing peer's routing tables

**Current alternatives:** AWS Transit Gateway, HashiCorp Consul Connect / service mesh,
Cloudflare Argo Tunnel, Netmaker

**Why wgmesh fits:** Centralized mode pushes configs via SSH with diff-based updates; mesh topology
can span any cloud without provider-specific constructs; route advertising propagates subnets into
the mesh

**Example personas:**
- "Staff SRE at a 200-person SaaS company migrating from AWS to multi-cloud who needs to keep
  production traffic off the public internet during the 6-month migration window"

---

### Segment 5 — Game Servers & LAN-Party Hosters

**Who they are:** Indie game developers or gaming communities who want a low-latency private network
for friends to play together or to run dedicated servers without exposing public ports.

**Pain points:**
- Port forwarding is blocked by ISP (CGNAT) for most players
- Existing solutions (Hamachi, Radmin VPN) are Windows-only or require accounts
- Self-hosted game servers need DDoS protection that is expensive

**Current alternatives:** Hamachi (LogMeIn), Radmin VPN, ZeroTier, Tailscale

**Why wgmesh fits:** LAN multicast discovery gives sub-second peer detection on the same network;
DHT handles the rest; no account required; WireGuard encryption is the best latency/overhead ratio
available

**Example personas:**
- "Indie game developer running dedicated servers for a multiplayer game, wanting players to connect
  via a private mesh without paying for a game-server hosting service"

---

### Segment 6 — Kubernetes / Container Networking Teams

**Who they are:** Platform teams running multi-cluster Kubernetes. They need cross-cluster
networking for service discovery, Velero backups, or shared observability stacks.

**Pain points:**
- Cluster-to-cluster networking with Cilium ClusterMesh or Linkerd requires shared CA, CRDs, and
  complex setup per pair of clusters
- Cloud-provider cluster peering is locked to one provider
- Istio multi-cluster is operationally heavy and requires expertise

**Current alternatives:** Cilium ClusterMesh, Submariner, Skupper, Linkerd multi-cluster

**Why wgmesh fits:** Subnet advertisement routes entire pod CIDRs across the mesh; works regardless
of cloud provider; doesn't require a Kubernetes operator — just a DaemonSet or node agent

**Example personas:**
- "Platform engineer managing 5 EKS clusters (prod/staging across 3 regions) who needs clusters to
  reach each other's services for integration tests"

---

### Segment 7 — Managed Service Providers (MSPs) / IT Consultancies

**Who they are:** MSPs who manage IT for 10–100 SMB clients. They need a way to remote-access
client networks without installing a VPN appliance at each site.

**Pain points:**
- Per-site VPN appliances (Meraki, Cisco ASA) cost $300–$1,000/site and require on-site config
- OpenVPN / WireGuard manual configs get out of sync when client's router IP changes
- IPSEC tunnels require expertise and are fragile across NAT

**Current alternatives:** Meraki AutoVPN, Cisco ASA, pfSense with OpenVPN, NinjaRMM VPN

**Why wgmesh fits:** Centralized SSH mode lets an MSP push configs from their control node to all
client machines; DHT handles IP change automatically; no per-site appliance required

**Example personas:**
- "MSP owner managing 30 SMB clients' Windows servers, tired of paying for VPN appliances and
  managing per-site configs"

---

### Segment 8 — Security-Conscious Startups Avoiding SaaS Lock-In

**Who they are:** Early-stage security, fintech, or healthtech startups with strict data residency
or audit requirements. They cannot use a US SaaS VPN because it would route all metadata through
a third-party SaaS.

**Pain points:**
- Tailscale, ZeroTier, and Cloudflare WARP send coordination metadata to US-hosted servers
- GDPR / HIPAA / SOC 2 auditors flag reliance on external coordination servers
- Building their own WireGuard mesh is a 2-week engineering project

**Current alternatives:** Self-hosted Headscale, Netmaker, manual WireGuard

**Why wgmesh fits:** No metadata leaves the mesh; coordination uses DHT (fully distributed) and a
GitHub Issues registry (auditable, no SaaS dependency); FOSS and auditable codebase

**Example personas:**
- "CTO of a 10-person healthtech startup building a HIPAA-compliant platform, needs internal
  service connectivity without routing metadata through third-party US servers"

---

### Segment 9 — Academic / Research Networks

**Who they are:** University labs or research institutes running distributed experiments across
multiple campuses or cloud accounts. They need persistent private connectivity but have no
dedicated networking staff.

**Pain points:**
- University IT won't provision VPNs for individual research groups
- SSH tunnels and port forwarding break when IPs change (DHCP, cloud reboot)
- Commercial mesh VPNs require purchasing licenses for each researcher

**Current alternatives:** SSH tunnels, autossh, ngrok, self-hosted WireGuard

**Why wgmesh fits:** `wgmesh join` requires no IT involvement; DHT discovery handles IP changes;
completely free and open source

**Example personas:**
- "PhD student running ML training jobs across 3 university HPC clusters and 2 cloud spot instances,
  needs all nodes to communicate without going through university IT"

---

### Segment 10 — CDN / Anycast Operators

**Who they are:** Small CDN operators or companies running their own anycast infrastructure across
multiple PoPs. They need encrypted back-channel connectivity between PoPs for cache invalidation,
health checks, and control-plane traffic.

**Pain points:**
- Cloud VPNs don't span across different hosting providers
- Manual WireGuard between N PoPs requires N*(N-1)/2 config entries that must be kept in sync
- BGP peering for private PoP-to-PoP traffic requires AS numbers and is overengineered for small
  CDNs

**Current alternatives:** Manual WireGuard, ZeroTier, private MPLS (too expensive)

**Why wgmesh fits:** Full-mesh topology is automatic; centralized mode handles fleet-wide config
pushes; route advertisement propagates PoP subnets across the control plane

**Example personas:**
- "Operator of a 12-PoP CDN running on Hetzner, OVH, and Vultr who needs encrypted back-channel
  without paying for a managed overlay"

---

### Segment 11 — Financial Services (Trading Desks / Quant Funds)

**Who they are:** Quantitative trading firms or fintech companies running co-location servers at
multiple exchanges. They need ultra-low-latency private connectivity between servers with no
third-party SaaS in the path.

**Pain points:**
- Any SaaS coordinator adds latency and is a compliance risk (FINRA, SEC data rules)
- Co-lo providers charge for private VLAN cross-connections
- Manual WireGuard setup across co-lo environments takes days and breaks on rekeys

**Current alternatives:** Private VLANs from co-lo providers, manual WireGuard, IPSEC over
dedicated circuits

**Why wgmesh fits:** Zero third-party SaaS; WireGuard's kernel-native path is the lowest-overhead
encrypted tunnel available; centralized mode handles config consistency

**Example personas:**
- "Infra lead at a 15-person quant fund with servers at Equinix NY4, LD4, and TY3 who needs
  encrypted low-latency connectivity without paying for cross-connect fees"

---

### Segment 12 — Open-Source Project Maintainers Running Multi-Region CI

**Who they are:** Maintainers of popular open-source projects with CI runners spread across
self-hosted machines, sponsor-donated VMs, and free-tier cloud accounts. They need runners to
reach internal caches (artifact stores, Docker registries).

**Pain points:**
- GitHub Actions runners on different networks can't reach private Docker registries without
  complex NAT or cloud setup
- SSH tunnels for private caches are brittle across ephemeral CI VMs
- Tailscale GitHub Actions integration requires Tailscale account and oauth key rotation

**Current alternatives:** Tailscale GitHub Actions, self-hosted VPN in CI cloud account, private
GitHub runners

**Why wgmesh fits:** A single wgmesh secret shared as a GitHub Actions secret lets any ephemeral
runner join the mesh at startup; no account rotation needed

**Example personas:**
- "Maintainer of a 5k-star open-source project running self-hosted CI on donated hardware across
  3 countries who needs runners to reach an internal Nexus cache"

---

### Segment 13 — Offshore / Nearshore Development Agencies

**Who they are:** Software agencies with engineering teams in Eastern Europe, Latin America, or
South-East Asia and clients in the US or EU. They need to give client engineers access to
staging and review environments without exposing them to the public internet.

**Pain points:**
- Setting up per-client VPN access takes hours per project
- Client firewalls often block non-standard VPN protocols
- Tailscale requires every client to create a Tailscale account

**Current alternatives:** WireGuard manual configs, OpenVPN, Cloudflare Access (for HTTP only)

**Why wgmesh fits:** No account needed; client gets one secret string, runs `wgmesh join`, and
immediately reaches the staging environment; revoke access by rotating the secret

**Example personas:**
- "CTO of a 60-person nearshore agency managing 12 simultaneous client projects, spending 3 hours/
  week on VPN provisioning for client reviews"

---

### Segment 14 — Blockchain / Web3 Infrastructure Operators

**Who they are:** Teams running Ethereum validators, IPFS pinning clusters, or layer-2 rollup
infrastructure. They run nodes globally and need a management back-channel to validators and
sequencers that avoids the public internet.

**Pain points:**
- Validator nodes must not expose unnecessary ports (slashing risk from hijacked validator keys)
- SSH access to validator nodes over the public internet is a security risk
- ZeroTier and Tailscale route coordination traffic through US servers, a compliance concern for
  some jurisdictions

**Current alternatives:** Bastion hosts with strict SSH, Tailscale, manual WireGuard

**Why wgmesh fits:** DHT-only discovery means no US SaaS in the path; WireGuard gives SSH a
private tunnel without exposing port 22 to the internet; full mesh between all validators gives
direct low-latency P2P gossip channels

**Example personas:**
- "DevOps lead at an Ethereum staking pool running 40 validators across 5 cloud providers who needs
  a management network without SaaS dependency"

---

### Segment 15 — Healthcare IT (Clinic Networks)

**Who they are:** Groups of medical clinics, dental practices, or diagnostic labs that share patient
records between locations. They need HIPAA-compliant private connectivity.

**Pain points:**
- MPLS circuits between clinics cost $500–2,000/month per location
- Site-to-site VPN appliances require on-site IT visits to configure
- SaaS VPN metadata storage outside the EU/US healthcare entity boundary violates HIPAA business
  associate rules

**Current alternatives:** MPLS, Meraki site-to-site VPN, OpenVPN

**Why wgmesh fits:** No metadata leaves the mesh; eliminates appliance costs; FOSS and auditable

**Example personas:**
- "IT director for a 5-clinic dental group paying $800/month per site for MPLS, looking to cut
  networking costs while maintaining HIPAA compliance"

---

### Segment 16 — Developer Tools / Platform-as-a-Service Startups

**Who they are:** Startups building developer tools (preview environments, ephemeral dev clusters,
CI/CD orchestrators) that need to give end-users secure access to short-lived compute environments.

**Pain points:**
- Generating and distributing per-user WireGuard configs for ephemeral environments is a 2-week
  engineering project
- Tailscale's API requires per-user account provisioning, adding 2–3 minutes to environment
  spin-up
- Each environment getting its own public IP is expensive and hits cloud quota limits

**Current alternatives:** WireGuard with custom provisioner, Tailscale API, ngrok for development

**Why wgmesh fits:** A single mesh secret per workspace → environments join the mesh automatically
at start, are destroyed and replaced without manual deprovisioning; deterministic mesh IPs make
DNS mapping trivial

**Example personas:**
- "Founder of a developer preview-environment platform (similar to Render/Railway) who needs each
  ephemeral environment to be reachable from the developer's laptop without a public IP"

---

### Segment 17 — Telecommunications Companies Running Small PoPs

**Who they are:** Small regional ISPs or telco startups running network PoPs across cities. They
need a management plane for their routers and network equipment at each PoP.

**Pain points:**
- Traditional NMS (SNMP, NETCONF) requires management VLANs that are hard to extend across
  third-party co-lo sites
- SD-WAN solutions (Cisco Viptela, Meraki) are expensive and over-engineered for small operators
- Manual WireGuard between N PoPs is error-prone as the network grows

**Current alternatives:** Manual WireGuard, Cisco SD-WAN, MPLS management plane

**Why wgmesh fits:** Route advertisement propagates PoP management CIDRs; centralized mode handles
fleet-wide config pushes from the NOC; no per-PoP cost

**Example personas:**
- "Network engineer at a 30-employee regional ISP managing 8 PoPs and needing a management
  back-channel without paying for Cisco SD-WAN licensing"

---

### Segment 18 — Penetration Testing / Red-Team Firms

**Who they are:** Security consultancies that deploy disposable attack infrastructure (C2 servers,
redirectors) during engagements and need those assets to communicate privately.

**Pain points:**
- Setting up WireGuard between ephemeral VPS instances at the start of each engagement wastes
  billable time
- Public IPs for redirectors need to rotate — manual key updates every engagement
- SaaS VPN providers (Tailscale) log connection metadata that could expose client engagements

**Current alternatives:** Manual WireGuard, tinc, OpenVPN on throwaway VPS

**Why wgmesh fits:** `wgmesh join` on a fresh VPS adds it to the engagement mesh in seconds;
rotating the secret invalidates the entire mesh at engagement end; DHT discovery handles IP
rotation without config updates

**Example personas:**
- "Lead red-team operator at a 15-person security consultancy running 3–5 simultaneous engagements,
  spending 45 minutes per engagement on VPN setup"

---

### Segment 19 — Robotics / Autonomous Vehicle Companies

**Who they are:** Companies deploying robotic fleets (warehouse robots, delivery drones, autonomous
vehicles) that need a private management network back to fleet operations.

**Pain points:**
- Cellular connectivity means dynamic IPs; static coordination servers fail when the server is
  unreachable
- Cloud VPN concentrators add 50–200 ms of latency to telemetry loops
- Manual config management for 100+ robots is not scalable

**Current alternatives:** AWS IoT Greengrass, Azure IoT Hub custom networking, manual WireGuard

**Why wgmesh fits:** DHT discovery handles dynamic IPs; deterministic mesh IPs from device identity
mean no DNS provisioning per device; route advertisement lets robots reach on-premises controllers
without a cloud relay

**Example personas:**
- "Infrastructure lead at a warehouse robotics company managing 150 robots across 3 fulfillment
  centers who needs sub-100 ms management latency without cloud relay"

---

### Segment 20 — Distributed Machine Learning / GPU Clusters

**Who they are:** ML teams or startups that rent GPU spot instances across providers (Lambda Labs,
CoreWeave, Vast.ai, RunPod) for training runs. They need all nodes to reach each other for
NCCL/all-reduce without routing through the public internet.

**Pain points:**
- Cloud-provider GPU clusters don't span providers: NCCL all-reduce over public internet has high
  latency and bandwidth costs
- WireGuard between spot instances requires re-configuration every time a spot instance is
  replaced
- VPC peering across providers isn't possible; dedicated inter-cloud links are too expensive for
  spot workloads

**Current alternatives:** Manual WireGuard, Nebula (Defined Networking), Netmaker, AWS Parallel
Cluster (AWS-only)

**Why wgmesh fits:** DHT auto-discovery handles spot instance replacement; deterministic mesh IPs
make NCCL host-file generation trivial; no cloud-provider lock-in

**Example personas:**
- "ML engineer at a 12-person AI startup training 70B-parameter models across 16 A100 nodes rented
  from 3 GPU cloud providers, needing sub-1 ms collective-communication latency"

---

### Segment 21 — Event / Conference AV Infrastructure

**Who they are:** AV production companies or conference organizers running temporary networks for
live events (video streaming, audio mixing, lighting control) across a venue.

**Pain points:**
- Venue WiFi is unreliable; laying Ethernet between stations is expensive and slow
- Manual WireGuard config at event setup takes 30–60 minutes and breaks when IPs change on venue
  network
- Renting managed switches and VLANs from the venue adds thousands to event cost

**Current alternatives:** Manual WireGuard, hamachi, ZeroTier, consumer VPN apps

**Why wgmesh fits:** LAN multicast discovery finds nodes on the same venue network in < 1 second;
`wgmesh join` requires one command per device; no account or coordinator server needed on-site

**Example personas:**
- "AV director for a 500-person tech conference needing to connect 20 A/V stations across a
  convention center without involving venue IT"

---

## Outreach Message Templates

Templates are short (< 200 words), reference one concrete pain the segment has, and propose a
specific call-to-action (CTA). Replace `[FIELD]` with actual values before sending.

### Template A — Remote-First Engineering Teams

**Subject:** Cut VPN onboarding to 30 seconds (no account needed)

> Hi [NAME],
>
> I noticed [COMPANY] is remote-first. Onboarding a new engineer or contractor to your staging
> network probably takes 20–30 minutes of VPN config today.
>
> wgmesh lets you share a single secret string with anyone — they run one command and they're on the
> mesh. No accounts, no coordinator server to maintain, WireGuard encryption throughout.
>
> We're giving free pilot access to 5 engineering teams this quarter. Would a 30-minute demo make
> sense?
>
> — [SENDER]

---

### Template B — Self-Hosted / Homelab Operators

**Subject:** WireGuard mesh without a coordinator server — free & open source

> Hi [NAME],
>
> I've seen your posts about [TOPIC] on r/homelab / [FORUM]. Thought wgmesh might save you the pain
> of keeping manual WireGuard configs in sync when your home IP changes.
>
> Generate a secret, run `wgmesh join` on each machine — DHT discovery handles the rest, even
> through CGNAT.
>
> Happy to share a quick walkthrough if useful.
>
> — [SENDER]

---

### Template C — IoT / Edge Deployments

**Subject:** Private back-channel for [N] edge devices without cloud VPN costs

> Hi [NAME],
>
> Teams deploying hardware at [N]+ sites often hit a wall with cloud VPN concentrator costs once
> device count grows past ~50. We built wgmesh to solve exactly this: DHT-based discovery means each
> device finds its peers through NAT without a central coordinator — no per-connection cloud charges.
>
> We've had early interest from IoT teams at [REFERENCE COMPANY TYPE]. Would a 20-minute technical
> call be worth your time?
>
> — [SENDER]

---

### Template D — Multi-Cloud Infrastructure

**Subject:** Cross-cloud mesh without Transit Gateway or VPC peering

> Hi [NAME],
>
> Running workloads across [AWS/GCP/Azure] and [PROVIDER 2]? Connecting them privately usually means
> cloud transit fees or a VPN appliance in each account.
>
> wgmesh builds a WireGuard mesh across any hosts — VMs, bare metal, containers — in minutes, with
> route advertisement so each side sees the other's subnets. No cloud-provider lock-in.
>
> We're running a free 30-day pilot for infrastructure teams. Interested?
>
> — [SENDER]

---

### Template E — Security / Compliance Focused

**Subject:** Mesh VPN with no US SaaS coordinator — GDPR / HIPAA friendly

> Hi [NAME],
>
> Many teams with GDPR or HIPAA requirements can't use Tailscale or ZeroTier because coordination
> metadata passes through US SaaS servers. wgmesh uses BitTorrent DHT for discovery — fully
> distributed, no third-party SaaS in the path, fully auditable FOSS codebase.
>
> Happy to share the architecture overview and our approach to key derivation if helpful for your
> compliance review.
>
> — [SENDER]

---

### Template F — MSPs / IT Consultancies

**Subject:** Manage 30 client networks from one control node — no appliances

> Hi [NAME],
>
> MSPs managing WireGuard for multiple clients typically spend 30–60 minutes per new site on key
> distribution and config sync. wgmesh's centralized mode pushes configs to all nodes via SSH from
> a single state file — diff-based, so no interface restarts.
>
> Would replacing per-site VPN appliances with a $0/month open-source alternative be worth a
> 20-minute call?
>
> — [SENDER]

---

### Template G — Kubernetes / Platform Engineering

**Subject:** Cross-cluster networking in 5 minutes — no CRDs, no service mesh

> Hi [NAME],
>
> Setting up Cilium ClusterMesh or Submariner across [N] clusters takes days. wgmesh takes 5
> minutes: one `wgmesh join` per node with `--advertise-routes <pod-CIDR>` and all clusters see
> each other's pods.
>
> No Kubernetes operator, no CRDs, no shared CA. Just WireGuard.
>
> Happy to walk through a 2-cluster demo — does that make sense?
>
> — [SENDER]

---

## Pilot / Trial Qualification Criteria

A prospect is qualified for a pilot when they meet ALL of the following:

| Criterion | Minimum Threshold |
|-----------|-------------------|
| Node count | ≥ 3 nodes to connect (trivial cases don't demonstrate value) |
| Current pain | Actively spending time or money on current VPN/mesh solution (≥ 30 min/week or ≥ $50/month) |
| Technical contact | At least one engineer who can run `sudo` on the target machines |
| Network diversity | Nodes span ≥ 2 networks, at least one behind NAT (tests DHT/hole-punching) |
| Timeline | Willing to complete setup within 2 weeks and provide feedback within 4 weeks |
| Communication | Agrees to a 30-minute post-pilot debrief call |

**Disqualifying factors** (do not advance to pilot):
- Nodes are all on the same flat LAN with no NAT (wgmesh provides no incremental value over
  standard WireGuard in this case)
- Requires Windows-only deployment (wgmesh currently supports Linux and macOS)
- Requires a GUI or zero-CLI interaction (wgmesh is CLI-first)
- Regulated environment where the pilot approval process > 60 days (reschedule to a future quarter)

---

## Case Study Questions for Early Adopters

Use these questions in a 30–45-minute debrief call after the pilot. The goal is to gather a
publishable case study and identify product gaps.

### Section 1: Context (5 minutes)

1. How would you describe your infrastructure in one sentence? (number of nodes, providers, purpose)
2. What were you using before wgmesh, and what triggered you to look for an alternative?
3. Who on your team was involved in evaluating and deploying wgmesh?

### Section 2: Setup Experience (10 minutes)

4. How long did it take from reading the README to having a running mesh?
5. Which installation method did you use, and were there any friction points?
6. Did you hit any errors during setup? If so, what were they and how did you resolve them?
7. Did the documentation cover your use case adequately, or did you have to piece things together?

### Section 3: Operational Value (10 minutes)

8. Which specific pain point has wgmesh eliminated or significantly reduced?
9. Can you quantify the time or money saved compared to your previous solution?
10. Have there been any production incidents involving the mesh? How were they resolved?
11. Which features do you use most frequently? (DHT discovery, LAN multicast, route advertisement,
    centralized mode, metrics)

### Section 4: Gaps and Improvements (10 minutes)

12. What's the one thing that would make you confident recommending wgmesh to a colleague?
13. Is there a feature or capability you expected to find but didn't?
14. Are there any operational workflows (monitoring, alerting, key rotation) where wgmesh doesn't
    fit yet?
15. What would have to be true for you to pay for a supported/hosted version?

### Section 5: Reference and Promotion (5 minutes)

16. Would you be willing to be named as a reference customer (case study, logo on website)?
17. Are there 2–3 colleagues in similar roles you'd be willing to introduce us to?
18. Would you leave a review on a platform like Product Hunt, G2, or Hacker News?

---

## Prioritization Recommendations

Based on alignment with wgmesh's current capabilities and likely reachability, the following
segments should be engaged first:

| Priority | Segment | Rationale |
|----------|---------|-----------|
| 1 | Remote-First Engineering Teams | Largest addressable pool; Tailscale fatigue is growing; easy to find via LinkedIn / HN |
| 2 | Self-Hosted / Homelab | Active community on r/homelab, r/selfhosted; low barrier to trial; word-of-mouth flywheel |
| 3 | IoT / Edge Deployments | Strong ROI story (no per-connection cloud cost); DHT advantage is unique |
| 4 | Security / Compliance Focused | Tailscale GDPR concern is a real, documented objection; clear differentiation |
| 5 | Multi-Cloud Infrastructure | Higher deal size; requires more sales effort; better for quarter 2+ |
````

## Affected Files

- **New:** `docs/customer-research.md` — full target customer research document with 21 segments,
  outreach templates, pilot qualification criteria, and case study questions

No code files are changed. No Go packages are touched. No new dependencies.

## Test Strategy

No automated tests required for documentation. Verify manually:

1. `docs/customer-research.md` renders without broken Markdown in a GitHub preview (all tables
   aligned, all code fences closed).
2. The document contains ≥ 20 named customer segments (acceptance criteria: 20+ specific
   companies/personas).
3. Each segment has a "Pain points", "Current alternatives", and "Why wgmesh fits" sub-section.
4. At least one outreach template per major segment type (A–G covers all 21 segments).
5. The pilot qualification table is present with ≥ 5 qualification criteria and ≥ 3 disqualifying
   factors.
6. The case study questions section contains ≥ 15 numbered questions grouped into labelled sections.

## Estimated Complexity
low

**Reasoning:** Pure documentation. One new Markdown file (~350 lines). No code changes, no
dependency updates, no build pipeline changes. Estimated effort: 45–60 minutes.
