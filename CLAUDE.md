# API Gateway Project - Development Guide

## Project Overview

This is an API Gateway service implemented in Go that acts as a unified entry point for backend services. The gateway provides essential cross-cutting concerns including request logging, session-based authorization, and rate limiting to protect backend services and provide a consistent client experience.

**Key Technologies:**
- **Language:** Go (Golang)
- **Architecture:** Stateless HTTP reverse proxy with middleware-based architecture
- **Authorization:** Session token validation (JWT-based)
- **Rate Limiting:** Token bucket algorithm with Redis backend
- **Observability:** Prometheus metrics, structured logging (JSON)
- **Deployment:** Containerized (Docker), Kubernetes-ready

## Architecture

### High-Level Components

1. **HTTP Server** - Handles TCP connections, TLS termination, HTTP protocol processing
2. **Middleware Chain** - Ordered execution of cross-cutting concerns (logging, auth, rate limiting)
3. **Routing Layer** - Maps incoming requests to backend service endpoints
4. **Proxy Layer** - Forwards requests to backend services and returns responses
5. **Configuration Layer** - Manages gateway configuration from multiple sources
6. **Observability Layer** - Collects and exposes metrics, logs, and health information

### Request Flow

```
Client → HTTPS → Gateway → Logging → Correlation ID → Auth → Rate Limit → Router → Proxy → Backend
                    ↓                                                                          ↓
                Response ← Logging ← Response Headers ← Proxy ← Backend Response ←────────────┘
```

### Middleware Execution Order

Pre-Request (before backend call):
1. Panic recovery
2. Request ID generation/extraction
3. Request logging
4. Request metrics collection
5. Session validation
6. Authorization check
7. Rate limiting
8. Request timeout enforcement

Post-Response (after backend call):
1. Response logging
2. Response metrics collection
3. Security headers injection
4. Compression

## Code Structure

### Directory Layout

```
api-gateway-go/
├── cmd/gateway/           # Main application entry point
├── internal/
│   ├── config/           # Configuration loading and validation
│   ├── server/           # HTTP server setup and lifecycle
│   ├── middleware/       # Middleware components
│   ├── auth/            # Session token validation and authorization
│   ├── ratelimit/       # Rate limiting implementation
│   ├── router/          # Request routing logic
│   ├── proxy/           # Backend proxying
│   ├── logging/         # Structured logging
│   └── metrics/         # Prometheus metrics
├── pkg/                  # Public libraries (if any)
├── test/                 # Integration and e2e tests
└── deployments/         # Kubernetes manifests, Dockerfile
```

## Development Guidelines

### Go Code Standards

1. **Follow Go idioms and conventions**
   - Use `gofmt` and `goimports` for formatting
   - Follow [Effective Go](https://go.dev/doc/effective_go) principles
   - Use meaningful variable and function names
   - Keep functions small and focused

2. **Error Handling**
   - Always check and handle errors
   - Wrap errors with context using `fmt.Errorf` with `%w`
   - Use custom error types for specific error conditions
   - Never ignore errors with `_` unless absolutely necessary and documented

3. **Concurrency**
   - Use goroutines and channels appropriately
   - Always use proper synchronization (mutexes, channels, sync.WaitGroup)
   - Avoid race conditions (tests run with `-race` flag)
   - Use context for cancellation and timeouts

4. **Code Organization**
   - Keep package dependencies acyclic
   - Use interfaces for testability
   - Minimize exported functions and types
   - Group related functionality in packages

### Testing Requirements

**CRITICAL: All tests must pass for CI/CD pipelines to succeed**

1. **Unit Tests**
   - Write unit tests for all business logic
   - Use table-driven tests for multiple scenarios
   - Mock external dependencies (Redis, backend services)
   - Target >80% code coverage
   - Run with: `go test -v -race -coverprofile=coverage.out ./...`

2. **Race Detection**
   - All tests must pass with `-race` flag enabled
   - Fix any race conditions immediately
   - Use proper synchronization primitives

3. **Integration Tests**
   - Test complete request flows end-to-end
   - Use test containers for Redis and other dependencies
   - Test error scenarios and edge cases

4. **Test Structure**
   - Test files: `*_test.go`
   - Table-driven tests preferred
   - Use `testify` for assertions if desired
   - Clean up resources in tests

### Linting Requirements

**CRITICAL: golangci-lint must pass for CI to succeed**

1. **Run golangci-lint locally before committing**
   ```bash
   golangci-lint run --timeout=5m
   ```

2. **Common Issues to Avoid**
   - Unused variables and imports
   - Shadowed variables
   - Error checking omissions
   - Inefficient string concatenation in loops
   - Unnecessary type conversions
   - Magic numbers (use named constants)
   - Deep nesting (refactor complex logic)

3. **Code Quality**
   - Avoid cyclomatic complexity >15
   - Keep function length reasonable (<100 lines)
   - Limit function parameters (<5 parameters)
   - Use named return values sparingly

### Build Requirements

**CRITICAL: `make build` must succeed for CI to pass**

1. **Build Command**
   ```bash
   make build
   ```

2. **Build Output**
   - Binary output: `bin/gateway`
   - Ensure all dependencies are in `go.mod`
   - Use Go modules (not vendor)

3. **Build Flags**
   - Production builds use ldflags for version info
   - Debug symbols stripped in release builds
   - Cross-compilation support for multiple platforms

### Docker Requirements

**CRITICAL: Docker build must succeed**

1. **Dockerfile Best Practices**
   - Multi-stage builds to minimize image size
   - Use specific Go version from go.mod
   - Run as non-root user for security
   - Include health check
   - Minimal base image (alpine or distroless)

2. **Docker Build**
   ```bash
   docker build -t api-gateway:latest .
   ```

## Security Considerations

### Authentication & Authorization

1. **Session Tokens (JWT)**
   - Tokens transmitted only over HTTPS
   - Cookies marked with: Secure, HttpOnly, SameSite
   - Signature verification using RS256 or HS256
   - Expiration enforcement with clock skew tolerance
   - Revocation list for immediate invalidation

2. **Authorization Policies**
   - Public routes (no auth)
   - Authenticated routes (valid token required)
   - Role-based routes (specific roles required)
   - Permission-based routes (specific permissions required)

3. **Security Best Practices**
   - Never log full session tokens (only last 4 chars for debugging)
   - Never log Authorization headers
   - Sanitize sensitive query parameters
   - Validate all inputs
   - Use constant-time comparison for secrets

### TLS Configuration

1. **TLS Requirements**
   - TLS 1.2 minimum, TLS 1.3 preferred
   - Strong cipher suites only
   - Forward secrecy enabled (ECDHE)
   - HSTS headers enforced

2. **Certificate Management**
   - Automated certificate renewal
   - Certificate expiration monitoring
   - Support for certificate rotation

### Rate Limiting

1. **Token Bucket Algorithm**
   - Allows burst traffic up to bucket capacity
   - Tokens refilled at fixed rate
   - Configurable per route/user/IP

2. **Redis Backend**
   - Production deployments use Redis for shared state
   - Atomic operations for counter updates
   - TTL for automatic cleanup
   - Fail-open vs fail-closed configurable

3. **Rate Limit Headers**
   ```
   X-RateLimit-Limit: 100
   X-RateLimit-Remaining: 45
   X-RateLimit-Reset: 1700000000
   Retry-After: 30
   ```

### Logging Security

**Never log sensitive data:**
- Session token values
- Authorization headers
- Passwords
- Credit card numbers
- PII (unless explicitly configured and compliant)
- API keys or secrets

**Sanitization:**
- Query parameters named "token", "password", "secret" are redacted
- Request/response bodies not logged by default
- Pattern matching for sensitive data

## Configuration

### Configuration Sources (Precedence)

1. Default configuration (embedded)
2. Configuration file (YAML/JSON)
3. Environment variables (override file)
4. Command-line flags (override all)

### Environment Variables

Use `GATEWAY_` prefix for all environment variables:

```bash
GATEWAY_HTTP_PORT=8080
GATEWAY_HTTPS_PORT=8443
GATEWAY_LOG_LEVEL=info
GATEWAY_REDIS_ADDR=localhost:6379
GATEWAY_SESSION_COOKIE_NAME=session_token
```

### Configuration Validation

- All configuration validated on startup
- Invalid configuration causes startup failure
- Clear error messages for misconfiguration
- Example configs provided for each environment

## Observability

### Metrics

**Prometheus metrics exposed at `/metrics`**

Key metrics:
- `gateway_http_requests_total` - Total HTTP requests by method, route, status
- `gateway_http_request_duration_seconds` - Request latency histogram
- `gateway_auth_attempts_total` - Authorization attempts by result
- `gateway_ratelimit_exceeded_total` - Rate limit exceeded events
- `gateway_backend_requests_total` - Backend requests by service, status
- `gateway_backend_duration_seconds` - Backend latency by service

### Logging

**Structured JSON logging to stdout**

Log levels: DEBUG, INFO, WARN, ERROR, FATAL

Standard log fields:
```json
{
  "timestamp": "2025-11-16T10:00:00Z",
  "level": "INFO",
  "logger": "http.server",
  "correlation_id": "550e8400-e29b-41d4-a716-446655440000",
  "message": "Request completed",
  "method": "GET",
  "path": "/api/v1/users",
  "status_code": 200,
  "duration_ms": 45,
  "client_ip": "192.168.1.100"
}
```

### Health Checks

- **Liveness:** `/_health/live` - Process is alive
- **Readiness:** `/_health/ready` - Ready to serve traffic (checks Redis, config)

## Performance Targets

### Latency

- p50: <10ms overhead (excluding backend)
- p95: <25ms overhead
- p99: <50ms overhead

### Throughput

- 10,000+ requests/second per instance
- Scales linearly with additional instances

### Resource Requirements

- CPU: 2-4 cores per instance at normal load
- Memory: 512MB - 1GB per instance
- Network: Gigabit Ethernet minimum

## Common Patterns

### Middleware Pattern

```go
type Middleware func(http.Handler) http.Handler

func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Pre-request logic
        start := time.Now()

        // Call next handler
        next.ServeHTTP(w, r)

        // Post-request logic
        duration := time.Since(start)
        log.Info("Request completed", "duration", duration)
    })
}
```

### Context Usage

```go
// Store user context in request context
type userContextKey struct{}

func SetUserContext(ctx context.Context, user *User) context.Context {
    return context.WithValue(ctx, userContextKey{}, user)
}

func GetUserContext(ctx context.Context) (*User, bool) {
    user, ok := ctx.Value(userContextKey{}).(*User)
    return user, ok
}
```

### Error Handling

```go
// Custom error types
type AuthError struct {
    Code    string
    Message string
    Err     error
}

func (e *AuthError) Error() string {
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AuthError) Unwrap() error {
    return e.Err
}

// Return structured JSON errors
type ErrorResponse struct {
    Error         string            `json:"error"`
    Message       string            `json:"message"`
    CorrelationID string            `json:"correlation_id"`
    Timestamp     time.Time         `json:"timestamp"`
    Path          string            `json:"path"`
    Details       map[string]any    `json:"details,omitempty"`
}
```

### Graceful Shutdown

```go
// Server with graceful shutdown
srv := &http.Server{
    Addr:    ":8080",
    Handler: handler,
}

// Start server in goroutine
go func() {
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatal("Server failed", "error", err)
    }
}()

// Wait for interrupt signal
quit := make(chan os.Signal, 1)
signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
<-quit

// Graceful shutdown with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := srv.Shutdown(ctx); err != nil {
    log.Error("Server forced to shutdown", "error", err)
}
```

## CI/CD Pipeline Requirements

### GitHub Actions Workflows

All workflows must pass for successful CI/CD:

1. **CI Workflow** (`.github/workflows/ci.yml`)
   - ✅ Tests must pass with race detection
   - ✅ Code coverage generated
   - ✅ golangci-lint must pass with no errors
   - ✅ Build must succeed using `make build`

2. **Docker Workflow** (`.github/workflows/docker.yml`)
   - ✅ Docker image must build successfully
   - ✅ Multi-platform support (linux/amd64)

3. **Release Workflow** (`.github/workflows/release.yml`)
   - ✅ Builds for multiple platforms (Linux, macOS, Windows)
   - ✅ Generates checksums
   - ✅ Creates GitHub releases

### Pre-Commit Checklist

Before committing, ensure:

- [ ] All tests pass: `go test -v -race ./...`
- [ ] Linter passes: `golangci-lint run --timeout=5m`
- [ ] Build succeeds: `make build`
- [ ] No sensitive data in code or logs
- [ ] Documentation updated if needed
- [ ] Error handling is comprehensive
- [ ] Race conditions avoided
- [ ] Code is formatted: `gofmt -w .`

## Common Issues and Solutions

### Issue: Race Detector Fails

**Problem:** Tests fail with `-race` flag
**Solution:**
- Use proper synchronization (mutex, channels)
- Avoid sharing memory without protection
- Use `sync.WaitGroup` for goroutine coordination
- Review concurrent map access

### Issue: golangci-lint Errors

**Problem:** Linter reports errors
**Solution:**
- Fix unused variables and imports
- Add error checking for all error returns
- Use named constants instead of magic numbers
- Reduce function complexity
- Add comments for exported functions

### Issue: Build Fails

**Problem:** `make build` fails
**Solution:**
- Run `go mod tidy` to clean dependencies
- Ensure `go.mod` is up to date
- Check for compilation errors
- Verify all imports are available

### Issue: Docker Build Fails

**Problem:** Docker image build fails
**Solution:**
- Ensure Dockerfile uses correct Go version from go.mod
- Check multi-stage build stages
- Verify file paths in COPY commands
- Test locally before pushing

## Reference Documentation

### Design Specification

See `API_GATEWAY_DESIGN_SPEC.md` for comprehensive design details including:
- Detailed architecture and component breakdown
- Request flow and lifecycle
- Authorization and rate limiting design
- Security considerations
- Scalability and performance characteristics
- Task breakdown and implementation plan

### Go Resources

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [Go HTTP Server Best Practices](https://go.dev/doc/articles/wiki/)

### Testing Resources

- [Testing in Go](https://go.dev/doc/tutorial/add-a-test)
- [Table-driven tests](https://go.dev/wiki/TableDrivenTests)
- [Race Detector](https://go.dev/doc/articles/race_detector)

## Getting Started

### Prerequisites

- Go 1.21+ (check go.mod for exact version)
- Docker (for containerized deployment)
- Redis (for rate limiting in production)
- Make

### Quick Start

```bash
# Clone repository
git clone <repository-url>
cd api-gateway-go

# Download dependencies
go mod download

# Run tests
go test -v -race ./...

# Run linter
golangci-lint run --timeout=5m

# Build
make build

# Run locally
./bin/gateway --config config/dev.yaml

# Or run with Docker
docker build -t api-gateway .
docker run -p 8080:8080 api-gateway
```

### Development Workflow

1. Create feature branch: `git checkout -b feature/my-feature`
2. Write code and tests
3. Run tests: `go test -v -race ./...`
4. Run linter: `golangci-lint run`
5. Commit with clear message
6. Push and create pull request
7. Ensure CI passes
8. Request review
9. Merge after approval

---

## Important Notes for AI Assistants

When working on this codebase:

1. **ALWAYS run tests with `-race` flag** - Race conditions are critical bugs
2. **ALWAYS run golangci-lint before suggesting code changes** - Linting is non-negotiable
3. **ALWAYS verify `make build` succeeds** - Build must pass in CI
4. **NEVER log sensitive data** - Security is paramount
5. **ALWAYS handle errors** - No ignored errors in production code
6. **ALWAYS write tests for new code** - Test coverage is required
7. **ALWAYS follow Go idioms** - Write idiomatic Go, not Go-flavored code from other languages
8. **ALWAYS consider concurrency safety** - This is a high-concurrency service
9. **ALWAYS validate inputs** - Security vulnerability prevention
10. **ALWAYS document exported functions** - godoc comments required

When making changes that affect CI/CD:
- Test locally first (tests, lint, build)
- Consider impact on all three workflows (CI, Docker, Release)
- Ensure backwards compatibility where needed
- Update documentation if behavior changes
