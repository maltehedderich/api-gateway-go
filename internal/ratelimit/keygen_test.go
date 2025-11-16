package ratelimit

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/maltehedderich/api-gateway-go/internal/auth"
)

func TestKeyGenerator_GenerateKey_IP(t *testing.T) {
	kg := NewKeyGenerator("ip")

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	key, ok := kg.GenerateKey(req)
	if !ok {
		t.Fatal("expected key generation to succeed")
	}

	expectedKey := "ratelimit:ip:192.168.1.100"
	if key != expectedKey {
		t.Errorf("expected key %s, got %s", expectedKey, key)
	}
}

func TestKeyGenerator_GenerateKey_XForwardedFor(t *testing.T) {
	kg := NewKeyGenerator("ip")

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1")

	key, ok := kg.GenerateKey(req)
	if !ok {
		t.Fatal("expected key generation to succeed")
	}

	// Should use first IP from X-Forwarded-For
	expectedKey := "ratelimit:ip:203.0.113.1"
	if key != expectedKey {
		t.Errorf("expected key %s, got %s", expectedKey, key)
	}
}

func TestKeyGenerator_GenerateKey_XRealIP(t *testing.T) {
	kg := NewKeyGenerator("ip")

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Real-IP", "203.0.113.5")

	key, ok := kg.GenerateKey(req)
	if !ok {
		t.Fatal("expected key generation to succeed")
	}

	expectedKey := "ratelimit:ip:203.0.113.5"
	if key != expectedKey {
		t.Errorf("expected key %s, got %s", expectedKey, key)
	}
}

func TestKeyGenerator_GenerateKey_User(t *testing.T) {
	kg := NewKeyGenerator("user")

	req := httptest.NewRequest("GET", "/test", nil)

	// Add user context
	userCtx := &auth.UserContext{
		UserID:    "user123",
		SessionID: "session456",
		Roles:     []string{"user"},
	}
	ctx := auth.SetUserContext(context.Background(), userCtx)
	req = req.WithContext(ctx)

	key, ok := kg.GenerateKey(req)
	if !ok {
		t.Fatal("expected key generation to succeed")
	}

	expectedKey := "ratelimit:user:user123"
	if key != expectedKey {
		t.Errorf("expected key %s, got %s", expectedKey, key)
	}
}

func TestKeyGenerator_GenerateKey_UserNoAuth(t *testing.T) {
	kg := NewKeyGenerator("user")

	req := httptest.NewRequest("GET", "/test", nil)

	// No user context - should fail
	key, ok := kg.GenerateKey(req)
	if ok {
		t.Errorf("expected key generation to fail for user template without auth, got key: %s", key)
	}
}

func TestKeyGenerator_GenerateKey_Route(t *testing.T) {
	kg := NewKeyGenerator("route")

	req := httptest.NewRequest("GET", "/api/v1/users", nil)

	key, ok := kg.GenerateKey(req)
	if !ok {
		t.Fatal("expected key generation to succeed")
	}

	expectedKey := "ratelimit:route:/api/v1/users"
	if key != expectedKey {
		t.Errorf("expected key %s, got %s", expectedKey, key)
	}
}

func TestKeyGenerator_GenerateKey_Composite_UserRoute(t *testing.T) {
	kg := NewKeyGenerator("user:route")

	req := httptest.NewRequest("GET", "/api/v1/orders", nil)

	// Add user context
	userCtx := &auth.UserContext{
		UserID:    "user789",
		SessionID: "session123",
	}
	ctx := auth.SetUserContext(context.Background(), userCtx)
	req = req.WithContext(ctx)

	key, ok := kg.GenerateKey(req)
	if !ok {
		t.Fatal("expected key generation to succeed")
	}

	expectedKey := "ratelimit:user:user789:route:/api/v1/orders"
	if key != expectedKey {
		t.Errorf("expected key %s, got %s", expectedKey, key)
	}
}

func TestKeyGenerator_GenerateKey_Composite_IPRoute(t *testing.T) {
	kg := NewKeyGenerator("ip:route")

	req := httptest.NewRequest("POST", "/api/v1/login", nil)
	req.RemoteAddr = "192.168.1.50:54321"

	key, ok := kg.GenerateKey(req)
	if !ok {
		t.Fatal("expected key generation to succeed")
	}

	expectedKey := "ratelimit:ip:192.168.1.50:route:/api/v1/login"
	if key != expectedKey {
		t.Errorf("expected key %s, got %s", expectedKey, key)
	}
}

func TestKeyGenerator_GenerateKey_InvalidTemplate(t *testing.T) {
	kg := NewKeyGenerator("invalid")

	req := httptest.NewRequest("GET", "/test", nil)

	// Should fail for unknown template
	key, ok := kg.GenerateKey(req)
	if ok {
		t.Errorf("expected key generation to fail for invalid template, got key: %s", key)
	}
}

func TestKeyGenerator_GenerateKey_EmptyTemplate(t *testing.T) {
	kg := NewKeyGenerator("")

	req := httptest.NewRequest("GET", "/test", nil)

	// Should fail for empty template
	key, ok := kg.GenerateKey(req)
	if ok {
		t.Errorf("expected key generation to fail for empty template, got key: %s", key)
	}
}

func TestKeyGenerator_GetClientIP_RemoteAddr(t *testing.T) {
	kg := NewKeyGenerator("ip")

	tests := []struct {
		name       string
		remoteAddr string
		expectedIP string
	}{
		{
			name:       "IPv4 with port",
			remoteAddr: "192.168.1.100:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "IPv4 without port",
			remoteAddr: "192.168.1.100",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "IPv6 with port",
			remoteAddr: "[2001:db8::1]:8080",
			expectedIP: "[2001:db8::1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			ip := kg.getClientIP(req)
			if ip != tt.expectedIP {
				t.Errorf("expected IP %s, got %s", tt.expectedIP, ip)
			}
		})
	}
}

func TestKeyGenerator_GetRoute(t *testing.T) {
	kg := NewKeyGenerator("route")

	tests := []struct {
		name          string
		path          string
		expectedRoute string
	}{
		{
			name:          "simple path",
			path:          "/api/users",
			expectedRoute: "/api/users",
		},
		{
			name:          "path with query",
			path:          "/api/users?page=1",
			expectedRoute: "/api/users",
		},
		{
			name:          "root path",
			path:          "/",
			expectedRoute: "/",
		},
		{
			name:          "nested path",
			path:          "/api/v1/users/123/orders",
			expectedRoute: "/api/v1/users/123/orders",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			route := kg.getRoute(req)
			if route != tt.expectedRoute {
				t.Errorf("expected route %s, got %s", tt.expectedRoute, route)
			}
		})
	}
}
