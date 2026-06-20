# Specification: Issue #792 — Exit-Intent Lead Capture Modal (WireGuard Mesh Quickstart PDF)

## Classification
feature

## Problem Analysis

`cloudroof.eu` is a marketing surface for wgmesh. Today it captures emails only through a static, in-page Buttondown embed (the "Follow the build" / `meshletter` block that appears in `index.html`, `public/index.html`, and `wgmesh.dev/index.html`). That embed is passive: a visitor must scroll to the bottom and choose to subscribe. Visitors who read the hero/pricing sections and then leave without scrolling never see a capture prompt, so a large fraction of high-intent traffic (people who evaluated the product enough to consider leaving) is lost.

The goal of this issue is to recover a portion of that exit traffic with an **exit-intent modal** that offers a tangible, relevant lead magnet — a free **"WireGuard Mesh Quickstart PDF"** — in exchange for an email address. This converts an anonymous leaving visitor into a known subscriber/lead who can be nurtured and eventually moved into the managed-ingress funnel.

Key constraints drawn from the existing codebase:

1. **No backend / no new server.** The marketing pages are static HTML served as-is (see `index.html`, `public/index.html`, `wgmesh.dev/index.html`). There is no form-handling service in the repo. The existing email path is Buttondown's hosted embed endpoint (`https://buttondown.com/api/emails/embed-subscribe/meshletter`). The modal must therefore either reuse Buttondown (preferred, to keep one source of truth for subscribers) or POST to an existing endpoint — it must **not** introduce a new backend that stores PII in this repo.
2. **Keep the repo public-safe.** No secrets, no API keys in the HTML, no customer PII committed. Email addresses must go directly to Buttondown (or an existing third-party) and never be written into the repository.
3. **Consistency with existing capture block.** The in-page `meshletter` form already exists and works; the modal should reuse the same Buttondown list and the same visual language (blue `#2563eb` CTA, rounded `8px` inputs, neutral card background) so the two surfaces feel like one product.
4. **Don't annoy returning visitors.** A modal that fires on every exit is hostile. It must be suppressible (sessionStorage / localStorage with a cooldown) so a user who dismissed or converted is not re-prompted for a configurable period.
5. **Accessibility & mobile.** Exit-intent via mouse-leave does not exist on touch devices; the mobile fallback must be a scroll-depth / time-on-page trigger, not a `mouseleave` listener that never fires.
6. **Analytics parity.** The pages already load OpenPanel (`window.op(...)`). Modal impressions, dismissals, and submissions should be tracked with the same `window.op` calls so conversion is measurable alongside existing events.
7. **The lead magnet must actually exist (or be explicitly stubbed).** The "WireGuard Mesh Quickstart PDF" is referenced by the modal's success state. If a real asset is not yet produced, the success path must still deliver a usable artifact (a redirect to a hosted PDF URL) rather than a dead link. The asset URL is treated as a configuration value, not hardcoded inline, so it can be swapped without touching modal logic.

## Proposed Approach

Add a single, dependency-free, inline JavaScript + CSS exit-intent modal to each marketing HTML page that currently hosts the Buttondown capture block (`index.html`, `public/index.html`, `wgmesh.dev/index.html`). The modal:

- Is triggered by **desktop exit-intent** (cursor leaving the viewport top, the standard heuristic) **or** a **mobile/touch fallback** (time-on-page ≥ 25s AND scroll-depth ≥ 50%, whichever comes first after the page loads).
- Fires **at most once per cooldown** (default 7 days) using `localStorage`. Within a single session it fires at most once via `sessionStorage` so a tab switch back and forth does not re-prompt.
- Presents the **"WireGuard Mesh Quickstart PDF"** offer, a one-line value proposition, a single email `<input>`, and a primary CTA ("Send me the PDF").
- On submit, POSTs the email to the **same Buttondown embed endpoint** already used by the in-page form (`https://buttondown.com/api/emails/embed-subscribe/meshletter`), using a hidden `<iframe>` as the submit target (matching the existing pattern's `target="popupwindow"`), then swaps the modal body to a **success state** that links to the PDF.
- Is fully dismissable via a close (×) button, the Escape key, and a click on the backdrop, each of which arms the cooldown.
- Emits OpenPanel events (`exit_modal_shown`, `exit_modal_dismissed`, `exit_modal_submitted`) via the existing `window.op(...)` call, guarded so the modal still works if OpenPanel is blocked or fails to load.
- Carries **no secrets**: the Buttondown list name and the PDF URL are the only external references and both are already public-facing values.

The lead magnet PDF is hosted as a static asset at `public/wgmesh-quickstart.pdf` (served alongside `public/index.html`) and is referenced through a relative URL so it works in production without a hardcoded absolute host. If the PDF is not yet authored at implementation time, the success state links to a clearly-named placeholder path and a follow-up task is created to drop the real file in place; the modal logic is unaffected.

All behavior is implemented inline (one `<style>` block and one `<script>` block appended before `</body>`) to match the existing static-page convention and avoid introducing a build step or external JS dependency. A thin, well-commented JS module (an IIFE) owns trigger detection, cooldown state, submit handling, and teardown — no global namespace pollution beyond a single `wgmeshExitModal` object used only for test hooks.

### Trigger logic (deterministic)

- **Cooldown guard (run first):** if `localStorage.getItem('wgmesh_exit_modal_dismissed_at')` exists and `now - value < COOLDOWN_MS` (default `7 * 24 * 3600 * 1000`), do nothing and abort listener registration entirely.
- **Session guard:** if `sessionStorage.getItem('wgmesh_exit_modal_shown') === '1'`, abort.
- **Desktop:** listen for `document.mouseleave` / `mouseleave` on `document` where `e.clientY <= 0` (cursor exited through the top of the viewport). Fire once.
- **Touch / no-mouse devices:** a `setTimeout`/scroll listener that fires when `timeOnPageMs >= 25000` AND `maxScrollPercent >= 50`. Detect touch via `matchMedia('(hover: none)')` or absence of a `mousemove` within the first 3 seconds.
- On any valid trigger: show modal, set `sessionStorage` flag, start the cooldown timer only after a dismissal or submission (not on bare impression).

### Submit / success flow

1. Validate email with a permissive RFC-ish regex; on invalid, show inline error text and do not submit.
2. POST to Buttondown embed endpoint via the existing form action/target pattern (hidden iframe named `wgmesh_exit_target`). This avoids CORS and matches the working in-page form.
3. Immediately swap modal body to success state containing the PDF download link (`/wgmesh-quickstart.pdf`, configurable).
4. Set the cooldown flag (so we don't re-prompt a converted user) and emit `exit_modal_submitted`.
5. Auto-dismiss the success state after 12 seconds (user can also close manually).

### Files

- `index.html` — append modal markup + `<style>` + `<script>` before `</body>`.
- `public/index.html` — same treatment (this is the deployed canonical page).
- `wgmesh.dev/index.html` — same treatment (alternate marketing surface).
- `public/wgmesh-quickstart.pdf` — placeholder/real lead-magnet asset (added separately if not present; modal references it by relative path either way).

### Public-safety notes

- No API keys, tokens, or customer data are committed. The only third-party identifiers referenced are the public Buttondown list slug (`meshletter`) and the OpenPanel client id already present in the pages.
- Email addresses are submitted directly to Buttondown and are never read, stored, or logged by code in this repository.
- No analytics payload includes the visitor's email or any PII — events carry only action names and coarse triggers (`desktop`/`mobile`).

## Acceptance Criteria

- On each of `index.html`, `public/index.html`, and `wgmesh.dev/index.html`, an exit-intent modal element exists with id `wgmesh-exit-modal`, hidden by default (`display:none` or `[hidden]`).
- **Desktop trigger:** moving the cursor out of the top of the viewport (with cooldown/session guards clear) reveals the modal exactly once per session.
- **Mobile fallback:** on a `hover: none` device (or no mouse activity), the modal reveals after ≥25s on page AND ≥50% scroll depth, once per session.
- **Cooldown:** after a dismissal or successful submission, the modal does not reappear for 7 days (verifiable by setting `localStorage` and reloading). Within the same session it never reappears after first show.
- **Dismissal:** the modal closes via the × button, the Escape key, and a backdrop click; each arms the cooldown and emits `exit_modal_dismissed`.
- **Submit:** entering a valid email and clicking "Send me the PDF" submits to the Buttondown `meshletter` embed endpoint and transitions the modal to a success state containing a working link to the WireGuard Mesh Quickstart PDF (relative URL, e.g. `/wgmesh-quickstart.pdf`).
- **Validation:** an obviously invalid email (e.g. `not-an-email`) shows an inline error and does not submit.
- **Analytics:** `window.op` is called with `exit_modal_shown`, `exit_modal_dismissed`, and `exit_modal_submitted` on the respective actions; if `window.op` is undefined, no error is thrown.
- **No PII/secrets:** `git grep` of the diff shows no email addresses, API keys, tokens, or customer data; the only external references are the public Buttondown slug and existing OpenPanel client id.
- **No layout regressions:** the rest of each page renders unchanged; the modal markup is appended before `</body>` and styles are scoped under `#wgmesh-exit-modal` so they do not leak into existing elements.
- **Accessibility:** the modal has `role="dialog"`, `aria-modal="true"`, a labelled heading, focus moves to the email input on open, focus returns to the trigger/last focused element on close, and the backdrop traps tab focus while open.
- **No new backend:** no new server, endpoint, or PII storage is introduced in this repository; email capture flows through Buttondown exactly as the existing in-page form does.
- Existing build/lint unaffected (no Go changes; `make build` and `make test` remain green).

## Out of scope

- Authoring the actual content of the "WireGuard Mesh Quickstart PDF" (tracked separately; the modal references the asset path regardless of whether the file is finalized).
- A/B testing multiple modal creatives or offers; this issue ships a single offer. (Instrumentation events are included so a later experiment can be measured, but no variant framework is added.)
- Server-side lead storage, CRM integration, or a dedicated lead-capture API in this repo. Email handling stays on Buttondown.
- Changing or removing the existing in-page "Follow the build" Buttondown block; the modal is additive.
- Designing a new email automation/drip sequence for PDF registrants (a marketing/ops concern, not a code change).
- Supporting pages other than the three marketing HTML files listed above (docs, `pp.html`, etc.).
