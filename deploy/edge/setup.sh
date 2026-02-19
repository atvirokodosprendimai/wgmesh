#!/usr/bin/env bash
# Edge node setup for cloudroof.eu CDN.
# Installs Caddy (TLS termination) and configures it to pull config from lighthouse.
#
# Usage: LIGHTHOUSE_URL=https://api.cloudroof.eu EDGE_NAME=edge-nbg1 ./setup.sh
# Prerequisites: WireGuard mesh already running on this node.

set -euo pipefail

# --- Configuration ---
EDGE_USER="${EDGE_USER:-caddy}"
LIGHTHOUSE_URL="${LIGHTHOUSE_URL:?Must set LIGHTHOUSE_URL}"
EDGE_NAME="${EDGE_NAME:?Must set EDGE_NAME}"
CADDY_CONFIG_DIR="/etc/caddy"
CADDY_DATA_DIR="/var/lib/caddy"
CONFIG_POLL_INTERVAL="${CONFIG_POLL_INTERVAL:-30}" # seconds

echo "=== Edge Node Setup ==="
echo "Name:       $EDGE_NAME"
echo "Lighthouse: $LIGHTHOUSE_URL"

# --- Install Caddy ---
if ! command -v caddy &>/dev/null; then
    echo "Installing Caddy..."
    apt-get update -qq
    apt-get install -y -qq debian-keyring debian-archive-keyring apt-transport-https curl
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
    apt-get update -qq
    apt-get install -y -qq caddy
    echo "Caddy installed: $(caddy version)"
fi

# --- Caddy directories ---
mkdir -p "$CADDY_CONFIG_DIR" "$CADDY_DATA_DIR"
chown "$EDGE_USER:$EDGE_USER" "$CADDY_DATA_DIR"

# --- Config puller script ---
# Pulls Caddyfile from lighthouse and reloads Caddy when config changes.
cat > /usr/local/bin/edge-config-pull <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail

LIGHTHOUSE_URL="${LIGHTHOUSE_URL:?}"
CADDY_CONFIG="/etc/caddy/Caddyfile"
CADDY_CONFIG_TMP="/etc/caddy/Caddyfile.new"

# Pull config from lighthouse
HTTP_CODE=$(curl -s -o "$CADDY_CONFIG_TMP" -w "%{http_code}" \
    "${LIGHTHOUSE_URL}/v1/xds/caddyfile" 2>/dev/null || echo "000")

if [ "$HTTP_CODE" != "200" ]; then
    echo "$(date -Is) Config pull failed: HTTP $HTTP_CODE"
    rm -f "$CADDY_CONFIG_TMP"
    exit 0 # Non-fatal — keep running with existing config
fi

# Check if config changed
if [ -f "$CADDY_CONFIG" ] && diff -q "$CADDY_CONFIG" "$CADDY_CONFIG_TMP" &>/dev/null; then
    rm -f "$CADDY_CONFIG_TMP"
    exit 0 # No changes
fi

# Validate new config
if ! caddy validate --config "$CADDY_CONFIG_TMP" --adapter caddyfile 2>/dev/null; then
    echo "$(date -Is) Invalid config from lighthouse, keeping current"
    rm -f "$CADDY_CONFIG_TMP"
    exit 0
fi

# Apply
mv "$CADDY_CONFIG_TMP" "$CADDY_CONFIG"
caddy reload --config "$CADDY_CONFIG" --adapter caddyfile 2>/dev/null || true
echo "$(date -Is) Config updated and reloaded"
SCRIPT
chmod +x /usr/local/bin/edge-config-pull

# --- Initial Caddyfile (minimal — will be replaced by lighthouse config) ---
cat > "$CADDY_CONFIG_DIR/Caddyfile" <<EOF
# Initial edge config — will be replaced by lighthouse-pulled config.
# This serves as a fallback until the first config pull succeeds.

:80 {
    respond "cloudroof edge: $EDGE_NAME" 200
}
EOF

# --- Config puller systemd timer ---
cat > /etc/systemd/system/edge-config-pull.service <<EOF
[Unit]
Description=Pull CDN config from lighthouse
After=network.target caddy.service

[Service]
Type=oneshot
Environment=LIGHTHOUSE_URL=$LIGHTHOUSE_URL
ExecStart=/usr/local/bin/edge-config-pull
EOF

cat > /etc/systemd/system/edge-config-pull.timer <<EOF
[Unit]
Description=Periodically pull CDN config from lighthouse

[Timer]
OnBootSec=10s
OnUnitActiveSec=${CONFIG_POLL_INTERVAL}s
AccuracySec=1s

[Install]
WantedBy=timers.target
EOF

systemctl daemon-reload
systemctl enable caddy
systemctl start caddy
systemctl enable edge-config-pull.timer
systemctl start edge-config-pull.timer

echo ""
echo "=== Edge node ready ==="
echo "Caddy:       running (port 80/443)"
echo "Config pull: every ${CONFIG_POLL_INTERVAL}s from $LIGHTHOUSE_URL"
echo ""
echo "Verify: curl -s http://localhost/"
echo "Logs:   journalctl -u caddy -f"
echo "Pull:   journalctl -u edge-config-pull -f"
