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

# ── Dragonfly (Redis-compatible cache) ──
# Install Dragonfly as a systemd service for persistent shared caching.
# Listens on 127.0.0.1:6379 only — not exposed externally.
if ! command -v dragonfly &>/dev/null; then
    echo "Installing Dragonfly..."
    ARCH=$(dpkg --print-architecture)
    # Dragonfly publishes releases for x86_64 and aarch64
    if [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; then
        DF_ARCH="aarch64"
    else
        DF_ARCH="x86_64"
    fi
    DF_VERSION=$(curl -sf https://api.github.com/repos/dragonflydb/dragonfly/releases/latest | jq -r '.tag_name')
    DF_URL="https://github.com/dragonflydb/dragonfly/releases/download/${DF_VERSION}/dragonfly-${DF_ARCH}.tar.gz"
    echo "Downloading Dragonfly ${DF_VERSION} for ${DF_ARCH}..."
    # The tarball contains "dragonfly-<arch>" binary, not "dragonfly".
    # Extract to /tmp then rename to get a consistent binary name.
    curl -sfL "$DF_URL" | tar xz -C /tmp
    mv "/tmp/dragonfly-${DF_ARCH}" /usr/local/bin/dragonfly
    chmod +x /usr/local/bin/dragonfly
fi

# Dragonfly data directory
mkdir -p /var/lib/dragonfly
chown "$CHIMNEY_USER:$CHIMNEY_USER" /var/lib/dragonfly 2>/dev/null || true

# Dragonfly systemd service
cat > /etc/systemd/system/dragonfly.service <<EOF
[Unit]
Description=Dragonfly — Redis-compatible cache for chimney
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/dragonfly --bind 127.0.0.1 --port 6379 --dbdir /var/lib/dragonfly --maxmemory 128mb --proactor_threads 1
Restart=always
RestartSec=3

# Hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/dragonfly
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable dragonfly
systemctl restart dragonfly
echo "Dragonfly started on 127.0.0.1:6379"

# ── Create service user ──
if ! id "$CHIMNEY_USER" &>/dev/null; then
    useradd --system --home-dir "$CHIMNEY_DIR" --shell /usr/sbin/nologin "$CHIMNEY_USER"
fi
mkdir -p "$CHIMNEY_DIR"
chown "$CHIMNEY_USER:$CHIMNEY_USER" "$CHIMNEY_DIR"

# ── Deploy chimney binary ──
# The binary is expected at /tmp/chimney, placed there by the CI workflow via scp.
# We do NOT support downloading from arbitrary URLs for security reasons.
# Stop the running service first to avoid "Text file busy" on the binary.
systemctl stop chimney 2>/dev/null || true

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
Description=chimney origin server (beerpub.dev dashboard)
After=network.target dragonfly.service
Requires=dragonfly.service

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
