#!/bin/bash
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

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------

log_info()  { echo -e "${GREEN}[INFO]${NC}  $(date +%H:%M:%S) $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $(date +%H:%M:%S) $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $(date +%H:%M:%S) $*"; }
log_test()  { echo -e "${BLUE}[TEST]${NC}  $(date +%H:%M:%S) $*"; }
log_bold()  { echo -e "${BOLD}$*${NC}"; }

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

    wait_for "full mesh ($expected_pairs pairs)" "$timeout" _check_all_pairs "${nodes[@]}"
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
# Usage: run_test <id> <name> <function> [args...]
run_test() {
    local id="$1" name="$2" func="$3"; shift 3
    log_test "--- $id: $name ---"
    local start rc notes=""
    start=$(date +%s)

    set +e
    output=$("$func" "$@" 2>&1)
    rc=$?
    set -e

    local duration=$(( $(date +%s) - start ))

    if [ $rc -eq 0 ]; then
        record_test "$id" "$name" "PASS" "$duration" "$output"
        log_test "${GREEN}PASS${NC} $id: $name (${duration}s)"
    elif [ $rc -eq 2 ]; then
        record_test "$id" "$name" "SKIP" "$duration" "$output"
        log_test "${YELLOW}SKIP${NC} $id: $name (${duration}s) — $output"
    else
        record_test "$id" "$name" "FAIL" "$duration" "$output"
        log_test "${RED}FAIL${NC} $id: $name (${duration}s)"
        echo "$output" | tail -10
    fi
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
print_github_summary() {
    local out="${GITHUB_STEP_SUMMARY:-/dev/null}"
    {
        echo "## wgmesh Integration Test Results"
        echo ""
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
    } >> "$out"
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
