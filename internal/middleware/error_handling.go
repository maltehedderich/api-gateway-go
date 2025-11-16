package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/maltehedderich/api-gateway-go/internal/config"
	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

// errorResponseWriter wraps http.ResponseWriter to capture status code and error
type errorResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *errorResponseWriter) WriteHeader(statusCode int) {
	if !rw.written {
		rw.statusCode = statusCode
		rw.written = true
		rw.ResponseWriter.WriteHeader(statusCode)
	}
}

func (rw *errorResponseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// ErrorHandling returns a middleware that implements error disclosure prevention
func ErrorHandling(cfg *config.SecurityConfig) func(http.Handler) http.Handler {
	log := logger.Get().WithComponent("middleware.error_handling")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Wrap response writer to capture status code
			wrapped := &errorResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				written:        false,
			}

			// Recover from panics
			defer func() {
				if err := recover(); err != nil {
					correlationID := logger.GetCorrelationID(r.Context())

					// Log panic with stack trace
					log.Error("panic recovered", logger.Fields{
						"correlation_id": correlationID,
						"error":          fmt.Sprintf("%v", err),
						"stack_trace":    string(debug.Stack()),
						"method":         r.Method,
						"path":           r.URL.Path,
					})

					// Don't write response if already written
					if wrapped.written {
						return
					}

					// Write sanitized error response
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)

					errorResp := buildErrorResponse(
						"internal_server_error",
						"An unexpected error occurred",
						correlationID,
						cfg.ProductionMode || cfg.HideInternalErrors,
						fmt.Sprintf("%v", err),
					)

					_ = json.NewEncoder(w).Encode(errorResp)
				}
			}()

			next.ServeHTTP(wrapped, r)
		})
	}
}

// buildErrorResponse builds an error response with optional details
func buildErrorResponse(errorCode, message, correlationID string, hideDetails bool, internalError string) map[string]interface{} {
	resp := map[string]interface{}{
		"error":          errorCode,
		"message":        message,
		"correlation_id": correlationID,
	}

	// Only include internal error details in development mode
	if !hideDetails && internalError != "" {
		resp["details"] = map[string]interface{}{
			"internal_error": internalError,
		}
	}

	return resp
}

// SanitizeError sanitizes an error message for client response
func SanitizeError(err error, cfg *config.SecurityConfig) string {
	if cfg.ProductionMode || cfg.HideInternalErrors {
		// Return generic error message in production
		return "An error occurred while processing your request"
	}

	// Return actual error in development
	if err != nil {
		return err.Error()
	}

	return "Unknown error"
}

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error         string                 `json:"error"`
	Message       string                 `json:"message"`
	CorrelationID string                 `json:"correlation_id"`
	Details       map[string]interface{} `json:"details,omitempty"`
}

// WriteJSONError writes a JSON error response with proper sanitization
func WriteJSONError(w http.ResponseWriter, r *http.Request, statusCode int, errorCode, message string, details map[string]interface{}, cfg *config.SecurityConfig) {
	correlationID := logger.GetCorrelationID(r.Context())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := ErrorResponse{
		Error:         errorCode,
		Message:       message,
		CorrelationID: correlationID,
	}

	// Only include details if not in production mode
	if !cfg.ProductionMode && !cfg.HideInternalErrors && details != nil {
		resp.Details = details
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log := logger.Get().WithComponent("middleware.error_handling")
		log.Error("failed to encode error response", logger.Fields{
			"error":          err.Error(),
			"correlation_id": correlationID,
		})
	}
}
