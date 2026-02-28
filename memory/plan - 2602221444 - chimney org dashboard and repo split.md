---
tldr: Evolve chimney into a TV-screen org dashboard with dynamic repo discovery, then migrate it to its own standalone repo
status: active
---

# Plan: Chimney org dashboard and repo split

## Context

- Spec: [[spec - chimney - dashboard server with github api proxy and two-layer cache]]
- Spec: [[spec - chimney cache control - runtime cache invalidation api]]
- Spec: [[spec - chimney metrics - prometheus text format endpoint for cache and request counters]]
- Issue: #334 — org-level dashboard with dynamic repo discovery and TV screen support
- Issue: #335 — split chimney into atvirokodosprendimai/chimney (blocked on #334)

## Phases

### Phase 1 - Org repo discovery - status: open

Replace hardcoded `GITHUB_REPO` with dynamic org-level discovery.

1. [ ] Add `GITHUB_ORG` env var; implement `/orgs/{org}/repos` polling with configurable TTL
   - Keep `GITHUB_REPO` as a single-repo fallback for backward compat
   - Cache repo list in-memory; refresh in background goroutine
   - Optional `GITHUB_REPOS_EXCLUDE` denylist (comma-separated)
2. [ ] Expose `GET /api/github/org/repos` — returns current discovered repo list
   - Used by frontend to build nav/tabs dynamically
3. [ ] Add `GET /api/github/org/activity` — aggregated open PRs across all discovered repos
   - Merge results from per-repo `/pulls?state=open` calls
   - Cache per repo; aggregate on read

### Phase 2 - TV screen dashboard - status: open

Redesign the frontend for a wall-mounted TV screen: glanceable, auto-refreshing, no interaction required.

4. [ ] Audit current `docs/` dashboard HTML/CSS — document what exists
5. [ ] Redesign layout: full-screen dark theme, no scroll at 1080p, large typography
   - Sidebar: repo list (auto-populated from `/api/github/org/repos`)
   - Main panel: aggregate open PR feed with repo badges
   - Status bar: last refresh time, org name, active repo count
6. [ ] Auto-refresh: poll `/api/github/org/activity` on configurable interval (default 30s)
   - No user interaction required; JS `setInterval` or SSE
7. [ ] Legibility pass: font sizes readable from 3–5 metres; contrast ratio ≥ 7:1
8. [ ] Smoke test: verify dashboard loads at `chimney.beerpub.dev` and shows all org repos

### Phase 3 - Eidos spec update - status: open

Update chimney specs to reflect new org-level purpose before the split.

9. [ ] Update [[spec - chimney - dashboard server with github api proxy and two-layer cache]] — reflect org-level view, dynamic discovery, TV intent
10. [ ] Update `eidos/chimney.md` overview — replace wgmesh-specific references with org scope

### Phase 4 - Repo split - status: open

Migrate chimney to `atvirokodosprendimai/chimney`. Blocked on Phase 1–3 complete.

11. [ ] Create `atvirokodosprendimai/chimney` repo on GitHub
    - Initialize with README, MIT license, `.gitignore`
    - Configure branch protection: require PR + passing CI on `main`
12. [ ] Migrate code: copy `cmd/chimney/` → new repo root; copy `deploy/chimney/` → `deploy/`
    - Adjust Go module name: `github.com/atvirokodosprendimai/chimney`
    - Update `go.mod`, `go.sum`
13. [ ] Migrate workflows: copy `.github/workflows/chimney-*.yml` → new repo
    - Update image path: `ghcr.io/atvirokodosprendimai/chimney` (drop `/wgmesh/`)
    - `chimney-deploy.yml` triggers on push to `main` (no longer `workflow_run`)
14. [ ] Update all image references: `compose.origin.yml`, `bluegreen.sh`, watchtower config
15. [ ] Configure secrets in new repo: `CHIMNEY_SSH_KEY`, `HCLOUD_TOKEN`, `GH_TOKEN`, `GHCR_TOKEN`
16. [ ] Trigger deploy from new repo; verify smoke tests pass at `chimney.beerpub.dev`
17. [ ] Clean `wgmesh`: remove `cmd/chimney/`, `deploy/chimney/`, `chimney-*.yml` workflows, chimney eidos specs
    - Update `wgmesh` README to link to `atvirokodosprendimai/chimney`

## Verification

- All org repos appear in dashboard without any static config change when a new repo is created
- Dashboard loads full-screen on a 1080p TV with no scroll; all text readable from 3m
- Auto-refreshes every 30s without user interaction
- `chimney.beerpub.dev` serves traffic from new repo's pipeline
- `wgmesh` repo has no chimney code or workflows
- Adding a new org repo → appears in dashboard within one poll interval (≤ 15 min)

## Adjustments

<!-- Plans evolve. Document changes with timestamps. -->

## Progress Log

<!-- Timestamped entries tracking work done. Updated after every action. -->
- 2602221444 — plan created
