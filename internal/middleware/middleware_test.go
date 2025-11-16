package middleware

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/maltehedderich/api-gateway-go/internal/config"
	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

// TestRecovery tests the panic recovery middleware
func TestRecovery(t *testing.T) {
	// Initialize logger
	logger.Init(logger.InfoLevel, "json", os.Stdout)

	tests := []struct {
		name           string
		handler        http.HandlerFunc
		expectPanic    bool
		expectedStatus int
	}{
		{
			name: "No panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("success"))
			},
			expectPanic:    false,
			expectedStatus: http.StatusOK,
		},
		{
			name: "Panic recovered",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic("test panic")
			},
			expectPanic:    true,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "Panic with correlation ID",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic("test panic with correlation")
			},
			expectPanic:    true,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.name == "Panic with correlation ID" {
				ctx := logger.WithCorrelationID(req.Context(), "test-correlation-123")
				req = req.WithContext(ctx)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Create middleware chain
			middleware := Recovery()
			handler := middleware(tt.handler)

			// Execute request
			handler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// If panic was expected, verify error response
			if tt.expectPanic {
				var response map[string]interface{}
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if response["error"] != "internal_server_error" {
					t.Errorf("expected error code 'internal_server_error', got %v", response["error"])
				}

				if tt.name == "Panic with correlation ID" {
					if response["correlation_id"] != "test-correlation-123" {
						t.Errorf("expected correlation_id 'test-correlation-123', got %v", response["correlation_id"])
					}
				}
			}
		})
	}
}

// TestLogging tests the request logging middleware
func TestLogging(t *testing.T) {
	// Initialize logger
	logger.Init(logger.InfoLevel, "json", os.Stdout)

	tests := []struct {
		name           string
		method         string
		path           string
		statusCode     int
		expectedStatus int
	}{
		{
			name:           "Success request",
			method:         "GET",
			path:           "/api/users",
			statusCode:     http.StatusOK,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Client error",
			method:         "POST",
			path:           "/api/invalid",
			statusCode:     http.StatusBadRequest,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Server error",
			method:         "GET",
			path:           "/api/error",
			statusCode:     http.StatusInternalServerError,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			ctx := logger.WithCorrelationID(req.Context(), "test-correlation")
			req = req.WithContext(ctx)

			// Create response recorder
			rr := httptest.NewRecorder()

			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte("test response"))
			})

			// Create middleware chain
			middleware := Logging()
			wrappedHandler := middleware(handler)

			// Execute request
			wrappedHandler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}

// TestCorrelationID tests the correlation ID middleware
func TestCorrelationID(t *testing.T) {
	tests := []struct {
		name               string
		incomingHeaderID   string
		expectGenerated    bool
		expectedInResponse bool
	}{
		{
			name:               "No incoming correlation ID",
			incomingHeaderID:   "",
			expectGenerated:    true,
			expectedInResponse: true,
		},
		{
			name:               "Existing correlation ID",
			incomingHeaderID:   "existing-123",
			expectGenerated:    false,
			expectedInResponse: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.incomingHeaderID != "" {
				req.Header.Set(CorrelationIDHeader, tt.incomingHeaderID)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Create test handler
			var contextCorrelationID string
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				contextCorrelationID = logger.GetCorrelationID(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			// Create middleware chain
			middleware := CorrelationID()
			wrappedHandler := middleware(handler)

			// Execute request
			wrappedHandler.ServeHTTP(rr, req)

			// Check response header
			responseHeaderID := rr.Header().Get(CorrelationIDHeader)
			if !tt.expectedInResponse && responseHeaderID == "" {
				t.Error("expected correlation ID in response header, got none")
			}

			if tt.expectedInResponse && responseHeaderID == "" {
				t.Error("expected correlation ID in response header, got none")
			}

			// Check if existing ID was preserved
			if !tt.expectGenerated && responseHeaderID != tt.incomingHeaderID {
				t.Errorf("expected correlation ID %s, got %s", tt.incomingHeaderID, responseHeaderID)
			}

			// Check context correlation ID
			if contextCorrelationID == "" {
				t.Error("expected correlation ID in context, got empty string")
			}

			if !tt.expectGenerated && contextCorrelationID != tt.incomingHeaderID {
				t.Errorf("expected context correlation ID %s, got %s", tt.incomingHeaderID, contextCorrelationID)
			}
		})
	}
}

// TestSecurity tests the security headers middleware
func TestSecurity(t *testing.T) {
	tests := []struct {
		name           string
		config         *SecurityConfig
		expectedHeaders map[string]string
	}{
		{
			name: "HSTS enabled",
			config: &SecurityConfig{
				EnableHSTS: true,
				HSTSMaxAge: 31536000,
				HSTSIncludeSubdomains: true,
				HSTSPreload: true,
			},
			expectedHeaders: map[string]string{
				"Strict-Transport-Security": "max-age=31536000; includeSubDomains; preload",
			},
		},
		{
			name: "All security headers",
			config: &SecurityConfig{
				EnableHSTS:            true,
				HSTSMaxAge:            31536000,
				ContentSecurityPolicy: "default-src 'self'",
				FrameOptions:          "DENY",
				ContentTypeNosniff:    true,
				XSSProtection:         true,
				XSSBlockMode:          true,
				ReferrerPolicy:        "strict-origin-when-cross-origin",
				PermissionsPolicy:     "geolocation=()",
			},
			expectedHeaders: map[string]string{
				"Strict-Transport-Security": "max-age=31536000",
				"Content-Security-Policy":   "default-src 'self'",
				"X-Frame-Options":           "DENY",
				"X-Content-Type-Options":    "nosniff",
				"X-XSS-Protection":          "1; mode=block",
				"Referrer-Policy":           "strict-origin-when-cross-origin",
				"Permissions-Policy":        "geolocation=()",
			},
		},
		{
			name: "XSS protection without block mode",
			config: &SecurityConfig{
				XSSProtection: true,
				XSSBlockMode:  false,
			},
			expectedHeaders: map[string]string{
				"X-XSS-Protection": "1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request
			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()

			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Create middleware chain
			middleware := Security(tt.config)
			wrappedHandler := middleware(handler)

			// Execute request
			wrappedHandler.ServeHTTP(rr, req)

			// Check expected headers
			for headerName, expectedValue := range tt.expectedHeaders {
				actualValue := rr.Header().Get(headerName)
				if actualValue != expectedValue {
					t.Errorf("expected header %s=%s, got %s", headerName, expectedValue, actualValue)
				}
			}
		})
	}
}

// TestHTTPSRedirect tests the HTTPS redirect middleware
func TestHTTPSRedirect(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func(*http.Request)
		path           string
		expectRedirect bool
		expectedURL    string
	}{
		{
			name: "HTTPS request - no redirect",
			setupRequest: func(r *http.Request) {
				r.TLS = &tls.ConnectionState{} // simulate TLS
			},
			path:           "/api/users",
			expectRedirect: false,
		},
		{
			name: "HTTP request with X-Forwarded-Proto=https - no redirect",
			setupRequest: func(r *http.Request) {
				r.Header.Set("X-Forwarded-Proto", "https")
			},
			path:           "/api/users",
			expectRedirect: false,
		},
		{
			name: "Health check path - no redirect",
			setupRequest: func(r *http.Request) {
				// No TLS, no X-Forwarded-Proto
			},
			path:           "/_health/live",
			expectRedirect: false,
		},
		{
			name: "HTTP request - redirect to HTTPS",
			setupRequest: func(r *http.Request) {
				// No TLS, no X-Forwarded-Proto
			},
			path:           "/api/users",
			expectRedirect: true,
			expectedURL:    "https://example.com/api/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request
			req := httptest.NewRequest("GET", tt.path, nil)
			req.Host = "example.com"
			tt.setupRequest(req)

			rr := httptest.NewRecorder()

			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Create middleware chain
			middleware := HTTPSRedirect()
			wrappedHandler := middleware(handler)

			// Execute request
			wrappedHandler.ServeHTTP(rr, req)

			if tt.expectRedirect {
				if rr.Code != http.StatusMovedPermanently {
					t.Errorf("expected status %d, got %d", http.StatusMovedPermanently, rr.Code)
				}

				location := rr.Header().Get("Location")
				if location != tt.expectedURL {
					t.Errorf("expected redirect to %s, got %s", tt.expectedURL, location)
				}
			} else {
				if rr.Code != http.StatusOK {
					t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
				}
			}
		})
	}
}

// TestInputValidation tests the input validation middleware
func TestInputValidation(t *testing.T) {
	// Initialize logger
	logger.Init(logger.InfoLevel, "json", os.Stdout)

	tests := []struct {
		name           string
		config         *config.SecurityConfig
		method         string
		path           string
		userAgent      string
		expectedStatus int
		expectedError  string
	}{
		{
			name: "Valid request",
			config: &config.SecurityConfig{
				AllowedMethods:   []string{"GET", "POST"},
				MaxURLPathLength: 100,
			},
			method:         "GET",
			path:           "/api/users",
			expectedStatus: http.StatusOK,
		},
		{
			name: "Method not allowed",
			config: &config.SecurityConfig{
				AllowedMethods: []string{"GET", "POST"},
			},
			method:         "DELETE",
			path:           "/api/users",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "method_not_allowed",
		},
		{
			name: "URL path too long",
			config: &config.SecurityConfig{
				MaxURLPathLength: 10,
			},
			method:         "GET",
			path:           "/api/users/123456789012345",
			expectedStatus: http.StatusRequestURITooLong,
			expectedError:  "uri_too_long",
		},
		{
			name: "Blocked user agent",
			config: &config.SecurityConfig{
				BlockedUserAgents: []string{"badbot", "malicious"},
			},
			method:         "GET",
			path:           "/api/users",
			userAgent:      "badbot/1.0",
			expectedStatus: http.StatusForbidden,
			expectedError:  "forbidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			ctx := logger.WithCorrelationID(req.Context(), "test-correlation")
			req = req.WithContext(ctx)

			if tt.userAgent != "" {
				req.Header.Set("User-Agent", tt.userAgent)
			}

			rr := httptest.NewRecorder()

			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Create middleware chain
			middleware := InputValidation(tt.config)
			wrappedHandler := middleware(handler)

			// Execute request
			wrappedHandler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check error response if expected
			if tt.expectedError != "" {
				var response map[string]interface{}
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if response["error"] != tt.expectedError {
					t.Errorf("expected error %s, got %v", tt.expectedError, response["error"])
				}
			}
		})
	}
}

// TestGetClientIP tests the getClientIP utility function
func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		setupRequest func(*http.Request)
		expectedIP string
	}{
		{
			name: "X-Forwarded-For header",
			setupRequest: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "192.168.1.100, 10.0.0.1")
			},
			expectedIP: "192.168.1.100",
		},
		{
			name: "X-Real-IP header",
			setupRequest: func(r *http.Request) {
				r.Header.Set("X-Real-IP", "192.168.1.200")
			},
			expectedIP: "192.168.1.200",
		},
		{
			name: "RemoteAddr",
			setupRequest: func(r *http.Request) {
				r.RemoteAddr = "192.168.1.50:12345"
			},
			expectedIP: "192.168.1.50",
		},
		{
			name: "IPv6 RemoteAddr",
			setupRequest: func(r *http.Request) {
				r.RemoteAddr = "[::1]:12345"
			},
			expectedIP: "::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			tt.setupRequest(req)

			ip := getClientIP(req)
			if ip != tt.expectedIP {
				t.Errorf("expected IP %s, got %s", tt.expectedIP, ip)
			}
		})
	}
}

// TestResponseWriter tests the ResponseWriter utility
func TestResponseWriter(t *testing.T) {
	t.Run("Status and Size tracking", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rw := NewResponseWriter(rr)

		// Write header
		rw.WriteHeader(http.StatusCreated)

		// Write body
		data := []byte("test response")
		n, err := rw.Write(data)
		if err != nil {
			t.Fatalf("failed to write: %v", err)
		}

		if n != len(data) {
			t.Errorf("expected to write %d bytes, wrote %d", len(data), n)
		}

		// Check status
		if rw.Status() != http.StatusCreated {
			t.Errorf("expected status %d, got %d", http.StatusCreated, rw.Status())
		}

		// Check size
		if rw.Size() != len(data) {
			t.Errorf("expected size %d, got %d", len(data), rw.Size())
		}
	})

	t.Run("Default status OK", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rw := NewResponseWriter(rr)

		// Write without explicit WriteHeader
		_, _ = rw.Write([]byte("test"))

		if rw.Status() != http.StatusOK {
			t.Errorf("expected default status %d, got %d", http.StatusOK, rw.Status())
		}
	})

	t.Run("Multiple WriteHeader calls", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rw := NewResponseWriter(rr)

		// First call should set the status
		rw.WriteHeader(http.StatusCreated)

		// Second call should be ignored
		rw.WriteHeader(http.StatusBadRequest)

		if rw.Status() != http.StatusCreated {
			t.Errorf("expected status %d, got %d", http.StatusCreated, rw.Status())
		}
	})
}

// TestWriteJSON tests the JSON writing utility
func TestWriteJSON(t *testing.T) {
	t.Run("Valid JSON", func(t *testing.T) {
		rr := httptest.NewRecorder()

		data := map[string]interface{}{
			"message": "success",
			"count":   42,
		}

		err := WriteJSON(rr, data)
		if err != nil {
			t.Fatalf("failed to write JSON: %v", err)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode JSON: %v", err)
		}

		if result["message"] != "success" {
			t.Errorf("expected message 'success', got %v", result["message"])
		}

		if result["count"].(float64) != 42 {
			t.Errorf("expected count 42, got %v", result["count"])
		}
	})
}

// TestIsMethodAllowed tests the method validation helper
func TestIsMethodAllowed(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		allowedMethods []string
		expected       bool
	}{
		{
			name:           "Allowed method",
			method:         "GET",
			allowedMethods: []string{"GET", "POST"},
			expected:       true,
		},
		{
			name:           "Case insensitive",
			method:         "post",
			allowedMethods: []string{"GET", "POST"},
			expected:       true,
		},
		{
			name:           "Not allowed",
			method:         "DELETE",
			allowedMethods: []string{"GET", "POST"},
			expected:       false,
		},
		{
			name:           "Empty allowed list",
			method:         "GET",
			allowedMethods: []string{},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMethodAllowed(tt.method, tt.allowedMethods)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsUserAgentBlocked tests the user agent blocking helper
func TestIsUserAgentBlocked(t *testing.T) {
	tests := []struct {
		name          string
		userAgent     string
		blockedAgents []string
		expected      bool
	}{
		{
			name:          "Blocked agent",
			userAgent:     "BadBot/1.0",
			blockedAgents: []string{"badbot", "malicious"},
			expected:      true,
		},
		{
			name:          "Case insensitive",
			userAgent:     "BADBOT/2.0",
			blockedAgents: []string{"badbot"},
			expected:      true,
		},
		{
			name:          "Partial match",
			userAgent:     "Mozilla/5.0 (compatible; badbot/1.0)",
			blockedAgents: []string{"badbot"},
			expected:      true,
		},
		{
			name:          "Not blocked",
			userAgent:     "Mozilla/5.0",
			blockedAgents: []string{"badbot"},
			expected:      false,
		},
		{
			name:          "Empty blocked list",
			userAgent:     "BadBot/1.0",
			blockedAgents: []string{},
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isUserAgentBlocked(tt.userAgent, tt.blockedAgents)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsHealthCheckPath tests the health check path helper
func TestIsHealthCheckPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Health check root",
			path:     "/_health",
			expected: true,
		},
		{
			name:     "Health check ready",
			path:     "/_health/ready",
			expected: true,
		},
		{
			name:     "Health check live",
			path:     "/_health/live",
			expected: true,
		},
		{
			name:     "Not health check",
			path:     "/api/users",
			expected: false,
		},
		{
			name:     "Partial match",
			path:     "/_health/other",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHealthCheckPath(tt.path)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestBuildHSTSHeader tests the HSTS header builder
func TestBuildHSTSHeader(t *testing.T) {
	tests := []struct {
		name     string
		config   *SecurityConfig
		expected string
	}{
		{
			name: "Max age only",
			config: &SecurityConfig{
				HSTSMaxAge: 31536000,
			},
			expected: "max-age=31536000",
		},
		{
			name: "With includeSubDomains",
			config: &SecurityConfig{
				HSTSMaxAge:            31536000,
				HSTSIncludeSubdomains: true,
			},
			expected: "max-age=31536000; includeSubDomains",
		},
		{
			name: "With preload",
			config: &SecurityConfig{
				HSTSMaxAge:  31536000,
				HSTSPreload: true,
			},
			expected: "max-age=31536000; preload",
		},
		{
			name: "All options",
			config: &SecurityConfig{
				HSTSMaxAge:            31536000,
				HSTSIncludeSubdomains: true,
				HSTSPreload:           true,
			},
			expected: "max-age=31536000; includeSubDomains; preload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildHSTSHeader(tt.config)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
