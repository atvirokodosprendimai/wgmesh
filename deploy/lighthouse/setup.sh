#!/usr/bin/env bash
# Lighthouse control plane setup for cloudroof.eu CDN.
# Installs Dragonfly (local state store) and the lighthouse binary.
#
# Usage: LIGHTHOUSE_NODE_ID=lh-nbg1 LIGHTHOUSE_MESH_IP=10.42.0.1 ./setup.sh
# Prerequisites: WireGuard mesh already running on this node.

set -euo pipefail

# --- Configuration ---
LIGHTHOUSE_USER="${LIGHTHOUSE_USER:-lighthouse}"
LIGHTHOUSE_DIR="/opt/lighthouse"
LIGHTHOUSE_ADDR="${LIGHTHOUSE_ADDR:-:8443}"
LIGHTHOUSE_NODE_ID="${LIGHTHOUSE_NODE_ID:?Must set LIGHTHOUSE_NODE_ID}"
LIGHTHOUSE_MESH_IP="${LIGHTHOUSE_MESH_IP:?Must set LIGHTHOUSE_MESH_IP}"
LIGHTHOUSE_DNS_TARGET="${LIGHTHOUSE_DNS_TARGET:-edge.cloudroof.eu}"
LIGHTHOUSE_PEERS="${LIGHTHOUSE_PEERS:-}" # Comma-separated mesh IPs of other lighthouses
DRAGONFLY_PORT="${DRAGONFLY_PORT:-6379}"

echo "=== Lighthouse Setup ==="
echo "Node ID:    $LIGHTHOUSE_NODE_ID"
echo "Mesh IP:    $LIGHTHOUSE_MESH_IP"
echo "Listen:     $LIGHTHOUSE_ADDR"
echo "DNS target: $LIGHTHOUSE_DNS_TARGET"

# --- Create user ---
if ! id "$LIGHTHOUSE_USER" &>/dev/null; then
    useradd --system --no-create-home --shell /usr/sbin/nologin "$LIGHTHOUSE_USER"
    echo "Created user: $LIGHTHOUSE_USER"
fi

# --- Install Dragonfly ---
if ! command -v dragonfly &>/dev/null && ! [ -f /usr/local/bin/dragonfly ]; then
    echo "Installing Dragonfly..."
    DF_VERSION=$(curl -sL https://api.github.com/repos/dragonflydb/dragonfly/releases/latest | \
        python3 -c "import sys,json; print(json.load(sys.stdin)['tag_name'])")

    if [ -z "$DF_VERSION" ] || [ "$DF_VERSION" = "null" ]; then
        echo "ERROR: Failed to fetch Dragonfly version from GitHub API"
        exit 1
    fi

    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)  DF_ARCH="x86_64" ;;
        aarch64) DF_ARCH="aarch64" ;;
        *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
    esac

    DF_URL="https://github.com/dragonflydb/dragonfly/releases/download/${DF_VERSION}/dragonfly-${DF_ARCH}.tar.gz"
    echo "Downloading: $DF_URL"
    curl -sL "$DF_URL" -o /tmp/dragonfly.tar.gz

    # Verify download
    if ! tar -tzf /tmp/dragonfly.tar.gz &>/dev/null; then
        echo "ERROR: Downloaded file is not a valid tarball"
        exit 1
    fi

    tar -xzf /tmp/dragonfly.tar.gz -C /tmp
    install -m 755 /tmp/dragonfly-* /usr/local/bin/dragonfly || \
        install -m 755 /tmp/dragonfly /usr/local/bin/dragonfly
    rm -f /tmp/dragonfly.tar.gz /tmp/dragonfly-*
    echo "Dragonfly installed: $(dragonfly --version 2>&1 || echo 'ok')"
fi

# --- Dragonfly data directory ---
mkdir -p /var/lib/dragonfly
chown "$LIGHTHOUSE_USER:$LIGHTHOUSE_USER" /var/lib/dragonfly

# --- Dragonfly systemd service ---
cat > /etc/systemd/system/dragonfly.service <<EOF
[Unit]
Description=Dragonfly - Redis-compatible in-memory store
After=network.target

[Service]
Type=simple
User=$LIGHTHOUSE_USER
ExecStart=/usr/local/bin/dragonfly --bind localhost --port $DRAGONFLY_PORT --dbfilename dump --dir /var/lib/dragonfly --maxmemory 256mb
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable dragonfly
systemctl start dragonfly
echo "Dragonfly running on localhost:$DRAGONFLY_PORT"

# --- Lighthouse directory ---
mkdir -p "$LIGHTHOUSE_DIR"
chown "$LIGHTHOUSE_USER:$LIGHTHOUSE_USER" "$LIGHTHOUSE_DIR"

# --- Lighthouse systemd service ---
# Build peer flags
PEER_FLAGS=""
if [ -n "$LIGHTHOUSE_PEERS" ]; then
    IFS=',' read -ra PEER_ARRAY <<< "$LIGHTHOUSE_PEERS"
    for peer in "${PEER_ARRAY[@]}"; do
        PEER_FLAGS="$PEER_FLAGS -peer $peer"
    done
fi

cat > /etc/systemd/system/lighthouse.service <<EOF
[Unit]
Description=Lighthouse - cloudroof.eu CDN control plane
After=network.target dragonfly.service
Requires=dragonfly.service

[Service]
Type=simple
User=$LIGHTHOUSE_USER
WorkingDirectory=$LIGHTHOUSE_DIR
ExecStart=$LIGHTHOUSE_DIR/lighthouse \\
    -addr $LIGHTHOUSE_ADDR \\
    -redis localhost:$DRAGONFLY_PORT \\
    -node-id $LIGHTHOUSE_NODE_ID \\
    -mesh-ip $LIGHTHOUSE_MESH_IP \\
    -dns-target $LIGHTHOUSE_DNS_TARGET $PEER_FLAGS
Restart=always
RestartSec=5
LimitNOFILE=65535

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/dragonfly $LIGHTHOUSE_DIR
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable lighthouse
echo "Lighthouse service installed (not started — deploy binary first)"

echo ""
echo "=== Next steps ==="
echo "1. Copy lighthouse binary to $LIGHTHOUSE_DIR/lighthouse"
echo "2. systemctl start lighthouse"
echo "3. Verify: curl -s http://localhost:8443/healthz"
echo "4. Register first org: curl -X POST http://localhost:8443/v1/orgs -d '{\"name\":\"myorg\"}'"
