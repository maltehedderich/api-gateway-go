package ratelimit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/maltehedderich/api-gateway-go/internal/config"
	"github.com/maltehedderich/api-gateway-go/internal/logger"
	"github.com/maltehedderich/api-gateway-go/internal/metrics"
)

// Middleware creates a rate limiting middleware.
// It checks rate limits before allowing requests to proceed.
// Returns 429 Too Many Requests if rate limit is exceeded.
func Middleware(limiter *Limiter, cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip rate limiting if disabled
			if !cfg.RateLimit.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			log := logger.Get().WithComponent("ratelimit")

			// Find applicable rate limits for this route
			limits := getApplicableLimits(r, cfg)

			// Check each limit
			for _, limitDef := range limits {
				checkStart := time.Now()
				result, err := limiter.Allow(r.Context(), r, &limitDef)
				metrics.RecordRateLimitCheckDuration(time.Since(checkStart))
				metrics.RecordRateLimitCheck()

				if err != nil {
					log.Error("rate limit check failed", logger.Fields{
						"error": err.Error(),
						"key":   limitDef.Key,
						"path":  r.URL.Path,
					})
					metrics.RecordRateLimitError("check_failed")

					// On error, apply failure mode
					if cfg.RateLimit.FailureMode == "fail-closed" {
						writeRateLimitError(w, r, &limitDef, nil)
						return
					}
					// fail-open: continue to next limit or allow request
					continue
				}

				// Record utilization
				if result.Limit > 0 {
					utilization := float64(result.Limit-result.Remaining) / float64(result.Limit) * 100
					metrics.RecordRateLimitUtilization(limitDef.Key, utilization)
				}

				// Add rate limit headers to response
				addRateLimitHeaders(w, result)

				// If not allowed, return 429
				if !result.Allowed {
					log.Warn("rate limit exceeded", logger.Fields{
						"key":       limitDef.Key,
						"limit":     result.Limit,
						"remaining": result.Remaining,
						"path":      r.URL.Path,
						"method":    r.Method,
					})
					metrics.RecordRateLimitExceeded(limitDef.Key, r.URL.Path)

					writeRateLimitError(w, r, &limitDef, result)
					return
				}
			}

			// All limits passed, continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// getApplicableLimits returns the rate limits that apply to the request.
// It checks both global limits and route-specific limits.
func getApplicableLimits(r *http.Request, cfg *config.Config) []config.LimitDefinition {
	limits := make([]config.LimitDefinition, 0)

	// Add global limits
	limits = append(limits, cfg.RateLimit.GlobalLimits...)

	// Find matching route and add route-specific limits
	for _, route := range cfg.Routes {
		if routeMatches(r, &route) {
			limits = append(limits, route.RateLimits...)
			break
		}
	}

	return limits
}

// routeMatches checks if a request matches a route configuration.
// This is a simple prefix match - in production, use the router's matching logic.
func routeMatches(r *http.Request, route *config.RouteConfig) bool {
	// Check if path matches (simple prefix match for now)
	pathMatches := r.URL.Path == route.PathPattern ||
		(len(route.PathPattern) > 0 && route.PathPattern[len(route.PathPattern)-1] == '*' &&
			len(r.URL.Path) >= len(route.PathPattern)-1 &&
			r.URL.Path[:len(route.PathPattern)-1] == route.PathPattern[:len(route.PathPattern)-1])

	if !pathMatches {
		return false
	}

	// Check if method matches
	if len(route.Methods) == 0 {
		return true
	}

	for _, method := range route.Methods {
		if method == r.Method {
			return true
		}
	}

	return false
}

// addRateLimitHeaders adds rate limit headers to the response.
// Headers include X-RateLimit-Limit, X-RateLimit-Remaining, and X-RateLimit-Reset.
func addRateLimitHeaders(w http.ResponseWriter, result *Result) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.Reset.Unix(), 10))

	if !result.Allowed && result.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
	}
}

// writeRateLimitError writes a 429 Too Many Requests error response.
func writeRateLimitError(w http.ResponseWriter, r *http.Request, limit *config.LimitDefinition, result *Result) {
	w.Header().Set("Content-Type", "application/json")

	// Set retry-after header if we have result
	if result != nil && result.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
	}

	w.WriteHeader(http.StatusTooManyRequests)

	// Get correlation ID from context if available
	correlationID := r.Header.Get("X-Correlation-ID")

	// Build error response
	errorResp := map[string]interface{}{
		"error":          "rate_limit_exceeded",
		"message":        "Rate limit exceeded for this resource",
		"correlation_id": correlationID,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"path":           r.URL.Path,
	}

	if result != nil {
		errorResp["details"] = map[string]interface{}{
			"limit":    result.Limit,
			"window":   limit.Window,
			"reset_at": result.Reset.UTC().Format(time.RFC3339),
		}
		errorResp["retry_after"] = int(result.RetryAfter.Seconds())
	}

	// Write JSON response
	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		// If JSON encoding fails, write plain text
		_, _ = fmt.Fprintf(w, "Rate limit exceeded\n")
	}
}
