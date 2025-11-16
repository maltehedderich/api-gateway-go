package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/maltehedderich/api-gateway-go/internal/config"
	"github.com/maltehedderich/api-gateway-go/internal/logger"
	"github.com/maltehedderich/api-gateway-go/internal/metrics"
	"github.com/maltehedderich/api-gateway-go/internal/router"
)

// Middleware provides authorization middleware
type Middleware struct {
	config            *config.AuthorizationConfig
	logger            *logger.ComponentLogger
	extractor         *TokenExtractor
	validator         *TokenValidator
	revocationChecker *RevocationChecker
	policyEvaluator   *PolicyEvaluator
	enabled           bool
}

// NewMiddleware creates a new authorization middleware
func NewMiddleware(cfg *config.AuthorizationConfig) (*Middleware, error) {
	if !cfg.Enabled {
		return &Middleware{
			config:  cfg,
			logger:  logger.Get().WithComponent("auth.middleware"),
			enabled: false,
		}, nil
	}

	// Create components
	extractor := NewTokenExtractor(cfg)

	validator, err := NewTokenValidator(cfg)
	if err != nil {
		return nil, err
	}

	revocationChecker := NewRevocationChecker(cfg)
	policyEvaluator := NewPolicyEvaluator(cfg.CacheAuthDecisions, cfg.CacheDecisionTTL)

	return &Middleware{
		config:            cfg,
		logger:            logger.Get().WithComponent("auth.middleware"),
		extractor:         extractor,
		validator:         validator,
		revocationChecker: revocationChecker,
		policyEvaluator:   policyEvaluator,
		enabled:           true,
	}, nil
}

// Handler returns the middleware handler
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If authorization is disabled, skip
		if !m.enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Get route match from context to determine policy
		routeMatch := getRouteFromContext(r)
		if routeMatch == nil {
			// No route match - this should not happen, but allow for health checks
			if isHealthCheckPath(r.URL.Path, m.config) {
				next.ServeHTTP(w, r)
				return
			}

			m.logger.Warn("no route match found for authorization", logger.Fields{
				"path":   r.URL.Path,
				"method": r.Method,
			})
			m.writeError(w, r, http.StatusInternalServerError, "internal_error", "Internal server error", nil)
			return
		}

		// Build policy from route configuration
		policy := m.buildPolicy(routeMatch)

		// For public routes, skip token validation
		if policy.Type == PolicyPublic {
			m.logger.Debug("public route, skipping authorization", logger.Fields{
				"path": r.URL.Path,
			})
			metrics.RecordAuthAttempt("bypass")
			next.ServeHTTP(w, r)
			return
		}

		// Extract token
		tokenString, err := m.extractor.ExtractToken(r)
		if err != nil {
			metrics.RecordAuthAttempt("failure")
			metrics.RecordAuthFailure("missing_token")
			m.handleAuthError(w, r, err, "token extraction failed")
			return
		}

		// Validate token
		validationStart := time.Now()
		claims, err := m.validator.ValidateToken(tokenString)
		metrics.RecordAuthValidationDuration(time.Since(validationStart))

		if err != nil {
			metrics.RecordAuthAttempt("failure")
			// Determine error type from validation error
			if valErr, ok := err.(*ValidationError); ok {
				switch valErr.Code {
				case "token_expired":
					metrics.RecordAuthFailure("expired_token")
				case "invalid_token":
					metrics.RecordAuthFailure("invalid_token")
				default:
					metrics.RecordAuthFailure("invalid_token")
				}
			} else {
				metrics.RecordAuthFailure("invalid_token")
			}
			m.handleAuthError(w, r, err, "token validation failed")
			return
		}

		// Check revocation
		revoked, err := m.revocationChecker.IsRevoked(r.Context(), claims.SessionID)
		if err != nil {
			m.logger.Warn("revocation check failed, allowing request", logger.Fields{
				"session_id": maskSessionID(claims.SessionID),
				"error":      err.Error(),
			})
			// Continue despite revocation check failure (fail-open)
		} else if revoked {
			m.logger.Info("token revoked", logger.Fields{
				"user_id":    claims.UserID,
				"session_id": maskSessionID(claims.SessionID),
			})
			metrics.RecordAuthAttempt("failure")
			metrics.RecordAuthFailure("revoked_token")
			m.writeError(w, r, http.StatusUnauthorized, "token_revoked", "Session token has been revoked", nil)
			return
		}

		// Create user context
		userCtx := NewUserContext(claims)

		// Evaluate policy
		decision, err := m.policyEvaluator.Evaluate(policy, userCtx)
		if err != nil {
			m.logger.Error("policy evaluation failed", logger.Fields{
				"error": err.Error(),
			})
			m.writeError(w, r, http.StatusInternalServerError, "internal_error", "Internal server error", nil)
			return
		}

		// Check authorization decision
		if !decision.Allowed {
			m.logger.Info("authorization denied", logger.Fields{
				"user_id":     claims.UserID,
				"path":        r.URL.Path,
				"reason":      decision.Reason,
				"policy_type": policy.Type,
			})
			metrics.RecordAuthAttempt("failure")
			metrics.RecordAuthFailure("insufficient_permissions")
			m.writeError(w, r, http.StatusForbidden, "forbidden", decision.Reason, decision.Details)
			return
		}

		// Store user context in request context
		ctx := SetUserContext(r.Context(), userCtx)

		// Log successful authorization
		m.logger.Info("authorization successful", logger.Fields{
			"user_id":     claims.UserID,
			"session_id":  maskSessionID(claims.SessionID),
			"path":        r.URL.Path,
			"roles":       claims.Roles,
			"policy_type": policy.Type,
		})

		// Record successful authorization
		metrics.RecordAuthAttempt("success")

		// Call next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// buildPolicy builds an authorization policy from route configuration
func (m *Middleware) buildPolicy(route *router.Route) *Policy {
	// Default to authenticated if no policy specified
	policyType := PolicyAuthenticated
	if route.AuthPolicy != "" {
		policyType = PolicyType(route.AuthPolicy)
	}

	policy := &Policy{
		Type: policyType,
	}

	// Add required roles if role-based
	if policyType == PolicyRoleBased {
		policy.Roles = route.RequiredRoles
		policy.Logic = "OR" // Default to OR logic
	}

	return policy
}

// handleAuthError handles authentication errors
func (m *Middleware) handleAuthError(w http.ResponseWriter, r *http.Request, err error, context string) {
	// Check if it's a validation error
	if valErr, ok := err.(*ValidationError); ok {
		statusCode := http.StatusUnauthorized
		if valErr.Code == "forbidden" {
			statusCode = http.StatusForbidden
		}

		m.logger.Info("authentication failed", logger.Fields{
			"path":    r.URL.Path,
			"code":    valErr.Code,
			"message": valErr.Message,
			"context": context,
		})

		m.writeError(w, r, statusCode, valErr.Code, valErr.Message, valErr.Details)
		return
	}

	// Generic error
	m.logger.Error("authentication error", logger.Fields{
		"path":    r.URL.Path,
		"error":   err.Error(),
		"context": context,
	})

	m.writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication failed", nil)
}

// writeError writes an error response
func (m *Middleware) writeError(w http.ResponseWriter, r *http.Request, statusCode int, code, message string, details map[string]interface{}) {
	// Get correlation ID
	correlationID := logger.GetCorrelationID(r.Context())

	// Build error response
	errResp := ErrorResponse{
		Error:         code,
		Message:       message,
		CorrelationID: correlationID,
		Timestamp:     time.Now(),
		Path:          r.URL.Path,
		Details:       details,
	}

	// Set headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Correlation-ID", correlationID)

	// For 401, add WWW-Authenticate header
	if statusCode == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
		w.Header().Set("Cache-Control", "no-store")
	}

	// Write response
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(errResp); err != nil {
		m.logger.Error("failed to encode error response", logger.Fields{
			"error": err.Error(),
		})
	}
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error         string                 `json:"error"`
	Message       string                 `json:"message"`
	CorrelationID string                 `json:"correlation_id"`
	Timestamp     time.Time              `json:"timestamp"`
	Path          string                 `json:"path"`
	Details       map[string]interface{} `json:"details,omitempty"`
}

// getRouteFromContext retrieves route from context
func getRouteFromContext(r *http.Request) *router.Route {
	// Try to get route match from context
	match := r.Context().Value("route_match")
	if match == nil {
		return nil
	}

	if routeMatch, ok := match.(*router.Match); ok {
		return routeMatch.Route
	}

	return nil
}

// isHealthCheckPath checks if the path is a health check endpoint
func isHealthCheckPath(path string, cfg *config.AuthorizationConfig) bool {
	healthPaths := []string{
		"/_health",
		"/_health/live",
		"/_health/ready",
		"/metrics",
	}

	for _, healthPath := range healthPaths {
		if path == healthPath {
			return true
		}
	}

	return false
}
