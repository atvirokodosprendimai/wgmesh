#!/bin/bash
# wgmesh Lima Test Lab
# Uses Lima (native macOS hypervisor) instead of VirtualBox
#
# Usage:
#   ./lima-lab.sh init     # Create 2 Lima VMs
#   ./lima-lab.sh deploy   # Build and deploy binary
#   ./lima-lab.sh start    # Start wgmesh on both VMs
#   ./lima-lab.sh test     # Test connectivity
#   ./lima-lab.sh stop     # Stop VMs
#   ./lima-lab.sh destroy  # Delete VMs

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SECRET="wgmesh-test-lima-lab-2024"

NODE_A="wgmesh-a"
NODE_B="wgmesh-b"

log_info() { echo -e "\033[0;32m[INFO]\033[0m $1"; }
log_error() { echo -e "\033[0;31m[ERROR]\033[0m $1"; }

generate_secret() {
    echo "wgmesh://v1/$(echo -n "$SECRET" | base32 | tr -d '=' | head -c 32)"
}

init_vms() {
    log_info "Creating Lima VM $NODE_A (introducer)..."
    limactl create -y --name=$NODE_A template:ubuntu \
        --cpus=2 --memory=2 --disk=10 \
        --mount="$SCRIPT_DIR/..:ro"

    log_info "Creating Lima VM $NODE_B..."
    limactl create -y --name=$NODE_B template:ubuntu \
        --cpus=2 --memory=2 --disk=10 \
        --mount="$SCRIPT_DIR/..:ro"

    log_info "Starting VMs..."
    limactl start $NODE_A
    limactl start $NODE_B
    
    log_info "Installing dependencies..."
    for node in $NODE_A $NODE_B; do
        limactl shell $node -- sudo apt-get update
        limactl shell $node -- sudo apt-get install -y wireguard-tools curl jq
        limactl shell $node -- sudo mkdir -p /opt/wgmesh /var/lib/wgmesh
    done
    
    log_info "VMs ready."
    limactl list
}

deploy_binary() {
    log_info "Building and deploying binary..."
    
    # Build for linux arm64 (matches Lima VM arch on Apple Silicon)
    cd "$SCRIPT_DIR/.."
    GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o testlab/wgmesh .
    
    # Copy to VMs via shared mount
    for node in $NODE_A $NODE_B; do
        limactl shell $node -- sudo cp /tmp/lima/testlab/wgmesh /opt/wgmesh/wgmesh
        limactl shell $node -- sudo chmod +x /opt/wgmesh/wgmesh
    done
    
    log_info "Binary deployed"
}

start_mesh() {
    log_info "Starting wgmesh on VMs..."
    
    # Start node-a as introducer
    limactl shell $NODE_A -- sudo pkill wgmesh 2>/dev/null || true
    limactl shell $NODE_A -- sudo ip link del wg0 2>/dev/null || true
    sleep 1
    
    limactl shell $NODE_A -- bash -c "sudo /opt/wgmesh/wgmesh join \
        --secret '$(generate_secret)' \
        --interface wg0 \
        --mesh-ip 10.248.0.1 \
        --introducer \
        > /var/log/wgmesh.log 2>&1 &"
    
    log_info "Waiting for introducer to start..."
    sleep 10
    
    # Start node-b
    limactl shell $NODE_B -- sudo pkill wgmesh 2>/dev/null || true
    limactl shell $NODE_B -- sudo ip link del wg0 2>/dev/null || true
    sleep 1
    
    limactl shell $NODE_B -- bash -c "sudo /opt/wgmesh/wgmesh join \
        --secret '$(generate_secret)' \
        --interface wg0 \
        --mesh-ip 10.248.0.2 \
        > /var/log/wgmesh.log 2>&1 &"
    
    log_info "Mesh started. Waiting 30s for discovery..."
    sleep 30
}

run_tests() {
    log_info "Running connectivity tests..."
    
    local PASS=0
    local FAIL=0
    
    echo ""
    echo "=== Node A -> Node B (10.248.0.2) ==="
    if limactl shell $NODE_A -- ping -c 3 -W 5 10.248.0.2; then
        echo -e "\033[0;32mPASS\033[0m"
        ((PASS++))
    else
        echo -e "\033[0;31mFAIL\033[0m"
        ((FAIL++))
    fi
    
    echo ""
    echo "=== Node B -> Node A (10.248.0.1) ==="
    if limactl shell $NODE_B -- ping -c 3 -W 5 10.248.0.1; then
        echo -e "\033[0;32mPASS\033[0m"
        ((PASS++))
    else
        echo -e "\033[0;31mFAIL\033[0m"
        ((FAIL++))
    fi
    
    echo ""
    echo "Results: $PASS passed, $FAIL failed"
    return $FAIL
}

show_logs() {
    echo ""
    echo "=== Node A logs ==="
    limactl shell $NODE_A -- sudo tail -50 /var/log/wgmesh.log 2>/dev/null || echo "No logs"
    
    echo ""
    echo "=== Node B logs ==="
    limactl shell $NODE_B -- sudo tail -50 /var/log/wgmesh.log 2>/dev/null || echo "No logs"
}

show_status() {
    echo ""
    echo "=== Node A WG status ==="
    limactl shell $NODE_A -- sudo wg show wg0 2>/dev/null || echo "WG not running"
    
    echo ""
    echo "=== Node B WG status ==="
    limactl shell $NODE_B -- sudo wg show wg0 2>/dev/null || echo "WG not running"
}

stop_vms() {
    log_info "Stopping VMs..."
    limactl stop $NODE_A 2>/dev/null || true
    limactl stop $NODE_B 2>/dev/null || true
}

destroy_vms() {
    log_info "Destroying VMs..."
    limactl delete $NODE_A 2>/dev/null || true
    limactl delete $NODE_B 2>/dev/null || true
}

case "${1:-help}" in
    init)
        init_vms
        ;;
    deploy)
        deploy_binary
        ;;
    start)
        start_mesh
        ;;
    test)
        run_tests
        ;;
    logs)
        show_logs
        ;;
    status)
        show_status
        ;;
    stop)
        stop_vms
        ;;
    destroy)
        destroy_vms
        ;;
    ssh-a)
        limactl shell $NODE_A
        ;;
    ssh-b)
        limactl shell $NODE_B
        ;;
    all)
        init_vms
        deploy_binary
        start_mesh
        run_tests
        show_status
        ;;
    help|*)
        echo "wgmesh Lima Test Lab"
        echo ""
        echo "Commands:"
        echo "  init      Create and provision VMs"
        echo "  deploy    Build and deploy binary to VMs"
        echo "  start     Start wgmesh on all VMs"
        echo "  test      Run connectivity tests"
        echo "  logs      Show logs from all VMs"
        echo "  status    Show WG status"
        echo "  stop      Stop VMs"
        echo "  destroy   Delete VMs"
        echo "  ssh-a     SSH to node A"
        echo "  ssh-b     SSH to node B"
        echo "  all       Run full test cycle"
        ;;
esac
