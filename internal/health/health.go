package health

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// Check represents a health check result
type Check struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Response represents the health check response
type Response struct {
	Status    Status           `json:"status"`
	Timestamp string           `json:"timestamp"`
	Checks    map[string]Check `json:"checks,omitempty"`
}

// Checker is a function that performs a health check
type Checker func() Check

// Manager manages health checks
type Manager struct {
	checks map[string]Checker
	mu     sync.RWMutex
}

// NewManager creates a new health check manager
func NewManager() *Manager {
	return &Manager{
		checks: make(map[string]Checker),
	}
}

// Register registers a health check
func (m *Manager) Register(name string, checker Checker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checks[name] = checker
}

// Unregister removes a health check
func (m *Manager) Unregister(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.checks, name)
}

// Check runs all health checks
func (m *Manager) Check() Response {
	m.mu.RLock()
	defer m.mu.RUnlock()

	checks := make(map[string]Check)
	overallStatus := StatusHealthy

	for name, checker := range m.checks {
		check := checker()
		checks[name] = check

		// Update overall status
		if check.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
		} else if check.Status == StatusDegraded && overallStatus == StatusHealthy {
			overallStatus = StatusDegraded
		}
	}

	return Response{
		Status:    overallStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	}
}

// LivenessHandler returns a handler for liveness probes
// Liveness indicates if the application is running
func (m *Manager) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Liveness is simple - if we can respond, we're alive
		response := Response{
			Status:    StatusHealthy,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

// ReadinessHandler returns a handler for readiness probes
// Readiness indicates if the application is ready to serve traffic
func (m *Manager) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := m.Check()

		w.Header().Set("Content-Type", "application/json")

		if response.Status == StatusHealthy {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(response)
	}
}

// HealthHandler returns a general health check handler
func (m *Manager) HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := m.Check()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

// Predefined health checkers

// ConfigChecker checks if configuration is valid
func ConfigChecker(isValid func() bool) Checker {
	return func() Check {
		if isValid() {
			return Check{
				Name:   "config",
				Status: StatusHealthy,
			}
		}
		return Check{
			Name:   "config",
			Status: StatusUnhealthy,
			Error:  "configuration is invalid",
		}
	}
}

// RedisChecker checks Redis connectivity
func RedisChecker(ping func() error) Checker {
	return func() Check {
		if err := ping(); err != nil {
			return Check{
				Name:   "redis",
				Status: StatusUnhealthy,
				Error:  err.Error(),
			}
		}
		return Check{
			Name:   "redis",
			Status: StatusHealthy,
		}
	}
}

// HTTPChecker checks HTTP endpoint connectivity
func HTTPChecker(name, url string, timeout time.Duration) Checker {
	return func() Check {
		client := &http.Client{
			Timeout: timeout,
		}

		resp, err := client.Get(url)
		if err != nil {
			return Check{
				Name:   name,
				Status: StatusUnhealthy,
				Error:  err.Error(),
			}
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return Check{
				Name:   name,
				Status: StatusHealthy,
			}
		}

		return Check{
			Name:   name,
			Status: StatusUnhealthy,
			Error:  "non-2xx status code",
		}
	}
}
