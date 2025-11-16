package ratelimit

import (
	"context"
	"sync"
	"time"
)

// MemoryStorage implements in-memory rate limit storage.
// It uses a map with mutex for thread-safe access.
// Entries are automatically cleaned up based on TTL.
// This is suitable for single-instance deployments and testing.
type MemoryStorage struct {
	mu      sync.RWMutex
	buckets map[string]*bucketEntry
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// bucketEntry stores a bucket state with expiration time
type bucketEntry struct {
	state  *BucketState
	expiry time.Time
}

// NewMemoryStorage creates a new in-memory storage backend.
// It starts a background goroutine to clean up expired entries.
func NewMemoryStorage() *MemoryStorage {
	ms := &MemoryStorage{
		buckets: make(map[string]*bucketEntry),
		stopCh:  make(chan struct{}),
	}

	// Start cleanup goroutine
	ms.wg.Add(1)
	go ms.cleanupLoop()

	return ms
}

// Get retrieves the bucket state for the given key.
func (ms *MemoryStorage) Get(ctx context.Context, key string) (*BucketState, bool, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	entry, exists := ms.buckets[key]
	if !exists {
		return nil, false, nil
	}

	// Check if entry has expired
	if time.Now().After(entry.expiry) {
		return nil, false, nil
	}

	// Return a copy of the state
	stateCopy := *entry.state
	return &stateCopy, true, nil
}

// Set stores the bucket state for the given key with a TTL.
func (ms *MemoryStorage) Set(ctx context.Context, key string, state *BucketState, ttl time.Duration) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Create a copy of the state
	stateCopy := *state

	ms.buckets[key] = &bucketEntry{
		state:  &stateCopy,
		expiry: time.Now().Add(ttl),
	}

	return nil
}

// Close stops the cleanup goroutine and releases resources.
func (ms *MemoryStorage) Close() error {
	close(ms.stopCh)
	ms.wg.Wait()
	return nil
}

// Ping checks if the storage is available.
// For in-memory storage, this always returns nil.
func (ms *MemoryStorage) Ping(ctx context.Context) error {
	return nil
}

// cleanupLoop runs periodically to remove expired entries.
func (ms *MemoryStorage) cleanupLoop() {
	defer ms.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ms.cleanup()
		case <-ms.stopCh:
			return
		}
	}
}

// cleanup removes expired entries from the map.
func (ms *MemoryStorage) cleanup() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	now := time.Now()
	for key, entry := range ms.buckets {
		if now.After(entry.expiry) {
			delete(ms.buckets, key)
		}
	}
}
