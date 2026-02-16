# wgmesh Test Lab

Testing mesh connectivity and NAT traversal.

## Quick Start (if VirtualBox works)

```bash
# Start all VMs
./lab.sh up

# Build and deploy binary
./lab.sh build

# Run connectivity tests
./lab.sh test
```

## Manual Testing (recommended for macOS)

VirtualBox on macOS often has kernel extension issues. For manual testing:

### Option 1: Production Mesh

Use the real mesh with lempa:

```bash
# On your machine
sudo ./wgmesh join --secret "wgmesh://v1/..." -interface utun10

# Have lempa run the same binary
# Check connectivity:
ping 10.248.95.75  # lempa's mesh IP
```

### Option 2: Cloud VMs (Hetzner/DigitalOcean)

Deploy 3-4 cheap VMs with different network positions:

```
terraform/
├── main.tf       # 4 VMs: introducer + 3 nodes
├── variables.tf
└── test.sh
```

Cost: ~€15/mo for 4x CX22 (2 vCPU, 4GB RAM each)

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

## Test Cases

| From | To | Expected | Notes |
|------|-----|----------|-------|
| node-a | introducer | PASS | Direct connection |
| node-a | node-b | PASS | Via introducer rendezvous |
| node-a | node-c | PASS | Via introducer (symmetric NAT) |
| node-b | node-c | PASS | Via introducer rendezvous |

## Files

- `Vagrantfile` - 4-VM topology definition
- `lab.sh` - Helper for VM management
- `test-mesh.sh` - Automated connectivity tests
