---
date: 2026-05-05
topic: bet-a-show-hn-demo
---

# Bet A — Show HN Demo Requirements (`wgmesh service add` v1)

## Summary

Phase A v1 ships `wgmesh service add` end-to-end on a single wgmesh-operated Hetzner edge, against the **existing `cr_*` API-key auth model that the lighthouse server, lighthouse-go SDK, and on-disk service.go already implement**. An LLM agent operator runs `wgmesh join --secret <s> --account <cr_*>` and `wgmesh service add ollama :11434`, gets a Polar.sh checkout, pays a flat monthly tier, and is reachable at `https://ollama.<mesh-id>.cloudroof.eu`. Phase B (key-challenge signup, p95 bandwidth billing, Ed25519-derived signing key, multi-edge) is explicitly deferred to v1.1, post-first-customer.

---

## Problem Frame

The wgmesh strategy commits to operators wiring agent fleets to edge nodes in minutes. Today that command does not exist end-to-end — an LLM agent operator with Ollama running on a homelab box reaches for ngrok, Cloudflare Tunnel, or Tailscale Funnel. Each solves the immediate problem at a structural cost: vendor lock-in, free-tier caps, control-plane trust, edges that feel bolted-on. Cloudflare Mesh (launched 2026-04-14) intensifies the pressure but doubles down on the centralized model, leaving the "yours forever, no quota, EU" wedge open.

The brainstorm corpus already promises *"connect your homelab to your VPS in 5 minutes"* as the converting tutorial, and the eidos service-cli spec already names the contract. The first iteration of this brainstorm proposed a key-challenge signup that turned out to be 3-4 cross-repo multi-week builds that don't fit the 8-12 week Bet A window. Phase A retrenches to the working code already on disk: lighthouse exposes `cr_*` org/key auth, lighthouse-go v0.1.x ships the matching client, wgmesh's own `service.go` is already 504 lines of working integration. Phase A is *plumbing what exists* into a public demo + paid signup; Phase B is the rewrite.

---

## Phasing

### Phase A — v1 (this brainstorm; 4–6 weeks)

- Existing `cr_*` API-key auth model retained on lighthouse + lighthouse-go + wgmesh CLI
- One Hetzner edge, single Lighthouse instance, deSEC wildcard DNS for `*.cloudroof.eu`
- Polar.sh checkout wired to Lighthouse for **flat monthly tier** (single SKU)
- `wgmesh service add` produces public HTTPS URL after payment
- Domain rewrite: `wgmesh.dev` → `cloudroof.eu` across spec + SDK + `service.go` constant

### Phase B — v1.1 (deferred until ≥1 paying customer)

- Key-challenge signup pattern (replaces `--account <cr_*>` flag)
- HKDF-derived Ed25519 keypair in `pkg/crypto/derive.go` (`MembershipSigningSeed [32]byte`, info `wgmesh-membership-sign-v1`)
- 95th-percentile bandwidth tier billing (per-mesh-ID 5-min counters, monthly p95-with-top-5%-drop, Polar.sh metered usage)
- Multi-edge / second region for survivability
- BYO-edge mode (`wgmesh edge enable` on customer VPS)

---

## Actors

- A1. **Operator** — LLM agent builder running Hermes/openclaw on their own VPS or homelab box, devs not devops. Holds a Lighthouse `cr_*` API key for their org. Pays via Polar.sh.
- A2. **wgmesh CLI / daemon** — runs on the operator's node. Reads `cr_*` from `/var/lib/wgmesh/account.json`, derives mesh IP from secret, calls Lighthouse via lighthouse-go SDK.
- A3. **Lighthouse** — control plane at `lighthouse.cloudroof.eu`. Stores orgs, API keys, sites. Receives Polar.sh webhooks; gates `POST /v1/sites` on org subscription state.
- A4. **Edge node** — wgmesh-operated Hetzner box. Caddy with on-demand TLS for `*.<mesh-id>.cloudroof.eu` (or shared `*.cloudroof.eu` — see Outstanding Questions). Pulls Caddyfile from Lighthouse; proxies to operator's mesh IP via wgmesh tunnel.
- A5. **Polar.sh** — billing rail (EU MoR). Hosts checkout, sends signed webhooks confirming subscription state.

---

## Key Flows

- F1. **First-time signup + service registration**
  - **Trigger:** Operator runs `wgmesh init --secret`, then `wgmesh signup --email <e>` (or visits cloudroof.eu and signs up via web).
  - **Actors:** A1, A2, A3, A5
  - **Steps:**
    1. Operator signs up at cloudroof.eu (web) or via `wgmesh signup` (CLI). Lighthouse provisions an org + initial `cr_*` API key, returns Polar.sh checkout URL for the flat tier.
    2. Operator completes payment in browser; Polar.sh webhook lands at Lighthouse → org marked `active`.
    3. Operator runs `wgmesh join --secret <s> --account <cr_*>` on their first node — `cr_*` written to `/var/lib/wgmesh/account.json`.
    4. Operator runs `wgmesh service add ollama :11434`.
    5. CLI calls `POST /v1/sites` with `cr_*` Bearer token + mesh IP + service name; Lighthouse verifies subscription is `active` and creates the site record.
    6. Lighthouse pushes the updated Caddyfile to the edge (or edge polls); edge's on-demand TLS provisions cert if first hit for that hostname.
    7. CLI prints `https://ollama.<mesh-id>.cloudroof.eu`.
  - **Outcome:** Service is reachable. Local state in `/var/lib/wgmesh/services.json` records the registration.
  - **Covered by:** R1, R2, R3, R5, R6, R7, R8, R9, R12

- F2. **Returning operator adds a second service**
  - **Trigger:** Operator with existing `cr_*` runs `wgmesh service add api :8080`.
  - **Actors:** A1, A2, A3
  - **Steps:**
    1. CLI loads `cr_*` from local state.
    2. `POST /v1/sites` succeeds immediately; org already `active`.
    3. Edge serves the new hostname (cert provisioned on-demand).
    4. CLI prints `https://api.<mesh-id>.cloudroof.eu`.
  - **Outcome:** Second service live in seconds.
  - **Covered by:** R1, R5, R6, R8, R9

- F3. **Lighthouse temporarily unreachable**
  - **Trigger:** Operator runs `wgmesh service list` while Lighthouse is down or operator is offline.
  - **Actors:** A1, A2
  - **Steps:**
    1. CLI calls `GET /v1/sites` → fails.
    2. Falls back to `/var/lib/wgmesh/services.json`.
    3. Prints cached list with `(local cache, Lighthouse unreachable)` indicator on each row.
  - **Outcome:** Operator sees their services without crash.
  - **Covered by:** R5

- F4. **Subscription lapse / payment failure**
  - **Trigger:** Polar.sh webhook reports subscription cancelled or expired.
  - **Actors:** A3, A5, A4
  - **Steps:**
    1. Lighthouse marks org `inactive`.
    2. Active sites for that org are removed from the edge Caddyfile within one config-pull cycle.
    3. Operator's existing services return 4xx/5xx with a "subscription required" body until reactivation.
    4. `wgmesh service list` continues to show services (reflects local + Lighthouse state) with status `subscription_inactive`.
  - **Outcome:** Operator can re-subscribe to restore service without re-running `service add`.
  - **Covered by:** R4, R8, R11

---

## Requirements

**`service add` / `service list` / `service remove` (Phase A)**

- R1. CLI MUST authenticate to Lighthouse using a `cr_*` API key loaded from `/var/lib/wgmesh/account.json`. The existing `--account <cr_*>` flag on `wgmesh join` writes the file. Phase A retains this flow; Phase B replaces it with key-challenge.
- R2. `wgmesh service add <name> <local-addr>` MUST call `POST /v1/sites` via lighthouse-go SDK and print `https://<name>.<mesh-id>.cloudroof.eu` on success.
- R3. `wgmesh service add` MUST surface a clear error when the org is `inactive` (subscription lapsed) — message includes a link to the Polar.sh customer portal for reactivation.
- R4. Subscription state changes (cancellation, lapse, reactivation) MUST propagate to the edge within one config-pull cycle (default 30s) — services are removed from / restored to the edge Caddyfile accordingly.
- R5. `wgmesh service list` MUST query Lighthouse first, fall back to `/var/lib/wgmesh/services.json` on failure, with a clear `(local cache, Lighthouse unreachable)` indicator on fallback rows.
- R6. `wgmesh service remove <name>` MUST deregister the site at Lighthouse (DELETE `/v1/sites/{id}`) and remove the local entry; failure to reach Lighthouse MUST surface clearly without removing the local entry.

**Lighthouse server (deployed for Phase A)**

- R7. Lighthouse MUST be deployed at `lighthouse.cloudroof.eu` with public HTTPS (single-domain cert, not wildcard) and persistent storage for orgs / keys / sites. Existing endpoints (`/v1/orgs`, `/v1/orgs/{id}/keys`, `/v1/sites`, `/v1/sites/{id}`, `/v1/sites/{id}/dns-status`, `/v1/edges`) MUST be reachable from the public internet without requiring a pre-existing wgmesh tunnel.
- R8. Lighthouse MUST integrate with Polar.sh — accept signed webhooks (HMAC verified using shared secret in env), reconcile via Polar API on webhook absence (cron poll, ~5 min interval), and gate site creation on `active` subscription state. Webhook handler MUST be idempotent on Polar's idempotency key.
- R9. Lighthouse MUST expose a self-serve signup endpoint (or web page on cloudroof.eu) that creates an org, mints the first `cr_*` API key, and returns/redirects to a Polar.sh checkout URL.

**Edge node (cloudroof.eu in Phase A)**

- R10. A single wgmesh-operated Hetzner edge MUST terminate TLS for managed hostnames, pull Caddyfile from Lighthouse, and proxy each request to the operator's mesh IP via the wgmesh tunnel. Caddy on-demand TLS handles per-hostname cert issuance.
- R11. Edge MUST recover gracefully from origin (mesh-IP) unreachability — return a 502 within 5 seconds with a body that names the edge and suggests checking the origin. Edge MUST also remove inactive-org sites within one config-pull cycle (R4).
- R12. Edge MUST gate origin requests with a Bearer token derived from the mesh secret + service name (HKDF info `wgmesh-service-token-v1:<service-name>`, 32 bytes hex-encoded). The CLI computes the token at `service add` time and POSTs it to Lighthouse alongside the site record; edge pulls the token via Caddyfile and validates `Authorization: Bearer <token>` per request. `--public` flag opts out of token-gating with a one-time CLI confirmation. Default origin MUST NOT be silently public. Token rotation = `wgmesh service rotate-token <name>` regenerates from a per-service nonce mixed into HKDF (deferred to Phase A.1 if not in initial cut).

**Wildcard DNS**

- R13. `*.cloudroof.eu` MUST resolve to the operated edge node — managed at deSEC. The DNS layout (flat `<service>-<mesh-id>.cloudroof.eu` under one wildcard, or nested `<service>.<mesh-id>.cloudroof.eu` per-mesh wildcard) is decided in planning per the Outstanding Questions; both have working ACME paths but different LE rate-limit profiles.

**Pricing surface (Phase A)**

- R14. The Polar.sh product/tier for Phase A cloudroof.eu MUST be live as a **single flat monthly tier at €5/mo** (mirrors the existing GitHub Sponsors Contributor tier in `FEATURE_MATRIX.md`). Sized for the modal customer running 3–10 small services at single-digit Mbps average. Egress overage is absorbed in Phase A — the tier is intentionally below cost-coverage breakeven for early customers and converts to p95 tiers in Phase B once we have egress data. Checkout MUST complete payment in EU-regulated rails (Polar.sh MoR).
- R15. wgmesh source code, binaries, and daemon MUST remain fully open source under the existing license. cloudroof.eu paid product MUST never gate the OSS path; running wgmesh standalone (no Lighthouse, no `--account` flag) MUST continue to work without any cloudroof account.

**Domain migration (Phase A)**

- R16. `lighthouse-go` SDK MUST ship v0.2.0 with `managedDomain` as a `NewClient` option (default `cloudroof.eu`); the v0.1.0 hardcoded `wgmesh.dev` discovery is removed. wgmesh `go.mod` bumps to v0.2.0 in lockstep.
- R17. `service.go` `managedDomain = "wgmesh.dev"` constant (line 25) MUST be updated to `"cloudroof.eu"`. Eidos `spec - service cli ...md` MUST be updated wherever it references `wgmesh.dev`. Old `wgmesh.dev` apex remains parked as a redirect to `cloudroof.eu` for OSS docs continuity for at least 12 months.

---

## Acceptance Examples

- AE1. **Covers R2, R3, R4.** Given an org with active Polar.sh subscription and a `cr_*` API key written to `/var/lib/wgmesh/account.json`, when the operator runs `wgmesh service add ollama :11434`, the CLI calls `POST /v1/sites` and prints `https://ollama.<mesh-id>.cloudroof.eu` within 5 seconds. The edge serves a 200 from `ollama.<mesh-id>.cloudroof.eu` within 60 seconds (covering on-demand TLS provisioning + Caddyfile reload).
- AE2. **Covers R3, R4.** Given an org whose Polar.sh subscription has lapsed, when the operator runs `wgmesh service add api :8080`, the CLI surfaces an error message naming the lapse and the Polar.sh customer-portal URL; no site record is created; existing services for that org return 5xx until reactivation.
- AE3. **Covers R5.** Given a previously registered service `ollama`, when `service list` runs and Lighthouse returns 5xx or is unreachable, the CLI prints the cached list with a `(local cache, Lighthouse unreachable)` indicator on each row and exits 0.
- AE4. **Covers R11.** Given an active site `ollama.<mesh-id>.cloudroof.eu` whose origin mesh IP is offline, when an external HTTPS request hits the edge, the response is a 502 within 5 seconds with a body that names the edge and suggests checking the origin — never a connection hang.
- AE5. **Covers R12.** Given a default `wgmesh service add ollama :11434` (no `--public`), the edge MUST require an operator-issued bearer token to proxy to the origin; an unauthenticated request returns 401 from the edge without ever reaching the origin.
- AE6. **Covers R15.** Given a fresh checkout of wgmesh with no `--account` flag and no cloudroof account, when an operator runs `wgmesh init --secret` and `wgmesh join` on multiple nodes, the mesh forms and decentralized features work — no Lighthouse calls fire on this path.

---

## Success Criteria

- First Polar.sh subscription on cloudroof.eu lands within the 8–12 week Bet A window, attributable to the demo flow above.
- A stranger reading a Show HN comment can paste the demo command, complete signup + payment + service registration, and be reachable on a public HTTPS URL in under 10 minutes (Phase A: relaxed from 5 min to acknowledge the Polar.sh checkout + on-demand TLS round-trips honestly).
- Internal: ce-plan can ingest this requirements doc and produce a phased implementation plan without inventing actor identities, missing endpoints, or pricing-tier mechanics.

---

## Scope Boundaries

### Deferred to Phase B (v1.1)

- Key-challenge signup pattern (mesh secret signs server-issued challenge instead of `--account` flag)
- Ed25519 keypair derivation in `pkg/crypto/derive.go`
- 95th-percentile bandwidth tier billing + per-mesh-ID 5-min counters + Polar metered-usage integration
- Second edge / multi-region survivability
- Polar.sh-failure runbook beyond R8 reconcile loop

### Outside this product's identity / not Phase B either

- `wgmesh up --provider hetzner --nodes <N>` — separate brainstorm
- BYO-edge mode (`wgmesh edge enable` on customer VPS) — separate brainstorm; tracked under Track 2 (edge-as-first-class)
- `wgmesh agent-id` per-agent crypto identity — separate ideation survivor
- Federated Lighthouse / xDS sync across edges — long-horizon
- Free tier on cloudroof.eu — explicitly out (paid from day 0)
- ACLs / segmentation in decentralized mode — Horizon 2
- Mobile clients, SSO, audit logs — Horizon 3

---

## Key Decisions

- **Phase A retains the existing `cr_*` API-key model.** lighthouse server, lighthouse-go SDK v0.1.x, and on-disk `service.go` (504 lines) already implement this flow end-to-end. Phase B's key-challenge rewrite is a multi-week cross-repo build that doesn't fit the 8-12 week window; deferring it preserves the Bet A timeline.
- **Phase A pricing = single flat monthly tier on Polar.sh.** p95 bandwidth tiers are deferred to Phase B once we have actual customer egress data to set tier cut-offs against. wgmesh code stays open source forever; the OSS path is never gated.
- **Edge ownership v1 = wgmesh-operated rentier.** Consistent with STRATEGY: cloudroof.eu is a bundled edge offering. Single-edge SPOF is a known accepted risk in Phase A — an Incident Response runbook + cold-spare image (deferred to planning) substitutes for true multi-edge until Phase B.
- **Default `wgmesh service add` requires authenticated origin** (R12) — the edge enforces a bearer token between the public internet and the operator's local service. `--public` opts out with a confirmation. This prevents the homelab-data-leak failure mode where `service add ollama :11434` would expose an unauthenticated Ollama to the world.
- **Domain rewrite is a Phase A deliverable across three artifacts** (R16, R17): lighthouse-go SDK v0.2.0, wgmesh `service.go` constant, eidos spec. Old `wgmesh.dev` parks as a redirect.
- **Counterfactual to beat = ngrok / Cloudflare Tunnel.** Time-to-public-URL bar is acknowledged honestly: ngrok's 30s is anonymous + free; wgmesh's ~10 minutes (Phase A) is paid + cert-provisioned + custom-subdomain — competing on different axes ("yours forever, no quota, EU"), not raw seconds.

---

## Dependencies / Assumptions

- **Lighthouse repo** (`github.com/atvirokodosprendimai/lighthouse`) already implements `cr_*` org/key auth + `/v1/sites` CRUD. Phase A adds: Polar.sh webhook handler with HMAC verification + Polar reconcile cron + signup endpoint that mints `cr_*` and returns checkout URL. No new auth model.
- **`lighthouse-go` SDK** v0.2.0 ships with parameterized `managedDomain`. wgmesh imports v0.2.0.
- **Polar.sh** product is created, priced (single flat tier), and integrated. EU MoR rails are live. Webhook signing secret stored in Lighthouse env, not source.
- **deSEC DNS** for cloudroof.eu has the wildcard record(s) set; ACME path resolved per R13 (planning decides flat-vs-nested).
- **Hetzner edge box** has been provisioned with `deploy/edge/setup.sh`, Caddy with on-demand TLS, and a wgmesh tunnel back to participating meshes. The setup script's "WireGuard mesh already running on this node" prerequisite (line 6) is updated for the public-internet bootstrap path: edge can serve before any operator's mesh is configured for it.
- **Lighthouse host bootstrap:** the public-internet endpoint at `lighthouse.cloudroof.eu` does NOT require a pre-existing wgmesh tunnel for Phase A — `deploy/lighthouse/setup.sh` line 6 prerequisite is relaxed. (Federated multi-Lighthouse with mesh sync is Phase B.)
- **`pkg/crypto/derive.go`** is unchanged in Phase A. Phase B's `MembershipSigningSeed` addition will go through the "do not modify without review" path noted in CLAUDE.md.

---

## Outstanding Questions

### Deferred to Planning

- [Affects R13][Needs research] DNS layout: flat `<service>-<mesh-id>.cloudroof.eu` (single `*.cloudroof.eu` wildcard, one DNS-01 issuance) vs nested `<service>.<mesh-id>.cloudroof.eu` (per-mesh-ID wildcard via Caddy on-demand TLS, hits LE 50/week/registered-domain rate limit). Trade-off: flat avoids rate limits but URL is less hierarchical; nested matches the spec but constrains signup velocity.
- [Affects R8][Technical] Polar.sh reconciliation cron interval and idempotency-key handling on retries.
- [Affects R10][Technical] Caddy on-demand TLS configuration and the deSEC API token storage path on the edge box.
- [Affects R7][Technical] Whether v1 Lighthouse uses Dragonfly (per spec) or a simpler durable store; Dragonfly was specced for federated sync which is out of Phase A.
- [Affects R10, R11][Technical] Edge config-pull cadence and reload semantics — the Caddy-pulls-from-Lighthouse pattern is in `deploy/edge/setup.sh` but the Phase A claim/payment flow may want push-style invalidation.
- [Affects single-edge SPOF][Operational] Incident-response runbook for the Phase A single-edge box: cold-spare image cadence, DNS-flip procedure, Show HN day on-call rotation.
