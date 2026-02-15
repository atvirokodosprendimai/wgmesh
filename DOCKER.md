# Docker and CI/CD Setup

This document describes the Docker and CI/CD setup for the wgmesh project.

## Overview

The repository is configured to automatically build and publish multi-architecture Docker images to GitHub Container Registry (ghcr.io) using GitHub Actions.

## Supported Architectures

The Docker images are built for the following platforms:
- `linux/amd64` - Standard 64-bit x86 processors (Intel/AMD)
- `linux/arm64` - ARM 64-bit processors (Apple Silicon, AWS Graviton, Raspberry Pi 4+)
- `linux/arm/v7` - ARM 32-bit processors (Raspberry Pi 2/3)

## CI/CD Pipeline

### Workflow Configuration

The workflow (`.github/workflows/docker-build.yml`) is triggered by:
- **Push to the repository's default branch** (typically `main` or `master`): Builds and pushes images tagged with branch name
- **Git tags** matching `v*.*.*`: Builds and pushes images with semantic version tags and `latest` (for stable releases only)
- **Pull requests**: Builds images but doesn't push them (for validation)
- **Manual dispatch**: Can be triggered manually from GitHub Actions UI

### Image Tags

Images are automatically tagged with:
- `latest` - Latest stable release (semantic version tags without pre-release identifiers like `-beta` or `-rc1`)
- `<branch-name>` - Branch-specific builds
- `<version>` - Semantic version (e.g., `v1.2.3`)
- `<major>.<minor>` - Major and minor version (e.g., `1.2`)
- `<major>` - Major version only (e.g., `1`)
- `sha-<commit-sha>` - Git commit SHA
- `pr-<number>` - Pull request builds

### Using the Images

Pull the latest image:
```bash
docker pull ghcr.io/nycterent/wgmesh:latest
```

Pull a specific version:
```bash
docker pull ghcr.io/nycterent/wgmesh:v1.0.0
```

Pull a specific architecture:
```bash
docker pull --platform linux/arm64 ghcr.io/nycterent/wgmesh:latest
```

## Dockerfile

The Dockerfile uses a multi-stage build:

1. **Builder stage** (golang:1.23-alpine):
   - Installs build dependencies (git)
   - Downloads Go dependencies
   - Builds a statically-linked binary with optimized flags

2. **Runtime stage** (alpine:3.19):
   - Minimal Alpine Linux base
   - Installs WireGuard tools and dependencies
   - Copies the binary from builder stage
   - Runs as root by default for WireGuard operations
   - Exposes UDP port 51820 (WireGuard default)

### Security Features

- Static binary with no CGO dependencies
- Runs as root by default (required for WireGuard network operations)
- Minimal attack surface (Alpine base)
- Only necessary runtime dependencies included

## Local Development

### Building Locally

Build for your current platform:
```bash
docker build -t wgmesh:local .
```

Build for a specific platform:
```bash
docker buildx build --platform linux/arm64 -t wgmesh:arm64 .
```

Build for multiple platforms:
```bash
docker buildx build \
  --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t wgmesh:multi \
  --load .
```

### Testing the Image

Run help command:
```bash
docker run --rm wgmesh:local --help
```

Run with state file:
```bash
docker run --rm \
  -v $(pwd)/data:/data \
  wgmesh:local -state /data/mesh-state.json -list
```

Run in privileged mode with host network (for WireGuard functionality):
```bash
docker run --rm \
  --privileged \
  --network host \
  -v $(pwd)/data:/data \
  wgmesh:local join --secret "wgmesh://v1/<secret>"
```

## Maintenance

### Updating Dependencies

To update base images or Go version:
1. Edit the `FROM` lines in `Dockerfile`
2. Test the build locally
3. Commit and push to trigger CI/CD

### Troubleshooting

**Build fails with network errors:**
- Check if Alpine mirrors are accessible
- Consider using alternative mirrors in Dockerfile

**Multi-arch build fails:**
- Ensure QEMU and buildx are properly set up in the workflow
- Check GitHub Actions logs for platform-specific errors

**Image size too large:**
- Review `.dockerignore` to exclude unnecessary files
- Verify multi-stage build is working correctly
- Consider additional build optimizations

## References

- [Docker Buildx Documentation](https://docs.docker.com/buildx/working-with-buildx/)
- [GitHub Actions Docker Documentation](https://docs.github.com/en/actions/publishing-packages/publishing-docker-images)
- [WireGuard Tools](https://www.wireguard.com/install/)
