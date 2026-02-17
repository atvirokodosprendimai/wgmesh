#!/bin/bash
# chaos.sh — Network impairment harness for wgmesh chaos testing
#
# Applies and removes network impairments on remote nodes using tc netem
# and iptables. Designed to be called from test-cloud.sh or used standalone.
#
# Usage:
#   source lib.sh   # for run_on / NODE_IPS
#   source chaos.sh
#
#   chaos_apply  <node> loss 30
#   chaos_apply  <node> delay 200 50
#   chaos_apply  <node> throttle 100
#   chaos_apply  <node> reorder 25 50
#   chaos_apply  <node> duplicate 30
#   chaos_clear  <node>
#
#   chaos_block       <node> <target-node>
#   chaos_unblock     <node> <target-node>
#   chaos_block_port  <node> <port>
#   chaos_unblock_port <node> <port>
#   chaos_block_dns   <node>
#   chaos_unblock_dns <node>
#   chaos_isolate     <node>
#   chaos_unisolate   <node>
#   chaos_skew_clock  <node> <minutes>
#   chaos_fix_clock   <node>
#   chaos_clear_all

set -euo pipefail

# ---------------------------------------------------------------------------
# tc netem impairments (applied to eth0 / default interface)
# ---------------------------------------------------------------------------

# Detect the primary network interface on a node.
_get_iface() {
    local node="$1"
    run_on "$node" "ip route show default | awk '/default/ {print \$5}' | head -1"
}

# Apply a tc netem impairment.
# Usage: chaos_apply <node> <type> [params...]
#
# Types:
#   loss <percent>                      — random packet loss
#   delay <ms> [jitter_ms]             — latency (optionally with jitter)
#   throttle <kbit>                    — bandwidth limit
#   reorder <percent> <correlation>    — packet reordering
#   duplicate <percent>                — packet duplication
#   corrupt <percent>                  — bit-level corruption
chaos_apply() {
    local node="$1" type="$2"; shift 2
    local iface
    iface=$(_get_iface "$node")

    # Clear any existing qdisc first
    run_on_ok "$node" "tc qdisc del dev $iface root 2>/dev/null"

    case "$type" in
        loss)
            local pct="${1:?loss percent required}"
            run_on "$node" "tc qdisc add dev $iface root netem loss ${pct}%"
            log_info "chaos: $node — ${pct}% packet loss on $iface"
            ;;
        delay)
            local ms="${1:?delay ms required}"
            local jitter="${2:-0}"
            if [ "$jitter" -gt 0 ] 2>/dev/null; then
                run_on "$node" "tc qdisc add dev $iface root netem delay ${ms}ms ${jitter}ms distribution normal"
            else
                run_on "$node" "tc qdisc add dev $iface root netem delay ${ms}ms"
            fi
            log_info "chaos: $node — ${ms}ms delay (jitter=${jitter}ms) on $iface"
            ;;
        throttle)
            local kbit="${1:?kbit required}"
            run_on "$node" "tc qdisc add dev $iface root tbf rate ${kbit}kbit burst 32kbit latency 400ms"
            log_info "chaos: $node — throttled to ${kbit}kbit on $iface"
            ;;
        reorder)
            local pct="${1:?reorder percent required}"
            local corr="${2:-50}"
            run_on "$node" "tc qdisc add dev $iface root netem delay 10ms reorder ${pct}% ${corr}%"
            log_info "chaos: $node — ${pct}% reorder (corr=${corr}%) on $iface"
            ;;
        duplicate)
            local pct="${1:?duplicate percent required}"
            run_on "$node" "tc qdisc add dev $iface root netem duplicate ${pct}%"
            log_info "chaos: $node — ${pct}% duplication on $iface"
            ;;
        corrupt)
            local pct="${1:?corrupt percent required}"
            run_on "$node" "tc qdisc add dev $iface root netem corrupt ${pct}%"
            log_info "chaos: $node — ${pct}% corruption on $iface"
            ;;
        *)
            log_error "Unknown chaos type: $type"
            return 1
            ;;
    esac
}

# Remove all tc netem impairments from a node.
chaos_clear() {
    local node="$1"
    local iface
    iface=$(_get_iface "$node")
    run_on_ok "$node" "tc qdisc del dev $iface root 2>/dev/null"
    log_info "chaos: $node — cleared tc impairments"
}

# ---------------------------------------------------------------------------
# iptables-based impairments (partitions, port blocks, DNS)
# ---------------------------------------------------------------------------

# Block all traffic from node to a specific target node.
chaos_block() {
    local node="$1" target="$2"
    local target_ip="${NODE_IPS[$target]}"
    run_on "$node" "iptables -A OUTPUT -d $target_ip -j DROP && iptables -A INPUT -s $target_ip -j DROP"
    log_info "chaos: $node — blocked traffic to/from $target ($target_ip)"
}

# Unblock traffic from node to a specific target node.
chaos_unblock() {
    local node="$1" target="$2"
    local target_ip="${NODE_IPS[$target]}"
    run_on_ok "$node" "iptables -D OUTPUT -d $target_ip -j DROP 2>/dev/null; iptables -D INPUT -s $target_ip -j DROP 2>/dev/null"
    log_info "chaos: $node — unblocked traffic to/from $target ($target_ip)"
}

# Block one direction only: node cannot SEND to target, but can RECEIVE from target.
chaos_block_outbound() {
    local node="$1" target="$2"
    local target_ip="${NODE_IPS[$target]}"
    run_on "$node" "iptables -A OUTPUT -d $target_ip -j DROP"
    log_info "chaos: $node — blocked outbound to $target ($target_ip)"
}

chaos_unblock_outbound() {
    local node="$1" target="$2"
    local target_ip="${NODE_IPS[$target]}"
    run_on_ok "$node" "iptables -D OUTPUT -d $target_ip -j DROP 2>/dev/null"
    log_info "chaos: $node — unblocked outbound to $target ($target_ip)"
}

# Block a specific UDP/TCP port on a node (both directions).
chaos_block_port() {
    local node="$1" port="$2"
    run_on "$node" "
        iptables -A INPUT -p udp --dport $port -j DROP
        iptables -A OUTPUT -p udp --sport $port -j DROP
        iptables -A INPUT -p tcp --dport $port -j DROP
        iptables -A OUTPUT -p tcp --sport $port -j DROP
    "
    log_info "chaos: $node — blocked port $port"
}

chaos_unblock_port() {
    local node="$1" port="$2"
    run_on_ok "$node" "
        iptables -D INPUT -p udp --dport $port -j DROP 2>/dev/null
        iptables -D OUTPUT -p udp --sport $port -j DROP 2>/dev/null
        iptables -D INPUT -p tcp --dport $port -j DROP 2>/dev/null
        iptables -D OUTPUT -p tcp --sport $port -j DROP 2>/dev/null
    "
    log_info "chaos: $node — unblocked port $port"
}

# Block DNS (UDP port 53) on a node.
chaos_block_dns() {
    local node="$1"
    run_on "$node" "iptables -A OUTPUT -p udp --dport 53 -j DROP && iptables -A OUTPUT -p tcp --dport 53 -j DROP"
    log_info "chaos: $node — blocked DNS"
}

chaos_unblock_dns() {
    local node="$1"
    run_on_ok "$node" "
        iptables -D OUTPUT -p udp --dport 53 -j DROP 2>/dev/null
        iptables -D OUTPUT -p tcp --dport 53 -j DROP 2>/dev/null
    "
    log_info "chaos: $node — unblocked DNS"
}

# Fully isolate a node from all other mesh nodes.
chaos_isolate() {
    local node="$1"
    for other in "${!NODE_IPS[@]}"; do
        [ "$other" = "$node" ] && continue
        chaos_block "$node" "$other"
    done
    log_info "chaos: $node — fully isolated from mesh"
}

# Remove full isolation.
chaos_unisolate() {
    local node="$1"
    for other in "${!NODE_IPS[@]}"; do
        [ "$other" = "$node" ] && continue
        chaos_unblock "$node" "$other"
    done
    log_info "chaos: $node — isolation removed"
}

# Create a network partition: two groups cannot communicate.
# Usage: chaos_partition <group1_csv> <group2_csv>
#   e.g. chaos_partition "intro,node-a" "node-b,node-c,node-d"
chaos_partition() {
    local group1_str="$1" group2_str="$2"
    IFS=',' read -ra group1 <<< "$group1_str"
    IFS=',' read -ra group2 <<< "$group2_str"

    for n1 in "${group1[@]}"; do
        for n2 in "${group2[@]}"; do
            chaos_block "$n1" "$n2"
            chaos_block "$n2" "$n1"
        done
    done
    log_info "chaos: partition created — [${group1_str}] <-X-> [${group2_str}]"
}

# Heal a partition.
chaos_heal_partition() {
    local group1_str="$1" group2_str="$2"
    IFS=',' read -ra group1 <<< "$group1_str"
    IFS=',' read -ra group2 <<< "$group2_str"

    for n1 in "${group1[@]}"; do
        for n2 in "${group2[@]}"; do
            chaos_unblock "$n1" "$n2"
            chaos_unblock "$n2" "$n1"
        done
    done
    log_info "chaos: partition healed — [${group1_str}] <---> [${group2_str}]"
}

# ---------------------------------------------------------------------------
# Clock skew
# ---------------------------------------------------------------------------

# Skew the system clock forward by N minutes.
chaos_skew_clock() {
    local node="$1" minutes="$2"
    run_on "$node" "timedatectl set-ntp false && date -s '+${minutes} minutes'"
    log_info "chaos: $node — clock skewed +${minutes}min"
}

# Restore NTP on a node.
chaos_fix_clock() {
    local node="$1"
    run_on_ok "$node" "timedatectl set-ntp true"
    # Force immediate sync
    run_on_ok "$node" "systemctl restart systemd-timesyncd 2>/dev/null"
    log_info "chaos: $node — clock restored (NTP enabled)"
}

# ---------------------------------------------------------------------------
# Random chaos
# ---------------------------------------------------------------------------

# Apply a random impairment to a random node for a given duration.
# Usage: chaos_random_hit <duration_sec>
chaos_random_hit() {
    local duration="${1:-15}"
    local nodes=("${!NODE_IPS[@]}")
    local node="${nodes[$RANDOM % ${#nodes[@]}]}"

    local impairments=("loss" "delay" "duplicate" "reorder")
    local imp="${impairments[$RANDOM % ${#impairments[@]}]}"

    case "$imp" in
        loss)      chaos_apply "$node" loss $(( RANDOM % 50 + 5 )) ;;
        delay)     chaos_apply "$node" delay $(( RANDOM % 300 + 10 )) $(( RANDOM % 100 )) ;;
        duplicate) chaos_apply "$node" duplicate $(( RANDOM % 30 + 5 )) ;;
        reorder)   chaos_apply "$node" reorder $(( RANDOM % 25 + 5 )) 50 ;;
    esac

    sleep "$duration"
    chaos_clear "$node"
}

# ---------------------------------------------------------------------------
# Cleanup everything
# ---------------------------------------------------------------------------

# Clear ALL chaos impairments on ALL nodes.
chaos_clear_all() {
    log_info "chaos: clearing all impairments on all nodes..."
    for node in "${!NODE_IPS[@]}"; do
        # Clear tc
        chaos_clear "$node" 2>/dev/null || true
        # Flush iptables additions (restore to ACCEPT default)
        run_on_ok "$node" "iptables -F INPUT 2>/dev/null; iptables -F OUTPUT 2>/dev/null; iptables -P INPUT ACCEPT; iptables -P OUTPUT ACCEPT"
        # Fix clock
        chaos_fix_clock "$node" 2>/dev/null || true
    done
    log_info "chaos: all impairments cleared"
}
