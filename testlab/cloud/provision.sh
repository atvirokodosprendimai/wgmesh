#!/usr/bin/env bash
# provision.sh — Create and destroy Hetzner Cloud VMs for wgmesh testing
#
# Usage:
#   source lib.sh
#   source provision.sh
#
#   provision_ssh_key          # Create or reuse SSH key
#   provision_vms 5            # Create 5 VMs (1 introducer + 4 nodes)
#   populate_node_info         # Fill NODE_IPS, NODE_MESH_IPS, etc.
#   teardown_vms               # Delete all VMs and SSH key
#   teardown_orphans           # Delete any stale wgmesh-ci-* VMs older than 30min

set -euo pipefail

# ---------------------------------------------------------------------------
# SSH Key management
# ---------------------------------------------------------------------------

# Create an ephemeral SSH key pair for this CI run.
provision_ssh_key() {
    if [ -f "$SSH_KEY_FILE" ]; then
        log_info "SSH key already exists: $SSH_KEY_FILE"
    else
        ssh-keygen -t ed25519 -f "$SSH_KEY_FILE" -N "" -q
        log_info "Generated SSH key: $SSH_KEY_FILE"
    fi

    # Upload to Hetzner if not already present
    local key_name="${VM_PREFIX}-key"
    if hcloud ssh-key describe "$key_name" >/dev/null 2>&1; then
        # Key exists — delete and re-create to ensure it matches local key
        hcloud ssh-key delete "$key_name" 2>/dev/null || true
    fi
    hcloud ssh-key create --name "$key_name" --public-key-from-file "${SSH_KEY_FILE}.pub"
    log_info "Uploaded SSH key to Hetzner: $key_name"
}

# ---------------------------------------------------------------------------
# VM provisioning
# ---------------------------------------------------------------------------

# Provision N VMs across multiple locations.
# VM 0 = introducer (hel1), VMs 1..N-1 = nodes spread across locations.
# Usage: provision_vms <count>
provision_vms() {
    local count="${1:-5}"
    local locations=("hel1" "nbg1" "fsn1")
    local key_name="${VM_PREFIX}-key"
    local run_id="${WGMESH_RUN_ID:-${GITHUB_RUN_ID:-$(date +%s)}}"

    local letters=(a b c d e f g h)

    log_info "Provisioning $count VMs (prefix=${VM_PREFIX}, run=${run_id})..."

    local names=()

    for (( i=0; i<count; i++ )); do
        local role="node"
        local name
        if [ $i -eq 0 ]; then
            role="introducer"
            name="${VM_PREFIX}-${run_id}-introducer"
        else
            name="${VM_PREFIX}-${run_id}-node-${letters[$((i - 1))]}"
        fi
        # Spread across locations, with fallback on unavailability
        local loc="${locations[$(( i % ${#locations[@]} ))]}"

        names+=("$name")

        local created=false
        for try_loc in "$loc" "${locations[@]}"; do
            if hcloud server create \
                --name "$name" \
                --type "$VM_TYPE" \
                --image "$VM_IMAGE" \
                --location "$try_loc" \
                --ssh-key "$key_name" \
                --label "role=$role" \
                --label "run=$run_id" \
                --label "created=$(date +%s)" \
                >/dev/null 2>&1; then
                created=true
                log_info "Created $name in $try_loc"
                break
            else
                log_warn "Failed to create $name in $try_loc, trying next location..."
            fi
        done
        if [ "$created" = "false" ]; then
            log_error "Failed to create $name in any location"
            return 1
        fi
    done

    log_info "All $count VMs created"

    # Wait for SSH to become available on all VMs
    log_info "Waiting for SSH on all VMs..."
    for name in "${names[@]}"; do
        local ip
        ip=$(hcloud server ip "$name")
        _ensure_ssh_opts
        wait_for "SSH on $name ($ip)" 120 ssh "${SSH_OPTS[@]}" "root@${ip}" "true"
    done

    log_info "All VMs reachable via SSH"
}

# ---------------------------------------------------------------------------
# Node info population
# ---------------------------------------------------------------------------

# Populate NODE_IPS, NODE_ROLES, NODE_LOCATIONS from live VMs.
# NOTE: NODE_MESH_IPS is NOT populated here — mesh IPs are derived dynamically
# by the daemon. Call populate_mesh_ips after starting the mesh.
populate_node_info() {
    local run_id="${WGMESH_RUN_ID:-${GITHUB_RUN_ID:-}}"

    # Get server names
    local server_names
    if [ -n "$run_id" ]; then
        server_names=$(hcloud server list -l "run=$run_id" -o noheader -o columns=name 2>/dev/null) || true
    else
        server_names=$(hcloud server list -o noheader -o columns=name 2>/dev/null | grep "^${VM_PREFIX}") || true
    fi

    if [ -z "$server_names" ]; then
        log_error "No VMs found with prefix $VM_PREFIX"
        return 1
    fi

    # Reset arrays
    NODE_ROLES=()
    NODE_IPS=()
    NODE_MESH_IPS=()
    NODE_LOCATIONS=()

    while read -r full_name; do
        [ -z "$full_name" ] && continue

        # Get IP and datacenter via hcloud
        local ip dc
        ip=$(hcloud server ip "$full_name" 2>/dev/null) || continue
        dc=$(hcloud server describe "$full_name" -o format='{{.Datacenter.Name}}' 2>/dev/null) || dc="unknown"

        # Normalize name to short form
        local short_name
        if [[ "$full_name" == *-introducer ]]; then
            short_name="introducer"
            NODE_ROLES["$short_name"]="introducer"
        else
            # Extract suffix: wgmesh-ci-12345-node-a -> node-a
            short_name="${full_name##*-node-}"
            short_name="node-${short_name}"
            NODE_ROLES["$short_name"]="node"
        fi

        NODE_IPS["$short_name"]="$ip"
        NODE_LOCATIONS["$short_name"]="$dc"

    done <<< "$server_names"

    log_info "Populated ${#NODE_IPS[@]} nodes:"
    for name in $(echo "${!NODE_IPS[@]}" | tr ' ' '\n' | sort); do
        log_info "  $name: ip=${NODE_IPS[$name]} role=${NODE_ROLES[$name]} dc=${NODE_LOCATIONS[$name]}"
    done
}

# Query actual mesh IPs from running wg0 interfaces on each node.
# Must be called AFTER the mesh is started and interfaces are up.
# Retries a few times since WG interface may take a moment to come up.
populate_mesh_ips() {
    log_info "Querying mesh IPs from running nodes..."
    NODE_MESH_IPS=()

    local max_retries=5
    local retry=0

    while [ $retry -lt $max_retries ]; do
        local missing=0
        for node in "${!NODE_IPS[@]}"; do
            # Skip nodes we already have
            [ -n "${NODE_MESH_IPS[$node]:-}" ] && continue

            local mesh_ip
            # Extract the 10.x.x.x address from the wg0 interface
            mesh_ip=$(run_on "$node" "ip -4 addr show $WG_INTERFACE 2>/dev/null | grep -oP 'inet \K10\.[0-9]+\.[0-9]+\.[0-9]+'" 2>/dev/null) || mesh_ip=""

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
            log_info "Waiting for $missing node(s) to get mesh IPs (attempt $retry/$max_retries)..."
            sleep 5
        fi
    done

    if [ ${#NODE_MESH_IPS[@]} -eq 0 ]; then
        log_error "No mesh IPs found on any node"
        return 1
    fi

    log_info "Mesh IPs discovered (${#NODE_MESH_IPS[@]}/${#NODE_IPS[@]}):"
    for name in $(echo "${!NODE_MESH_IPS[@]}" | tr ' ' '\n' | sort); do
        log_info "  $name: mesh=${NODE_MESH_IPS[$name]}"
    done

    # Warn about missing nodes
    for node in "${!NODE_IPS[@]}"; do
        if [ -z "${NODE_MESH_IPS[$node]:-}" ]; then
            log_warn "No mesh IP for $node (wg0 not up?)"
        fi
    done
}

# ---------------------------------------------------------------------------
# VM setup (install dependencies, deploy binary, configure systemd)
# ---------------------------------------------------------------------------

# Install dependencies and configure wgmesh systemd service on all VMs.
# Requires: BINARY_PATH, MESH_SECRET
setup_all_vms() {
    [ -z "$BINARY_PATH" ] && { log_error "BINARY_PATH not set"; return 1; }
    [ -z "$MESH_SECRET" ] && { log_error "MESH_SECRET not set"; return 1; }

    log_info "Setting up all VMs (install deps, deploy binary, configure systemd)..."

    local pids=()
    for node in "${!NODE_IPS[@]}"; do
        _setup_single_vm "$node" &
        pids+=($!)
    done

    local failed=0
    for pid in "${pids[@]}"; do
        wait "$pid" || failed=$((failed + 1))
    done
    if [ "$failed" -gt 0 ]; then
        log_error "$failed VM(s) failed setup"
        return 1
    fi

    log_info "All VMs configured"
}

_setup_single_vm() {
    local node="$1"
    local role="${NODE_ROLES[$node]}"

    # Install dependencies
    run_on "$node" "
        export DEBIAN_FRONTEND=noninteractive
        apt-get update -qq
        apt-get install -y -qq wireguard-tools iperf3 jq iproute2 netcat-openbsd >/dev/null 2>&1
        mkdir -p /usr/local/bin /var/lib/wgmesh
    "

    # Optional: install coroot-node-agent for kernel-level (eBPF) observability.
    # Opt-in via WGMESH_COROOT=1 to avoid paying the install cost on every run.
    # Collector endpoint defaults to https://table.beerpub.dev (per memory:
    # Coroot observability setup); override via WGMESH_COROOT_COLLECTOR.
    #
    # Source defaults to nycterent's PR coroot/coroot-node-agent#301 branch
    # `fix-l7-memory-oom` (replaces stateful Prometheus objects with
    # lightweight storage to prevent OOM — fixes the agent OOMing at 9GB on
    # busy nodes, which would otherwise OOM-kill mid-test and obscure the
    # very hang we're trying to observe). Override the source with
    # WGMESH_COROOT_REPO and WGMESH_COROOT_REF for upstream comparison.
    #
    # Built from source on each VM so the binary matches whatever revision
    # WGMESH_COROOT_REF points to. ~1 minute extra setup; only runs when
    # WGMESH_COROOT=1.
    #
    # Agent needs root + privileged + debugfs to attach BPF programs. Useful
    # when diagnosing kernel-level hangs (e.g., wgmesh shutdown blocking on
    # netlink/RTNL — the symptom from Hetzner run 25609234757).
    if [ "${WGMESH_COROOT:-0}" = "1" ]; then
        local coroot_endpoint="${WGMESH_COROOT_COLLECTOR:-https://table.beerpub.dev}"
        local coroot_api_key="${WGMESH_COROOT_API_KEY:-${COROOT_API_TOKEN:-}}"
        if [ -z "$coroot_api_key" ]; then
            log_error "VM $node coroot enabled but COROOT_API_TOKEN/WGMESH_COROOT_API_KEY is empty — agent would be silently 401'd at ingestion. Pass the secret through to the test step env."
            return 1
        fi
        # Default to nycterent's fork — that's where PR coroot/coroot-node-agent#301
        # `fix-l7-memory-oom` lives. Upstream `coroot/coroot-node-agent` does NOT
        # have this branch (verified 2026-05-10 via gh pr view 301 — headRepositoryOwner
        # is `nycterent`). Run 25625632355 hit this: clone "succeeded" with
        # "Cloning into ..." but then `cd` failed because the branch was absent
        # in upstream. The remote bash session lacked `set -e`, so the cd/build/chmod
        # failures cascaded silently and the systemd unit ended up pointing at a
        # missing binary. Test VMs reported nothing to Coroot even though the
        # provision step claimed "built + started".
        local coroot_repo="${WGMESH_COROOT_REPO:-https://github.com/nycterent/coroot-node-agent.git}"
        local coroot_ref="${WGMESH_COROOT_REF:-fix-l7-memory-oom}"
        # Wrap in `set -e` so any failure (clone, build, install) aborts the
        # remote shell with non-zero, propagating to run_on, propagating to
        # this loop, propagating to the test step. No more lying log lines.
        # Add a final verification step that confirms the binary exists +
        # service became active before logging success.
        run_on "$node" "
            set -euo pipefail
            export DEBIAN_FRONTEND=noninteractive
            apt-get install -y -qq golang-go git build-essential libsystemd-dev libelf-dev pkg-config clang llvm linux-headers-generic >/dev/null 2>&1
            mkdir -p /opt/coroot
            cd /tmp
            rm -rf coroot-node-agent-src
            git clone --depth 1 --branch '${coroot_ref}' '${coroot_repo}' coroot-node-agent-src
            cd coroot-node-agent-src
            CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o /opt/coroot/coroot-node-agent .
            cd / && rm -rf /tmp/coroot-node-agent-src
            chmod +x /opt/coroot/coroot-node-agent
            test -x /opt/coroot/coroot-node-agent
            cat > /etc/systemd/system/coroot-node-agent.service << 'UNIT'
[Unit]
Description=Coroot eBPF node agent (test VM observability — PR #301 fix-l7-memory-oom)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/opt/coroot/coroot-node-agent --collector-endpoint=${coroot_endpoint} --api-key=${coroot_api_key} --cgroupfs-root=/sys/fs/cgroup
Restart=on-failure
LimitNOFILE=65535
MemoryHigh=2G
MemoryMax=4G

[Install]
WantedBy=multi-user.target
UNIT
            systemctl daemon-reload
            systemctl enable --now coroot-node-agent
            # Verify service reached active state (give it 5s to start).
            for i in 1 2 3 4 5; do
                state=\$(systemctl is-active coroot-node-agent 2>/dev/null || echo failed)
                [ \"\$state\" = active ] && break
                sleep 1
            done
            test \"\$(systemctl is-active coroot-node-agent)\" = active
        "
        log_info "VM $node coroot-node-agent built + started (ref=${coroot_ref}, collector=${coroot_endpoint})"
    fi

    # Copy binary
    copy_to "$node" "$BINARY_PATH" "/usr/local/bin/wgmesh"
    run_on "$node" "chmod +x /usr/local/bin/wgmesh"

    # Create systemd unit
    local extra_flags=""
    [ "$role" = "introducer" ] && extra_flags="--introducer"
    [ "${WGMESH_PPROF:-0}" = "1" ] && extra_flags="${extra_flags} --pprof localhost:6060"

    run_on "$node" "cat > /etc/systemd/system/wgmesh.service << 'UNIT'
[Unit]
Description=wgmesh integration test
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/wgmesh join \\
    --secret \"${MESH_SECRET}\" \\
    --interface ${WG_INTERFACE} \\
    --gossip \\
    ${extra_flags}
Restart=no
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
UNIT
    systemctl daemon-reload
    "

    log_info "VM $node setup complete (role=$role)"
}

# ---------------------------------------------------------------------------
# Start mesh: introducer first, then nodes
# ---------------------------------------------------------------------------

start_mesh() {
    local settle="${1:-30}"
    emit_event "mesh_start" "start_mesh" "settle=$settle"

    # Start introducer first
    for node in "${!NODE_ROLES[@]}"; do
        if [ "${NODE_ROLES[$node]}" = "introducer" ]; then
            start_mesh_node "$node"
            break
        fi
    done
    sleep 5

    # Start all other nodes
    for node in "${!NODE_ROLES[@]}"; do
        if [ "${NODE_ROLES[$node]}" != "introducer" ]; then
            start_mesh_node "$node"
        fi
    done

    log_info "Waiting ${settle}s for mesh to form..."
    sleep "$settle"

    # Discover actual mesh IPs from running WG interfaces
    populate_mesh_ips
    emit_event "mesh_started" "start_mesh" "nodes=${#NODE_IPS[@]}"
}

# Stop mesh on all nodes.
stop_mesh() {
    emit_event "mesh_stop" "stop_mesh"
    for node in "${!NODE_IPS[@]}"; do
        stop_mesh_node "$node" &
    done
    wait
    # Clean up WG interfaces
    for node in "${!NODE_IPS[@]}"; do
        run_on_ok "$node" "ip link del $WG_INTERFACE 2>/dev/null"
    done
    log_info "Mesh stopped on all nodes"
    emit_event "mesh_stopped" "stop_mesh"
}

# ---------------------------------------------------------------------------
# Teardown
# ---------------------------------------------------------------------------

# Delete ALL VMs and SSH key for this run.
teardown_vms() {
    local run_id="${WGMESH_RUN_ID:-${GITHUB_RUN_ID:-}}"

    log_info "Tearing down VMs..."

    # Delete by label if we have a run_id
    if [ -n "$run_id" ]; then
        local names
        names=$(hcloud server list -l "run=$run_id" -o noheader -o columns=name 2>/dev/null) || true
        for name in $names; do
            hcloud server delete "$name" &
        done
        wait
    else
        # Fallback: delete by prefix
        local names
        names=$(hcloud server list -o noheader -o columns=name | grep "^${VM_PREFIX}" 2>/dev/null) || true
        for name in $names; do
            hcloud server delete "$name" &
        done
        wait
    fi

    # Delete SSH key
    local key_name="${VM_PREFIX}-key"
    hcloud ssh-key delete "$key_name" 2>/dev/null || true

    log_info "Teardown complete"
}

# Delete orphaned VMs older than 30 minutes (safety net).
teardown_orphans() {
    local max_age=1800  # 30 minutes
    local now
    now=$(date +%s)

    log_info "Checking for orphaned ${VM_PREFIX}-* VMs..."

    local names
    names=$(hcloud server list -o noheader -o columns=name,labels | grep "^${VM_PREFIX}" 2>/dev/null) || true

    [ -z "$names" ] && { log_info "No orphans found"; return 0; }

    while read -r name labels; do
        local created
        created=$(echo "$labels" | grep -oP 'created=\K\d+' || echo "0")
        local age=$(( now - created ))
        if [ "$age" -gt "$max_age" ]; then
            log_warn "Deleting orphan: $name (age=${age}s)"
            hcloud server delete "$name" &
        fi
    done <<< "$names"
    wait
}
