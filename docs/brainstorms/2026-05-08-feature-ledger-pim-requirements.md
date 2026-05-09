---
date: 2026-05-08
topic: feature-ledger-pim
---

# Feature Ledger PIM

## Summary

A feature-ledger PIM that uses derived `STATUS.md` (generated from YAML frontmatter on `eidos/*.md`) as the visible inventory, plus four compat-locking mechanisms — testscript golden corpus for CLI/output and failure-mode behavior, a monotonic `CapabilityVersion` integer plus per-version `testdata/compat/<feature>/v0.X.Y/` fixtures for wire + on-disk state, `oasdiff breaking` in CI for external REST APIs, and (optional) `gorelease` for Go RPC exported types — so reverts have an exact target version, extensions cannot silently break backwards compat, and the gap roster lives where the work happens.

---

## Problem Frame

The wgmesh repo ships durable artifacts across several places — `eidos/` (architectural specs), `specs/issue-NNN-spec.md` (issue-driven specs), `STRATEGY.md` (high-level tracks), `docs/plans/`, GitHub Issues + labels — but there is no single load-bearing surface that answers "is feature X built, tested, stable, and what behavior is contractually locked?". `specs/STATUS.md` was a hand-curated answer; it last ran 2026-02-16 and is now ~2.5 months stale. Most `issue-NNN-spec.md` files sit unsorted in `specs/` root; only three reached `specs/implemented/`. `eidos/` carries no per-spec status field.

The cost is concrete and recurrent: the same NAT peering bug class has been filed three times across three months (#152, #457, #556) — each closure relied on a unit-test-level assertion, none of them anchored to "the verified-fixed version", so the bug came back unanchored each time. PR #570 (auto-verify e2e) closes the predicate-only-test loophole at the merge gate; it does not, by itself, give a founder the "go back to that version" button when something later regresses.

Multiple founders + Goose/Copilot agents are now the writers. Hand-curated audit indices fail under that load profile — the moment audit pauses, the doc lies. The product also ships at-most-weekly, and the four compat dimensions the founders care about (CLI + flags, wire protocol + on-disk state, behavior under failure modes, external API surfaces) span surfaces unit tests do not naturally guard. Each release re-litigates contract surfaces from memory because nothing in the repo says "this exact wire envelope shape was locked at v0.2.0".

---

## Actors

- **A1. Founders** — multiple humans authoring features, reviewing extensions, owning the compat decisions. They need the gap roster, the "verified-stable" stamp, and the regression diff surface.
- **A2. Goose / Copilot agents** — autonomous writers of new ledger entries and compat tests when implementing approved spec PRs. They consume the frontmatter convention as input and emit ledger entries + testscript corpora as output.
- **A3. Future contributors** — outside-collaborator humans inheriting the repo. They need the ledger to discover what's stable to extend and what's missing to build.
- **A4. CI** — agent-equivalent. Runs testscript suite, oasdiff, gorelease, and the `make status` generator; surfaces breakage at PR time, not at release time.

---

## Key Flows

- **F1. Register a new feature**
  - **Trigger:** A founder ships an `eidos/<name>.md` describing a new feature.
  - **Actors:** A1 (or A2 acting on A1's spec PR).
  - **Steps:** Add YAML frontmatter to the spec file (`status: provisional | implementable | implemented | deprecated`, `compat-dimensions: [...]`, `tracking-issue:`, `since:` empty until first verified release). Commit the spec. The next `make status` run picks it up.
  - **Outcome:** Feature appears in derived `STATUS.md`. No compat locks yet — `since:` is empty.
  - **Covered by:** R1, R8.

- **F2. Lock a feature's contract**
  - **Trigger:** Feature implementation lands and passes integration tests on a tagged release.
  - **Actors:** A1 (decides what to lock), A4 (executes locking).
  - **Steps:** Author writes one testscript `.txtar` per locked dimension (CLI, behavior). For wire/on-disk dimensions, author commits a binary fixture under `testdata/compat/<feature>/v0.X.Y/`. For external-API dimensions, author confirms `oasdiff breaking` is wired into CI for the relevant repo. Author updates the spec frontmatter `status: implemented` + `since: v0.X.Y`.
  - **Outcome:** Feature is contractually locked. Future PRs failing any locked test fail CI.
  - **Covered by:** R2, R3, R4, R5, R7.

- **F3. Extend a feature without breaking compat**
  - **Trigger:** A founder or agent ships a PR that touches code paths under a locked feature.
  - **Actors:** A1 / A2, A4.
  - **Steps:** Author edits code. CI runs the locked compat tests. If any locked dimension breaks, CI fails with the diff. Author either fixes the regression OR (if the change is intentional) bumps the wire `CapabilityVersion`, regenerates fixtures via `make update-golden` for the new version, leaves prior versions in place, and documents the version bump in a Key Decision in the relevant `eidos/` spec.
  - **Outcome:** Either compat preserved with no version bump, or compat intentionally evolved with the prior version still locked for replay-readers.
  - **Covered by:** R3, R6, R9.

- **F4. Render the inventory + gap roster**
  - **Trigger:** `make status` (cron, pre-release, or on-demand).
  - **Actors:** A4.
  - **Steps:** Generator script reads frontmatter from every `eidos/*.md`, cross-references presence of testscript corpus + binary fixtures + CI gate per declared compat dimension, and emits `STATUS.md` with feature × dimension matrix. Features with `status: provisional` and no compat tests are flagged as roadmap items.
  - **Outcome:** Generated `STATUS.md` shows green/red/missing per feature × dimension. Hand edits are forbidden (header warns).
  - **Covered by:** R10, R11, R12.

- **F5. Identify regression target**
  - **Trigger:** A founder observes a behavioral regression in the wild.
  - **Actors:** A1.
  - **Steps:** Open `STATUS.md`. Read the `since:` version on the affected feature. `git checkout v0.X.Y -- testdata/compat/<feature>/` to retrieve the locked corpus. Diff current behavior against locked corpus to localize the regression.
  - **Outcome:** Founder has an exact version + an exact corpus to replay against, without searching commit history.
  - **Covered by:** R4, R5, R10.

---

## Requirements

**Inventory + gap roster**
- R1. Each `eidos/*.md` MUST carry a YAML frontmatter block with at minimum: `status` (one of `provisional`, `implementable`, `implemented`, `deprecated`), `compat-dimensions` (subset of `[cli, wire, behavior, api]`), `tracking-issue` (GitHub issue number or empty if none), `since` (release tag where the feature was first locked, empty until then).
- R2. The repo MUST provide a generator (target name `make status` or equivalent) that reads frontmatter across all `eidos/*.md` and emits a single `STATUS.md` showing feature × compat-dimension matrix.
- R10. The generated `STATUS.md` MUST carry a header noting it is generated and MUST NOT be hand-edited; CI MUST fail PRs that hand-edit `STATUS.md` (e.g., via a presubmit comparing the committed file to a fresh generation).
- R11. Features with `status: provisional` or `implementable` and no committed compat artifacts MUST appear in `STATUS.md` as gap-roster entries.
- R12. The generator MUST be idempotent — running it twice without source-of-truth changes MUST produce no diff.

**Compat locking — CLI + flags**
- R3. Each feature whose `compat-dimensions` includes `cli` MUST own at least one `.txtar` testscript file under `testdata/script/` exercising the locked CLI surface (subcommands, flag combinations, exit codes, output format).

**Compat locking — wire + on-disk state**
- R4. Wire-format and on-disk-state versions MUST be expressed as monotonically increasing integers in source (e.g., `wgmesh-state.json` schema version, mesh-secret envelope version, gossip envelope version, JSON-RPC capability negotiation), modeled after Tailscale's `CapabilityVersion` pattern.
- R5. Each locked wire/state version MUST have a corresponding fixture directory under `testdata/compat/<feature>/v0.X.Y/` containing canonical binary or JSON samples; CI MUST replay these fixtures and assert the current binary still parses + produces equivalent output.

**Compat locking — failure-mode behavior**
- R6. Each feature whose `compat-dimensions` includes `behavior` MUST own at least one testscript scenario that drives the documented failure flow (e.g., NAT relay flap, hot-reload via SIGHUP, key rotation invariants) and asserts the contract holds.

**Compat locking — external API surfaces**
- R7. Each feature whose `compat-dimensions` includes `api` MUST be backed by an `openapi.yaml` (or equivalent contract spec) in the relevant repo (chimney, lighthouse), and the relevant repo MUST run `oasdiff breaking` in CI comparing HEAD against the last release tag.

**Authoring + maintenance**
- R8. The convention MUST allow Goose / Copilot agents to add a complete frontmatter block + testscript corpus during their normal spec-implementation flow, without requiring a separate human-authored audit step.
- R9. CI MUST provide a single regen affordance (e.g., `make update-golden` or `WGMESH_UPDATE_GOLDEN=1 go test`) that regenerates all golden corpora in-place, surfacing the diff in the PR for human review.

---

## Acceptance Examples

- **AE1. Covers R1, R2, R10.** A founder lands a new `eidos/foo.md` without frontmatter. CI's `make status` step fails with a clear error naming the file and required keys. The PR cannot merge until the frontmatter is added.
- **AE2. Covers R3, R6, R9.** A founder edits `pkg/daemon/relay.go` in a way that changes the daemon's stdout during NAT flap. CI replays `testdata/script/nat-relay-flap.txtar` and fails with a stdout diff. The author runs `make update-golden` locally, sees the diff, and either reverts (if unintentional) or commits the regenerated `.txtar` (if intentional). PR review sees the .txtar diff.
- **AE3. Covers R4, R5.** A founder bumps `MeshStateFormatVersion` from 2 to 3 in `pkg/daemon/cache.go`. They commit a new `testdata/compat/mesh-state/v0.3.0/` fixture directory with the new shape but leave `v0.2.0/` in place. CI replays both fixtures: the v0.2.0 fixture must parse cleanly under the read path; the v0.3.0 fixture must round-trip cleanly under read + write. If either fails, CI blocks.
- **AE4. Covers R7.** A chimney PR removes a field from a JSON response. `oasdiff breaking` flags the removal as breaking and the PR cannot merge without an explicit override label or a version bump.
- **AE5. Covers R11.** Issue #571 (Tier 5 NAT Simulation) corresponds to an `eidos/nat-simulation-tier.md` with `status: provisional` and no testscript corpus. `STATUS.md` lists it under the gap roster with its tracking issue link.
- **AE6. Covers R8.** Copilot opens an impl PR for a new spec issue. The PR includes the eidos frontmatter block, one testscript corpus, and (if applicable) an openapi diff annotation. No human-authored audit step is required for the impl PR to enter `awaiting-verification`.

---

## Success Criteria

- A founder reading `STATUS.md` after a release can identify, for any feature, which compat dimensions are locked and which release the lock is anchored to — without opening a code editor.
- A regression in a locked feature surfaces in CI as a diff against the locked golden corpus or fixture, not as a runtime bug discovered by a reporter.
- The same NAT-class bug filed three times across three months (#152, #457, #556) becomes structurally impossible: by the time the second NAT bug closes, the locked corpus + version pin make the recurrence either visible (diff against locked behavior) or impossible (CI fails before regression ships).
- A new contributor can ship a feature extension and know within one CI run whether they broke an existing contract.
- The generated `STATUS.md` and the underlying frontmatter survive multi-week cadence pauses without drift, because the source of truth is the frontmatter + tests, not the index.

---

## Scope Boundaries

- Migrating existing `specs/issue-NNN-spec.md`, `docs/plans/*`, and `docs/brainstorms/*` into the ledger — those remain sibling artifacts and feed into eidos features rather than being the ledger themselves.
- Renegotiating the deprecation policy (when may we break a wire format, what notice period to give downstream daemons) — separate decision document.
- Multi-version daemon coexistence in a live mesh (the network-effect of compat) — covered by tier-5 NAT scope in #571, not this PIM.
- A web dashboard or UI rendered on top of `STATUS.md` — markdown is sufficient; reading raw markdown in a browser via GitHub is the dashboard.
- Auto-fix tooling that mutates source code based on a compat verdict — too magical; humans review diffs.
- Replacing GitHub Issues / Projects / labels for in-flight work tracking — labels remain truth for what's being built; this PIM is for shipped + stable features only.
- Per-feature owner / RACI assignments — premature even with multiple founders.
- Deprecation SLA timelines or formal versioning policy (semver vs calver vs custom).
- Migration of `chimney`, `lighthouse`, `lighthouse-go` to OpenAPI if they have not already adopted it — that's a one-time prerequisite captured in Dependencies, not part of this PIM's deliverable.

---

## Key Decisions

- **Inventory derived, not curated.** Source of truth = YAML frontmatter on `eidos/*.md` + tests on disk. `STATUS.md` is generated. Hand-curated indices fail under multi-author load (proven by `specs/STATUS.md` going 2.5 months stale). Modeled after Kubernetes KEP `kep.yaml` + Linux `Documentation/features/` patterns.
- **Compat dimensions are explicit per-feature, not implicit.** Each `eidos` spec declares which subset of `[cli, wire, behavior, api]` is locked. Not every feature locks every dimension; declaring the subset prevents over-locking and makes the gap visible.
- **Per-dimension mechanism, not one-size-fits-all.** Different mechanisms because the failure mode is different per dimension: testscript for CLI/behavior (text-shaped contracts), monotonic version int + binary fixtures for wire/on-disk (parse-shaped contracts), oasdiff for external APIs (catalog-shaped breaking-rule detection). One-tool-fits-all would either over-engineer the simple cases or under-cover the complex ones.
- **Wire/state versioning uses monotonic integer, not semver.** Tailscale's `CapabilityVersion` model: a single increasing int, with comments documenting what each version enables. Avoids semver-bikeshed; reads naturally for daemons that need to ask "is the peer at least version N?".
- **Per-version fixtures committed, not cleaned up.** When a wire format bumps from v2 to v3, the v2 fixture stays in `testdata/compat/<feature>/v0.2.0/`. Replay continues for as long as the read path supports v2. Removing the fixture happens only when the read path drops support — that itself becomes a deliberate, documented compat break.
- **Regen affordance is one env-var-gated command.** Modeled on CUE's `CUE_UPDATE` convention. `WGMESH_UPDATE_GOLDEN=1 go test` (or `make update-golden`) regenerates all goldens in-place and surfaces the diff in the PR. No interactive review tool — the PR diff is the review surface.
- **`gorelease` is optional, not mandatory.** Useful for daemon Go RPC exported types, but redundant for the JSON-RPC wire shape (which testscript covers). Adopt only if the Go-internal SDK boundary becomes externally consumed (e.g., third parties importing `pkg/rpc`).
- **OpenAPI is a prerequisite, not a deliverable.** This PIM assumes chimney + lighthouse already publish (or can quickly publish) `openapi.yaml`. If they don't, the prerequisite work is one-time per repo and tracked as Dependencies.
- **Frontmatter target is `eidos/`, not `specs/`.** `eidos/*.md` already holds architectural feature specs and is the natural home for "what feature does this anchor". `specs/issue-NNN-spec.md` remains issue-driven (one file per GitHub issue) and feeds into eidos features without being the ledger itself.
- **Verified-in-version trust = passing CI on the tag commit.** No separate human-typed attestation. If CI green on tag v0.X.Y, all `since: v0.X.Y` claims are anchored to that exact commit.

---

## Dependencies / Assumptions

- `chimney` and `lighthouse` either already publish `openapi.yaml`, or generating one from existing route definitions is a small one-time task per repo (estimated ~1 hour each per the research note). If neither is true, the `api` dimension cannot be enforced until that prerequisite ships.
- `rogpeppe/go-internal/testscript` is acceptable as a dependency in the wgmesh test build (it is widely used in the Go ecosystem; CUE, the Go toolchain, and many CLI projects depend on it).
- `oasdiff` is acceptable as a CI tool (already exists as `oasdiff/oasdiff-action`).
- The release-tag workflow (`hetzner-integration.yml` → `release.yml`) continues to gate releases on full integration test pass; this PIM consumes the tag as the "verified" anchor and does not change release gating itself.
- Goose and Copilot agents can read structured YAML frontmatter and write valid frontmatter when prompted via the existing `.github/copilot-instructions.md` and Goose recipes; instructions for the new convention will need to be added to those files.

---

## Outstanding Questions

- Should the generator also surface stale entries (e.g., features whose `since` is older than N releases without any test changes — possible drift signal)? Defer to first generator iteration; revisit once `STATUS.md` is in regular use.
- Should `oasdiff` failures be hard-block or soft-warn at first? Lean hard-block for parity with testscript, but accept revisiting once the first false positive shows up.
- Where does the generator script live — `scripts/status/`, `tools/cmd/status/`, or as a `go generate` directive embedded in `eidos/`? Defer to implementation; not a scope-shaping question.
- Should the `STATUS.md` file be committed to the repo or `.gitignore`-d and regenerated on demand? Lean toward committed-as-generated (with the DO-NOT-EDIT header) so external readers browsing the repo on GitHub see it; revisit if the diff churn becomes annoying.
