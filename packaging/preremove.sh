#!/bin/sh
set -e
systemctl stop wgmesh.service 2>/dev/null || true
systemctl disable wgmesh.service 2>/dev/null || true
