#!/bin/bash
# wgmesh Lima Test Lab with separate networks
# Creates 2 VMs on different vzNAT networks to simulate real NAT traversal
#
# Usage:
#   ./lima-bridge-lab.sh init     # Create 2 Lima VMs
#   ./lima-bridge-lab.sh deploy   # Build and deploy binary
#   ./lima-bridge-lab.sh start    # Start wgmesh on both VMs
#   ./lima-bridge-lab.sh test     # Test connectivity
#   ./lima-bridge-lab.sh destroy  # Delete VMs

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SECRET="wgmesh-lima-test-2026"

NODE_A="wgmesh-a"
NODE_B="wgmesh-b"

log_info() { echo -e "\033[0;32m[INFO]\033[0m $1"; }
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

generate_secret() {
    echo "wgmesh://v1/$SECRET"
}

init_vms() {
    log_info "Creating Lima VM $NODE_A (introducer)..."
    limactl create -y --name=$NODE_A template:ubuntu \
        --cpus=2 --memory=2 --disk=10 \
        --network=vzNAT
    limactl start $NODE_A
    
    log_info "Creating Lima VM $NODE_B..."
    limactl create -y --name=$NODE_B template:ubuntu \
        --cpus=2 --memory=2 --disk=10 \
        --network=vzNAT
    limactl start $NODE_B
    
    log_info "Installing dependencies..."
    for node in $NODE_A $NODE_B; do
        limactl shell $node -- sudo apt-get update -qq
        limactl shell $node -- sudo apt-get install -y -qq wireguard-tools curl jq
        limactl shell $node -- sudo mkdir -p /opt/wgmesh /var/lib/wgmesh
        limactl shell $node -- sudo touch /var/log/wgmesh.log
        limactl shell $node -- sudo chmod 666 /var/log/wgmesh.log
    done
    
    log_info "VMs ready."
    limactl list
}

get_vm_ip() {
    local vm=$1
    limactl shell $vm -- ip -4 addr show 2>/dev/null | grep -oP 'inet \K[\d.]+' | grep -v '^127\.' | head -1
}

deploy_binary() {
    log_info "Building and deploying binary..."
    
    cd "$SCRIPT_DIR/.."
    GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o testlab/wgmesh .
    
    limactl copy testlab/wgmesh $NODE_A:/tmp/wgmesh
    limactl copy testlab/wgmesh $NODE_B:/tmp/wgmesh
    
    limactl shell $NODE_A -- sudo cp /tmp/wgmesh /opt/wgmesh/wgmesh
    limactl shell $NODE_A -- sudo chmod +x /opt/wgmesh/wgmesh
    
    limactl shell $NODE_B -- sudo cp /tmp/wgmesh /opt/wgmesh/wgmesh
    limactl shell $NODE_B -- sudo chmod +x /opt/wgmesh/wgmesh
    
    log_info "Binary deployed"
}

start_mesh() {
    log_info "Getting VM IPs..."
    IP_A=$(get_vm_ip $NODE_A)
    IP_B=$(get_vm_ip $NODE_B)
    
    log_info "Node A IP: $IP_A"
    log_info "Node B IP: $IP_B"
    
    # Start introducer on node-a
    log_info "Starting introducer on $NODE_A..."
    limactl shell $NODE_A -- bash -c "sudo pkill wgmesh 2>/dev/null || true"
    limactl shell $NODE_A -- bash -c "sudo ip link del wg0 2>/dev/null || true"
    limactl shell $NODE_A -- bash -c "sudo /opt/wgmesh/wgmesh join --secret '$(generate_secret)' --interface wg0 --introducer > /var/log/wgmesh.log 2>&1 &"
    
    sleep 5
    
    # Start node-b
    log_info "Starting node on $NODE_B..."
    limactl shell $NODE_B -- bash -c "sudo pkill wgmesh 2>/dev/null || true"
    limactl shell $NODE_B -- bash -c "sudo ip link del wg0 2>/dev/null || true"
    limactl shell $NODE_B -- bash -c "sudo /opt/wgmesh/wgmesh join --secret '$(generate_secret)' --interface wg0 > /var/log/wgmesh.log 2>&1 &"
    
    log_info "Mesh started. Waiting 45s for discovery..."
    sleep 45
    log_info "Discovery complete"
}

run_tests() {
    log_info "Running connectivity tests..."
    
    # Get mesh IPs from logs
    MESH_A=$(limactl shell $NODE_A -- grep "Mesh IP:" /var/log/wgmesh.log 2>/dev/null | grep -oP '\d+\.\d+\.\d+\.\d+' | head -1)
    MESH_B=$(limactl shell $NODE_B -- grep "Mesh IP:" /var/log/wgmesh.log 2>/dev/null | grep -oP '\d+\.\d+\.\d+\.\d+' | head -1)
    
    log_info "Mesh IPs: A=$MESH_A, B=$MESH_B"
    
    local PASS=0
    local FAIL=0
    
    echo ""
    echo "============================================"
    echo "         wgmesh Connectivity Tests"
    echo "============================================"
    
    if [ -n "$MESH_B" ]; then
        printf "%-30s " "Node A -> Node B ($MESH_B)"
        if limactl shell $NODE_A -- ping -c 3 -W 5 $MESH_B 2>&1 | grep -q "bytes from"; then
            echo -e "${GREEN}PASS${NC}"
            ((PASS++))
        else
            echo -e "${RED}FAIL${NC}"
            ((FAIL++))
        fi
    fi
    
    if [ -n "$MESH_A" ]; then
        printf "%-30s " "Node B -> Node A ($MESH_A)"
        if limactl shell $NODE_B -- ping -c 3 -W 5 $MESH_A 2>&1 | grep -q "bytes from"; then
            echo -e "${GREEN}PASS${NC}"
            ((PASS++))
        else
            echo -e "${RED}FAIL${NC}"
            ((FAIL++))
        fi
    fi
    
    echo "--------------------------------------------"
    echo "Results: $PASS passed, $FAIL failed"
    
    return $FAIL
}

show_logs() {
    for node in $NODE_A $NODE_B; do
        echo ""
        echo "=== $node logs ==="
        limactl shell $node -- tail -40 /var/log/wgmesh.log 2>/dev/null || echo "No logs"
    done
}

show_status() {
    for node in $NODE_A $NODE_B; do
        echo ""
        echo "=== $node WG status ==="
        limactl shell $node -- sudo wg show wg0 2>/dev/null || echo "WG not running"
    done
}

destroy_vms() {
    log_info "Destroying VMs..."
    limactl stop --force $NODE_A $NODE_B 2>/dev/null || true
    limactl delete --force $NODE_A $NODE_B 2>/dev/null || true
}

case "${1:-help}" in
    init) init_vms ;;
    deploy) deploy_binary ;;
    start) start_mesh ;;
    test) run_tests; show_status ;;
    logs) show_logs ;;
    status) show_status ;;
    destroy) destroy_vms ;;
    ssh-a) limactl shell $NODE_A ;;
    ssh-b) limactl shell $NODE_B ;;
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
        echo "  init      Create VMs"
        echo "  deploy    Build and deploy binary"
        echo "  start     Start wgmesh"
        echo "  test      Run tests"
        echo "  logs      Show logs"
        echo "  status    Show WG status"
        echo "  destroy   Delete VMs"
        echo "  all       Full test cycle"
        ;;
esac
