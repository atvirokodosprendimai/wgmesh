# syntax=docker/dockerfile:1.7
# Multi-stage build for wgmesh
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy source code
COPY . .

# Build the application
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" -o wgmesh .

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    wireguard-tools \
    iptables \
    iproute2 \
    ca-certificates

# Copy binary from builder
COPY --from=builder /build/wgmesh /usr/local/bin/wgmesh

# Create directory for state files
RUN mkdir -p /data

# Set working directory
WORKDIR /data

# Expose WireGuard port
EXPOSE 51820/udp

# User setup
# Note: wgmesh performs privileged networking operations (e.g., WireGuard
# interface management, route updates) that require NET_ADMIN and related
# capabilities. To ensure these operations succeed, the container runs as
# root by default. The wgmesh user/group are created for optional use if
# you want to run non-privileged operations with explicit capability grants.
RUN addgroup -g 1000 wgmesh && \
    adduser -D -u 1000 -G wgmesh wgmesh && \
    chown -R wgmesh:wgmesh /data

# Run as root by default for WireGuard operations
# USER wgmesh

ENTRYPOINT ["/usr/local/bin/wgmesh"]
CMD ["--help"]
