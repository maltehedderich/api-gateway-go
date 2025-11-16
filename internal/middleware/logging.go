package middleware

import (
	"net/http"
	"time"

	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

// responseWriter wraps http.ResponseWriter to capture status code and size
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// Logging returns a middleware that logs HTTP requests and responses
func Logging() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Get logger with correlation ID
			log := logger.FromContext(r.Context(), "http")

			// Log request
			log.Info("incoming request", logger.Fields{
				"method":      r.Method,
				"path":        r.URL.Path,
				"query":       sanitizeQuery(r.URL.RawQuery),
				"remote_ip":   getClientIP(r),
				"user_agent":  r.UserAgent(),
				"protocol":    r.Proto,
				"host":        r.Host,
				"content_length": r.ContentLength,
			})

			// Process request
			next.ServeHTTP(rw, r)

			// Calculate duration
			duration := time.Since(start)

			// Determine log level based on status code
			logLevel := logger.InfoLevel
			if rw.statusCode >= 400 && rw.statusCode < 500 {
				logLevel = logger.WarnLevel
			} else if rw.statusCode >= 500 {
				logLevel = logger.ErrorLevel
			}

			// Log response
			fields := logger.Fields{
				"method":         r.Method,
				"path":           r.URL.Path,
				"status":         rw.statusCode,
				"duration_ms":    duration.Milliseconds(),
				"response_size":  rw.size,
				"remote_ip":      getClientIP(r),
			}

			message := "request completed"
			switch logLevel {
			case logger.InfoLevel:
				log.Info(message, fields)
			case logger.WarnLevel:
				log.Warn(message, fields)
			case logger.ErrorLevel:
				log.Error(message, fields)
			}
		})
	}
}

// sanitizeQuery sanitizes query string for logging
func sanitizeQuery(query string) string {
	// In a real implementation, this would redact sensitive parameters
	// For now, just return as is
	return query
}
