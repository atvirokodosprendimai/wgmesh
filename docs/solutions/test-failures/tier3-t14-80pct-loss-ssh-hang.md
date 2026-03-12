---
title: "Tier 3 T14 (80% packet loss) hangs indefinitely due to SSH control plane impairment"
category: test-failures
date: 2026-03-12
tags: [chaos-testing, ssh, tc-netem, integration-tests, bash]
modules: [testlab/cloud/chaos.sh, testlab/cloud/lib.sh, testlab/cloud/test-cloud.sh]
prs: ["#433", "#434", "#435", "#436", "#437"]
---

## Problem

Tier 3 integration test T14 (80% packet loss soak) hung for 99 minutes until the GHA 120-minute timeout killed it.
All other 6 tiers passed consistently.
This blocked the v0.2.1 release because releases are gated behind integration test success.

**Symptom:** The `chaos_clear` SSH call after the 180s soak never returned.
The GHA runner showed orphan `ssh` and `bash` processes at job cleanup.

## Root Cause (Multi-Layered)

**Layer 1: tc netem on eth0 impairs SSH itself.**
`chaos_apply` uses `tc qdisc add dev eth0 root netem loss 80%` on Hetzner Cloud VMs.
Since eth0 carries ALL traffic (including SSH), an 80% loss rate makes subsequent SSH commands unreliable.
The `chaos_clear` call after `sleep 180` had to SSH through the impaired link to remove the impairment — a circular dependency.

**Layer 2: No SSH keepalive = silent hang.**
The SSH client had no `ServerAliveInterval` configured.
When packets were lost, the TCP connection entered a half-open state.
The SSH client waited indefinitely for a response that would never arrive, rather than timing out.

**Layer 3: No safety net for impairment removal.**
If the SSH-based `chaos_clear` failed, the impairment remained indefinitely.
There was no fallback mechanism to ensure the tc qdisc was eventually removed.

## Solution

Four incremental fixes, each addressing one layer:

### 1. SSH Keepalive (PR #433, #434)

Added to all SSH helpers (`run_on`, `run_on_ok`, `copy_to`, provision SSH probe):

```bash
-o ServerAliveInterval=15
-o ServerAliveCountMax=4
```

This kills dead SSH sessions after ~60s instead of hanging indefinitely.
Later refactored into a shared `_ssh_opts()` helper (PR #437).

### 2. Auto-Clear Safety Net (PR #435)

For severe impairments (>=50% loss), schedule a self-removing timer on the remote node in the **same SSH command** as the impairment:

```bash
run_on "$node" "tc qdisc add dev $iface root netem loss ${pct}% && \
  bash -c 'echo \$\$ > /tmp/chaos-autoclear.pid; \
  (sleep $autoclear_secs; tc qdisc del dev $iface root 2>/dev/null; \
   rm -f /tmp/chaos-autoclear.pid) </dev/null >/dev/null 2>&1 &'"
```

Key insight: the auto-clear MUST be scheduled in the same SSH command as the `tc qdisc add`.
A second SSH call to schedule it would go through the already-impaired network and might fail.

### 3. Timing Alignment (PR #436)

Set `CHAOS_AUTOCLEAR_SECS=170` with `sleep 170` so the auto-clear fires and the soak ends at approximately the same time.
The `chaos_clear` call after sleep is best-effort cleanup, not the primary removal mechanism.

### 4. Input Validation (PR #436, #437)

Added `_validate_int()` helper to validate all numeric parameters passed to `chaos_apply` and `chaos_skew_clock`, preventing shell injection via unvalidated interpolation into remote SSH commands.

## Key Insight

**When applying network impairments that affect the control plane, schedule the undo mechanism atomically with the impairment itself.**
Never rely on a separate network call to undo an impairment that degrades that same network.
This is analogous to setting a hardware watchdog timer before entering a potentially-hanging operation.

## Related Issues

- **nc -z probe consuming listener** (PR #431): OpenBSD `nc -l` accepts one connection then exits.
  A `nc -z` readiness probe consumes the listener before the real data transfer.
  Fixed by switching to `ss -tlnp` for listener readiness checks.
- **NODE_MESH_IPS lost in subshell** (PR #432, #437): `run_test` used `| tee` which creates a subshell, losing associative array modifications.
  Fixed by switching to `> >(tee)` process substitution.
- **Release gating** (PR #430): Releases are gated behind integration test success via `workflow_run` trigger.

## Prevention

1. **Always use SSH keepalive** for remote operations that may traverse impaired networks.
2. **Schedule cleanup atomically** with the operation that creates the mess.
3. **Test chaos tests themselves** — a chaos test that can't clean up after itself is worse than no test.
4. **Avoid `| tee` in bash** when the left side modifies globals — use `> >(tee)` instead.
