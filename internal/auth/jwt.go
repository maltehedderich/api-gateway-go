package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/maltehedderich/api-gateway-go/internal/config"
	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

// TokenValidator validates JWT tokens
type TokenValidator struct {
	config    *config.AuthorizationConfig
	logger    *logger.ComponentLogger
	publicKey *rsa.PublicKey
	hmacKey   []byte
	mu        sync.RWMutex
}

// Claims represents the JWT claims we expect
type Claims struct {
	jwt.RegisteredClaims
	UserID      string   `json:"user_id"`
	SessionID   string   `json:"session_id"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// NewTokenValidator creates a new token validator
func NewTokenValidator(cfg *config.AuthorizationConfig) (*TokenValidator, error) {
	tv := &TokenValidator{
		config: cfg,
		logger: logger.Get().WithComponent("auth.validator"),
	}

	// Load signing key based on algorithm
	if err := tv.loadSigningKey(); err != nil {
		return nil, fmt.Errorf("failed to load signing key: %w", err)
	}

	tv.logger.Info("token validator initialized", logger.Fields{
		"algorithm": cfg.JWTSigningAlgorithm,
	})

	return tv, nil
}

// loadSigningKey loads the signing key based on configuration
func (tv *TokenValidator) loadSigningKey() error {
	algo := tv.config.JWTSigningAlgorithm

	// RS* algorithms require public key
	if algo == "RS256" || algo == "RS384" || algo == "RS512" {
		if tv.config.JWTPublicKeyFile == "" {
			return fmt.Errorf("RS* algorithm requires public key file")
		}
		return tv.loadRSAPublicKey(tv.config.JWTPublicKeyFile)
	}

	// HS* algorithms require shared secret
	if algo == "HS256" || algo == "HS384" || algo == "HS512" {
		if tv.config.JWTSharedSecret == "" {
			return fmt.Errorf("HS* algorithm requires shared secret")
		}
		tv.hmacKey = []byte(tv.config.JWTSharedSecret)
		return nil
	}

	// ES* algorithms not yet implemented
	if algo == "ES256" || algo == "ES384" || algo == "ES512" {
		return fmt.Errorf("ES* algorithms not yet implemented")
	}

	return fmt.Errorf("unsupported algorithm: %s", algo)
}

// loadRSAPublicKey loads an RSA public key from a PEM file
func (tv *TokenValidator) loadRSAPublicKey(path string) error {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read public key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}

	// Try parsing as PKIX public key
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// Try parsing as PKCS1 public key
		pubKey, err = x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse public key: %w", err)
		}
	}

	rsaKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("public key is not RSA")
	}

	tv.mu.Lock()
	tv.publicKey = rsaKey
	tv.mu.Unlock()

	return nil
}

// ValidateToken validates a JWT token and returns the claims
func (tv *TokenValidator) ValidateToken(tokenString string) (*Claims, error) {
	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, tv.keyFunc)
	if err != nil {
		tv.logger.Warn("token validation failed", logger.Fields{
			"error": err.Error(),
		})

		// Check if it's an expiration error
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, &ValidationError{
				Code:    "token_expired",
				Message: "Token has expired",
				Err:     err,
			}
		}

		return nil, &ValidationError{
			Code:    "invalid_token",
			Message: "Token validation failed",
			Err:     err,
		}
	}

	// Check if token is valid
	if !token.Valid {
		return nil, &ValidationError{
			Code:    "invalid_token",
			Message: "Token is not valid",
		}
	}

	// Extract claims
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, &ValidationError{
			Code:    "invalid_claims",
			Message: "Failed to extract claims",
		}
	}

	// Validate expiration with clock skew tolerance
	if err := tv.validateExpiration(claims); err != nil {
		return nil, err
	}

	// Validate required claims
	if err := tv.validateRequiredClaims(claims); err != nil {
		return nil, err
	}

	tv.logger.Debug("token validated successfully", logger.Fields{
		"user_id":    claims.UserID,
		"session_id": maskSessionID(claims.SessionID),
		"roles":      claims.Roles,
	})

	return claims, nil
}

// keyFunc returns the key for validating the token
func (tv *TokenValidator) keyFunc(token *jwt.Token) (interface{}, error) {
	// Verify signing method
	expectedMethod := tv.config.JWTSigningAlgorithm

	if token.Method.Alg() != expectedMethod {
		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	}

	// Return appropriate key based on algorithm
	switch expectedMethod {
	case "RS256", "RS384", "RS512":
		tv.mu.RLock()
		defer tv.mu.RUnlock()
		return tv.publicKey, nil
	case "HS256", "HS384", "HS512":
		return tv.hmacKey, nil
	default:
		return nil, fmt.Errorf("unsupported signing method: %s", expectedMethod)
	}
}

// validateExpiration validates token expiration with clock skew tolerance
func (tv *TokenValidator) validateExpiration(claims *Claims) error {
	now := time.Now()
	tolerance := tv.config.ClockSkewTolerance

	// Check expiration
	if claims.ExpiresAt != nil {
		expiresAt := claims.ExpiresAt.Time
		if now.After(expiresAt.Add(tolerance)) {
			return &ValidationError{
				Code:    "token_expired",
				Message: "Token has expired",
				Details: map[string]interface{}{
					"expired_at": expiresAt.Format(time.RFC3339),
				},
			}
		}
	}

	// Check not before
	if claims.NotBefore != nil {
		notBefore := claims.NotBefore.Time
		if now.Before(notBefore.Add(-tolerance)) {
			return &ValidationError{
				Code:    "token_not_yet_valid",
				Message: "Token is not yet valid",
			}
		}
	}

	return nil
}

// validateRequiredClaims validates that required claims are present
func (tv *TokenValidator) validateRequiredClaims(claims *Claims) error {
	for _, requiredClaim := range tv.config.RequiredClaims {
		switch requiredClaim {
		case "user_id":
			if claims.UserID == "" {
				return &ValidationError{
					Code:    "missing_claim",
					Message: fmt.Sprintf("Required claim missing: %s", requiredClaim),
				}
			}
		case "session_id":
			if claims.SessionID == "" {
				return &ValidationError{
					Code:    "missing_claim",
					Message: fmt.Sprintf("Required claim missing: %s", requiredClaim),
				}
			}
		case "roles":
			if len(claims.Roles) == 0 {
				return &ValidationError{
					Code:    "missing_claim",
					Message: fmt.Sprintf("Required claim missing: %s", requiredClaim),
				}
			}
		case "permissions":
			if len(claims.Permissions) == 0 {
				return &ValidationError{
					Code:    "missing_claim",
					Message: fmt.Sprintf("Required claim missing: %s", requiredClaim),
				}
			}
		}
	}

	return nil
}

// maskSessionID masks a session ID for logging (shows only last 4 characters)
func maskSessionID(sessionID string) string {
	if len(sessionID) <= 4 {
		return "****"
	}
	return "****" + sessionID[len(sessionID)-4:]
}

// ValidationError represents a token validation error
type ValidationError struct {
	Code    string
	Message string
	Details map[string]interface{}
	Err     error
}

func (e *ValidationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}
