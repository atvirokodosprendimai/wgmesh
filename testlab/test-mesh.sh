#!/bin/bash
# wgmesh Integration Test
# Tests mesh connectivity in Vagrant lab environment
#
# Prerequisites:
#   - VirtualBox installed
#   - Vagrant installed
#   - Go toolchain for building
#
# Usage:
#   ./test-mesh.sh              # Full test cycle
#   ./test-mesh.sh --skip-build # Skip rebuild, just test
#   ./test-mesh.sh --cleanup    # Destroy VMs after test

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="$(dirname "$SCRIPT_DIR")"
SECRET="wgmesh-test-vagrant-lab-2024"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Build wgmesh for Linux amd64
build_binary() {
    log_info "Building wgmesh for Linux..."
    cd "$BUILD_DIR"
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o testlab/wgmesh .
    log_info "Binary built: $(ls -lh testlab/wgmesh)"
}

# Start Vagrant VMs
start_vms() {
    log_info "Starting Vagrant VMs..."
    cd "$SCRIPT_DIR"
    vagrant up --provision
    log_info "VMs ready"
}

# Copy binary to all VMs
deploy_binary() {
    log_info "Deploying binary to VMs..."
    cd "$SCRIPT_DIR"
    for node in introducer node-a node-b node-c; do
        vagrant upload wgmesh /opt/wgmesh/wgmesh $node
        vagrant ssh $node -c "chmod +x /opt/wgmesh/wgmesh" 2>/dev/null
    done
    log_info "Binary deployed to all nodes"
}

# Generate mesh secret
generate_secret() {
    # Use a deterministic secret for testing
    # Format: wgmesh://v1/<base32-encoded-key>
    echo "wgmesh://v1/$(echo -n "$SECRET" | base32 | tr -d '=' | head -c 32)"
}

# Start wgmesh on a node
start_node() {
    local node=$1
    local mesh_ip=$2
    local extra_flags=$3
    
    log_info "Starting wgmesh on $node (mesh IP: $mesh_ip)..."
    cd "$SCRIPT_DIR"
    
    vagrant ssh $node -c "
        sudo pkill wgmesh 2>/dev/null || true
        sudo ip link del wg0 2>/dev/null || true
        sleep 1
        sudo /opt/wgmesh/wgmesh join \
            --secret '$(generate_secret)' \
            --interface wg0 \
            --mesh-ip $mesh_ip \
            $extra_flags \
            > /var/log/wgmesh.log 2>&1 &
        sleep 2
        echo 'Started wgmesh on $node'
    " 2>/dev/null
}

# Check if mesh IP is reachable
check_connectivity() {
    local from=$1
    local to_ip=$2
    local timeout=${3:-10}
    
    cd "$SCRIPT_DIR"
    vagrant ssh $from -c "
        timeout $timeout ping -c 3 -W 2 $to_ip 2>/dev/null
    " 2>/dev/null | grep -q "bytes from"
}

# Run WireGuard handshake check
check_handshake() {
    local node=$1
    local expected_peers=$2
    
    cd "$SCRIPT_DIR"
    vagrant ssh $node -c "
        sudo wg show wg0 latest-handshakes 2>/dev/null | wc -l
    " 2>/dev/null | tr -d '\r'
}

# Test matrix
run_tests() {
    log_info "Running connectivity tests..."
    
    local TESTS_PASSED=0
    local TESTS_FAILED=0
    
    # Test matrix: (from, to_mesh_ip)
    local tests=(
        "node-a:10.248.0.1:node-a -> introducer"
        "node-a:10.248.0.20:node-a -> node-b"
        "node-a:10.248.0.30:node-a -> node-c"
        "node-b:10.248.0.1:node-b -> introducer"
        "node-b:10.248.0.10:node-b -> node-a"
        "node-b:10.248.0.30:node-b -> node-c"
        "node-c:10.248.0.1:node-c -> introducer"
        "node-c:10.248.0.10:node-c -> node-a"
        "node-c:10.248.0.20:node-c -> node-b"
    )
    
    echo ""
    echo "============================================"
    echo "         wgmesh Connectivity Tests"
    echo "============================================"
    printf "%-25s %-15s %s\n" "Test" "Result" "Latency"
    echo "--------------------------------------------"
    
    for test in "${tests[@]}"; do
        IFS=':' read -r from to_ip name <<< "$test"
        
        if check_connectivity "$from" "$to_ip" 15; then
            printf "%-25s ${GREEN}%-15s${NC} OK\n" "$name" "PASS"
            ((TESTS_PASSED++))
        else
            printf "%-25s ${RED}%-15s${NC} --\n" "$name" "FAIL"
            ((TESTS_FAILED++))
        fi
    done
    
    echo "--------------------------------------------"
    echo "Passed: $TESTS_PASSED / $((TESTS_PASSED + TESTS_FAILED))"
    
    if [ $TESTS_FAILED -eq 0 ]; then
        log_info "All tests passed!"
        return 0
    else
        log_error "$TESTS_FAILED tests failed"
        return 1
    fi
}

# Show logs from all nodes
show_logs() {
    log_info "Collecting logs..."
    cd "$SCRIPT_DIR"
    for node in introducer node-a node-b node-c; do
        echo ""
        echo "=== $node ==="
        vagrant ssh $node -c "sudo tail -30 /var/log/wgmesh.log 2>/dev/null || echo 'No logs'" 2>/dev/null
    done
}

# Cleanup VMs
cleanup() {
    log_info "Destroying VMs..."
    cd "$SCRIPT_DIR"
    vagrant destroy -f 2>/dev/null || true
    rm -f wgmesh
}

# Main
main() {
    local skip_build=false
    local do_cleanup=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-build) skip_build=true; shift ;;
            --cleanup) do_cleanup=true; shift ;;
            --logs) show_logs; exit 0 ;;
            *) log_error "Unknown option: $1"; exit 1 ;;
        esac
    done
    
    if [ "$skip_build" = false ]; then
        build_binary
    fi
    
    start_vms
    deploy_binary
    
    # Start introducer first
    start_node "introducer" "10.248.0.1" "--introducer"
    
    # Wait for introducer to be ready
    sleep 5
    
    # Start other nodes
    start_node "node-a" "10.248.0.10" ""
    start_node "node-b" "10.248.0.20" ""
    start_node "node-c" "10.248.0.30" ""
    
    # Wait for mesh to form
    log_info "Waiting for mesh to form (30s)..."
    sleep 30
    
    # Run tests
    run_tests
    local test_result=$?
    
    if [ $test_result -ne 0 ]; then
        show_logs
    fi
    
    if [ "$do_cleanup" = true ]; then
        cleanup
    fi
    
    exit $test_result
}

main "$@"
