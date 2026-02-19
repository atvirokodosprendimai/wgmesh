#!/usr/bin/env bash
# setup.sh — Bootstrap a chimney edge/origin server on Ubuntu 24.04
#
# Usage: SSH_KEY="..." GITHUB_TOKEN="..." ROLE=origin|edge bash setup.sh
#
# This script is idempotent — safe to re-run.
set -euo pipefail

ROLE="${ROLE:-origin}"
CHIMNEY_USER="chimney"
CHIMNEY_DIR="/opt/chimney"
BINARY_URL="${BINARY_URL:-}"  # Set by CI workflow

echo "=== chimney setup (role=$ROLE) ==="

# ── System packages ──
apt-get update -qq
apt-get install -y -qq caddy curl jq

# ── Create service user ──
if ! id "$CHIMNEY_USER" &>/dev/null; then
    useradd --system --home-dir "$CHIMNEY_DIR" --shell /usr/sbin/nologin "$CHIMNEY_USER"
fi
mkdir -p "$CHIMNEY_DIR" /var/log/caddy
chown "$CHIMNEY_USER:$CHIMNEY_USER" "$CHIMNEY_DIR"

# ── Deploy chimney binary ──
if [ -n "$BINARY_URL" ]; then
    echo "Downloading chimney binary..."
    curl -fsSL "$BINARY_URL" -o "$CHIMNEY_DIR/chimney"
    chmod +x "$CHIMNEY_DIR/chimney"
elif [ -f /tmp/chimney ]; then
    cp /tmp/chimney "$CHIMNEY_DIR/chimney"
    chmod +x "$CHIMNEY_DIR/chimney"
fi

# ── Deploy dashboard files ──
mkdir -p "$CHIMNEY_DIR/docs"
if [ -d /tmp/docs ]; then
    cp -r /tmp/docs/* "$CHIMNEY_DIR/docs/"
fi

# ── Chimney systemd service ──
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
