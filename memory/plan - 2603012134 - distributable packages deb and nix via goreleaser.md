---
tldr: Replace manual binary-build workflow with goreleaser — produces .deb, .rpm, Nix flake, and Homebrew on every tag
status: active
---

# Plan: Distributable packages — .deb, .rpm, Nix via goreleaser

## Context

- Issue: #358 — feat: distributable packages (Debian .deb + Nix)
- Spec: [[spec - cli entry point - dual mode dispatch with daemon wiring and rpc server]]
- Existing: `.github/workflows/binary-build.yml` builds raw binaries + Homebrew tap update
- Existing: `Dockerfile` (Alpine-based, wireguard-tools + iptables)
- Go 1.25, module `github.com/atvirokodosprendimai/wgmesh`
- Systemd support already in `pkg/daemon/systemd.go`

## Phases

### Phase 1 — goreleaser config + .deb/.rpm — status: completed

1. [x] Add `.goreleaser.yml` with nfpm config
   - => builds: linux-amd64, linux-arm64, linux-armv7, darwin-amd64, darwin-arm64
   - => nfpm: .deb and .rpm with wireguard-tools dep, systemd unit, /etc/wgmesh/ and /var/lib/wgmesh/ dirs
   - => `brews` section preserved (deprecated but functional — `homebrew_casks` has different schema for CLIs)
2. [x] Create standalone systemd unit file at `packaging/wgmesh.service`
   - => moved to `packaging/` to avoid conflict with goreleaser `dist/` output dir
   - => static unit based on template in `pkg/daemon/systemd.go`, reads WGMESH_SECRET from env file
3. [x] Create packaging scripts: `packaging/postinstall.sh`, `packaging/preremove.sh`
4. [x] Replace `binary-build.yml` with goreleaser-based release workflow
   - => `release.yml` — runs `goreleaser release --clean` on v* tags
   - => removed old binary-build.yml (matrix build + manual Homebrew update)
5. [x] Test locally with `goreleaser build --snapshot --clean`
   - => all 5 targets built successfully

### Phase 2 — Nix flake — status: completed

1. [x] Add `flake.nix` with package derivation
   - => buildGoModule with vendorHash (not vendor dir — kept vendor/ out of git)
   - => `subPackages = [ "." ]` to avoid building cmd/chimney (has undefined refs)
   - => outputs for all default systems via flake-utils
   - => NixOS module with `services.wgmesh.enable`, `secretFile`, `extraArgs` options
2. [x] Test with `nix build`
   - => `nix flake check` passes, `nix build .#default` produces working binary
   - => flake.lock auto-generated on first check

### Phase 3 — Verify end-to-end — status: open

1. [ ] Tag a pre-release (e.g. `v0.2.0-rc1`) to trigger the release workflow
2. [ ] Verify: .deb and .rpm attached to GitHub release
3. [ ] Verify: `dpkg -i wgmesh_*.deb` installs binary + systemd unit
4. [ ] Verify: `nix build` produces working binary
5. [ ] Verify: Homebrew tap updated

## Verification

- `goreleaser build --snapshot` succeeds locally
- Tagged release produces .deb, .rpm, binaries, Homebrew update
- `dpkg -i` installs wgmesh with systemd unit, `/etc/wgmesh/` dir, wireguard-tools dep
- `nix build .#wgmesh` produces working binary
- Existing Homebrew flow preserved

## Adjustments

## Progress Log

- 2603012134 — Plan created
- 2603012145 — Phase 1 complete. goreleaser config, packaging scripts, release workflow, snapshot build tested.
- 2603012200 — Phase 2 complete. Nix flake with vendorHash, NixOS module, `nix build` verified.
