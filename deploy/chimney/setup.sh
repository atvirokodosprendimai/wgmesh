#!/usr/bin/env bash
# setup.sh — Bootstrap a chimney edge/origin server on Ubuntu 24.04
#
# Usage: GITHUB_TOKEN="..." ROLE=origin|edge bash setup.sh
#
# The binary should be pre-deployed to /tmp/chimney by the CI workflow.
# This script is idempotent — safe to re-run.
set -euo pipefail

ROLE="${ROLE:-origin}"
CHIMNEY_USER="chimney"
CHIMNEY_DIR="/opt/chimney"

echo "=== chimney setup (role=$ROLE) ==="

# ── Validate inputs ──
if [ -z "${GITHUB_TOKEN:-}" ]; then
    echo "WARNING: GITHUB_TOKEN not set — chimney will use unauthenticated GitHub API (60 req/hr)"
fi

# ── System packages ──
apt-get update -qq
apt-get install -y -qq caddy curl jq

# ── Create service user ──
if ! id "$CHIMNEY_USER" &>/dev/null; then
    useradd --system --home-dir "$CHIMNEY_DIR" --shell /usr/sbin/nologin "$CHIMNEY_USER"
fi
mkdir -p "$CHIMNEY_DIR"
chown "$CHIMNEY_USER:$CHIMNEY_USER" "$CHIMNEY_DIR"

# ── Deploy chimney binary ──
# The binary is expected at /tmp/chimney, placed there by the CI workflow via scp.
# We do NOT support downloading from arbitrary URLs for security reasons.
if [ -f /tmp/chimney ]; then
    cp /tmp/chimney "$CHIMNEY_DIR/chimney"
    chmod +x "$CHIMNEY_DIR/chimney"
else
    if [ -f "$CHIMNEY_DIR/chimney" ]; then
        echo "Using existing chimney binary (no new binary in /tmp)"
    else
        echo "ERROR: No chimney binary found at /tmp/chimney or $CHIMNEY_DIR/chimney" >&2
        exit 1
    fi
fi

# ── Deploy dashboard files ──
mkdir -p "$CHIMNEY_DIR/docs"
if [ -d /tmp/docs ]; then
    cp -r /tmp/docs/* "$CHIMNEY_DIR/docs/"
fi

# ── Chimney systemd service ──
# Note: chimney logs to stdout/stderr, captured by systemd journald.
# No file-based logging — ProtectSystem=strict is safe without extra ReadWritePaths.
cat > /etc/systemd/system/chimney.service <<EOF
[Unit]
Description=chimney origin server (cloudroof.eu dashboard)
After=network.target

[Service]
Type=simple
User=$CHIMNEY_USER
WorkingDirectory=$CHIMNEY_DIR
ExecStart=$CHIMNEY_DIR/chimney -addr :8080 -docs $CHIMNEY_DIR/docs
Restart=always
RestartSec=5
Environment=GITHUB_TOKEN=${GITHUB_TOKEN:-}

# Hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$CHIMNEY_DIR
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable chimney
systemctl restart chimney

# ── Caddy config ──
if [ -f /tmp/Caddyfile ]; then
    cp /tmp/Caddyfile /etc/caddy/Caddyfile
    systemctl enable caddy
    systemctl restart caddy
fi

echo "=== chimney setup complete (role=$ROLE) ==="
systemctl status chimney --no-pager || true
