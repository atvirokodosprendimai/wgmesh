# Contributing to wgmesh

This project uses an AI-assisted development pipeline. Here's how it works and what you need to do.

## The Pipeline

Every change flows through these stages:

```
Issue → Spec → Review Spec → Build → Review Code → Merge
```

Most of the work is automated. Your job as a maintainer is to **review at two checkpoints**.

## Board Columns

The [project board](https://github.com/orgs/atvirokodosprendimai/projects/1) shows where everything is. Columns move automatically based on labels — never drag cards manually.

| Column | What's happening | Who acts | What to do |
|--------|-----------------|----------|------------|
| **Triage** | New issue arrived | Maintainer | Add `type:` and `complexity:` labels. Copilot will auto-triage if you don't. |
| **Spec in Progress** | Copilot is writing a spec PR | Nobody | Wait. Copilot is drafting a technical spec. |
| **Review Spec** | Spec PR is ready | Maintainer | Read the spec PR. Approve it or request changes. |
| **Building** | Goose is implementing | Nobody | Wait. Goose is writing code based on the approved spec. |
| **Review Code** | Implementation PR is ready | Maintainer | Review the code. Approve and merge, or request changes. |
| **Done** | Merged or closed | Nobody | Nothing to do. |

**TL;DR** — You only need to act on items in **Triage**, **Review Spec**, and **Review Code**.

## Step by Step

### 1. File an Issue

Open an issue describing the bug or feature. Be specific about what's wrong or what you want.

### 2. Triage (automatic or manual)

- Copilot auto-triages new issues and adds labels
- If you want to override, manually set `type: bug`/`type: feature`/etc. and `complexity: low`/`medium`/`high`

### 3. Request a Spec

Comment `/spec` on the issue. This triggers Copilot to:
- Create a spec branch
- Write a technical specification as a PR
- Label it `spec-ready` when done

### 4. Review the Spec

The spec PR appears in **Review Spec** on the board. Review it:
- **Approve** → triggers Goose to start building
- **Request changes** → Copilot revises the spec

### 5. Wait for Implementation

Goose reads the approved spec and creates an implementation PR. This is automatic.

### 6. Review the Code

The implementation PR appears in **Review Code** on the board. Review it:
- **Approve and merge** → done
- **Request changes** → Goose retries based on your feedback

## Labels That Drive the Pipeline

These labels are applied by workflows. Don't remove them manually unless you know what you're doing.

| Label | Meaning | Applied by |
|-------|---------|-----------|
| `needs-triage` | Issue needs classification | Auto on new issues |
| `copilot-triaging` | Copilot is analyzing the issue | Triage workflow |
| `copilot-revising` | Copilot is revising a spec | Review feedback |
| `spec-ready` | Spec PR ready for review | Copilot |
| `approved-for-build` | Spec approved, implementation starting | Approve workflow |
| `goose-implementation` | Goose is building | Goose workflow |
| `needs-review` | Implementation PR ready for code review | Goose workflow |

## Manual Development

If you want to skip the AI pipeline and contribute directly:

1. Fork the repo
2. Create a feature branch from `main`
3. Make your changes
4. Run tests: `go test ./...`
5. Run linter: `golangci-lint run`
6. Open a PR against `main`

## Building

```bash
go build -o wgmesh .
```

## Testing

```bash
go test ./...
go test -race ./...  # with race detector
```
