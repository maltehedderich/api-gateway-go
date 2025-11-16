package ratelimit

import (
	"context"
	"time"
)

// Storage is the interface for rate limit state storage backends.
// It abstracts the storage mechanism for rate limit counters,
// allowing different implementations (in-memory, Redis, etc.).
type Storage interface {
	// Get retrieves the token bucket state for the given key.
	// Returns the bucket state and true if found, or nil and false if not found.
	Get(ctx context.Context, key string) (*BucketState, bool, error)

	// Set stores the token bucket state for the given key with a TTL.
	// The TTL is used to automatically clean up old entries.
	Set(ctx context.Context, key string, state *BucketState, ttl time.Duration) error

	// Close cleans up any resources used by the storage backend.
	Close() error

	// Ping checks if the storage backend is available.
	// Returns nil if available, error otherwise.
	Ping(ctx context.Context) error
}

// Limit represents a rate limit configuration.
type Limit struct {
	// Key is the rate limit key (used for storage)
	Key string
	// Capacity is the maximum number of tokens (burst capacity)
	Capacity int
	// RefillRate is the number of tokens added per second
	RefillRate float64
	// Window is the time window for the limit (used for TTL calculation)
	Window time.Duration
}

// Result represents the result of a rate limit check.
type Result struct {
	// Allowed indicates if the request is allowed
	Allowed bool
	// Limit is the maximum number of requests allowed
	Limit int
	// Remaining is the number of requests remaining in the current window
	Remaining int
	// Reset is the time when the limit resets
	Reset time.Time
	// RetryAfter is the duration until the next request can be made (if not allowed)
	RetryAfter time.Duration
}
