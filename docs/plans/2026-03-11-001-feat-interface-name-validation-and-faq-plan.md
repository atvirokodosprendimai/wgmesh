---
title: "feat: Interface name validation and FAQ"
type: feat
status: completed
date: 2026-03-11
---

# feat: Interface name validation and FAQ

## Overview

A user asks: "Can I use an arbitrary interface name like `cloudroof0`?"

**Answer: Yes on Linux, no on macOS** — but the codebase has zero validation, so invalid names fail deep in OS commands with unclear errors. Worse, the centralized SSH deployment path interpolates the interface name into shell strings via `fmt.Sprintf` + `client.Run()`, creating a **shell injection vector**.

This plan adds input validation, hardens the shell-exposed paths, and documents the rules in an FAQ.

## Problem Statement / Motivation

1. **No validation exists.** Invalid names (too long, path traversal, shell metacharacters) only fail at the OS level with cryptic errors.
2. **Shell injection in centralized deploy.** `pkg/wireguard/apply.go` passes interface names through `fmt.Sprintf` into SSH commands — a name like `wg0; rm -rf /` would execute on the remote host.
3. **Systemd template shell exposure.** `pkg/daemon/systemd.go` wraps ExecStart in `sh -c 'exec ...'` — a name containing `'` breaks the quoting.
4. **No documentation.** Users don't know the platform constraints or what names are valid.

## Proposed Solution

Three-layer defense:

1. **`ValidateInterfaceName()` function** — standalone, called from `NewConfig()` AND centralized deploy entry points
2. **Shell-escape hardening** — defense-in-depth in `apply.go` and `systemd.go` even after validation
3. **FAQ documentation** — user-facing rules with examples

## Technical Considerations

### Validation Rules

**Linux:**
- Regex: `^[a-zA-Z][a-zA-Z0-9_-]*$`
- Max 15 chars (kernel IFNAMSIZ = 16 including null terminator)
- Reject: `/`, `\0`, `..`, spaces, shell metacharacters, names starting with `-`

**macOS:**
- Regex: `^utun[0-9]+$`
- `wireguard-go` on macOS only creates utun interfaces — anything else fails

**Both platforms:**
- Empty string is NOT an error — it falls through to platform default (`wg0` / `utun20`)
- Validation is OS-aware via `runtime.GOOS`

### Shell Injection Vectors (from SpecFlow analysis)

| Path | File | Risk | Mitigation |
|------|------|------|------------|
| Local exec.Command | `daemon/helpers.go` | Low (argv-based) | Validation only |
| SSH deployment | `wireguard/apply.go:63-91` | **Critical** (shell string) | Validation + shell-escape |
| Systemd unit | `daemon/systemd.go:19` | Medium (sh -c wrapper) | Validation + shell-escape |

### Files Keyed by Interface Name

- `/var/lib/wgmesh/<iface>.json` — daemon state
- `/var/lib/wgmesh/<iface>-peers.json` — peer cache
- `/var/lib/wgmesh/<iface>-<tag>-dht.nodes` — DHT bootstrap
- `/var/lib/wgmesh/<iface>.reload` — hot-reload config
- `/etc/wireguard/<iface>.conf` — centralized deploy
- `/tmp/wg-key-<iface>` — temp key file in apply.go

Path traversal in the name (e.g. `../etc/evil`) could write files to arbitrary locations.

## Acceptance Criteria

- [x] `ifname.Validate(name string) error` function in `pkg/ifname/validate.go` (extracted to avoid import cycle)
- [x] Called from `NewConfig()` when name is non-empty
- [x] Called from `ApplyFullConfiguration()` entry point in `apply.go`
- [x] OS-aware: Linux regex + 15-char limit; macOS utun pattern
- [x] Rejects path traversal (`/`, `..`, null bytes)
- [x] Rejects shell metacharacters (`;`, `$`, backtick, `'`, `"`, `|`, `&`, etc.)
- [x] Clear error messages with platform-specific guidance
- [x] Shell-escape interface name in `apply.go` SSH commands (defense-in-depth)
- [x] Shell-escape interface name in `systemd.go` template (defense-in-depth)
- [x] Tests covering: valid names, length boundary, path traversal, shell metacharacters, macOS utun enforcement, empty string passthrough, names starting with `-`
- [x] FAQ entry documenting rules with examples

## Implementation Phases

### Phase 1: Validation function + tests

**Files to create/modify:**
- Create `pkg/daemon/validate.go` — `ValidateInterfaceName()` function
- Create `pkg/daemon/validate_test.go` — comprehensive test table
- Modify `pkg/daemon/config.go` — call validation in `NewConfig()`

### Phase 2: Harden shell-exposed paths

**Files to modify:**
- `pkg/wireguard/apply.go` — call validation at entry points + shell-escape in sprintf
- `pkg/daemon/systemd.go` — shell-escape interface name in template

### Phase 3: FAQ documentation

**Files to create:**
- `docs/FAQ.md` — interface naming rules, examples, platform differences

## Documented Limitations (not addressed in this plan)

- **Single systemd instance:** `wgmesh.service` is hardcoded — only one interface per host via systemd. Could become `wgmesh@<iface>.service` in future.
- **Existing invalid names:** If a node already runs with a name that would now fail validation, the daemon would refuse to start after upgrade. Mitigation: log a warning instead of hard-failing for names loaded from persisted state.

## Sources & References

- `pkg/daemon/config.go:19-21` — DefaultInterface constants
- `pkg/daemon/config.go:68-75` — default resolution in NewConfig
- `pkg/daemon/helpers.go:92-163` — interface creation (Linux `ip link`, macOS `wireguard-go`)
- `pkg/wireguard/apply.go:63-91` — SSH shell injection surface
- `pkg/daemon/systemd.go:19` — systemd sh -c template
- `eidos/spec - daemon lifecycle - secret-derived identity with interface setup and hot-reload.md` — interface name is not hot-reloadable
- Linux IFNAMSIZ: 16 bytes (15 usable chars)
