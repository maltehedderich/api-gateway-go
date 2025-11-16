package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStorage implements rate limit storage using Redis.
// It uses Redis strings with JSON-encoded bucket state.
// TTL is used for automatic cleanup of old entries.
// This is suitable for distributed deployments with multiple gateway instances.
type RedisStorage struct {
	client *redis.Client
}

// RedisConfig contains configuration for Redis storage.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// NewRedisStorage creates a new Redis storage backend.
func NewRedisStorage(cfg RedisConfig) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStorage{
		client: client,
	}, nil
}

// Get retrieves the bucket state for the given key from Redis.
func (rs *RedisStorage) Get(ctx context.Context, key string) (*BucketState, bool, error) {
	data, err := rs.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			// Key doesn't exist
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to get key from Redis: %w", err)
	}

	var state BucketState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal bucket state: %w", err)
	}

	return &state, true, nil
}

// Set stores the bucket state for the given key in Redis with a TTL.
func (rs *RedisStorage) Set(ctx context.Context, key string, state *BucketState, ttl time.Duration) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal bucket state: %w", err)
	}

	if err := rs.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set key in Redis: %w", err)
	}

	return nil
}

// Close closes the Redis connection.
func (rs *RedisStorage) Close() error {
	return rs.client.Close()
}

// Ping checks if Redis is available.
func (rs *RedisStorage) Ping(ctx context.Context) error {
	return rs.client.Ping(ctx).Err()
}
