package auth

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/maltehedderich/api-gateway-go/internal/config"
	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

func init() {
	// Initialize logger for tests
	logger.Init(logger.InfoLevel, "json", &bytes.Buffer{})
}

func TestTokenValidator_ValidateToken(t *testing.T) {
	// Generate test keys
	privateKey, publicKey := generateTestKeys(t)
	publicKeyFile := writePublicKeyToTempFile(t, publicKey)
	defer func() {
		_ = os.Remove(publicKeyFile)
	}()

	// Create validator
	cfg := &config.AuthorizationConfig{
		JWTSigningAlgorithm: "RS256",
		JWTPublicKeyFile:    publicKeyFile,
		ClockSkewTolerance:  5 * time.Second,
		RequiredClaims:      []string{"user_id"},
	}

	validator, err := NewTokenValidator(cfg)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	t.Run("ValidToken", func(t *testing.T) {
		// Create valid token
		claims := &Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			UserID:    "user123",
			SessionID: "session456",
			Roles:     []string{"user", "admin"},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tokenString, err := token.SignedString(privateKey)
		if err != nil {
			t.Fatalf("Failed to sign token: %v", err)
		}

		// Validate token
		validatedClaims, err := validator.ValidateToken(tokenString)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if validatedClaims.UserID != "user123" {
			t.Errorf("Expected UserID user123, got: %s", validatedClaims.UserID)
		}

		if validatedClaims.SessionID != "session456" {
			t.Errorf("Expected SessionID session456, got: %s", validatedClaims.SessionID)
		}
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		// Create expired token
		claims := &Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			},
			UserID:    "user123",
			SessionID: "session456",
		}

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tokenString, err := token.SignedString(privateKey)
		if err != nil {
			t.Fatalf("Failed to sign token: %v", err)
		}

		// Validate token - should fail
		_, err = validator.ValidateToken(tokenString)
		if err == nil {
			t.Error("Expected error for expired token, got nil")
		}

		valErr, ok := err.(*ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got: %T", err)
		} else if valErr.Code != "token_expired" {
			t.Errorf("Expected error code token_expired, got: %s", valErr.Code)
		}
	})

	t.Run("MissingRequiredClaims", func(t *testing.T) {
		// Create token without required claims
		claims := &Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			// Missing UserID
			SessionID: "session456",
		}

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tokenString, err := token.SignedString(privateKey)
		if err != nil {
			t.Fatalf("Failed to sign token: %v", err)
		}

		// Validate token - should fail
		_, err = validator.ValidateToken(tokenString)
		if err == nil {
			t.Error("Expected error for missing required claims, got nil")
		}

		valErr, ok := err.(*ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got: %T", err)
		} else if valErr.Code != "missing_claim" {
			t.Errorf("Expected error code missing_claim, got: %s", valErr.Code)
		}
	})

	t.Run("InvalidSignature", func(t *testing.T) {
		// Create token signed with different key
		wrongKey, _ := rsa.GenerateKey(rand.Reader, 2048)

		claims := &Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			UserID:    "user123",
			SessionID: "session456",
		}

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tokenString, err := token.SignedString(wrongKey)
		if err != nil {
			t.Fatalf("Failed to sign token: %v", err)
		}

		// Validate token - should fail
		_, err = validator.ValidateToken(tokenString)
		if err == nil {
			t.Error("Expected error for invalid signature, got nil")
		}
	})
}

func TestTokenValidator_HMAC(t *testing.T) {
	// Create validator with HMAC
	cfg := &config.AuthorizationConfig{
		JWTSigningAlgorithm: "HS256",
		JWTSharedSecret:     "test-secret-key-for-hmac",
		ClockSkewTolerance:  5 * time.Second,
	}

	validator, err := NewTokenValidator(cfg)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	t.Run("ValidHMACToken", func(t *testing.T) {
		// Create valid token
		claims := &Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			UserID:    "user123",
			SessionID: "session456",
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(cfg.JWTSharedSecret))
		if err != nil {
			t.Fatalf("Failed to sign token: %v", err)
		}

		// Validate token
		validatedClaims, err := validator.ValidateToken(tokenString)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if validatedClaims.UserID != "user123" {
			t.Errorf("Expected UserID user123, got: %s", validatedClaims.UserID)
		}
	})

	t.Run("InvalidHMACSecret", func(t *testing.T) {
		// Create token with wrong secret
		claims := &Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			UserID:    "user123",
			SessionID: "session456",
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte("wrong-secret"))
		if err != nil {
			t.Fatalf("Failed to sign token: %v", err)
		}

		// Validate token - should fail
		_, err = validator.ValidateToken(tokenString)
		if err == nil {
			t.Error("Expected error for invalid HMAC signature, got nil")
		}
	})
}

// Helper functions

func generateTestKeys(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	return privateKey, &privateKey.PublicKey
}

func writePublicKeyToTempFile(t *testing.T, publicKey *rsa.PublicKey) string {
	// Marshal public key to PKIX format
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("Failed to marshal public key: %v", err)
	}

	// Create PEM block
	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	// Write to temp file
	tmpFile := filepath.Join(t.TempDir(), "public_key.pem")
	if err := os.WriteFile(tmpFile, pubKeyPEM, 0600); err != nil {
		t.Fatalf("Failed to write public key file: %v", err)
	}

	return tmpFile
}
