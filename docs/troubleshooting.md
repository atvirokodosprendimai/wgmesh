# Troubleshooting

## Connection Issues

```bash
# Test SSH connectivity
ssh root@node-address

# Check WireGuard status on a node
ssh root@node-address wg show
```

## Check Persistence

```bash
# Check if systemd service is enabled and running
ssh root@node-address systemctl status wg-quick@wg0

# View the persistent configuration file
ssh root@node-address cat /etc/wireguard/wg0.conf

# Check if service starts on boot
ssh root@node-address systemctl is-enabled wg-quick@wg0
```

## Check Interface and Routes

```bash
# Check interface status
ssh root@node-address ip addr show wg0

# Check routing table
ssh root@node-address ip route

# Test connectivity through mesh
ssh root@node-address ping -c 3 10.99.0.2
```

## View WireGuard Logs

```bash
# View systemd service logs
ssh root@node-address journalctl -u wg-quick@wg0 -n 50

# Follow logs in real-time
ssh root@node-address journalctl -u wg-quick@wg0 -f
```

## Test Reboot Persistence

```bash
# Reboot a node
ssh root@node-address reboot

# Wait for reboot, then check if WireGuard came back up
sleep 30
ssh root@node-address wg show
ssh root@node-address ip route | grep 192.168
```

## Rebuild Configuration

If something goes wrong, you can force a fresh configuration:
```bash
# On each node, stop and disable the service
ssh root@node-address systemctl stop wg-quick@wg0
ssh root@node-address systemctl disable wg-quick@wg0

# Then redeploy
wgmesh -deploy
```
