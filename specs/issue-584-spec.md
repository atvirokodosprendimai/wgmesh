# Specification: Issue #584

> Title convention: `spec: Issue #584 - Add Polar checkout CTAs to wgmesh.dev + cloudroof.eu landing pages`
> Target branch: `main`. PR contains ONLY this spec file.

## Classification

**feature** (landing-page / documentation content change — no Go code).

> Note: This spec scopes the work to **only what remains**. The `wgmesh.dev` half
> of the issue is already satisfied by `wgmesh.dev/index.html` (see Problem
> Analysis), so the only actionable deliverable is the `cloudroof.eu` slides file
> in this repo. If the implementation agent later determines the wgmesh.dev file
> needs further changes, treat that as out-of-scope — that file lives in the
> external `wgmeshdev` repo and is only mirrored here.

## Problem Analysis

The issue asks for three things:

1. `wgmesh.dev` `#pricing` section with 3 Polar checkout CTAs + footer "Payment via Polar.sh".
2. `cloudroof.eu` (the slides deck, served from `evolution/wgmesh-cdn-slides.html`) gets a final-slide CTA to the $20 Edge Node Polar tier.
3. Footer "Payment via Polar.sh" link on both pages.

**Status of each, verified by reading the actual repo files:**

- **`wgmesh.dev/index.html` — DONE.** Verified at the lines below; no changes needed:
  - `wgmesh.dev/index.html` already has a `<section id="pricing">` titled "Sponsor wgmesh.dev" with three `.pricing-card` blocks.
  - Each card already links to the correct Polar checkout URL with `target="_blank" rel="noopener noreferrer"`:
    - Founding Member → `https://polar.sh/checkout?productId=3f5d75de-936b-49d8-a21b-4b79d9fd22c1`
    - Edge Node → `https://polar.sh/checkout?productId=1927e637-4cfd-4c94-8bee-c5518803bc89`
    - Mesh Operator → `https://polar.sh/checkout?productId=eb20683e-55ea-4354-9d8c-070e55a4eff5`
  - The footer already contains `| <a href="https://polar.sh/" ...>Payment via Polar.sh</a>`.
  - **No edit task is generated for `wgmesh.dev/index.html`.**

- **`evolution/wgmesh-cdn-slides.html` — MISSING.** Verified by `grep -ci "polar"` → 0 matches, and there is no `<footer>` element at all. The deck ends with slide `id="slide-7"` ("Summary / Your Own Global CDN"), whose last element is a GitHub link paragraph. This is the cloudroof.eu surface that needs the final-slide CTA and footer link.

The canonical Polar URLs / product IDs are already used in `docs/index.html` (lines 351, 363, 375) and `wgmesh.dev/index.html`. The issue only asks for the **$20 Edge Node** tier on cloudroof.eu:
`https://polar.sh/checkout?productId=1927e637-4cfd-4c94-8bee-c5518803bc89`

> The `.bak` sibling file `evolution/wgmesh-cdn-slides.html.bak` is byte-identical
> to the live file (diff produces no output). It should be left alone — it is a
> backup artifact, not a published page. Do NOT edit it.

## Implementation Tasks

### Task 1: Add the $20 Edge Node Polar checkout CTA to the final slide
- **File:** `evolution/wgmesh-cdn-slides.html` (modify)
- **What:** Add a styled call-to-action block to the final summary slide (`<section class="slide slide-2" id="slide-7">`), placed AFTER the existing closing GitHub-link paragraph (`<p style="text-align: center; margin-top: 3rem; color: var(--text-muted);">…github.com/atvirokodosprendimai/wgmesh →…</p>`) and BEFORE the closing `</div>` of the `.content` container for that slide.
- **Detail:** The CTA is a single link `<a>` with class `cta-btn` that points to `https://polar.sh/checkout?productId=1927e637-4cfd-4c94-8bee-c5518803bc89` and uses `target="_blank" rel="noopener noreferrer"`. Link text must reference the **$20/month Edge Node** tier (e.g. "Reserve your edge node — $20/mo via Polar →"). The CTA must be visually prominent and consistent with the deck's existing dark/green design system. Because the deck has no existing CTA button style, add the CSS for `.cta-btn` inside the `<style>` block in `<head>` (see Task 2). The exact insertion point is the last child of `<div class="content">` inside the `id="slide-7"` section.

### Task 2: Add CSS for the CTA button (matches deck design system)
- **File:** `evolution/wgmesh-cdn-slides.html` (modify)
- **What:** Append a new `.cta-btn` rule set inside the existing `<style>` element in `<head>`.
- **Detail:** Use the deck's existing CSS custom properties (defined in `:root` at the top of the same `<style>`): `--bg-card`, `--green`, `--text-primary`, `--text-muted`. The button must be:
  - `display: inline-block` with generous padding (`1rem 2.5rem`).
  - Background `var(--green)`, text color `#0a0a0f` (the deck's `--bg-dark` value, to ensure contrast on the bright green).
  - `text-decoration: none`, `border-radius: 12px`, `font-weight: 700`.
  - `font-family: 'Space Grotesk', sans-serif` to match the deck's heading font.
  - A subtle hover effect (`transform: translateY(-2px)` + `box-shadow`), matching the hover treatment already used by `.card:hover` in this same file.
  - Do NOT introduce any new external font, image, or `<script>`. Reuse only what is already loaded in this file.

### Task 3: Add a "Payment via Polar.sh" footer link to the deck
- **File:** `evolution/wgmesh-cdn-slides.html` (modify)
- **What:** Add a site-wide footer line at the very bottom of `<body>`, immediately BEFORE the closing `</body>` tag and AFTER the existing `<script>…</script>` block.
- **Detail:** Create a `<footer>` element containing the text "Payment via Polar.sh" where "Polar.sh" is a link to `https://polar.sh/atvirokodosprendimai` with `target="_blank" rel="noopener noreferrer`. Style it inline so it does not depend on any new external CSS: `position: fixed; bottom: 1.5rem; right: 2rem;` (mirroring the existing fixed-position `.slide-counter` which lives at `bottom: 2rem; left: 2rem`), with `font-family: 'JetBrains Mono', monospace; font-size: 0.75rem; color: var(--text-muted); z-index: 100;`. This matches the footer requirement in the acceptance criteria without overlapping the slide counter (which is anchored to the opposite corner).

### Task 4: Verification (no code change — manual / scripted check only)
- **File:** `evolution/wgmesh-cdn-slides.html` (read-only verification)
- **What:** After edits, confirm the following grep checks all return ≥ 1:
  - `grep -c "polar.sh/checkout?productId=1927e637-4cfd-4c94-8bee-c5518803bc89" evolution/wgmesh-cdn-slides.html` ≥ 1
  - `grep -c "Payment via Polar" evolution/wgmesh-cdn-slides.html` ≥ 1
  - `grep -c "cta-btn" evolution/wgmesh-cdn-slides.html` ≥ 2 (one CSS rule + one usage)
- **Detail:** If any count is 0, the corresponding task was not applied correctly. There are no Go tests for this issue (it is static HTML); "test click-through" in the acceptance criteria is a manual browser check performed post-deploy and is not automatable inside this repo.

## Affected Files

```
evolution/wgmesh-cdn-slides.html   (modify: add CTA to final slide, add .cta-btn CSS, add footer link)
```

Files explicitly NOT modified (and why):
- `wgmesh.dev/index.html` — already satisfies all three wgmesh.dev acceptance criteria (verified).
- `docs/index.html` — already contains the canonical Polar CTAs; out of scope.
- `evolution/wgmesh-cdn-slides.html.bak` — byte-identical backup artifact; not a published page.
- No Go source (`*.go`), `go.mod`, `go.sum`, or test files change.

## Acceptance Criteria

- `evolution/wgmesh-cdn-slides.html` contains the $20 Edge Node Polar checkout URL `https://polar.sh/checkout?productId=1927e637-4cfd-4c94-8bee-c5518803bc89` exactly once, inside a `target="_blank" rel="noopener noreferrer"` anchor on the final slide (`id="slide-7"`).
- `evolution/wgmesh-cdn-slides.html` contains a `<footer>` with a "Payment via Polar.sh" link to `https://polar.sh/atvirokodosprendimai`.
- The new CTA uses only CSS custom properties and fonts already defined in the file's `:root` / `<head>` — no new external resources are introduced.
- The diff is confined to `evolution/wgmesh-cdn-slides.html`; no other tracked file changes.
- No product IDs other than `1927e637-4cfd-4c94-8bee-c5518803bc89` (Edge Node) are added to the slides deck, matching the issue's explicit scope ("final-slide CTA to the Polar checkout for the $20 tier").
- `wgmesh.dev/index.html` is **not** modified (it already meets its criteria).
- There is no Go build/test impact; `go build ./...` and `go test ./...` remain unaffected and are not required gating checks for this content-only change.

## Estimated Complexity

**low** — a single static HTML file, ~3 small edits (CTA markup, CSS rule, footer line), no code/logic, no dependencies, no build step.
