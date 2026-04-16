# Specification: Issue #523

## Classification
documentation

## Problem Analysis

Presence-stage signals are partially in place, but the required public entry path is broken:

1. `docs/index.html` is currently an internal pipeline dashboard, not a product landing page
   (`<title>wgmesh — Agent Pipeline Dashboard</title>`). Because GitHub Pages deploys the `docs/`
   directory as-is (`.github/workflows/pages.yml`), visitors to the site root land on dashboard
   internals instead of clear product positioning.
2. `docs/quickstart.md` exists and is comprehensive, but there is no `docs/quickstart.html` page,
   so the quickstart is not published as a first-class GitHub Pages page.
3. Install-flow evidence exists (`scripts/verify-install.sh`, `docs/install-verification.md`), but
   Presence exit criteria require a consolidated audit that explicitly marks each requirement pass/fail.

This issue should deliver a concrete Presence audit and close the two missing public-presence gaps
(landing page and published quickstart page), then re-verify install flow.

## Implementation Tasks

### Task 1: Restore GitHub Pages root to a product landing page

Apply the already-defined landing-page implementation from `specs/issue-496-spec.md` exactly:

1. Rename `docs/index.html` to `docs/pipeline.html` with no content edits.
2. Create a new `docs/index.html` using the exact HTML block from **Issue #496 / Task 2**.
3. Create `docs/quickstart.html` using the exact HTML block from **Issue #496 / Task 3**.

Do not redesign or reword those pages in this issue; copy them verbatim from the spec so this issue
is a pure completion of missing Presence deliverables.

### Task 2: Create an explicit Presence audit record

Create `docs/presence-audit.md` with:

1. A section `# Presence Stage Audit (Issue #523)`.
2. A table with these columns: `Requirement`, `Evidence`, `Status`.
3. Exactly these four requirements as rows:
   - Landing page exists with clear positioning
   - Quickstart guide published and working
   - Install flow documented and functional
   - Product positioning clear to target audience
4. Evidence links pointing to:
   - `docs/index.html` (landing)
   - `docs/quickstart.html` and `docs/quickstart.md`
   - `README.md`, `docs/quickstart.md`, `scripts/verify-install.sh`, `docs/install-verification.md`
5. A final `## Exit Criteria` section that marks all three exit criteria as met only after verification.

### Task 3: Re-verify install flow and update verification artifacts

1. Run `bash scripts/verify-install.sh` from repo root.
2. In `docs/install-verification.md`, update `## Post-install Checklist` so each checkbox is marked
   based on actual verification results from this run (expected: all checked).
3. Add a `Last verified` line in `docs/install-verification.md` with date and command used.
4. In `docs/presence-audit.md`, set `Install flow documented and functional` to pass only if step 1 succeeds.

### Task 4: Confirm public-presence links resolve

After Tasks 1–3, verify:

1. `docs/index.html` links to `quickstart.html` and `pipeline.html`.
2. `docs/quickstart.html` loads and renders quickstart content (or clean fallback link to GitHub doc).
3. README links already used as install/positioning evidence resolve:
   - `docs/quickstart.md`
   - `docs/quickstart.md#troubleshooting`

Do not add new product claims or marketing copy in this issue beyond the copied landing-page content.

## Affected Files

- **Modified:** `docs/index.html` (replaced with landing page)
- **Modified:** `docs/install-verification.md` (checklist + last-verified metadata)
- **New:** `docs/pipeline.html` (moved dashboard)
- **New:** `docs/quickstart.html` (published quickstart page for GitHub Pages)
- **New:** `docs/presence-audit.md` (issue #523 audit artifact)

## Test Strategy

1. Run `bash scripts/verify-install.sh` and require success.
2. Open `docs/index.html`, `docs/pipeline.html`, and `docs/quickstart.html` locally; verify they render.
3. Click-through check:
   - Landing page CTA to quickstart works.
   - Landing page link to pipeline dashboard works.
4. Confirm `docs/presence-audit.md` marks all four audit rows and all exit criteria with explicit evidence.

