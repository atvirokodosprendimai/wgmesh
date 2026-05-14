# Specification: Issue #571

## Classification
feature

## Deliverables
code

## Problem Analysis

The `hetzner-integration.yml` workflow comment (line 79) reads:

```
Tier 5 (NAT Simulation):   ~1 min  (all SKIP — not yet implemented)
```

Since that comment was written, `testlab/cloud/test-cloud.sh` gained T25 (Cone NAT), T26
(Symmetric NAT), and T27 (Mixed NAT topology), and `testlab/cloud/chaos.sh` gained `nat_setup()`
/ `nat_teardown()`. Those three tests verify that peering **succeeds** under NAT. However, two
acceptance criteria from issue #571 remain unmet:

1. **No relay-fallback activation test.** T25–T27 only assert the positive outcome (peering
   eventually works). They do not assert that relay activation specifically is what produced
   connectivity, and they do not fail when relay silently drops and traffic is blacked out for
   30 s (the #457/#556 symptom: "2 min online, 2 min offline").
2. **No route-flap threshold test.** Nothing counts WireGuard endpoint transitions during a
   sustained NAT soak and fails when the count exceeds 2 in a 5-minute window.

Additionally, the stale workflow comment causes two operational problems:

* `release.yml` requires `Hetzner Integration Tests` to complete with `conclusion == 'success'`.
  The comment "all SKIP" encouraged the misconception that tier 5 adds no gating signal, leading
  founders and maintainers to reach for the `workflow_dispatch + skip_integration_check` escape
  hatch as a matter of course rather than as a true emergency.
* The comment will remain wrong after this fix unless explicitly updated.

The fix is surgical: add two new tests (T28 and T29) to tier 5, renumber the existing tier 6
tests (old T28–T38 → new T30–T40) and tier 7 tests (old T39–T41 → new T41–T43) to keep the
global sequence gap-free, update `TOTAL_TESTS_IN_TIER` for affected tiers, add one shared
helper to `testlab/cloud/lib.sh`, and fix the workflow comment.

## Proposed Approach

1. Add helper `wg_peer_endpoint` to `testlab/cloud/lib.sh` (returns the current WireGuard
   endpoint string for a mesh peer, as seen by a given node).
2. Add test `test_t28_nat_relay_required` to `testlab/cloud/test-cloud.sh`:
   symmetric NAT + WireGuard hole-punch blocked → relay must activate → sustained ping loop
   must see ≤ 5 consecutive failures at any point in 5 minutes.
3. Add test `test_t29_nat_route_flap` to `testlab/cloud/test-cloud.sh`:
   cone NAT → monitor WireGuard endpoint transitions on a reference node for 5 minutes →
   fail if > 2 transitions observed.
4. Add both tests to the tier 5 `run_test` block; update `TOTAL_TESTS_IN_TIER=5`.
5. Renumber tier 6 function names and labels (T28→T30 through T38→T40); update
   `TOTAL_TESTS_IN_TIER=11` (count unchanged, renaming only).
6. Renumber tier 7 function names and labels (T39→T41, T40→T42, T41→T43); update
   `TOTAL_TESTS_IN_TIER=3` (count unchanged, renaming only).
7. Fix the workflow comment on line 79 of `.github/workflows/hetzner-integration.yml`.

## Implementation Tasks

### Task 1: Add `wg_peer_endpoint` helper to `testlab/cloud/lib.sh`

**File:** `testlab/cloud/lib.sh`

Insert the following function **after** the `wg_active_peer_count()` function (after line 273):

```bash
# Get the WireGuard endpoint for a peer identified by its mesh IP, as seen
# from a given node.  Uses `wg show <iface> dump` (tab-separated fields):
#   pubkey  psk  endpoint  allowed-ips  last-handshake  rx  tx  keepalive
# Returns the endpoint string (e.g. "1.2.3.4:51821") or empty string.
# Usage: wg_peer_endpoint <from_node> <mesh_ip>
#
# Pattern mirrors wg_handshake_age(): $WG_INTERFACE and $mesh_ip expand
# locally before SSH; \$aips / \$endpoint are escaped for the remote shell.
wg_peer_endpoint() {
    local from="$1" mesh_ip="$2"
    run_on "$from" "
        wg show $WG_INTERFACE dump 2>/dev/null | while IFS=$'\t' read -r pubkey psk endpoint aips hs rx tx ka; do
            if echo \"\$aips\" | grep -qF '$mesh_ip'; then
                echo \"\$endpoint\"
                exit 0
            fi
        done
    " 2>/dev/null
}
```

### Task 2: Add `test_t28_nat_relay_required` to `testlab/cloud/test-cloud.sh`

**File:** `testlab/cloud/test-cloud.sh`

Insert the following function **immediately after** `test_t27_mixed_nat()` (after the closing
`}` of that function, before the `# === TIER 6` comment block):

```bash
# --- T28: NAT relay required (relay fallback must activate) ---
# Reproduces the #457/#556 failure class: symmetric NAT + hole-punch blocked
# → wgmesh must fall back to relay; traffic must stay alive with at most
# 5 consecutive ping failures during a 5-minute soak.
test_t28_nat_relay_required() {
    _nat_pick_roles || return $?
    _chaos_setup

    # Set up symmetric NAT.  Symmetric NAT assigns a different source port for
    # each destination, so classic hole-punch never succeeds.
    nat_setup "$NAT_GW" "$NAT_NODE" "symmetric"

    # Block WireGuard UDP from the natted node to every non-gateway peer.
    # This ensures the WireGuard handshake can NEVER complete directly, making
    # relay the only viable path.  We use an autoclear safety net (same SSH
    # command) in case cleanup hangs — mirrors the tier-3 T14 lesson.
    local autoclear_secs=400
    for peer in "${!NODE_IPS[@]}"; do
        [ "$peer" = "$NAT_NODE" ] && continue
        [ "$peer" = "$NAT_GW" ]  && continue
        local peer_ip="${NODE_IPS[$peer]}"
        run_on "$NAT_NODE" "
            iptables -A OUTPUT -d $peer_ip -p udp -j DROP
            iptables -A INPUT  -s $peer_ip -p udp -j DROP
            (sleep $autoclear_secs
             iptables -D OUTPUT -d $peer_ip -p udp -j DROP 2>/dev/null
             iptables -D INPUT  -s $peer_ip -p udp -j DROP 2>/dev/null
            ) </dev/null >/dev/null 2>&1 &
        "
    done

    restart_mesh_node "$NAT_NODE"
    sleep 15

    # Relay must activate within 120 s.
    wait_for "relay-required NAT initial peering" 120 mesh_ping "$NAT_NODE" "$NAT_GW" 1

    # Soak: 5-minute ping loop.  Count consecutive failures — more than 5 in a
    # row means relay dropped (the "2 min online, 2 min offline" pattern).
    local soak_secs=300 interval=5 max_consecutive=5
    local start consecutive_fail=0
    start=$(date +%s)

    # Pick a public reference node (not NAT_NODE, not NAT_GW)
    local ref_node=""
    for node in "${!NODE_IPS[@]}"; do
        [ "$node" = "$NAT_NODE" ] && continue
        [ "$node" = "$NAT_GW" ]  && continue
        ref_node="$node"
        break
    done
    if [ -z "$ref_node" ]; then
        log_warn "T28: no third node available for relay soak — skipping soak phase"
    else
        while [ $(( $(date +%s) - start )) -lt "$soak_secs" ]; do
            if mesh_ping "$NAT_NODE" "$ref_node" 1 2>/dev/null; then
                consecutive_fail=0
            else
                consecutive_fail=$(( consecutive_fail + 1 ))
                log_warn "T28: ping fail streak=$consecutive_fail at $(( $(date +%s) - start ))s"
                if [ "$consecutive_fail" -gt "$max_consecutive" ]; then
                    log_error "T28: relay dropped — $consecutive_fail consecutive ping failures"
                    nat_teardown "$NAT_GW" "$NAT_NODE"
                    return 1
                fi
            fi
            sleep "$interval"
        done
    fi

    nat_teardown "$NAT_GW" "$NAT_NODE"
    # Flush the WG-UDP drop rules before restart
    for peer in "${!NODE_IPS[@]}"; do
        [ "$peer" = "$NAT_NODE" ] && continue
        [ "$peer" = "$NAT_GW" ]  && continue
        local peer_ip="${NODE_IPS[$peer]}"
        run_on_ok "$NAT_NODE" "
            iptables -D OUTPUT -d $peer_ip -p udp -j DROP 2>/dev/null
            iptables -D INPUT  -s $peer_ip -p udp -j DROP 2>/dev/null
        "
    done
    sleep 5
    restart_mesh_node "$NAT_NODE"
    verify_full_mesh 60
    scan_logs_for_errors
}
```

### Task 3: Add `test_t29_nat_route_flap` to `testlab/cloud/test-cloud.sh`

**File:** `testlab/cloud/test-cloud.sh`

Insert the following function **immediately after** `test_t28_nat_relay_required()`:

```bash
# --- T29: NAT route flap stability ---
# Verifies that under cone NAT the WireGuard endpoint seen by a reference
# node transitions ≤ 2 times in a 5-minute window.
# > 2 transitions = the "2 min alive, 2 min dead" relay-flap bug.
test_t29_nat_route_flap() {
    _nat_pick_roles || return $?
    _chaos_setup

    nat_setup "$NAT_GW" "$NAT_NODE" "cone"
    restart_mesh_node "$NAT_NODE"

    # Wait for initial peering
    wait_for "T29: initial cone NAT peering" 120 mesh_ping "$NAT_NODE" "$NAT_GW" 1

    # Identify a reference node that is public (not natted, not gateway)
    local ref_node=""
    for node in "${!NODE_IPS[@]}"; do
        [ "$node" = "$NAT_NODE" ] && continue
        [ "$node" = "$NAT_GW" ]  && continue
        ref_node="$node"
        break
    done
    if [ -z "$ref_node" ]; then
        log_warn "T29: no public reference node — using gateway as reference"
        ref_node="$NAT_GW"
    fi

    # Discover NAT node's mesh IP
    populate_mesh_ips 2>/dev/null || true
    local nat_mesh_ip="${NODE_MESH_IPS[$NAT_NODE]:-}"
    if [ -z "$nat_mesh_ip" ]; then
        log_error "T29: cannot determine mesh IP of $NAT_NODE"
        nat_teardown "$NAT_GW" "$NAT_NODE"
        return 1
    fi

    # Monitor endpoint transitions for 5 minutes
    local soak_secs=300 interval=10 flap_threshold=2
    local start flaps=0 prev_endpoint=""
    start=$(date +%s)

    while [ $(( $(date +%s) - start )) -lt "$soak_secs" ]; do
        local curr_endpoint
        curr_endpoint=$(wg_peer_endpoint "$ref_node" "$nat_mesh_ip")
        if [ -n "$prev_endpoint" ] && [ -n "$curr_endpoint" ] && \
           [ "$curr_endpoint" != "$prev_endpoint" ]; then
            flaps=$(( flaps + 1 ))
            log_warn "T29: flap #$flaps at $(( $(date +%s) - start ))s: $prev_endpoint -> $curr_endpoint"
        fi
        [ -n "$curr_endpoint" ] && prev_endpoint="$curr_endpoint"
        sleep "$interval"
    done

    nat_teardown "$NAT_GW" "$NAT_NODE"
    sleep 5
    restart_mesh_node "$NAT_NODE"
    verify_full_mesh 60
    scan_logs_for_errors

    if [ "$flaps" -gt "$flap_threshold" ]; then
        log_error "T29: $flaps route flaps in ${soak_secs}s (threshold: $flap_threshold)"
        return 1
    fi
    log_info "T29: $flaps route flaps in ${soak_secs}s — within threshold"
}
```

### Task 4: Update the tier 5 run-block in `testlab/cloud/test-cloud.sh`

**File:** `testlab/cloud/test-cloud.sh`

Find the exact block:

```bash
if should_run_tier 5; then
    CURRENT_TIER=5
    TOTAL_TESTS_IN_TIER=3
    log_bold "\n=========================================="
    log_bold "  TIER 5: NAT Simulation ($TOTAL_TESTS_IN_TIER tests)"
    log_bold "=========================================="
    tier_start=$(date +%s)
    TIER_START_EPOCH[5]=$tier_start
    emit_event "tier_start" "tier_5" "tests=$TOTAL_TESTS_IN_TIER"
    run_test T25 "Cone NAT"                      test_t25_cone_nat
    run_test T26 "Symmetric NAT"                 test_t26_symmetric_nat
    run_test T27 "Mixed NAT topology"            test_t27_mixed_nat
    log_info "Tier 5 data plane gate..."
    verify_data_plane
    TIER_END_EPOCH[5]=$(date +%s)
    emit_event "tier_end" "tier_5"
    log_bold "  Tier 5 complete in $(( ${TIER_END_EPOCH[5]} - tier_start ))s"
fi
```

Replace it with:

```bash
if should_run_tier 5; then
    CURRENT_TIER=5
    TOTAL_TESTS_IN_TIER=5
    log_bold "\n=========================================="
    log_bold "  TIER 5: NAT Simulation ($TOTAL_TESTS_IN_TIER tests)"
    log_bold "=========================================="
    tier_start=$(date +%s)
    TIER_START_EPOCH[5]=$tier_start
    emit_event "tier_start" "tier_5" "tests=$TOTAL_TESTS_IN_TIER"
    run_test T25 "Cone NAT"                      test_t25_cone_nat
    run_test T26 "Symmetric NAT"                 test_t26_symmetric_nat
    run_test T27 "Mixed NAT topology"            test_t27_mixed_nat
    run_test T28 "NAT relay required"            test_t28_nat_relay_required
    run_test T29 "NAT route flap stability"      test_t29_nat_route_flap
    log_info "Tier 5 data plane gate..."
    verify_data_plane
    TIER_END_EPOCH[5]=$(date +%s)
    emit_event "tier_end" "tier_5"
    log_bold "  Tier 5 complete in $(( ${TIER_END_EPOCH[5]} - tier_start ))s"
fi
```

### Task 5: Renumber tier 6 tests (T28–T38 → T30–T40)

**File:** `testlab/cloud/test-cloud.sh`

The new T28 and T29 in tier 5 would collide with the existing tier 6 T28/T29 labels. Rename
all tier 6 function names and their corresponding `run_test` labels by adding 2 to each number:

| Old function            | New function            | Old label | New label |
|-------------------------|-------------------------|-----------|-----------|
| `test_t28_rapid_cycling`  | `test_t30_rapid_cycling`  | T28       | T30       |
| `test_t29_rolling_restart`| `test_t31_rolling_restart`| T29       | T31       |
| `test_t30_simultaneous_restart` | `test_t32_simultaneous_restart` | T30 | T32 |
| `test_t31_random_chaos`   | `test_t33_random_chaos`   | T31       | T33       |
| `test_t32_udp_flood`      | `test_t34_udp_flood`      | T32       | T34       |
| `test_t33_port_flap`      | `test_t35_port_flap`      | T33       | T35       |
| `test_t34_dns_blackhole`  | `test_t36_dns_blackhole`  | T34       | T36       |
| `test_t35_clock_skew_5min`| `test_t37_clock_skew_5min`| T35       | T37       |
| `test_t36_clock_skew_15min`| `test_t38_clock_skew_15min`| T36     | T38       |
| `test_t37_goodbye_forgery`| `test_t39_goodbye_forgery`| T37       | T39       |
| `test_t38_stale_cache`    | `test_t40_stale_cache`    | T38       | T40       |

Apply these renames:

**a) Function definitions** — rename each `test_tNN_*()` definition header (the `# --- TNN: ...`
comment line and the `test_tNN_*() {` line) for all 11 tier 6 functions.

**b) `run_test` invocations** — in the tier 6 run-block, change every `run_test TNN "..."
test_tNN_*` line to use the new numbers. The block becomes:

```bash
    run_test T30 "Rapid peer cycling"            test_t30_rapid_cycling
    run_test T31 "Rolling restart"               test_t31_rolling_restart
    run_test T32 "Simultaneous restart"          test_t32_simultaneous_restart
    run_test T33 "Random impairment rotation"    test_t33_random_chaos
    run_test T34 "UDP flood"                     test_t34_udp_flood
    run_test T35 "Port flap"                     test_t35_port_flap
    run_test T36 "DNS blackhole"                 test_t36_dns_blackhole
    run_test T37 "Clock skew +5min"              test_t37_clock_skew_5min
    run_test T38 "Clock skew +15min (isolation)" test_t38_clock_skew_15min
    run_test T39 "GOODBYE forgery resistance"    test_t39_goodbye_forgery
    run_test T40 "Stale cache resurrection"      test_t40_stale_cache
```

`TOTAL_TESTS_IN_TIER=11` for tier 6 is unchanged (count stays 11, only labels shift).

### Task 6: Renumber tier 7 tests (T39–T41 → T41–T43)

**File:** `testlab/cloud/test-cloud.sh`

| Old function         | New function         | Old label | New label |
|----------------------|----------------------|-----------|-----------|
| `test_t39_clean_soak`  | `test_t41_clean_soak`  | T39       | T41       |
| `test_t40_chaos_soak`  | `test_t42_chaos_soak`  | T40       | T42       |
| `test_t41_long_soak`   | `test_t43_long_soak`   | T41       | T43       |

Apply the same two-step rename (function definition header + run_test invocation):

```bash
    run_test T41 "5-min clean soak"              test_t41_clean_soak
    run_test T42 "10-min chaos soak"             test_t42_chaos_soak
    run_test T43 "15-min long soak with churn"   test_t43_long_soak
```

`TOTAL_TESTS_IN_TIER=3` for tier 7 is unchanged.

### Task 7: Fix the stale comment in `.github/workflows/hetzner-integration.yml`

**File:** `.github/workflows/hetzner-integration.yml`, line 79.

Find the exact line:

```yaml
  #   Tier 5 (NAT Simulation):   ~1 min  (all SKIP — not yet implemented)
```

Replace it with:

```yaml
  #   Tier 5 (NAT Simulation):  ~25 min  (T25–T29: cone, symmetric, mixed, relay-required, flap)
```

The `~25 min` estimate is based on: 3 × existing NAT tests (~5 min each) + 2 × new soak tests
(5 min each) + setup/teardown overhead, comparable to tier 4 (~15 min) with added soak time.

### Task 8: Verify

After all edits are applied, run a local syntax check:

```bash
bash -n testlab/cloud/test-cloud.sh
bash -n testlab/cloud/lib.sh
bash -n testlab/cloud/chaos.sh
```

All three must exit 0 with no output. This catches typos in the new shell functions before they
reach Hetzner CI.

## Affected Files

- `testlab/cloud/lib.sh` — add `wg_peer_endpoint()` helper (Task 1)
- `testlab/cloud/test-cloud.sh` — add `test_t28_nat_relay_required()`, `test_t29_nat_route_flap()`, update tier 5 run-block, renumber tier 6 functions and labels T28–T38→T30–T40, renumber tier 7 functions and labels T39–T41→T41–T43 (Tasks 2–6)
- `.github/workflows/hetzner-integration.yml` — fix stale comment on line 79 (Task 7)

## Test Strategy

1. `bash -n` syntax check on all three modified shell files (Task 8) — must exit 0.
2. Confirm tier 6 and 7 `TOTAL_TESTS_IN_TIER` constants remain unchanged (11 and 3) — only
   labels shift.
3. Full CI run on a `v*.*.*` tag triggers `hetzner-integration.yml` tier 5 with all 5 tests.
   Expected outcomes:
   - T25 (Cone NAT): PASS — already proved working
   - T26 (Symmetric NAT): PASS — already proved working
   - T27 (Mixed NAT): SKIP if < 5 VMs, PASS if ≥ 5 VMs
   - T28 (Relay required): should PASS on PR #564's merge commit; would FAIL if the
     `shouldRelayPeerWithSubnets` bug is re-introduced
   - T29 (Route flap): should PASS with ≤ 2 flaps; would FAIL with > 2 flaps (the "2 min
     online, 2 min offline" pattern)
4. Verify `release.yml` gates correctly: `workflow_run` → conclusion `success` on tag ref →
   GoReleaser fires only after tier 5 actually exercises NAT paths.

## Estimated Complexity
medium
