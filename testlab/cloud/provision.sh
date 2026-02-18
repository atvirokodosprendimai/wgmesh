#!/usr/bin/env bash
# provision.sh — Create and destroy Hetzner Cloud VMs for wgmesh testing
#
# Usage:
#   source lib.sh
#   source provision.sh
#
#   provision_infra 5          # Create SSH key + 5 VMs via OpenTofu
#   teardown_infra             # Destroy all via OpenTofu + hcloud fallback
#   teardown_orphans           # Delete any stale wgmesh-ci-* VMs older than 30min
#
# Legacy (hcloud CLI only — used as fallback):
#   provision_ssh_key          # Create or reuse SSH key
#   provision_vms 5            # Create 5 VMs (1 introducer + 4 nodes)
#   populate_node_info         # Fill NODE_IPS, NODE_MESH_IPS, etc.
#   teardown_vms               # Delete all VMs and SSH key

set -euo pipefail

TOFU_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/tofu"

# ---------------------------------------------------------------------------
# OpenTofu provisioning (primary path)
# ---------------------------------------------------------------------------

# Provision SSH key + VMs via OpenTofu.
# Replaces: provision_ssh_key + provision_vms
# Usage: provision_infra [vm_count]
provision_infra() {
    local vm_count="${1:-5}"
    local run_id="${GITHUB_RUN_ID:-$(date +%s)}"

    # Ensure SSH key exists locally
    if [ ! -f "$SSH_KEY_FILE" ]; then
        ssh-keygen -t ed25519 -f "$SSH_KEY_FILE" -N "" -q
        log_info "Generated SSH key: $SSH_KEY_FILE"
    fi

    # Try different server types until one works
    # Valid Hetzner cloud server types (as of 2025):
    # - cpx22, cpx32 (AMD shared vCPU - Gen 2)
    # - cax11, cax21 (ARM64 Ampere)
    # - cx23, cx33 (Intel/AMD cost-optimized Gen 3)
    # - ccx13, ccx23 (AMD dedicated vCPU)
    local server_types=("cpx22" "cpx32" "cax11" "cax21" "cx23" "cx33" "ccx13" "ccx23")
    local provisioned=false

    for server_type in "${server_types[@]}"; do
        log_info "Trying to provision with server type: $server_type"

        tofu -chdir="$TOFU_DIR" init -input=false

        # Try up to 3 times per server type with backoff
        for attempt in 1 2 3; do
            if [ $attempt -gt 1 ]; then
                log_warn "Retry attempt $attempt for server type: $server_type"
                sleep $((attempt * 5))  # 5s, 10s, 15s backoff
            fi

            if tofu -chdir="$TOFU_DIR" apply -auto-approve -input=false \
                -var="hcloud_token=${HCLOUD_TOKEN}" \
                -var="run_id=${run_id}" \
                -var="vm_count=${vm_count}" \
                -var="ssh_public_key_path=${SSH_KEY_FILE}.pub" \
                -var="server_type=${server_type}"; then

                provisioned=true
                log_info "Successfully provisioned with server type: $server_type"
                break 2
            else
                log_warn "Failed attempt $attempt for server type: $server_type"
            fi
        done

        # Clean up any partial resources before trying next type
        log_info "Cleaning up failed attempt for server type: $server_type"
        tofu -chdir="$TOFU_DIR" destroy -auto-approve -input=false \
            -var="hcloud_token=${HCLOUD_TOKEN}" \
            -var="run_id=${run_id}" \
            -var="vm_count=${vm_count}" \
            -var="ssh_public_key_path=${SSH_KEY_FILE}.pub" \
            -var="server_type=${server_type}" 2>/dev/null || true
        
        # Short delay before trying next type
        sleep 2
    done

    if [ "$provisioned" = "false" ]; then
        log_error "Failed to provision VMs with any server type"
        return 1
    fi

    load_tofu_outputs

    # Wait for SSH on all nodes
    log_info "Waiting for SSH on all VMs..."
    for node in "${!NODE_IPS[@]}"; do
        wait_for "SSH on $node (${NODE_IPS[$node]})" 120 ssh \
            -o StrictHostKeyChecking=no \
            -o UserKnownHostsFile=/dev/null \
            -o ConnectTimeout=5 \
            -o LogLevel=ERROR \
            -i "$SSH_KEY_FILE" \
            "root@${NODE_IPS[$node]}" "true"
    done

    log_info "All $vm_count VMs created and reachable via SSH"
}

# Populate NODE_IPS, NODE_ROLES, NODE_LOCATIONS from OpenTofu outputs.
# Replaces: populate_node_info
load_tofu_outputs() {
    local outputs
    outputs=$(tofu -chdir="$TOFU_DIR" output -json)

    # Reset arrays
    NODE_ROLES=()
    NODE_IPS=()
    NODE_MESH_IPS=()
    NODE_LOCATIONS=()

    while IFS='=' read -r key val; do
        NODE_IPS["$key"]="$val"
    done < <(echo "$outputs" | jq -r '.node_ips.value | to_entries[] | "\(.key)=\(.value)"')

    while IFS='=' read -r key val; do
        NODE_ROLES["$key"]="$val"
    done < <(echo "$outputs" | jq -r '.node_roles.value | to_entries[] | "\(.key)=\(.value)"')

    while IFS='=' read -r key val; do
        NODE_LOCATIONS["$key"]="$val"
    done < <(echo "$outputs" | jq -r '.node_locations.value | to_entries[] | "\(.key)=\(.value)"')

    log_info "Loaded ${#NODE_IPS[@]} nodes from OpenTofu outputs:"
    for name in $(echo "${!NODE_IPS[@]}" | tr ' ' '\n' | sort); do
        log_info "  $name: ip=${NODE_IPS[$name]} role=${NODE_ROLES[$name]} dc=${NODE_LOCATIONS[$name]}"
    done
}

# Destroy all infrastructure via OpenTofu, with hcloud CLI fallback.
# Replaces: teardown_vms
teardown_infra() {
    if [ -f "$TOFU_DIR/terraform.tfstate" ]; then
        log_info "Destroying infrastructure with OpenTofu..."
        tofu -chdir="$TOFU_DIR" destroy -auto-approve -input=false \
            -var="hcloud_token=${HCLOUD_TOKEN}" \
            -var="run_id=${GITHUB_RUN_ID:-0}" \
            -var="ssh_public_key_path=${SSH_KEY_FILE}.pub" \
            || log_warn "OpenTofu destroy failed, falling back to hcloud CLI"
    else
        log_warn "No OpenTofu state found, using hcloud CLI fallback"
    fi

    # Safety fallback: clean up by label/prefix if tofu state is missing or destroy failed
    teardown_vms
}

# ---------------------------------------------------------------------------
# Legacy hcloud CLI provisioning (kept as fallback)
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

# Provision N VMs across multiple locations (legacy hcloud CLI path).
# VM 0 = introducer (hel1), VMs 1..N-1 = nodes spread across locations.
# Usage: provision_vms <count>
provision_vms() {
    local count="${1:-5}"
    local locations=("hel1" "nbg1" "fsn1")
    local key_name="${VM_PREFIX}-key"
    local run_id="${GITHUB_RUN_ID:-$(date +%s)}"

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
        # Try different server types and locations
        local server_types=("$VM_TYPE" "cpx32" "cax11" "cax21" "cx23" "cx33" "ccx13" "ccx23")
        for try_type in "${server_types[@]}"; do
            for try_loc in "$loc" "${locations[@]}"; do
                if hcloud server create \
                    --name "$name" \
                    --type "$try_type" \
                    --image "$VM_IMAGE" \
                    --location "$try_loc" \
                    --ssh-key "$key_name" \
                    --label "role=$role" \
                    --label "run=$run_id" \
                    --label "created=$(date +%s)" \
                    >/dev/null 2>&1; then
                    created=true
                    log_info "Created $name in $try_loc with type $try_type"
                    break 2
                else
                    log_warn "Failed to create $name ($try_type) in $try_loc, trying..."
                fi
            done
        done
        if [ "$created" = "false" ]; then
            log_error "Failed to create $name with any server type or location"
            return 1
        fi
    done

    log_info "All $count VMs created"

    # Wait for SSH to become available on all VMs
    log_info "Waiting for SSH on all VMs..."
    for name in "${names[@]}"; do
        local ip
        ip=$(hcloud server ip "$name")
        wait_for "SSH on $name ($ip)" 120 ssh \
            -o StrictHostKeyChecking=no \
            -o UserKnownHostsFile=/dev/null \
            -o ConnectTimeout=5 \
            -o LogLevel=ERROR \
            -i "$SSH_KEY_FILE" \
            "root@${ip}" "true"
    done

    log_info "All VMs reachable via SSH"
}

# Populate NODE_IPS, NODE_ROLES, NODE_LOCATIONS from live VMs (legacy hcloud CLI path).
# NOTE: NODE_MESH_IPS is NOT populated here — mesh IPs are derived dynamically
# by the daemon. Call populate_mesh_ips after starting the mesh.
populate_node_info() {
    local run_id="${GITHUB_RUN_ID:-}"

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
        apt-get install -y -qq wireguard-tools iperf3 jq iproute2 >/dev/null 2>&1
        mkdir -p /usr/local/bin /var/lib/wgmesh
    "

    # Copy binary
    copy_to "$node" "$BINARY_PATH" "/usr/local/bin/wgmesh"
    run_on "$node" "chmod +x /usr/local/bin/wgmesh"

    # Create systemd unit
    local extra_flags=""
    [ "$role" = "introducer" ] && extra_flags="--introducer"

    # Build OTel environment block if endpoint is configured
    local otel_env=""
    if [ -n "${OTEL_EXPORTER_OTLP_ENDPOINT:-}" ]; then
        otel_env="Environment=OTEL_EXPORTER_OTLP_ENDPOINT=${OTEL_EXPORTER_OTLP_ENDPOINT}
Environment=OTEL_EXPORTER_OTLP_HEADERS=${OTEL_EXPORTER_OTLP_HEADERS:-}
Environment=OTEL_RESOURCE_ATTRIBUTES=test.run_id=${GITHUB_RUN_ID:-local},test.vm=${node},test.role=${role}"
    fi

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
${otel_env}

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
}

# Stop mesh on all nodes.
stop_mesh() {
    for node in "${!NODE_IPS[@]}"; do
        stop_mesh_node "$node" &
    done
    wait
    # Clean up WG interfaces
    for node in "${!NODE_IPS[@]}"; do
        run_on_ok "$node" "ip link del $WG_INTERFACE 2>/dev/null"
    done
    log_info "Mesh stopped on all nodes"
}

# ---------------------------------------------------------------------------
# Teardown
# ---------------------------------------------------------------------------

# Delete ALL VMs and SSH key for this run.
teardown_vms() {
    local run_id="${GITHUB_RUN_ID:-}"

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
