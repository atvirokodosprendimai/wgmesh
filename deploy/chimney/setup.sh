#!/usr/bin/env bash
# setup.sh — Bootstrap a chimney server on Ubuntu 24.04 using Docker Compose.
#
# Usage: GITHUB_TOKEN="..." WGMESH_SECRET="..." ROLE=origin|edge bash setup.sh
#
# Files expected in /tmp (placed by CI workflow):
#   /tmp/chimney     — compiled chimney binary (linux/ARCH)
#   /tmp/docs/       — dashboard static files
#   /tmp/compose.yml — Docker Compose stack definition
#   /tmp/Caddyfile   — Caddy reverse proxy config
#   /tmp/Dockerfile  — Dockerfile for chimney container
#
# This script is idempotent — safe to re-run.
# Cattle, not pets: on failure, reprovision the server rather than debugging state.
set -euo pipefail

ROLE="${ROLE:-origin}"
DEPLOY_DIR="/opt/chimney"

echo "=== chimney setup (role=$ROLE) ==="

# ── Docker ──
if ! command -v docker &>/dev/null; then
    echo "Installing Docker..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable docker
    systemctl start docker
    echo "Docker installed: $(docker --version)"
else
    echo "Docker already installed: $(docker --version)"
fi

# ── Deploy directory ──
mkdir -p "$DEPLOY_DIR/docs"

# ── Copy compose files ──
cp /tmp/compose.yml  "$DEPLOY_DIR/compose.yml"
cp /tmp/Caddyfile    "$DEPLOY_DIR/Caddyfile"
cp /tmp/Dockerfile   "$DEPLOY_DIR/Dockerfile"

if [ -d /tmp/docs ]; then
    cp -r /tmp/docs/. "$DEPLOY_DIR/docs/"
fi

# ── Copy chimney binary into build context ──
if [ -f /tmp/chimney ]; then
    cp /tmp/chimney "$DEPLOY_DIR/chimney"
    chmod +x "$DEPLOY_DIR/chimney"
fi

# ── Write .env (secrets — never logged) ──
cat > "$DEPLOY_DIR/.env" <<EOF
GITHUB_TOKEN=${GITHUB_TOKEN:-}
WGMESH_SECRET=${WGMESH_SECRET:-}
EOF
chmod 600 "$DEPLOY_DIR/.env"

# ── Build chimney image and start stack ──
echo "Building chimney image..."
docker compose -f "$DEPLOY_DIR/compose.yml" --project-directory "$DEPLOY_DIR" build chimney

echo "Starting stack..."
if ! docker compose -f "$DEPLOY_DIR/compose.yml" --project-directory "$DEPLOY_DIR" \
    --env-file "$DEPLOY_DIR/.env" up -d 2>&1; then
    echo "ERROR: docker compose up failed — dumping dragonfly logs:"
    docker logs chimney-dragonfly-1 2>&1 | tail -50 || true
    docker inspect chimney-dragonfly-1 2>&1 | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)[0]
    state = d.get('State', {})
    print('Container state:', state.get('Status'))
    print('Exit code:', state.get('ExitCode'))
    print('Error:', state.get('Error'))
    print('OOMKilled:', state.get('OOMKilled'))
except Exception as e:
    print('Could not parse:', e)
" || true
    exit 1
fi

# ── Wait for chimney to be healthy ──
echo "Waiting for chimney to be healthy (up to 60s)..."
for i in $(seq 1 20); do
    STATUS=$(docker inspect --format='{{.State.Health.Status}}' chimney-chimney-1 2>/dev/null || echo "not_found")
    if [ "$STATUS" = "healthy" ]; then
        echo "chimney healthy after $((i * 3))s"
        break
    fi
    if [ "$i" = "20" ]; then
        echo "WARNING: chimney not healthy after 60s (status=$STATUS)"
        docker compose -f "$DEPLOY_DIR/compose.yml" --project-directory "$DEPLOY_DIR" logs --tail=30
    fi
    sleep 3
done

echo "=== chimney setup complete (role=$ROLE) ==="
docker compose -f "$DEPLOY_DIR/compose.yml" --project-directory "$DEPLOY_DIR" ps
