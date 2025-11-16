package router

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/maltehedderich/api-gateway-go/internal/config"
	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

func init() {
	// Initialize logger for tests
	logger.Init(logger.InfoLevel, "json", os.Stdout)
}

func TestPatternToRegex(t *testing.T) {
	r := New()

	tests := []struct {
		name           string
		pattern        string
		expectedRegex  string
		expectedParams []string
	}{
		{
			name:           "exact match",
			pattern:        "/api/v1/users",
			expectedRegex:  "/api/v1/users",
			expectedParams: []string{},
		},
		{
			name:           "single parameter",
			pattern:        "/api/v1/users/{id}",
			expectedRegex:  "/api/v1/users/([^/]+)",
			expectedParams: []string{"id"},
		},
		{
			name:           "multiple parameters",
			pattern:        "/api/v1/users/{userId}/orders/{orderId}",
			expectedRegex:  "/api/v1/users/([^/]+)/orders/([^/]+)",
			expectedParams: []string{"userId", "orderId"},
		},
		{
			name:           "wildcard segment",
			pattern:        "/api/v1/*",
			expectedRegex:  "/api/v1/[^/]*",
			expectedParams: []string{},
		},
		{
			name:           "wildcard prefix",
			pattern:        "/api/v1/**",
			expectedRegex:  "/api/v1/.*",
			expectedParams: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regex, params := r.patternToRegex(tt.pattern)

			if regex != tt.expectedRegex {
				t.Errorf("expected regex %q, got %q", tt.expectedRegex, regex)
			}

			if len(params) != len(tt.expectedParams) {
				t.Errorf("expected %d params, got %d", len(tt.expectedParams), len(params))
			}

			for i, param := range params {
				if i >= len(tt.expectedParams) || param != tt.expectedParams[i] {
					t.Errorf("param %d: expected %q, got %q", i, tt.expectedParams[i], param)
				}
			}
		})
	}
}

func TestCalculatePriority(t *testing.T) {
	r := New()

	tests := []struct {
		name     string
		patterns []string
		// Expected order after sorting (indices)
		expectedOrder []int
	}{
		{
			name: "exact before wildcard",
			patterns: []string{
				"/api/v1/**",
				"/api/v1/users",
			},
			expectedOrder: []int{1, 0}, // exact match first
		},
		{
			name: "longer exact before shorter exact",
			patterns: []string{
				"/api",
				"/api/v1/users",
			},
			expectedOrder: []int{1, 0}, // longer first
		},
		{
			name: "parameter before double wildcard",
			patterns: []string{
				"/api/**",
				"/api/{id}",
			},
			expectedOrder: []int{1, 0}, // parameter first
		},
		{
			name: "single wildcard before double wildcard",
			patterns: []string{
				"/api/**",
				"/api/*",
			},
			expectedOrder: []int{1, 0}, // single wildcard first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priorities := make([]int, len(tt.patterns))
			for i, pattern := range tt.patterns {
				priorities[i] = r.calculatePriority(pattern)
			}

			// Verify ordering
			for i := 0; i < len(tt.expectedOrder)-1; i++ {
				currentIdx := tt.expectedOrder[i]
				nextIdx := tt.expectedOrder[i+1]

				if priorities[currentIdx] >= priorities[nextIdx] {
					t.Errorf("expected %s (priority %d) to have higher priority than %s (priority %d)",
						tt.patterns[currentIdx], priorities[currentIdx],
						tt.patterns[nextIdx], priorities[nextIdx])
				}
			}
		})
	}
}

func TestRouterMatch(t *testing.T) {
	r := New()

	routes := []config.RouteConfig{
		{
			PathPattern: "/api/v1/users",
			Methods:     []string{"GET", "POST"},
			BackendURL:  "http://localhost:3001",
			Timeout:     10 * time.Second,
			AuthPolicy:  "public",
		},
		{
			PathPattern: "/api/v1/users/{id}",
			Methods:     []string{"GET", "PUT", "DELETE"},
			BackendURL:  "http://localhost:3001",
			Timeout:     10 * time.Second,
			AuthPolicy:  "authenticated",
		},
		{
			PathPattern: "/api/v1/orders/{orderId}/items/{itemId}",
			Methods:     []string{"GET"},
			BackendURL:  "http://localhost:3002",
			Timeout:     5 * time.Second,
			AuthPolicy:  "authenticated",
		},
		{
			PathPattern: "/api/v1/public/**",
			Methods:     []string{"GET"},
			BackendURL:  "http://localhost:3003",
			Timeout:     10 * time.Second,
			AuthPolicy:  "public",
		},
	}

	err := r.LoadRoutes(routes)
	if err != nil {
		t.Fatalf("failed to load routes: %v", err)
	}

	tests := []struct {
		name           string
		method         string
		path           string
		expectMatch    bool
		expectedBackend string
		expectedParams map[string]string
	}{
		{
			name:           "exact match GET",
			method:         "GET",
			path:           "/api/v1/users",
			expectMatch:    true,
			expectedBackend: "http://localhost:3001",
			expectedParams: map[string]string{},
		},
		{
			name:           "exact match POST",
			method:         "POST",
			path:           "/api/v1/users",
			expectMatch:    true,
			expectedBackend: "http://localhost:3001",
			expectedParams: map[string]string{},
		},
		{
			name:        "exact match wrong method",
			method:      "DELETE",
			path:        "/api/v1/users",
			expectMatch: false,
		},
		{
			name:           "parameter match",
			method:         "GET",
			path:           "/api/v1/users/123",
			expectMatch:    true,
			expectedBackend: "http://localhost:3001",
			expectedParams: map[string]string{"id": "123"},
		},
		{
			name:           "multiple parameters",
			method:         "GET",
			path:           "/api/v1/orders/456/items/789",
			expectMatch:    true,
			expectedBackend: "http://localhost:3002",
			expectedParams: map[string]string{"orderId": "456", "itemId": "789"},
		},
		{
			name:           "wildcard match",
			method:         "GET",
			path:           "/api/v1/public/docs/readme.html",
			expectMatch:    true,
			expectedBackend: "http://localhost:3003",
			expectedParams: map[string]string{},
		},
		{
			name:        "no match",
			method:      "GET",
			path:        "/api/v2/users",
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, tt.path, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			match, err := r.Match(req)

			if tt.expectMatch {
				if err != nil {
					t.Errorf("expected match but got error: %v", err)
					return
				}
				if match == nil {
					t.Error("expected match but got nil")
					return
				}

				if match.Route.BackendURL != tt.expectedBackend {
					t.Errorf("expected backend %q, got %q", tt.expectedBackend, match.Route.BackendURL)
				}

				if len(match.Params) != len(tt.expectedParams) {
					t.Errorf("expected %d params, got %d", len(tt.expectedParams), len(match.Params))
				}

				for key, expectedValue := range tt.expectedParams {
					if actualValue, ok := match.Params[key]; !ok {
						t.Errorf("missing parameter %q", key)
					} else if actualValue != expectedValue {
						t.Errorf("param %q: expected %q, got %q", key, expectedValue, actualValue)
					}
				}
			} else {
				if err == nil && match != nil {
					t.Errorf("expected no match but got route: %s", match.Route.PathPattern)
				}
			}
		})
	}
}

func TestRouterPriority(t *testing.T) {
	r := New()

	routes := []config.RouteConfig{
		{
			PathPattern: "/api/**",
			Methods:     []string{"GET"},
			BackendURL:  "http://wildcard",
			Timeout:     10 * time.Second,
		},
		{
			PathPattern: "/api/v1/users",
			Methods:     []string{"GET"},
			BackendURL:  "http://exact",
			Timeout:     10 * time.Second,
		},
		{
			PathPattern: "/api/v1/{resource}",
			Methods:     []string{"GET"},
			BackendURL:  "http://param",
			Timeout:     10 * time.Second,
		},
	}

	err := r.LoadRoutes(routes)
	if err != nil {
		t.Fatalf("failed to load routes: %v", err)
	}

	// Test that exact match takes precedence
	req, _ := http.NewRequest("GET", "/api/v1/users", nil)
	match, err := r.Match(req)
	if err != nil {
		t.Fatalf("expected match: %v", err)
	}
	if match.Route.BackendURL != "http://exact" {
		t.Errorf("expected exact match, got %s", match.Route.BackendURL)
	}

	// Test that parameter match takes precedence over wildcard
	req, _ = http.NewRequest("GET", "/api/v1/orders", nil)
	match, err = r.Match(req)
	if err != nil {
		t.Fatalf("expected match: %v", err)
	}
	if match.Route.BackendURL != "http://param" {
		t.Errorf("expected param match, got %s", match.Route.BackendURL)
	}

	// Test that wildcard matches everything else
	req, _ = http.NewRequest("GET", "/api/v2/something", nil)
	match, err = r.Match(req)
	if err != nil {
		t.Fatalf("expected match: %v", err)
	}
	if match.Route.BackendURL != "http://wildcard" {
		t.Errorf("expected wildcard match, got %s", match.Route.BackendURL)
	}
}
