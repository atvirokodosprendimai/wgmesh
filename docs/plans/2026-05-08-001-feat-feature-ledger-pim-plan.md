---
title: "feat: Feature ledger PIM — derived STATUS.md + per-dimension compat locks"
type: feat
status: active
date: 2026-05-08
origin: docs/brainstorms/2026-05-08-feature-ledger-pim-requirements.md
---

# feat: Feature ledger PIM — derived STATUS.md + per-dimension compat locks

## Summary

Land the feature-ledger PIM in six implementation units: a frontmatter spec + CI validator on `eidos/*.md`, a `make status` generator that emits a derived `STATUS.md`, a `testscript` harness bootstrap with one example `.txtar` proof, a monotonic `CapabilityVersion` + per-version fixture-replay scaffold for wire/on-disk locks, an `oasdiff` CI workflow snippet for the external-API repos, and a one-shot frontmatter backfill across existing `eidos/*.md` files. Sequence prefers foundation-first (frontmatter + generator) so subsequent units have ground to anchor on.

---

## Problem Frame

Feature-ledger PIM described in origin doc; this plan defines the build sequence. (See origin: docs/brainstorms/2026-05-08-feature-ledger-pim-requirements.md.) Plan-specific framing: the work is foundation-shaped — every later mechanism (testscript corpus, fixture replay, oasdiff) consumes the frontmatter convention U1 establishes, so U1 is a hard prerequisite for U2 and U6 and a soft prerequisite for U3-U5 (those can land independently but their value compounds once frontmatter is live).

---

## Requirements

- R1. Each `eidos/*.md` MUST carry YAML frontmatter (`status`, `compat-dimensions`, `tracking-issue`, `since`).
- R2. `make status` (or equivalent target) generates `STATUS.md` from frontmatter + on-disk compat artifacts.
- R3. CLI compat dimension locked via `.txtar` testscript files.
- R4. Wire / on-disk compat dimension uses monotonic integer versions.
- R5. Per-version fixture replay under `testdata/compat/<feature>/v0.X.Y/`.
- R6. Failure-mode behavior locked via testscript scenarios.
- R7. External-API compat locked via `oasdiff breaking` in CI.
- R8. Convention supports Goose / Copilot agents authoring complete frontmatter + corpus.
- R9. Single regen affordance (`make update-golden` / `WGMESH_UPDATE_GOLDEN=1`).
- R10. `STATUS.md` is generated; CI fails hand-edits.
- R11. Provisional/implementable features without compat artifacts surface in gap roster.
- R12. Generator is idempotent.

**Origin actors:** A1 (founders), A2 (Goose/Copilot agents), A3 (future contributors), A4 (CI).
**Origin flows:** F1 (register feature), F2 (lock contract), F3 (extend without breaking), F4 (render inventory + gap roster), F5 (identify regression target).
**Origin acceptance examples:** AE1 (covers R1, R2, R10), AE2 (covers R3, R6, R9), AE3 (covers R4, R5), AE4 (covers R7), AE5 (covers R11), AE6 (covers R8).

---

## Scope Boundaries

- This plan does NOT migrate `chimney` / `lighthouse` / `lighthouse-go` to publish `openapi.yaml` — that prerequisite is documented and tracked separately. U5 ships only the GitHub Action snippet + a doc; consuming repos must adopt it themselves.
- This plan does NOT write compat tests for every existing feature — only the bootstrap example in U3 + the envelope fixture in U4. Backfilling compat tests across the full feature set is follow-up work.
- This plan does NOT add `gorelease` — origin marked it optional; deferred until daemon Go RPC types become externally consumed.
- This plan does NOT add a web dashboard.

### Deferred to Follow-Up Work

- **Per-feature compat-test backfill**: each existing eidos feature gets its own `.txtar` + fixture set in subsequent PRs after this foundation lands.
- **`gorelease` wiring** for `pkg/rpc` exported surface: separate PR if/when third-party consumers appear.
- **`openapi.yaml` adoption in chimney + lighthouse**: tracked per-repo; this plan only delivers the workflow snippet.
- **U5 propagation to consuming repos** (chimney, lighthouse, lighthouse-go): this plan delivers the reusable workflow + doc; cross-repo PRs land separately.

---

## Context & Research

### Relevant Code and Patterns

- `eidos/spec - <name>.md` — existing per-feature architectural specs; U1 adds frontmatter to these.
- `specs/STATUS.md` — stale 2026-02-16 audit; U2's generator output replaces it.
- `cmd/wgmesh/main_test.go` — existing pattern for CLI integration tests via built binary; U3 wires `testscript` here.
- `pkg/crypto/envelope.go` — owns the AES-256-GCM envelope shape; U4's CapabilityVersion lives in or near this file.
- `scripts/workflows/<name>.js` + `<name>.test.js` — handler module pattern (top-of-file contract, exported helpers, `node --test`); U1 + U2 follow this for any Node-based scripts. Generator may be Go instead — see U2 Approach.
- `.github/workflows/impl-merged-close.yml` — workflow + handler module split pattern; U5 follows the same style.
- `Makefile` — existing target structure (`make build`, `make test`, `make lint`); U2/U3 add new targets here.

### Institutional Learnings

- `memory/MEMORY.md`: idempotence in infrastructure operations — applies to U2 (generator must be idempotent) and U6 (backfill should be safe to re-run).
- `project_lfg_568_shipped.md`: PR #570's L4 gate showed how to extend `impl-merged-close-handler.js` without regressing existing gates; same approach for any future bridge between this PIM and the bug-PR gate.
- `project_pulse_2026_05_08_signals.md`: Goose Implementation has been failing repeatedly on main since 2026-05-05 — agent-authoring assumption (R8) needs to survive even when Goose is unhealthy. Conventions must be human-completable too.

### External References

- Kubernetes KEP `kep.yaml` schema — frontmatter shape inspiration.
- `rogpeppe/go-internal/testscript` + CUE's `CUE_UPDATE` env-var convention — testscript bootstrap pattern.
- Tailscale `tailcfg.CapabilityVersion` — monotonic integer wire-protocol versioning model.
- `oasdiff/oasdiff-action` — drop-in GitHub Action with 450+ breaking-change rules.

---

## Key Technical Decisions

- **Generator language: Go, not Node.** The repo's primary toolchain is Go. A `cmd/status-gen/main.go` builds with `go build` like everything else and integrates cleanly with `go run` from the Makefile. Avoids adding a Node dependency for a step that runs in CI on every PR. Node `scripts/workflows/*.js` exists for GitHub-Actions-context handlers (where Octokit is the natural fit); a frontmatter walker doesn't need Octokit.
- **Generator output mode: committed-as-generated, not gitignored.** External readers browsing on GitHub see the rendered `STATUS.md` directly. CI compares the committed file against fresh generation; mismatch fails the PR. Diff churn cost accepted.
- **Frontmatter spec lives in code (Go struct + validation), not in a JSON schema file.** A single `eidosmeta` package owns the canonical schema; the generator + linter both import it. Avoids the "two places to update" problem.
- **`oasdiff` failure: hard-block.** Origin Outstanding Question — resolved hard-block. Soft-warn produces the same drift as the stale `specs/STATUS.md` failure mode. First false positive earns an explicit ignore-rule entry; that's cheaper than the policy laxity.
- **Stale-entry detection: deferred.** Origin Outstanding Question — first generator iteration ships without it. Add only if drift becomes visible in `STATUS.md` audits. YAGNI on the carrying cost.
- **Generator script location: `cmd/status-gen/`.** Origin Outstanding Question — resolved. Mirrors the `cmd/wgmesh/` pattern. Discoverable, buildable, testable like any other Go cmd.
- **`testscript` harness location: `cmd/wgmesh/main_test.go` (extend existing) plus a new `testdata/script/` directory next to it.** Existing main_test.go already builds a binary to `/tmp/wgmesh-test`; testscript can reuse that binary registration. Don't fork a parallel test entry point.
- **Per-version fixture format: protobuf binary OR raw bytes for envelopes; JSON pretty-printed for state files.** Wire envelope is binary; round-trip test is `decrypt(read(fixture)) == expected_plaintext`. State files are JSON; round-trip is `parse(read(fixture)) == ExpectedShape{...}`. Format choice flows from the artifact's natural shape, not a single uniform decision.
- **Frontmatter schema is permissive, not strict, on unknown keys.** Future ledger entries may add fields (e.g., `risk-tier`, `stable-since`); the linter rejects missing required keys but warns-only on unknown keys. Avoids breaking-change cascade when the schema evolves.
- **U6 backfill commits frontmatter only — no compat tests.** Each existing eidos file gets minimal frontmatter (`status: implemented` if clearly shipped + tested, `status: provisional` otherwise; `compat-dimensions: []`; `tracking-issue: <inferred or empty>`; `since: <empty>`). Compat tests for existing features land per-feature in follow-up PRs to keep the foundation PR review-able.

---

## Open Questions

### Resolved During Planning

- **oasdiff hard-block vs soft-warn?** Hard-block (see Key Decisions).
- **STATUS.md gitignored or committed?** Committed-as-generated (see Key Decisions).
- **Generator script location?** `cmd/status-gen/` (see Key Decisions).
- **Stale-entry detection?** Deferred — not in v1 (see Key Decisions).
- **Generator language?** Go (see Key Decisions).

### Deferred to Implementation

- Exact frontmatter field names where the origin doc was directional but not pinned (`tracking-issue` could be `tracking_issue` or `gh-issue` — pick during U1 implementation).
- Whether `STATUS.md` lives at repo root or under `docs/` — defer; U2 author picks based on which placement looks better with the existing `docs/` tree.
- testscript dependency vendoring vs `go.mod` add — defer to U3; standard `go.mod` add unless surface-level reason emerges.
- Per-fixture file naming convention inside `testdata/compat/<feature>/v0.X.Y/` (e.g., `envelope.bin` vs `envelope-v1.bin`) — defer to U4 after touching the first real fixture.

---

## Output Structure

    cmd/
      status-gen/
        main.go               # NEW (U2)
    eidos/
      spec - <name>.md        # frontmatter added by U6
      eidosmeta/              # NEW (U1)
        meta.go               # frontmatter struct + validator
        meta_test.go
        lint.go               # CLI linter entrypoint
    testdata/
      script/                 # NEW (U3)
        version.txtar         # example covering `wgmesh --version`
      compat/                 # NEW (U4)
        envelope/
          v1/
            envelope.bin
            expected.json
    .github/
      workflows/
        status-check.yml      # NEW (U2 — fails on hand-edit drift)
        oasdiff-template.yml  # NEW (U5 — reusable workflow snippet)
    Makefile                  # MODIFIED — add `status`, `update-golden` targets
    STATUS.md                 # NEW (U2 generated, committed)
    docs/
      compat-tracking.md      # NEW (U5 — adoption doc for chimney/lighthouse)

---

## Implementation Units

### U1. Frontmatter spec + CI validator on `eidos/*.md`

**Goal:** Establish the canonical YAML frontmatter schema for eidos feature specs and enforce it in CI.

**Requirements:** R1, R8, AE1.

**Dependencies:** None.

**Files:**
- Create: `eidos/eidosmeta/meta.go` — Go struct + validator
- Create: `eidos/eidosmeta/meta_test.go` — unit tests for parser + validator
- Create: `eidos/eidosmeta/lint.go` — CLI entry that walks `eidos/*.md` and exits non-zero on missing required keys
- Create: `.github/workflows/status-check.yml` — CI step that runs the linter
- Modify: `Makefile` — add `make lint-eidos` target

**Approach:**
- Define struct with required fields: `Status` (enum: `provisional`/`implementable`/`implemented`/`deprecated`), `CompatDimensions` (subset of `[cli, wire, behavior, api]`), `TrackingIssue` (int or empty), `Since` (release tag string or empty).
- Use a permissive YAML parser — unknown keys produce warnings, not errors (Key Decision).
- Validator returns structured `[]Diag` with file:line + reason; CLI prints them and exits 1 if any are at error severity.
- Detect frontmatter delimiters (`---` ... `---`) at file start; tolerate files without frontmatter only when explicitly opted out via a deny-list (no opt-out path in v1 — every `eidos/*.md` must have it once U6 lands).
- Lint runs as part of the existing `make test` target AND as its own step in `.github/workflows/status-check.yml`.

**Patterns to follow:**
- Existing `pkg/<name>` package layout — exported types, internal helpers, table-driven tests.
- Existing handler-style top-of-file contract comment from `scripts/workflows/impl-merged-close-handler.js` (transposed to Go doc-comment).

**Test scenarios:**
- Happy path: `eidos/spec - daemon-lifecycle.md` with valid frontmatter → validator returns no diags.
- Edge case: file with frontmatter but missing `status` → diag with file:line and "missing required key: status".
- Edge case: file with `status: invalid-value` → diag listing allowed enum values.
- Edge case: file with unknown key `risk-tier` → warning diag, NOT an error.
- Edge case: file without frontmatter delimiters → diag "frontmatter block required".
- Edge case: `compat-dimensions: [cli, behavior, made-up-dim]` → diag flags `made-up-dim`.
- Error path: file with malformed YAML → diag with parser error message.
- Integration (Covers AE1): full lint run across `eidos/*.md` in a fixture corpus → unhealthy fixtures produce expected diag set; CI step exits 1.

**Verification:**
- `go test ./eidos/eidosmeta/...` passes with new tests.
- `make lint-eidos` runs cleanly on a fixture eidos directory with valid frontmatter; fails with informative output on a fixture with missing keys.

### U2. STATUS.md generator (`cmd/status-gen/`) + drift check

**Goal:** Generate `STATUS.md` from frontmatter + on-disk compat artifacts, and detect hand-edits via CI drift check.

**Requirements:** R2, R10, R11, R12, F4, AE1, AE5.

**Dependencies:** U1.

**Files:**
- Create: `cmd/status-gen/main.go` — generator entrypoint
- Create: `cmd/status-gen/render.go` — markdown rendering helpers
- Create: `cmd/status-gen/render_test.go` — table-driven tests over fixture eidos sets
- Create: `STATUS.md` — generated, committed
- Modify: `Makefile` — add `make status` target invoking the generator
- Modify: `.github/workflows/status-check.yml` — add drift-detection step (regenerate, compare to committed, fail on mismatch)

**Approach:**
- Generator walks `eidos/*.md`, parses frontmatter via `eidosmeta`, cross-references compat artifacts on disk (presence of `testdata/script/<feature>*.txtar`, `testdata/compat/<feature>/`, `openapi.yaml` near the relevant repo path) per declared `compat-dimensions`.
- Rendered `STATUS.md` has a top header noting it is generated and forbidding hand-edits, then sections for: feature × dimension matrix (rows=feature, cols=cli/wire/behavior/api, cells=`✓`/`—`/`MISSING`); gap roster (provisional/implementable features without artifacts); summary stats.
- Idempotence: deterministic ordering (alphabetical by feature name), stable timestamps replaced with `git rev-parse HEAD` of the source files only when needed.
- Drift check: CI step runs `make status`, then `git diff --exit-code STATUS.md`. Non-zero exit means hand-edit drift; PR fails.

**Patterns to follow:**
- Existing `cmd/wgmesh/main.go` — Go cmd entrypoint with flag parsing + `os.Exit(code)`.
- Idempotence pattern from `memory/MEMORY.md` learning.

**Test scenarios:**
- Happy path: fixture with 3 eidos files (1 implemented + locked, 1 provisional, 1 deprecated) → output STATUS.md has correct matrix rows; gap roster lists the provisional one.
- Edge case: empty `eidos/` directory → STATUS.md still renders with header + empty matrix + "no features found" note.
- Edge case (Covers AE1): an eidos file declares `compat-dimensions: [cli]` but no `testdata/script/<feature>*.txtar` exists → matrix cell shows `MISSING`; gap roster includes the dimension.
- Edge case (Covers AE5): provisional feature with no artifacts and a tracking-issue → gap roster row links the issue (e.g., `[#571](https://github.com/...)`).
- Idempotence (Covers R12): generate twice in a row without source changes → `diff` between runs is empty.
- Error path: frontmatter validator returns errors → generator refuses to run, prints diags from U1, exits 1.
- Integration: drift-check workflow on a PR that hand-edits `STATUS.md` → CI fails with the diff posted in the step output.

**Verification:**
- `go test ./cmd/status-gen/...` passes.
- `make status` produces a non-empty `STATUS.md` against the real `eidos/` directory.
- Drift-check workflow fails on a hand-edited `STATUS.md` (verify with a deliberate test edit + revert).

### U3. testscript harness bootstrap + example `.txtar`

**Goal:** Wire `rogpeppe/go-internal/testscript` into the existing test entry point and ship one working example so future features can copy the pattern.

**Requirements:** R3, R6, R8, R9, AE2, AE6.

**Dependencies:** None (independent of U1/U2).

**Files:**
- Modify: `cmd/wgmesh/main_test.go` — add `TestMain` registering testscript binary + `testscript.Run` test
- Create: `testdata/script/version.txtar` — example covering `wgmesh --version`
- Modify: `go.mod` — add `github.com/rogpeppe/go-internal` dependency
- Modify: `Makefile` — add `make update-golden` target wrapping `WGMESH_UPDATE_GOLDEN=1 go test ./cmd/wgmesh/...`
- Modify: `CLAUDE.md` (project) — short paragraph explaining the testscript convention + regen affordance

**Approach:**
- `TestMain` calls `testscript.RunMain` to register the wgmesh binary as `wgmesh` for in-script use, then defers to `m.Run()`.
- `TestScript` function reads `testdata/script/*.txtar` files and runs them via `testscript.Run` with `UpdateScripts: os.Getenv("WGMESH_UPDATE_GOLDEN") != ""` (CUE-style env var).
- Example `version.txtar` exercises `wgmesh --version` and locks the stdout shape (e.g., `wgmesh version <semver>`). Use testscript's stdout regex matcher for the version string portion (avoid pinning to a literal version that changes per build).
- `make update-golden` is the founder-facing affordance; running it regenerates all `.txtar` golden output in place so the diff lands in the PR.

**Execution note:** Test-first — write the `.txtar` for `--version` first; expect it to fail until the harness is wired up; iterate until green.

**Patterns to follow:**
- Existing `cmd/wgmesh/main_test.go` build-binary-to-tmp pattern; testscript replaces the manual exec dance for new tests but doesn't disrupt existing ones.
- CUE's `CUE_UPDATE` env-var convention — `WGMESH_UPDATE_GOLDEN` mirrors this.

**Test scenarios:**
- Happy path (Covers AE2 partial): `go test ./cmd/wgmesh/...` runs `version.txtar`, output matches, test passes.
- Regen workflow (Covers R9, AE2): change the version-output format in `cmd/wgmesh/main.go`, run `go test` → fails with diff. Run `WGMESH_UPDATE_GOLDEN=1 go test` → `.txtar` updated in place. Run `go test` again → passes. Verify the .txtar diff matches the change.
- Edge case: `testdata/script/` empty → `TestScript` runs with zero scripts and reports "no test scripts found" (informational, not error).
- Integration (Covers AE6): a Goose / Copilot agent generates a new `.txtar` file and commits it; the next CI run picks it up automatically (no test-runner registration needed per file).

**Verification:**
- `go test ./cmd/wgmesh/...` passes including the new TestScript invocation.
- `make update-golden` overwrites the `.txtar` file with new content when behavior changes; `git diff testdata/script/version.txtar` shows the regen.
- `go.sum` and `go.mod` are clean after `go mod tidy`.

### U4. CapabilityVersion + per-version fixture replay scaffold

**Goal:** Establish the wire/on-disk compat-locking pattern with a monotonic version constant + one working fixture replay test.

**Requirements:** R4, R5, AE3.

**Dependencies:** None (independent; can land before or after U1/U2 as long as U6 doesn't claim wire dimension on this feature without these tests existing).

**Files:**
- Create or modify: `pkg/crypto/capability_version.go` — exports `EnvelopeCapabilityVersion` (current monotonic int) + comment block listing each version's added/removed semantics
- Create: `pkg/crypto/envelope_compat_test.go` — test that walks `testdata/compat/envelope/v*/` and replays each fixture
- Create: `testdata/compat/envelope/v1/envelope.bin` — binary fixture of an envelope at the current version
- Create: `testdata/compat/envelope/v1/expected.json` — companion file describing what the fixture decodes to
- Create: `testdata/compat/envelope/v1/README.md` — short note explaining how the fixture was generated and the regen recipe

**Approach:**
- Export a single `EnvelopeCapabilityVersion = 1` constant; comment block is the canonical "what each version means" registry.
- Test reads each `v<N>/envelope.bin` + `v<N>/expected.json`; calls the current decode path; asserts the decoded value matches `expected.json`. If the fixture cannot be decoded under the current code, the test fails with a clear message naming the version + suspected breakage.
- For round-trip dimensions, also re-encode the decoded value and assert byte-equality with the fixture (locks the encode path too). Skip round-trip if the encoder is intentionally non-deterministic (e.g., random nonce); document the skip reason in the test.
- `expected.json` is human-readable. When envelope shape changes, founder regenerates fixture under a NEW version directory (`v2/`), leaves `v1/` in place. The encode path bumps `EnvelopeCapabilityVersion`.
- This unit ships ONLY the envelope fixture as a working example. State-file (mesh-state.json) and gossip envelope follow in subsequent PRs once the pattern is proven.

**Patterns to follow:**
- Tailscale `tailcfg.CapabilityVersion` — comment block as the registry, integer constant as the source of truth.
- Existing `pkg/crypto/envelope.go` for the encode/decode call surface.

**Test scenarios:**
- Happy path (Covers AE3 partial): `v1/envelope.bin` decodes under current code → matches `v1/expected.json`.
- Edge case: corrupt fixture (truncated bytes) → test fails with explicit "envelope.bin at v1 failed to decode: <reason>".
- Edge case: fixture matches but `expected.json` is missing → test fails with "fixture lacks expected.json".
- Round-trip case: re-encode the decoded fixture; assert byte-equality with original. (Skip with reason if the encoder is non-deterministic for this artifact.)
- Future-version case (Covers AE3): when v2 is added, both v1 and v2 fixtures must continue to decode under current code (one-way read compat — current binary reads any committed prior version).
- Error path: `pkg/crypto` envelope decoder is broken in a way that affects v1 → test fails loud, blocking merge.

**Verification:**
- `go test ./pkg/crypto/...` passes including envelope_compat_test.go.
- The fixture file roundtrips cleanly; the README.md regen recipe is followable by another founder.
- `EnvelopeCapabilityVersion` constant is referenced from the encode path so future bumps are forced through one place.

### U5. oasdiff CI workflow snippet + adoption doc

**Goal:** Ship a reusable `oasdiff breaking` GitHub Actions workflow snippet plus a short doc explaining how chimney / lighthouse / lighthouse-go adopt it.

**Requirements:** R7, AE4.

**Dependencies:** None for this plan; consuming repos must publish `openapi.yaml` before they can adopt.

**Files:**
- Create: `.github/workflows/oasdiff-template.yml` — reusable workflow (uses `workflow_call`)
- Create: `docs/compat-tracking.md` — adoption doc covering: prerequisite (openapi.yaml present), how to call the reusable workflow from a consumer repo, hard-block policy, ignore-rule procedure for false positives
- Modify: `eidos/eidosmeta/meta.go` (from U1) — add comment noting that `compat-dimensions: [api]` features must reference an openapi.yaml in their adoption doc

**Approach:**
- Reusable workflow exposes inputs: `openapi-path` (default `openapi.yaml`), `base-ref` (default `${{ github.event.repository.default_branch }}`), `fail-on-warning` (default `false`).
- Internally calls `oasdiff/oasdiff-action@vX` with `breaking` mode comparing HEAD against base ref.
- Adoption doc is short — the wgmesh repo isn't the consumer; chimney + lighthouse + lighthouse-go each include a one-line workflow that calls the reusable one.
- Document the ignore-rule procedure: edit `oasdiff.yaml` config in the consuming repo; ignore-rules are repo-local, not in the reusable workflow.

**Patterns to follow:**
- `.github/workflows/impl-merged-close.yml` — workflow structure (top-of-file policy comment, permissions block, action versions pinned).

**Test scenarios:**
None directly testable in the wgmesh repo — this unit ships infrastructure consumed by other repos. Test expectation: validation happens at adoption time in chimney / lighthouse / lighthouse-go via their own CI runs. Document expected behavior in `docs/compat-tracking.md` with a worked example (Covers AE4): "removing a JSON response field in chimney → oasdiff flags it as breaking → PR cannot merge without explicit ignore-rule entry or a version bump."

**Verification:**
- `actionlint .github/workflows/oasdiff-template.yml` clean.
- Doc renders cleanly on GitHub preview.
- One downstream repo (start with whichever already has `openapi.yaml`) successfully calls the reusable workflow and produces a meaningful diff in a test PR. (This last step happens in a follow-up cross-repo PR; flagging here so the wgmesh PR isn't held on it.)

### U6. Backfill frontmatter on existing `eidos/*.md`

**Goal:** Add minimal valid frontmatter to every existing `eidos/*.md` file so U1's CI validator passes from day one and U2's generator has data to render.

**Requirements:** R1, AE1.

**Dependencies:** U1.

**Files:**
- Modify: every `eidos/spec - <name>.md` and `eidos/<other>.md` file — add YAML frontmatter block at top
- No new files.

**Approach:**
- Walk `eidos/*.md`. For each file:
  - Default `status: implemented` if the spec describes shipped behavior (cross-reference `pkg/<name>/CLAUDE.md` existence + recent commits in the relevant package).
  - Default `status: provisional` for specs of unbuilt or partially-built features.
  - `compat-dimensions: []` initially — no compat tests yet exist, so claiming dimensions would lie.
  - `tracking-issue: <inferred from filename or content if the spec references a GitHub issue>` — empty otherwise.
  - `since: ""` (empty) — no version is locked yet because compat-dimensions is empty.
- Founders manually review the resulting diffs; agent-authored backfill is an OK starting point but founders own the truth.
- Land as one large PR with a clear "manual review needed" callout in the description.

**Patterns to follow:**
- Existing `eidos/*.md` structure — preserve the body verbatim; only prepend the frontmatter block.

**Test scenarios:**
- Happy path: post-backfill, `make lint-eidos` (from U1) exits 0 across all files.
- Idempotence: running the backfill script a second time on already-backfilled files produces no diff.
- Edge case: files without a clear `status` mapping → backfill defaults to `provisional` and surfaces a comment in the PR description listing them for founder review.
- Integration: post-backfill, `make status` (from U2) produces a non-empty `STATUS.md` with a row per file; gap roster has the provisional ones.

**Verification:**
- `make lint-eidos` exits 0 on the full repo after the backfill PR merges.
- `make status` produces a coherent `STATUS.md`.
- No `eidos/*.md` body content was modified — only frontmatter prepended.

---

## System-Wide Impact

- **Interaction graph:** U1's linter + U2's generator + U2's drift-check workflow form a chain — if any breaks, `STATUS.md` accuracy degrades silently. U2 depends on U1's frontmatter shape; future schema changes must update both packages atomically.
- **Error propagation:** Frontmatter parser errors in U1 propagate up to CI as a hard fail. U2's generator refuses to render on lint errors (decision in Key Technical Decisions). Drift-check failure also hard-fails CI. None of these crash the daemon or affect runtime — failures are at PR-gate time only.
- **State lifecycle risks:** `STATUS.md` is committed-as-generated. Race condition possible if two PRs land in quick succession that touch different `eidos/*.md` files — both regenerate `STATUS.md` from their own base. The drift check on the second PR will fail post-merge until that PR rebases. Acceptable cost; alternative (gitignored STATUS.md) loses external-reader value.
- **API surface parity:** None. This work doesn't change daemon CLI, RPC, or wire format — it adds infrastructure to lock those surfaces. (Once founders write compat tests in follow-up PRs, those tests pin the surfaces; that's deliberate.)
- **Integration coverage:** U1 + U2 + U6 together produce the foundation. End-to-end behavior verified by AE1 and AE5 in unit tests; the live-on-main proof is "after merge, `STATUS.md` exists, `make lint-eidos` is green, the gap roster lists at least #571 (Tier 5 NAT)".
- **Unchanged invariants:** All daemon, CLI, RPC, mesh-state, and crypto-envelope behavior is unchanged by this plan. U4 adds a `EnvelopeCapabilityVersion` constant + a fixture-replay test, but the encode/decode path is not touched.

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Founders don't fill in frontmatter on new eidos files (drift mode that killed `specs/STATUS.md`) | U1's CI validator hard-blocks PRs. Pre-commit hook also enforces locally for early feedback. Convention documented in CLAUDE.md update during U3. |
| `oasdiff` false positives are noisy and erode hard-block trust | First false positive earns an explicit `oasdiff.yaml` ignore-rule entry in the consuming repo. Document the ignore procedure in `docs/compat-tracking.md`. Hard-block remains policy. |
| `testscript` golden corpus produces brittle tests if snapshots are too fine-grained | Use stdout regex matchers for non-deterministic fields (timestamps, hashes, version strings). The example `.txtar` in U3 demonstrates the pattern. |
| Per-version fixtures grow unbounded over time | Acceptable — fixtures are small (KB scale), VCS handles them well. Removing a fixture is a deliberate compat break that requires its own PR with `## Breaking Change` rationale. |
| Goose / Copilot agents generate frontmatter that's syntactically valid but semantically wrong (e.g., claims `compat-dimensions: [cli]` without committing a `.txtar`) | U2's generator surfaces the lie — the cell renders as `MISSING` in `STATUS.md` even though the dimension is declared. Drift check + visible gap roster make the inconsistency obvious at PR review. |
| `STATUS.md` drift races between concurrent PRs | Acceptable; second PR rebases and regenerates. Drift-check workflow makes the failure mode visible rather than silent. |
| Generator language choice (Go) requires founders to `go run` instead of `node scripts/...` they're used to elsewhere | Wrapper `make status` target hides the language choice. Founders interact with the Makefile, not the generator binary. |

---

## Documentation / Operational Notes

- Update `CLAUDE.md` (project root) as part of U3 with: paragraph explaining the testscript convention + `WGMESH_UPDATE_GOLDEN` env var + how new features should claim compat dimensions.
- Update `.github/copilot-instructions.md` as part of U6 with: section explaining frontmatter requirement so Goose / Copilot agents author it on first try.
- Update `memory/MEMORY.md` (out of scope; user-managed, but flag for follow-up): add an entry under "CI/workflow gaps" pointing to the new feature-ledger PIM as the post-PR-#570 next layer of compound-engineering hygiene.
- No rollout / monitoring concerns — this is dev-time infrastructure, not a runtime change. Failures surface at PR gate, not in production.

---

## Sources & References

- **Origin document:** [docs/brainstorms/2026-05-08-feature-ledger-pim-requirements.md](docs/brainstorms/2026-05-08-feature-ledger-pim-requirements.md)
- Related code: `eidos/spec - <name>.md`, `cmd/wgmesh/main_test.go`, `pkg/crypto/envelope.go`, `Makefile`, `.github/workflows/impl-merged-close.yml`
- Related PRs: #570 (auto-verify e2e — adjacent infrastructure), #582 (Copilot stuck-workflow fix — same workflow-handler split pattern this plan follows for U2 drift check)
- Related issues: #571 (Tier 5 NAT — first concrete entry in the gap roster once U2 lands)
- External docs:
  - Kubernetes KEP `kep.yaml`: https://github.com/kubernetes/enhancements
  - testscript / CUE_UPDATE convention: https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript
  - Tailscale CapabilityVersion: https://github.com/tailscale/tailscale (search `CapabilityVersion` in `tailcfg`)
  - oasdiff: https://github.com/oasdiff/oasdiff
