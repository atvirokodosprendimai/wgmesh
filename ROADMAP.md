# Roadmap

Three horizons. Each item has a one-line rationale because in six months we'll forget why it was on the list.

The roadmap is organized two ways:

- **By horizon** — Now / Next / Later, with rough quarterly buckets.
- **By bet** — what we believe will produce the first paying customer, what we believe will produce the tenth, what we believe will produce the hundredth.

The two views overlap intentionally. If they diverge, that's the bug — fix the one that's wrong.

---

## Horizon 1 — Now (Q2 2026, next ~8–12 weeks)

The theme is **close the gap to the first paying customer**. Most of these are not engineering; the engineering items are the ones that block the commercial items.

| # | Item | Owner | Why now |
|---|---|---|---|
| 1 | Polar.sh billing wired up (#376) | needs-human | GitHub Sponsors has too many steps. Conversion blocker per first-customer brainstorm. |
| 2 | Cost tracking populated (#373) | needs-human | Can't compute runway with €2400 capital and `costs.json` full of nulls. |
| 3 | Landing page repositioning (cloudroof.eu) | needs-human | Currently opaque to non-engineers. ICP needs to recognize themselves above the fold. |
| 4 | Show HN post + stargazer outreach | Marty | Distribution. The autonomous pipeline is the story; ship it before the hype window closes. |
| 5 | "Homelab to VPS in 5 minutes" tutorial | Marty | SEO-friendly, shareable, converts the drive-by visitor. |
| 6 | `wgmesh service add` CLI spec (#372) | dev pipeline | Foundation-stage blocker called out in the assessments. |
| 7 | Hostname propagation in peer announcements (#81 phase 3, #181) | dev pipeline | `peers list` shows truncated pubkeys instead of hostnames. Embarrassing during demos. |
| 8 | Truncated pubkey copy-paste fix (#178) | dev pipeline | You can't `peers get` what you can't paste. |
| 9 | SSH host key verification | dev pipeline | `InsecureIgnoreHostKey` is the only thing preventing a credible "we take security seriously" claim. |
| 10 | Secret rotation protocol | dev pipeline | Promised in archived bootstrap plan, still not built. Single biggest "what if my secret leaks" gap. |
| 11 | Nix package (`memory/plan - 2603012134`) | dev pipeline | `nix run github:atvirokodosprendimai/wgmesh` is the 30-second try-it path. |

**Stage exit criterion:** one paying customer, even at $5/mo, and `costs.json` populated enough that the autonomous loop can compute runway.

---

## Horizon 2 — Next (Q3 2026)

The theme is **make the product feel real to a buyer who isn't already convinced**. After Q2, we should have a few users; Q3 is about giving the next ten of them what's currently missing.

| # | Item | Why |
|---|---|---|
| 1 | Decentralized ACLs | Centralized has groups + policies. Decentralized doesn't. Closes the parity gap and unblocks "real" deployments. |
| 2 | STUN integration (`pion/stun`) | Cleaner endpoint detection than the current SSH-host-comparison heuristic. |
| 3 | TURN / relay fallback | For nodes behind strict CGNAT. Currently they just fail; EasyTier handles this and we don't. |
| 4 | Web dashboard (mesh management) | Read-only is fine for now. Lives on Chimney. The first thing a buyer asks for after CLI. |
| 5 | DNS / mesh-internal name resolution | `ssh node1` instead of `ssh 10.99.0.5`. Tailscale's MagicDNS is the feature people miss most. |
| 6 | IPv6 mesh subnet | Currently /16 IPv4 only. Some prospects will literally not consider us without v6. |
| 7 | Lighthouse CDN: managed ingress productized | The thing the $20 Edge Node tier is selling. Currently the code exists; the offering doesn't. |
| 8 | Self-serve signup | The path from "I read the Show HN" to "I have a working mesh" without DMing the founder. |
| 9 | `wgmesh install-service` | Phase 4 of the original plan. systemd unit generation for decentralized mode. |
| 10 | Mistral evaluation for control loop | EU-native LLM as a hedge against OpenRouter availability and as a positioning consistency check. |
| 11 | Multi-region edge proxies | Required for the CDN narrative to hold. Until we have nodes in 3+ regions, "anycast CDN" is a sketch. |

**Stage exit criterion:** ~10 paying customers, the dashboard at chimney.beerpub.dev shows them by name, and the Lighthouse-managed-ingress is something a stranger can sign up for without our help.

---

## Horizon 3 — Later (Q4 2026 – Q1 2027)

The theme is **become legible to a buyer who wasn't a homelabber to start with**. This is where the "no coordination server" architecture either continues to be the moat or becomes the thing we have to apologize for.

| # | Item | Why |
|---|---|---|
| 1 | Mobile clients (iOS/Android) | The killer feature road warriors won't compromise on. wireguard-go-based, with the daemon's discovery layer ported. Hard but unavoidable. |
| 2 | SSO / OIDC for centralized mode | Makes the product legible to enterprise IT. Without it we are not in the conversation. |
| 3 | Audit logging | Not for the homelabber. For the buyer who has to write a SOC 2 control. |
| 4 | Compliance posture (GDPR doc, SOC 2 readiness) | The EU positioning is a real edge but only if we can produce the artifacts on request. |
| 5 | Plugin / integration ecosystem | Terraform provider, Coolify integration, GitHub Action. Each one is a distribution channel. |
| 6 | Automated customer health scoring | Per first-customer spec. When you have 100 customers you can no longer remember who is healthy. |
| 7 | Web dashboard: full read-write | Add/remove nodes, manage groups, edit policies. The thing that lets a non-CLI person operate a mesh. |
| 8 | Public floodfill / garlic bundling | Privacy features deferred from the bootstrap plan. Only matters if we pick up an adversarial-network use case. |

**Stage exit criterion:** a coherent commercial offering that a buyer can evaluate in <30 minutes without talking to us, and the autonomous loop is responsible for routine support and upsell.

---

## By Bet

Same items, regrouped by what we think they'll produce. The horizons answer "when"; the bets answer "why."

### Bet A — First paying customer (next 12 weeks)
Polar.sh, cost tracking, landing page rewrite, Show HN, stargazer outreach, the 5-minute tutorial.
*Belief: the technical product is good enough; the path to revenue is reducing friction and finding the first human, not building more features.*

### Bet B — Tenth paying customer (Q3)
Decentralized ACLs, STUN/TURN, DNS, Lighthouse-managed-ingress, self-serve signup, IPv6.
*Belief: the next nine customers each want a thing the first customer didn't ask about. The job is to build the smallest list that covers the most of them.*

### Bet C — Hundredth paying customer (Q4–Q1)
Mobile, SSO, audit logs, GDPR/SOC 2 posture, plugin ecosystem, full-write dashboard.
*Belief: somewhere around customer 30 we stop selling to homelabbers and start selling to small companies. They want different things. We should know which group we're talking to.*

### Bet D — The thing that probably won't work but should be tried anyway
Indie integrations: Coolify, CapRover, Coolify-style PaaS networking. The "AI inference mesh" angle. Terraform provider. Each of these is a 1-in-5 chance of being the unlock.
*Belief: most of these will be ignored. One might compound into the actual distribution channel. We won't know which until we try.*

---

## Things deliberately NOT on this roadmap

Worth saying out loud so they don't keep showing up in standups.

- **Hosted control plane for someone else's wgmesh deployment.** We are not Headscale-as-a-service. cloudroof.eu uses wgmesh internally as part of a bundled edge offering; we don't sell a control plane for your mesh.
- **Windows native client.** Linux + Docker covers the homelab market. Windows is mostly a way to lose two months and ship something worse than what's already in the box.
- **Custom WireGuard fork.** Stay on upstream. Every fork I've seen of WG has regretted it within 18 months.
- **Free tier with "limit removed in paid."** Either it's free or it's paid; the in-between makes the product feel cheap to evaluators.
- **Proprietary protocol on top of WG.** Same reason as no fork. The WG protocol is the contract; layering proprietary stuff on top breaks interop, which is half the value.

---

## How this roadmap is maintained

- **Source of truth is this file**, in the repo, on `main`. If something here disagrees with a slide, the slide is wrong.
- **Each item links to a GitHub issue** once it's actively being worked. If an item is on this roadmap but has no issue and no owner after 30 days, it's not actually on the roadmap; delete it.
- **Reviewed monthly** by checking the `memory/next - YYMMHHHH - aggregated actionable items` file against what's here. Drift is interesting; resolve it.
- **The autonomous company loop** can propose additions but cannot silently delete items. Deletion requires a human (Marty) — these are decisions, not optimizations.
