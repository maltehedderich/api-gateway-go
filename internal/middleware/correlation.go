package middleware

import (
	"net/http"

	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

const (
	// CorrelationIDHeader is the HTTP header for correlation ID
	CorrelationIDHeader = "X-Correlation-ID"
)

// CorrelationID returns a middleware that adds correlation ID to requests
func CorrelationID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try to get correlation ID from header
			correlationID := r.Header.Get(CorrelationIDHeader)

			// Generate new correlation ID if not present
			if correlationID == "" {
				correlationID = logger.GenerateCorrelationID()
			}

			// Add correlation ID to context
			ctx := logger.WithCorrelationID(r.Context(), correlationID)
			r = r.WithContext(ctx)

			// Add correlation ID to response header
			w.Header().Set(CorrelationIDHeader, correlationID)

			next.ServeHTTP(w, r)
		})
	}
}
