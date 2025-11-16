package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

// Middleware represents a middleware function
// It wraps an http.Handler and returns a new http.Handler
type Middleware func(http.Handler) http.Handler

// Chain represents a chain of middleware
type Chain struct {
	middlewares []Middleware
	logger      *logger.ComponentLogger
}

// NewChain creates a new middleware chain
func NewChain(middlewares ...Middleware) *Chain {
	return &Chain{
		middlewares: middlewares,
		logger:      logger.Get().WithComponent("middleware.chain"),
	}
}

// Then chains the middleware and returns the final handler
func (c *Chain) Then(h http.Handler) http.Handler {
	// Apply middleware in reverse order so that the first middleware
	// in the chain is executed first
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		h = c.middlewares[i](h)
	}
	return h
}

// Append adds middleware to the end of the chain
func (c *Chain) Append(middlewares ...Middleware) *Chain {
	newChain := &Chain{
		middlewares: make([]Middleware, 0, len(c.middlewares)+len(middlewares)),
		logger:      c.logger,
	}
	newChain.middlewares = append(newChain.middlewares, c.middlewares...)
	newChain.middlewares = append(newChain.middlewares, middlewares...)
	return newChain
}

// Timing middleware measures the time taken by the handler
func Timing() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a response writer wrapper to capture status code
			rw := NewResponseWriter(w)

			// Call next handler
			next.ServeHTTP(rw, r)

			// Calculate duration
			duration := time.Since(start)

			// Get correlation ID if available
			correlationID := logger.GetCorrelationID(r.Context())

			// Log timing information
			logger.Get().WithComponent("middleware.timing").Debug("request completed", logger.Fields{
				"correlation_id": correlationID,
				"method":         r.Method,
				"path":           r.URL.Path,
				"status":         rw.Status(),
				"duration_ms":    duration.Milliseconds(),
				"duration_us":    duration.Microseconds(),
			})

			// Store duration in context for potential use by other middleware
			ctx := context.WithValue(r.Context(), ContextKeyDuration, duration)
			*r = *r.WithContext(ctx)
		})
	}
}

// ContextKey is a type for context keys
type ContextKey string

const (
	// ContextKeyDuration is the context key for request duration
	ContextKeyDuration ContextKey = "duration"
	// ContextKeyRouteMatch is the context key for route match information
	ContextKeyRouteMatch ContextKey = "route_match"
	// ContextKeyBackendURL is the context key for backend URL
	ContextKeyBackendURL ContextKey = "backend_url"
)

// GetDuration retrieves the request duration from context
func GetDuration(ctx context.Context) time.Duration {
	if duration, ok := ctx.Value(ContextKeyDuration).(time.Duration); ok {
		return duration
	}
	return 0
}

// RouteMatchFromContext retrieves route match information from context
func RouteMatchFromContext(ctx context.Context) interface{} {
	return ctx.Value(ContextKeyRouteMatch)
}

// BackendURLFromContext retrieves backend URL from context
func BackendURLFromContext(ctx context.Context) string {
	if url, ok := ctx.Value(ContextKeyBackendURL).(string); ok {
		return url
	}
	return ""
}

// Registry manages middleware registration and composition
type Registry struct {
	middlewares map[string]Middleware
	logger      *logger.ComponentLogger
	mu          sync.RWMutex
}

// NewRegistry creates a new middleware registry
func NewRegistry() *Registry {
	return &Registry{
		middlewares: make(map[string]Middleware),
		logger:      logger.Get().WithComponent("middleware.registry"),
	}
}

// Register registers a middleware with a name
func (r *Registry) Register(name string, mw Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.middlewares[name] = mw
	r.logger.Debug("middleware registered", logger.Fields{
		"name": name,
	})
}

// Get retrieves a middleware by name
func (r *Registry) Get(name string) (Middleware, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	mw, ok := r.middlewares[name]
	return mw, ok
}

// BuildChain builds a middleware chain from a list of names
func (r *Registry) BuildChain(names []string) (*Chain, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	middlewares := make([]Middleware, 0, len(names))

	for _, name := range names {
		mw, ok := r.middlewares[name]
		if !ok {
			return nil, fmt.Errorf("middleware not found: %s", name)
		}
		middlewares = append(middlewares, mw)
	}

	return NewChain(middlewares...), nil
}

// GetAll returns all registered middleware names
func (r *Registry) GetAll() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.middlewares))
	for name := range r.middlewares {
		names = append(names, name)
	}
	return names
}
