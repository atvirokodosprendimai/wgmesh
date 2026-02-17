# wgmesh Test Lab

Testing mesh connectivity and NAT traversal.

## Status

| Method | Works | Issue |
|--------|-------|-------|
| VirtualBox | ❌ | macOS 26 + VirtualBox 7.2 kernel extension incompatibility |
| Lima shared/vzNAT | ❌ | Both VMs share same external IP, can't discover each other |
| Lima bridged | ⚠️ | Requires `sudo` network setup on host |
| **Production mesh** | ✅ | **Recommended** - use with lempa |
| **Cloud VMs** | ✅ | Hetzner/DigitalOcean with public IPs |

## Recommended: Production Mesh

Share binaries with lempa:

```bash
# Build for both platforms
GOOS=darwin GOARCH=arm64 go build -o wgmesh-darwin-arm64 .
GOOS=linux GOARCH=amd64 go build -o wgmesh-linux-amd64 .

# Send linux binary to lempa, then both run:
sudo ./wgmesh join --secret "wgmesh://v1/..." --interface wg0 [--introducer]
```

## Test Files

```
testlab/
├── Vagrantfile         # VirtualBox 4-VM topology (blocked on macOS 26)
├── lima-lab.sh         # Lima 2-VM quick test (shared NAT limitation)
├── lima-bridge-lab.sh  # Lima with vzNAT (same limitation)
├── lab.sh              # Vagrant helper
├── test-mesh.sh        # Full test suite
└── README.md           # This file
```

## Topology

```
                    ┌─────────────────┐
                    │   Introducer    │
                    │  10.248.0.1     │
                    │  --introducer   │
                    └────────┬────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
   ┌────▼────┐          ┌────▼────┐          ┌────▼────┐
   │  Node A │          │  Node B │          │  Node C │
   │10.248.0.10         │10.248.0.20         │10.248.0.30
   │  (NAT)  │          │  (NAT)  │          │(symmetric)
   └─────────┘          └─────────┘          └─────────┘
```

## Cloud VM Option (Hetzner)

```bash
# Create 4 CX22 VMs (€3.79/mo each = ~€15/mo total)
# One with --introducer, others regular nodes

# After setup:
hcloud server create --name introducer --type cx22 --image ubuntu-22.04
hcloud server create --name node-a --type cx22 --image ubuntu-22.04
# etc.
```

## Lima Limitations

Lima VMs share the host's external IP via NAT. For isolated networks:
1. Use `lima: bridged` network type
2. Run `sudo` network setup on host first
3. See: https://lima-vm.io/docs/config/network/
