# Multi-stage build for wgmesh
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o wgmesh .

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk update && apk add --no-cache \
    wireguard-tools \
    iptables \
    iproute2 \
    openssh-client \
    ca-certificates && \
    rm -rf /var/cache/apk/*

# Copy binary from builder
COPY --from=builder /build/wgmesh /usr/local/bin/wgmesh

# Create directory for state files
RUN mkdir -p /data

# Set working directory
WORKDIR /data

# Make binary executable
RUN chmod +x /usr/local/bin/wgmesh

# Expose WireGuard port
EXPOSE 51820/udp

# Run as non-root user for better security
RUN addgroup -g 1000 wgmesh && \
    adduser -D -u 1000 -G wgmesh wgmesh && \
    chown -R wgmesh:wgmesh /data

USER wgmesh

ENTRYPOINT ["/usr/local/bin/wgmesh"]
CMD ["--help"]
