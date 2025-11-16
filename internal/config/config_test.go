package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigFromYAML(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
server:
  http_port: 9000
  https_port: 9443
logging:
  level: debug
  format: json
authorization:
  enabled: true
  cookie_name: test_session
  jwt_shared_secret: test-secret-key
rate_limit:
  enabled: true
  backend: memory
`

	if err := os.WriteFile(configFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Server.HTTPPort != 9000 {
		t.Errorf("Expected HTTP port 9000, got %d", cfg.Server.HTTPPort)
	}
	if cfg.Server.HTTPSPort != 9443 {
		t.Errorf("Expected HTTPS port 9443, got %d", cfg.Server.HTTPSPort)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected log level debug, got %s", cfg.Logging.Level)
	}
	if cfg.Authorization.CookieName != "test_session" {
		t.Errorf("Expected cookie name test_session, got %s", cfg.Authorization.CookieName)
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()

	if cfg.Server.HTTPPort != 8080 {
		t.Errorf("Expected default HTTP port 8080, got %d", cfg.Server.HTTPPort)
	}
	if cfg.Server.HTTPSPort != 8443 {
		t.Errorf("Expected default HTTPS port 8443, got %d", cfg.Server.HTTPSPort)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Expected default log level info, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Expected default log format json, got %s", cfg.Logging.Format)
	}
	if cfg.Authorization.CookieName != "session_token" {
		t.Errorf("Expected default cookie name session_token, got %s", cfg.Authorization.CookieName)
	}
	if cfg.RateLimit.Backend != "memory" {
		t.Errorf("Expected default rate limit backend memory, got %s", cfg.RateLimit.Backend)
	}
}

func TestEnvOverrides(t *testing.T) {
	// Set environment variables
	_ = os.Setenv("GATEWAY_HTTP_PORT", "7000")
	_ = os.Setenv("GATEWAY_LOG_LEVEL", "warn")
	_ = os.Setenv("GATEWAY_AUTH_ENABLED", "false")
	defer func() {
		_ = os.Unsetenv("GATEWAY_HTTP_PORT")
		_ = os.Unsetenv("GATEWAY_LOG_LEVEL")
		_ = os.Unsetenv("GATEWAY_AUTH_ENABLED")
	}()

	cfg := &Config{}
	cfg.setDefaults()

	if err := applyEnvOverrides(cfg); err != nil {
		t.Fatalf("Failed to apply env overrides: %v", err)
	}

	if cfg.Server.HTTPPort != 7000 {
		t.Errorf("Expected HTTP port 7000 from env, got %d", cfg.Server.HTTPPort)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("Expected log level warn from env, got %s", cfg.Logging.Level)
	}
	if cfg.Authorization.Enabled != false {
		t.Errorf("Expected auth enabled false from env, got %v", cfg.Authorization.Enabled)
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Config)
		wantErr bool
	}{
		{
			name: "valid config",
			setup: func(c *Config) {
				c.setDefaults()
				c.Authorization.JWTSharedSecret = "test-secret"
			},
			wantErr: false,
		},
		{
			name: "invalid http port",
			setup: func(c *Config) {
				c.setDefaults()
				c.Server.HTTPPort = 0
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			setup: func(c *Config) {
				c.setDefaults()
				c.Logging.Level = "invalid"
			},
			wantErr: true,
		},
		{
			name: "auth enabled without credentials",
			setup: func(c *Config) {
				c.setDefaults()
				c.Authorization.Enabled = true
				c.Authorization.JWTPublicKeyFile = ""
				c.Authorization.JWTSharedSecret = ""
			},
			wantErr: true,
		},
		{
			name: "invalid rate limit backend",
			setup: func(c *Config) {
				c.setDefaults()
				c.RateLimit.Backend = "invalid"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			tt.setup(cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRouteValidation(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()
	cfg.Authorization.JWTSharedSecret = "test-secret"

	// Add invalid route (missing path pattern)
	cfg.Routes = []RouteConfig{
		{
			PathPattern: "",
			Methods:     []string{"GET"},
			BackendURL:  "http://localhost:3000",
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for missing path pattern")
	}

	// Add invalid route (missing methods)
	cfg.Routes = []RouteConfig{
		{
			PathPattern: "/api/test",
			Methods:     []string{},
			BackendURL:  "http://localhost:3000",
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for missing methods")
	}

	// Add valid route
	cfg.Routes = []RouteConfig{
		{
			PathPattern: "/api/test",
			Methods:     []string{"GET", "POST"},
			BackendURL:  "http://localhost:3000",
			AuthPolicy:  "public",
			Timeout:     30 * time.Second,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected no validation error for valid route, got: %v", err)
	}
}
