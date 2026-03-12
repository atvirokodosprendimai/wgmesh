---
title: "fix: Integration test data plane checksum mismatch"
type: fix
status: active
date: 2026-03-12
---

# fix: Integration test data plane checksum mismatch

## Overview

All Hetzner integration tests (tiers 1-7) fail at the `verify_data_plane` gate due to systematic `mesh_transfer` checksum mismatches on every node pair. Individual test cases PASS, but the data plane gate (introduced in commit `c960598` on Feb 19) fails every tier.

This blocks all releases since PR #430 gates releases behind integration test success.

## Problem Statement

**Symptom:** Every `mesh_transfer` call produces `result=mismatch` — 100% failure rate across all node pairs, all tiers, multiple runs (v0.2.0-rc1 on March 1, v0.2.0 on March 12).

**Root cause analysis:** The `mesh_transfer` function in `testlab/cloud/lib.sh:321-390` uses OpenBSD `nc` (netcat) for TCP data transfer over the WireGuard mesh. The pattern is:

```
1. Start receiver: nc -l $port > /tmp/mesh-rx.bin &
2. Probe until listener ready: nc -z -w 1 $to_ip $port
3. Generate 1MB random data, tee to file + sha256sum
4. Send: nc -w 10 $to_ip $port < /tmp/mesh-tx.bin
5. sleep 2
6. Checksum receiver file: sha256sum /tmp/mesh-rx.bin
7. Compare checksums
```

**Likely failure modes** (ranked by probability):

### 1. Receiver `nc` exits before all data is written (HIGH)

OpenBSD `nc -l` closes the connection after the first client disconnects. If the sender's `nc -w 10` completes its write and closes the TCP connection, the receiver `nc` exits. However, there's a race: the sender may close TCP before the receiver has flushed all data to disk via the `>` redirect.

The `sleep 2` between send and checksum may be insufficient — on high-latency cross-DC paths (Helsinki-Frankfurt), TCP buffers may not have drained.

### 2. `nc -z` probe consumes the listener (MEDIUM)

OpenBSD `nc -l` (without `-k`) accepts one connection and exits. The `nc -z` probe from the sender opens a TCP connection to check readiness — but this may cause the listener to accept and close, exhausting its single-connection listener before the actual data transfer begins.

If this happens, the send `nc -w 10 $to_ip $port` connects to nothing, `/tmp/mesh-rx.bin` is empty (0 bytes from the probe), and checksums naturally mismatch.

### 3. SSH command buffering corrupts hash capture (LOW)

The `src_hash` is captured via `run_on "$from" "dd ... | tee ... | sha256sum | awk ..."`. If `run_on` (SSH) has output buffering issues, the hash string could be truncated or contain extra whitespace/newlines.

## Evidence

**From v0.2.0 run (22995657773) tier-1 trace.jsonl:**
```
data_transfer  node-d->node-b     result=mismatch
data_transfer  node-d->node-c     result=mismatch
data_transfer  node-d->node-a     result=mismatch
data_transfer  node-d->introducer result=mismatch
data_transfer  node-b->node-c     result=mismatch
data_transfer  node-b->node-a     result=mismatch
data_transfer  node-b->introducer result=mismatch
data_transfer  node-c->node-a     result=mismatch
data_transfer  node-c->introducer result=mismatch
data_transfer  node-a->introducer result=mismatch
```

All 10 pairs fail. MTU checks (`data_mtu`) all pass. All 4 individual tests PASS. The tier fails at `verify_data_plane` (line 1164 of `test-cloud.sh`).

**Timeline:**
- Feb 19: `c960598` merges data plane validation. Last successful CI run (22165702364) was either before this commit or didn't exercise the gate
- March 1: v0.2.0-rc1 — first tag to trigger integration tests with data plane gate, all tiers fail
- March 12: v0.2.0 — same pattern, all tiers fail

## Proposed Solution

Fix `mesh_transfer` in `testlab/cloud/lib.sh` to handle OpenBSD netcat correctly.

### Phase 1: Fix `nc -z` probe consuming the listener

The probe `nc -z -w 1 $to_ip $port` likely consumes the single-connection `nc -l` listener. Fix by using `-k` (keep-listening) on the receiver, or switch the readiness check to not open a connection.

```bash
# Option A: Use -k flag (keep listening for multiple connections)
nc -l -k $port > /tmp/mesh-rx.bin 2>/dev/null &

# Option B: Replace nc -z probe with port check that doesn't consume listener
# Use ss/netstat to check listener is bound, via SSH on the receiver
run_on "$to" "ss -tlnp | grep -q ':$port'" 2>/dev/null
```

**Recommended: Option B** — avoids consuming the listener entirely.

### Phase 2: Fix receiver data flush race

Replace `sleep 2` with an active wait for the receiver `nc` process to exit:

```bash
# After sending, wait for receiver nc to exit (means connection closed + data flushed)
local nc_pid
nc_pid=$(cat "/tmp/_nc_pid_$$" 2>/dev/null)
if [ -n "$nc_pid" ]; then
    # Wait up to 30s for nc to exit (it exits when sender closes connection)
    local wait_count=0
    while run_on "$to" "kill -0 $nc_pid 2>/dev/null" && [ "$wait_count" -lt 30 ]; do
        sleep 1
        ((wait_count++))
    done
fi
# Add sync to ensure write buffers are flushed
run_on "$to" "sync" 2>/dev/null
```

### Phase 3: Add diagnostic logging

When mismatch occurs, log both file sizes and first/last bytes to aid debugging:

```bash
log_error "mesh_transfer $from->$to: checksum MISMATCH"
log_error "  src: hash=$src_hash size=$(run_on "$from" "wc -c < /tmp/mesh-tx.bin")"
log_error "  dst: hash=$dst_hash size=$(run_on "$to" "wc -c < /tmp/mesh-rx.bin")"
```

### Phase 4: Verify fix

- [ ] Run integration tests on `main` with the fix (`workflow_dispatch`)
- [ ] Confirm all data plane gates pass
- [ ] Tag v0.2.1-rc1 to verify the full release pipeline (gate + release)

## Acceptance Criteria

- [ ] `mesh_transfer` succeeds on all node pairs in tier 1 (data_transfer result=ok)
- [ ] `verify_data_plane` gate passes in all tiers
- [ ] All 7 tiers complete successfully
- [ ] Fix does not increase test runtime significantly (< 5s overhead per pair)
- [x] Diagnostic logging added for future debugging

## Technical Considerations

- **nc -l vs nc -lk**: OpenBSD `nc` supports `-k` for keep-listening. Using it means the receiver won't auto-exit after the transfer — need to explicitly kill it.
- **Cross-DC latency**: Helsinki-Frankfurt latency is ~20ms. With 1MB of data, TCP should complete in <1s, but kernel buffer flushes may add delay.
- **Concurrent transfers**: `mesh_transfer` uses a fixed port 19999. If called concurrently (it's not currently), port collision would cause failures.
- **WireGuard MTU**: Standard Ethernet MTU minus WireGuard overhead gives ~1372 bytes payload. The `data_mtu` checks pass, confirming the tunnel itself works.

## Files to Modify

- `testlab/cloud/lib.sh` — `mesh_transfer` function (lines 321-390)

## Sources

- Failed run: https://github.com/atvirokodosprendimai/wgmesh/actions/runs/22995657773
- Data plane validation commit: `c960598`
- Test infrastructure: `testlab/cloud/lib.sh`, `testlab/cloud/test-cloud.sh`
- Related: PR #430 (release gate depends on these tests passing)
