# Team Dogfooding — wgmesh

This document tracks internal team usage of wgmesh ("dogfooding") to validate
real-world reliability before broader release. Every team member runs wgmesh on
their own machines and reports results here.

## Active Usage Patterns

### Decentralized Mesh Usage

The team operates a fully-decentralized WireGuard mesh — no single relay or
"hub" node. Every peer connects directly to every other peer via wgmesh's NAT
traversal. Each node maintains its own `mesh-state.json` and the mesh
re-converges automatically if any node drops offline.

**Mesh Nodes**

| Node | OS | Network | Joined |
|---|---|---|---|
| dev-laptop-1 | macOS 14 arm64 | Home network (NAT) | 2026-03-01 |
| dev-laptop-2 | Ubuntu 24.04 | Home network (NAT) | 2026-03-01 |
| build-runner-1 | Ubuntu 24.04 | Hetzner cloud | 2026-03-01 |
| staging-vm-1 | Ubuntu 24.04 | Hetzner cloud | 2026-03-05 |
| staging-vm-2 | Ubuntu 24.04 | Hetzner cloud | 2026-03-05 |

### Centralized Mode

Some workflows temporarily use `build-runner-1` as a coordination point for
CI/CD pipelines (e.g., triggering deployments to staging VMs). This is **not**
a hub-and-spoke topology — all peers still maintain direct tunnels. The
centralized mode simply means `build-runner-1` is the node that initiates
 orchestrated jobs.

### Daily Workflows

Team members rely on the mesh every day for:

- **SSH without public IPs** — log in to any node from any other node, even
  behind NAT.
- **Database access from laptops** — connect to PostgreSQL/Redis running on
  staging VMs directly through mesh IPs.
- **Internal HTTP services** — reach dashboards, APIs, and build artifacts on
  private ports.
- **Cross-NAT connectivity** — laptop-to-laptop and laptop-to-cloud tunnels
  form automatically with no manual port-forwarding.

## Stability Metrics

| Metric | Value |
|---|---|
| Minimum continuous uptime | 28+ days |
| Connection success rate | 99.8 % |
| Critical bugs found | 0 |
| Total connection attempts logged | 463 |
| Total successful connections | 462 |
| Total failed connections | 1 (ISP outage) |

See the full event log: [stability-log.md](stability-log.md).

## Dogfood Stage Completion Criteria

| # | Criterion | Status |
|---|---|---|
| 1 | Team has used wgmesh continuously for **1+ week** | ✅ Completed |
| 2 | **0 critical bugs** filed against mesh connectivity | ✅ Completed |
| 3 | **95 %+** connection success rate maintained | ✅ Completed (99.8 %) |
| 4 | Stability log maintained with regular entries | ✅ Completed |
| 5 | At least one team member **signs off** on stability | ⬜ Pending sign-off |

## Advancing to Presence Stage

Once all criteria above are met, the project advances to the **Presence**
stage, where a broader set of early adopters is invited to join the mesh.
The transition checklist is:

1. Collect final sign-off from all dogfood participants.
2. Archive the stability log under `docs/dogfooding/stability-log.md`.
3. Create a Presence-stage onboarding guide (`docs/presence/onboarding.md`).
4. Announce the mesh endpoint and invitation procedure.
5. Begin tracking Presence-stage metrics in a separate log.
