# Specification: Issue #532

## Classification
documentation

## Deliverables
documentation

## Problem Analysis

wgmesh has installation guides, a quickstart, FAQ, and centralized-mode reference, but no single document that helps a network administrator decide in 15 minutes whether wgmesh fits their situation.

Gaps relative to the acceptance criteria:

1. **No infrastructure requirements checklist.** An admin cannot quickly verify whether their hosts, kernel versions, and firewall constraints are compatible with wgmesh before investing time in a pilot.
2. **No use-case fit questionnaire.** There is no structured way for an admin to match their problem (site-to-site, remote workers, home lab, container networking) to one of wgmesh's two operating modes.
3. **No decision framework.** Existing docs describe what wgmesh does, but never compare it to Tailscale, Netmaker, or innernet — the three alternatives named in the README. Admins have no documented guidance on when to choose wgmesh.
4. **No concrete evaluation test scenarios.** Docs describe commands but do not define pass/fail criteria an evaluator can record and hand off to a team lead.
5. **No pilot setup recommendations.** There is no suggested minimal topology, timeline, or success metric for a proof-of-concept before committing to production.

The deliverable is a new file `docs/evaluation-checklist.md` plus a short link from `README.md`.

## Implementation Tasks

### Task 1: Create `docs/evaluation-checklist.md`

Create the file `docs/evaluation-checklist.md` with exactly the following content:

```markdown
# Evaluation Checklist for Network Administrators

Use this checklist to assess whether wgmesh fits your use case in roughly 15 minutes.
Work through each section top to bottom; the final section gives you a clear go/no-go recommendation.

---

## Section 1 — Infrastructure Requirements (5 minutes)

Mark each item ✅ (met), ❌ (not met), or ⚠️ (needs investigation).

### 1.1 — Kernel / OS

| Requirement | Notes | Status |
|-------------|-------|--------|
| Linux kernel ≥ 5.6, **or** macOS with `wireguard-go` installed | WireGuard is built into Linux 5.6+; older kernels need the DKMS module | |
| `wireguard-tools` package installed (`wg` command available) | `apt install wireguard-tools` / `yum install wireguard-tools` | |
| Root access or `CAP_NET_ADMIN` capability on each node | Required to create and configure WireGuard interfaces | |

### 1.2 — Network

| Requirement | Notes | Status |
|-------------|-------|--------|
| Outbound UDP is not completely blocked on every node | At minimum, one node must be reachable over UDP for direct connections | |
| At least one node has a public IP **or** NAT traversal (UDP hole-punching) is acceptable | Nodes behind symmetric NAT may require a relay path | |
| Nodes can reach the public internet for initial DHT discovery | Outbound TCP 443 or UDP 6881 to BitTorrent DHT bootstrap nodes | |

### 1.3 — Storage and State

| Requirement | Notes | Status |
|-------------|-------|--------|
| `/var/lib/wgmesh/` is writable on each node | Stores WireGuard keypair and peer cache; ~few KB per node | |
| Systemd available (if persistent service is required) | `wgmesh install-service` creates a `wgmesh.service` unit | |

**Section 1 verdict:** If any item is ❌, resolve it before proceeding. Items marked ⚠️ are addressed in Section 3.

---

## Section 2 — Use Case Fit (5 minutes)

Answer each question and tally the mode recommendations.

### 2.1 — Topology Questions

**Q1: How many nodes will the mesh contain at steady state?**
- ≤ 50 nodes → **decentralized mode preferred**
- 50–200 nodes → **either mode works; decentralized scales to this range**
- > 200 nodes → ⚠️ evaluate centralized mode or contact the project for guidance

**Q2: Who manages node additions and removals?**
- Nodes join/leave autonomously (e.g., auto-scaling, developer laptops) → **decentralized mode**
- An operator controls all changes via SSH from a central host → **centralized mode**

**Q3: Are all nodes behind NAT, or do some have public IPs?**
- All behind NAT → decentralized mode with UDP hole-punching; check that at least one bootstrap peer is reachable
- At least one node has a public IP → direct endpoint configuration; both modes handle this

**Q4: Do you require site-to-site routing (advertising subnets)?**
- Yes → both modes support `--advertise-routes`; decentralized mode propagates routes automatically
- No → no impact on mode choice

### 2.2 — Operational Questions

**Q5: Is there a central operations team that manages WireGuard state?**
- Yes → **centralized mode** (state file in `mesh-state.json`, SSH-based deployment)
- No / decentralized DevOps → **decentralized mode**

**Q6: Do nodes need to discover each other without pre-sharing IP addresses?**
- Yes (dynamic IPs, cloud auto-scaling, remote workers) → **decentralized mode required**
- No (static IP fleet) → either mode works

**Q7: Is an encrypted state file at rest required?**
- Yes → centralized mode supports AES-256-GCM + PBKDF2 encryption of `mesh-state.json`
- No preference → either mode

### 2.3 — Mode Recommendation

Count your answers:
- Mostly decentralized → proceed with **decentralized mode** (`wgmesh join`)
- Mostly centralized → proceed with **centralized mode** (`wgmesh -deploy`)
- Mixed → decentralized mode is the default; use centralized for operator-controlled fleets

---

## Section 3 — Decision Framework: wgmesh vs Alternatives (2 minutes)

Use this table to confirm wgmesh is the right tool. If a competing tool better fits your profile, the ❌ cells explain why.

| Scenario | wgmesh | Tailscale | Netmaker | innernet |
|----------|--------|-----------|----------|----------|
| No coordination server to host or trust | ✅ (DHT-based, no server) | ❌ (requires Tailscale control plane) | ❌ (requires Netmaker server) | ❌ (requires innernet server) |
| Self-hosted, open-source, auditable | ✅ | ❌ (SaaS) | ✅ (self-hosted) | ✅ (self-hosted) |
| Serverless peer discovery (NAT traversal included) | ✅ | ✅ | ❌ (server required) | ❌ (server required) |
| Share one secret to add any node | ✅ | ❌ (ACL/invite required) | ❌ (token per node) | ❌ (certificate per node) |
| macOS and Linux support | ✅ | ✅ | ✅ | ✅ |
| Windows support | ❌ (not yet) | ✅ | ✅ | ❌ |
| Web UI / dashboard | ❌ | ✅ | ✅ | ❌ |
| Per-node access control lists | ✅ (centralized mode policy engine) | ✅ | ✅ | ✅ |
| Zero external dependencies for cold start | ✅ (GitHub Issues registry as L0 bootstrap) | ❌ | ❌ | ❌ |

**Choose wgmesh when:** you want a coordination-server-free, self-hosted mesh where any node can join with a shared secret, and you do not need Windows support or a web dashboard.

**Choose an alternative when:** you need Windows clients, a web UI, or an enterprise-grade access-control system with per-user identity.

---

## Section 4 — Evaluation Test Scenarios (3 minutes to read; 15–30 minutes to run)

Run these tests during your pilot to confirm wgmesh behaves as expected.

### Test A — Two-node basic mesh (decentralized mode)

**Setup:** Two hosts (can be VMs or VPS). Both must have outbound internet access.

**Steps:**
1. On **host-1**: `wgmesh init --secret` → copy the printed secret.
2. On **host-1**: `sudo wgmesh join --secret "<secret>" --log-level debug`
3. On **host-2**: `sudo wgmesh join --secret "<secret>" --log-level debug`
4. Wait up to 30 seconds for DHT discovery.
5. On **host-1**: `wgmesh peers list`

**Pass criteria:**
- `wgmesh peers list` shows **host-2** with a mesh IP and a non-stale `LAST SEEN` timestamp.
- `ping <host-2-mesh-ip>` from **host-1** succeeds.
- `sudo wg show wg0` on **host-1** shows **host-2** with a recent `latest-handshake`.

**Fail indicator:** No peers appear after 2 minutes → check UDP outbound; run `wgmesh status` and look for `[dht]` lines in debug output.

---

### Test B — NAT traversal (both hosts behind NAT)

**Setup:** Two hosts behind different NAT gateways (e.g., two cloud VMs in different VPCs without public IPs, or two developer laptops on different home networks).

**Steps:**
1. Repeat Test A steps with both hosts behind NAT.
2. Observe `ENDPOINT` column in `wgmesh peers list`.

**Pass criteria:**
- Peer appears with an `ENDPOINT` of the form `<public-ip>:<port>` (hole-punched) **or** `(relayed)`.
- `ping <mesh-ip>` succeeds in either case.

**Note:** Symmetric NAT on both ends may result in `(relayed)` — this is expected and functional.

---

### Test C — Subnet advertisement (site-to-site routing)

**Setup:** **host-1** has a private subnet `192.168.10.0/24` behind it (or a loopback alias for testing).

**Steps:**
1. On **host-1**:
   ```bash
   sudo wgmesh join --secret "<secret>" --advertise-routes "192.168.10.0/24"
   ```
2. On **host-2**: `wgmesh peers list` → confirm **host-1** shows `ROUTES: 192.168.10.0/24`.
3. On **host-2**: `ip route get 192.168.10.1` → output should route through `wg0`.
4. On **host-2**: `ping 192.168.10.1` (or a host in that subnet).

**Pass criteria:**
- Route `192.168.10.0/24 via <host-1-mesh-ip> dev wg0` is present on **host-2**.
- Traffic to the subnet is forwarded correctly.

---

### Test D — Node restart / persistence

**Steps:**
1. After Test A is passing, stop the daemon on **host-1**: `sudo systemctl stop wgmesh` (or Ctrl-C).
2. Restart: `sudo systemctl start wgmesh` (or re-run `sudo wgmesh join ...`).
3. On **host-1**: wait 30 seconds, then `wgmesh peers list`.

**Pass criteria:**
- **host-2** reappears in the peer list within 60 seconds without manual intervention.
- Same mesh IPs are used as before (deterministic from secret).

---

### Test E — Adding a third node

**Steps:**
1. While Test A is passing, bring up **host-3** with the same secret.
2. On **host-1** and **host-2**: `wgmesh peers list` after 30 seconds.

**Pass criteria:**
- All three nodes see each other.
- `wgmesh peers count` returns `2` on each node (two remote peers).

---

## Section 5 — Pilot Setup Recommendations

### Minimal pilot topology

- **2–3 nodes**: one with a public IP (or Hetzner/DigitalOcean cheapest VPS), one or two behind NAT.
- **Duration**: 48–72 hours continuous operation to confirm stability and peer reconnection after restarts.
- **Monitoring**: tail daemon logs (`journalctl -u wgmesh -f`) and periodically run `wgmesh peers list`.

### Recommended pilot sequence

1. **Day 1 — Basic connectivity (Test A + Test B):** Validate peer discovery and NAT traversal.
2. **Day 1–2 — Subnet routing (Test C):** If site-to-site is required, validate route propagation.
3. **Day 2 — Persistence (Test D):** Simulate reboots and daemon restarts; confirm auto-recovery.
4. **Day 2–3 — Scale (Test E):** Add the third node; confirm mesh expands correctly.
5. **Day 3 — Load and stability:** Keep the mesh running for 24 hours; check for unexpected disconnections in logs.

### Success criteria for go decision

All of the following must be true after the pilot:

- [ ] All pilot nodes appear in `wgmesh peers list` on each other within 60 seconds of starting.
- [ ] `ping <mesh-ip>` succeeds between every node pair.
- [ ] Daemon survives a node reboot without manual reconfiguration.
- [ ] If subnet routing is required: routes are present and traffic flows.
- [ ] No unexplained crashes in `journalctl -u wgmesh` over 24 hours of operation.

### No-go triggers

Stop the evaluation and investigate (or file a bug) if:

- Peers never appear after 5 minutes with `--log-level debug` showing no DHT activity.
- `ping` between mesh IPs times out despite both peers showing in `wgmesh peers list`.
- Daemon exits unexpectedly within 24 hours of running.
- Required feature (e.g., Windows support, web UI) is in the ❌ column of Section 3.

---

## Summary: Go / No-Go Decision

| Signal | Go | No-Go |
|--------|----|-------|
| Section 1: All infrastructure items ✅ | ✅ | Any ❌ unresolved |
| Section 2: Mode recommendation is clear | ✅ | No clear mode fit |
| Section 3: wgmesh column fits your scenario | ✅ | Competing tool fits better |
| Section 4: Tests A–D pass | ✅ | Any test fails after troubleshooting |
| Section 5: Pilot success criteria met | ✅ | Any criterion unmet |

If all rows are **Go**: proceed to production rollout using [docs/quickstart.md](quickstart.md) (decentralized) or [docs/centralized-mode.md](centralized-mode.md) (centralized).

If any row is **No-Go**: file an issue at https://github.com/atvirokodosprendimai/wgmesh/issues with the failing section and debug output.
```

### Task 2: Add a link to the evaluation checklist in `README.md`

In `README.md`, locate the existing `## Quick Start` section. It ends with:

```markdown
That's it. Nodes find each other via DHT, exchange keys, and build the mesh.

For a step-by-step walkthrough with verification steps, troubleshooting, and all install methods,
see [docs/quickstart.md](docs/quickstart.md).
```

Replace those two final sentences with:

```markdown
That's it. Nodes find each other via DHT, exchange keys, and build the mesh.

For a step-by-step walkthrough with verification steps, troubleshooting, and all install methods,
see [docs/quickstart.md](docs/quickstart.md).

Evaluating whether wgmesh fits your infrastructure? Use the
[15-minute evaluation checklist](docs/evaluation-checklist.md) to reach a go/no-go decision.
```

If the `README.md` does not yet contain the `docs/quickstart.md` pointer sentence (i.e., the `## Quick Start` section still ends with just `That's it. Nodes find each other via DHT, exchange keys, and build the mesh.`), append the following two new lines directly after that sentence instead:

```markdown
For a step-by-step walkthrough with verification steps, troubleshooting, and all install methods,
see [docs/quickstart.md](docs/quickstart.md).

Evaluating whether wgmesh fits your infrastructure? Use the
[15-minute evaluation checklist](docs/evaluation-checklist.md) to reach a go/no-go decision.
```

## Affected Files

- **New:** `docs/evaluation-checklist.md` — the evaluation checklist document (~200 lines of Markdown)
- **Modified:** `README.md` — add one sentence linking to the checklist from `## Quick Start`

No code files are changed. No Go packages are touched. No new dependencies.

## Test Strategy

No automated tests required for documentation. Verify manually:

1. `docs/evaluation-checklist.md` renders without broken Markdown in GitHub preview (all tables aligned, all code fences closed).
2. Every relative link in `docs/evaluation-checklist.md` resolves to an existing file:
   - `quickstart.md` → `docs/quickstart.md` ✓
   - `centralized-mode.md` → `docs/centralized-mode.md` ✓
   - `https://github.com/atvirokodosprendimai/wgmesh/issues` → valid GitHub URL ✓
3. The link `docs/evaluation-checklist.md` added to `README.md` resolves to the new file.
4. The checklist covers all five areas from the issue acceptance criteria:
   - Infrastructure requirements ✓ (Section 1)
   - Use case fit ✓ (Section 2)
   - Decision framework vs alternatives ✓ (Section 3)
   - Concrete test scenarios with pass/fail criteria ✓ (Section 4)
   - Pilot setup recommendations ✓ (Section 5)

## Estimated Complexity
low

**Reasoning:** Pure documentation. One new Markdown file (~200 lines) and one targeted edit to `README.md` (3 lines added). No code changes, no dependency updates, no build pipeline changes. Estimated effort: 30–45 minutes.
