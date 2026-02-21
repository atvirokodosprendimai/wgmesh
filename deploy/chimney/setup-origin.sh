#!/usr/bin/env bash
# setup-origin.sh — Bootstrap a chimney origin server (nbg1, fsn1).
#
# Usage:
#   GITHUB_TOKEN="..." WGMESH_SECRET="..." bash setup-origin.sh
#
# Idempotent — safe to re-run. On first run installs Docker and starts
# the full stack. On subsequent runs just updates config files.
#
# After initial bootstrap, deployments run via bluegreen.sh.
set -euo pipefail

DEPLOY_DIR="/opt/chimney"

echo "=== chimney origin setup ==="

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

# ── Deploy directory ──
mkdir -p "$DEPLOY_DIR"

# ── Copy compose files from /tmp ──
cp /tmp/compose.origin.yml     "$DEPLOY_DIR/compose.origin.yml"
cp /tmp/Caddyfile.origin       "$DEPLOY_DIR/Caddyfile.origin"
cp /tmp/bluegreen.sh           "$DEPLOY_DIR/bluegreen.sh"
chmod +x "$DEPLOY_DIR/bluegreen.sh"

# ── Write .env ──
cat > "$DEPLOY_DIR/.env" <<EOF
GITHUB_TOKEN=${GITHUB_TOKEN:-}
WGMESH_SECRET=${WGMESH_SECRET:-}
EOF
chmod 600 "$DEPLOY_DIR/.env"

# ── Initialize upstream.conf (point to blue on first boot) ──
if [ ! -f "$DEPLOY_DIR/upstream.conf" ]; then
    echo "reverse_proxy localhost:8081" > "$DEPLOY_DIR/upstream.conf"
fi

# ── Initialize active-slot ──
if [ ! -f "$DEPLOY_DIR/active-slot" ]; then
    echo "blue" > "$DEPLOY_DIR/active-slot"
fi

# ── Clean up stale dragonfly volume on fresh deploy ──
echo "Stopping any existing stack..."
docker compose \
    -f "$DEPLOY_DIR/compose.origin.yml" \
    --project-directory "$DEPLOY_DIR" \
    down --remove-orphans 2>/dev/null || true
docker volume rm chimney_dragonfly_data 2>/dev/null || true

# ── Pull images ──
echo "Pulling images..."
docker compose \
    -f "$DEPLOY_DIR/compose.origin.yml" \
    --project-directory "$DEPLOY_DIR" \
    --env-file "$DEPLOY_DIR/.env" \
    pull --ignore-pull-failures 2>/dev/null || true

# ── Start full stack ──
echo "Starting chimney origin stack..."
docker compose \
    -f "$DEPLOY_DIR/compose.origin.yml" \
    --project-directory "$DEPLOY_DIR" \
    --env-file "$DEPLOY_DIR/.env" \
    up -d 2>&1 || {
    echo "ERROR: docker compose up failed — diagnostics:"
    sleep 5
    docker compose -f "$DEPLOY_DIR/compose.origin.yml" --project-directory "$DEPLOY_DIR" logs --tail=40 2>&1 || true
    exit 1
}

# ── Wait for chimney-blue to be healthy ──
echo "Waiting for chimney-blue healthy (up to 60s)..."
for i in $(seq 1 20); do
    if curl -sf "http://localhost:8081/healthz" >/dev/null 2>&1; then
        echo "chimney-blue healthy after $((i * 3))s"
        break
    fi
    if [ "$i" = "20" ]; then
        echo "WARNING: chimney-blue not healthy after 60s"
        docker compose -f "$DEPLOY_DIR/compose.origin.yml" --project-directory "$DEPLOY_DIR" logs --tail=30
    fi
    sleep 3
done

echo "=== origin setup complete ==="
docker compose -f "$DEPLOY_DIR/compose.origin.yml" --project-directory "$DEPLOY_DIR" ps
