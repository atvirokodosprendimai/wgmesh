---
title: Bash ( cmd ) & subshell PID mismatch orphans ssh and hangs Hetzner integration CI for 2 hours
date: 2026-05-15
category: test-failures
module: testlab/cloud
problem_type: test_failure
component: testing_framework
severity: critical
symptoms:
  - "Hetzner integration runs cancelled at GitHub Actions job timeout (120/180/240 min consumed identically)"
  - "Tier 6 (Chaos Monkey) hung indefinitely on every CI run despite repeated timeout bumps"
  - "trace.jsonl shows only ~15 min of activity inside a 244-min job — a 229-min silent gap after the last test event"
  - "`kill -KILL $pid` returns success but the ssh child process keeps the TCP connection open"
  - "Watchdog `::error::` diagnostics never surface; bare `wait` blocks despite SIGKILL"
  - "Same hang pattern recurred at 4 separate call sites after each surface-level fix"
root_cause: logic_error
resolution_type: code_fix
related_components:
  - tooling
  - development_workflow
tags:
  - bash
  - subshell
  - ssh
  - ci-hang
  - watchdog
  - hetzner
  - integration-tests
  - process-management
  - timeout
---

# Bash `( cmd ) &` subshell PID mismatch orphans ssh and hangs Hetzner integration CI for 2 hours

## Problem

The Hetzner integration test suite hung for ~2 hours on every CI run, cancelled only when GitHub Actions hit its job-level timeout (120 min initially, raised to 180 then 240 — every ceiling consumed identically). Root cause: bash `( cmd ) &` forks a subshell, so `$!` captures the **subshell PID**, not the actual command PID. Watchdog SIGKILL kills only the subshell wrapper; the ssh child becomes orphaned, retains its TCP connection, and blocks the parent shell's `wait` indefinitely. The same anti-pattern bit in four separate test-script sites.

## Symptoms

- CI runs cancelled at the GitHub Actions job timeout. Bumping the ceiling never helped — Tier 6 consumed 120 min, 180 min, and 240 min identically.
- `trace.jsonl` artifact from cancelled run 25919050735 showed only 14.8 minutes of actual test events inside a 244-minute job duration. The last event was `chaos_setup_end` for T30 "Simultaneous restart"; the next 229 minutes were silent.
- No watchdog diagnostics ever surfaced — `cmd 2>/dev/null || true` on cleanup paths swallowed the `::error::` messages from PR #615's watchdog.
- Orphaned `ssh` processes were observable on the runner with the killed subshell's PID gone but the ssh child still holding sockets.

## What Didn't Work

1. **PR #631 alone (`timeout(1)` for SSH watchdog only).** Replaced `_run_with_timeout` with `timeout(1)` in `run_on`/`run_on_ok`/`copy_to`, but Tier 6 still cancelled because `stop_mesh`'s bare `wait` still blocked on orphans created by `wait_for`'s shell-function predicate path.
2. **PR #633 (120 → 180 min) and PR #635 (180 → 240 min) timeout ceiling bumps.** Raising the cap never addressed the hang. Tier 6 just ate each new ceiling.
3. **The original "unsuppress stderr" theory (PR #617).** Removing `2>/dev/null` from `stop_mesh` call sites surfaced the watchdog signal — but the watchdog itself was broken (PID mismatch), so making it visible didn't fix the hang. (session history)
4. **Coroot eBPF observability detour (PRs #600-#614).** Burned several diagnostic runs trying to get kernel-level visibility. The Coroot agent had its own auth bug (missing `--api-key` in ExecStart) that masked progress. In the end, plain `trace.jsonl` timing-gap analysis pinpointed the line — Coroot dashboards contributed nothing to the root-cause discovery. (session history)

## Solution

Four sites required fixes — all variants of the same PID-mismatch pattern.

### Fix 1 (PR #631) — `_run_with_timeout` → `timeout(1)`

In `testlab/cloud/lib.sh`:

Before:
```bash
_run_with_timeout() {
    local timeout_sec="$1"; shift
    ( "$@" ) &
    local pid=$!                 # subshell pid, NOT cmd pid
    # ... watchdog poll loop ...
    kill -KILL "$pid"            # kills subshell wrapper
    wait "$pid"                  # subshell reaped, ssh orphaned
}

run_on() {
    _run_with_timeout "$t" ssh "${SSH_OPTS[@]}" "root@${ip}" "$@"
}
```

After:
```bash
run_on() {
    local t="${RUN_ON_TIMEOUT_SEC:-300}"
    timeout --kill-after=5 "$t" ssh "${SSH_OPTS[@]}" "root@${ip}" "$@"
    # timeout(1) exec's ssh directly — no subshell, no PID mismatch.
    # --kill-after=5 sends SIGKILL 5s after SIGTERM if ssh ignores it.
}
```

Also added `TimeoutStopSec=10` + `KillMode=mixed` to the wgmesh systemd unit in `provision.sh` so `systemctl stop wgmesh` always returns within 10s, regardless of daemon shutdown behavior.

### Fix 2 (PR #632) — `stop_mesh` bare `wait`

In `testlab/cloud/provision.sh`:

Before:
```bash
stop_mesh() {
    for node in "${!NODE_IPS[@]}"; do
        stop_mesh_node "$node" &
    done
    wait   # blocks on ALL bg children of the current shell, including orphans
}
```

After:
```bash
stop_mesh() {
    local pids=()
    for node in "${!NODE_IPS[@]}"; do
        stop_mesh_node "$node" &
        pids+=($!)
    done
    local pid
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done
}
```

### Fix 3 (PR #632) — `wait_for` predicate descendants

In `testlab/cloud/lib.sh`. `wait_for` runs **shell-function** predicates (`_t1_check`, `_t5_check`, etc.) inside `( "$@" 2>/dev/null ) &`, so it cannot switch to `timeout(1)` — `timeout` execs an external program and would lose shell function visibility. Instead, kill descendants explicitly with `pkill -P` before reaping the subshell wrapper:

```bash
if [ "$probe_done" -eq 0 ]; then
    pkill -KILL -P "$probe_pid" 2>/dev/null || true
    sleep 0.2
    pkill -KILL -P "$probe_pid" 2>/dev/null || true   # grandchildren
    kill -KILL "$probe_pid" 2>/dev/null || true
    wait "$probe_pid" 2>/dev/null || true
fi
```

### Fix 4 (PR #636) — `test_t30_simultaneous_restart`

In `testlab/cloud/test-cloud.sh`. Same pattern as `stop_mesh`. This was the **last** bare-`wait`-after-`&` site in the test scripts:

```bash
test_t30_simultaneous_restart() {
    _chaos_setup
    local pids=()
    for node in "${!NODE_IPS[@]}"; do
        restart_mesh_node "$node" &
        pids+=($!)
    done
    local pid
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done
    sleep 10
    verify_full_mesh 90
    scan_logs_for_errors
}
```

### Workflow-level fallback

PRs #633 and #635 raised the matrix `timeout-minutes` from 120 → 180 → 240 to give Tier 6 enough headroom. These bumps were not the fix — they only kept the cap from biting while real diagnostic work proceeded.

## Why This Works

In bash, `( cmd ) &` is `fork(subshell) ; subshell.fork(cmd)`. `$!` returns the subshell's PID, not `cmd`'s. When the watchdog sends `SIGKILL` to that PID, the subshell process dies but `cmd` (and any TCP connections it holds) re-parents to `init` (or the systemd user-session leader). The original parent shell's `wait` only resolves when all its tracked background children exit — including orphans it can no longer signal. An SSH connection to an unreachable or unresponsive host can hold its socket for hours via TCP keepalive/RST timeouts.

`timeout(1)` solves this because it **execs** the target command directly. The process group is rooted at `timeout` itself, so SIGKILL reaches the real ssh process. For shell-function predicates where `exec` isn't possible (the `wait_for` case), `pkill -P <pid>` walks the descendant tree and reaps grandchildren explicitly. Replacing bare `wait` with explicit PID arrays prevents the parent shell from blocking on background children it shouldn't track.

**Diagnostic insight:** the breakthrough came from `trace.jsonl` timing gaps. When run 25919050735's trace showed only 14.8 minutes of activity in a 244-minute job, the gap localized the hang to the exact line that emitted the last event — `chaos_setup_end` for T30 — far faster than instrumenting probe logging would have.

## Prevention

1. **Grep guard for the anti-pattern.** After any `for ... cmd "$x" &; done` loop, the next non-comment line must not be a bare `wait`. Use explicit PID arrays:
   ```bash
   pids=()
   for x in "${list[@]}"; do work "$x" & pids+=($!); done
   for p in "${pids[@]}"; do wait "$p" 2>/dev/null || true; done
   ```

2. **Prefer `timeout(1)` over bash-watchdog wrappers** when wrapping external commands. Reserve bash watchdogs only for shell-function predicates (the `wait_for` case), and always clean descendants with `pkill -P` before reaping the parent.

3. **Never use `( cmd ) & pid=$!`** when you intend to signal `cmd` later. The PID is the subshell's. Either drop the subshell (`cmd & pid=$!`) or switch to `timeout(1)`.

4. **Don't suppress watchdog stderr on cleanup paths.** `cmd 2>/dev/null || true` patterns hide the `::error::` diagnostics that would surface this class of hang months earlier (see PR #615 → PR #617).

5. **For systemd-managed services that may hang on stop**, set `TimeoutStopSec=10` and `KillMode=mixed` so `systemctl stop` always returns within bound, regardless of the daemon's shutdown behavior.

6. **Use `trace.jsonl` timing gaps as the first diagnostic.** A trace showing N minutes of activity inside an M-minute job (M ≫ N) localizes the hang to the line emitting the last trace event. Far faster than adding probe logging.

## Related Issues

- **`docs/solutions/test-failures/tier3-t14-80pct-loss-ssh-hang.md`** — sibling SSH-hang doc. Different root cause (`tc-netem` on eth0 + missing SSH keepalive), but same symptom family (orphan ssh on the GitHub runner). The T14 doc's prevention list should be extended with the `( cmd ) &` PID-mismatch warning.
- **GitHub issue #634** — "Tier 6 Hetzner integration timeout — needs 240min or trim chaos_setup overhead." Open follow-up: after the PID-mismatch fix, Tier 6 still needs the 240min ceiling because chaos_setup adds ~3 min per test. Consolidation work pending.
- **GitHub issue #571** (CLOSED) — Tier 5 NAT Simulation: implement and gate releases. Resolved separately while Tier 6 timeouts persisted; cross-listed for historical NAT recurrence context.
- **PRs #631, #632, #633, #635, #636** — the five-iteration fix chain.
- **Run 25930983971** (2026-05-15T17:18Z) — first fully-clean 7-tier pass in 104 min after PR #636. Triggered GoReleaser publication of **v0.3.0-rc2**.
