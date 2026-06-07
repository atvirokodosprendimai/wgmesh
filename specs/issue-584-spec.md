# Specification: Issue #584

## Classification
feature

## Deliverables
code

## Problem Analysis

Issue #584 requests adding Polar checkout CTAs to two landing pages: `wgmesh.dev` and `cloudroof.eu`. Partial work was done — PR #646 (`d3af198`) added a `#pricing` section with all three Polar checkout tiers and a "Payment via Polar.sh" footer link to `wgmesh.dev/index.html`. However, the `cloudroof.eu` slides file (`evolution/wgmesh-cdn-slides.html`) was never updated on `main`. It currently has:

- **Zero** references to Polar, checkout, or any payment URL
- **No footer element** at all
- **No CTA slide** for the $20 Edge Node tier

The file `evolution/wgmesh-cdn-slides.html` currently has 9 `<section class="slide">` elements and 9 `<button class="nav-dot">` elements (indices 0–8), but the hardcoded slide counter at line 608 says `08`. The JS at line ~1062 overrides this at runtime with `String(slides.length).padStart(2, '0')`, so the hardcoded value is cosmetic but should be corrected.

The last slide is `id="slide-7"` (Summary, lines 1005–1049). It ends with a GitHub link. The `<script>` block begins immediately after line 1049.

Acceptance criteria status on `main`:
| Criterion | Status |
|---|---|
| `wgmesh.dev` `#pricing` with 3 tiers + Polar CTAs | ✅ Done (lines 122–172) |
| `wgmesh.dev` footer "Payment via Polar.sh" | ✅ Done (line 200) |
| `cloudroof.eu` final-slide CTA to $20 Polar checkout | ❌ Missing |
| `cloudroof.eu` footer "Payment via Polar.sh" | ❌ Missing |

## Proposed Approach

Add a new closing CTA slide as `slide-8` (index 8) to the end of `evolution/wgmesh-cdn-slides.html`, featuring the $20 Edge Node Polar checkout as the primary call-to-action alongside a smaller link to the full pricing page. Append a minimal site footer with a "Payment via Polar.sh" link after all slides and before the `<script>` block. Increment the nav-dot list to include `data-slide="9"` and fix the hardcoded total from `08` to `10` (the JS overrides this, but the HTML default should be correct for non-JS contexts or if JS fails).

## Implementation Tasks

### Task 1: Add CTA slide and footer to `evolution/wgmesh-cdn-slides.html`

- **File:** `evolution/wgmesh-cdn-slides.html` (modify)
- **What:** Insert a new `<section class="slide slide-3" id="slide-8">` between the closing `</section>` of slide-7 (line 1049) and the opening `<script>` tag (line 1051). Also insert a `<footer>` element between the new slide and the `<script>` tag.
- **Detail:** The new slide must contain:

  1. A `<span class="tag">Support wgmesh</span>` tag line.
  2. An `<h2>` heading: `Become an Edge Node Sponsor`.
  3. A subtitle `<p>` in `var(--text-muted)` explaining: "Fund the project and get priority support, early access to beta builds, and advanced network insights."
  4. A highlight box (`<div class="highlight-box">`) centered, containing:
     - The price: `<p style="font-size: 2.5rem; font-weight: bold; color: var(--green); margin-bottom: 0.5rem;">$20 / month</p>`
     - An anchor `<a>` styled as the existing CTA (green color, no underline, large font, bold weight) linking to `https://polar.sh/checkout?productId=1927e637-4cfd-4c94-8bee-c5518803bc89` with `target="_blank" rel="noopener noreferrer"`. Text: `Sponsor via Polar.sh →`
  5. Below the highlight box, a small `<p>` centered in `var(--text-muted)` with: "Other tiers: `<a href="https://polar.sh/checkout?productId=3f5d75de-936b-49d8-a21b-4b79d9fd22c1" style="color: var(--green);">$5 Founding Member</a> · <a href="https://polar.sh/checkout?productId=eb20683e-55ea-4354-9d8c-070e55a4eff5" style="color: var(--green);">$100 Mesh Operator</a>`"

  The slide wraps with `<div class="grid-overlay"></div>` and `<div class="content">` matching the pattern of all other slides.

  After the new slide's closing `</section>`, insert a `<footer>` element:

  ```html
  <footer style="text-align: center; padding: 2rem; color: var(--text-muted); font-size: 0.85rem;">
      <p>&copy; 2024–2026 wgmesh. <a href="https://polar.sh/" target="_blank" rel="noopener noreferrer" style="color: var(--green); text-decoration: none;">Payment via Polar.sh</a></p>
  </footer>
  ```

### Task 2: Add nav-dot for new slide and fix slide counter

- **File:** `evolution/wgmesh-cdn-slides.html` (modify)
- **What:** In the `<nav class="nav">` block (lines 595–604), add a 10th nav-dot button after the existing `data-slide="8"` button. In the `<div class="slide-counter">` block (line 608), change the hardcoded `08` to `10`.
- **Detail:** Insert this line after line 604 (`<button class="nav-dot" data-slide="8"></button>`):

  ```html
  <button class="nav-dot" data-slide="9"></button>
  ```

  Change line 608 from:
  ```html
  <span id="total">08</span>
  ```
  to:
  ```html
  <span id="total">10</span>
  ```

  The existing JS at `totalSlides.textContent = String(slides.length).padStart(2, '0');` dynamically computes the correct total, so this hardcoded value serves as the fallback default.

## Affected Files
```
evolution/wgmesh-cdn-slides.html  (modify: add CTA slide, footer, nav-dot, fix counter)
```

## Acceptance Criteria

- The file `evolution/wgmesh-cdn-slides.html` contains a final slide (`id="slide-8"`) with a Polar checkout link to `https://polar.sh/checkout?productId=1927e637-4cfd-4c94-8bee-c5518803bc89` as a clickable anchor with `target="_blank"`
- The file contains a `<footer>` element between the last `</section>` and `<script>` with text "Payment via Polar.sh" linking to `https://polar.sh/`
- The file contains exactly 10 `<button class="nav-dot">` elements (indices 0–9) matching the 10 slide sections
- The hardcoded `<span id="total">` value is `10`
- `grep -c 'class="slide '` on the file returns `10`
- `grep -c 'polar.sh/checkout'` on the file returns at least `3` (the $20 primary CTA plus the $5 and $100 secondary links)
- No changes to `wgmesh.dev/index.html`, `public/index.html`, `docs/index.html`, or any Go source files
- `make build` still passes (no Go code changed)

## Estimated Complexity
low (1 file, ~60 lines of HTML)
