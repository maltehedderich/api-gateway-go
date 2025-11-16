package ratelimit

import (
	"math"
	"time"
)

// TokenBucket implements the token bucket rate limiting algorithm.
// Tokens are added to the bucket at a fixed rate (refill rate).
// Each request consumes one or more tokens.
// Requests are allowed if sufficient tokens are available.
type TokenBucket struct {
	// Capacity is the maximum number of tokens the bucket can hold
	Capacity float64
	// RefillRate is the number of tokens added per second
	RefillRate float64
	// Tokens is the current number of tokens in the bucket
	Tokens float64
	// LastRefill is the timestamp of the last refill operation
	LastRefill time.Time
}

// NewTokenBucket creates a new token bucket with the specified capacity and refill rate.
// The bucket starts full (all tokens available).
func NewTokenBucket(capacity int, refillRate float64) *TokenBucket {
	return &TokenBucket{
		Capacity:   float64(capacity),
		RefillRate: refillRate,
		Tokens:     float64(capacity), // Start with a full bucket
		LastRefill: time.Now(),
	}
}

// NewTokenBucketFromState creates a token bucket from saved state.
// This is used when loading state from storage (Redis or in-memory).
func NewTokenBucketFromState(capacity int, refillRate, tokens float64, lastRefill time.Time) *TokenBucket {
	return &TokenBucket{
		Capacity:   float64(capacity),
		RefillRate: refillRate,
		Tokens:     tokens,
		LastRefill: lastRefill,
	}
}

// Allow checks if n tokens can be consumed from the bucket.
// If enough tokens are available, they are consumed and true is returned.
// If not enough tokens are available, false is returned and no tokens are consumed.
// The bucket is refilled based on elapsed time before checking.
func (tb *TokenBucket) Allow(n int) bool {
	tb.refill()

	tokensNeeded := float64(n)
	if tb.Tokens >= tokensNeeded {
		tb.Tokens -= tokensNeeded
		return true
	}

	return false
}

// Remaining returns the number of tokens currently available in the bucket.
// The bucket is refilled based on elapsed time before returning the count.
func (tb *TokenBucket) Remaining() int {
	tb.refill()
	return int(math.Floor(tb.Tokens))
}

// Reset returns the time when the bucket will be full again.
// This is used for the X-RateLimit-Reset header.
func (tb *TokenBucket) Reset() time.Time {
	tb.refill()

	if tb.Tokens >= tb.Capacity {
		// Already full
		return time.Now()
	}

	// Calculate time needed to fill bucket completely
	tokensNeeded := tb.Capacity - tb.Tokens
	secondsToFull := tokensNeeded / tb.RefillRate

	return time.Now().Add(time.Duration(secondsToFull * float64(time.Second)))
}

// refill adds tokens to the bucket based on elapsed time since last refill.
// Tokens are added at the configured refill rate.
// The bucket never exceeds its maximum capacity.
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.LastRefill).Seconds()

	// Calculate tokens to add based on elapsed time
	tokensToAdd := elapsed * tb.RefillRate

	// Add tokens but don't exceed capacity
	tb.Tokens = math.Min(tb.Capacity, tb.Tokens+tokensToAdd)

	// Update last refill time
	tb.LastRefill = now
}

// State returns the current state of the token bucket for serialization.
type BucketState struct {
	Capacity   float64
	RefillRate float64
	Tokens     float64
	LastRefill time.Time
}

// GetState returns the current state of the token bucket.
// This is used when saving state to storage (Redis or in-memory).
func (tb *TokenBucket) GetState() BucketState {
	tb.refill() // Ensure state is up-to-date
	return BucketState{
		Capacity:   tb.Capacity,
		RefillRate: tb.RefillRate,
		Tokens:     tb.Tokens,
		LastRefill: tb.LastRefill,
	}
}
