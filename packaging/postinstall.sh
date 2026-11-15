#!/bin/sh
set -e
mkdir -p /etc/wgmesh
mkdir -p /var/lib/wgmesh
if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
    systemctl daemon-reload
    # Restart only if already running, so an upgrade picks up the new binary.
    # Never enables or starts a stopped service.
    systemctl try-restart wgmesh.service || true
fi
