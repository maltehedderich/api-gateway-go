package ratelimit

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/maltehedderich/api-gateway-go/internal/auth"
)

// KeyGenerator generates rate limit keys from HTTP requests.
// Keys are used to identify unique rate limit counters.
type KeyGenerator struct {
	keyTemplate string
}

// NewKeyGenerator creates a new key generator with the specified template.
// Supported templates:
//   - "ip" - rate limit by client IP address
//   - "user" - rate limit by authenticated user ID
//   - "route" - rate limit by request path
//   - "user:route" - composite key by user and route
//   - "ip:route" - composite key by IP and route
func NewKeyGenerator(keyTemplate string) *KeyGenerator {
	return &KeyGenerator{
		keyTemplate: keyTemplate,
	}
}

// GenerateKey generates a rate limit key from the HTTP request.
// Returns the key string and a boolean indicating if the key could be generated.
// If the key cannot be generated (e.g., user template but no auth), returns false.
func (kg *KeyGenerator) GenerateKey(r *http.Request) (string, bool) {
	parts := strings.Split(kg.keyTemplate, ":")
	keyParts := make([]string, 0, len(parts))

	for _, part := range parts {
		switch strings.TrimSpace(part) {
		case "ip":
			ip := kg.getClientIP(r)
			if ip == "" {
				return "", false
			}
			keyParts = append(keyParts, fmt.Sprintf("ip:%s", ip))

		case "user":
			userID := kg.getUserID(r)
			if userID == "" {
				// No authenticated user - cannot generate user-based key
				return "", false
			}
			keyParts = append(keyParts, fmt.Sprintf("user:%s", userID))

		case "route":
			route := kg.getRoute(r)
			keyParts = append(keyParts, fmt.Sprintf("route:%s", route))

		default:
			// Unknown template part - skip
			continue
		}
	}

	if len(keyParts) == 0 {
		return "", false
	}

	// Construct final key with namespace prefix
	key := fmt.Sprintf("ratelimit:%s", strings.Join(keyParts, ":"))
	return key, true
}

// getClientIP extracts the client IP address from the request.
// It checks X-Forwarded-For, X-Real-IP headers before falling back to RemoteAddr.
func (kg *KeyGenerator) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (may contain multiple IPs)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (original client)
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	// RemoteAddr format is "IP:port", we only want the IP
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}

	return addr
}

// getUserID extracts the user ID from the request context.
// Returns empty string if no authenticated user is present.
func (kg *KeyGenerator) getUserID(r *http.Request) string {
	userCtx, ok := auth.GetUserContext(r.Context())
	if !ok {
		return ""
	}
	return userCtx.UserID
}

// getRoute extracts the request path (route) from the request.
func (kg *KeyGenerator) getRoute(r *http.Request) string {
	return r.URL.Path
}
