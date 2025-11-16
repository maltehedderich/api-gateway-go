package metrics

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP Request Metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests by method, route, and status code",
		},
		[]string{"method", "route", "status_code"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "gateway",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "route", "status_code"},
	)

	httpRequestSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "gateway",
			Subsystem: "http",
			Name:      "request_size_bytes",
			Help:      "HTTP request size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "route"},
	)

	httpResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "gateway",
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "HTTP response size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "route", "status_code"},
	)

	httpActiveRequests = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "gateway",
			Subsystem: "http",
			Name:      "active_requests",
			Help:      "Number of currently active HTTP requests",
		},
	)

	// Authorization Metrics
	authAttemptsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Subsystem: "auth",
			Name:      "attempts_total",
			Help:      "Total number of authorization attempts by result",
		},
		[]string{"result"}, // success, failure, bypass
	)

	authFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Subsystem: "auth",
			Name:      "failures_total",
			Help:      "Total number of authorization failures by error type",
		},
		[]string{"error_type"}, // missing_token, invalid_token, expired_token, revoked_token, insufficient_permissions
	)

	authValidationDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "gateway",
			Subsystem: "auth",
			Name:      "validation_duration_seconds",
			Help:      "Duration of token validation in seconds",
			Buckets:   []float64{.001, .0025, .005, .01, .025, .05, .1, .25, .5, 1},
		},
	)

	authCacheHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Subsystem: "auth",
			Name:      "cache_hits_total",
			Help:      "Total number of cache hits/misses for authorization decisions",
		},
		[]string{"result"}, // hit, miss
	)

	// Rate Limiting Metrics
	rateLimitChecksTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Subsystem: "ratelimit",
			Name:      "checks_total",
			Help:      "Total number of rate limit checks performed",
		},
	)

	rateLimitExceededTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Subsystem: "ratelimit",
			Name:      "exceeded_total",
			Help:      "Total number of requests that exceeded rate limits",
		},
		[]string{"key_type", "route"},
	)

	rateLimitUtilization = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "gateway",
			Subsystem: "ratelimit",
			Name:      "utilization_percent",
			Help:      "Rate limit utilization as percentage",
			Buckets:   []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 95, 99, 100},
		},
		[]string{"key_type"},
	)

	rateLimitCheckDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "gateway",
			Subsystem: "ratelimit",
			Name:      "check_duration_seconds",
			Help:      "Duration of rate limit checks in seconds",
			Buckets:   []float64{.0001, .0005, .001, .0025, .005, .01, .025, .05, .1},
		},
	)

	rateLimitErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Subsystem: "ratelimit",
			Name:      "errors_total",
			Help:      "Total number of rate limiter errors",
		},
		[]string{"error_type"},
	)

	// Backend Service Metrics
	backendRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Subsystem: "backend",
			Name:      "requests_total",
			Help:      "Total number of backend requests by service and status",
		},
		[]string{"backend_service", "status_code"},
	)

	backendRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "gateway",
			Subsystem: "backend",
			Name:      "request_duration_seconds",
			Help:      "Backend request duration in seconds",
			Buckets:   []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
		},
		[]string{"backend_service"},
	)

	backendErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Subsystem: "backend",
			Name:      "errors_total",
			Help:      "Total number of backend errors",
		},
		[]string{"backend_service", "error_type"}, // timeout, connection_refused, bad_gateway
	)

	// Circuit Breaker Metrics
	circuitBreakerState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "gateway",
			Subsystem: "circuitbreaker",
			Name:      "state",
			Help:      "Circuit breaker state (0=closed, 1=open, 2=half-open)",
		},
		[]string{"backend_service"},
	)

	circuitBreakerTransitionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Subsystem: "circuitbreaker",
			Name:      "transitions_total",
			Help:      "Total number of circuit breaker state transitions",
		},
		[]string{"backend_service", "from_state", "to_state"},
	)

	// Health Check Metrics
	healthCheckTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gateway",
			Subsystem: "health",
			Name:      "checks_total",
			Help:      "Total number of health checks performed",
		},
		[]string{"check_name", "status"},
	)

	healthCheckDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "gateway",
			Subsystem: "health",
			Name:      "check_duration_seconds",
			Help:      "Duration of health checks in seconds",
			Buckets:   []float64{.001, .005, .01, .05, .1, .5, 1, 2},
		},
		[]string{"check_name"},
	)

	once sync.Once
)

// Init initializes and registers all metrics with Prometheus
func Init() {
	once.Do(func() {
		// Register HTTP metrics
		prometheus.MustRegister(httpRequestsTotal)
		prometheus.MustRegister(httpRequestDuration)
		prometheus.MustRegister(httpRequestSize)
		prometheus.MustRegister(httpResponseSize)
		prometheus.MustRegister(httpActiveRequests)

		// Register authorization metrics
		prometheus.MustRegister(authAttemptsTotal)
		prometheus.MustRegister(authFailuresTotal)
		prometheus.MustRegister(authValidationDuration)
		prometheus.MustRegister(authCacheHitsTotal)

		// Register rate limiting metrics
		prometheus.MustRegister(rateLimitChecksTotal)
		prometheus.MustRegister(rateLimitExceededTotal)
		prometheus.MustRegister(rateLimitUtilization)
		prometheus.MustRegister(rateLimitCheckDuration)
		prometheus.MustRegister(rateLimitErrorsTotal)

		// Register backend metrics
		prometheus.MustRegister(backendRequestsTotal)
		prometheus.MustRegister(backendRequestDuration)
		prometheus.MustRegister(backendErrorsTotal)

		// Register circuit breaker metrics
		prometheus.MustRegister(circuitBreakerState)
		prometheus.MustRegister(circuitBreakerTransitionsTotal)

		// Register health check metrics
		prometheus.MustRegister(healthCheckTotal)
		prometheus.MustRegister(healthCheckDuration)
	})
}

// Handler returns an HTTP handler for the Prometheus metrics endpoint
func Handler() http.Handler {
	return promhttp.Handler()
}

// HTTP Metrics functions
func RecordHTTPRequest(method, route, statusCode string, duration time.Duration, requestSize, responseSize int) {
	httpRequestsTotal.WithLabelValues(method, route, statusCode).Inc()
	httpRequestDuration.WithLabelValues(method, route, statusCode).Observe(duration.Seconds())
	httpRequestSize.WithLabelValues(method, route).Observe(float64(requestSize))
	httpResponseSize.WithLabelValues(method, route, statusCode).Observe(float64(responseSize))
}

func IncActiveRequests() {
	httpActiveRequests.Inc()
}

func DecActiveRequests() {
	httpActiveRequests.Dec()
}

// Authorization Metrics functions
func RecordAuthAttempt(result string) {
	authAttemptsTotal.WithLabelValues(result).Inc()
}

func RecordAuthFailure(errorType string) {
	authFailuresTotal.WithLabelValues(errorType).Inc()
}

func RecordAuthValidationDuration(duration time.Duration) {
	authValidationDuration.Observe(duration.Seconds())
}

func RecordAuthCacheHit(hit bool) {
	if hit {
		authCacheHitsTotal.WithLabelValues("hit").Inc()
	} else {
		authCacheHitsTotal.WithLabelValues("miss").Inc()
	}
}

// Rate Limiting Metrics functions
func RecordRateLimitCheck() {
	rateLimitChecksTotal.Inc()
}

func RecordRateLimitExceeded(keyType, route string) {
	rateLimitExceededTotal.WithLabelValues(keyType, route).Inc()
}

func RecordRateLimitUtilization(keyType string, utilizationPercent float64) {
	rateLimitUtilization.WithLabelValues(keyType).Observe(utilizationPercent)
}

func RecordRateLimitCheckDuration(duration time.Duration) {
	rateLimitCheckDuration.Observe(duration.Seconds())
}

func RecordRateLimitError(errorType string) {
	rateLimitErrorsTotal.WithLabelValues(errorType).Inc()
}

// Backend Metrics functions
func RecordBackendRequest(backendService, statusCode string, duration time.Duration) {
	backendRequestsTotal.WithLabelValues(backendService, statusCode).Inc()
	backendRequestDuration.WithLabelValues(backendService).Observe(duration.Seconds())
}

func RecordBackendError(backendService, errorType string) {
	backendErrorsTotal.WithLabelValues(backendService, errorType).Inc()
}

// Circuit Breaker Metrics functions
func SetCircuitBreakerState(backendService string, state int) {
	circuitBreakerState.WithLabelValues(backendService).Set(float64(state))
}

func RecordCircuitBreakerTransition(backendService, fromState, toState string) {
	circuitBreakerTransitionsTotal.WithLabelValues(backendService, fromState, toState).Inc()
}

// Health Check Metrics functions
func RecordHealthCheck(checkName, status string, duration time.Duration) {
	healthCheckTotal.WithLabelValues(checkName, status).Inc()
	healthCheckDuration.WithLabelValues(checkName).Observe(duration.Seconds())
}
