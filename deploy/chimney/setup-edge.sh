#!/usr/bin/env bash
# setup-edge.sh — Bootstrap a chimney edge server (hel1, ash).
#
# Usage:
#   WGMESH_SECRET="..." ORIGIN_WG_IP_1="10.x.x.x" ORIGIN_WG_IP_2="10.x.x.x" bash setup-edge.sh
#
# Edge nodes are stateless — watchtower auto-updates caddy+wgmesh on new image.
# No blue/green needed: Caddy restarts in <1s with zero dropped connections.
set -euo pipefail

DEPLOY_DIR="/opt/chimney-edge"

echo "=== chimney edge setup ==="

# ── Docker ──
if ! command -v docker &>/dev/null; then
    echo "Installing Docker..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable docker
    systemctl start docker
    echo "Docker installed: $(docker --version)"
else
    echo "Docker: $(docker --version)"
fi

# ── WireGuard kernel module ──
if ! lsmod | grep -q wireguard; then
    apt-get install -y -qq wireguard-tools 2>/dev/null || true
    modprobe wireguard 2>/dev/null || true
fi

# ── Deploy directory ──
mkdir -p "$DEPLOY_DIR"

# ── Copy files ──
cp /tmp/compose.edge.yml    "$DEPLOY_DIR/compose.edge.yml"
cp /tmp/Caddyfile.edge      "$DEPLOY_DIR/Caddyfile.edge"

# ── Write .env (origin WG IPs + mesh secret) ──
cat > "$DEPLOY_DIR/.env" <<EOF
WGMESH_SECRET=${WGMESH_SECRET:-}
ORIGIN_WG_IP_1=${ORIGIN_WG_IP_1:-}
ORIGIN_WG_IP_2=${ORIGIN_WG_IP_2:-}
EOF
chmod 600 "$DEPLOY_DIR/.env"

# ── Stop any existing stack ──
echo "Stopping any existing edge stack..."
docker compose \
    -f "$DEPLOY_DIR/compose.edge.yml" \
    --project-directory "$DEPLOY_DIR" \
    down --remove-orphans 2>/dev/null || true

# ── Pull images ──
echo "Pulling edge images..."
docker compose \
    -f "$DEPLOY_DIR/compose.edge.yml" \
    --project-directory "$DEPLOY_DIR" \
    --env-file "$DEPLOY_DIR/.env" \
    pull --ignore-pull-failures 2>/dev/null || true

# ── Start stack ──
echo "Starting edge stack..."
docker compose \
    -f "$DEPLOY_DIR/compose.edge.yml" \
    --project-directory "$DEPLOY_DIR" \
    --env-file "$DEPLOY_DIR/.env" \
    up -d 2>&1 || {
    echo "ERROR: edge stack failed to start — diagnostics:"
    sleep 5
    docker compose -f "$DEPLOY_DIR/compose.edge.yml" --project-directory "$DEPLOY_DIR" logs --tail=30 2>&1 || true
    exit 1
}

# ── Wait for Caddy ──
echo "Waiting for Caddy to start (up to 30s)..."
for i in $(seq 1 10); do
    if curl -sf "http://localhost/healthz" >/dev/null 2>&1; then
        echo "Edge Caddy healthy after $((i * 3))s"
        break
    fi
    if [ "$i" = "10" ]; then
        echo "WARNING: Caddy not yet proxying (WG mesh may still be converging)"
    fi
    sleep 3
done

echo "=== edge setup complete ==="
docker compose -f "$DEPLOY_DIR/compose.edge.yml" --project-directory "$DEPLOY_DIR" ps
