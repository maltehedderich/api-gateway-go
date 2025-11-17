package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.checks == nil {
		t.Fatal("expected non-nil checks map")
	}
}

func TestRegisterUnregister(t *testing.T) {
	m := NewManager()

	// Register a check
	checker := func() Check {
		return Check{
			Name:   "test",
			Status: StatusHealthy,
		}
	}

	m.Register("test", checker)

	// Verify check is registered
	m.mu.RLock()
	if _, exists := m.checks["test"]; !exists {
		t.Error("expected check to be registered")
	}
	m.mu.RUnlock()

	// Unregister
	m.Unregister("test")

	// Verify check is removed
	m.mu.RLock()
	if _, exists := m.checks["test"]; exists {
		t.Error("expected check to be unregistered")
	}
	m.mu.RUnlock()
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name           string
		checks         map[string]Checker
		expectedStatus Status
	}{
		{
			name:           "No checks - healthy",
			checks:         map[string]Checker{},
			expectedStatus: StatusHealthy,
		},
		{
			name: "All checks healthy",
			checks: map[string]Checker{
				"check1": func() Check {
					return Check{Name: "check1", Status: StatusHealthy}
				},
				"check2": func() Check {
					return Check{Name: "check2", Status: StatusHealthy}
				},
			},
			expectedStatus: StatusHealthy,
		},
		{
			name: "One check degraded",
			checks: map[string]Checker{
				"check1": func() Check {
					return Check{Name: "check1", Status: StatusHealthy}
				},
				"check2": func() Check {
					return Check{Name: "check2", Status: StatusDegraded}
				},
			},
			expectedStatus: StatusDegraded,
		},
		{
			name: "One check unhealthy",
			checks: map[string]Checker{
				"check1": func() Check {
					return Check{Name: "check1", Status: StatusHealthy}
				},
				"check2": func() Check {
					return Check{Name: "check2", Status: StatusUnhealthy, Error: "error"}
				},
			},
			expectedStatus: StatusUnhealthy,
		},
		{
			name: "Unhealthy overrides degraded",
			checks: map[string]Checker{
				"check1": func() Check {
					return Check{Name: "check1", Status: StatusDegraded}
				},
				"check2": func() Check {
					return Check{Name: "check2", Status: StatusUnhealthy, Error: "error"}
				},
			},
			expectedStatus: StatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			for name, checker := range tt.checks {
				m.Register(name, checker)
			}

			response := m.Check()

			if response.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, response.Status)
			}

			if response.Timestamp == "" {
				t.Error("expected non-empty timestamp")
			}

			if len(response.Checks) != len(tt.checks) {
				t.Errorf("expected %d checks, got %d", len(tt.checks), len(response.Checks))
			}
		})
	}
}

func TestLivenessHandler(t *testing.T) {
	m := NewManager()
	handler := m.LivenessHandler()

	req := httptest.NewRequest("GET", "/_health/live", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response Response
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != StatusHealthy {
		t.Errorf("expected status %s, got %s", StatusHealthy, response.Status)
	}

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Error("expected Content-Type: application/json")
	}
}

func TestReadinessHandler(t *testing.T) {
	tests := []struct {
		name           string
		checks         map[string]Checker
		expectedStatus int
		expectedHealth Status
	}{
		{
			name:           "No checks - healthy",
			checks:         map[string]Checker{},
			expectedStatus: http.StatusOK,
			expectedHealth: StatusHealthy,
		},
		{
			name: "All healthy",
			checks: map[string]Checker{
				"check1": func() Check {
					return Check{Name: "check1", Status: StatusHealthy}
				},
			},
			expectedStatus: http.StatusOK,
			expectedHealth: StatusHealthy,
		},
		{
			name: "Degraded - returns 503",
			checks: map[string]Checker{
				"check1": func() Check {
					return Check{Name: "check1", Status: StatusDegraded}
				},
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedHealth: StatusDegraded,
		},
		{
			name: "Unhealthy - returns 503",
			checks: map[string]Checker{
				"check1": func() Check {
					return Check{Name: "check1", Status: StatusUnhealthy, Error: "error"}
				},
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedHealth: StatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			for name, checker := range tt.checks {
				m.Register(name, checker)
			}

			handler := m.ReadinessHandler()

			req := httptest.NewRequest("GET", "/_health/ready", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			var response Response
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if response.Status != tt.expectedHealth {
				t.Errorf("expected status %s, got %s", tt.expectedHealth, response.Status)
			}
		})
	}
}

func TestHealthHandler(t *testing.T) {
	m := NewManager()

	m.Register("test", func() Check {
		return Check{
			Name:   "test",
			Status: StatusUnhealthy,
			Error:  "test error",
		}
	})

	handler := m.HealthHandler()

	req := httptest.NewRequest("GET", "/_health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Health handler always returns 200, even if checks are unhealthy
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response Response
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != StatusUnhealthy {
		t.Errorf("expected status %s, got %s", StatusUnhealthy, response.Status)
	}

	if check, ok := response.Checks["test"]; ok {
		if check.Error != "test error" {
			t.Errorf("expected error 'test error', got %s", check.Error)
		}
	} else {
		t.Error("expected 'test' check in response")
	}
}

func TestConfigChecker(t *testing.T) {
	tests := []struct {
		name           string
		isValid        func() bool
		expectedStatus Status
		expectError    bool
	}{
		{
			name: "Valid config",
			isValid: func() bool {
				return true
			},
			expectedStatus: StatusHealthy,
			expectError:    false,
		},
		{
			name: "Invalid config",
			isValid: func() bool {
				return false
			},
			expectedStatus: StatusUnhealthy,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := ConfigChecker(tt.isValid)
			check := checker()

			if check.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, check.Status)
			}

			if tt.expectError && check.Error == "" {
				t.Error("expected error message")
			}

			if !tt.expectError && check.Error != "" {
				t.Errorf("expected no error, got %s", check.Error)
			}
		})
	}
}

func TestRedisChecker(t *testing.T) {
	tests := []struct {
		name           string
		ping           func() error
		expectedStatus Status
		expectError    bool
	}{
		{
			name: "Successful ping",
			ping: func() error {
				return nil
			},
			expectedStatus: StatusHealthy,
			expectError:    false,
		},
		{
			name: "Failed ping",
			ping: func() error {
				return errors.New("connection refused")
			},
			expectedStatus: StatusUnhealthy,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := RedisChecker(tt.ping)
			check := checker()

			if check.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, check.Status)
			}

			if tt.expectError && check.Error == "" {
				t.Error("expected error message")
			}

			if !tt.expectError && check.Error != "" {
				t.Errorf("expected no error, got %s", check.Error)
			}
		})
	}
}

func TestHTTPChecker(t *testing.T) {
	// Create test servers
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthyServer.Close()

	unhealthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer unhealthyServer.Close()

	tests := []struct {
		name           string
		url            string
		timeout        time.Duration
		expectedStatus Status
	}{
		{
			name:           "Healthy endpoint",
			url:            healthyServer.URL,
			timeout:        2 * time.Second,
			expectedStatus: StatusHealthy,
		},
		{
			name:           "Unhealthy endpoint",
			url:            unhealthyServer.URL,
			timeout:        2 * time.Second,
			expectedStatus: StatusUnhealthy,
		},
		{
			name:           "Invalid URL",
			url:            "http://invalid-host-that-does-not-exist:99999",
			timeout:        1 * time.Second,
			expectedStatus: StatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := HTTPChecker("test", tt.url, tt.timeout)
			check := checker()

			if check.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, check.Status)
			}
		})
	}
}

type mockPinger struct {
	shouldFail bool
}

func (m *mockPinger) Ping(ctx context.Context) error {
	if m.shouldFail {
		return errors.New("ping failed")
	}
	return nil
}

func TestRateLimiterChecker(t *testing.T) {
	tests := []struct {
		name           string
		pinger         Pinger
		expectedStatus Status
		expectError    bool
	}{
		{
			name:           "Successful ping",
			pinger:         &mockPinger{shouldFail: false},
			expectedStatus: StatusHealthy,
			expectError:    false,
		},
		{
			name:           "Failed ping",
			pinger:         &mockPinger{shouldFail: true},
			expectedStatus: StatusUnhealthy,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := RateLimiterChecker(tt.pinger)
			check := checker()

			if check.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, check.Status)
			}

			if tt.expectError && check.Error == "" {
				t.Error("expected error message")
			}

			if !tt.expectError && check.Error != "" {
				t.Errorf("expected no error, got %s", check.Error)
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager()

	// Add initial check
	m.Register("test", func() Check {
		return Check{Name: "test", Status: StatusHealthy}
	})

	// Concurrent operations
	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			_ = m.Check()
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(id int) {
			m.Register(string(rune(id)), func() Check {
				return Check{Name: string(rune(id)), Status: StatusHealthy}
			})
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}
