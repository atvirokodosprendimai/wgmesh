# Use Cases

Real-world deployment scenarios showing when and why to choose wgmesh.

Each scenario is self-contained and takes 15–30 minutes to complete end-to-end.

| Scenario | Mode | Time to first value |
|----------|------|---------------------|
| [Remote Development Team VPN](remote-dev-team.md) | Decentralized | ~15 min |
| [Multi-Cloud Private Network](multi-cloud.md) | Decentralized | ~20 min |
| [Hybrid Office-to-Cloud Site-to-Site](hybrid-site-to-site.md) | Decentralized | ~25 min |
| [Managed Fleet (Centralized SSH)](managed-fleet.md) | Centralized | ~20 min |

## Choosing a scenario

- **Remote Development Team VPN** — developers on laptops / home networks who need to reach internal services without a hosted VPN server.
- **Multi-Cloud Private Network** — VMs or containers spread across AWS, GCP, and/or Hetzner that must communicate privately.
- **Hybrid Office-to-Cloud Site-to-Site** — on-premises LAN that needs a routed tunnel to a cloud VPC.
- **Managed Fleet (Centralized SSH)** — operator-controlled fleet where a central machine pushes WireGuard configs via SSH.

For a tool-selection comparison (wgmesh vs Tailscale, Netmaker, innernet) see [docs/evaluation-checklist.md](../evaluation-checklist.md).
