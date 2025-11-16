package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/maltehedderich/api-gateway-go/internal/middleware"
)

// Middleware returns a metrics collection middleware
func Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip metrics for metrics endpoint itself
			if r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			// Increment active requests
			IncActiveRequests()
			defer DecActiveRequests()

			// Record start time
			start := time.Now()

			// Get request size
			requestSize := int(r.ContentLength)
			if requestSize < 0 {
				requestSize = 0
			}

			// Wrap response writer to capture status code and response size
			wrapped := middleware.NewResponseWriter(w)

			// Call next handler
			next.ServeHTTP(wrapped, r)

			// Record metrics
			duration := time.Since(start)
			statusCode := strconv.Itoa(wrapped.StatusCode())
			route := r.URL.Path
			method := r.Method
			responseSize := wrapped.BytesWritten()

			RecordHTTPRequest(method, route, statusCode, duration, requestSize, responseSize)
		})
	}
}
