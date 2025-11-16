package auth

import (
	"fmt"
	"net/http"

	"github.com/maltehedderich/api-gateway-go/internal/config"
	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

// TokenExtractor extracts tokens from HTTP requests
type TokenExtractor struct {
	config         *config.AuthorizationConfig
	securityConfig *config.SecurityConfig
	logger         *logger.ComponentLogger
}

// NewTokenExtractor creates a new token extractor
func NewTokenExtractor(cfg *config.AuthorizationConfig) *TokenExtractor {
	// Get global config for security settings
	globalCfg := config.Get()
	securityCfg := &config.SecurityConfig{}
	if globalCfg != nil {
		securityCfg = &globalCfg.Security
	}

	return &TokenExtractor{
		config:         cfg,
		securityConfig: securityCfg,
		logger:         logger.Get().WithComponent("auth.extractor"),
	}
}

// ExtractToken extracts the session token from the request
func (te *TokenExtractor) ExtractToken(r *http.Request) (string, error) {
	// Extract from cookie
	cookie, err := r.Cookie(te.config.CookieName)
	if err != nil {
		if err == http.ErrNoCookie {
			te.logger.Debug("session cookie not found", logger.Fields{
				"cookie_name": te.config.CookieName,
				"path":        r.URL.Path,
			})
			return "", &ValidationError{
				Code:    "missing_token",
				Message: "Session token is required for this resource",
			}
		}
		return "", fmt.Errorf("failed to read cookie: %w", err)
	}

	// Validate cookie value
	if cookie.Value == "" {
		return "", &ValidationError{
			Code:    "missing_token",
			Message: "Session token is empty",
		}
	}

	// Log cookie attributes for security validation (in debug mode)
	te.logger.Debug("session cookie found", logger.Fields{
		"cookie_name": te.config.CookieName,
		"secure":      cookie.Secure,
		"http_only":   cookie.HttpOnly,
		"same_site":   sameSiteToString(cookie.SameSite),
		"path":        r.URL.Path,
	})

	// Validate and potentially enforce cookie security attributes
	if err := te.validateCookieSecurity(cookie); err != nil {
		return "", err
	}

	return cookie.Value, nil
}

// sameSiteToString converts SameSite value to string
func sameSiteToString(sameSite http.SameSite) string {
	switch sameSite {
	case http.SameSiteDefaultMode:
		return "Default"
	case http.SameSiteLaxMode:
		return "Lax"
	case http.SameSiteStrictMode:
		return "Strict"
	case http.SameSiteNoneMode:
		return "None"
	default:
		return "Unknown"
	}
}

// validateCookieSecurity validates cookie security attributes and enforces them if configured
func (te *TokenExtractor) validateCookieSecurity(cookie *http.Cookie) error {
	warnings := make([]string, 0)
	errors := make([]string, 0)

	enforceMode := te.securityConfig != nil && te.securityConfig.EnforceCookieSecurity

	// Check Secure flag
	if !cookie.Secure {
		msg := "Secure flag not set"
		if enforceMode {
			errors = append(errors, msg)
		} else {
			warnings = append(warnings, msg)
		}
	}

	// Check HttpOnly flag
	if !cookie.HttpOnly {
		msg := "HttpOnly flag not set"
		if enforceMode {
			errors = append(errors, msg)
		} else {
			warnings = append(warnings, msg)
		}
	}

	// Check SameSite attribute
	expectedSameSite := parseSameSite(te.securityConfig.CookieSameSite)
	if cookie.SameSite != expectedSameSite {
		msg := fmt.Sprintf("SameSite attribute mismatch (expected: %s, got: %s)",
			te.securityConfig.CookieSameSite, sameSiteToString(cookie.SameSite))
		if enforceMode && expectedSameSite != http.SameSiteDefaultMode {
			errors = append(errors, msg)
		} else {
			warnings = append(warnings, msg)
		}
	}

	// Log warnings if any
	if len(warnings) > 0 {
		te.logger.Warn("session cookie has security issues", logger.Fields{
			"cookie_name": cookie.Name,
			"warnings":    warnings,
		})
	}

	// Return error if enforcement is enabled and there are issues
	if enforceMode && len(errors) > 0 {
		te.logger.Error("session cookie failed security validation", logger.Fields{
			"cookie_name": cookie.Name,
			"errors":      errors,
		})

		return &ValidationError{
			Code:    "invalid_cookie_security",
			Message: "Session cookie does not meet security requirements",
		}
	}

	return nil
}

// parseSameSite converts a string to http.SameSite constant
func parseSameSite(sameSite string) http.SameSite {
	switch sameSite {
	case "Strict":
		return http.SameSiteStrictMode
	case "Lax":
		return http.SameSiteLaxMode
	case "None":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteDefaultMode
	}
}
