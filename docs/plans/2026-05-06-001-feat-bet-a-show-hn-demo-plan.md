---
title: "feat: Bet A Phase A — Show HN demo (`wgmesh service add` v1)"
type: feat
status: active
date: 2026-05-06
deepened: 2026-05-06
origin: docs/brainstorms/2026-05-05-bet-a-show-hn-demo-requirements.md
---

# feat: Bet A Phase A — Show HN demo (`wgmesh service add` v1)

> **Target repos (multi-repo plan):**
>
> - **wgmesh** — this repo. Path-relative throughout unless prefixed.
> - **lighthouse** (`github.com/atvirokodosprendimai/lighthouse`) — separate repo. Prefix paths with `lighthouse:` in unit file lists.
> - **lighthouse-go** (`github.com/atvirokodosprendimai/lighthouse-go`) — separate repo. Prefix paths with `lighthouse-go:` in unit file lists.

## Summary

Phase A ships `wgmesh service add` end-to-end on the existing `cr_*` API-key model in 6–8 weeks across three coordinated repos. CLI derives an HKDF service token; Lighthouse stores only its SHA-256 hash and validates by hashing the presented Bearer (never sees mesh secret). Lead with `lighthouse-go` SDK v0.2.0 (managed-domain + signup + typed errors + idempotency-key) and a non-modifying `pkg/crypto/service_token.go`. CLI bundles into one big edit (token wiring + domain rewrite + R3/R5/R6 fixes) plus a separate `signup.go`. Lighthouse server splits into Polar foundation (webhook+reconcile+checkout) and signup+gating; edge config and edge auth are separate units. Deploy scripts swap `apt install caddy` for `xcaddy` with `caddy-dns/desec` and tighten the deSEC token scope. A dedicated runbook unit closes single-edge brand risk for Show HN day.

---

## Problem Frame

The wgmesh strategy commits to operators wiring agent fleets to edge nodes in minutes, but the converting demo (`wgmesh service add`) does not work end-to-end today: Lighthouse is unprovisioned, Polar.sh is not wired, the existing CLI prints `wgmesh.dev` URLs that don't resolve, and origins are exposed without authentication. The first iteration of this brainstorm proposed a key-challenge signup that turned out to be a multi-week cross-repo rewrite incompatible with the 8–12 week Bet A window. Phase A retrenches to the working `cr_*` API-key path that lighthouse, lighthouse-go v0.1.0, and the on-disk 504-line `service.go` already implement. (See origin: `docs/brainstorms/2026-05-05-bet-a-show-hn-demo-requirements.md`.)

---

## Requirements

**CLI (`service add` / `service list` / `service remove`)**

- R1. CLI MUST authenticate to Lighthouse using a `cr_*` API key from `/var/lib/wgmesh/account.json` (origin R1).
- R2. `wgmesh service add` MUST call `POST /v1/sites` and print the managed URL (origin R2).
- R3. CLI MUST surface `subscription_inactive` with the Polar.sh customer-portal URL (origin R3).
- R4. **Subscription state changes MUST result in the edge's next config-pull (max 30s) showing inactive orgs' sites removed and the Caddyfile reloaded within 5s of pull completion.** (Reworded from origin R4 for unambiguous pull-completion semantics.)
- R5. `wgmesh service list` MUST query Lighthouse first, fall back to local with `(local cache, Lighthouse unreachable)` indicator on each row (origin R5).
- R6. `wgmesh service remove` MUST surface Lighthouse failure clearly without removing the local entry (origin R6).

**Lighthouse server**

- R7. Lighthouse MUST be reachable at `lighthouse.cloudroof.eu` over public HTTPS without requiring a pre-existing wgmesh tunnel (origin R7).
- R8. **Lighthouse MUST verify Polar.sh webhook HMAC (Standard Webhooks spec, multi-signature for rotation, 5-min replay window). Webhook handler MUST persist a `webhook_id` ledger in Postgres (TTL 24h ≥ max rotation window) for replay rejection. Polar metered-event ingest, when emitted, MUST use Polar's `external_id` field as the dedup key (separate concern from webhook replay protection). Reconcile cron MUST run at 5-min interval, switching to 1-min when webhook-disabled state is detected, with reconcile as the source of truth on conflict.** (Origin R8 split into two distinct idempotency mechanisms.)
- R9. Lighthouse MUST expose a self-serve signup endpoint creating an org, minting `cr_*`, and returning a Polar.sh checkout URL. Email is unique on the org table; on Polar duplicate-customer error, return HTTP 409 with `{title:"email_already_registered"}` (origin R9).

**Edge node**

- R10. The wgmesh-operated Hetzner edge MUST terminate TLS for `*.cloudroof.eu` via Caddy on-demand TLS, pull Caddyfile from Lighthouse, and proxy via wgmesh tunnel (origin R10).
- R11. Edge MUST return a 502 within 5s on origin unreachability and remove inactive-org sites within one config-pull cycle (origin R11).
- R12. **Edge MUST gate origin requests with a Bearer token derived in the wgmesh CLI from mesh secret + service name (HKDF info `wgmesh-service-token-v1:<service-name>`, 32 bytes, hex-encoded). CLI POSTs the token to Lighthouse on `service add`; Lighthouse persists only `SHA-256(token)` (never the token itself, never the mesh secret). Edge `forward_auth` validates by hashing the presented Bearer and constant-time-comparing against the stored hash. `--public` opts out: CLI does NOT derive a token, passes `public:true` on the request, server stores no hash, edge omits `forward_auth` for that handle. `--public` requires a one-time CLI confirmation (or `--yes` in non-tty CI) AND emits a server-side audit log row plus an `is_public=true` flag visible in `service list`.** (Origin R12 + token-architecture decision.)

**Wildcard DNS**

- R13. `*.cloudroof.eu` MUST resolve to the operated edge node, managed at deSEC. Phase A locks the URL shape to flat `<service>-<mesh-id>.cloudroof.eu` under one shared `*.cloudroof.eu` wildcard (origin R13).

**Pricing surface**

- R14. Polar.sh product/tier MUST be live at €5/mo flat. EU-regulated rails (Polar.sh MoR) (origin R14).
- R15. wgmesh source code MUST remain fully open source. cloudroof.eu paid product MUST never gate the OSS path (origin R15).

**Domain migration**

- R16. `lighthouse-go` SDK MUST ship v0.2.0 with `managedDomain` option, `Signup` method, `OriginAuthToken` field on `CreateSiteRequest`, typed errors, and per-call `Idempotency-Key` header support. **Pre-tag verification:** check `pkg.go.dev/.../?tab=importedby` for external consumers. If any exist, ship additive `NewClientWithOptions` constructor; keep `NewClient(baseURL, apiKey)` working unchanged.
- R17. Domain rewrite `wgmesh.dev` → `cloudroof.eu` MUST land across `service.go:25`, eidos spec, SDK, and `service_test.go` fixtures (origin R17).

**Edge config integrity**

- R18. **Edge config-pull from Lighthouse MUST authenticate with a per-edge `EDGE_TOKEN` (cr_edge_*-prefixed). Token is minted by Lighthouse at edge-provision time, deposited at `/etc/wgmesh-edge/token.env` (mode 0600), validated by Lighthouse middleware on `/v1/xds/*`. Rotation procedure: mint new, deploy, revoke old.** (New requirement; resolves the unauthenticated config-pull surface.)

**Origin actors:** A1 (Operator), A2 (wgmesh CLI / daemon), A3 (Lighthouse), A4 (Edge node), A5 (Polar.sh)
**Origin flows:** F1 (signup + service registration), F2 (returning operator second service), F3 (Lighthouse temporarily unreachable), F4 (subscription lapse / payment failure)
**Origin acceptance examples:** AE1 (covers R2/R3/R4), AE2 (covers R3/R4 lapse), AE3 (covers R5), AE4 (covers R11), AE5 (covers R12), AE6 (covers R15)

---

## Scope Boundaries

### Phase A.1 candidates (same 8–12 week Bet A window, may re-enter)

- Token rotation command (`wgmesh service rotate-token <name>`) — adds per-service nonce mixed into HKDF info string + a `derivation_scheme_version` column on the site record.
- ETag / `If-None-Match` on edge config-pull (CONFIG_POLL_INTERVAL=30s efficiency).

### Phase A → Phase B token migration

Phase A tokens persist valid into Phase B. Phase B introduces a new info string (e.g., `wgmesh-service-token-v2:`) alongside `v1`; Lighthouse stores `derivation_scheme_version` per row and accepts both during a 90-day sunset window post-Phase-B-launch. After sunset, Phase A customers must re-add services. The 90-day window matches Polar.sh subscription cycles. Migration trigger: Phase B planning explicitly inherits this contract.

### Phase B (deferred, separate brainstorm post-first-customer)

- Key-challenge signup pattern (replaces `--account <cr_*>`)
- Ed25519 `MembershipSigningSeed` in `pkg/crypto/derive.go`
- 95th-percentile bandwidth tier billing + per-mesh-ID 5-min counters + Polar metered-usage integration. **Phase A does NOT emit metered events** — backfill from edge logs in Phase B once meter design is final.
- Multi-edge / multi-region survivability + fallback CA path (ZeroSSL/Buypass) for cert-revocation tolerance.
- BYO-edge mode (`wgmesh edge enable` on customer VPS).

### Outside this product's identity

- `wgmesh up --provider hetzner` (Hetzner provisioning) — separate brainstorm.
- `wgmesh agent-id` — separate brainstorm.
- Federated Lighthouse / xDS sync.
- Free tier on cloudroof.eu — explicitly out.
- Mobile clients, SSO, audit logs, ACLs decentralized — Horizons 2/3.
- Public Suffix List entry for `cloudroof.eu` — multi-month process; required only if Phase B/C revisits per-mesh wildcards.

---

## Context & Research

### Relevant Code and Patterns

- **wgmesh `service.go`** — already 504-line working `cr_*` client. Three SDK call-sites (`service.go:125`, `service.go:223`, `service.go:321`). Domain constant at `service.go:25`. Flagset for `service add` at `service.go:56-64`.
- **wgmesh `pkg/crypto/derive.go:296`** — private `deriveHKDF` helper, NOT exposed. Phase A reimplements HKDF inline in a new sibling file using stdlib `crypto/hkdf` to avoid the CLAUDE.md "do not modify without review" gate. **HKDF derivation is wgmesh-CLI-only**; Lighthouse never calls it.
- **wgmesh `pkg/mesh/account.go:11-14`** — `AccountConfig{APIKey, LighthouseURL}` schema with atomic `SaveAccount` (0600 perms). Backwards-compatible `omitempty` extension fits Phase A's `SubscriptionStatus`/`PortalURL` additions.
- **wgmesh `pkg/mesh/services.go:11-19`** — `ServiceEntry` schema. Adding `LastStatus` powers the R5 indicator.
- **wgmesh `deploy/edge/setup.sh`** — Caddyfile-pull pattern is sound (`CONFIG_POLL_INTERVAL=30`, validate-before-reload, fall back on pull failure). Gaps: unauthenticated fetch (resolved by R18), no ETag (Phase A.1), swallowed reload errors (`|| true` → logged-error). Replace `apt install caddy` (line 25-30) with `xcaddy build --with github.com/caddy-dns/desec`.
- **wgmesh `deploy/lighthouse/setup.sh:6`** — "WireGuard mesh already running on this node" prerequisite is documentation drift; the actual install does NOT depend on a tunnel. Phase A drops the comment and adds public-TLS termination in front of `:8443`.
- **lighthouse-go v0.1.0** at `~/go/pkg/mod/github.com/atvirokodosprendimai/lighthouse-go@v0.1.0/`. Single-file SDK (~210 LOC). Hardcoded `wgmesh.dev` at `client.go:174` and `:185`. v0.2.0 is a clean module bump (~75–150 LOC delta), no replace directives. Pre-tag importer-check via `pkg.go.dev` is mandatory.
- **lighthouse repo** — extracted, has `cr_*` Bearer auth + org/key/site CRUD. Adds: signup endpoint, Polar webhook handler, reconcile cron, `/v1/edge/auth` (Bearer hash-validation), `/v1/xds/*` middleware (EDGE_TOKEN check), Caddyfile generator with forward_auth blocks, schema migration for `origin_auth_token_hash` and `is_public` columns.

### Institutional Learnings

- **`docs/solutions/logic-errors/nat-punch-self-connection-via-shared-public-ip.md`** — same-NAT colocated peer pair is the modal first-customer environment; LAN-multicast + WGPubKey-not-IP self-filter must be merged on whatever tag the demo binary ships from.
- **`docs/solutions/logic-errors/custom-subnet-silent-fallback-and-missed-callsites.md`** — Goose-pipeline meta-lesson: diff plan vs `git diff --stat` because `package main` callsites get missed. Phase A unit U3 touches both `service.go` (root package) and `pkg/mesh/`; this lesson applies directly.
- **`docs/solutions/integration-issues/goreleaser-dual-tag-same-commit-conflict.md`** — clean rc tags before final release tag. Show-HN-day footgun for the launch tag.

### External References

- **Polar.sh — Standard Webhooks spec.** Headers: `webhook-id`, `webhook-timestamp`, `webhook-signature` (multi-sig for rotation). HMAC-SHA256 input is `id.timestamp.body`. Secret format `whsec_<base64>` — base64-decode before HMAC. 5-min replay window. Up to 10 retries with exp backoff; endpoint auto-disabled after 10 failures. Polar's `external_id` is the event-dedup key (no `Idempotency-Key` header on metered ingest).
- **Polar.sh — checkout creation.** `POST /v1/checkouts/` with `external_customer_id=<orgID>` for per-org binding. `success_url` supports `{CHECKOUT_ID}` template. Polar customer keying is by email globally — Phase A enforces email uniqueness at lighthouse + 409 on Polar duplicate-customer error.
- **Polar.sh — EU MoR.** VAT number EU372061545. Handles global VAT/GST. No own VAT registration required.
- **Caddy v2 — DNS-01 with deSEC.** Module `dns.providers.desec` (`caddy-dns/desec`). Build via `xcaddy build --with github.com/caddy-dns/desec`. Default apt package does NOT include it.
- **Caddy v2 — `forward_auth` directive.** Lighthouse exposes `/v1/edge/auth` returning 200 + `X-Org-ID` / `X-Service-Name` headers (forwarded to origin) or 401. Default cache TTL = 60s with stale-while-error 5-min to bound Lighthouse-outage blast radius.
- **Caddy 2.8.x DNS-01 regression caddyserver/caddy#6557** — pin to 2.7.6 or 2.9.x verified-fix release. If 2.9.x ships without fix, alternative is HTTP-01 per-domain (hits LE rate limit only on signup; flat URL shape avoids fleet-wide reissuance).
- **Let's Encrypt rate limits.** 50 certs / registered-domain / 7d (counts wildcards). 5 duplicate / 7d. 5 failed-validation / identifier / hour. Default cert lifetime dropped 90→45 days on 2025-12-02.
- **deSEC API — token scoping.** Per-rrset policies require deny-default + explicit allow. **deSEC does NOT support glob `subname` matching.** For Caddy DNS-01 against `_acme-challenge.<svc>-<mesh-id>.cloudroof.eu`, the practical scope is `domain=cloudroof.eu, type=TXT, perm_write=true` (no subname filter) — narrower than full-zone but broader than apex-only. Set `allowed_subnets` to edge egress IP, `max_age=365d`, `max_unused_period=14d`.
- **Go 1.25 stdlib `crypto/hkdf`.** `hkdf.Key(sha256.New, secret, nil, info, 32)` is the canonical idiom. Drop `golang.org/x/crypto/hkdf` references. Constant-time compare via `crypto/subtle.ConstantTimeCompare`.

---

## Key Technical Decisions

- **Token storage at Lighthouse: SHA-256 hash, not plaintext.** Lighthouse stores `origin_auth_token_hash` only. CLI derives token via HKDF + posts plaintext to Lighthouse over HTTPS at site-creation time; Lighthouse hashes immediately and persists only the hash. Edge `forward_auth` validates by hashing the presented Bearer and `subtle.ConstantTimeCompare` against the stored hash. **Lighthouse never stores or sees the mesh secret; never re-derives.** This eliminates the HKDF-info-string-drift risk (function lives only in wgmesh) and bounds Lighthouse-DB compromise to per-token brute force (32-byte HKDF output → 2^256 search space).
- **DNS shape: flat `<service>-<mesh-id>.cloudroof.eu` under one shared `*.cloudroof.eu` wildcard.** Per-mesh wildcards would hit the LE 50-cert/7d cliff at ~7 meshes/day capacity. Single shared wildcard issues once per renewal; cert blast-radius accepted as Phase A constraint, mitigated by HTTP-layer `forward_auth` gating + the U11 single-edge runbook.
- **Edge auth: Caddy `forward_auth` to lighthouse `/v1/edge/auth`** with cache TTL 60s + stale-while-error 5min default, set in U10. Bounds Lighthouse-outage blast radius from "instant 401 storm" to "5-min stale-but-served" worst case.
- **HKDF: stdlib `crypto/hkdf` (Go 1.24+).** Repo is on 1.25. `hkdf.Key(sha256.New, secret, nil, info, 32)`.
- **Service-token file: new `pkg/crypto/service_token.go`.** Re-implements HKDF using stdlib `crypto/hkdf` directly; does NOT export `derive.go`'s private `deriveHKDF` helper or modify `derive.go`. Keeps the CLAUDE.md "do not modify without review" gate intact.
- **Caddy version pin: 2.9.x verified-fix release.** 2.8.x has the DNS-01 regression. `xcaddy build` step in `deploy/edge/setup.sh`. Pre-U8 sprint: confirm fix in 2.9.x release notes; if absent, switch to HTTP-01 per-domain.
- **Polar webhook verification: hand-rolled per Standard Webhooks spec** (~30 LOC in lighthouse repo). `POLAR_WEBHOOK_SECRET` env accepts comma-separated list to support rotation without process restart. Webhook-id ledger persisted in Postgres (24h TTL).
- **Polar metered ingestion deferred to Phase B.** Phase A does not emit metered events. Phase B backfills history from edge access logs.
- **Reconcile cron: 5-min interval default, 1-min during webhook-disabled state**, polling Polar's webhook-health endpoint for state detection. Reconcile is the source of truth on conflict; webhook is a fast-path hint.
- **Idempotency:** wgmesh CLI sends `Idempotency-Key` header on `CreateSite` / `Signup`. Webhook handler dedups via `webhook-id` ledger. (No Polar metered ingest in Phase A → no `external_id` concern.)
- **`Authorization: Bearer <token>` transport encoding: hex** (per origin R12). 64-char hex string carried as a Bearer token.
- **`--public` audit:** server-side `is_public=true` flag on site record, structured log entry on creation, `service list` shows `[PUBLIC]` indicator next to such services.

---

## Open Questions

### Resolved During Planning

- DNS shape (flat vs per-mesh) → flat. (See Key Decisions.)
- Edge auth mechanism (forward_auth vs in-process JWT) → forward_auth + cache.
- HKDF library (stdlib vs x/crypto) → stdlib.
- Caddy version pin → 2.9.x with HTTP-01 fallback.
- Webhook verification (library vs hand-rolled) → hand-rolled.
- Service-token derivation file location → new `pkg/crypto/service_token.go`, no `derive.go` modification.
- **Token storage at Lighthouse (re-derive vs cache)** → SHA-256 hash. CLI posts plaintext over HTTPS once at create time; server hashes + stores hash; edge presents Bearer and server hash-compares.
- **HKDF info-string drift across repos** → resolved by hash-storage. Function exists only in wgmesh.
- **U5/U6 sizing** → split. New U9 (signup + gate) and U10 (edge auth) created; U-IDs U1-U8 unchanged.
- **Polar metered ingestion in Phase A** → dropped. Phase B feature.
- **Show HN runbook ownership** → new U11.
- **EDGE_TOKEN lifecycle** → R18 + provisioned in U8 + validated in U10.
- **Polar customer-by-email collision** → 409 `email_already_registered` (R9).
- **Phase A → Phase B token migration** → 90-day dual-derivation window post-Phase-B launch (Scope Boundaries).
- **`--public --yes` audit trail** → server-side `is_public=true` + log + `service list` indicator.
- **Polar webhook auto-disable + reconcile cadence** → 1-min interval during webhook-disabled state (R8).
- **Webhook replay ledger persistence** → Postgres, 24h TTL (R8).

### Deferred to Implementation

- Caddy 2.9.x DNS-01 fix verification — pre-U8 sprint check; HTTP-01 fallback if no fix.
- Forward_auth cache invalidation on subscription state change — coarse expiry vs explicit purge endpoint.
- Lighthouse Polar-event ingestion: not applicable in Phase A.
- Webhook-disabled detection mechanism — Polar webhook-health endpoint vs N-consecutive-failure heuristic.
- `xcaddy build` reproducibility — Dockerfile or pinned-version build job.
- Postgres schema migration tooling choice (golang-migrate, sqlc, etc.) — lighthouse repo's existing convention applies.

### FYI / Future hardening

- Requirements grouping: applied above (header per concern). Done.
- Eidos line-number brittleness in U8: replace specific lines with `grep -n wgmesh.dev`.
- Terminology drift "flat": treat "flat URL shape" + "single-wildcard cert" as two distinct decisions in any future doc.
- `cr_*` API key UX as control plane (positioning) — flag for cloudroof.eu landing copy: lead with "managed edge service" framing, downplay "control plane" connotations.
- Flat DNS shape locks customer URLs (Phase B migration cost) — covered by 90-day dual-derivation window.
- 10-min time-to-URL "minutes wedge" risk — the Show HN reel script SHOULD lead with a running URL and back-wind into setup; "minutes" SLA applies post-payment, not post-`wgmesh init`.
- Plan-time budget: 6–8 weeks realistic, 4–6 weeks stretch (no integration discoveries), 8 weeks buffer ceiling.
- HKDF `v1` info-string versioning is performative without `derivation_scheme_version` schema column. Phase A.1 rotation introduces both.
- Default token-gate breaks browsable demo — Show HN reel uses `--public` for the demo URL OR demonstrates curl-with-Bearer.
- Competitor pacing — Cloudflare/Tailscale/ngrok release-cycle risk: track during Phase A weeks 4-6, surface adjustments at planning sync.

---

## High-Level Technical Design

> *This illustrates the intended approach and is directional guidance for review, not implementation specification. The implementing agent should treat it as context, not code to reproduce.*

### Signup + service-add sequence (covers F1, hash-storage architecture)

```mermaid
sequenceDiagram
  participant Op as Operator (A1)
  participant CLI as wgmesh CLI (A2)
  participant LH as Lighthouse (A3)
  participant Polar as Polar.sh (A5)
  participant Edge as Edge Node (A4)

  Op->>CLI: wgmesh signup --email e
  CLI->>LH: POST /v1/orgs/signup {email}
  LH->>LH: enforce email-unique on org table
  LH->>Polar: POST /v1/checkouts {external_customer_id, products:[€5/mo]}
  Polar-->>LH: 201 Checkout{url, id}
  LH-->>CLI: 200 {api_key:"cr_...", checkout_url}
  CLI->>CLI: SaveAccount(cr_*)
  CLI-->>Op: print checkout_url
  Op->>Polar: open URL, complete payment
  Polar->>LH: POST /webhooks/polar (subscription.active)
  LH->>LH: verify HMAC, dedupe via webhook-id ledger, mark org active
  Op->>CLI: wgmesh service add ollama :11434
  CLI->>CLI: token = hex(HKDF(secret, "wgmesh-service-token-v1:ollama", 32))
  CLI->>LH: POST /v1/sites {name, mesh_ip, port, origin_auth_token: token}
  LH->>LH: gate on active subscription (R8)
  LH->>LH: hash := sha256(token); persist site with origin_auth_token_hash; discard token
  LH->>LH: regenerate Caddyfile
  Edge->>LH: GET /v1/xds/caddyfile (every 30s, with EDGE_TOKEN)
  Edge->>Edge: caddy validate, reload
  CLI-->>Op: print https://ollama-<mesh-id>.cloudroof.eu
```

### Edge request gating (covers F1 + R12 — hash-compare architecture)

```mermaid
sequenceDiagram
  participant Client
  participant Edge
  participant LH as Lighthouse
  participant Origin as Operator's mesh-IP origin

  Client->>Edge: HTTPS GET https://ollama-<mesh-id>.cloudroof.eu (Authorization: Bearer T)
  Edge->>LH: forward_auth GET /v1/edge/auth (Authorization: Bearer T) [cached 60s + stale-5m]
  LH->>LH: lookup site by hostname → origin_auth_token_hash (or is_public=true)
  alt is_public=true
    LH-->>Edge: 200 (X-Org-ID, X-Service-Name)
  else token_match
    LH->>LH: presented_hash := sha256(T); subtle.ConstantTimeCompare(presented_hash, stored_hash)
    LH-->>Edge: 200 (X-Org-ID, X-Service-Name) or 401
  end
  alt 200
    Edge->>Origin: reverse_proxy via wgmesh tunnel
    Origin-->>Edge: response
    Edge-->>Client: response
  else 401
    Edge-->>Client: 401
  end
```

---

## Implementation Units

> Stable U-IDs assigned sequentially. ce-work references blockers and verification by U-ID; do not renumber on plan edits. **Splits during this plan revision: U5/U6 retained their original IDs; new sibling units took U9, U10, U11.**

- U1. **lighthouse-go SDK v0.2.0**

**Goal:** Ship a backwards-compatible SDK release (additive `NewClientWithOptions`; `NewClient(baseURL, apiKey)` unchanged) supporting managed-domain, signup endpoint, origin-auth-token field, typed errors, and per-call idempotency-key header. wgmesh's `go.mod` bumps to v0.2.0 in lockstep.

**Requirements:** R3, R9, R12, R16, R17

**Dependencies:** None (foundational; everything else depends on this)

**Files (target repo: lighthouse-go):**
- Modify: `lighthouse-go:client.go` — add `NewClientWithOptions(baseURL, apiKey string, opts ...Option)` with `WithManagedDomain(string)` and `WithIdempotencyKey(string)`. Keep `NewClient` as a shim returning `NewClientWithOptions(baseURL, apiKey)`. `DiscoverLighthouse` reads managed-domain from `Client` struct (default `cloudroof.eu`). New `Signup(ctx, email)` method calling `POST /v1/orgs/signup`. New typed errors `ErrSubscriptionInactive` (with `PortalURL`) and `ErrEmailAlreadyRegistered`.
- Modify: `lighthouse-go:types.go` — add `OriginAuthToken string` and `IsPublic bool` to `CreateSiteRequest`. New `SignupRequest`/`SignupResponse` types.
- Modify: `lighthouse-go:README.md` — domain examples flip `wgmesh.dev` → `cloudroof.eu`. Document additive constructor.
- Test: `lighthouse-go:client_test.go` — option pattern, signup happy path, subscription-inactive typed-error, email-collision typed-error, idempotency-key passthrough, backwards-compat: existing `NewClient` works unchanged.

**Approach:**
- Pre-tag step: `go list -m -versions github.com/atvirokodosprendimai/lighthouse-go` and `pkg.go.dev/github.com/atvirokodosprendimai/lighthouse-go?tab=importedby` to verify the only-known-consumer assumption. If external importers exist, additive constructor is mandatory (already planned).
- Functional-options on `NewClientWithOptions`. Existing two-arg `NewClient` becomes a one-line wrapper.
- `Idempotency-Key` is opaque per-request; accept on a per-call option (`WithIdempotencyKey(string)`) so callers can mint fresh UUIDs without holding a Client-level key.
- `ErrSubscriptionInactive` returned for HTTP 402 with problem-detail title `subscription_inactive`; SDK exposes `PortalURL` from problem-detail's `detail`.
- `ErrEmailAlreadyRegistered` returned for HTTP 409 with title `email_already_registered`.

**Patterns to follow:**
- Existing single-file SDK shape; preserve.

**Test scenarios:**
- Happy path: `NewClient` (legacy) with no opts uses default `cloudroof.eu` → `DiscoverLighthouse` returns `https://lighthouse.<mesh-id>.cloudroof.eu`.
- Happy path: `NewClientWithOptions(..., WithManagedDomain("example.test"))` propagates.
- Happy path: `Signup(ctx, "user@example.com")` with mock 200 → returns `{api_key, checkout_url}`.
- Happy path: `CreateSite` with `OriginAuthToken` and `IsPublic=false` field passes through in JSON body.
- Edge case: `WithIdempotencyKey("uuid-1")` sets the header; same key on retry collapses; new key creates a new logical write.
- Error path: HTTP 402 with `{title:"subscription_inactive", detail:"https://..."}` → `errors.Is(err, ErrSubscriptionInactive)` true and `PortalURL` populated.
- Error path: HTTP 409 with `{title:"email_already_registered"}` → `errors.Is(err, ErrEmailAlreadyRegistered)` true.
- Error path: HTTP 5xx → wrapped error preserves response body.
- `Covers AE1.` (signup + create-site happy path), `Covers AE2.` (subscription-inactive typed error).

**Verification:**
- `go test ./...` green in lighthouse-go.
- `pkg.go.dev/.../?tab=importedby` checked; importer count documented in commit message.
- A v0.2.0 git tag pushes; `go list -m github.com/atvirokodosprendimai/lighthouse-go@latest` resolves to v0.2.0 (allow ~5 min for proxy.golang.org indexing).
- **Gate:** U2/U3/U4/U5/U6/U9/U10 must not start until `go list -m github.com/atvirokodosprendimai/lighthouse-go@v0.2.0` returns successfully from a fresh module cache.

---

- U2. **`pkg/crypto/service_token.go` — non-modifying HKDF token derivation**

**Goal:** Add `DeriveServiceToken(secret []byte, serviceName string) (string, error)` returning a 32-byte hex-encoded token, used by the wgmesh CLI at `service add` time. **Lighthouse never imports this function.** `pkg/crypto/derive.go` is NOT modified.

**Requirements:** R12

**Dependencies:** None (parallel-safe with U1)

**Files (target repo: wgmesh):**
- Create: `pkg/crypto/service_token.go`
- Test: `pkg/crypto/service_token_test.go`

**Approach:**
- Use Go 1.25 stdlib `crypto/hkdf`: `hkdf.Key(sha256.New, secret, nil, info, 32)`. Salt nil because mesh secret is already high-entropy 32 bytes.
- HKDF info string: `"wgmesh-service-token-v1:" + serviceName`. The `v1` suffix is reserved for future migration but does not by itself enable rotation; rotation requires a per-service nonce + schema column (Phase A.1, deferred).
- Validate `len(secret) >= MinSecretLength` (re-uses existing `MinSecretLength` constant from `derive.go` via package-level read access — does not modify the file).
- Return hex-encoded 32-byte token (64 chars). Constant-time compare belongs to the verifier (lighthouse, comparing hashes), not this derive function.

**Execution note:** Test-first. Write fixtures with known-good HKDF outputs (compute once, paste into table-driven test) before writing the implementation.

**Technical design:**

```
package crypto

import (
    "crypto/hkdf"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
)

const hkdfInfoServiceTokenPrefix = "wgmesh-service-token-v1:"

func DeriveServiceToken(secret []byte, serviceName string) (string, error) {
    if len(secret) < MinSecretLength {
        return "", fmt.Errorf("secret must be at least %d bytes", MinSecretLength)
    }
    info := hkdfInfoServiceTokenPrefix + serviceName
    out, err := hkdf.Key(sha256.New, secret, nil, info, 32)
    if err != nil {
        return "", fmt.Errorf("derive service token %q: %w", serviceName, err)
    }
    return hex.EncodeToString(out), nil
}
```

> *Directional guidance for review, not implementation specification.*

**Patterns to follow:**
- `pkg/crypto/membership.go` mirrors stdlib-crypto-only patterns and explicit `MinSecretLength` validation. Use the same shape.

**Test scenarios:**
- Happy path: deterministic — same `(secret, serviceName)` → identical token across calls.
- Happy path: distinct service names → distinct tokens. (Three table rows, expected hex computed once and pasted.)
- Edge case: `len(secret) < MinSecretLength` → returns error, no token leak.
- Edge case: empty service name → still produces a token (info = "wgmesh-service-token-v1:"). Acceptable; the CLI rejects empty names upstream.
- Edge case: 32-byte mesh secret produces a 64-character hex string.
- `Covers AE5.` — token derivation for non-`--public` path.

**Verification:**
- `go test ./pkg/crypto/...` green.
- `go test -race ./pkg/crypto/...` green.
- `git diff pkg/crypto/derive.go` empty.

---

- U3. **wgmesh CLI — Phase A bundle (token wiring + domain rewrite + R3/R5/R6 fixes + `--public`)**

**Goal:** One coordinated edit landing all CLI changes for Phase A: derive HKDF token at `service add` and post via SDK's `OriginAuthToken` field, flip `managedDomain` to `cloudroof.eu`, surface `subscription_inactive` typed error with portal URL, fix `service list` fallback indicator (R5), fix `service remove` to preserve local entry on Lighthouse failure (R6), add `--public` flag with confirmation prompt + audit + `[PUBLIC]` indicator in `service list`.

**Requirements:** R1 (already met), R2, R3, R5, R6, R12, R17

**Dependencies:** U1 (SDK v0.2.0), U2 (service-token derivation)

**Files (target repo: wgmesh):**
- Modify: `service.go` — replace `managedDomain = "wgmesh.dev"` (line 25). Add `--public` and `--yes` flags to `serviceAddCmd` flagset (lines 56-64). Replace the three `lighthouse.DiscoverLighthouse` call-sites (`service.go:125, 223, 321`) with the SDK option pattern. In `serviceAdd`: when `--public` is NOT set, derive token via `crypto.DeriveServiceToken(secret, name)` and pass on `CreateSiteRequest` with `IsPublic=false`; when `--public` IS set, skip derivation entirely, pass `OriginAuthToken=""` and `IsPublic=true`. `--public` triggers a confirmation prompt: "Public origin: any unauthenticated client can hit your service. Continue? [y/N]". Skip prompt if stdin is non-tty AND `--yes` is set. In `serviceList`: append `(local cache, Lighthouse unreachable)` to fallback rows; show `[PUBLIC]` next to public services. In `serviceRemove`: return early with a clear error when Lighthouse is unreachable; do NOT remove the local entry. Match `errors.Is(err, lighthouse.ErrSubscriptionInactive)` and print a multi-line message including `err.PortalURL`.
- Modify: `pkg/mesh/services.go` — extend `ServiceEntry` with `LastStatus string`, `LastChecked time.Time`, `IsPublic bool` (omitempty for backwards compat).
- Modify: `pkg/mesh/account.go` — extend `AccountConfig` with `SubscriptionStatus string`, `LastStatusCheck time.Time`, `PortalURL string` (omitempty).
- Modify: `go.mod` / `go.sum` — bump `lighthouse-go v0.1.0` → `v0.2.0`. Run `make deps` (= `go mod tidy`) after.
- Modify: `service_test.go` — flip every `wgmesh.dev` fixture to `cloudroof.eu` (lines 75-76, 224, 248, 266-267). Add tests covering token wiring, `--public` opt-out, `--public --yes` non-tty path, R5 fallback indicator, R6 preserve-on-failure, R3 typed-error display, R3+R9 typed-error display for email-already-registered.
- Modify: `pkg/mesh/services_test.go` — domain fixtures (lines 29, 57-58); add `IsPublic` test.

**Approach:**
- Rewrite the three SDK call-sites first (mechanical; per the Goose-pipeline meta-lesson, `git diff --stat` should show 3 hits in `service.go`).
- `--public` is exclusive of token derivation: the CLI does NOT derive a token when `--public`. This avoids the "wasted CPU but safer" path; the server-side `is_public` flag carries the decision. Audit happens server-side in U9.
- R3 typed-error path: SDK returns `ErrSubscriptionInactive` (per U1). Match with `errors.Is`, print the portal URL on a separate line so it's copy-pasteable.
- R5 indicator on fallback rows: use the existing `fromLighthouse` bool at `service.go:218`; when false, append `" (local cache, Lighthouse unreachable)"` to the `Status` column.
- R6 fix: today `service.go:326-328` removes the local entry with a warning; change to return an error and leave the local file untouched.
- Cache `SubscriptionStatus` and `PortalURL` in `account.json` after each successful call so offline `service add` invocations can surface the right error message without a Lighthouse round-trip.

**Patterns to follow:**
- `pkg/mesh/account.go` atomic temp-file rename (mode 0600).
- Three-group import ordering (stdlib / external / internal) per AGENTS.md.

**Test scenarios:**
- Happy path: `service add ollama :11434` (no `--public`) with active subscription returns `https://ollama-<mesh-id>.cloudroof.eu`; SDK request body includes `origin_auth_token: <64-hex>` and `is_public: false`. Token NOT logged.
- Happy path: `service add ollama :11434 --public` (after confirmation) succeeds; SDK request body includes `origin_auth_token: ""` and `is_public: true`.
- Happy path: `service list` with reachable Lighthouse shows live rows; public services tagged `[PUBLIC]`.
- Edge case: `--public` without `--yes` in tty → prompt; user types `n` → abort, no API call.
- Edge case: `--public --yes` in non-tty (CI) → skip prompt, API call.
- Edge case: `--public` in non-tty WITHOUT `--yes` → reject with error message, exit non-zero.
- Error path: HTTP 402 from Lighthouse → CLI prints `Subscription inactive. Manage your subscription at: <portal_url>` with non-zero exit.
- Error path: HTTP 409 from Lighthouse on signup → CLI prints `Email already registered. Sign in or use a different email.` with non-zero exit.
- Error path: `service list` with Lighthouse unreachable → each cached row ends `(local cache, Lighthouse unreachable)`, exit 0.
- Error path: `service remove` with Lighthouse unreachable → prints clear error, leaves `services.json` entry intact, exit non-zero.
- Integration: `service add` happy path persists `LastStatus` and `IsPublic` in `services.json`.
- `Covers AE1.` (happy path), `Covers AE2.` (subscription lapse), `Covers AE3.` (Lighthouse-unreachable fallback indicator), `Covers AE5.` (token-gated origin / `--public`).

**Verification:**
- `make test` and `make build` green.
- `go test -race ./...` green.
- Integration test against a mock Lighthouse exercises happy path end-to-end.

---

- U4. **wgmesh CLI — `signup.go`**

**Goal:** New `wgmesh signup --email <e>` subcommand calling SDK `Signup(email)`, persisting the returned `cr_*` to `account.json`, and printing the Polar.sh checkout URL.

**Requirements:** R9

**Dependencies:** U1 (SDK v0.2.0)

**Files (target repo: wgmesh):**
- Create: `signup.go`
- Modify: `main.go` — add `case "signup": signupCmd(); return` to the dispatch switch (lines 39-78).
- Modify: `main_test.go` — smoke test that builds `/tmp/wgmesh-test` and exec's `wgmesh signup --help`; asserts exit 0 and help text mentions `--email`.
- Test: `signup_test.go`

**Approach:**
- Flagset: `--email` (required), `--secret` (or `WGMESH_SECRET` env), `--state-dir` (default `/var/lib/wgmesh`), `--lighthouse-url` (override).
- Validate email format minimally (contains `@`).
- Discover Lighthouse via SDK or use override.
- Call `client.Signup(ctx, email)` with an `Idempotency-Key` (UUID minted per invocation).
- On success: persist `cr_*` via `mesh.SaveAccount`. Print: `Account created. Complete payment at: <checkout_url>\n\nThen run: wgmesh service add <name> <addr>`.
- On `ErrEmailAlreadyRegistered`: surface the typed error message, non-zero exit.
- On other errors: print problem-detail body, non-zero exit.

**Patterns to follow:**
- `service.go` flagset shape and `resolveAccount` helper.
- `main.go:639-670` — `handleAccountFlag` / `saveAccountAPIKey` for the persistence path.

**Test scenarios:**
- Happy path: mock SDK returns `{api_key:"cr_test", checkout_url:"https://..."}` → `account.json` created with mode 0600, stdout includes the checkout URL.
- Edge case: `--email` missing → flag-parse error, exit 2.
- Edge case: invalid email format → CLI-level error before SDK call.
- Error path: SDK returns `ErrEmailAlreadyRegistered` → CLI prints the dedicated message with hint, exits non-zero.
- Error path: SDK returns 5xx → CLI prints body, exits non-zero.
- Smoke: built binary `wgmesh signup --help` exits 0 with usage text.
- `Covers AE1.` (signup is the F1 starting point).

**Verification:**
- `go test ./... -run TestSignup` green.
- `make build && /tmp/wgmesh-test signup --help` exits 0.

---

- U5. **lighthouse server — Polar foundation (webhook + reconcile + checkout client)**

**Goal:** Three Polar.sh server-side primitives: webhook handler with Standard-Webhooks HMAC verification + webhook-id ledger, reconcile cron pulling Polar's list endpoints (1-min during webhook-disabled state, 5-min otherwise), and a thin Polar checkout-client wrapper. **No signup endpoint or site-creation gate in this unit; those move to U9.**

**Requirements:** R8, R14

**Dependencies:** U1 (SDK v0.2.0)

**Files (target repo: lighthouse):**
- Create: `lighthouse:internal/polar/webhook.go` — Standard Webhooks signature verification (~30 LOC). `POLAR_WEBHOOK_SECRET` env accepts comma-separated list; verification accepts any matching signature.
- Create: `lighthouse:internal/polar/reconcile.go` — cron loop calling Polar's `GET /v1/subscriptions` and `GET /v1/orders` with `modified_at` filter (verify Polar API support during U5 sprint; if unsupported, fall back to full pagination + state-diff).
- Create: `lighthouse:internal/polar/checkout.go` — `POST /v1/checkouts/` wrapper.
- Create: `lighthouse:internal/handlers/webhook_polar.go` — `POST /webhooks/polar` handler.
- Create: `lighthouse:internal/store/migrations/0001_add_origin_auth_token_hash.sql` — site-record schema migration: add `origin_auth_token_hash bytea`, `is_public boolean default false`, `created_with_public boolean default false` (audit) columns. Create `webhook_id_ledger` table with `(webhook_id text primary key, processed_at timestamptz, expires_at timestamptz)` and an expiry cleanup index.
- Modify: `lighthouse:cmd/lighthouse/main.go` — register webhook route, wire env (`POLAR_WEBHOOK_SECRET` comma-sep, `POLAR_API_TOKEN`), start reconcile cron with state-aware interval.
- Test: `lighthouse:internal/polar/webhook_test.go`, `reconcile_test.go`, `internal/handlers/webhook_polar_test.go`, schema-migration smoke test.

**Approach:**
- Webhook verification: base64-decode each `whsec_`-prefixed secret, HMAC-SHA256 over `id.timestamp.body`, multi-signature support for rotation, 5-min replay window via `webhook-timestamp` check, idempotency via `webhook-id` ledger (Postgres, 24h TTL).
- Reconcile cron: list subscriptions and orders modified in `[last_successful_reconcile_at − 10min, now]` (or full pagination fallback). Upsert by Polar resource ID (idempotent). Track `last_successful_reconcile_at` and `last_webhook_received_at`. Switch interval to 1-min when no webhook in N minutes (default 15) → polls Polar webhook-health endpoint to confirm; on disabled state, increases reconcile cadence and surfaces an alert.
- Reconcile is source of truth on conflict; webhook is a fast-path hint.
- Idempotent upsert pattern for site rows is already in lighthouse — apply to subscription/order rows.

**Patterns to follow:**
- Lighthouse's existing problem-detail mux for typed error responses.
- Idempotent upsert pattern for site rows.

**Test scenarios:**
- Happy path: webhook with valid signature (single sig) → 200, org marked `active`.
- Happy path: webhook with rotated signatures (old + new in env) → both verify; idempotent on retry via ledger.
- Edge case: timestamp older than 5 min → rejected with 4xx.
- Edge case: timestamp in future > 30s → rejected.
- Edge case: replay of already-processed `webhook-id` → 200 + no-op (verified via ledger).
- Edge case: dedup ledger row expired (24h+1) → replay processes again. Documented but not blocked.
- Error path: missing/malformed signature header → 401.
- Error path: Polar API returns 5xx during reconcile → loop continues, error logged with backoff.
- Edge case: webhook-disabled state detected → reconcile flips to 1-min interval; metric exposed.
- `Covers AE1, AE2.` (paid signup → active; lapse → inactive — at the webhook layer; signup endpoint per U9).

**Verification:**
- `go test ./... -race` green in lighthouse repo.
- Schema migration runs cleanly on a fresh Postgres; existing rows preserved.
- Webhook stub server exercises HMAC verification end-to-end.

---

- U6. **lighthouse server — Caddyfile generator**

**Goal:** `GET /v1/xds/caddyfile` endpoint that emits per-org sites with `forward_auth` blocks (omitted for `is_public=true` sites), respects `subscription_status=active` (omits inactive orgs), and validates against `caddy validate` in tests. **No edge-auth or edge-check endpoint in this unit; those move to U10.**

**Requirements:** R4, R10, R11

**Dependencies:** U5 (subscription state + site rows with `origin_auth_token_hash`/`is_public`)

**Files (target repo: lighthouse):**
- Create: `lighthouse:internal/handlers/xds_caddyfile.go`
- Modify: `lighthouse:cmd/lighthouse/main.go` — register route on `/v1/xds/caddyfile` with EDGE_TOKEN middleware (per R18; middleware itself lands in U10).
- Test: `lighthouse:internal/handlers/xds_caddyfile_test.go` — integration test running `caddy validate` against generator output (requires Caddy 2.9.x with caddy-dns/desec in CI).

**Approach:**
- Caddyfile structure (high-level):
  ```
  {
    email ops@cloudroof.eu
    acme_dns desec {env.DESEC_TOKEN}
  }

  *.cloudroof.eu, cloudroof.eu {
    tls {
      dns desec {env.DESEC_TOKEN}
    }
    @site_<orgID>_<svc> host <svc>-<mesh-id>.cloudroof.eu

    # Token-gated site (is_public=false)
    handle @site_<orgID>_<svc> {
      forward_auth lighthouse.cloudroof.eu {
        uri /v1/edge/auth
        copy_headers Authorization X-Org-ID X-Service-Name
        cache 60s stale-while-error 5m
      }
      reverse_proxy <mesh-ip>:<port> {
        transport http {
          dial_timeout 2s
          response_header_timeout 5s
        }
      }
    }

    # Public site (is_public=true) — same handle, no forward_auth
    handle @site_<orgID>_<svc_public> {
      reverse_proxy <mesh-ip>:<port> { transport http { dial_timeout 2s; response_header_timeout 5s } }
    }

    handle { respond "Unknown service" 404 }
  }
  ```
- Generator pulls only sites whose org has `subscription_status=active`. Sites for inactive orgs are dropped → next config-pull cycle (max 30s) removes them at the edge (R4 + R11).
- `transport http { dial_timeout 2s; response_header_timeout 5s }` so R11's 502-within-5s outcome falls out naturally.

**Patterns to follow:**
- Lighthouse's existing `/v1/xds/...` endpoint shape if present.

**Test scenarios:**
- Happy path: org with one active token-gated site → generator emits one `@site_*` matcher + handle with forward_auth.
- Happy path: org with one `is_public=true` site → handle has no forward_auth.
- Happy path: org marked `inactive` → generator omits all its sites.
- Edge case: zero active sites → emits a deny-all default handle, valid Caddyfile.
- Integration: `caddy validate --adapter caddyfile` accepts the generated output.
- `Covers AE4.` (502 — verified via the `transport` directive).
- `Covers F4 / R4.` (subscription lapse propagation).

**Verification:**
- `go test ./... -race` green.
- `caddy validate` against generator output green (CI runs Caddy 2.9.x).

---

- U7. **lighthouse server — public-TLS bootstrap**

**Goal:** Get `lighthouse.cloudroof.eu` reachable over public HTTPS without requiring a pre-existing wgmesh tunnel. Caddy in front of the existing `:8443` listener with a single-domain LE cert.

**Requirements:** R7

**Dependencies:** U5, U6, U10 (nothing to terminate TLS for until those handlers exist)

**Files (target repo: lighthouse):**
- Create: `lighthouse:deploy/public/Caddyfile` — single-domain reverse-proxy template.
- (See U8 for the wgmesh-repo `deploy/lighthouse/setup.sh` updates.)

**Approach:**
- Caddy in front, single-domain LE cert via HTTP-01 (no DNS provider needed for a single-domain cert at `lighthouse.cloudroof.eu`).
- Caddy reverse-proxies to `localhost:8443` (the existing Lighthouse listener).
- Public TLS termination on Caddy means Lighthouse keeps its current internal HTTP/HTTPS as-is.

**Test scenarios:**
- Happy path: external `curl https://lighthouse.cloudroof.eu/v1/orgs/signup` reaches Lighthouse.
- Edge case: certificate renewal succeeds without manual intervention.

**Verification:**
- `curl -sS -o /dev/null -w "%{http_code}\n" https://lighthouse.cloudroof.eu/healthz` returns 200 from any external host.
- LE certificate visible in Caddy's data dir, expiry > 30 days out.

---

- U8. **deploy scripts + DNS + Polar dashboard**

**Goal:** Close out infrastructure plumbing across deploy scripts in the wgmesh repo, configure deSEC DNS, and provision the Polar.sh product/webhook. Pre-flight checks fail fast on missing env (no silent reload swallowing).

**Requirements:** R7, R10, R13, R14, R18

**Dependencies:** U5 (Polar env wiring), U7 (Lighthouse public Caddy), U10 (EDGE_TOKEN middleware)

**Files (target repo: wgmesh):**
- Modify: `deploy/lighthouse/setup.sh` — drop "WireGuard mesh already running on this node" prereq comment (line 6). Add Caddy install + `Caddyfile` from U7 in front of Lighthouse on `:8443`. Wire `POLAR_WEBHOOK_SECRET` (comma-sep) and `POLAR_API_TOKEN` env via `EnvironmentFile=/etc/lighthouse/polar.env` (mode 0600, owner lighthouse). Add LE single-domain cert provisioning on first boot. **Pre-flight check:** fail fast if `polar.env` missing or empty before starting lighthouse.service.
- Modify: `deploy/edge/setup.sh` — drop "WireGuard mesh already running on this node" prereq (line 7). Replace `apt install caddy` (lines 25-30) with `xcaddy build vN.N.N --with github.com/caddy-dns/desec` (Caddy 2.9.x verified-fix release; HTTP-01 fallback documented). Add `DESEC_TOKEN` env via `EnvironmentFile=/etc/caddy/desec.env` (mode 0600). Add `EDGE_TOKEN` env via `EnvironmentFile=/etc/wgmesh-edge/token.env` (mode 0600). Add `Authorization: Bearer $EDGE_TOKEN` to the config-pull `curl`. **Pre-flight check:** fail fast if `desec.env` or `token.env` missing or empty before starting Caddy. Replace `|| true` reload-swallowing with logged error + exit-non-zero on persistent failure.
- Modify: `AGENTS.md` — Go version drift fix (says 1.23, actual 1.25).
- Modify: `eidos/spec - service cli - register local services for managed ingress via lighthouse.md` — replace every `wgmesh.dev` reference with `cloudroof.eu` (use `grep -n wgmesh.dev` to locate). Update authentication section to reflect `--account` flow stays in Phase A; key-challenge is Phase B.
- Modify: `eidos/spec - first-customer - roadmap to first paying customer.md` — domain rewrite where `wgmesh.dev` appears.
- Modify: `ROADMAP.md` — annotate items #6 and #8 as Phase A; clarify item #7 as in-progress.
- Modify: `FEATURE_MATRIX.md` line 135 — flip `wgmesh service add CLI` from `📋` to `🔧 in-flight (Phase A)`.

**Files (target repo: lighthouse-go):**
- Modify: `lighthouse-go:client.go` line 174, 185 — covered by U1 (cross-repo domain sweep).
- Modify: `lighthouse-go:README.md` lines 17, 21, 27, 49-51 — domain examples.

**Out-of-tree (not committed; configuration deliverables):**
- **deSEC DNS:** add `*.cloudroof.eu` A/AAAA → edge node IPv4/IPv6; `lighthouse.cloudroof.eu` A/AAAA → lighthouse box IP. Mint a deSEC token: `perm_create_domain=false`, `perm_delete_domain=false`, `perm_manage_tokens=false`, `allowed_subnets=[<edge-egress-ip>/32, <edge-egress-ipv6>/128]`, `max_age=365d`, `max_unused_period=14d`. Per-rrset policy: deny-default + allow `domain=cloudroof.eu, type=TXT, perm_write=true` (no glob `subname` support; this is broader than apex-only and is the accepted Phase A scope). Deposit token in `/etc/caddy/desec.env`.
- **Polar.sh dashboard:** create EU MoR product `cloudroof Phase A`, recurring price €5/mo (single SKU). Configure webhook endpoint `https://lighthouse.cloudroof.eu/webhooks/polar` with secret rotated to a fresh `whsec_*`. Generate Organization Access Token with `subscriptions:read`, `orders:read`, `checkouts:write` scopes (Phase A drops `events:write` since no metered ingest). Deposit secrets in `/etc/lighthouse/polar.env`.
- **Edge token:** mint via lighthouse admin API or CLI at edge-provision time; deposit in `/etc/wgmesh-edge/token.env`.

**Approach:**
- Idempotent reprovisioning: `xcaddy build` cache + version pin keeps deploy reproducible.
- Pre-flight checks make missing-env failures loud, not silent.
- AGENTS.md / FEATURE_MATRIX / ROADMAP edits are doc-only; commit separately for clean revert if needed.

**Patterns to follow:**
- Existing `setup.sh` idiom (apt install, systemd unit, EnvironmentFile pattern).

**Test scenarios:**
- Smoke (manual, post-deploy): provision a fresh Hetzner box; run `deploy/edge/setup.sh`; `caddy version` shows 2.9.x with `dns.providers.desec` listed; systemd unit running.
- Smoke: `curl -sS https://lighthouse.cloudroof.eu/healthz` returns 200 from any external host.
- Smoke: create test org → service-add → certificate issues for `<svc>-<mesh-id>.cloudroof.eu` within 60s → external HTTPS request succeeds.
- Edge case: deSEC token rate-limited mid-renewal → `journalctl -u caddy` shows the 429 with backoff; cert renews on next interval.
- Edge case: missing `desec.env` at edge-provision time → setup.sh fails fast with clear message before Caddy starts.
- Doc-edit verification: `grep -r wgmesh.dev eidos/` returns zero hits after the sweep.

**Verification:**
- All `wgmesh.dev` references in `eidos/` and `ROADMAP.md` / `FEATURE_MATRIX.md` are gone (or annotated as legacy).
- `deploy/edge/setup.sh && deploy/lighthouse/setup.sh` rerun on a fresh box without errors.
- AE1 timer (`wgmesh init` → reachable URL) under 10 min on a real Hetzner provision (Phase A acknowledges the relaxed bound).

---

- U9. **lighthouse server — signup endpoint + site-creation gate**

**Goal:** Self-serve `POST /v1/orgs/signup` that mints `cr_*`, creates a Polar checkout via U5's client, returns `{api_key, checkout_url}`. Email is unique on the org table; on Polar duplicate-customer error, return 409 with `email_already_registered`. Gate `POST /v1/sites` on org `subscription_status=active`. Persist `origin_auth_token_hash` (SHA-256 of CLI-supplied token) and `is_public`/`created_with_public` audit fields. Emit structured audit log on `is_public=true` site creation.

**Requirements:** R3, R9, R12

**Dependencies:** U5 (Polar checkout client + schema migration), U6 (Caddyfile generator consumes the new fields)

**Files (target repo: lighthouse):**
- Create: `lighthouse:internal/handlers/signup.go` — signup endpoint.
- Modify: `lighthouse:internal/handlers/sites.go` (or equivalent existing handler) — site-creation gate on subscription state; SHA-256 the supplied `origin_auth_token` and persist hash; persist `is_public`/`created_with_public`; emit audit log row.
- Test: `lighthouse:internal/handlers/signup_test.go`, `sites_test.go` extensions.

**Approach:**
- Signup flow: validate email format → unique-check on org table → create org row → mint `cr_*` API key (Lighthouse's existing `internal/auth/keys.go` pattern) → call U5's checkout client with `external_customer_id=<orgID>`, `success_url=https://app.cloudroof.eu/billing/return?checkout_id={CHECKOUT_ID}` → persist Polar's checkout ID → return `{api_key, checkout_url}`. Wrap in transaction; on Polar duplicate-customer error, return HTTP 409 with `{title:"email_already_registered", detail:"<sign-in-hint>"}`.
- Site-creation gate: in the existing `POST /v1/sites` handler, look up the org's `subscription_status`. On anything other than `active`, return HTTP 402 with `{title:"subscription_inactive", detail:"<portal_url>"}`. Reuse existing problem-detail mux.
- Token hashing: `sha256(token)` happens server-side immediately on receipt; raw token is discarded after hashing. Lighthouse never persists the plaintext token.
- Audit on `is_public=true`: structured log entry with `org_id`, `service_name`, `mesh_id`, timestamp.

**Patterns to follow:**
- Existing `cr_*` Bearer middleware for the signup endpoint (signup itself is unauthenticated, but goes through the same router).
- Idempotent upsert pattern.

**Test scenarios:**
- Happy path: valid email → org created → checkout returned. `cr_*` written to org row.
- Happy path: site-create with `is_public=false` and `origin_auth_token` → row persists with `origin_auth_token_hash=sha256(token)`, `is_public=false`. Plaintext token NOT in DB.
- Happy path: site-create with `is_public=true` and empty `origin_auth_token` → row persists with no hash, audit log row emitted.
- Edge case: signup with duplicate email → HTTP 409 with `email_already_registered`. No org row created.
- Edge case: site-create with `is_public=true` AND non-empty `origin_auth_token` → reject with 400 (mutually exclusive).
- Error path: site-create against `inactive` org → HTTP 402 with portal URL.
- Integration: signup → checkout-create round-trip with mock Polar → returns valid URL.
- `Covers AE1, AE2.`

**Verification:**
- `go test ./... -race` green.
- E2E: stub Polar with `httptest.NewServer`; signup → mock webhook (U5) → service-add round-trip succeeds.
- Plaintext `origin_auth_token` does NOT appear in Postgres (assert via SQL grep in test setup).

---

- U10. **lighthouse server — edge auth endpoint + on-demand TLS ask + EDGE_TOKEN middleware**

**Goal:** `/v1/edge/auth` validates Bearer tokens by hashing presented value and constant-time-comparing against stored `origin_auth_token_hash`. EDGE_TOKEN middleware on `/v1/xds/*` routes. (`/v1/edge/check` for on-demand-TLS ask is dropped — flat DNS shape eliminates need.)

**Requirements:** R12, R18

**Dependencies:** U5 (schema), U6 (Caddyfile points to this endpoint)

**Files (target repo: lighthouse):**
- Create: `lighthouse:internal/handlers/edge_auth.go` — `GET /v1/edge/auth`.
- Create: `lighthouse:internal/middleware/edge_token.go` — `EDGE_TOKEN` Bearer middleware for `/v1/xds/*`.
- Modify: `lighthouse:cmd/lighthouse/main.go` — register `/v1/edge/auth`, mount middleware on `/v1/xds/*`.
- Test: `lighthouse:internal/handlers/edge_auth_test.go`, `internal/middleware/edge_token_test.go`.

**Approach:**
- `/v1/edge/auth`: extract hostname from `X-Forwarded-Host` (Caddy passes via `forward_auth`), parse `<svc>-<mesh-id>.cloudroof.eu`, look up site → org. If `is_public=true`, return 200 with `X-Org-ID`/`X-Service-Name` headers, no Bearer required. Else extract `Authorization: Bearer <hex>`, compute `sha256(decoded_hex)`, `subtle.ConstantTimeCompare` against stored `origin_auth_token_hash`. Return 200 + headers, or 401.
- EDGE_TOKEN middleware: simple Bearer check against a list of `cr_edge_*` tokens (one per provisioned edge). Per-edge token enables future per-edge revocation.
- Cache hint: handler is constant-time and DB-cheap (single indexed lookup); Caddy's `forward_auth cache 60s stale-while-error 5m` lives in U6's generator.

**Patterns to follow:**
- `crypto/subtle` constant-time compare.
- Existing Bearer middleware shape in lighthouse.

**Test scenarios:**
- Happy path: Bearer matches `origin_auth_token_hash` → 200 + `X-Org-ID`, `X-Service-Name` headers.
- Happy path: site marked `is_public=true` → 200 with headers, no Bearer required.
- Edge case: missing Authorization header on token-gated site → 401.
- Edge case: invalid Bearer (length, encoding) → 401 with no leak.
- Edge case: unknown hostname → 401.
- Edge case: EDGE_TOKEN missing or invalid on `/v1/xds/*` → 401.
- Edge case: Bearer for an `inactive` org — handler returns 401 (Caddyfile generator already drops the route, so this is defense-in-depth).
- Constant-time invariant: timing-resistance asserted via use of `subtle.ConstantTimeCompare` (linter-checked or asserted in test setup).
- `Covers AE5.` (token-gated origin).

**Verification:**
- `go test ./... -race` green.
- Hash invariant: `len(stored_hash) == 32`, never plaintext.

---

- U11. **Single-edge runbook + Show HN day on-call**

**Goal:** Operationalize the single-edge SPOF risk. Cold-spare image cadence, DNS-flip procedure, on-call rotation, cert-revocation recovery ETA bound. Concrete deliverables — not "deferred to planning."

**Requirements:** Operational risk mitigation (no R-ID; cross-cuts R7, R10, R13).

**Dependencies:** U7, U8 (target infra exists)

**Files (target repo: wgmesh):**
- Create: `docs/runbooks/single-edge-recovery.md` — ETA-bound runbook covering edge-host loss, deSEC outage, LE cert revocation, ACME renewal stall, kernel panic. Each scenario has a labeled trigger, rollback path, recovery SLA.
- Create: `docs/runbooks/show-hn-day.md` — pre-launch checklist (cert validity, edge health, lighthouse health, polar webhook test event, monitoring dashboards), on-call rotation (founder-only is acceptable for Phase A; document handoff to cofounder for sleep windows).
- Create: `deploy/edge/cold-spare/build.sh` (or similar) — script to build a cold-spare Hetzner image with current `xcaddy` + `caddy-dns/desec` + edge config skeleton. Output: a `.img` or snapshot ID stored at a known location.
- Modify: `deploy/edge/setup.sh` — emit a `recovery-notes.txt` post-install with edge-specific identifiers (mesh IPs, EDGE_TOKEN cert path, deSEC token path) for runbook reference.

**Approach:**
- Runbook is short and concrete. Each section: trigger ("you observe X"), action ("run Y, verify Z"), SLA ("under N minutes/hours"). No prose-heavy framing.
- Cold-spare image refreshed monthly via a scheduled job (Goose-pipeline candidate).
- DNS-flip: pre-staged secondary deSEC record set, swapped manually under a documented procedure.
- Cert-revocation: LE allows 5 duplicate certs/7d; cold-spare can issue immediately. ETA bound: 60 min in worst case (DNS propagation).

**Patterns to follow:**
- Existing `docs/dogfooding/stability-log.md` voice — terse, scenario-based.

**Test scenarios:**
- Tabletop drill: walk runbook with a stopwatch on a fresh cold-spare image. Document elapsed time. SLA: under 60 min from "edge dead" observation to "URL responding from spare".
- Cert revocation simulation: revoke staging cert (LE has staging environment), validate runbook recovers within ETA.
- DNS-flip simulation: change deSEC A record to spare IP, validate `dig` propagation < 5 min.

**Verification:**
- Runbook review by cofounder before Show HN.
- Tabletop drill completed at least once before launch; elapsed-time logged.
- Cold-spare image snapshot exists and is < 30 days old at launch.

---

## System-Wide Impact

- **Interaction graph:** wgmesh CLI ↔ lighthouse-go SDK ↔ lighthouse server ↔ Polar.sh API; lighthouse server → Caddyfile generator → edge box (config-pull); edge box → lighthouse `/v1/edge/auth` per request (cached 60s + stale-while-error 5min).
- **Error propagation:** SDK exposes typed errors (`ErrSubscriptionInactive`, `ErrEmailAlreadyRegistered`); CLI maps them to user messages with portal links and recovery hints; Lighthouse problem-detail bodies preserve enough context for SDK to reconstruct typed errors.
- **State lifecycle risks:** Polar webhook → org-state flip; reconcile cron → idempotent upsert. Webhook-id ledger (Postgres, 24h TTL) prevents replay; reconcile is source-of-truth on conflict. `service add` idempotency comes from `Idempotency-Key` on the SDK side. **Token-storage invariant: only SHA-256 hash persists at Lighthouse; plaintext token is discarded after hashing.**
- **API surface parity:** `lighthouse-go` v0.2.0 ships additive `NewClientWithOptions` constructor; legacy `NewClient` continues to work. Pre-tag importer-check via `pkg.go.dev` per R16.
- **Integration coverage:** end-to-end signup → checkout → webhook → service-add → public URL must be exercised against a stub Polar in CI and a real Polar sandbox before launch.
- **Unchanged invariants:** `pkg/crypto/derive.go` is untouched in Phase A. The OSS path (`init`, `join` without `--account`, `status`, `peers list`) makes zero Lighthouse calls — preserved by AE6. wgmesh decentralized mode behavior is unchanged.

---

## Risks & Dependencies

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Caddy 2.9.x DNS-01 regression unresolved at deploy time | Med | High | Verify against current 2.9.x release notes pre-U8; HTTP-01 fallback (per-domain certs hit LE rate limit only on signup, flat URL shape avoids fleet-wide reissuance). |
| Single edge box outage during Show HN | Med | High (brand) | U11 cold-spare runbook + tabletop drill; DNS-flip SLA bound at 60 min. Phase B adds true multi-edge. |
| Single shared `*.cloudroof.eu` cert SPOF (compromise/revocation/deSEC outage) | Low | Critical | U11 runbook covers cert-revocation recovery ETA. Phase B adds fallback CA path (ZeroSSL/Buypass). 5-duplicate-cert/7d LE allowance suffices for emergency reissuance. |
| Polar.sh webhook auto-disabled after 10 failures | Low | Med | 5-min reconcile cron switches to 1-min on webhook-disabled detection. AE2 SLA documented as `reconcile-interval + config-pull-interval` during webhook outages (max ~5.5min). |
| Polar webhook secret rotation drops events | Low | Med | `POLAR_WEBHOOK_SECRET` accepts comma-separated list; both old and new validate during rotation window without process restart. |
| LE rate-limit hit despite shared wildcard | Low | Med | Single shared `*.cloudroof.eu` issuance only on renewal. Monitor `letsencrypt.org` notices around `cloudroof.eu`. |
| `pkg/crypto/derive.go` accidental modification | Low | Med | U2 deliberately uses a sibling file. Pre-commit hook or PR-template checkbox to flag any touch to `derive.go`. |
| Polar customer collision via shared email | Low | Low | Email-unique constraint on org table + 409 on Polar duplicate-customer error (R9). |
| Goose pipeline produces a partial `package main` change | Low | Med | U3 cross-cuts root + `pkg/mesh/`; reviewer diff-checks plan vs `git diff --stat`. |
| Forward_auth cache miss-storm on Lighthouse outage | Med | Med | 60s cache + stale-while-error 5min default. Lighthouse-side health metric exposed for alerting. |
| EDGE_TOKEN compromise leaks Caddyfile (mesh IPs, port routing) | Low | Med | Per-edge tokens; rotation procedure documented. Caddyfile content is operationally visible already (every active site responds to lookup). |
| Phase A → Phase B token migration breaks early customers | Low | Med | 90-day dual-derivation window post-Phase-B launch. `derivation_scheme_version` column added at Phase A.1 token-rotation work. |
| lighthouse-go v0.2.0 breaks external importers | Low | Med | Additive `NewClientWithOptions`; `NewClient` shim preserved. Pre-tag pkg.go.dev importer check. |

---

## Documentation / Operational Notes

- **Public-facing copy:** cloudroof.eu landing repositioning needs to lead with Phase A's actual contract — `wgmesh service add` produces a public HTTPS URL via paid managed edge. Drop "key-challenge" / "p95 billing" claims until Phase B.
- **Show HN reel:** demo command sequence is `wgmesh init --secret`, `wgmesh signup --email <e>`, complete payment in browser, `wgmesh join --secret <s> --account <cr_*>`, `wgmesh service add ollama :11434`. Total target time ≤ 10 min (origin Success Criteria, Phase A relaxation from 5 min). **Reel framing:** open on the running URL; back-wind to setup. The 10-min number is post-`wgmesh init`; the "minutes" wedge applies to `service add → reachable URL` (~60s).
- **Operator-facing docs:** update `docs/quickstart.md` after U3 lands so the homelab→VPS path includes the cloudroof.eu signup steps.
- **Polar.sh metered ingestion** is deferred to Phase B. Phase A backfills history from edge access logs at Phase B planning time.
- **Sponsor-tier alignment:** Phase A €5/mo product mirrors GitHub Sponsors Contributor; FEATURE_MATRIX.md sponsor-tier section consistent post-edit.
- **Hash-storage architecture verbal explainer (for cloudroof.eu landing FAQ or operator docs):** "We never see your mesh secret. Your wgmesh CLI derives a per-service token locally and sends only the SHA-256 hash to our server. Even if our database is breached, attackers get hashes — not your secret, not your tokens. Plaintext tokens never live at rest on our infrastructure."

---

## Sources & References

- **Origin document:** [docs/brainstorms/2026-05-05-bet-a-show-hn-demo-requirements.md](../brainstorms/2026-05-05-bet-a-show-hn-demo-requirements.md)
- **STRATEGY.md** at repo root — Phase A maps to all three tracks.
- **ROADMAP.md** Horizon 1 items #1, #3, #6 land in Phase A; Horizons 2 #7, #8 partially pull forward.
- **`docs/solutions/`** four entries inform sequencing (NAT, custom-subnet, chaos-test, GoReleaser).
- **Polar.sh docs:** https://docs.polar.sh/ (webhook signing, checkout, EU MoR).
- **Caddy v2 docs:** https://caddyserver.com/docs/ (on-demand TLS, forward_auth, DNS-01).
- **Let's Encrypt:** https://letsencrypt.org/docs/rate-limits/.
- **deSEC:** https://desec.readthedocs.io/en/latest/auth/tokens.html.
- **Go 1.25 stdlib `crypto/hkdf`:** https://pkg.go.dev/crypto/hkdf.
- **Standard Webhooks:** https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md.
- **Caddy 2.8.x DNS-01 issue:** https://github.com/caddyserver/caddy/issues/6557.
