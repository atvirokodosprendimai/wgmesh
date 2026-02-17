#!/bin/bash
# Quick helper for wgmesh testlab
#
# Usage:
#   ./lab.sh up          # Start VMs
#   ./lab.sh down        # Stop VMs
#   ./lab.sh ssh <node>  # SSH to node (introducer, node-a, node-b, node-c)
#   ./lab.sh logs        # Show logs from all nodes
#   ./lab.sh test        # Run connectivity tests
#   ./lab.sh build       # Rebuild and deploy binary
#   ./lab.sh restart     # Restart wgmesh on all nodes
#   ./lab.sh status      # Show WG status on all nodes

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

SECRET="wgmesh-test-vagrant-lab-2024"

generate_secret() {
    echo "wgmesh://v1/$(echo -n "$SECRET" | base32 | tr -d '=' | head -c 32)"
}

case "${1:-help}" in
    up)
        vagrant up --provision
        ;;
    down)
        vagrant halt
        ;;
    destroy)
        vagrant destroy -f
        ;;
    ssh)
        node=${2:-introducer}
        vagrant ssh "$node"
        ;;
    logs)
        for node in introducer node-a node-b node-c; do
            echo ""
            echo "=== $node ==="
            vagrant ssh $node -c "sudo tail -50 /var/log/wgmesh.log 2>/dev/null || echo 'No logs'" 2>/dev/null
        done
        ;;
    test)
        ./test-mesh.sh --skip-build
        ;;
    build)
        GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o wgmesh ..
        for node in introducer node-a node-b node-c; do
            vagrant upload wgmesh /opt/wgmesh/wgmesh $node
            vagrant ssh $node -c "chmod +x /opt/wgmesh/wgmesh" 2>/dev/null
        done
        echo "Binary rebuilt and deployed"
        ;;
    restart)
        for node in introducer node-a node-b node-c; do
            vagrant ssh $node -c "sudo pkill wgmesh; sudo ip link del wg0 2>/dev/null" 2>/dev/null || true
        done
        
        # Start introducer
        vagrant ssh introducer -c "
            sudo /opt/wgmesh/wgmesh join --secret '$(generate_secret)' --interface wg0 --mesh-ip 10.248.0.1 --introducer > /var/log/wgmesh.log 2>&1 &
        " 2>/dev/null
        
        sleep 3
        
        # Start nodes
        vagrant ssh node-a -c "
            sudo /opt/wgmesh/wgmesh join --secret '$(generate_secret)' --interface wg0 --mesh-ip 10.248.0.10 > /var/log/wgmesh.log 2>&1 &
        " 2>/dev/null
        
        vagrant ssh node-b -c "
            sudo /opt/wgmesh/wgmesh join --secret '$(generate_secret)' --interface wg0 --mesh-ip 10.248.0.20 > /var/log/wgmesh.log 2>&1 &
        " 2>/dev/null
        
        vagrant ssh node-c -c "
            sudo /opt/wgmesh/wgmesh join --secret '$(generate_secret)' --interface wg0 --mesh-ip 10.248.0.30 > /var/log/wgmesh.log 2>&1 &
        " 2>/dev/null
        
        echo "All nodes restarted"
        ;;
    status)
        for node in introducer node-a node-b node-c; do
            echo ""
            echo "=== $node ==="
            vagrant ssh $node -c "sudo wg show wg0 2>/dev/null | head -20 || echo 'WG not running'" 2>/dev/null
        done
        ;;
    ping)
        from=${2:-node-a}
        to=${3:-10.248.0.1}
        vagrant ssh $from -c "ping -c 5 $to" 2>/dev/null
        ;;
    help|*)
        echo "wgmesh Test Lab Helper"
        echo ""
        echo "Commands:"
        echo "  up              Start VMs"
        echo "  down            Stop VMs"
        echo "  destroy         Destroy VMs"
        echo "  ssh <node>      SSH to node (introducer, node-a, node-b, node-c)"
        echo "  logs            Show logs from all nodes"
        echo "  test            Run connectivity tests"
        echo "  build           Rebuild and deploy binary to all VMs"
        echo "  restart         Restart wgmesh on all nodes"
        echo "  status          Show WG status on all nodes"
        echo "  ping <from> <to> Ping from one node to another"
        echo ""
        echo "Mesh IPs:"
        echo "  introducer: 10.248.0.1"
        echo "  node-a:     10.248.0.10"
        echo "  node-b:     10.248.0.20"
        echo "  node-c:     10.248.0.30"
        ;;
esac
