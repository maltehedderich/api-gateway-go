package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStorage_GetSet(t *testing.T) {
	ms := NewMemoryStorage()
	defer func() { _ = ms.Close() }()

	ctx := context.Background()
	key := "test:key"

	// Get non-existent key
	state, exists, err := ms.Get(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected key to not exist")
	}
	if state != nil {
		t.Error("expected nil state for non-existent key")
	}

	// Set a key
	testState := &BucketState{
		Capacity:   10.0,
		RefillRate: 5.0,
		Tokens:     7.0,
		LastRefill: time.Now(),
	}

	err = ms.Set(ctx, key, testState, 1*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error setting key: %v", err)
	}

	// Get the key back
	retrievedState, exists, err := ms.Get(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error getting key: %v", err)
	}
	if !exists {
		t.Error("expected key to exist")
	}
	if retrievedState == nil {
		t.Fatal("expected non-nil state")
	}

	// Verify state values
	if retrievedState.Capacity != testState.Capacity {
		t.Errorf("expected capacity %f, got %f", testState.Capacity, retrievedState.Capacity)
	}
	if retrievedState.RefillRate != testState.RefillRate {
		t.Errorf("expected refill rate %f, got %f", testState.RefillRate, retrievedState.RefillRate)
	}
	if retrievedState.Tokens != testState.Tokens {
		t.Errorf("expected tokens %f, got %f", testState.Tokens, retrievedState.Tokens)
	}
}

func TestMemoryStorage_TTL(t *testing.T) {
	ms := NewMemoryStorage()
	defer func() { _ = ms.Close() }()

	ctx := context.Background()
	key := "test:expiring"

	testState := &BucketState{
		Capacity:   10.0,
		RefillRate: 5.0,
		Tokens:     10.0,
		LastRefill: time.Now(),
	}

	// Set with very short TTL
	err := ms.Set(ctx, key, testState, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should exist immediately
	_, exists, err := ms.Get(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected key to exist before expiration")
	}

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Should not exist after expiration
	_, exists, err = ms.Get(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected key to be expired")
	}
}

func TestMemoryStorage_Cleanup(t *testing.T) {
	ms := NewMemoryStorage()
	defer func() { _ = ms.Close() }()

	ctx := context.Background()

	// Add several keys with short TTL
	for i := 0; i < 10; i++ {
		key := "test:cleanup:" + string(rune('0'+i))
		state := &BucketState{
			Capacity:   10.0,
			RefillRate: 5.0,
			Tokens:     10.0,
			LastRefill: time.Now(),
		}
		err := ms.Set(ctx, key, state, 50*time.Millisecond)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Verify all keys exist
	ms.mu.RLock()
	count := len(ms.buckets)
	ms.mu.RUnlock()

	if count != 10 {
		t.Errorf("expected 10 keys, got %d", count)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Trigger cleanup
	ms.cleanup()

	// Verify keys are cleaned up
	ms.mu.RLock()
	count = len(ms.buckets)
	ms.mu.RUnlock()

	if count != 0 {
		t.Errorf("expected 0 keys after cleanup, got %d", count)
	}
}

func TestMemoryStorage_Ping(t *testing.T) {
	ms := NewMemoryStorage()
	defer func() { _ = ms.Close() }()

	ctx := context.Background()
	err := ms.Ping(ctx)
	if err != nil {
		t.Errorf("expected ping to succeed, got error: %v", err)
	}
}

func TestMemoryStorage_Close(t *testing.T) {
	ms := NewMemoryStorage()

	// Add a key
	ctx := context.Background()
	state := &BucketState{
		Capacity:   10.0,
		RefillRate: 5.0,
		Tokens:     10.0,
		LastRefill: time.Now(),
	}
	err := ms.Set(ctx, "test:key", state, 1*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Close should stop cleanup goroutine
	err = ms.Close()
	if err != nil {
		t.Errorf("expected close to succeed, got error: %v", err)
	}

	// Wait a bit to ensure goroutine has stopped
	time.Sleep(100 * time.Millisecond)

	// Data should still be accessible after close
	_, exists, err := ms.Get(ctx, "test:key")
	if err != nil {
		t.Errorf("unexpected error after close: %v", err)
	}
	if !exists {
		t.Error("expected key to still exist after close")
	}
}

func TestMemoryStorage_ConcurrentAccess(t *testing.T) {
	ms := NewMemoryStorage()
	defer func() { _ = ms.Close() }()

	ctx := context.Background()
	iterations := 100

	// Concurrent writes
	done := make(chan bool, iterations)
	for i := 0; i < iterations; i++ {
		go func(n int) {
			key := "test:concurrent"
			state := &BucketState{
				Capacity:   10.0,
				RefillRate: 5.0,
				Tokens:     float64(n),
				LastRefill: time.Now(),
			}
			err := ms.Set(ctx, key, state, 1*time.Minute)
			if err != nil {
				t.Errorf("unexpected error in concurrent write: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all writes to complete
	for i := 0; i < iterations; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		go func() {
			key := "test:concurrent"
			_, _, err := ms.Get(ctx, key)
			if err != nil {
				t.Errorf("unexpected error in concurrent read: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all reads to complete
	for i := 0; i < iterations; i++ {
		<-done
	}
}
