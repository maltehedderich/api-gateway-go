# API Gateway

> [!WARNING]
> **EXPERIMENTAL CODE - NOT PRODUCTION READY**
>
> All code in this repository was generated as an experiment with Claude Code Web and **MUST BE CAREFULLY REVIEWED** before any use in production environments. This is a proof-of-concept and has not undergone the necessary security audits, testing, and validation required for production deployment.

A high-performance API Gateway written in Go that provides essential cross-cutting concerns including request logging, session-based authorization, and rate limiting to protect backend services.

## Overview

This API Gateway acts as a unified entry point for backend services, implementing:

- **Request/Response Logging**: Comprehensive structured logging with correlation IDs for distributed tracing
- **Session Token Authorization**: JWT-based authentication and authorization with role-based access control
- **Rate Limiting**: Token bucket algorithm for protecting backend services from overload
- **Health Checks**: Liveness and readiness probes for orchestration platforms
- **Graceful Shutdown**: Connection draining and clean shutdown handling
- **TLS Support**: HTTPS with configurable cipher suites and modern TLS versions
- **Observability**: Prometheus metrics, structured logging, and distributed tracing support

## Architecture

The gateway follows a middleware-based architecture with the following components:

- **HTTP Server**: Handles connections, TLS termination, and HTTP protocol processing
- **Middleware Chain**: Ordered execution of cross-cutting concerns (logging, auth, rate limiting)
- **Router**: Maps incoming requests to backend services
- **Configuration**: Hot-reloadable configuration from YAML/JSON files with environment overrides
- **Health Manager**: Manages health checks for readiness and liveness probes

## Project Structure

```
.
├── cmd/
│   └── gateway/          # Main application entry point
│       └── main.go
├── internal/             # Private application code
│   ├── config/          # Configuration management
│   ├── logger/          # Structured logging
│   ├── server/          # HTTP server implementation
│   ├── middleware/      # Middleware components
│   ├── router/          # Request routing
│   └── health/          # Health check handlers
├── configs/             # Example configuration files
│   ├── config.dev.yaml
│   ├── config.staging.yaml
│   └── config.prod.yaml
└── API_GATEWAY_DESIGN_SPEC.md

```

## Quick Start

### Prerequisites

- Go 1.21 or later
- (Optional) Redis for distributed rate limiting

### Installation

```bash
# Clone the repository
git clone https://github.com/maltehedderich/api-gateway-go.git
cd api-gateway-go

# Download dependencies
go mod download

# Build the application
go build -o bin/gateway ./cmd/gateway

# Run with development configuration
./bin/gateway -config configs/config.dev.yaml
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...
```

### Running Locally

For local development without TLS:

```bash
# Run with development config (no TLS, debug logging)
go run ./cmd/gateway -config configs/config.dev.yaml
```

The gateway will start on:
- HTTP: `http://localhost:8080`
- Health: `http://localhost:8080/_health`
- Metrics: `http://localhost:9090/metrics`

## Configuration

The gateway supports configuration from multiple sources with the following precedence:

1. Command-line flags (highest priority)
2. Environment variables (prefix: `GATEWAY_`)
3. Configuration file (YAML or JSON)
4. Default values (lowest priority)

### Example Configuration

```yaml
server:
  http_port: 8080
  https_port: 8443
  tls_enabled: false
  read_timeout: 30s
  write_timeout: 30s

logging:
  level: info
  format: json
  output: stdout

authorization:
  enabled: true
  cookie_name: session_token
  jwt_signing_algorithm: RS256
  jwt_public_key_file: /path/to/public.pem

rate_limit:
  enabled: true
  backend: redis
  redis_addr: localhost:6379
  failure_mode: fail-closed

routes:
  - path_pattern: /api/v1/users
    methods: [GET, POST]
    backend_url: http://user-service:8080
    auth_policy: authenticated
```

### Environment Variable Overrides

You can override any configuration value using environment variables:

```bash
export GATEWAY_HTTP_PORT=9000
export GATEWAY_LOG_LEVEL=debug
export GATEWAY_TLS_ENABLED=true
export GATEWAY_REDIS_ADDR=redis:6379

./bin/gateway -config configs/config.prod.yaml
```

## Features

### Logging

- **Structured Logging**: JSON format for machine processing
- **Multiple Log Levels**: DEBUG, INFO, WARN, ERROR, FATAL
- **Correlation IDs**: Automatic generation and propagation for request tracing
- **Field Sanitization**: Automatic redaction of sensitive fields (passwords, tokens)
- **Component-Specific Levels**: Different log levels per component

Example log entry:
```json
{
  "timestamp": "2025-11-16T10:30:00Z",
  "level": "INFO",
  "component": "http",
  "correlation_id": "550e8400-e29b-41d4-a716-446655440000",
  "message": "request completed",
  "fields": {
    "method": "GET",
    "path": "/api/v1/users",
    "status": 200,
    "duration_ms": 45
  }
}
```

### Authorization

- **JWT Token Validation**: Cryptographic signature verification
- **Role-Based Access Control**: Route-specific role requirements
- **Token Revocation**: Support for immediate token invalidation
- **Flexible Policies**: Public, authenticated, role-based, and permission-based policies
- **Caching**: Optional caching of authorization decisions

### Rate Limiting

- **Token Bucket Algorithm**: Allows bursts while maintaining average rate
- **Multiple Keying Strategies**: By IP, user ID, route, or composite keys
- **Distributed State**: Redis backend for multi-instance deployments
- **Configurable Failure Modes**: Fail-open or fail-closed when rate limiter unavailable
- **Rate Limit Headers**: Standard X-RateLimit headers in responses

### Health Checks

- **Liveness Probe** (`/_health/live`): Indicates if the application is running
- **Readiness Probe** (`/_health/ready`): Indicates if ready to serve traffic
- **Extensible Checks**: Register custom health checks for dependencies

## API Endpoints

### Health Endpoints

- `GET /_health` - General health status with all checks
- `GET /_health/ready` - Readiness probe (200 if ready, 503 if not)
- `GET /_health/live` - Liveness probe (always 200 if running)

### Metrics Endpoint

- `GET /metrics` - Prometheus metrics (default port: 9090)

## Development

### Phase 1 Implementation Status

✅ **Task 1.1**: Project Setup - Go module and directory structure
✅ **Task 1.2**: Configuration System - YAML/JSON loading with env overrides
✅ **Task 1.3**: Logging Infrastructure - Structured logging with correlation IDs
✅ **Task 1.4**: HTTP Server Foundation - HTTP/HTTPS server with graceful shutdown

### Next Phases

- **Phase 2**: Routing and Middleware - Request proxying and circuit breaker
- **Phase 3**: Authorization - JWT validation and policy evaluation
- **Phase 4**: Rate Limiting - Token bucket implementation with Redis
- **Phase 5**: Observability - Metrics and distributed tracing
- **Phase 6**: Security Hardening - TLS configuration and security headers
- **Phase 7**: Testing - Unit, integration, and performance tests
- **Phase 8**: Deployment - Containerization and Kubernetes manifests

### Building for Production

```bash
# Build with version information
VERSION=$(git describe --tags --always --dirty)
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse HEAD)

go build -ldflags="-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
  -o bin/gateway ./cmd/gateway
```

## Docker

### Building Docker Image

```bash
# Build Docker image
docker build -t api-gateway:latest .

# Run container
docker run -p 8080:8080 -v $(pwd)/configs:/etc/gateway/configs api-gateway:latest \
  -config /etc/gateway/configs/config.prod.yaml
```

### Docker Compose

```bash
# Start gateway with dependencies
docker-compose up -d

# View logs
docker-compose logs -f gateway
```

## Monitoring

### Metrics

The gateway exposes Prometheus metrics on the `/metrics` endpoint:

- **Request Metrics**: Total requests, duration, size
- **Authorization Metrics**: Auth attempts, failures, cache hits
- **Rate Limit Metrics**: Rate limit checks, exceeded events
- **System Metrics**: CPU, memory, goroutines

### Logging

Configure centralized logging by setting the output to a logging aggregator:

```yaml
logging:
  level: info
  format: json
  output: stdout  # Captured by container orchestrator
```

## Security Considerations

- **TLS 1.2+**: Minimum TLS version enforced
- **Strong Cipher Suites**: Only modern, secure cipher suites enabled
- **Cookie Security**: HttpOnly, Secure, SameSite attributes
- **HSTS**: HTTP Strict Transport Security headers
- **Sensitive Data**: Automatic sanitization in logs
- **Input Validation**: Request size limits and header validation

## Performance

### Target Performance Characteristics

- **Latency**: < 10ms p50, < 50ms p99 (gateway overhead only)
- **Throughput**: 10,000+ requests/second per instance
- **Memory**: 512MB - 1GB per instance
- **CPU**: 2-4 cores per instance

### Optimization Tips

1. Enable authorization decision caching for reduced latency
2. Use Redis for rate limiting in multi-instance deployments
3. Tune log sampling for high-volume endpoints
4. Configure appropriate timeouts for backend services

## Troubleshooting

### Common Issues

**Gateway won't start**
- Check configuration file syntax (YAML/JSON)
- Verify TLS certificate files exist and are readable
- Ensure ports are not already in use

**Authorization failures**
- Verify JWT public key is correctly configured
- Check token expiration and clock skew tolerance
- Review authorization logs for detailed error messages

**Rate limiting not working**
- Verify Redis connectivity (if using Redis backend)
- Check rate limit configuration and key templates
- Review rate limit metrics for errors

### Debug Mode

Enable debug logging for troubleshooting:

```bash
export GATEWAY_LOG_LEVEL=debug
./bin/gateway -config configs/config.dev.yaml
```

## Contributing

Please see the [API Gateway Design Specification](API_GATEWAY_DESIGN_SPEC.md) for detailed architecture and implementation guidelines.

## License

Copyright © 2025. All rights reserved.

## References

- [API Gateway Design Specification](API_GATEWAY_DESIGN_SPEC.md)
- [Go HTTP Server Best Practices](https://go.dev/doc/articles/wiki/)
- [JWT Best Practices](https://datatracker.ietf.org/doc/html/rfc8725)
- [Rate Limiting Patterns](https://redis.io/docs/reference/patterns/rate-limiting/)
- [Prometheus Metrics](https://prometheus.io/docs/practices/naming/)
