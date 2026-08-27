# Outreach Templates for DevOps Communities

This document provides templates for outreach to Discord and Slack communities.

## Template Guidelines

### Core Principles

1. **Lead with value, not promotion**
   - Share knowledge first
   - Contextualize wgmesh/cloudroof as solution to real problems
   - Avoid generic "check out my product" posts

2. **Customize for each community**
   - Use community-specific terminology
   - Reference community discussions or pain points
   - Align with community interests and priorities

3. **Be transparent**
   - Clearly identify as wgmesh/cloudroof team member
   - Disclose trial offer nature
   - Make affiliation clear

4. **Keep it concise**
   - Respect community attention spans
   - Provide clear call-to-action
   - Include links for more details

---

## Template 1: Kubernetes & Cloud Native Communities

### Context

**Target**: Kubernetes users, cloud-native engineers, platform teams

**Pain Points**:
- Multi-cluster networking complexity
- Service mesh control plane overhead
- Cross-cluster service discovery
- Pod-to-pod connectivity across clouds

### Initial Post Template

```markdown
Subject: Multi-cluster mesh networking without the control plane headache

Hi everyone 👋

I've been working on wgmesh, a decentralized WireGuard mesh network tool
that's particularly useful for multi-cluster Kubernetes setups.

The problem we're solving:
- Connecting pods/services across clusters without a service mesh control plane
- NAT traversal for edge nodes and on-prem clusters
- Simple mesh networking that "just works" without coordination servers

It's open source (Go, MIT-licensed) and we have a managed ingress service
called cloudroof for easier setup.

We're running a 30-day trial for K8s communities:
- Promo code: K8S{CODE}
- 10 node limit (suitable for multi-cluster testing)
- Full feature access

If this is useful for anyone, I'm happy to answer questions or help with setup.

Docs: https://wgmesh.dev
GitHub: https://github.com/atvirokodosprendimai/wgmesh
```

### Follow-up Template (when someone asks questions)

```markdown
Great question! Here's how wgmesh handles [specific topic]:

[Detailed technical answer about their question]

For your specific use case, you'd want to:
1. [Step 1]
2. [Step 2]

If you'd like to try it out, you can use the trial code: K8S{CODE}

Happy to do a quick screenshare if you want to see it in action.
```

---

## Template 2: Homelab & Self-Hosted Communities

### Context

**Target**: Homelab enthusiasts, self-hosting advocates, homelab reddit

**Pain Points**:
- Connecting homelab to cloud VPS
- NAT traversal for home networks
- Dynamic DNS and public IP problems
- Secure remote access without exposing ports

### Initial Post Template

```markdown
Subject: Homelab to VPS in 5 minutes with mesh networking

Hey fellow homelabbers! 🏠

I built a tool that might be useful for folks connecting their homelab
to cloud VPSes or remote servers.

wgmesh is a WireGuard mesh builder that handles:
- NAT traversal (CGNAT-friendly)
- DHT-based peer discovery
- No coordination server or public ports needed
- Persistent mesh IPs regardless of real IP changes

Typical use case:
- Your homelab gets a mesh IP (e.g., 10.99.0.1)
- Your Hetzner VPS gets a mesh IP (e.g., 10.99.0.2)
- They connect directly, no port forwarding needed

We have a 30-day trial for homelab communities:
- Promo code: LAB{CODE}
- 5 node limit (perfect for homelab + few VPSes)

If anyone wants to try it, I'm around to help with setup.

GitHub: https://github.com/atvirokodosprendimai/wgmesh
Demo: [Link to quickstart video or docs]
```

### Follow-up Template

```markdown
@[username] - great question about [topic]!

[Technical answer with homelab-specific examples]

For your setup (looks like you have [hardware/ISP]), you'd want to:
1. Generate a secret: wgmesh init --secret
2. Run on homelab: wgmesh join --secret "[SECRET]"
3. Run on VPS: wgmesh join --secret "[SECRET]"

The trial code LAB{CODE} gives you 30 days to test it out.

Let me know if you run into any issues - happy to troubleshoot!
```

---

## Template 3: DevOps & SRE Communities

### Context

**Target**: DevOps engineers, SREs, infrastructure teams

**Pain Points**:
- Emergency access paths
- Backup connectivity
- Ad-hoc fleet networking
- VPN server maintenance

### Initial Post Template

```markdown
Subject: Ad-hoc mesh networking for DevOps/SRE workflows

Hi all 👋

I wanted to share wgmesh, a tool we've built for ad-hoc mesh networking
in DevOps contexts. It's been useful for:

- Emergency access paths when primary VPN fails
- Backup connectivity across regions
- Ad-hoc fleet networking without VPN server overhead
- Temporary test environments

Key features:
- Decentralized (no coordination server to host)
- WireGuard encryption
- DHT peer discovery
- NAT traversal built-in

We're offering trials for DevOps teams:
- 45-day trial (OPS{CODE})
- 10 node limit
- CLI interface + managed ingress option

If this solves a problem you're facing, I'm happy to answer questions.

GitHub: https://github.com/atvirokodosprendimai/wgmesh
Docs: https://wgmesh.dev
```

---

## Template 4: Networking & Security Communities

### Context

**Target**: Network engineers, security professionals, cryptography enthusiasts

**Pain Points**:
- Trust boundaries in cloud-hosted coordination servers
- Encryption and authentication
- DHT and P2P networking
- Network topology and routing

### Initial Post Template

```markdown
Subject: Decentralized mesh networking with WireGuard + DHT

Hi network/security folks 👋

I've been working on wgmesh, a decentralized WireGuard mesh that uses
DHT for peer discovery. Crypto stack is:

- HKDF-SHA256 for key derivation from shared secret
- AES-256-GCM for encrypted peer exchange
- WireGuard for data plane
- DHT for NAT traversal and endpoint detection

Architecture decisions:
- No coordination server (trust boundary consideration)
- DHT-based discovery (BitTorrent Mainline DHT)
- Dandelion++ relay for announcement privacy

We have trials available:
- 30-day trial
- Promo code: NET{CODE}
- Open source + managed ingress option

If anyone's interested in the crypto architecture or DHT implementation,
I'm happy to dive into details.

GitHub: https://github.com/atvirokodosprendimai/wgmesh
Crypto docs: https://wgmesh.dev/encryption.html
```

---

## Template 5: General DevOps Tooling Communities

### Context

**Target**: General DevOps, infrastructure as code, automation

**Pain Points**:
- Tool complexity
- Configuration management
- Automation-friendly interfaces
- Integration with existing stacks

### Initial Post Template

```markdown
Subject: CLI-first mesh networking tool

Hi everyone 👋

I wanted to share wgmesh, a CLI tool for building WireGuard mesh networks.
It's designed to be:

- Simple: One shared secret, then `wgmesh join` on each node
- Decentralized: No coordination server or control plane
- Automation-friendly: CLI interface, JSON status output

Use cases:
- Multi-region fleet networking
- Backup connectivity paths
- Test environment isolation
- Development-to-production access

Trial available: 30 days, 5 nodes
Promo code: DEV{CODE}

If anyone's interested in mesh networking or has questions about WireGuard,
I'm around to help.

GitHub: https://github.com/atvirokodosprendimai/wgmesh
Quickstart: https://wgmesh.dev/quickstart.html
```

---

## Response Templates for Common Questions

### "How is this different from Tailscale?"

```markdown
Great question! Key differences from Tailscale:

Architecture:
- Tailscale: Coordination server required (derp servers, control plane)
- wgmesh: Fully decentralized (DHT discovery, no servers)

Trust model:
- Tailscale: Trust Tailscale infrastructure
- wgmesh: Only trust your own secret (HKDF-derived keys)

Use case alignment:
- Tailscale: Great for zero-config personal VPNs
- wgmesh: Better for autonomous fleet operations and self-hosted setups

Trade-offs:
- Tailscale: Easier setup, more features
- wgmesh: More control, no external dependencies

Both use WireGuard under the hood. Choose based on your trust/autonomy needs.
```

### "How is this different from NetBird?"

```markdown
Comparison with NetBird:

NetBird approach:
- Managed WireGuard with coordination server
- Commercial open source (team/business pricing)

wgmesh approach:
- Decentralized (DHT discovery)
- MIT-licensed, fully open source
- No coordination server required

Similar:
- Both use WireGuard
- Both target mesh networking

Different:
- NetBird: Requires management server
- wgmesh: Serverless, DHT-based

If you're okay with a management server, NetBird is great.
If you need full autonomy, wgmesh might be a better fit.
```

### "What about Cloudflare Mesh / Tunnel?"

```markdown
Cloudflare Mesh/Tunnel comparison:

Cloudflare approach:
- All-edge architecture (no P2P)
- Requires Cloudflare infrastructure
- Workers-based deployment

wgmesh approach:
- P2P mesh (direct node-to-node)
- No infrastructure required
- Runs anywhere you can run Go

Cloudflare Mesh gaps (from their blog):
- No agent-level identity-aware routing
- No self-hosted/GDPR-strict path
- Limited framework integrations

wgmesh fills these gaps:
- Per-agent identity (HKDF-derived keys)
- Fully self-hostable
- Framework integrations (upcoming)

If you're already in Cloudflare ecosystem, Tunnel is great.
If you need autonomy and P2P, wgmesh is worth considering.
```

### "Is this production-ready?"

```markdown
Production readiness status:

✅ Stable features:
- WireGuard mesh networking
- DHT discovery
- NAT traversal
- Encryption (HKDF, AES-256-GCM)
- CLI interface

✅ Dogfooding:
- We use it for our own infrastructure
- Stability log: https://wgmesh.dev/dogfooding/stability-log.html

🔄 Active development:
- Managed ingress (cloudroof) in beta
- Framework integrations (upcoming)
- More documentation and examples

⚠️ Known limitations:
- No web UI yet (CLI-only)
- No Windows support (Linux/macOS only)
- DHT bootstrap can be slow initially

For production use:
- Test thoroughly in your environment
- Have backup connectivity
- Join our Discord for support

Trial is perfect for production testing!
```

### "What happens when my trial ends?"

```markdown
When your trial ends:

Options:
1. Self-host wgmesh (open source, free forever)
   - Full feature set
   - No cost
   - You manage infrastructure

2. Subscribe to cloudroof managed ingress
   - We handle setup and monitoring
   - Priority support
   - Pricing: [Link to pricing]

3. Request trial extension
   - Just reply to this message
   - We'll extend based on your use case

4. Use the CLI tool only
   - wgmesh CLI is free and open source
   - No trial needed
   - Manual setup

No data loss:
- Your mesh configuration stays intact
- Export your config before trial ends
- Self-hosting uses same config format

Need help deciding? Just let me know your use case!
```

---

## Direct Message Templates

### Initial DM (after helpful interaction)

```markdown
Hey [Name],

Thanks for your help with [topic we discussed].

I mentioned I'm working on wgmesh (mesh networking tool) - we have
a trial running if you ever want to try it out:

Promo code: [CODE]
Trial length: [DAYS] days
Node limit: [N] nodes

No pressure at all - just wanted to offer it since you seem to know
your way around networking infrastructure.

Cheers!
[Your Name]
```

### Follow-up DM (if they express interest)

```markdown
Hey [Name],

Great! Here's how to get started with the trial:

1. Generate a secret:
   wgmesh init --secret

2. Join your first node:
   wgmesh join --secret "[SECRET]"

3. Join your second node (this activates your trial):
   wgmesh join --secret "[SECRET]"

Your trial code: [CODE]

Docs: https://wgmesh.dev/quickstart.html

If you run into any issues, just ping me - I'm happy to help troubleshoot.
```

---

## Anti-Patterns to Avoid

### ❌ Don't do this

```markdown
Subject: Check out my new product! 🚀

Hey everyone! I built this cool thing called wgmesh.
It's amazing and you should all try it.

Use code DISCOUNT2026 for 50% off!

Link: [Link]
```

**Why this fails**:
- No value provided
- Generic promotion
- No context for the community
- "Salesy" tone

### ❌ Don't do this either

```markdown
Subject: wgmesh - better than Tailscale!

Tailscale sucks. Use wgmesh instead. It's decentralized
and open source.

Link: [Link]
```

**Why this fails**:
- Negative comparison (bad community etiquette)
- No value explanation
- Alienates potential users who like Tailscale

---

## Post-Outreach Follow-up

### Check-in Template (1 week after initial post)

```markdown
Hey all 👋

I posted last week about wgmesh (mesh networking tool).
Just wanted to check in:

- Did anyone try it out?
- Any questions or feedback?
- Any use cases I didn't cover?

If you're interested but haven't tried it yet, I'm happy to do a
quick demo or help with setup.

Also, I'm working on [specific feature based on community interest] -
let me know if that would be useful for you.

Cheers!
```

---

## Community-Specific Adaptations

### Adaptation Template

Before posting in a new community, review:

```markdown
Community: [Name]
Platform: [Discord/Slack]
Primary topics: [List]
Member count: [X]
Self-promotion rules: [Summary]

Adaptations needed:
1. Terminology: [Terms to use/avoid]
2. Pain points: [Community-specific issues]
3. Examples: [Use cases relevant to this community]
4. Tone: [Formal/casual/technical]

Modified template:
[Paste adapted template here]
```

---

## Analytics Tracking

### Track Community Engagement

For each community post, track:

```yaml
Community: [Name]
Platform: [Discord/Slack]
PostedDate: [Date]
InitialPost: [Link/screenshot]
ResponseType: [Questions/Reactions/None]
QuestionCount: [X]
DemoRequests: [X]
TrialSignups: [X]
ActivatedTrials: [X]
Conversions: [X]

Notes:
- [What worked]
- [What didn't work]
- [Community-specific feedback]
```

---

## Status

- [X] Initial post templates created (5 variations)
- [X] Follow-up templates created
- [X] Response templates for common questions
- [X] DM templates created
- [X] Anti-patterns documented
- [ ] Community-specific adaptations filled in
- [ ] Post-outreach tracking implemented

Last updated: [Date]
