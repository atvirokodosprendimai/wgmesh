#!/usr/bin/env bash
# bluegreen.sh — Zero-downtime blue/green deploy for chimney origin servers.
#
# Usage:
#   IMAGE=ghcr.io/atvirokodosprendimai/wgmesh/chimney:sha-abc1234 bash bluegreen.sh
#
# What it does:
#   1. Detects which slot (blue/green) is currently active
#   2. Pulls the new image into the inactive slot
#   3. Starts the inactive slot and waits for /healthz to return 200
#   4. Atomically writes upstream.conf pointing to the new slot
#   5. Reloads Caddy (zero dropped connections)
#   6. Stops the old slot (graceful shutdown, 30s drain)
#   7. Updates /opt/chimney/active-slot so next deploy knows the new active
#
# Prerequisites (on host):
#   - Docker Compose stack running at /opt/chimney
#   - Caddy admin API on localhost:2019
#   - /opt/chimney/active-slot file: "blue" or "green"
set -euo pipefail

DEPLOY_DIR="/opt/chimney"
ACTIVE_SLOT_FILE="$DEPLOY_DIR/active-slot"
IMAGE="${IMAGE:-ghcr.io/atvirokodosprendimai/wgmesh/chimney:latest}"
CADDY_ADMIN="http://localhost:2019"
HEALTH_TIMEOUT=60  # seconds to wait for new slot to be healthy

# ── Determine current active and inactive slots ──
if [ -f "$ACTIVE_SLOT_FILE" ]; then
    ACTIVE=$(cat "$ACTIVE_SLOT_FILE" | tr -d '[:space:]')
else
    ACTIVE="blue"
fi

case "$ACTIVE" in
    blue)  INACTIVE="green"; INACTIVE_PORT=8082 ;;
    green) INACTIVE="blue";  INACTIVE_PORT=8081 ;;
    *)
        echo "ERROR: unknown active slot '$ACTIVE' in $ACTIVE_SLOT_FILE"
        exit 1
        ;;
esac

echo "=== Blue/Green Deploy ==="
echo "Active slot:   $ACTIVE"
echo "Deploying to:  $INACTIVE (port $INACTIVE_PORT)"
echo "New image:     $IMAGE"

# ── Pull new image ──
echo ""
echo "Pulling new image..."
docker pull "$IMAGE"

# ── Update the inactive slot's image in compose override ──
# Write a compose override that pins the inactive slot to the new image
cat > "$DEPLOY_DIR/docker-compose.override.yml" <<EOF
services:
  chimney-${INACTIVE}:
    image: ${IMAGE}
EOF

# ── Restart the inactive slot with new image ──
echo ""
echo "Starting inactive slot: chimney-${INACTIVE}..."
docker compose \
    -f "$DEPLOY_DIR/compose.origin.yml" \
    -f "$DEPLOY_DIR/docker-compose.override.yml" \
    --project-directory "$DEPLOY_DIR" \
    --env-file "$DEPLOY_DIR/.env" \
    up -d --no-deps --force-recreate "chimney-${INACTIVE}"

# ── Wait for new slot to be healthy ──
echo ""
echo "Waiting for chimney-${INACTIVE} to be healthy (up to ${HEALTH_TIMEOUT}s)..."
DEADLINE=$((SECONDS + HEALTH_TIMEOUT))
while [ $SECONDS -lt $DEADLINE ]; do
    STATUS=$(docker inspect --format='{{.State.Health.Status}}' "chimney-chimney-${INACTIVE}-1" 2>/dev/null || echo "unknown")
    if [ "$STATUS" = "healthy" ]; then
        echo "chimney-${INACTIVE} is healthy"
        break
    fi
    # Also try direct HTTP check on the port
    if curl -sf "http://localhost:${INACTIVE_PORT}/healthz" >/dev/null 2>&1; then
        echo "chimney-${INACTIVE} responding on port ${INACTIVE_PORT}"
        break
    fi
    echo "  waiting... (status=$STATUS)"
    sleep 3
done

# Final verification
if ! curl -sf "http://localhost:${INACTIVE_PORT}/healthz" >/dev/null 2>&1; then
    echo "ERROR: chimney-${INACTIVE} failed health check after ${HEALTH_TIMEOUT}s — aborting"
    echo "Logs from failing slot:"
    docker logs "chimney-chimney-${INACTIVE}-1" --tail=30 2>&1 || true
    # Roll back override
    rm -f "$DEPLOY_DIR/docker-compose.override.yml"
    exit 1
fi

# ── Switch Caddy upstream ──
echo ""
echo "Switching Caddy upstream → ${INACTIVE} (port ${INACTIVE_PORT})..."
echo "reverse_proxy localhost:${INACTIVE_PORT}" > "$DEPLOY_DIR/upstream.conf"

# Reload Caddy config atomically via admin API
if curl -sf -X POST "${CADDY_ADMIN}/load" \
    -H "Content-Type: text/caddyfile" \
    --data-binary @"$DEPLOY_DIR/Caddyfile.origin" >/dev/null 2>&1; then
    echo "Caddy reloaded via admin API"
else
    echo "WARNING: Caddy admin reload failed — attempting caddy reload via docker exec..."
    docker exec chimney-caddy-1 caddy reload --config /etc/caddy/Caddyfile 2>&1 || true
fi

# Give Caddy a moment to drain connections from old slot
sleep 5

# ── Stop old active slot ──
echo ""
echo "Stopping old slot: chimney-${ACTIVE}..."
docker compose \
    -f "$DEPLOY_DIR/compose.origin.yml" \
    --project-directory "$DEPLOY_DIR" \
    --env-file "$DEPLOY_DIR/.env" \
    stop --timeout 30 "chimney-${ACTIVE}" || true

# ── Record new active slot ──
echo "$INACTIVE" > "$ACTIVE_SLOT_FILE"

# ── Clean up override ──
rm -f "$DEPLOY_DIR/docker-compose.override.yml"

echo ""
echo "=== Blue/Green Deploy Complete ==="
echo "Active slot: $INACTIVE (port $INACTIVE_PORT)"
echo "Image: $IMAGE"

# Final smoke check
RESULT=$(curl -sf "http://localhost/healthz" 2>/dev/null || echo "FAIL")
echo "Caddy→${INACTIVE} health: $RESULT"
