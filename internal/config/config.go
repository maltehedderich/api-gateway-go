package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete gateway configuration
type Config struct {
	Server        ServerConfig        `yaml:"server" json:"server"`
	Logging       LoggingConfig       `yaml:"logging" json:"logging"`
	Authorization AuthorizationConfig `yaml:"authorization" json:"authorization"`
	RateLimit     RateLimitConfig     `yaml:"rate_limit" json:"rate_limit"`
	Security      SecurityConfig      `yaml:"security" json:"security"`
	Routes        []RouteConfig       `yaml:"routes" json:"routes"`
	Observability ObservabilityConfig `yaml:"observability" json:"observability"`
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	HTTPPort         int           `yaml:"http_port" json:"http_port"`
	HTTPSPort        int           `yaml:"https_port" json:"https_port"`
	TLSEnabled       bool          `yaml:"tls_enabled" json:"tls_enabled"`
	TLSCertFile      string        `yaml:"tls_cert_file" json:"tls_cert_file"`
	TLSKeyFile       string        `yaml:"tls_key_file" json:"tls_key_file"`
	ReadTimeout      time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout     time.Duration `yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout      time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	HandlerTimeout   time.Duration `yaml:"handler_timeout" json:"handler_timeout"`
	MaxHeaderBytes   int           `yaml:"max_header_bytes" json:"max_header_bytes"`
	ShutdownTimeout  time.Duration `yaml:"shutdown_timeout" json:"shutdown_timeout"`
	EnableHTTP2      bool          `yaml:"enable_http2" json:"enable_http2"`
	TrustedProxies   []string      `yaml:"trusted_proxies" json:"trusted_proxies"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level            string            `yaml:"level" json:"level"`
	Format           string            `yaml:"format" json:"format"` // json or text
	Output           string            `yaml:"output" json:"output"` // stdout, stderr, or file path
	SanitizePatterns []string          `yaml:"sanitize_patterns" json:"sanitize_patterns"`
	ComponentLevels  map[string]string `yaml:"component_levels" json:"component_levels"`
	EnableSampling   bool              `yaml:"enable_sampling" json:"enable_sampling"`
	SamplingRate     float64           `yaml:"sampling_rate" json:"sampling_rate"`
}

// AuthorizationConfig contains authorization configuration
type AuthorizationConfig struct {
	Enabled              bool          `yaml:"enabled" json:"enabled"`
	CookieName           string        `yaml:"cookie_name" json:"cookie_name"`
	JWTSigningAlgorithm  string        `yaml:"jwt_signing_algorithm" json:"jwt_signing_algorithm"`
	JWTPublicKeyFile     string        `yaml:"jwt_public_key_file" json:"jwt_public_key_file"`
	JWTSharedSecret      string        `yaml:"jwt_shared_secret" json:"jwt_shared_secret"`
	ClockSkewTolerance   time.Duration `yaml:"clock_skew_tolerance" json:"clock_skew_tolerance"`
	RequiredClaims       []string      `yaml:"required_claims" json:"required_claims"`
	RevocationListURL    string        `yaml:"revocation_list_url" json:"revocation_list_url"`
	RevocationListCache  time.Duration `yaml:"revocation_list_cache" json:"revocation_list_cache"`
	CacheAuthDecisions   bool          `yaml:"cache_auth_decisions" json:"cache_auth_decisions"`
	CacheDecisionTTL     time.Duration `yaml:"cache_decision_ttl" json:"cache_decision_ttl"`
}

// RateLimitConfig contains rate limiting configuration
type RateLimitConfig struct {
	Enabled      bool              `yaml:"enabled" json:"enabled"`
	Backend      string            `yaml:"backend" json:"backend"` // memory or redis
	RedisAddr    string            `yaml:"redis_addr" json:"redis_addr"`
	RedisPassword string           `yaml:"redis_password" json:"redis_password"`
	RedisDB      int               `yaml:"redis_db" json:"redis_db"`
	FailureMode  string            `yaml:"failure_mode" json:"failure_mode"` // fail-open or fail-closed
	GlobalLimits []LimitDefinition `yaml:"global_limits" json:"global_limits"`
}

// LimitDefinition defines a rate limit
type LimitDefinition struct {
	Key      string `yaml:"key" json:"key"` // ip, user, route, or composite
	Limit    int    `yaml:"limit" json:"limit"`
	Window   string `yaml:"window" json:"window"` // e.g., "1m", "1h"
	Burst    int    `yaml:"burst" json:"burst"`
}

// RouteConfig defines a route
type RouteConfig struct {
	PathPattern    string            `yaml:"path_pattern" json:"path_pattern"`
	Methods        []string          `yaml:"methods" json:"methods"`
	BackendURL     string            `yaml:"backend_url" json:"backend_url"`
	Timeout        time.Duration     `yaml:"timeout" json:"timeout"`
	AuthPolicy     string            `yaml:"auth_policy" json:"auth_policy"` // public, authenticated, role-based
	RequiredRoles  []string          `yaml:"required_roles" json:"required_roles"`
	RateLimits     []LimitDefinition `yaml:"rate_limits" json:"rate_limits"`
	StripPrefix    string            `yaml:"strip_prefix" json:"strip_prefix"`
}

// SecurityConfig contains security configuration
type SecurityConfig struct {
	// TLS Configuration
	TLSMinVersion         string   `yaml:"tls_min_version" json:"tls_min_version"` // 1.2 or 1.3
	TLSCipherSuites       []string `yaml:"tls_cipher_suites" json:"tls_cipher_suites"`
	EnableHTTPSRedirect   bool     `yaml:"enable_https_redirect" json:"enable_https_redirect"`

	// HSTS (HTTP Strict Transport Security)
	EnableHSTS            bool `yaml:"enable_hsts" json:"enable_hsts"`
	HSTSMaxAge            int  `yaml:"hsts_max_age" json:"hsts_max_age"`
	HSTSIncludeSubdomains bool `yaml:"hsts_include_subdomains" json:"hsts_include_subdomains"`
	HSTSPreload           bool `yaml:"hsts_preload" json:"hsts_preload"`

	// Security Headers
	ContentSecurityPolicy string `yaml:"content_security_policy" json:"content_security_policy"`
	FrameOptions          string `yaml:"frame_options" json:"frame_options"` // DENY, SAMEORIGIN
	ContentTypeNosniff    bool   `yaml:"content_type_nosniff" json:"content_type_nosniff"`
	XSSProtection         bool   `yaml:"xss_protection" json:"xss_protection"`
	XSSBlockMode          bool   `yaml:"xss_block_mode" json:"xss_block_mode"`
	ReferrerPolicy        string `yaml:"referrer_policy" json:"referrer_policy"`
	PermissionsPolicy     string `yaml:"permissions_policy" json:"permissions_policy"`

	// Cookie Security
	EnforceCookieSecurity bool `yaml:"enforce_cookie_security" json:"enforce_cookie_security"`
	CookieSameSite        string `yaml:"cookie_same_site" json:"cookie_same_site"` // Strict, Lax, None

	// Input Validation
	MaxRequestBodySize   int64    `yaml:"max_request_body_size" json:"max_request_body_size"` // bytes
	MaxURLPathLength     int      `yaml:"max_url_path_length" json:"max_url_path_length"`
	AllowedMethods       []string `yaml:"allowed_methods" json:"allowed_methods"`
	BlockedUserAgents    []string `yaml:"blocked_user_agents" json:"blocked_user_agents"`

	// Error Disclosure
	HideInternalErrors   bool `yaml:"hide_internal_errors" json:"hide_internal_errors"`
	ProductionMode       bool `yaml:"production_mode" json:"production_mode"`
}

// ObservabilityConfig contains observability configuration
type ObservabilityConfig struct {
	MetricsEnabled bool   `yaml:"metrics_enabled" json:"metrics_enabled"`
	MetricsPort    int    `yaml:"metrics_port" json:"metrics_port"`
	MetricsPath    string `yaml:"metrics_path" json:"metrics_path"`
	HealthPath     string `yaml:"health_path" json:"health_path"`
	ReadinessPath  string `yaml:"readiness_path" json:"readiness_path"`
	LivenessPath   string `yaml:"liveness_path" json:"liveness_path"`
	TracingEnabled bool   `yaml:"tracing_enabled" json:"tracing_enabled"`
	TracingEndpoint string `yaml:"tracing_endpoint" json:"tracing_endpoint"`
}

var (
	globalConfig *Config
	configMu     sync.RWMutex
)

// Load loads configuration from file with environment variable overrides
func Load(configPath string) (*Config, error) {
	cfg := &Config{}

	// Set defaults
	cfg.setDefaults()

	// Load from file if provided
	if configPath != "" {
		if err := loadFromFile(configPath, cfg); err != nil {
			return nil, fmt.Errorf("failed to load config from file: %w", err)
		}
	}

	// Apply environment variable overrides
	if err := applyEnvOverrides(cfg); err != nil {
		return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Set as global config
	configMu.Lock()
	globalConfig = cfg
	configMu.Unlock()

	return cfg, nil
}

// Get returns the global configuration
func Get() *Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return globalConfig
}

// Reload reloads configuration from file
func Reload(configPath string) error {
	newConfig, err := Load(configPath)
	if err != nil {
		return err
	}

	configMu.Lock()
	globalConfig = newConfig
	configMu.Unlock()

	return nil
}

// setDefaults sets default values for configuration
func (c *Config) setDefaults() {
	// Server defaults
	c.Server.HTTPPort = 8080
	c.Server.HTTPSPort = 8443
	c.Server.TLSEnabled = false
	c.Server.ReadTimeout = 30 * time.Second
	c.Server.WriteTimeout = 30 * time.Second
	c.Server.IdleTimeout = 120 * time.Second
	c.Server.HandlerTimeout = 30 * time.Second
	c.Server.MaxHeaderBytes = 1 << 20 // 1 MB
	c.Server.ShutdownTimeout = 30 * time.Second
	c.Server.EnableHTTP2 = true

	// Logging defaults
	c.Logging.Level = "info"
	c.Logging.Format = "json"
	c.Logging.Output = "stdout"
	c.Logging.SamplingRate = 1.0

	// Authorization defaults
	c.Authorization.Enabled = true
	c.Authorization.CookieName = "session_token"
	c.Authorization.JWTSigningAlgorithm = "RS256"
	c.Authorization.ClockSkewTolerance = 5 * time.Second
	c.Authorization.CacheAuthDecisions = true
	c.Authorization.CacheDecisionTTL = 5 * time.Minute
	c.Authorization.RevocationListCache = 30 * time.Second

	// Rate limit defaults
	c.RateLimit.Enabled = true
	c.RateLimit.Backend = "memory"
	c.RateLimit.FailureMode = "fail-closed"
	c.RateLimit.RedisDB = 0

	// Observability defaults
	c.Observability.MetricsEnabled = true
	c.Observability.MetricsPort = 9090
	c.Observability.MetricsPath = "/metrics"
	c.Observability.HealthPath = "/_health"
	c.Observability.ReadinessPath = "/_health/ready"
	c.Observability.LivenessPath = "/_health/live"
	c.Observability.TracingEnabled = false

	// Security defaults
	c.Security.TLSMinVersion = "1.2"
	c.Security.EnableHTTPSRedirect = false
	c.Security.EnableHSTS = true
	c.Security.HSTSMaxAge = 31536000 // 1 year
	c.Security.HSTSIncludeSubdomains = true
	c.Security.HSTSPreload = false
	c.Security.ContentSecurityPolicy = "default-src 'self'"
	c.Security.FrameOptions = "DENY"
	c.Security.ContentTypeNosniff = true
	c.Security.XSSProtection = true
	c.Security.XSSBlockMode = true
	c.Security.ReferrerPolicy = "strict-origin-when-cross-origin"
	c.Security.PermissionsPolicy = "geolocation=(), microphone=(), camera=()"
	c.Security.EnforceCookieSecurity = true
	c.Security.CookieSameSite = "Strict"
	c.Security.MaxRequestBodySize = 10 << 20 // 10 MB
	c.Security.MaxURLPathLength = 2048
	c.Security.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}
	c.Security.HideInternalErrors = true
	c.Security.ProductionMode = false
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.HTTPPort <= 0 || c.Server.HTTPPort > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", c.Server.HTTPPort)
	}
	if c.Server.HTTPSPort <= 0 || c.Server.HTTPSPort > 65535 {
		return fmt.Errorf("invalid HTTPS port: %d", c.Server.HTTPSPort)
	}
	if c.Server.TLSEnabled {
		if c.Server.TLSCertFile == "" {
			return fmt.Errorf("TLS enabled but cert file not specified")
		}
		if c.Server.TLSKeyFile == "" {
			return fmt.Errorf("TLS enabled but key file not specified")
		}
		if _, err := os.Stat(c.Server.TLSCertFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS cert file does not exist: %s", c.Server.TLSCertFile)
		}
		if _, err := os.Stat(c.Server.TLSKeyFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS key file does not exist: %s", c.Server.TLSKeyFile)
		}
	}
	if c.Server.ReadTimeout <= 0 {
		return fmt.Errorf("read timeout must be positive")
	}
	if c.Server.WriteTimeout <= 0 {
		return fmt.Errorf("write timeout must be positive")
	}

	// Validate logging config
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true, "fatal": true}
	if !validLevels[strings.ToLower(c.Logging.Level)] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}
	if c.Logging.Format != "json" && c.Logging.Format != "text" {
		return fmt.Errorf("invalid log format: %s (must be 'json' or 'text')", c.Logging.Format)
	}

	// Validate authorization config
	if c.Authorization.Enabled {
		if c.Authorization.CookieName == "" {
			return fmt.Errorf("authorization enabled but cookie name not specified")
		}
		validAlgos := map[string]bool{"RS256": true, "RS384": true, "RS512": true, "HS256": true, "HS384": true, "HS512": true, "ES256": true, "ES384": true, "ES512": true}
		if !validAlgos[c.Authorization.JWTSigningAlgorithm] {
			return fmt.Errorf("invalid JWT signing algorithm: %s", c.Authorization.JWTSigningAlgorithm)
		}
		// Require either public key file or shared secret
		if c.Authorization.JWTPublicKeyFile == "" && c.Authorization.JWTSharedSecret == "" {
			return fmt.Errorf("authorization enabled but neither public key file nor shared secret specified")
		}
	}

	// Validate rate limit config
	if c.RateLimit.Enabled {
		if c.RateLimit.Backend != "memory" && c.RateLimit.Backend != "redis" {
			return fmt.Errorf("invalid rate limit backend: %s (must be 'memory' or 'redis')", c.RateLimit.Backend)
		}
		if c.RateLimit.Backend == "redis" && c.RateLimit.RedisAddr == "" {
			return fmt.Errorf("rate limit backend is redis but redis address not specified")
		}
		if c.RateLimit.FailureMode != "fail-open" && c.RateLimit.FailureMode != "fail-closed" {
			return fmt.Errorf("invalid failure mode: %s (must be 'fail-open' or 'fail-closed')", c.RateLimit.FailureMode)
		}
	}

	// Validate routes
	for i, route := range c.Routes {
		if route.PathPattern == "" {
			return fmt.Errorf("route %d: path pattern is required", i)
		}
		if len(route.Methods) == 0 {
			return fmt.Errorf("route %d: at least one HTTP method is required", i)
		}
		if route.BackendURL == "" {
			return fmt.Errorf("route %d: backend URL is required", i)
		}
		validAuthPolicies := map[string]bool{"public": true, "authenticated": true, "role-based": true, "permission-based": true}
		if route.AuthPolicy != "" && !validAuthPolicies[route.AuthPolicy] {
			return fmt.Errorf("route %d: invalid auth policy: %s", i, route.AuthPolicy)
		}
		if route.AuthPolicy == "role-based" && len(route.RequiredRoles) == 0 {
			return fmt.Errorf("route %d: role-based auth requires at least one role", i)
		}
	}

	return nil
}

// loadFromFile loads configuration from a file (YAML or JSON)
func loadFromFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Determine format by extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, cfg); err != nil {
			return fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config file format: %s (use .yaml, .yml, or .json)", ext)
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides
// Environment variables should be prefixed with GATEWAY_
func applyEnvOverrides(cfg *Config) error {
	prefix := "GATEWAY_"

	// Server overrides
	if val := os.Getenv(prefix + "HTTP_PORT"); val != "" {
		port, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid HTTP_PORT: %w", err)
		}
		cfg.Server.HTTPPort = port
	}
	if val := os.Getenv(prefix + "HTTPS_PORT"); val != "" {
		port, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid HTTPS_PORT: %w", err)
		}
		cfg.Server.HTTPSPort = port
	}
	if val := os.Getenv(prefix + "TLS_ENABLED"); val != "" {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid TLS_ENABLED: %w", err)
		}
		cfg.Server.TLSEnabled = enabled
	}
	if val := os.Getenv(prefix + "TLS_CERT_FILE"); val != "" {
		cfg.Server.TLSCertFile = val
	}
	if val := os.Getenv(prefix + "TLS_KEY_FILE"); val != "" {
		cfg.Server.TLSKeyFile = val
	}

	// Logging overrides
	if val := os.Getenv(prefix + "LOG_LEVEL"); val != "" {
		cfg.Logging.Level = val
	}
	if val := os.Getenv(prefix + "LOG_FORMAT"); val != "" {
		cfg.Logging.Format = val
	}
	if val := os.Getenv(prefix + "LOG_OUTPUT"); val != "" {
		cfg.Logging.Output = val
	}

	// Authorization overrides
	if val := os.Getenv(prefix + "AUTH_ENABLED"); val != "" {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid AUTH_ENABLED: %w", err)
		}
		cfg.Authorization.Enabled = enabled
	}
	if val := os.Getenv(prefix + "AUTH_COOKIE_NAME"); val != "" {
		cfg.Authorization.CookieName = val
	}
	if val := os.Getenv(prefix + "JWT_SIGNING_ALGORITHM"); val != "" {
		cfg.Authorization.JWTSigningAlgorithm = val
	}
	if val := os.Getenv(prefix + "JWT_PUBLIC_KEY_FILE"); val != "" {
		cfg.Authorization.JWTPublicKeyFile = val
	}
	if val := os.Getenv(prefix + "JWT_SHARED_SECRET"); val != "" {
		cfg.Authorization.JWTSharedSecret = val
	}

	// Rate limit overrides
	if val := os.Getenv(prefix + "RATELIMIT_ENABLED"); val != "" {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid RATELIMIT_ENABLED: %w", err)
		}
		cfg.RateLimit.Enabled = enabled
	}
	if val := os.Getenv(prefix + "RATELIMIT_BACKEND"); val != "" {
		cfg.RateLimit.Backend = val
	}
	if val := os.Getenv(prefix + "REDIS_ADDR"); val != "" {
		cfg.RateLimit.RedisAddr = val
	}
	if val := os.Getenv(prefix + "REDIS_PASSWORD"); val != "" {
		cfg.RateLimit.RedisPassword = val
	}

	return nil
}
