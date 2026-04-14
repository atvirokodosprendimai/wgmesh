# Specification: Issue #520

## Classification
documentation

## Deliverables
documentation

## Problem Analysis

`README.md` currently covers Quick Start (3 commands), How It Works, Usage, Installation
(binaries, source, Docker), Security, Architecture, Troubleshooting, Contributing, and License.

Four concrete gaps exist relative to the issue acceptance criteria:

1. **Homebrew install method is absent from README.** It appears in `docs/quickstart.md` but the
   README's `## Installation` section lists only Pre-built Binaries, From Source, and Docker. A
   user who reads only the README has no idea Homebrew is available.

2. **No "Common Use Cases" section.** The README describes *how* the tool works but not *when* to
   reach for it. New users cannot quickly self-identify their scenario (home lab, remote workers,
   container networking, etc.) and find the matching command set.

3. **Quick Start does not link to the detailed walkthrough.** The three-command block in
   `## Quick Start` sends users straight to `wgmesh join` with no pointer to
   `docs/quickstart.md` for post-join verification or troubleshooting. Acceptance criteria state
   "new users can get running in under 5 minutes" — the detailed guide already enables this; it
   just needs prominent exposure.

4. **No Support / Community section.** There is no entry point for users who need help: no
   GitHub Discussions reference, no Issues link for bug reports, and no mention of where to ask
   questions. This is required by the acceptance criteria ("Support/community information").

The existing `docs/quickstart.md` is comprehensive and already handles the full installation
walkthrough — no changes to that file are needed. All changes are confined to `README.md`.

## Implementation Tasks

### Task 1: Add Homebrew to the `## Installation` section in `README.md`

In `README.md`, find the `## Installation` section. It currently starts with:

```markdown
## Installation

### Pre-built Binaries
```

Insert a new `### Homebrew` subsection **before** `### Pre-built Binaries` so the section becomes:

```markdown
## Installation

### Homebrew (macOS and Linux)

```bash
brew install atvirokodosprendimai/tap/wgmesh
wgmesh version
```

### Pre-built Binaries
```

Leave all subsequent subsections (`### Pre-built Binaries`, `### From Source`, `### Docker`,
`### Verify Installation`) unchanged.

---

### Task 2: Add a "5-minute" pointer to `docs/quickstart.md` in `## Quick Start`

In `README.md`, the `## Quick Start` block ends with:

```markdown
That's it. Nodes find each other via DHT, exchange keys, and build the mesh.
```

Replace that closing sentence with:

```markdown
That's it. Nodes find each other via DHT, exchange keys, and build the mesh.

For a step-by-step walkthrough with verification steps, troubleshooting, and all install methods,
see [docs/quickstart.md](docs/quickstart.md).
```

---

### Task 3: Add a `## Common Use Cases` section

In `README.md`, insert a new section **between** `## Quick Start` and `## How It Works`. Use the
following exact content:

```markdown
## Common Use Cases

### Home lab / self-hosted services

Connect your home server, a VPS, and a laptop into a single private network without opening
firewall ports. Every node gets a stable mesh IP regardless of its real IP or NAT situation.

```bash
# On each machine — same secret, automatic discovery
sudo wgmesh join --secret "wgmesh://v1/<your-secret>"
```

### Remote development / team VPN

Give every developer a persistent mesh IP. Expose internal services (databases, staging
environments) without a VPN server or static IP. New team member joins by receiving the secret.

```bash
# Developer laptop joins the team mesh
sudo wgmesh join --secret "wgmesh://v1/<team-secret>" --interface wg1
```

### Advertising subnets (site-to-site)

A node can advertise a local subnet into the mesh so all peers route traffic through it — useful
for connecting office networks or exposing a Kubernetes pod CIDR.

```bash
sudo wgmesh join \
  --secret "wgmesh://v1/<your-secret>" \
  --advertise-routes "192.168.10.0/24"
```

### Fleet management (centralized mode)

Manage WireGuard across a large fleet from a single control node. Topology lives in a state file;
changes are deployed via SSH with zero interface restarts.

```bash
wgmesh -init
wgmesh -add node1:10.99.0.1:192.168.1.10
wgmesh -add node2:10.99.0.2:203.0.113.50
wgmesh -deploy
```

See [docs/centralized-mode.md](docs/centralized-mode.md) for the full reference.
```

---

### Task 4: Add a `## Support` section

In `README.md`, insert a new `## Support` section **between** `## Contributing` and `## License`.
Use the following exact content:

```markdown
## Support

- **Bug reports & feature requests** — [Open an issue](https://github.com/atvirokodosprendimai/wgmesh/issues)
- **Questions & discussion** — [GitHub Discussions](https://github.com/atvirokodosprendimai/wgmesh/discussions)
- **Documentation** — [docs/](docs/) for detailed guides; [docs/quickstart.md](docs/quickstart.md) for the getting-started walkthrough
- **Troubleshooting** — [docs/quickstart.md#troubleshooting](docs/quickstart.md#troubleshooting) (decentralized mode) and [docs/troubleshooting.md](docs/troubleshooting.md) (centralized mode)
```

---

## Affected Files

- **Modified:** `README.md` — add Homebrew install method, quickstart link, Common Use Cases
  section, and Support section. No other files are changed.

No code files are touched. No Go packages are changed. No new dependencies.

## Test Strategy

No automated tests required for documentation. Verify manually:

1. Open `README.md` and confirm all four changes are present:
   - `### Homebrew (macOS and Linux)` subsection appears before `### Pre-built Binaries` in `## Installation`.
   - `docs/quickstart.md` link appears at the end of `## Quick Start`.
   - `## Common Use Cases` section appears between `## Quick Start` and `## How It Works`.
   - `## Support` section appears between `## Contributing` and `## License`.
2. Confirm all relative links in the new content resolve to existing files:
   - `docs/quickstart.md` ✓ (exists)
   - `docs/centralized-mode.md` ✓ (exists)
   - `docs/troubleshooting.md` ✓ (exists)
   - `docs/quickstart.md#troubleshooting` ✓ (anchor exists in quickstart.md)
   - `https://github.com/atvirokodosprendimai/wgmesh/issues` ✓ (GitHub URL)
   - `https://github.com/atvirokodosprendimai/wgmesh/discussions` ✓ (GitHub URL)
3. Confirm Markdown renders without broken fences or table misalignment in a GitHub preview.

## Estimated Complexity
low

**Reasoning:** Pure documentation. Four targeted edits to a single file (`README.md`), adding
roughly 60–80 lines of new Markdown. No code changes, no dependency updates, no build pipeline
changes. Estimated effort: 30–45 minutes.
