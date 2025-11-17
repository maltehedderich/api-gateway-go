package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/maltehedderich/api-gateway-go/internal/config"
)

// Limiter is the main rate limiting component that coordinates
// token bucket algorithm, storage backend, and key generation.
type Limiter struct {
	storage     Storage
	failureMode string // "fail-open" or "fail-closed"
}

// NewLimiter creates a new rate limiter with the specified configuration.
func NewLimiter(cfg *config.RateLimitConfig) (*Limiter, error) {
	var storage Storage
	var err error

	// Create storage backend
	switch cfg.Backend {
	case "memory":
		storage = NewMemoryStorage()
	case "redis":
		storage, err = NewRedisStorage(RedisConfig{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Redis storage: %w", err)
		}
	case "dynamodb":
		// DynamoDB backend for serverless deployments
		storage, err = NewDynamoDBStorage(cfg.DynamoDBTable, cfg.DynamoDBRegion)
		if err != nil {
			return nil, fmt.Errorf("failed to create DynamoDB storage: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported storage backend: %s", cfg.Backend)
	}

	return &Limiter{
		storage:     storage,
		failureMode: cfg.FailureMode,
	}, nil
}

// Allow checks if a request is allowed based on the rate limit.
// It returns a Result indicating whether the request is allowed and rate limit metadata.
func (l *Limiter) Allow(ctx context.Context, r *http.Request, limitDef *config.LimitDefinition) (*Result, error) {
	// Generate rate limit key
	keyGen := NewKeyGenerator(limitDef.Key)
	key, ok := keyGen.GenerateKey(r)
	if !ok {
		// Could not generate key (e.g., user-based limit but no auth)
		// Allow the request but log a warning
		return &Result{
			Allowed:   true,
			Limit:     limitDef.Limit,
			Remaining: limitDef.Limit,
			Reset:     time.Now(),
		}, nil
	}

	// Parse window duration
	window, err := time.ParseDuration(limitDef.Window)
	if err != nil {
		return nil, fmt.Errorf("invalid window duration: %w", err)
	}

	// Calculate refill rate (tokens per second)
	refillRate := float64(limitDef.Limit) / window.Seconds()

	// Determine burst capacity (use Burst if set, otherwise use Limit)
	capacity := limitDef.Burst
	if capacity == 0 {
		capacity = limitDef.Limit
	}

	// Get or create token bucket
	bucket, err := l.getBucket(ctx, key, capacity, refillRate, window)
	if err != nil {
		// Storage error - apply failure mode
		if l.failureMode == "fail-open" {
			// Allow request on storage failure
			return &Result{
				Allowed:   true,
				Limit:     limitDef.Limit,
				Remaining: limitDef.Limit,
				Reset:     time.Now(),
			}, nil
		}
		// fail-closed: reject request on storage failure
		return &Result{
			Allowed:    false,
			Limit:      limitDef.Limit,
			Remaining:  0,
			Reset:      time.Now().Add(window),
			RetryAfter: window,
		}, err
	}

	// Check if request is allowed (consumes 1 token)
	allowed := bucket.Allow(1)
	remaining := bucket.Remaining()
	reset := bucket.Reset()

	// Save updated bucket state
	state := bucket.GetState()
	_ = l.storage.Set(ctx, key, &state, window*2)
	// Ignore storage error - the request decision has already been made
	// and we don't want to fail the request due to storage issues

	result := &Result{
		Allowed:   allowed,
		Limit:     limitDef.Limit,
		Remaining: remaining,
		Reset:     reset,
	}

	if !allowed {
		result.RetryAfter = time.Until(reset)
		if result.RetryAfter < 0 {
			result.RetryAfter = 0
		}
	}

	return result, nil
}

// getBucket retrieves or creates a token bucket for the given key.
func (l *Limiter) getBucket(ctx context.Context, key string, capacity int, refillRate float64, window time.Duration) (*TokenBucket, error) {
	// Try to get existing bucket state
	state, exists, err := l.storage.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket state: %w", err)
	}

	if exists {
		// Restore bucket from saved state
		return NewTokenBucketFromState(capacity, refillRate, state.Tokens, state.LastRefill), nil
	}

	// Create new bucket (starts full)
	bucket := NewTokenBucket(capacity, refillRate)

	// Save initial state
	initialState := bucket.GetState()
	if err := l.storage.Set(ctx, key, &initialState, window*2); err != nil {
		return nil, fmt.Errorf("failed to save bucket state: %w", err)
	}

	return bucket, nil
}

// Close closes the limiter and releases resources.
func (l *Limiter) Close() error {
	return l.storage.Close()
}

// Ping checks if the storage backend is available.
func (l *Limiter) Ping(ctx context.Context) error {
	return l.storage.Ping(ctx)
}
