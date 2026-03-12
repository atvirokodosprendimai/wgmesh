---
title: "chore: release v0.2.0"
type: chore
status: completed
date: 2026-03-12
---

# chore: release v0.2.0

## Overview

Cut the stable v0.2.0 release of wgmesh — promoting v0.2.0-rc1 to stable with all features and fixes merged since March 1. The release pipeline (GoReleaser + GitHub Actions + Docker + Nix + Homebrew) is already fully operational from the v0.2.0-rc1 cycle.

## What's New Since v0.1.0

### Features
- **Service CLI** (`service add/list/remove`) — register local services for managed ingress via Lighthouse (#398)
- **Custom mesh subnet** (`--mesh-subnet` / `--network`) — user-defined IPv6 subnet instead of derived (#425)
- **Chimney multi-repo dashboard** — org-level repo discovery with aggregate endpoints (#424)
- **Interface name validation** + shell injection hardening (#422)
- **Polar.sh checkout links** — swap sponsor links to Polar.sh (#423)

### Fixes
- Goose review dispatch race condition — time-based dedup + cancel-in-progress (#426)
- Goose review provider cascade — Z.ai GLM-5 primary + OpenRouter BYOK fallbacks
- Company loop issue dedup with fuzzy keyword matching
- README restructure for clarity

## Pre-Release Checklist

### Phase 1: Merge Open PRs

Merge in dependency order (rebase each after previous merge):

- [x] #426 — `fix(ci): prevent goose-review dispatch race condition`
- [x] #422 — `feat: interface name validation + shell injection hardening + FAQ`
- [x] #423 — `feat: swap sponsor links to Polar.sh checkout`
- [x] #424 — `feat(chimney): multi-repo org dashboard (#334)` (rebase conflict in docs/index.html resolved)
- [x] #425 — `feat: custom mesh subnet support (--mesh-subnet / --network)` (rebase conflict in config.go resolved)

All 5 PRs currently pass CI and are mergeable (verified 2026-03-12).

### Phase 2: Verify Main

- [x] Pull latest main after all merges
- [x] Run `make test` — all tests pass
- [x] Run `make lint` — no lint errors
- [x] Run `go build ./...` — all binaries compile (wgmesh, lighthouse, chimney)
- [x] Verify `go vet ./...` clean

### Phase 3: Tag and Release

- [x] Create annotated tag: `git tag -a v0.2.0 -m "v0.2.0"`
- [x] Push tag: `git push origin v0.2.0`
- [x] Verify release workflow triggers (`.github/workflows/release.yml`)
- [x] Verify Docker build workflow triggers (`.github/workflows/docker-build.yml`)

### Phase 4: Verify Artifacts

- [x] GitHub Release page created with changelog
- [x] Binary archives present: linux-amd64, linux-arm64, linux-armv7, darwin-amd64, darwin-arm64
- [x] .deb packages present (amd64, arm64, armhf)
- [x] .rpm packages present (x86_64, aarch64, armv7hl)
- [x] Checksums file attached
- [x] Homebrew tap updated (`atvirokodosprendimai/homebrew-tap`)
- [ ] Docker image pushed to `ghcr.io/atvirokodosprendimai/wgmesh:v0.2.0` and `:latest` (build in progress)
- [x] Spot-check: download one binary, verify `wgmesh --version` shows `0.2.0`

### Phase 5: Post-Release

- [ ] Verify `nix run github:atvirokodosprendimai/wgmesh` still works (flake.nix vendorHash may need update if deps changed)
- [ ] Update any deployment references to v0.2.0

## Skipped PRs

- **#421** (ops: runway tracking) — operational, not a code change
- **#405** (--account flag) — has merge conflict, defer to next release

## Release Infrastructure Reference

| Component | File | Notes |
|-----------|------|-------|
| GoReleaser | `.goreleaser.yml` | Builds, nfpms, homebrew, changelog |
| Release workflow | `.github/workflows/release.yml` | Triggers on `v*.*.*` tags |
| Docker workflow | `.github/workflows/docker-build.yml` | Multi-arch, tags `:latest` for stable |
| Nix flake | `flake.nix` | May need vendorHash update |
| Packaging | `packaging/` | systemd unit, postinstall, preremove |
| Version var | `main.go:24` | `var version = "dev"` (injected via ldflags) |

## Sources

- Previous release: v0.2.0-rc1 (2026-03-01)
- Release pipeline plan: `memory/plan - 2603012134 - distributable packages deb and nix via goreleaser.md`
- GoReleaser config: `.goreleaser.yml`
