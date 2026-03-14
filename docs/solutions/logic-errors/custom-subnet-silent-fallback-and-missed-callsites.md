---
title: "Custom subnet feature: silent fallback to wrong address space + missed callsites"
category: logic-errors
date: 2026-03-12
tags:
  - ip-derivation
  - custom-subnet
  - collision-resolution
  - address-space
  - feature-implementation
modules:
  - pkg/daemon/collision.go
  - pkg/daemon/config.go
  - pkg/daemon/daemon.go
  - pkg/crypto/derive.go
  - pkg/mesh/mesh.go
  - pkg/mesh/deploy.go
  - service.go
  - main.go
---

# Custom subnet feature: silent fallback to wrong address space + missed callsites

## Problem

When implementing `--mesh-subnet` (custom IPv4 CIDR for mesh IP derivation), multiple bugs were introduced across the initial implementation and survived the first round of code review:

1. **Silent fallback to legacy derivation** — When `DeriveMeshIPInSubnet` failed (e.g., invalid subnet), collision resolution code silently fell back to `DeriveMeshIPWithNonce` which always produces `10.x.x.x` addresses. A node configured with `--mesh-subnet 192.168.100.0/24` would silently get a `10.x.x.x` IP, making it unreachable.

2. **IPv6 overflow** — `uint64(1) << hostBits` wraps to 0 when `hostBits >= 64` (any IPv6 subnet), producing `maxHosts = 0 - 2 = 18446744073709551614` (wrapping underflow). The modulus then produces effectively random results instead of an error.

3. **Missed callsite in service.go** — The plan explicitly listed `service.go:454,464` but the file was never updated. `deriveMeshIPForService` always called legacy `crypto.DeriveMeshIP`, producing wrong IPs for service registration.

4. **`return` instead of `continue` in collision loop** — `CheckAndResolveCollisions` used `return` on error, exiting the entire loop and silently skipping remaining collisions.

5. **Unguarded empty string from `ResolveCollision`** — When `ResolveCollision` returned `""` on error, caller logged it as the expected new IP without checking.

## Root Cause

The root pattern: **when adding a new code path alongside an existing one, error handlers in the new path fell back to the old path instead of failing explicitly**. This is natural instinct (keep things working) but wrong when the two paths produce values in incompatible domains (different IP address spaces).

Secondary: **plan listed 11 files but self-review only checked 10** — `service.go` was missed because it's in `package main` (root), not in the `pkg/` tree where all other changes lived.

## Solution

### 1. Remove all silent fallbacks (collision.go)

```go
// BEFORE (wrong): falls back to 10.x.x.x on error
if err != nil {
    log.Printf("[Collision] Failed to derive IP in custom subnet: %v", err)
    return DeriveMeshIPWithNonce(meshSubnet, loser.WGPubKey, secret, 1)  // silently switches address space!
}

// AFTER (correct): return empty string, don't silently switch address space
if err != nil {
    log.Printf("[Collision] CRITICAL: Failed to derive IP in custom subnet: %v", err)
    return ""
}
```

Applied to all 4 fallback sites in `collision.go`.

### 2. Reject IPv6 subnets (derive.go)

```go
func validateIPv4Subnet(subnet *net.IPNet) (int, error) {
    ones, bits := subnet.Mask.Size()
    if bits != 32 {
        return 0, fmt.Errorf("only IPv4 subnets are supported (got %d-bit)", bits)
    }
    hostBits := bits - ones
    if hostBits < 2 {
        return 0, fmt.Errorf("subnet /%d too small: need at least 2 host bits", ones)
    }
    return hostBits, nil
}
```

### 3. Early IPv4 validation at config boundaries (config.go, mesh.go)

```go
// In NewConfig:
if customSubnet.IP.To4() == nil {
    return nil, fmt.Errorf("mesh subnet must be an IPv4 CIDR, got %q", customSubnet.String())
}

// In InitializeWithNetwork:
ipv4 := parsedNet.IP.To4()
if ipv4 == nil {
    return fmt.Errorf("only IPv4 networks are supported")
}
```

### 4. Update service.go

Thread `customSubnet *net.IPNet` through `deriveMeshIPForService` and add `--mesh-subnet` flag to `service add` command.

### 5. Fix collision loop control flow

```go
// BEFORE: return (skips remaining collisions)
// AFTER: continue (processes remaining collisions)
if err != nil {
    log.Printf("[Collision] CRITICAL: Failed to derive IP: %v — keeping current IP", err)
    continue
}
```

## Prevention

- **When adding a parallel code path, error handlers must NOT fall back to the original path** if the two paths produce values in different domains. Fail explicitly instead.
- **After implementing a plan, diff the plan's file list against `git diff --stat`** to catch missed files. The plan listed 11 files; the initial implementation touched 10.
- **Integer overflow on bit shifts**: any `1 << n` where `n` can exceed the type width must have a guard. For IPv4 subnets, `n <= 30` is safe for `uint64`; for arbitrary input, validate first.
- **Loop control flow audit**: when changing `return` to error handling inside a `for` loop, verify whether `return` or `continue` is correct — they have very different effects.
