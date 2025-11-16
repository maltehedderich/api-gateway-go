package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

// Recovery returns a middleware that recovers from panics
func Recovery() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Get stack trace
					stack := debug.Stack()

					// Log the panic with correlation ID if available
					correlationID := logger.GetCorrelationID(r.Context())
					compLogger := logger.Get().WithComponent("recovery")
					ctxLogger := compLogger.WithCorrelationID(correlationID)

					ctxLogger.Error("panic recovered", logger.Fields{
						"error":      fmt.Sprintf("%v", err),
						"stack":      string(stack),
						"method":     r.Method,
						"path":       r.URL.Path,
						"remote_ip":  getClientIP(r),
					})

					// Send error response
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)

					errorResponse := map[string]interface{}{
						"error":   "internal_server_error",
						"message": "An internal error occurred",
					}

					if correlationID != "" {
						errorResponse["correlation_id"] = correlationID
					}

					// Write error response (ignore errors here as we're already in recovery)
					_ = writeJSON(w, errorResponse)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
