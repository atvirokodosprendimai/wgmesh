---
tldr: A post-write hook in wgmesh reverts main.go and .github/workflows/*.yml to older versions; new handlers in separate .go files are unaffected
---

# Learning: wgmesh linter hook reverts main.go and workflow files

## What happens

After any Write or Edit to `cmd/chimney/main.go` or `.github/workflows/*.yml`,
a hook fires and overwrites the file with a different version (appears to be a
gofmt/linter that re-applies a cached or pre-existing state). The working tree
reverts; committed content is preserved.

## Effect

- Edits to `main.go` or workflow files in the working tree may not survive to
  the next tool call — the hook runs between invocations.
- Commits capture the correct version if staged and committed immediately after
  the write, before the hook fires a second time.
- `git checkout HEAD -- <file>` restores the committed (correct) version after
  a hook revert.

## Workaround

**Put new handlers in separate `.go` files** (same package `main`).
The hook only targets specific existing files. Files like `deploy.go` and
`cache_invalidate.go` are not reverted.

For workflow file edits: commit immediately after editing, then restore with
`git checkout HEAD -- .github/workflows/<file>.yml` if the working tree is dirty.

## Observed in

- Phase 2-5 of chimney integration (PR #351)
- Every session that touches main.go or chimney-deploy.yml
