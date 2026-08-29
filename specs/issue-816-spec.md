# Specification: Issue #816 — Deploy privacy-friendly analytics (Umami) on cloudroof.eu with tracking snippet across all pages

## Classification
chore

## Problem Analysis

cloudroof.eu (the public marketing/product site for wgmesh) currently has no
first-party, privacy-friendly web analytics that the team controls. Today the
served HTML pages embed a **third-party** analytics loader — OpenPanel — using a
remote script from `https://openpanel.dev/op1.js` configured against an external
endpoint (`counter.hackrsvalv.com`). This setup has several problems:

1. **Privacy posture.** The product is marketed around privacy (WireGuard mesh,
   privacy mode, etc.). Relying on a third-party tracker host and a third-party
   collection domain undermines that message and exposes visitors to cross-site
   tracking. Umami is cookieless by default, does not fingerprint, and is
   GDPR/PECR friendly; running it first-party (on a `cloudroof.eu` subdomain)
   keeps all raw event data inside infrastructure we own.

2. **Ownership/availability.** The current collector lives on a personally-named
   host (`hackrsvalv.com`) outside the project's domain. There is no
   project-owned dashboard, no documented admin access, and no SLA. Issue #816
   explicitly asks to host Umami on `cloudroof.eu`.

3. **Coverage is inconsistent.** The OpenPanel snippet is present in only some of
   the served HTML files, and there is no single source of truth for the snippet.
   A repo-wide grep for `op1.js` / `window.op` finds it in:
   - `index.html` (repo-root landing page; also contains an exit-intent modal
     that calls `window.op("track", ...)` for `exit_modal_shown`,
     `exit_modal_dismissed`, `exit_modal_submitted`).
   - `public/index.html` (production-served landing page; same exit-modal
     `window.op` tracking calls).
   - `wgmesh.dev/index.html` (legacy/alternate domain landing page).
   - `evolution/wgmesh-cdn-slides.html` (presentation deck).
   - `docs/index.html` (docs/index landing page).

   The issue requires the Umami tracking snippet to be present **across all
   pages**.

4. **Custom-event coupling.** The exit-intent modal in `index.html` and
   `public/index.html` fires custom events through `window.op("track", ...)`.
   A naive snippet swap would silently break funnel analytics
   (modal shown / dismissed / submitted) because Umami's API is different
   (`umami.track(...)` / `umami.trackEvent(...)`). The migration must adapt these
   calls, not just replace the loader.

5. **No deployment artifact for Umami itself.** The repo has a `docker-compose.yml`
   for wgmesh nodes but nothing describing how the Umami server (app + Postgres)
   is stood up behind cloudroof.eu. An implementation agent needs a concrete
   deployment definition plus the reverse-proxy expectation so the snippet's
   `src` URL is resolvable.

Note: this issue is classified `chore` (infrastructure + content wiring, not a
user-facing feature or a defect in wgmesh's networking behavior). It is clearly
actionable; no info is missing to begin.

## Proposed Approach

Two parallel workstreams: **(A) deploy the Umami server** first-party on
cloudroof.eu, and **(B) instrument every served HTML page** with the Umami
snippet, adapting the existing custom-event calls.

### A. Deploy Umami server (first-party)

Umami is a Node.js app backed by PostgreSQL. Run it as containers behind the
existing edge that fronts cloudroof.eu.

1. **Add a compose file for Umami.** Create `deploy/umami/docker-compose.yml`
   with two services:
   - `umami-db`: PostgreSQL 16 (official image), a named volume for data
     persistence, a healthcheck, and a non-default DB name/user. The database
     password is sourced **only** from environment variables
     (`POSTGRES_PASSWORD`, `DATABASE_URL` / `UMAMI_DB_URL`) — never hardcoded.
   - `umami`: the official Umami image
     (`ghcr.io/umami-software/umami:postgresql-latest`), `depends_on` the
     healthy db, `environment` reading `DATABASE_URL` and `APP_SECRET` from the
     host environment, exposing its HTTP port on the internal container network
     only (not published to 0.0.0.0).
   Both services on a dedicated bridge network (e.g. `umami-net`). Add
   `restart: unless-stopped`. Pin/record the Umami image digest or tag so the
     deploy is reproducible.

2. **Reverse proxy / TLS.** Umami must be reachable at a first-party origin on
   cloudroof.eu, e.g. `https://stats.cloudroof.eu`. Document (in
   `deploy/umami/README.md`) that the existing cloudroof.eu edge/reverse proxy
   must:
   - terminate TLS for `stats.cloudroof.eu` (cert via the project's existing ACME
     flow),
   - proxy `/` to the `umami` service's internal port,
   - set `X-Forwarded-Proto: https` and `X-Forwarded-Host: stats.cloudroof.eu`
     so Umami generates correct asset/script URLs,
   - and send the Umami telemetry script from the **same** origin
     (`https://stats.cloudroof.eu/script.js`).
   Do **not** commit any private keys, certificates, `.env` files, or DNS
   provider tokens. Provide a `.env.example` with placeholder names only.

3. **Bootstrap + create the site.** After `docker compose up -d`, the first-run
   admin is created on the login screen (`admin`/`umami` default — must be
   changed immediately). Then add a "Website" for cloudroof.eu. Record only the
   resulting **Website ID** (a UUID, not secret) and the script origin in
   `deploy/umami/README.md` so the snippet can reference them. Admin credentials
   and the `APP_SECRET` / DB password are operator-managed secrets, never
   committed.

### B. Instrument all pages with the Umami snippet

4. **Define one canonical snippet.** In each HTML file's `<head>`, immediately
   before `</head>` (and after any existing metadata/meta tags), insert:

   ```html
   <!-- Umami analytics (first-party, cookieless, self-hosted on cloudroof.eu) -->
   <script defer src="https://stats.cloudroof.eu/script.js"
           data-website-id="REPLACE_WITH_CLOUDROOF_EU_WEBSITE_ID"></script>
   ```

   The implementation agent must substitute the real Website ID produced in step
   3 for `data-website-id`. Use the synchronous-looking `defer` form (Umami's
   recommended snippet) so pageviews fire reliably without a separate init call.

5. **Apply across all served HTML pages.** Add the snippet to **every** HTML file
   that is served from cloudroof.eu and is currently (or should be) tracked:
   - `index.html`
   - `public/index.html`
   - `wgmesh.dev/index.html`
   - `evolution/wgmesh-cdn-slides.html`
   - `docs/index.html`

6. **Remove the OpenPanel loader from all pages.** In each of the five files,
   delete the two lines:
   ```html
   <script src="https://openpanel.dev/op1.js" defer async></script>
   <script>
     window.op = window.op || function(...args) { (window.op.q = window.op.q || []).push(args); };
     window.op('init', { ... });
   </script>
   ```
   (the `<script>` init block up to and including its closing `</script>`). This
   removes the third-party dependency and the external collection domain.

7. **Adapt custom events in the exit-intent modal.** Both `index.html` and
   `public/index.html` contain an IIFE whose `track()` helper wraps OpenPanel:
   ```js
   function track(eventName, props) {
     try {
       if (typeof window === "object" && typeof window.op === "function") {
         window.op("track", eventName, props || {});
       }
     } catch (e) { /* analytics must never break the modal */ }
   }
   ```
   Replace the body so it calls Umami's API while preserving the same guard
   semantics (analytics must never break the modal) and the same event names:
   ```js
   function track(eventName, props) {
     try {
       if (typeof window === "object" && window.umami) {
         if (typeof window.umami.track === "function") {
           // Umami v2: track(name, props)
           window.umami.track(eventName, props || {});
         } else if (typeof window.umami.trackEvent === "function") {
           // Umami v1 fallback
           window.umami.trackEvent(eventName, props || {});
         }
       }
     } catch (e) { /* analytics must never break the modal */ }
   }
   ```
   The three existing call sites — `track("exit_modal_shown", {...})`,
   `track("exit_modal_dismissed")`, `track("exit_modal_submitted", {...})` — are
   left unchanged; only the helper is rewritten. Event names stay identical so
   historical funnel naming is preserved.

8. **Do not change anything else.** No CSS, layout, pricing text, links, modal
   behavior, or unrelated files. The change is strictly: remove OpenPanel, add
   Umami snippet, rewrite the one `track()` helper.

### Notes on privacy and public-repo safety

- Umami is cookieless and does not set a visitor ID cookie by default; no
  consent banner is required for the cookieless pageview/script configuration
  used here. (Operators should keep Umami's "Disable cookies" website setting on
  to preserve this property.)
- Nothing in the committed artifacts is a secret: the Website ID is a public
  identifier (it is exposed in the page source by design), and the snippet URL
  is a public origin. All credentials (`APP_SECRET`, DB password, admin login)
  stay in operator-managed env/secret storage.
- No customer PII, emails, or revenue figures appear in any file touched by this
  spec.

## Acceptance Criteria

- `deploy/umami/docker-compose.yml` exists, defines `umami` + a Postgres service
  on a dedicated network, reads `DATABASE_URL`/`APP_SECRET`/`POSTGRES_PASSWORD`
  from the environment (none hardcoded), uses persistent named volumes, and
  `restart: unless-stopped`. `docker compose -f deploy/umami/docker-compose.yml
  config` validates without error.
- `deploy/umami/README.md` documents: how to bring the stack up, the
  `stats.cloudroof.eu` reverse-proxy/TLS requirements (including
  `X-Forwarded-Proto`/`X-Forwarded-Host`), first-run admin password change, and
  where the Website ID + script origin are recorded.
- `deploy/umami/.env.example` exists with placeholder variable names only (no
  real passwords, secrets, or tokens).
- A `grep -rn "openpanel.dev\|op1.js" .` (excluding `.git/`) returns **zero**
  matches — the third-party loader is fully removed.
- A `grep -rln "umami" --include='*.html' .` (excluding `.git/`) returns at least
  `index.html`, `public/index.html`, `wgmesh.dev/index.html`,
  `evolution/wgmesh-cdn-slides.html`, and `docs/index.html`, and each of those
  files contains the canonical snippet with `src="https://stats.cloudroof.eu/script.js"`
  and a non-placeholder `data-website-id` matching the deployed Website ID.
- In `index.html` and `public/index.html`, the `track()` helper now calls
  `window.umami` (v2 `track` with v1 `trackEvent` fallback), still wrapped in the
  existing `try/catch` so a blocked/unavailable Umami cannot throw into modal
  code. The three event names (`exit_modal_shown`, `exit_modal_dismissed`,
  `exit_modal_submitted`) are unchanged.
- No `window.op(` references remain in any committed HTML file.
- After deploy: opening `https://cloudroof.eu/` in a browser loads
  `https://stats.cloudroof.eu/script.js` (200, first-party, no third-party
  analytics request to `openpanel.dev` or `counter.hackrsvalv.com`), and a
  pageview appears in the Umami dashboard for the cloudroof.eu website.
- After deploy: triggering the exit-intent modal produces `exit_modal_shown`,
  and submitting/dismissing it produces the corresponding events in Umami's
  events view.
- HTML still validates as well as before (no new unclosed tags introduced); the
  snippet sits inside `<head>` and the rewritten `track()` helper introduces no
  new global variables beyond the existing `window.wgmeshExitModal`.

## Out of scope

- Migrating, backfilling, or reconciling historical analytics data from
  OpenPanel/`counter.hackrsvalv.com` into Umami.
- Changing the exit-intent modal's UX, triggers, cooldown timing, or the
  Buttondown form integration.
- Adding a cookie/consent banner (Umami is used cookieless here; if cookies are
  enabled later that would be a separate issue).
- Replacing the in-product telemetry in `pkg/analytics/*` and
  `cmd/analytics-dashboard` — that is the wgmesh CLI/daemon analytics, unrelated
  to the cloudroof.eu website tracker.
- Changes to pricing copy, Polar.sh product IDs, OG metadata, or any content
  other than the analytics snippet and the `track()` helper.
- Hardening/HA of the Umami deployment (replicas, backups automation, monitoring
  alerts) — a reasonable single-instance deploy is sufficient for this issue;
  production hardening is a follow-up.
- Provisioning DNS records or TLS certificates themselves (operator action);
  this spec only documents the requirements.
