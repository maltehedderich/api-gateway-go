# Build stage
FROM golang:alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'docker') -X main.buildTime=$(date -u '+%Y-%m-%d_%H:%M:%S') -X main.gitCommit=$(git rev-parse HEAD 2>/dev/null || echo 'unknown')" \
    -o /app/bin/gateway \
    ./cmd/gateway

# Runtime stage
FROM alpine:3.19

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 gateway && \
    adduser -D -u 1000 -G gateway gateway

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/gateway /app/gateway

# Copy default configs
COPY --from=builder /app/configs /app/configs

# Change ownership
RUN chown -R gateway:gateway /app

# Switch to non-root user
USER gateway

# Expose ports
EXPOSE 8080 8443 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/_health/live || exit 1

# Set entrypoint
ENTRYPOINT ["/app/gateway"]

# Default command (can be overridden)
CMD ["-config", "/app/configs/config.dev.yaml"]
