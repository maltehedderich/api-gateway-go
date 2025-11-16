package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/maltehedderich/api-gateway-go/internal/config"
)

// SecurityConfig contains security middleware configuration
type SecurityConfig struct {
	// HSTS
	EnableHSTS       bool
	HSTSMaxAge       int
	HSTSIncludeSubdomains bool
	HSTSPreload      bool

	// Content Security Policy
	ContentSecurityPolicy string

	// Frame Options
	FrameOptions string // DENY, SAMEORIGIN

	// Content Type Options
	ContentTypeNosniff bool

	// XSS Protection
	XSSProtection bool
	XSSBlockMode  bool

	// Referrer Policy
	ReferrerPolicy string

	// Permissions Policy
	PermissionsPolicy string
}

// Security returns a middleware that adds security headers to responses
func Security(cfg *SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add HSTS header
			if cfg.EnableHSTS {
				hstsValue := buildHSTSHeader(cfg)
				w.Header().Set("Strict-Transport-Security", hstsValue)
			}

			// Add Content-Security-Policy header
			if cfg.ContentSecurityPolicy != "" {
				w.Header().Set("Content-Security-Policy", cfg.ContentSecurityPolicy)
			}

			// Add X-Frame-Options header
			if cfg.FrameOptions != "" {
				w.Header().Set("X-Frame-Options", cfg.FrameOptions)
			}

			// Add X-Content-Type-Options header
			if cfg.ContentTypeNosniff {
				w.Header().Set("X-Content-Type-Options", "nosniff")
			}

			// Add X-XSS-Protection header
			if cfg.XSSProtection {
				xssValue := "1"
				if cfg.XSSBlockMode {
					xssValue = "1; mode=block"
				}
				w.Header().Set("X-XSS-Protection", xssValue)
			}

			// Add Referrer-Policy header
			if cfg.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", cfg.ReferrerPolicy)
			}

			// Add Permissions-Policy header
			if cfg.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", cfg.PermissionsPolicy)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// buildHSTSHeader builds the HSTS header value
func buildHSTSHeader(cfg *SecurityConfig) string {
	parts := []string{}

	// Add max-age
	parts = append(parts, "max-age="+strconv.Itoa(cfg.HSTSMaxAge))

	// Add includeSubDomains
	if cfg.HSTSIncludeSubdomains {
		parts = append(parts, "includeSubDomains")
	}

	// Add preload
	if cfg.HSTSPreload {
		parts = append(parts, "preload")
	}

	return strings.Join(parts, "; ")
}

// HTTPSRedirect returns a middleware that redirects HTTP requests to HTTPS
func HTTPSRedirect() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if request is already HTTPS
			if r.TLS != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Check X-Forwarded-Proto header (for proxies)
			if r.Header.Get("X-Forwarded-Proto") == "https" {
				next.ServeHTTP(w, r)
				return
			}

			// Skip redirect for health check endpoints
			if isHealthCheckPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Redirect to HTTPS
			httpsURL := "https://" + r.Host + r.RequestURI
			http.Redirect(w, r, httpsURL, http.StatusMovedPermanently)
		})
	}
}

// isHealthCheckPath checks if the path is a health check endpoint
func isHealthCheckPath(path string) bool {
	healthPaths := []string{
		"/_health",
		"/_health/ready",
		"/_health/live",
	}

	for _, healthPath := range healthPaths {
		if path == healthPath {
			return true
		}
	}

	return false
}

// NewSecurityConfigFromConfig creates a SecurityConfig from the main config
func NewSecurityConfigFromConfig(cfg *config.Config) *SecurityConfig {
	return &SecurityConfig{
		EnableHSTS:            cfg.Security.EnableHSTS,
		HSTSMaxAge:            cfg.Security.HSTSMaxAge,
		HSTSIncludeSubdomains: cfg.Security.HSTSIncludeSubdomains,
		HSTSPreload:           cfg.Security.HSTSPreload,
		ContentSecurityPolicy: cfg.Security.ContentSecurityPolicy,
		FrameOptions:          cfg.Security.FrameOptions,
		ContentTypeNosniff:    cfg.Security.ContentTypeNosniff,
		XSSProtection:         cfg.Security.XSSProtection,
		XSSBlockMode:          cfg.Security.XSSBlockMode,
		ReferrerPolicy:        cfg.Security.ReferrerPolicy,
		PermissionsPolicy:     cfg.Security.PermissionsPolicy,
	}
}
