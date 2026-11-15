#!/bin/sh
set -e
# Stop and disable only on real removal. dpkg calls prerm with "upgrade" on
# every upgrade or reinstall, and rpm calls %preun with $1=1 on upgrade; in
# those cases the service must keep running (postinstall try-restarts it).
case "$1" in
    remove|purge|0)
        systemctl stop wgmesh.service 2>/dev/null || true
        systemctl disable wgmesh.service 2>/dev/null || true
        ;;
esac
