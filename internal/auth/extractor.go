package auth

import (
	"fmt"
	"net/http"

	"github.com/maltehedderich/api-gateway-go/internal/config"
	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

// TokenExtractor extracts tokens from HTTP requests
type TokenExtractor struct {
	config *config.AuthorizationConfig
	logger *logger.ComponentLogger
}

// NewTokenExtractor creates a new token extractor
func NewTokenExtractor(cfg *config.AuthorizationConfig) *TokenExtractor {
	return &TokenExtractor{
		config: cfg,
		logger: logger.Get().WithComponent("auth.extractor"),
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

	// Warn if cookie is missing security attributes
	te.validateCookieSecurity(cookie)

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

// validateCookieSecurity validates cookie security attributes and logs warnings
func (te *TokenExtractor) validateCookieSecurity(cookie *http.Cookie) {
	warnings := make([]string, 0)

	// Check Secure flag
	if !cookie.Secure {
		warnings = append(warnings, "Secure flag not set")
	}

	// Check HttpOnly flag
	if !cookie.HttpOnly {
		warnings = append(warnings, "HttpOnly flag not set")
	}

	// Check SameSite attribute
	if cookie.SameSite == http.SameSiteNoneMode || cookie.SameSite == http.SameSiteDefaultMode {
		warnings = append(warnings, "SameSite attribute not properly configured")
	}

	// Log warnings if any
	if len(warnings) > 0 {
		te.logger.Warn("session cookie has security issues", logger.Fields{
			"cookie_name": cookie.Name,
			"warnings":    warnings,
		})
	}
}
