#!/bin/sh
set -e
mkdir -p /etc/wgmesh
mkdir -p /var/lib/wgmesh
if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
    systemctl daemon-reload || true
fi
