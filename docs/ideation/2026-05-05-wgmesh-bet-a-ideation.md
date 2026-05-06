---
date: 2026-05-05
topic: wgmesh-bet-a
focus: Concrete moves to close Bet A (first paying customer in 8-12 weeks) given autonomy / no-control-plane approach; edge-as-first-class load-bearing
mode: repo-grounded
---

# Ideation: wgmesh Bet A — first paying customer in 8-12 weeks

## Grounding Context

### Codebase Context

wgmesh = Go decentralized WireGuard mesh. Two modes — centralized (SSH config push) and decentralized (DHT discovery, no coordination server, HKDF-derived keys from shared secret). Discovery layers L0-L3 (GitHub registry, LAN multicast, BitTorrent DHT, in-mesh gossip). Encrypted gossip envelopes. RPC over Unix socket.

Strategy (`STRATEGY.md`, written 2026-05-04):

- **Target problem:** operators wiring agent fleets to edge nodes need fast, ad-hoc, secure meshes; existing tools either treat edge as second-class (Tailscale), feel bolted-on (Cloudflare Tunnel), or "work" in demos and fail at minute-3.
- **Approach:** wgmesh is autonomous — discovers, peers, heals itself with no control plane to host or trust.
- **Persona:** LLM agent builders running stacks like Hermes / openclaw — devs not devops.
- **Tracks:** mesh autonomy core / edge as first-class / commercial pipe.
- **Bet A:** first paying customer in 8-12 weeks via Polar.sh, cloudroof.eu landing repositioning, Show HN, "Homelab to VPS in 5 minutes" tutorial, cost tracking populate.

Existing internal GTM corpus (rich):

- `eidos/spec - first-customer - roadmap to first paying customer.md`
- `memory/brainstorm - 2602240805 - first paying customer for wgmesh.md`
- `memory/brainstorm - 2603012302 - what do we need for a first customer.md`
- `memory/outreach - 2602240805 - show hn post draft.md`
- `memory/outreach - 2602240805 - stargazer dm template.md`
- `eidos/spec - lighthouse - cdn control plane ...md`
- `eidos/spec - service cli - register local services for managed ingress via lighthouse.md`
- `memory/decision - 2603151026 - decouple lighthouse from wgmesh into separate repo.md`
- `docs/dogfooding/stability-log.md`

### Past learnings (`docs/solutions/`)

- **NAT punch self-connection** — same-NAT/CGNAT peers share public IP; must filter by WGPubKey not IP. Same-NAT colocated peer pair is the most likely first-customer environment shape; 40-HELLO punch loop ends a trial in <5 minutes.
- **Custom subnet silent fallback** — Goose-pipeline meta-lesson: diff plan vs `git diff --stat` because `package main` callsites get missed.
- **Tier 3 chaos test SSH hang** — atomic-undo principle for ops scripts; SSH `ServerAliveInterval=15`.
- **GoReleaser dual-tag** — clean rc tags before final tag; day-1 install footgun.

### External context (web research, 2026-05-04)

**Direct competitor: Cloudflare Mesh launched 2026-04-14** (20 days before STRATEGY locked in)

- All-edge architecture (no P2P)
- Free tier: 50 nodes / 50 users
- Stated gaps that ARE wgmesh's opportunity surface:
  1. Cloudflare-lock — agents must run on Workers
  2. No DHT / P2P
  3. Agent identity unresolved
  4. No self-hosted / GDPR-strict path
  5. No major agent framework (CrewAI, LangGraph, AutoGen) committed yet

**Closest commercial analog: NetBird** — WireGuard OSS + managed; pricing Free → Team $5/user → Business $10/user → Enterprise. 10-agent fleet = $50/mo on NetBird — wgmesh could undercut via no-coord-server model for ephemeral fleets.

**Adjacent OSS revenue patterns** — Plausible (EU-first, paid-only-day-one, $400→$22K MRR via HN); n8n (fair-code, $40M ARR; 55% cloud / 30% enterprise license / 15% embedded); Polar.sh (Apache 2.0, monetize via hosted MoR, 17K devs, OSS-as-distribution + hosted-as-monetization + community contributors as evangelists).

**Adjacent agent infra** — LM Studio LM Link uses Tailscale for local LLM mesh (direct evidence); Fly.io private networking is Fly-only; Modal is Python-ML-specific. Service mesh 2026 comeback (Istio Ambient Mode); Microsoft Agent Governance Toolkit ships Helm charts for `agent-mesh` primitive.

## Ranked Ideas

### 1. Show HN Demo Combined — `wgmesh up` + `wgmesh expose`

**Description:** Build a single 90-second paste-able demo: `wgmesh up --provider hetzner --nodes 3 --region nbg1 && wgmesh expose :11434 --as ollama`. Cloud-init pre-baked one-shot provisioning hits Hetzner API → 3 EU nodes joined to mesh → `expose` returns `https://ollama-<short>.<mesh>.cloudroof.eu`. Wraps existing decentralized mode + Lighthouse ingress + Hetzner provider plumbing into a screenshare-able artifact.

**Warrant:** `direct:` `eidos/spec - first-customer ...md` lines 197-212 already names the contract `wgmesh service add ollama :11434 → https://<service>.<mesh>.wgmesh.dev`. STRATEGY's edge-as-first-class track is exactly this surface. Existing brainstorm corpus calls "Connect your homelab to your VPS in 5 minutes" the converting content but the command doesn't exist yet.

**Rationale:** Strategy says "agent-to-edge in minutes." Today there's no command. Without it, every Show HN visitor lands on a pure mesh tool that they have to glue to Caddy themselves. With it, the demo IS a paste-able bash block.

**Downsides:** Hetzner provider plumbing (~1 week), `wgmesh service add` CLI per #372, Lighthouse managed-ingress productized, ACME-on-mesh. Touches three open subprojects. `wgmesh down` becomes destructive — needs hardening.

**Confidence:** 85%
**Complexity:** Medium
**Status:** Explored

### 2. `wgmesh agent-id` — per-agent crypto identity primitive

**Description:** First-class identity for individual agents. Each agent gets a derived keypair (HKDF from mesh secret + agent-name), an in-mesh DID, wire-level mTLS so policy can say "agent A may call agent B but not C." `wgmesh agent add summarizer-v3 --can-call db-reader`.

**Warrant:** `external:` Cloudflare Mesh blog (April 14 2026) explicitly lists "agent-level identity-aware routing" as unresolved gap. Microsoft Agent Governance Toolkit ships `agent-mesh` Helm charts. `direct:` HKDF-derived keys infra (`pkg/crypto/derive.go`) already has substrate.

**Rationale:** Single feature that lets us answer "why not Cloudflare Mesh" with one sentence. Substrate already exists. CF would have to re-architect.

**Downsides:** Touches `pkg/crypto/derive.go` (CLAUDE.md flags as "do not modify without review"). Spec-track work — needs to align with MCP/A2A still landing.

**Confidence:** 75%
**Complexity:** High
**Status:** Unexplored

### 3. Framework upstream PR — Hermes / openclaw + 1 of CrewAI/AutoGen/LangGraph

**Description:** Land contrib PRs adding wgmesh as default private transport. ~150 LOC + meshtest example per framework. Ships before CF builds its own integrations.

**Warrant:** `external:` "no major agent framework committed yet to CF Mesh"; LM Studio uses Tailscale precedent. `direct:` Hermes/openclaw IS the persona per STRATEGY.

**Rationale:** Distribution-by-embed compounds — every framework user becomes wgmesh user without per-customer outreach. Each merged PR is a permanent channel.

**Downsides:** Maintainer politics; rejection risk; may need to maintain shim packages permanently.

**Confidence:** 70%
**Complexity:** Medium
**Status:** Unexplored

### 4. Reference architecture stack — `wgmesh-stack-template`

**Description:** Single template repo. `make deploy` provisions Hetzner edge → installs Lighthouse + chimney + wgmesh + wires Polar.sh checkout. Working agent platform with billing in 15min for ~€15/mo, all EU. Apache 2.0.

**Warrant:** `external:` Polar.sh's own playbook (OSS-as-distribution + hosted-as-monetization) applied recursively. `direct:` `eidos/spec - lighthouse*` and `eidos/spec - service cli*` already specify components — gap is the bundle.

**Rationale:** Every founder who forks the template carries our default. Forks are organic distribution; bug reports harden quality; case studies write themselves.

**Downsides:** Template drift maintenance; €15/mo per fork experimenter; depends on Lighthouse + Polar self-serve.

**Confidence:** 80%
**Complexity:** Medium
**Status:** Unexplored

### 5. `wgmesh-action` — composite GitHub Action for ephemeral CI meshes

**Description:** Drop-in Action spinning ephemeral mesh across CI runners + self-hosted nodes + laptops for workflow duration. Marketplace-listed.

**Warrant:** `reasoned:` no-coord-server architecture uniquely fits ephemeral fleets that NetBird/Tailscale per-seat pricing can't price. `external:` Marketplace as permanent organic distribution.

**Rationale:** Marketplace placement compounds. Every workflow that uses it is a daily wgmesh activation. Bridges agent-builder persona to enterprise CI.

**Downsides:** Restrictive CI egress may block DHT bootstrap; binary size constraints.

**Confidence:** 75%
**Complexity:** Low–Medium
**Status:** Unexplored

### 6. Goose pipeline ships its own onboarding fixes (autonomy-as-flywheel)

**Description:** Wire telemetry from `wgmesh status` (opt-in) + cloudroof funnel + GitHub issues into spec issues → Goose merges fixes within 7 days, publicly tagged `auto-fix-from-install-telemetry`.

**Warrant:** `direct:` STRATEGY's autonomy approach + active Goose pipeline + `docs/dogfooding/stability-log.md` shows team catching defects manually today.

**Rationale:** Autonomy thesis IS the compounding lever. Each fix lowers churn for next user; pipeline shipping fixes IS the marketing. CF Mesh can't replicate this.

**Downsides:** Telemetry opt-in vs no-surveillance positioning; PII scrubbing must be bulletproof; bad-fix gate needed.

**Confidence:** 70%
**Complexity:** Medium
**Status:** Unexplored

### 7. NIS2/GDPR compliance bundle — `wgmesh compliance pack`

**Description:** Auto-generated signed audit-2026.pdf with each release: data-flow diagram, residency attestation derived from configured regions, DPA template, "no traffic exits EU" proof. Auto-updates as topology changes.

**Warrant:** `external:` NIS2 enforcement + Schrems-II + Plausible's $400→$22K MRR via "GDPR-as-identity-not-feature" stance. CF Mesh structurally cannot promise EU-only residency.

**Rationale:** Buys access to a buyer segment (mid-market EU companies with compliance officers) CF Mesh can't serve at all.

**Downsides:** Compliance is a deep rabbit hole; need to anchor on one regulation to avoid scope creep.

**Confidence:** 75%
**Complexity:** Medium
**Status:** Unexplored

## Rejection Summary

| # | Idea | Reason rejected |
|---|------|-----------------|
| F1#2 | Same-NAT first-run guarantee | Folded into idea #6 (autonomy-flywheel telemetry will catch + fix this) |
| F1#5 | `wgmesh doctor` | Adjacent / supporting; not load-bearing for first paying customer |
| F1#8 | Public chaos day | Overlaps idea #6; chaos events become artifacts of the autonomy flywheel |
| F2#1 | Auto-enroll public demo mesh | Identity tension — joining strangers' mesh dilutes "no control plane to trust" claim |
| F2#2 | `wgmesh adopt` | Strong but addresses centralized mode; focus is decentralized + edge for Bet A |
| F2#3 | Zero-config service exposure | Captured by idea #1 as v2 enhancement |
| F2#4 | Repo IS landing page | Worth doing in parallel as cheap copy reposition; not survivor-grade alone |
| F2#5 | Auto-stargazer outreach | Worth doing in parallel; cheap GitHub Action; not survivor-grade alone |
| F2#6 | Polar-receipts as a peer | Too clever — new attack surface, complex semantics, low leverage |
| F2#7 | cloudroof.eu IS a wgmesh customer | Subsumed by idea #4 |
| F2#8 | Daemon as tutor | Adjacent / supporting; not load-bearing |
| F3#1 | Mesh-as-import (pip/npm) | Subsumed by idea #3 |
| F3#2 | Sell to the agent (per-handshake) | Too early — MCP/A2A payment standards still nascent |
| F3#3 | Sell pipeline not wgmesh | Subject-replacement |
| F3#4 | €499 Mesh Kit flat | Subsumed by idea #7 |
| F3#5 | Pay-for-edge | Subsumed by idea #4 |
| F3#6 | Paid by framework | Premature — no leverage to negotiate yet; PR upstream first (idea #3) |
| F3#7 | €99 course | Subject-replacement-adjacent |
| F3#8 | €5 mint mesh secrets | ASP too low to fund runway |
| F4#2 | `wgmesh-bench` benchmark | Worth doing as content artifact during weeks 6–10; not load-bearing |
| F4#8 | mesh-state.json as forkable spec | Out of horizon — standards work compounds over years |
| F5#1 | Svalbard Vault insurance tier | Too speculative — needs CF Mesh outage to convert |
| F5#2 | Bicycle co-op €99 deposit | Niche; insufficient ASP × volume |
| F5#3 | Microgrid IEEE 1547 compliance | Subsumed by idea #7 |
| F5#4 | Couchsurfing vouch network | Adds complexity without near-term revenue path |
| F5#5 | `wgmesh raid` ephemeral coordination | Subsumed by idea #5 |
| F5#6 | AO3 Sovereignty SLA €399 lifetime | Worth pricing experiment alongside idea #7; not survivor-grade alone |
| F5#7 | Hospital side-network positioning | Captured implicitly in idea #4 |
| F5#8 | Drone swarm adversarial demo | Subsumed by idea #6-as-content |
| F6#1 | €0 Bet A | Diagnostic, not a candidate move |
| F6#2 | €1M Bet A | Diagnostic, not a candidate move |
| F6#3 | Zero-human Bet A | Too speculative for 8–12 weeks; flywheel (idea #6) is realistic version |
| F6#4 | One-week Bet A | Diagnostic, not a candidate move |
| F6#5 | Two-year EU standards body | Out of Bet A horizon |
| F6#6 | Kid-homelab persona | Subject-replacement (different ICP) |
| F6#7 | Plausible flip — paid from day 0 | Worth as pricing-page experiment; not survivor-grade alone |
| F6#8 | Lithuanian-only landing | Worth as cofounder-voice tactical play during weeks 1–2; not survivor-grade |
