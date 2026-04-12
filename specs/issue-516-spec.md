# Specification: Issue #516

## Classification
documentation

## Deliverables
documentation

## Problem Analysis

`CONTRIBUTING.md` already exists at the repo root but is minimal (~100 lines). It covers the AI pipeline flow and a brief "Manual Development" section but is missing several sections required by the acceptance criteria:

- No **Prerequisites** section listing required tools and versions
- No **Linting** section with exact commands
- No explicit reference to `make` targets from the Makefile (`make build`, `make test`, `make fmt`, `make lint`, `make deps`)
- No links to `CLAUDE.md` (agent coding guide) or `.goosehints` (Goose context hints)
- The spec → build → impl → merge pipeline is described but not in a self-contained "AI Contributors" section
- Copilot and Goose bot conventions are not documented

The fix is to **replace `CONTRIBUTING.md`** with a more complete version that satisfies every acceptance criterion while keeping the existing content (pipeline table, labels table, manual development, building, testing).

## Implementation Tasks

### Task 1: Replace `CONTRIBUTING.md` with the full content below

Overwrite the file `CONTRIBUTING.md` at the repository root with **exactly** the following content (no additions, no omissions):

```markdown
# Contributing to wgmesh

This project uses an AI-assisted development pipeline. Here's how it works and what you need to do.

## Prerequisites

Before contributing, install the following tools:

| Tool | Version | Install |
|------|---------|---------|
| Go | ≥ 1.25.5 | https://go.dev/dl/ |
| golangci-lint | latest | https://golangci-lint.run/usage/install/ |
| WireGuard tools (`wg`) | any | OS package manager |
| Make | any | OS package manager |

Verify your Go version:

```bash
go version   # should print go1.25.5 or newer
```

Clone the repository and download dependencies:

```bash
git clone https://github.com/atvirokodosprendimai/wgmesh.git
cd wgmesh
make deps
```

## Building

```bash
make build          # produces ./wgmesh binary
```

Or without Make:

```bash
go build -o wgmesh .
```

## Testing

```bash
make test                     # go test ./...
go test -race ./...           # required when changing concurrency code
go test ./pkg/crypto/...      # single package
go test ./pkg/daemon -run TestPeerStore -v  # single test by name
```

Generate and view a coverage report:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Linting

```bash
make lint           # golangci-lint run
make fmt            # go fmt ./...
```

If `golangci-lint` is not installed, see https://golangci-lint.run/usage/install/.

## Available Makefile Targets

| Target | Command | Description |
|--------|---------|-------------|
| `make build` | `go build -o wgmesh` | Compile the binary |
| `make install` | `go install` | Install to `$GOPATH/bin` |
| `make test` | `go test ./...` | Run all tests |
| `make fmt` | `go fmt ./...` | Format source code |
| `make lint` | `golangci-lint run` | Run the linter |
| `make deps` | `go mod download && go mod tidy` | Sync dependencies |
| `make clean` | `rm -f wgmesh mesh-state.json` | Remove build artefacts |

## PR Conventions

1. Fork the repo and create a feature branch from `main` (`git checkout -b my-feature`).
2. Make your changes, following the code style in `CLAUDE.md`.
3. Run `make fmt && make lint && make test` — all must pass before opening a PR.
4. Run `go test -race ./...` if you changed any concurrency code.
5. Open a PR against `main`. Write a clear title and description explaining *why* the change is needed.
6. Reference the related issue in the PR body (`Closes #NNN` or `Addresses #NNN`).
7. Wait for CI to go green and a maintainer review.

## The AI-Assisted Pipeline

Every change in this project flows through a spec-first pipeline:

```
Issue → Spec PR → Review Spec → Implementation PR → Review Code → Merge
```

### Stage details

| Stage | Who acts | What to do |
|-------|----------|------------|
| **Triage** | Maintainer (or Copilot) | Add `type:` and `complexity:` labels |
| **Spec in Progress** | Copilot (automated) | Writing a technical spec PR |
| **Review Spec** | Maintainer | Read spec PR, approve or request changes |
| **Building** | Goose (automated) | Implementing based on approved spec |
| **Review Code** | Maintainer | Review implementation PR, approve and merge |
| **Done** | — | Merged or closed |

### Labels that drive the pipeline

| Label | Meaning | Applied by |
|-------|---------|-----------|
| `needs-triage` | Issue needs classification | Auto on new issues |
| `copilot-triaging` | Copilot is analysing the issue | Triage workflow |
| `copilot-revising` | Copilot is revising a spec | Review feedback |
| `spec-ready` | Spec PR ready for review | Copilot |
| `approved-for-build` | Spec approved, implementation starting | Approve workflow |
| `goose-implementation` | Goose is building | Goose workflow |
| `needs-review` | Implementation PR ready for code review | Goose workflow |

### Requesting a spec

Comment `/spec` on any open issue to trigger Copilot to write a specification PR.  
Comment `/build` on an approved spec PR to trigger Goose to start implementing.

## AI Contributor Conventions

### Copilot (spec agent)

- Triggered by `/spec` comment or the `approved-for-triage` label on issues
- Creates a branch `copilot/<slug>` and opens a PR with `specs/issue-NNN-spec.md`
- Reads `CLAUDE.md` for code conventions before writing any spec
- Must NOT write implementation code — spec documents only
- Spec PRs use the title prefix `spec: Issue #NNN - <brief description>`
- Uses `Addresses #NNN` (not `Closes`) so the original issue stays open until the implementation PR merges

### Goose (implementation agent)

- Triggered by the `approved-for-build` label on an approved spec PR
- Reads the spec file and `CLAUDE.md` before writing any code
- Also reads `.goosehints` for project-specific hints (key file locations, conventions)
- Creates a branch `goose/<slug>` and opens an implementation PR
- If CI fails or a reviewer requests changes, Goose retries automatically

### Reference files for AI agents

| File | Purpose |
|------|---------|
| [`CLAUDE.md`](./CLAUDE.md) | Coding conventions, architecture overview, build & test commands |
| [`.goosehints`](./.goosehints) | Concise context hints for the Goose implementation agent |
| `specs/` directory | All spec documents; implemented specs move to `specs/implemented/` |

## Manual Development (skip the AI pipeline)

If you want to contribute directly without the AI pipeline:

1. Fork the repo
2. Create a feature branch from `main`
3. Make your changes following the conventions in `CLAUDE.md`
4. Run `make fmt && make lint && make test && go test -race ./...`
5. Open a PR against `main`
```

## Affected Files

- **Modified:** `CONTRIBUTING.md` — replaced with the expanded version above

No other files are changed.

## Test Strategy

No automated tests required for documentation changes. Verify manually:

1. `CONTRIBUTING.md` renders correctly in GitHub Markdown (all tables, code blocks, and links).
2. The link `./CLAUDE.md` in the "Reference files" table resolves to the file at the repo root.
3. The link `./.goosehints` resolves to the file at the repo root.
4. `make build`, `make test`, `make fmt`, `make lint`, and `make deps` all exist in the `Makefile` and behave as documented.

## Estimated Complexity
low
