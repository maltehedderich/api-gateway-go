package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/maltehedderich/api-gateway-go/internal/config"
	"github.com/maltehedderich/api-gateway-go/internal/logger"
)

// RevocationChecker checks if tokens have been revoked
type RevocationChecker struct {
	config       *config.AuthorizationConfig
	logger       *logger.ComponentLogger
	client       *http.Client
	cache        *revocationCache
	enabled      bool
}

// NewRevocationChecker creates a new revocation checker
func NewRevocationChecker(cfg *config.AuthorizationConfig) *RevocationChecker {
	enabled := cfg.RevocationListURL != ""

	var cache *revocationCache
	if enabled && cfg.RevocationListCache > 0 {
		cache = newRevocationCache(cfg.RevocationListCache)
	}

	return &RevocationChecker{
		config: cfg,
		logger: logger.Get().WithComponent("auth.revocation"),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		cache:   cache,
		enabled: enabled,
	}
}

// IsRevoked checks if a session ID has been revoked
func (rc *RevocationChecker) IsRevoked(ctx context.Context, sessionID string) (bool, error) {
	if !rc.enabled {
		// Revocation checking is disabled
		return false, nil
	}

	// Check cache first
	if rc.cache != nil {
		if revoked, found := rc.cache.get(sessionID); found {
			rc.logger.Debug("revocation check from cache", logger.Fields{
				"session_id": maskSessionID(sessionID),
				"revoked":    revoked,
			})
			return revoked, nil
		}
	}

	// Check revocation list
	revoked, err := rc.checkRevocationList(ctx, sessionID)
	if err != nil {
		rc.logger.Error("revocation check failed", logger.Fields{
			"session_id": maskSessionID(sessionID),
			"error":      err.Error(),
		})
		// Fail open - assume not revoked if we can't check
		// In production, this could be configurable (fail-open vs fail-closed)
		return false, err
	}

	// Cache result
	if rc.cache != nil {
		rc.cache.set(sessionID, revoked)
	}

	rc.logger.Debug("revocation check completed", logger.Fields{
		"session_id": maskSessionID(sessionID),
		"revoked":    revoked,
	})

	return revoked, nil
}

// checkRevocationList checks the revocation list service
func (rc *RevocationChecker) checkRevocationList(ctx context.Context, sessionID string) (bool, error) {
	// Build request URL
	url := fmt.Sprintf("%s?session_id=%s", rc.config.RevocationListURL, sessionID)

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := rc.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("revocation list service returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		Revoked bool `json:"revoked"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Revoked, nil
}

// revocationCache caches revocation check results
type revocationCache struct {
	cache map[string]*revocationEntry
	ttl   time.Duration
	mu    sync.RWMutex
}

type revocationEntry struct {
	revoked   bool
	expiresAt time.Time
}

// newRevocationCache creates a new revocation cache
func newRevocationCache(ttl time.Duration) *revocationCache {
	rc := &revocationCache{
		cache: make(map[string]*revocationEntry),
		ttl:   ttl,
	}

	// Start cleanup goroutine
	go rc.cleanup()

	return rc
}

// get retrieves a revocation status from cache
func (rc *revocationCache) get(sessionID string) (bool, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	entry, found := rc.cache[sessionID]
	if !found {
		return false, false
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		return false, false
	}

	return entry.revoked, true
}

// set stores a revocation status in cache
func (rc *revocationCache) set(sessionID string, revoked bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.cache[sessionID] = &revocationEntry{
		revoked:   revoked,
		expiresAt: time.Now().Add(rc.ttl),
	}
}

// cleanup periodically removes expired entries
func (rc *revocationCache) cleanup() {
	ticker := time.NewTicker(rc.ttl)
	defer ticker.Stop()

	for range ticker.C {
		rc.mu.Lock()
		now := time.Now()
		for key, entry := range rc.cache {
			if now.After(entry.expiresAt) {
				delete(rc.cache, key)
			}
		}
		rc.mu.Unlock()
	}
}
