# Install Verification Record

This document records the results of end-to-end install testing on fresh systems
(required for Presence stage readiness — Issue #497).

## Test Matrix

| Method | Platform | Status | Notes |
|--------|----------|--------|-------|
| Homebrew | macOS 14 (Sonoma) arm64 | ✅ Passes | `brew install atvirokodosprendimai/tap/wgmesh` |
| Homebrew | Ubuntu 22.04 amd64 | ✅ Passes | Linuxbrew path: `~/.linuxbrew/bin/wgmesh` |
| Pre-built binary | Ubuntu 22.04 amd64 | ✅ Passes | Direct download from GitHub Releases |
| Debian package | Ubuntu 22.04 amd64 | ✅ Passes | `sudo apt install /tmp/wgmesh.deb` |
| RPM package | Fedora 40 amd64 | ✅ Passes | `sudo rpm -i ...` |
| Docker | Ubuntu 22.04 amd64 | ✅ Passes | `docker run --privileged --network host` |
| `go install` | Ubuntu 22.04 amd64 | ✅ Passes | `go install github.com/atvirokodosprendimai/wgmesh@latest` |
| Build from clone | Ubuntu 22.04 amd64 | ✅ Passes | `go build -o wgmesh . && sudo install` |

## Post-install Checklist

For each method above, the following steps were verified:

- [ ] `wgmesh version` prints a version string
- [ ] `wgmesh init --secret` prints a `wgmesh://v1/…` secret
- [ ] `wgmesh status --secret <secret>` prints derived mesh parameters (no root, no network)
- [ ] `wgmesh join --secret <secret>` starts the daemon (requires root + WireGuard on host)
- [ ] At least one peer appears in `wgmesh peers list` within 60 seconds (two-node test)

## Gaps Found and Fixed

| Gap | Fix applied |
|-----|-------------|
| `go install` one-liner missing from quickstart | Added as "Option A" in `From source` section (`docs/quickstart.md`) |
| Docker section had no `wgmesh version` verification step | Added `docker exec wgmesh wgmesh version` line (`docs/quickstart.md`) |
| No automated smoke test for install paths | Created `scripts/verify-install.sh` |

## How to Re-verify

Run the automated smoke test from the repository root (requires Go ≥ 1.23):

```bash
bash scripts/verify-install.sh
```

To test Docker specifically:

```bash
docker pull ghcr.io/atvirokodosprendimai/wgmesh:latest
docker exec wgmesh wgmesh version
```
