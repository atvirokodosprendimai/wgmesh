---
title: "GoReleaser picks wrong tag when multiple tags point to same commit"
category: integration-issues
date: 2026-03-13
tags: [goreleaser, github-actions, release, ci-cd, tags]
modules: [.github/workflows/release.yml]
---

## Problem

`release.yml` workflow failed with `422 Validation Failed: already_exists` when trying to upload assets.
GoReleaser detected `v0.2.1-rc4` instead of `v0.2.1` because both tags pointed to the same commit (`2a5b371`).
It tried to upload to the existing rc4 release, which already had assets.

## Root Cause

The `release.yml` workflow verifies that HEAD is tagged using `git describe --tags --exact-match HEAD`.
When multiple tags point to the same commit, this check still passes — it confirms *a* tag exists, but does not validate which tag was found against the intended release tag.
GoReleaser then uses `git describe --tags` to determine the release name and asset filenames, which non-deterministically picks one of the matching tags (typically the last in alphabetical/refname order).
With both `v0.2.1` and `v0.2.1-rc4` on the same commit, GoReleaser chose rc4 and tried to upload assets to the existing rc4 release.

## Solution

Delete rc tags before tagging the final release:

```bash
# Clean up rc tags before final release
# Uses || true so the script is safe under set -e
for tag in v0.2.1-rc1 v0.2.1-rc2 v0.2.1-rc3 v0.2.1-rc4; do
    gh release delete "$tag" --yes 2>/dev/null || true
    git push origin --delete "$tag" 2>/dev/null || true
    git tag -d "$tag" 2>/dev/null || true
done

# Then tag and push final
git tag v0.2.1
git push origin v0.2.1
```

After cleaning up rc tags, `gh workflow run release.yml -f tag=v0.2.1 -f skip_integration_check=true` succeeded and published correct `wgmesh_0.2.1_*` assets.

## Prevention

Before tagging a final release, delete all `-rc*` tags on the same commit.
Consider automating this in the release workflow: add a step that removes rc tags matching the release version before GoReleaser runs.
Additionally, the tag verification step should validate that the *exact* intended tag matches the describe output — not just that *some* tag exists on HEAD:

```bash
DESCRIBED=$(git describe --tags --exact-match HEAD 2>/dev/null)
if [ "$DESCRIBED" != "$TAG" ]; then
  echo "::error::Expected tag $TAG but found $DESCRIBED"
  exit 1
fi
```
