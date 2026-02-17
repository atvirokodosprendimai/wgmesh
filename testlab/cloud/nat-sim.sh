#!/usr/bin/env bash
# nat-sim.sh — NAT simulation helpers using network namespaces
#
# Simulates cone and symmetric NAT on Hetzner VMs by placing wgmesh inside
# a network namespace behind iptables MASQUERADE (cone) or SNAT --random-fully
# (symmetric). The introducer stays on the bare host — it acts as the relay
# and rendezvous introducer.
#
# Prerequisites (installed by provision.sh):
#   - iproute2, iptables, wireguard-tools
#
# Usage:
#   source lib.sh
#   source nat-sim.sh
#
#   nat_setup  <node> cone       # Put node behind cone NAT
#   nat_setup  <node> symmetric  # Put node behind symmetric NAT
#   nat_start  <node>            # Start wgmesh inside the NAT namespace
#   nat_stop   <node>            # Stop wgmesh inside the namespace
#   nat_teardown <node>          # Remove namespace and NAT rules
#   nat_teardown_all             # Teardown all NAT namespaces
#
# How it works:
#   1. Creates netns "nat-<node>" on the remote VM
#   2. Creates veth pair: veth-<node> (host side) <-> veth0 (namespace side)
#   3. Assigns private IPs: host=10.99.<idx>.1/24, ns=10.99.<idx>.2/24
#   4. Enables IP forwarding on host
#   5. Applies MASQUERADE (cone) or SNAT --random-fully (symmetric) on host
#   6. Sets up DNS (8.8.8.8) inside namespace
#   7. Runs wgmesh inside the namespace via ip netns exec
#
# The key difference between cone and symmetric:
#   - Cone (MASQUERADE): Endpoint-independent mapping. Same source port
#     regardless of destination. STUN sees same IP:port → NATType="cone".
#   - Symmetric (SNAT --random-fully): Per-connection port allocation.
#     Different source port for each destination. STUN sees different
#     IP:port → NATType="symmetric".

set -euo pipefail

# Track which nodes have NAT namespaces
declare -A NAT_NODES=()  # node -> "cone"|"symmetric"
declare -A NAT_IDX=()    # node -> subnet index (1,2,3...)
_NAT_NEXT_IDX=1

# ---------------------------------------------------------------------------
# Setup / Teardown
# ---------------------------------------------------------------------------

# Create a NAT namespace on a remote node.
# Usage: nat_setup <node> <cone|symmetric>
nat_setup() {
    local node="$1" nat_type="$2"
    local idx=$_NAT_NEXT_IDX
    _NAT_NEXT_IDX=$(( _NAT_NEXT_IDX + 1 ))

    NAT_NODES["$node"]="$nat_type"
    NAT_IDX["$node"]="$idx"

    local ns="nat-${node}"
    local veth_host="veth-${node}"
    local veth_ns="veth0"
    local host_ip="10.99.${idx}.1"
    local ns_ip="10.99.${idx}.2"
    local subnet="10.99.${idx}.0/24"
    local iface
    iface=$(_get_iface "$node")

    log_info "nat-sim: setting up $nat_type NAT on $node (ns=$ns, subnet=$subnet)"

    # Stop wgmesh if running — we'll restart it inside the namespace
    run_on_ok "$node" "systemctl stop wgmesh 2>/dev/null; ip link del $WG_INTERFACE 2>/dev/null"
    # Wait for port 51820 to be released
    sleep 1

    run_on "$node" "
        set -e

        # Clean up any previous state
        ip netns del '$ns' 2>/dev/null || true
        ip link del '$veth_host' 2>/dev/null || true

        # Create namespace
        ip netns add '$ns'

        # Create veth pair
        ip link add '$veth_host' type veth peer name '$veth_ns'

        # Move one end into the namespace
        ip link set '$veth_ns' netns '$ns'

        # Configure host side
        ip addr add '${host_ip}/24' dev '$veth_host'
        ip link set '$veth_host' up

        # Configure namespace side
        ip netns exec '$ns' ip addr add '${ns_ip}/24' dev '$veth_ns'
        ip netns exec '$ns' ip link set '$veth_ns' up
        ip netns exec '$ns' ip link set lo up

        # Disable IPv6 inside namespace — force IPv4-only so wgmesh
        # doesn't try IPv6 endpoints that can't route through our NAT
        ip netns exec '$ns' sysctl -w net.ipv6.conf.all.disable_ipv6=1 >/dev/null 2>&1 || true
        ip netns exec '$ns' sysctl -w net.ipv6.conf.default.disable_ipv6=1 >/dev/null 2>&1 || true

        # Default route inside namespace → host side of veth
        ip netns exec '$ns' ip route add default via '$host_ip'

        # DNS inside namespace
        mkdir -p /etc/netns/'$ns'
        echo 'nameserver 8.8.8.8' > /etc/netns/'$ns'/resolv.conf

        # Enable forwarding on host
        sysctl -w net.ipv4.ip_forward=1 >/dev/null

        # NAT rules on host
        # Allow forwarding for this subnet (and established/related return traffic)
        iptables -t filter -A FORWARD -s '$subnet' -j ACCEPT
        iptables -t filter -A FORWARD -d '$subnet' -m state --state ESTABLISHED,RELATED -j ACCEPT
        iptables -t filter -A FORWARD -d '$subnet' -j ACCEPT
    "

    local host_pub_ip="${NODE_IPS[$node]}"

    # Apply the appropriate NAT type
    case "$nat_type" in
        cone)
            # MASQUERADE: endpoint-independent mapping (port-preserving)
            # The kernel reuses the same external port for a given internal
            # source IP:port regardless of destination.
            run_on "$node" "
                iptables -t nat -A POSTROUTING -s '$subnet' -o '$iface' -j MASQUERADE
            "
            log_info "nat-sim: $node — cone NAT (MASQUERADE) on $iface for $subnet"
            ;;
        symmetric)
            # Symmetric NAT: per-connection random port allocation.
            # Use MASQUERADE for DNS (port 53) so DNS resolution works reliably,
            # then SNAT --random-fully for everything else so STUN sees
            # different mappings per destination → NATType=symmetric.
            run_on "$node" "
                # DNS must work — use simple MASQUERADE for DNS traffic
                iptables -t nat -A POSTROUTING -s '$subnet' -o '$iface' -p udp --dport 53 -j MASQUERADE
                iptables -t nat -A POSTROUTING -s '$subnet' -o '$iface' -p tcp --dport 53 -j MASQUERADE
                # Everything else: random port per connection
                iptables -t nat -A POSTROUTING -s '$subnet' -o '$iface' \
                    -j SNAT --to-source '${host_pub_ip}:32768-60999' --random-fully
            "
            log_info "nat-sim: $node — symmetric NAT (SNAT --random-fully) on $iface for $subnet"
            ;;
        *)
            log_error "nat-sim: unknown NAT type: $nat_type (expected cone|symmetric)"
            return 1
            ;;
    esac

    # DNAT: Forward WG port (51820/udp) and gossip port range from the host's
    # public IP into the namespace. This is needed because peers will send
    # WG handshakes and gossip to host_pub_ip:51820 — without DNAT, the host
    # has no listener on that port (wgmesh is inside the namespace).
    # This simulates a NAT/router with port forwarding (UPnP/PMP).
    #
    # Port ranges:
    #   51820      = WireGuard data plane
    #   51821-52821 = gossip/exchange port (derived: 51821 + HKDF % 1000)
    #   exchange+1 = DHT port
    run_on "$node" "
        # Forward WG port into namespace
        iptables -t nat -A PREROUTING -d '${host_pub_ip}' -p udp --dport 51820 \
            -j DNAT --to-destination '${ns_ip}:51820'
        # Forward entire gossip+DHT port range into namespace
        iptables -t nat -A PREROUTING -d '${host_pub_ip}' -p udp --dport 51821:52822 \
            -j DNAT --to-destination '${ns_ip}'
    "
    log_info "nat-sim: $node — DNAT port forwarding (51820-52822/udp) to $ns_ip"

    # Verify connectivity from namespace to internet
    if run_on "$node" "ip netns exec '$ns' ping -c 1 -W 5 8.8.8.8" >/dev/null 2>&1; then
        log_info "nat-sim: $node namespace has internet connectivity"
    else
        log_error "nat-sim: $node namespace CANNOT reach internet — NAT setup failed"
        return 1
    fi

    # Verify DNS works inside namespace
    if run_on "$node" "ip netns exec '$ns' nslookup google.com 8.8.8.8 2>/dev/null | grep -q 'Address'" 2>/dev/null; then
        log_info "nat-sim: $node namespace has DNS resolution"
    else
        log_warn "nat-sim: $node namespace DNS may not work (nslookup failed)"
    fi
}

# Start wgmesh inside the NAT namespace.
# Creates a new systemd unit that runs inside the namespace.
# Usage: nat_start <node>
nat_start() {
    local node="$1"
    local ns="nat-${node}"
    local nat_type="${NAT_NODES[$node]:-}"

    if [ -z "$nat_type" ]; then
        log_error "nat-sim: $node has no NAT namespace set up"
        return 1
    fi

    local role="${NODE_ROLES[$node]}"
    local extra_flags=""
    [ "$role" = "introducer" ] && extra_flags="--introducer"

    # Create a systemd unit that runs wgmesh inside the namespace
    run_on "$node" "cat > /etc/systemd/system/wgmesh-nat.service << 'UNIT'
[Unit]
Description=wgmesh NAT integration test
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/bin/ip netns exec ${ns} /usr/local/bin/wgmesh join \
    --secret \"${MESH_SECRET}\" \
    --interface ${WG_INTERFACE} \
    --gossip \
    ${extra_flags}
Restart=no
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
UNIT
    systemctl daemon-reload
    systemctl start wgmesh-nat
    "

    log_info "nat-sim: started wgmesh in $nat_type NAT namespace on $node"
}

# Stop wgmesh inside the NAT namespace.
nat_stop() {
    local node="$1"
    run_on_ok "$node" "
        systemctl stop wgmesh-nat 2>/dev/null
        systemctl stop wgmesh 2>/dev/null
    "
    # Clean up WG interface inside namespace
    local ns="nat-${node}"
    run_on_ok "$node" "ip netns exec '$ns' ip link del $WG_INTERFACE 2>/dev/null"
    log_info "nat-sim: stopped wgmesh on $node"
}

# Remove the NAT namespace and all associated rules.
nat_teardown() {
    local node="$1"
    local ns="nat-${node}"
    local idx="${NAT_IDX[$node]:-}"
    local subnet="10.99.${idx}.0/24"
    local veth_host="veth-${node}"

    run_on_ok "$node" "
        # Stop wgmesh services
        systemctl stop wgmesh-nat 2>/dev/null
        systemctl stop wgmesh 2>/dev/null

        # Remove NAT/forward rules for this subnet
        iptables -t nat -F 2>/dev/null
        iptables -t filter -D FORWARD -s '$subnet' -j ACCEPT 2>/dev/null
        iptables -t filter -D FORWARD -d '$subnet' -j ACCEPT 2>/dev/null

        # Remove namespace (also removes veth)
        ip netns del '$ns' 2>/dev/null
        ip link del '$veth_host' 2>/dev/null

        # Remove DNS config
        rm -rf /etc/netns/'$ns'

        # Clean up WG interface on host
        ip link del $WG_INTERFACE 2>/dev/null

        # Remove the NAT systemd unit
        rm -f /etc/systemd/system/wgmesh-nat.service
        systemctl daemon-reload 2>/dev/null
    "

    unset 'NAT_NODES[$node]' 2>/dev/null || true
    unset 'NAT_IDX[$node]' 2>/dev/null || true

    log_info "nat-sim: teardown complete on $node"
}

# Teardown all NAT namespaces.
nat_teardown_all() {
    log_info "nat-sim: tearing down all NAT namespaces..."
    for node in "${!NAT_NODES[@]}"; do
        nat_teardown "$node" 2>/dev/null || true
    done
    # Also flush iptables NAT table on all nodes (safety net)
    for node in "${!NODE_IPS[@]}"; do
        run_on_ok "$node" "iptables -t nat -F 2>/dev/null"
    done
    log_info "nat-sim: all NAT teardown complete"
}

# ---------------------------------------------------------------------------
# Query helpers
# ---------------------------------------------------------------------------

# Get mesh IP for a NATed node (wg0 is inside namespace).
# Usage: nat_mesh_ip <node>
nat_mesh_ip() {
    local node="$1"
    local ns="nat-${node}"
    run_on "$node" "ip netns exec '$ns' ip -4 addr show $WG_INTERFACE 2>/dev/null \
        | grep -oP 'inet \K10\.[0-9]+\.[0-9]+\.[0-9]+'" 2>/dev/null
}

# Ping from a NATed node to a mesh IP.
# Usage: nat_ping <from-node> <mesh-ip> [count]
nat_ping() {
    local from="$1" mesh_ip="$2" count="${3:-3}"
    local ns="nat-${from}"
    if [ -n "${NAT_NODES[$from]:-}" ]; then
        # Source is NATed — ping from inside namespace
        run_on "$from" "ip netns exec '$ns' ping -c $count -W 5 '$mesh_ip'" >/dev/null 2>&1
    else
        # Source is not NATed — normal ping
        run_on "$from" "ping -c $count -W 5 '$mesh_ip'" >/dev/null 2>&1
    fi
}

# Get the WG endpoint for a specific peer on a node.
# Returns the endpoint IP:port as seen by WG (shows if traffic goes via relay).
# Usage: nat_wg_endpoint <node> <peer-mesh-ip>
nat_wg_endpoint() {
    local node="$1" peer_mesh_ip="$2"
    local ns="nat-${node}"
    local cmd="wg show $WG_INTERFACE dump 2>/dev/null | while IFS=\$'\t' read -r pubkey psk endpoint aips handshake rx tx ka; do
        if echo \"\$aips\" | grep -q '$peer_mesh_ip'; then
            echo \"\$endpoint\"
            exit 0
        fi
    done"

    if [ -n "${NAT_NODES[$node]:-}" ]; then
        run_on "$node" "ip netns exec '$ns' $cmd" 2>/dev/null
    else
        run_on "$node" "$cmd" 2>/dev/null
    fi
}

# Check WG handshake age for a NATed node.
nat_handshake_age() {
    local node="$1" peer_mesh_ip="$2"
    local ns="nat-${node}"
    local cmd="
        now=\$(date +%s)
        wg show $WG_INTERFACE dump 2>/dev/null | while IFS=\$'\t' read -r pubkey psk endpoint aips handshake rx tx ka; do
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
    "

    if [ -n "${NAT_NODES[$node]:-}" ]; then
        run_on "$node" "ip netns exec '$ns' $cmd" 2>/dev/null
    else
        run_on "$node" "$cmd" 2>/dev/null
    fi
}

# Get the NAT type as detected by wgmesh (from its logs).
# Usage: nat_detected_type <node>
nat_detected_type() {
    local node="$1"
    run_on "$node" "journalctl -u wgmesh-nat --no-pager 2>/dev/null \
        | grep -oP 'NAT type: \K(cone|symmetric|unknown)' | tail -1" 2>/dev/null
}

# Get wgmesh logs from the NAT service.
nat_logs() {
    local node="$1"
    run_on "$node" "journalctl -u wgmesh-nat --no-pager 2>/dev/null" 2>/dev/null
}

# Populate mesh IPs for NATed nodes (wg0 is inside namespace).
nat_populate_mesh_ips() {
    log_info "nat-sim: querying mesh IPs from NATed nodes..."
    local max_retries=10
    local retry=0

    while [ $retry -lt $max_retries ]; do
        local missing=0
        for node in "${!NODE_IPS[@]}"; do
            [ -n "${NODE_MESH_IPS[$node]:-}" ] && continue

            local mesh_ip=""
            if [ -n "${NAT_NODES[$node]:-}" ]; then
                mesh_ip=$(nat_mesh_ip "$node") || mesh_ip=""
            else
                mesh_ip=$(run_on "$node" "ip -4 addr show $WG_INTERFACE 2>/dev/null \
                    | grep -oP 'inet \K10\.[0-9]+\.[0-9]+\.[0-9]+'" 2>/dev/null) || mesh_ip=""
            fi

            if [ -n "$mesh_ip" ]; then
                NODE_MESH_IPS["$node"]="$mesh_ip"
            else
                missing=$((missing + 1))
            fi
        done

        if [ $missing -eq 0 ]; then
            break
        fi

        retry=$((retry + 1))
        if [ $retry -lt $max_retries ]; then
            log_info "nat-sim: waiting for $missing node(s) to get mesh IPs (attempt $retry/$max_retries)..."
            sleep 5
        fi
    done

    for name in $(echo "${!NODE_MESH_IPS[@]}" | tr ' ' '\n' | sort); do
        log_info "  $name: mesh=${NODE_MESH_IPS[$name]} nat=${NAT_NODES[$name]:-none}"
    done
}
