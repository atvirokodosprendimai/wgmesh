# Specification: Issue #803 — 60-second quickstart install page on cloudroof.eu

## Classification
feature

## Problem Analysis

The issue asks for a "60-second quickstart" install experience on cloudroof.eu: a single `curl … | sh` one-command install that ends with a working mesh trial node joined to a shared, time-boxed trial mesh.

Findings from the codebase:

1. **There is no install script in the repo.** `grep`/`find` for `install.sh` returns nothing. The GoReleaser config (`.goreleaser.yml`) publishes `.tar.gz` archives, `.deb`/`.rpm` packages, a Homebrew formula, and a `checksums.txt`, but no curl-able installer. Each existing install path in `docs/quickstart.md` and `README.md` is a multi-step manual sequence (download → `chmod`/`install` → `wgmesh version`). That is not a 60-second one-liner.

2. **The landing pages already advertise a one-liner that does not resolve.** Both `index.html` and `wgmesh.dev/index.html` contain `curl -sSfL https://install.wgmesh.dev | sh`, and `index.html` has an `#install` section. That URL returns nothing today — there is no `install.wgmesh.dev` target, no script committed, and no hosting/redirect documented. This is a broken promise on the marketing surface.

3. **Release asset naming is known and stable.** GoReleaser archive template is `wgmesh_{{ .Version }}_{{ .Os }}_{{ .Arch }}` (e.g. `wgmesh_v0.2.0_linux_amd64.tar.gz` containing a `wgmesh` binary), plus `wgmesh_{{ .Version }}_{{ .Os }}_{{ .Arch }}.deb` / `.rpm` via nfpms, and `checksums.txt` per release. The "latest" install can be built against `https://github.com/atvirokodosprendimai/wgmesh/releases/latest/download/...` using a **stable** latest-asset naming scheme that the installer must resolve (see Open Question / convention below).

4. **`wgmesh join` is the single command that produces a working node.** Per `docs/quickstart.md`, after the binary is present, `sudo wgmesh join --secret "wgmesh://v1/<secret>"` derives all crypto params (HKDF), generates a keypair, and runs discovery (DHT/LAN/registry). So "spins up a working mesh trial" = install the binary + `join` a pre-provisioned **trial secret** + verify with `wgmesh peers list`.

5. **A trial secret must exist and be safe to publish.** The decentralized design means anyone holding the secret can join the mesh. For a public trial this is acceptable **if** the trial mesh is explicitly isolated, ephemeral, rate-limited, and carries no customer PII and no route advertisements into real networks. There is no trial-secret concept in the repo today; the spec must define how the trial secret is provisioned and rotated, and the public install must never embed a production or customer secret.

6. **No web-serving of static assets from the wgmesh binary.** As noted in the related `issue-753-spec.md`, `main.go` only serves pprof/metrics. cloudroof.eu is served externally (Caddy edge per `deploy/edge/setup.sh`). So the "page" is a static asset (`public/`) plus an installer hosted at a resolvable URL; the wgmesh binary is not the web server.

7. **Verification must be quick and honest.** A real 60-second promise requires: tiny binary download, a clear success state, and a `wgmesh peers list` / ping that lights up green. The page needs a live "are nodes online?" check (read-only status endpoint on the trial mesh) so the user sees the mesh working without reading logs.

Concerns: The marketing copy already ships a broken `install.wgmesh.dev` link. The realistic deliverable is the committed installer script + a hosted redirect + a redesigned quickstart section in `public/` + a trial-mesh bootstrap runbook. Web-server config for cloudroof.eu itself is out of scope (covered by `deploy/edge/`), but the spec must state where `install.sh` is hosted.

## Proposed Approach

Deliver four things, all of which are independently testable:

1. **A versioned, curl-able installer script** (`scripts/install.sh`, committed and version-controlled) that detects OS/arch, downloads the matching release artifact from GitHub `releases/latest`, verifies the SHA256 against the published `checksums.txt`, installs the binary to `/usr/local/bin/wgmesh` (or `~/.local/bin` when not root), and runs `wgmesh version` to confirm. It is the single source of truth — `install.wgmesh.dev` will be a redirect to the raw file in the public repo (no separate copy to drift).

2. **A `wgmesh quickstart` (a.k.a. `trial`) subcommand** that wraps the end-to-end trial flow: takes the published trial secret (from `WGMESH_TRIAL_SECRET` env or the default embedded public trial), runs `join` with trial-safe defaults (no `--advertise-routes`, fixed trial interface name, `--log-level info`), and prints a one-line success + the `wgmesh peers list` result. This makes "working mesh trial" a single command and gives the page a deterministic success string to reference.

3. **A redesigned 60-second quickstart section in `public/index.html`** (the cloudroof.eu page) with: the one-liner front and center, a copy button, a three-step timeline (install → join trial → verify), prerequisite chips (Linux/macOS, root/sudo, UDP 51820 open), and a read-only trial status badge that pings a public status JSON for "nodes online in the last 60s". No secrets, PII, or revenue figures on the page.

4. **A trial-mesh bootstrap + rotation runbook** (`docs/trial-mesh.md`) describing how the public trial secret is generated (`wgmesh init --secret`), how it is published to the installer default, the isolation rules (separate interface, no route advertisement, ephemeral peer TTL), how the status JSON is produced, and the rotation cadence. Public-repo-safe: only the *public* trial secret (an intentionally shared, low-value credential) is referenced; never customer or production secrets.

Hosting decision (stated, not built here): `install.wgmesh.dev` resolves via DNS to a redirect (Caddy on the existing edge, per `deploy/edge/setup.sh`) to `https://raw.githubusercontent.com/atvirokodosprendimai/wgmesh/main/scripts/install.sh`. This keeps one source of truth in the repo. The web-server config change itself is noted as an out-of-scope ops task.

## Acceptance Criteria

- `scripts/install.sh` exists, is `set -euo pipefail`, and is `shellcheck`-clean (`shellcheck scripts/install.sh` exits 0).
- Running `curl -fsSL https://raw.githubusercontent.com/atvirokodosprendimai/wgmesh/main/scripts/install.sh | sh` on a clean Linux amd64 VM and a macOS arm64 host installs `wgmesh` and `wgmesh version` prints a non-empty version, with no manual `chmod`/`mv`/`wget` steps required by the user.
- The installer rejects unsupported platforms with a clear message and non-zero exit (e.g. Windows/i386), and falls back gracefully when neither root nor a writable `/usr/local/bin` is available (installs to `~/.local/bin` and prints a PATH hint).
- The installer verifies the downloaded artifact's SHA256 against the release `checksums.txt` and aborts with a non-zero exit on mismatch (tamper/failure detection).
- The installer prints the single next command to join the trial (`wgmesh quickstart`) so the page's 60-second promise is literally two steps: install, then quickstart.
- `wgmesh quickstart --help` exists; `wgmesh quickstart` (using the embedded public trial secret) reaches a `join`ed state and prints a deterministic success line containing the substring `trial mesh joined` followed by a `wgmesh peers list` summary. A `go test` exercises the CLI dispatch for this subcommand.
- `public/index.html` contains a 60-second quickstart block featuring the `curl … | sh` one-liner, a copy-to-clipboard control, the three steps (install / `wgmesh quickstart` / verify), prerequisite chips, and a trial status badge element that fetches a public status URL. The page contains no secrets, no PII, no revenue numbers.
- A public trial status artifact path is defined (e.g. `https://install.wgmesh.dev/trial-status.json`) and the page degrades gracefully (shows "checking…" then "trial online"/"trial offline") if the fetch fails, never throwing an uncaught error.
- `docs/trial-mesh.md` documents: how to generate the trial secret, where it is published, isolation defaults, the status JSON schema, and the rotation cadence. It contains only the **public trial** secret reference and a statement that production/customer secrets are never embedded.
- The broken `install.wgmesh.dev` references already present in `index.html` and `wgmesh.dev/index.html` are reconciled: the one-liner points at the script committed in this repo (raw URL or the redirect), and the copy matches the installer's printed next-step.
- `go build ./...` and `go test ./...` pass (the new subcommand and its tests are the only Go changes). `make lint` passes.
- No secrets, customer PII, or revenue figures are introduced anywhere in the diff.

## Out of scope

- **Cloudroof.eu web-server / Caddy / DNS configuration for `install.wgmesh.dev` and `trial-status.json`.** Stated as the hosting target; the actual infra change is an ops task tracked separately (see `deploy/edge/setup.sh`).
- **Trial-mesh infrastructure provisioning** (the VM(s) that actually run the trial mesh nodes). The runbook describes how; standing/rotating the live trial fleet is operational work.
- **Account system, sign-ups, or billing gating** on the trial. The trial is an open, shared, low-value mesh — no auth, no PII collection. (Trial nurture/billing is covered by other issues.)
- **Windows and i386 support** in the installer. Installer detects and exits cleanly with a message; native support is a follow-up.
- **Redesign of the whole cloudroof.eu landing page**, pricing, or chat widget (covered by issues #732, #753, and the ROADMAP landing-repositioning item). Only the 60-second quickstart block is in scope here.
- **Managed-ingress / Lighthouse CDN trial features.** This trial is decentralized-mesh-only (peer connectivity). Lighthouse-managed ingress trials are a separate effort.
- **Automated browser/E2E tests of the landing page badge.** Manual verification + a documented curl check of `trial-status.json`; full Playwright/Cypress coverage is out of scope.
