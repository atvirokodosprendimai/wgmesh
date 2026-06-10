# Pilot Troubleshooting Guide

Common issues encountered during the wgmesh 30-day pilot evaluation and their resolutions.

## Week 1 Issues

### Peers don't discover each other

**Symptoms**: `wgmesh peers list` shows no peers, or fewer than expected.

**Causes and fixes**:

1. **Different secrets** — Verify all nodes use the exact same secret URI:
   ```bash
   # On each node, verify the network ID matches
   wgmesh status --secret "<URI>"
   # Compare the Network ID line — it must be identical
   ```

2. **Firewall blocking DHT** — wgmesh uses UDP for DHT discovery:
   ```bash
   # Check if outgoing UDP is allowed
   # The DHT uses standard BitTorrent ports
   sudo tcpdump -i any udp port 6881 -c 10
   ```

3. **LAN discovery disabled** — If nodes are on the same network:
   ```bash
   # Don't use --no-lan-discovery if nodes are local
   wgmesh join --secret "<URI>"  # without --no-lan-discovery
   ```

4. **Bootstrap delay** — DHT discovery can take 30–60 seconds on first join.
   Wait and check again.

### WireGuard interface not created

**Symptoms**: `wg show` returns nothing or "does not exist".

**Causes and fixes**:

1. **Missing WireGuard tools**:
   ```bash
   # Linux
   sudo apt install wireguard-tools  # Debian/Ubuntu
   sudo dnf install wireguard-tools  # Fedora

   # macOS
   brew install wireguard-go
   ```

2. **Kernel module not loaded** (Linux):
   ```bash
   sudo modprobe wireguard
   ```

3. **Permission denied**: wgmesh requires root for interface management:
   ```bash
   sudo wgmesh join --secret "<URI>"
   ```

### Mesh IP not assigned

**Symptoms**: `wg show` shows interface but no mesh IP address.

**Fix**: Check daemon logs for IP derivation errors:
```bash
journalctl -u wgmesh --no-pager -n 50
```

## Week 2 Issues

### NAT traversal fails

**Symptoms**: Nodes behind NAT can't reach peers, or only one-direction traffic works.

**Fixes**:

1. **Enable persistent keepalive** — This is handled automatically by wgmesh.
   Verify it's set:
   ```bash
   wg show wg0 | grep "persistent keepalive"
   ```

2. **Try rendezvous introducer** — If one node has a public IP:
   ```bash
   # On the public node
   wgmesh join --secret "<URI>" --introducer
   ```

3. **Force relay mode** — As a fallback:
   ```bash
   wgmesh join --secret "<URI>" --force-relay
   ```

4. **Check NAT type** — Symmetric NAT is hardest to traverse:
   ```bash
   # Look in logs for NAT type detection results
   journalctl -u wgmesh | grep "NAT"
   ```

### Node can't be removed cleanly

**Symptoms**: Stale peer entries remain after removing a node.

**Fix**: Peers are automatically cleaned up after a timeout (10 minutes).
To force immediate cleanup:
```bash
# On the remaining nodes, restart the daemon
sudo systemctl restart wgmesh
```

## Week 3 Issues

### Daemon doesn't recover after restart

**Symptoms**: After `systemctl restart wgmesh`, peers don't reconnect.

**Fixes**:

1. **Check peer cache** — The daemon restores peers from cache:
   ```bash
   ls -la /var/lib/wgmesh/wg0-peers.json
   ```

2. **Verify DHT reconnection**:
   ```bash
   journalctl -u wgmesh --since "5 minutes ago" | grep -i "dht\|discovery"
   ```

3. **Manual cache clear** (last resort):
   ```bash
   sudo rm /var/lib/wgmesh/wg0-peers.json
   sudo systemctl restart wgmesh
   ```

### Secret rotation breaks connectivity

**Symptoms**: After rotating the secret, some nodes can't connect.

**Fixes**:

1. **Verify grace period** — During the grace period, both old and new secrets should work:
   ```bash
   wgmesh rotate-secret --current "<OLD>" --grace 24h
   ```

2. **Update nodes gradually** — Update one node at a time, verifying connectivity after each.

3. **Check key derivation** — Verify the new secret produces different network IDs:
   ```bash
   wgmesh status --secret "<NEW_URI>"
   ```

## Week 4 Issues

### Systemd service won't start on boot

**Symptoms**: `systemctl status wgmesh` shows failed after reboot.

**Fixes**:

1. **Verify service is enabled**:
   ```bash
   sudo systemctl enable wgmesh
   sudo systemctl is-enabled wgmesh
   ```

2. **Check journal for errors**:
   ```bash
   journalctl -u wgmesh -b  # Current boot logs
   ```

3. **Network not ready** — Add a dependency on network-online:
   ```bash
   # The service file should include
   # After=network-online.target
   # Wants=network-online.target
   ```

### Prometheus metrics not available

**Symptoms**: `curl localhost:9090/metrics` fails or returns empty.

**Fixes**:

1. **Verify metrics flag** — Must be specified in the join or install-service command:
   ```bash
   wgmesh install-service --secret "<URI>" --metrics :9090
   ```

2. **Check firewall** — Metrics port may be blocked:
   ```bash
   sudo ss -tlnp | grep 9090
   ```

3. **Re-register metrics** — Restart the daemon if metrics were enabled after startup.

## General Issues

### `wgmesh pilot validate` shows failures

The validate command checks mesh health via the RPC socket. Common failures:

| Check | Failure | Fix |
|---|---|---|
| Interface exists | Daemon not running | `systemctl start wgmesh` |
| Peers connected | No peers discovered | Check secrets match, wait for DHT |
| Daemon responding | RPC socket missing | Verify daemon is running |
| Clock skew | System clock wrong | Enable NTP: `sudo timedatectl set-ntp true` |

### `wgmesh pilot` commands fail with "no pilot state"

**Fix**: Initialize pilot tracking first:
```bash
wgmesh pilot init --mode decentralized --use-case general --nodes 3
```

### Pilot state file permissions

The pilot state file is stored at `~/.wgmesh/pilot.json` with `0600` permissions.
If you see permission errors:
```bash
mkdir -p ~/.wgmesh
chmod 700 ~/.wgmesh
```

## Getting Help

- **Logs**: `journalctl -u wgmesh -f` (systemd) or check terminal output
- **Health check**: `wgmesh pilot validate`
- **Status**: `wgmesh pilot status`
- **Report**: `wgmesh pilot report --format markdown`
- **GitHub Issues**: https://github.com/atvirokodosprendimai/wgmesh/issues
