# Specification: Issue #60

## Classification
feature

## Deliverables
code

## Problem Analysis

Docker builds currently take ~8 minutes for every run, including PR validation builds that never push an image. The bottleneck is QEMU-emulated multi-arch builds (`linux/amd64,linux/arm64,linux/arm/v7`) running even on PRs where only a compile check is needed.

This task involves only build infrastructure changes (GitHub Actions workflow, Dockerfile, .dockerignore) — no Go code changes are required.

## Proposed Approach

### 1. Workflow changes (`docker-build.yml`)

- Skip QEMU setup on PRs (`if: github.event_name != 'pull_request'`)
- Build only `linux/amd64` on PRs; keep full multi-arch on push/tag:
  ```yaml
  platforms: ${{ github.event_name == 'pull_request' && 'linux/amd64' || 'linux/amd64,linux/arm64,linux/arm/v7' }}
  ```
- Bump `docker/build-push-action` from v5 to v6

### 2. Dockerfile improvements

- Add `# syntax=docker/dockerfile:1.7` directive to enable BuildKit features
- Use `--mount=type=cache,target=/go/pkg/mod` on `go mod download` for module cache
- Use `--mount=type=cache,target=/go/pkg/mod` and `--mount=type=cache,target=/root/.cache/go-build` on `go build` for build cache
- Replace `-a -installsuffix cgo` with `-trimpath` (the former is redundant with `CGO_ENABLED=0`, the latter improves reproducibility)
- Remove redundant `apk update` (already using `--no-cache`)
- Remove redundant `chmod +x` (binary is already executable from `go build`)

### 3. `.dockerignore` update

- Add `.github` to exclude workflows/templates from Docker build context

## Affected Files

- `.github/workflows/docker-build.yml`
- `Dockerfile`
- `.dockerignore`

## Test Strategy

1. Open the PR and verify the Docker build on the PR runs in ~1-2 min (amd64 only, no QEMU)
2. After merge, verify a push build still produces multi-arch images
3. Verify GHA cache hits on subsequent builds

## Estimated Complexity
low
