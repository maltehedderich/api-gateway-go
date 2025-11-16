package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/maltehedderich/api-gateway-go/internal/config"
	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

// InputValidation returns a middleware that validates request inputs
func InputValidation(cfg *config.SecurityConfig) func(http.Handler) http.Handler {
	log := logger.Get().WithComponent("middleware.input_validation")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			correlationID := logger.GetCorrelationID(r.Context())

			// Validate HTTP method
			if len(cfg.AllowedMethods) > 0 {
				if !isMethodAllowed(r.Method, cfg.AllowedMethods) {
					log.Warn("method not allowed", logger.Fields{
						"correlation_id": correlationID,
						"method":         r.Method,
						"path":           r.URL.Path,
					})

					writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed",
						"HTTP method not allowed", correlationID)
					return
				}
			}

			// Validate URL path length
			if cfg.MaxURLPathLength > 0 && len(r.URL.Path) > cfg.MaxURLPathLength {
				log.Warn("URL path too long", logger.Fields{
					"correlation_id": correlationID,
					"path_length":    len(r.URL.Path),
					"max_length":     cfg.MaxURLPathLength,
				})

				writeErrorResponse(w, http.StatusRequestURITooLong, "uri_too_long",
					"Request URI exceeds maximum length", correlationID)
				return
			}

			// Validate User-Agent against blocked list
			if len(cfg.BlockedUserAgents) > 0 {
				userAgent := r.Header.Get("User-Agent")
				if isUserAgentBlocked(userAgent, cfg.BlockedUserAgents) {
					log.Warn("blocked user agent", logger.Fields{
						"correlation_id": correlationID,
						"user_agent":     userAgent,
						"path":           r.URL.Path,
					})

					writeErrorResponse(w, http.StatusForbidden, "forbidden",
						"Access denied", correlationID)
					return
				}
			}

			// Validate request body size
			if cfg.MaxRequestBodySize > 0 {
				// Use MaxBytesReader to limit request body size
				r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxRequestBodySize)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isMethodAllowed checks if the HTTP method is in the allowed list
func isMethodAllowed(method string, allowedMethods []string) bool {
	method = strings.ToUpper(method)
	for _, allowed := range allowedMethods {
		if strings.ToUpper(allowed) == method {
			return true
		}
	}
	return false
}

// isUserAgentBlocked checks if the User-Agent matches any blocked patterns
func isUserAgentBlocked(userAgent string, blockedAgents []string) bool {
	userAgent = strings.ToLower(userAgent)
	for _, blocked := range blockedAgents {
		if strings.Contains(userAgent, strings.ToLower(blocked)) {
			return true
		}
	}
	return false
}

// writeErrorResponse writes a JSON error response
func writeErrorResponse(w http.ResponseWriter, statusCode int, errorCode, message, correlationID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResp := map[string]interface{}{
		"error":          errorCode,
		"message":        message,
		"correlation_id": correlationID,
	}

	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		// Log encoding error but don't expose it to client
		log := logger.Get().WithComponent("middleware.input_validation")
		log.Error("failed to encode error response", logger.Fields{
			"error":          err.Error(),
			"correlation_id": correlationID,
		})
	}
}
