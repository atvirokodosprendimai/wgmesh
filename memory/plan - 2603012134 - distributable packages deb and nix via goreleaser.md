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

### Phase 1 — goreleaser config + .deb/.rpm — status: open

1. [ ] Add `.goreleaser.yml` with nfpm config
   - builds: linux-amd64, linux-arm64, linux-armv7, darwin-amd64, darwin-arm64
   - nfpm: .deb and .rpm for linux targets
   - package contents: binary → `/usr/bin/wgmesh`, systemd unit → `/lib/systemd/system/wgmesh.service`, config dir → `/etc/wgmesh/`
   - dependencies: wireguard-tools
   - scripts: postinstall (systemd daemon-reload), preremove (stop service)
2. [ ] Create standalone systemd unit file at `dist/wgmesh.service`
   - based on existing template in `pkg/daemon/systemd.go` but static (reads from env file)
3. [ ] Create packaging scripts: `dist/postinstall.sh`, `dist/preremove.sh`
4. [ ] Replace `binary-build.yml` with goreleaser-based release workflow
   - `goreleaser release` on tags
   - `goreleaser build --snapshot` on main pushes (artifacts only, no publish)
   - keep Homebrew tap update (goreleaser has native support)
5. [ ] Test locally with `goreleaser build --snapshot --clean`

### Phase 2 — Nix flake — status: open

1. [ ] Add `flake.nix` with package derivation
   - buildGoModule with vendored deps or gomod2nix
   - output: `packages.x86_64-linux.default`, `packages.aarch64-linux.default`
   - NixOS module with systemd service option
2. [ ] Add `flake.lock`
3. [ ] Test with `nix build`

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
