package ratelimit

import (
	"math"
	"testing"
	"time"
)

func TestNewTokenBucket(t *testing.T) {
	capacity := 10
	refillRate := 2.0

	tb := NewTokenBucket(capacity, refillRate)

	if tb.Capacity != float64(capacity) {
		t.Errorf("expected capacity %f, got %f", float64(capacity), tb.Capacity)
	}

	if tb.RefillRate != refillRate {
		t.Errorf("expected refill rate %f, got %f", refillRate, tb.RefillRate)
	}

	// New bucket should start full
	if tb.Tokens != float64(capacity) {
		t.Errorf("expected tokens %f, got %f", float64(capacity), tb.Tokens)
	}
}

func TestTokenBucket_Allow(t *testing.T) {
	tests := []struct {
		name           string
		capacity       int
		refillRate     float64
		requests       []int
		sleepBetween   time.Duration
		expectedResult []bool
	}{
		{
			name:           "allow requests within capacity",
			capacity:       5,
			refillRate:     10.0,
			requests:       []int{1, 1, 1, 1, 1},
			sleepBetween:   0,
			expectedResult: []bool{true, true, true, true, true},
		},
		{
			name:           "reject requests exceeding capacity",
			capacity:       3,
			refillRate:     1.0,
			requests:       []int{1, 1, 1, 1},
			sleepBetween:   0,
			expectedResult: []bool{true, true, true, false},
		},
		{
			name:           "allow after refill",
			capacity:       2,
			refillRate:     2.0, // 2 tokens per second
			requests:       []int{1, 1, 1},
			sleepBetween:   600 * time.Millisecond, // 0.6 seconds should add ~1.2 tokens
			expectedResult: []bool{true, true, true},
		},
		{
			name:           "consume multiple tokens",
			capacity:       10,
			refillRate:     5.0,
			requests:       []int{5, 3, 3},
			sleepBetween:   0,
			expectedResult: []bool{true, true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := NewTokenBucket(tt.capacity, tt.refillRate)

			for i, n := range tt.requests {
				if i > 0 && tt.sleepBetween > 0 {
					time.Sleep(tt.sleepBetween)
				}

				result := tb.Allow(n)
				if result != tt.expectedResult[i] {
					t.Errorf("request %d: expected %v, got %v (tokens: %f)",
						i, tt.expectedResult[i], result, tb.Tokens)
				}
			}
		})
	}
}

func TestTokenBucket_Remaining(t *testing.T) {
	tb := NewTokenBucket(10, 5.0)

	// Initially should have full capacity
	remaining := tb.Remaining()
	if remaining != 10 {
		t.Errorf("expected 10 remaining tokens, got %d", remaining)
	}

	// Consume 3 tokens
	tb.Allow(3)
	remaining = tb.Remaining()
	if remaining != 7 {
		t.Errorf("expected 7 remaining tokens, got %d", remaining)
	}

	// Consume all remaining
	tb.Allow(7)
	remaining = tb.Remaining()
	if remaining != 0 {
		t.Errorf("expected 0 remaining tokens, got %d", remaining)
	}
}

func TestTokenBucket_Reset(t *testing.T) {
	capacity := 10
	refillRate := 2.0 // 2 tokens per second

	tb := NewTokenBucket(capacity, refillRate)

	// Full bucket should reset immediately
	reset := tb.Reset()
	if time.Until(reset) > 100*time.Millisecond {
		t.Errorf("full bucket should reset immediately, got %v", time.Until(reset))
	}

	// Empty bucket
	tb.Allow(10)

	reset = tb.Reset()
	// Should take 5 seconds to refill (10 tokens / 2 tokens per second)
	expectedDuration := 5 * time.Second
	actualDuration := time.Until(reset)

	// Allow some tolerance (100ms)
	tolerance := 100 * time.Millisecond
	if actualDuration < expectedDuration-tolerance || actualDuration > expectedDuration+tolerance {
		t.Errorf("expected reset duration ~%v, got %v", expectedDuration, actualDuration)
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	capacity := 10
	refillRate := 10.0 // 10 tokens per second

	tb := NewTokenBucket(capacity, refillRate)

	// Consume all tokens
	tb.Allow(10)
	if tb.Remaining() != 0 {
		t.Errorf("expected 0 tokens after consuming all, got %d", tb.Remaining())
	}

	// Wait for 0.5 seconds (should add ~5 tokens)
	time.Sleep(500 * time.Millisecond)

	remaining := tb.Remaining()
	// Should have approximately 5 tokens (allow for timing variation)
	if remaining < 4 || remaining > 6 {
		t.Errorf("expected ~5 tokens after 0.5s refill, got %d", remaining)
	}
}

func TestTokenBucket_CapacityLimit(t *testing.T) {
	capacity := 5
	refillRate := 10.0

	tb := NewTokenBucket(capacity, refillRate)

	// Bucket starts full
	if tb.Remaining() != capacity {
		t.Errorf("expected %d tokens initially, got %d", capacity, tb.Remaining())
	}

	// Wait for refill (should not exceed capacity)
	time.Sleep(1 * time.Second)

	remaining := tb.Remaining()
	if remaining != capacity {
		t.Errorf("bucket should not exceed capacity %d, got %d", capacity, remaining)
	}

	// Tokens should equal capacity
	if tb.Tokens > tb.Capacity {
		t.Errorf("tokens (%f) should not exceed capacity (%f)", tb.Tokens, tb.Capacity)
	}
}

func TestTokenBucket_GetState(t *testing.T) {
	capacity := 10
	refillRate := 5.0

	tb := NewTokenBucket(capacity, refillRate)
	tb.Allow(3) // Consume 3 tokens

	state := tb.GetState()

	if state.Capacity != float64(capacity) {
		t.Errorf("expected capacity %f, got %f", float64(capacity), state.Capacity)
	}

	if state.RefillRate != refillRate {
		t.Errorf("expected refill rate %f, got %f", refillRate, state.RefillRate)
	}

	expectedTokens := 7.0
	epsilon := 0.001 // Allow for floating point precision errors
	if math.Abs(state.Tokens-expectedTokens) > epsilon {
		t.Errorf("expected tokens ~%f, got %f", expectedTokens, state.Tokens)
	}

	if state.LastRefill.IsZero() {
		t.Error("expected non-zero last refill time")
	}
}

func TestNewTokenBucketFromState(t *testing.T) {
	capacity := 10
	refillRate := 5.0
	tokens := 7.0
	lastRefill := time.Now().Add(-1 * time.Second)

	tb := NewTokenBucketFromState(capacity, refillRate, tokens, lastRefill)

	if tb.Capacity != float64(capacity) {
		t.Errorf("expected capacity %f, got %f", float64(capacity), tb.Capacity)
	}

	if tb.RefillRate != refillRate {
		t.Errorf("expected refill rate %f, got %f", refillRate, tb.RefillRate)
	}

	// After creation, refill will happen automatically
	// Should have ~7 tokens + (1 second * 5 tokens/sec) = ~12 tokens, capped at 10
	remaining := tb.Remaining()
	if remaining != capacity {
		t.Errorf("expected capacity %d after refill, got %d", capacity, remaining)
	}
}
