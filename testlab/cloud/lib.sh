#!/usr/bin/env bash
# lib.sh — Shared functions for wgmesh cloud integration tests
#
# Source this file from other scripts:
#   SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
#   source "$SCRIPT_DIR/lib.sh"

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

: "${HCLOUD_TOKEN:?HCLOUD_TOKEN must be set}"
: "${SSH_KEY_FILE:=${HOME}/.ssh/wgmesh-ci}"
: "${VM_PREFIX:=wgmesh-ci}"
: "${VM_TYPE:=cax11}"
: "${VM_IMAGE:=ubuntu-24.04}"
: "${MESH_SECRET:=}"
: "${WG_INTERFACE:=wg0}"
: "${BINARY_PATH:=}"
: "${LOG_DIR:=/tmp/wgmesh-ci-logs}"
: "${TEST_TIMEOUT:=1800}"  # 30 min hard ceiling

# Node roles and locations
declare -A NODE_ROLES=()    # name -> role (introducer|node)
declare -A NODE_IPS=()      # name -> public IPv4
declare -A NODE_IPV6=()     # name -> public IPv6
declare -A NODE_MESH_IPS=() # name -> mesh IPv4
declare -A NODE_LOCATIONS=() # name -> hetzner location

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

# Test results accumulator
declare -a TEST_RESULTS=()
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Observability: timing + tracing
CURRENT_TIER=""
SUITE_START_EPOCH=$(date +%s)
declare -A TIER_START_EPOCH=()
declare -A TIER_END_EPOCH=()
declare -a TEST_TIMING_EVENTS=()   # "tier|id|name|start_epoch|end_epoch|result"
TRACE_FILE="${LOG_DIR}/trace.jsonl"
mkdir -p "$LOG_DIR"

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------

log_info()  { echo -e "${GREEN}[INFO]${NC}  $(date +%H:%M:%S) $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $(date +%H:%M:%S) $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $(date +%H:%M:%S) $*"; }
log_test()  { echo -e "${BLUE}[TEST]${NC}  $(date +%H:%M:%S) $*"; }
log_bold()  { echo -e "${BOLD}$*${NC}"; }

# ---------------------------------------------------------------------------
# NDJSON event tracing (Phase 2 observability)
# ---------------------------------------------------------------------------

# Emit a structured event to the trace file.
# Usage: emit_event <type> <name> [key=value ...]
emit_event() {
    local etype="$1" ename="$2"; shift 2
    local extra=""
    while [ $# -gt 0 ]; do
        local k="${1%%=*}" v="${1#*=}"
        # Escape quotes in values
        v="${v//\"/\\\"}"
        extra="${extra},\"${k}\":\"${v}\""
        shift
    done
    printf '{"ts":%.3f,"type":"%s","name":"%s","tier":"%s"%s}\n' \
        "$(date +%s.%N)" "$etype" "$ename" "${CURRENT_TIER:-0}" "$extra" \
        >> "$TRACE_FILE" 2>/dev/null || true
}

# ---------------------------------------------------------------------------
# SSH helpers
# ---------------------------------------------------------------------------

# Run a command on a remote node via SSH.
# Usage: run_on <node-name> <command...>
run_on() {
    local node="$1"; shift
    local ip="${NODE_IPS[$node]}"
    ssh -o StrictHostKeyChecking=no \
        -o UserKnownHostsFile=/dev/null \
        -o ConnectTimeout=10 \
        -o LogLevel=ERROR \
        -i "$SSH_KEY_FILE" \
        "root@${ip}" "$@"
}

# Run a command on a remote node, tolerating failure.
# Returns the exit code without aborting.
run_on_ok() {
    local node="$1"; shift
    local ip="${NODE_IPS[$node]}"
    ssh -o StrictHostKeyChecking=no \
        -o UserKnownHostsFile=/dev/null \
        -o ConnectTimeout=10 \
        -o LogLevel=ERROR \
        -i "$SSH_KEY_FILE" \
        "root@${ip}" "$@" 2>/dev/null || true
}

# Copy a file to a remote node.
# Usage: copy_to <node-name> <local-path> <remote-path>
copy_to() {
    local node="$1" src="$2" dst="$3"
    local ip="${NODE_IPS[$node]}"
    scp -o StrictHostKeyChecking=no \
        -o UserKnownHostsFile=/dev/null \
        -o LogLevel=ERROR \
        -i "$SSH_KEY_FILE" \
        "$src" "root@${ip}:${dst}"
}

# Run a command on ALL nodes in parallel, wait for all.
# Usage: run_on_all <command...>
run_on_all() {
    local pids=()
    for node in "${!NODE_IPS[@]}"; do
        run_on "$node" "$@" &
        pids+=($!)
    done
    local rc=0
    for pid in "${pids[@]}"; do
        wait "$pid" || rc=1
    done
    return $rc
}

# ---------------------------------------------------------------------------
# Wait / polling helpers
# ---------------------------------------------------------------------------

# Wait until a condition command succeeds, with timeout.
# Usage: wait_for <description> <timeout_sec> <command...>
wait_for() {
    local desc="$1" timeout="$2"; shift 2
    local start end
    start=$(date +%s)
    end=$((start + timeout))

    while true; do
        if "$@" 2>/dev/null; then
            local elapsed=$(( $(date +%s) - start ))
            log_info "$desc — succeeded after ${elapsed}s"
            return 0
        fi
        if [ "$(date +%s)" -ge "$end" ]; then
            local elapsed=$(( $(date +%s) - start ))
            log_error "$desc — timed out after ${elapsed}s"
            return 1
        fi
        sleep 2
    done
}

# ---------------------------------------------------------------------------
# Mesh operations
# ---------------------------------------------------------------------------

# Start wgmesh on a node via systemd.
start_mesh_node() {
    local node="$1"
    local role="${NODE_ROLES[$node]}"
    local extra=""
    [ "$role" = "introducer" ] && extra="--introducer"

    run_on "$node" "systemctl start wgmesh"
    log_info "Started wgmesh on $node (role=$role)"
}

# Stop wgmesh on a node.
stop_mesh_node() {
    local node="$1"
    run_on_ok "$node" "systemctl stop wgmesh"
    log_info "Stopped wgmesh on $node"
}

# Kill wgmesh with SIGKILL (simulate crash).
crash_mesh_node() {
    local node="$1"
    run_on_ok "$node" "kill -9 \$(pgrep wgmesh) 2>/dev/null; ip link del $WG_INTERFACE 2>/dev/null"
    log_info "Crashed wgmesh on $node (SIGKILL)"
}

# Restart wgmesh on a node.
restart_mesh_node() {
    local node="$1"
    run_on "$node" "systemctl restart wgmesh"
    log_info "Restarted wgmesh on $node"
}

# Generate a fresh mesh secret.
generate_mesh_secret() {
    # Use openssl to generate a random 32-byte key, base64url-encode it
    local key
    key=$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=')
    echo "wgmesh://v1/${key}"
}

# ---------------------------------------------------------------------------
# Mesh verification
# ---------------------------------------------------------------------------

# Check if node A can ping node B's mesh IP.
# Usage: mesh_ping <from-node> <to-node> [count]
mesh_ping() {
    local from="$1" to="$2" count="${3:-3}"
    local to_ip="${NODE_MESH_IPS[$to]}"
    run_on "$from" "ping -c $count -W 3 $to_ip" >/dev/null 2>&1
}

# Check if node A can ping node B's mesh IPv6.
mesh_ping6() {
    local from="$1" to="$2" count="${3:-3}"
    # Get mesh IPv6 from wg show
    local to_ip6
    to_ip6=$(run_on "$to" "wg show $WG_INTERFACE allowed-ips 2>/dev/null | grep -oP 'fd[0-9a-f:]+' | head -1" 2>/dev/null) || return 1
    [ -z "$to_ip6" ] && return 1
    run_on "$from" "ping6 -c $count -W 3 $to_ip6" >/dev/null 2>&1
}

# Get WG handshake age for a specific peer on a node.
# Returns seconds since last handshake, or 999999 if no handshake.
wg_handshake_age() {
    local node="$1" peer_mesh_ip="$2"
    run_on "$node" "
        now=\$(date +%s)
        wg show $WG_INTERFACE dump 2>/dev/null | while IFS=$'\t' read -r pubkey psk endpoint aips handshake rx tx ka; do
            if echo \"\$aips\" | grep -q '$peer_mesh_ip'; then
                if [ \"\$handshake\" -gt 0 ] 2>/dev/null; then
                    echo \$(( now - handshake ))
                else
                    echo 999999
                fi
                exit 0
            fi
        done
        echo 999999
    " 2>/dev/null
}

# Count active WG peers (with handshake < 180s) on a node.
wg_active_peer_count() {
    local node="$1"
    run_on "$node" "
        now=\$(date +%s)
        count=0
        wg show $WG_INTERFACE dump 2>/dev/null | tail -n +2 | while IFS=$'\t' read -r pubkey psk endpoint aips handshake rx tx ka; do
            if [ \"\$handshake\" -gt 0 ] 2>/dev/null; then
                age=\$(( now - handshake ))
                if [ \$age -lt 180 ]; then
                    count=\$(( count + 1 ))
                fi
            fi
        done
        echo \$count
    " 2>/dev/null
}

# Check all N*(N-1)/2 pairs are connected.
# Usage: verify_full_mesh [timeout_sec]
verify_full_mesh() {
    local timeout="${1:-90}"
    local nodes=("${!NODE_IPS[@]}")
    local n=${#nodes[@]}
    local expected_pairs=$(( n * (n - 1) / 2 ))

    emit_event "mesh_verify_start" "full_mesh" "pairs=$expected_pairs" "timeout=$timeout"
    wait_for "full mesh ($expected_pairs pairs)" "$timeout" _check_all_pairs "${nodes[@]}"
    emit_event "mesh_verify_end" "full_mesh" "pairs=$expected_pairs"
}

_check_all_pairs() {
    local nodes=("$@")
    local n=${#nodes[@]}
    for (( i=0; i<n; i++ )); do
        for (( j=i+1; j<n; j++ )); do
            mesh_ping "${nodes[$i]}" "${nodes[$j]}" 1 || return 1
        done
    done
    return 0
}

# Verify full mesh excluding a specific node.
verify_mesh_without() {
    local excluded="$1" timeout="${2:-60}"
    local nodes=()
    for node in "${!NODE_IPS[@]}"; do
        [ "$node" = "$excluded" ] || nodes+=("$node")
    done
    local n=${#nodes[@]}
    local expected_pairs=$(( n * (n - 1) / 2 ))

    wait_for "mesh without $excluded ($expected_pairs pairs)" "$timeout" _check_all_pairs "${nodes[@]}"
}

# ---------------------------------------------------------------------------
# Data plane verification (TCP transfer, throughput, MTU)
#
# These go beyond ICMP ping to verify actual data flows through WG tunnels.
# ---------------------------------------------------------------------------

# Transfer data between two nodes over the mesh and verify integrity.
# Uses netcat (nc) to send random data over TCP via mesh IPs.
# Usage: mesh_transfer <from-node> <to-node> [size_mb]
# Returns: 0 if checksums match, 1 otherwise.
mesh_transfer() {
    local from="$1" to="$2" size_mb="${3:-1}"
    local to_ip="${NODE_MESH_IPS[$to]}"
    [ -z "$to_ip" ] && { log_error "mesh_transfer: no mesh IP for node '$to'"; return 1; }

    # Use a high ephemeral port; note: concurrent mesh_transfer calls may collide.
    local port=19999

    # Kill any lingering nc on the port
    run_on_ok "$to" "pkill -f 'nc.*-l.*$port' 2>/dev/null"
    sleep 1

    # Start receiver: listen, write to file
    run_on "$to" "
        rm -f /tmp/mesh-rx.bin /tmp/mesh-rx.sha256
        nc -l $port > /tmp/mesh-rx.bin 2>/dev/null &
        echo \$!
    " > "/tmp/_nc_pid_$$" 2>/dev/null

    # Wait for listener to be ready by probing from the sender side.
    local attempt listener_ready=0
    for attempt in {1..20}; do
        if run_on "$from" "nc -z -w 1 $to_ip $port" 2>/dev/null; then
            listener_ready=1
            break
        fi
        sleep 0.25
    done

    if [ "$listener_ready" -ne 1 ]; then
        log_error "mesh_transfer $from->$to: listener on $to_ip:$port not reachable after $attempt attempts"
        run_on_ok "$to" "pkill -f 'nc.*-l.*$port' 2>/dev/null; rm -f /tmp/mesh-rx.bin"
        rm -f "/tmp/_nc_pid_$$"
        return 1
    fi

    # Generate random data, compute checksum, send
    local src_hash
    src_hash=$(run_on "$from" "
        dd if=/dev/urandom bs=1M count=$size_mb 2>/dev/null | tee /tmp/mesh-tx.bin | sha256sum | awk '{print \$1}'
    " 2>/dev/null)

    # Send the data
    run_on "$from" "nc -w 10 $to_ip $port < /tmp/mesh-tx.bin" 2>/dev/null || true
    sleep 2

    # Get receiver checksum
    local dst_hash
    dst_hash=$(run_on "$to" "sha256sum /tmp/mesh-rx.bin 2>/dev/null | awk '{print \$1}'" 2>/dev/null)

    # Cleanup
    run_on_ok "$to" "pkill -f 'nc.*-l.*$port' 2>/dev/null; rm -f /tmp/mesh-rx.bin"
    run_on_ok "$from" "rm -f /tmp/mesh-tx.bin"
    rm -f "/tmp/_nc_pid_$$"

    if [ -z "$src_hash" ] || [ -z "$dst_hash" ]; then
        log_error "mesh_transfer $from->$to: failed to compute checksums (src='$src_hash' dst='$dst_hash')"
        return 1
    fi

    if [ "$src_hash" = "$dst_hash" ]; then
        log_info "mesh_transfer $from->$to: ${size_mb}MB OK (sha256 match)"
        emit_event "data_transfer" "$from->$to" "size_mb=$size_mb" "result=ok"
        return 0
    else
        log_error "mesh_transfer $from->$to: checksum MISMATCH (src=$src_hash dst=$dst_hash)"
        emit_event "data_transfer" "$from->$to" "size_mb=$size_mb" "result=mismatch"
        return 1
    fi
}

# Run iperf3 throughput test between two nodes over mesh IPs.
# Usage: mesh_iperf <from-node> <to-node> [duration_sec]
# Outputs: throughput in Mbits/sec to stdout, logs result.
mesh_iperf() {
    local from="$1" to="$2" duration="${3:-5}"
    local to_ip="${NODE_MESH_IPS[$to]:-}"
    [ -z "$to_ip" ] && { log_error "mesh_iperf: no mesh IP for node '$to'"; return 1; }

    # Kill any lingering iperf3 on the receiver
    run_on_ok "$to" "pkill iperf3 2>/dev/null"
    sleep 1

    # Start server on receiver (mesh IP)
    run_on "$to" "iperf3 -s -B $to_ip -D -1 2>/dev/null" || true
    sleep 1

    # Run client, parse bandwidth
    local result
    result=$(run_on "$from" "iperf3 -c $to_ip -t $duration -J 2>/dev/null") || {
        log_error "mesh_iperf $from->$to: iperf3 client failed"
        run_on_ok "$to" "pkill iperf3 2>/dev/null"
        return 1
    }

    # Extract sender bits_per_second from JSON
    local bps
    bps=$(echo "$result" | jq -r '.end.sum_sent.bits_per_second // 0' 2>/dev/null) || bps=0
    local mbps
    mbps=$(echo "$bps" | awk '{printf "%.1f", $1/1000000}')

    run_on_ok "$to" "pkill iperf3 2>/dev/null"

    log_info "mesh_iperf $from->$to: ${mbps} Mbits/sec (${duration}s)"
    emit_event "data_iperf" "$from->$to" "mbps=$mbps" "duration=$duration"
    echo "$mbps"
}

# Check MTU path by sending a large ping with DF bit set.
# Usage: mesh_mtu_check <from-node> <to-node> [payload_size]
# Default payload 1372 bytes = 1400 byte packet (28 byte ICMP header).
# WG overhead is ~60 bytes, so 1400 byte mesh packets need MTU >= 1420.
mesh_mtu_check() {
    local from="$1" to="$2" payload="${3:-1372}"
    local to_ip="${NODE_MESH_IPS[$to]:-}"
    [ -z "$to_ip" ] && { log_error "mesh_mtu_check: no mesh IP for node '$to'"; return 1; }

    if run_on "$from" "ping -M do -c 3 -W 3 -s $payload $to_ip" >/dev/null 2>&1; then
        log_info "mesh_mtu $from->$to: ${payload}+28 byte packets OK (DF bit set)"
        emit_event "data_mtu" "$from->$to" "payload=$payload" "result=ok"
        return 0
    else
        log_error "mesh_mtu $from->$to: ${payload}+28 byte packets FAILED (MTU too low or fragmentation needed)"
        emit_event "data_mtu" "$from->$to" "payload=$payload" "result=fail"
        return 1
    fi
}

# Verify data plane for all node pairs: 1MB transfer + MTU check.
# Usage: verify_data_plane
verify_data_plane() {
    local nodes=("${!NODE_MESH_IPS[@]}")
    local n=${#nodes[@]}

    if [ "$n" -eq 0 ]; then
        log_error "verify_data_plane: no mesh IPs available (NODE_MESH_IPS empty)"
        return 1
    fi

    local failures=0

    emit_event "data_plane_gate_start" "verify_data_plane" "nodes=$n"
    log_info "Data plane verification: ${n} nodes, $(( n * (n-1) / 2 )) pairs"

    for (( i=0; i<n; i++ )); do
        for (( j=i+1; j<n; j++ )); do
            mesh_transfer "${nodes[$i]}" "${nodes[$j]}" 1 || ((failures++)) || true
            mesh_mtu_check "${nodes[$i]}" "${nodes[$j]}" || ((failures++)) || true
        done
    done

    if [ "$failures" -gt 0 ]; then
        log_error "Data plane: $failures check(s) failed"
        emit_event "data_plane_gate_end" "verify_data_plane" "result=fail" "failures=$failures"
        return 1
    fi
    log_info "Data plane: all pairs verified (transfer + MTU)"
    emit_event "data_plane_gate_end" "verify_data_plane" "result=ok"
}

# Quick data plane check: 1MB transfer on one random pair.
# Used in _chaos_setup to avoid adding too much time per test.
# Usage: verify_data_plane_quick
verify_data_plane_quick() {
    local nodes=("${!NODE_MESH_IPS[@]}")
    local n=${#nodes[@]}
    [ "$n" -lt 2 ] && { log_error "verify_data_plane_quick: need at least 2 nodes, have $n"; return 1; }

    # Pick a random pair; j wraps around to 0 when i is the last index,
    # intentionally testing bidirectional reachability across the ring.
    local i=$(( RANDOM % n ))
    local j=$(( (i + 1) % n ))
    mesh_transfer "${nodes[$i]}" "${nodes[$j]}" 1
}

# Full data plane benchmark: transfer + MTU + iperf + large transfer.
# Usage: verify_data_plane_full
verify_data_plane_full() {
    local nodes=("${!NODE_MESH_IPS[@]}")
    local n=${#nodes[@]}

    if [ "$n" -eq 0 ]; then
        log_error "verify_data_plane_full: no mesh IPs available (NODE_MESH_IPS empty)"
        return 1
    fi

    local failures=0

    emit_event "data_plane_gate_start" "verify_data_plane_full" "nodes=$n"
    log_info "Full data plane benchmark: ${n} nodes"

    # 1. All-pairs 1MB transfer + MTU
    verify_data_plane || ((failures++)) || true

    # 2. iperf3 throughput on one pair (wraps around for bidirectional coverage)
    local i=$(( RANDOM % n ))
    local j=$(( (i + 1) % n ))
    local throughput
    throughput=$(mesh_iperf "${nodes[$i]}" "${nodes[$j]}" 5) || ((failures++)) || true
    log_info "Throughput sample: ${throughput} Mbits/sec (${nodes[$i]}->${nodes[$j]})"

    # 3. Large 100MB transfer on one pair (wraps around for bidirectional coverage)
    local a=$(( RANDOM % n ))
    local b=$(( (a + 1) % n ))
    log_info "Large transfer: 100MB ${nodes[$a]}->${nodes[$b]}"
    mesh_transfer "${nodes[$a]}" "${nodes[$b]}" 100 || ((failures++)) || true

    if [ "$failures" -gt 0 ]; then
        log_error "Full data plane: $failures check(s) failed"
        emit_event "data_plane_gate_end" "verify_data_plane_full" "result=fail" "failures=$failures"
        return 1
    fi
    log_info "Full data plane: all checks passed"
    emit_event "data_plane_gate_end" "verify_data_plane_full" "result=ok"
}

# ---------------------------------------------------------------------------
# pprof profile collection (Phase 3 observability)
# ---------------------------------------------------------------------------

# Collect a CPU profile from a node (requires --pprof flag on wgmesh binary).
# Usage: collect_pprof_cpu <node> [duration_sec]
collect_pprof_cpu() {
    local node="$1" duration="${2:-30}"
    local outfile="${LOG_DIR}/pprof-cpu-${node}.prof"

    log_info "Collecting ${duration}s CPU profile from $node"
    emit_event "pprof_collect" "$node" "type=cpu" "duration=$duration"
    run_on "$node" "curl -sf 'http://localhost:6060/debug/pprof/profile?seconds=$duration'" \
        > "$outfile" 2>/dev/null || {
        log_warn "pprof CPU collection failed on $node (pprof not enabled?)"
        return 1
    }
    local size
    size=$(wc -c < "$outfile")
    log_info "CPU profile saved: $outfile ($size bytes)"
}

# Collect goroutine dump from a node.
collect_pprof_goroutine() {
    local node="$1"
    local outfile="${LOG_DIR}/pprof-goroutine-${node}.prof"
    run_on "$node" "curl -sf 'http://localhost:6060/debug/pprof/goroutine'" \
        > "$outfile" 2>/dev/null || true
}

# Collect heap profile from a node.
collect_pprof_heap() {
    local node="$1"
    local outfile="${LOG_DIR}/pprof-heap-${node}.prof"
    run_on "$node" "curl -sf 'http://localhost:6060/debug/pprof/heap'" \
        > "$outfile" 2>/dev/null || true
}

# Collect all profile types from all nodes.
# Only runs when WGMESH_PPROF=1 is set.
collect_all_pprof() {
    [ "${WGMESH_PPROF:-0}" != "1" ] && return 0
    log_info "Collecting pprof profiles from all nodes..."
    emit_event "pprof_collect_all_start" "all_nodes"
    for node in "${!NODE_IPS[@]}"; do
        collect_pprof_cpu "$node" 30 &
    done
    wait
    for node in "${!NODE_IPS[@]}"; do
        collect_pprof_goroutine "$node"
        collect_pprof_heap "$node"
    done
    emit_event "pprof_collect_all_end" "all_nodes"
}

# ---------------------------------------------------------------------------
# Log collection and analysis
# ---------------------------------------------------------------------------

# Collect logs from all nodes into LOG_DIR.
collect_logs() {
    mkdir -p "$LOG_DIR"
    for node in "${!NODE_IPS[@]}"; do
        run_on_ok "$node" "journalctl -u wgmesh --no-pager 2>/dev/null" > "$LOG_DIR/${node}.log" 2>/dev/null || true
    done
    log_info "Logs collected to $LOG_DIR"
}

# Scan logs for bad patterns. Returns 1 if any found.
scan_logs_for_errors() {
    local errors=0
    for node in "${!NODE_IPS[@]}"; do
        local log
        log=$(run_on_ok "$node" "journalctl -u wgmesh --no-pager 2>/dev/null") || continue
        if echo "$log" | grep -qiE 'panic|fatal|data race|goroutine \d+ \['; then
            log_error "Bad pattern in $node logs:"
            echo "$log" | grep -iE 'panic|fatal|data race|goroutine \d+ \[' | head -5
            errors=1
        fi
    done
    return $errors
}

# ---------------------------------------------------------------------------
# Test framework
# ---------------------------------------------------------------------------

# Record a test result.
# Usage: record_test <id> <name> <PASS|FAIL|SKIP> <duration_sec> [notes]
record_test() {
    local id="$1" name="$2" result="$3" duration="$4" notes="${5:-}"
    TEST_RESULTS+=("${id}|${name}|${result}|${duration}|${notes}")
    case "$result" in
        PASS) ((TESTS_PASSED++)) || true ;;
        FAIL) ((TESTS_FAILED++)) || true ;;
        SKIP) ((TESTS_SKIPPED++)) || true ;;
    esac
}

# Run a test function, record timing and result.
# Output streams in real-time to stdout AND is captured for the record.
# Usage: run_test <id> <name> <function> [args...]
run_test() {
    local id="$1" name="$2" func="$3"; shift 3
    local total="${TOTAL_TESTS_IN_TIER:-?}"
    local seq=$(( TESTS_PASSED + TESTS_FAILED + TESTS_SKIPPED + 1 ))

    echo ""
    log_test "=== [$seq/$total] $id: $name ==="

    local start rc tmpfile
    start=$(date +%s)
    tmpfile=$(mktemp)
    emit_event "test_start" "$id" "name=$name"

    # Stream output in real-time while also capturing it
    set +e
    "$func" "$@" 2>&1 | tee "$tmpfile"
    rc=${PIPESTATUS[0]}
    set -e

    local output
    output=$(cat "$tmpfile")
    rm -f "$tmpfile"

    local end_epoch
    end_epoch=$(date +%s)
    local duration=$(( end_epoch - start ))
    local result="FAIL"

    if [ $rc -eq 0 ]; then
        result="PASS"
        record_test "$id" "$name" "PASS" "$duration" ""
        log_test "${GREEN}PASS${NC} $id: $name (${duration}s)"
    elif [ $rc -eq 2 ]; then
        result="SKIP"
        record_test "$id" "$name" "SKIP" "$duration" "$output"
        log_test "${YELLOW}SKIP${NC} $id: $name (${duration}s) — $output"
    else
        record_test "$id" "$name" "FAIL" "$duration" "$output"
        log_test "${RED}FAIL${NC} $id: $name (${duration}s)"
    fi

    # Record timing for Gantt chart
    TEST_TIMING_EVENTS+=("${CURRENT_TIER:-0}|${id}|${name}|${start}|${end_epoch}|${result}")
    emit_event "test_end" "$id" "name=$name" "result=$result" "duration=$duration"

    # Running tally after each test
    log_test "  Progress: ${GREEN}${TESTS_PASSED} passed${NC}, ${RED}${TESTS_FAILED} failed${NC}, ${YELLOW}${TESTS_SKIPPED} skipped${NC} of $total"
}

# Print test summary table.
print_summary() {
    echo ""
    log_bold "============================================"
    log_bold "         Test Results Summary"
    log_bold "============================================"
    printf "%-8s %-40s %-6s %8s  %s\n" "ID" "Name" "Result" "Duration" "Notes"
    echo "------------------------------------------------------------------------------------------------------------"

    for entry in "${TEST_RESULTS[@]}"; do
        IFS='|' read -r id name result duration notes <<< "$entry"
        local color="$NC"
        case "$result" in
            PASS) color="$GREEN" ;;
            FAIL) color="$RED" ;;
            SKIP) color="$YELLOW" ;;
        esac
        printf "%-8s %-40s ${color}%-6s${NC} %7ss  %s\n" "$id" "$name" "$result" "$duration" "${notes:0:40}"
    done

    echo "------------------------------------------------------------------------------------------------------------"
    echo -e "Total: ${GREEN}${TESTS_PASSED} passed${NC}, ${RED}${TESTS_FAILED} failed${NC}, ${YELLOW}${TESTS_SKIPPED} skipped${NC}"
    echo ""
}

# Output results as GitHub Actions job summary (markdown).
# Writes to both GITHUB_STEP_SUMMARY and a local file for artifact upload.
print_github_summary() {
    local out="${GITHUB_STEP_SUMMARY:-/dev/null}"
    local local_summary="${LOG_DIR}/tier-summary.md"
    mkdir -p "$(dirname "$local_summary")"

    _emit_summary() {
        echo "| ID | Name | Result | Duration | Notes |"
        echo "|---|---|---|---|---|"
        for entry in "${TEST_RESULTS[@]}"; do
            IFS='|' read -r id name result duration notes <<< "$entry"
            local icon="?"
            case "$result" in
                PASS) icon="pass" ;;
                FAIL) icon="FAIL" ;;
                SKIP) icon="skip" ;;
            esac
            echo "| $id | $name | $icon | ${duration}s | ${notes:0:60} |"
        done
        echo ""
        echo "**Total: ${TESTS_PASSED} passed, ${TESTS_FAILED} failed, ${TESTS_SKIPPED} skipped**"
    }

    # Generate Mermaid Gantt chart from timing data.
    _emit_gantt() {
        [ "${#TEST_TIMING_EVENTS[@]}" -eq 0 ] && return

        # Find the earliest start time as baseline
        local first_start=""
        for event in "${TEST_TIMING_EVENTS[@]}"; do
            IFS='|' read -r _tier _id _name start _end _result <<< "$event"
            if [ -z "$first_start" ] || [ "$start" -lt "$first_start" ]; then
                first_start="$start"
            fi
        done
        [ -z "$first_start" ] && return

        echo ""
        echo "### Test Timeline"
        echo ""
        echo '```mermaid'
        echo "gantt"
        echo "    title Test Execution Timeline"
        echo "    dateFormat X"
        echo "    axisFormat %M:%S"

        local prev_tier=""
        for event in "${TEST_TIMING_EVENTS[@]}"; do
            IFS='|' read -r tier id name start end result <<< "$event"
            local offset=$(( start - first_start ))
            local duration=$(( end - start ))
            [ "$duration" -lt 1 ] && duration=1

            # New section per tier
            if [ "$tier" != "$prev_tier" ]; then
                echo "    section Tier $tier"
                prev_tier="$tier"
            fi

            # Mermaid task status
            local status=""
            [ "$result" = "FAIL" ] && status="crit, "
            [ "$result" = "SKIP" ] && status="done, "

            echo "    ${id} ${name} :${status}${offset}, ${duration}"
        done
        echo '```'
    }

    # Write to GitHub step summary
    {
        echo "## wgmesh Integration Test Results"
        echo ""
        _emit_summary
        _emit_gantt
    } >> "$out"

    # Write to local file (picked up by tier summary step)
    {
        _emit_summary
        _emit_gantt
    } > "$local_summary"
}

# Exit with appropriate code.
finish_tests() {
    print_summary
    print_github_summary
    collect_logs

    if [ "$TESTS_FAILED" -gt 0 ]; then
        log_error "$TESTS_FAILED test(s) failed"
        exit 1
    fi
    log_info "All tests passed"
    exit 0
}
