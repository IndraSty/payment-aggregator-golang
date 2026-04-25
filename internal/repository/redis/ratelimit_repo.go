package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const rateLimitKeyPrefix = "ratelimit:"

// RateLimitRepository handles sliding window rate limiting in Redis.
type RateLimitRepository struct {
	client *redis.Client
	window time.Duration
}

// NewRateLimitRepository creates a new Redis-backed rate limit repository.
func NewRateLimitRepository(client *redis.Client, window time.Duration) *RateLimitRepository {
	return &RateLimitRepository{
		client: client,
		window: window,
	}
}

// Allow checks if a request is allowed under the rate limit.
// Uses sliding window algorithm with Redis sorted sets.
// Returns (allowed bool, current count int64, error).
func (r *RateLimitRepository) Allow(ctx context.Context, key string, limit int) (bool, int64, error) {
	redisKey := rateLimitKeyPrefix + key
	now := time.Now()
	windowStart := now.Add(-r.window)

	// Use a pipeline for atomic operations
	pipe := r.client.Pipeline()

	// Remove entries outside the window
	pipe.ZRemRangeByScore(ctx, redisKey,
		"0",
		fmt.Sprintf("%d", windowStart.UnixMicro()),
	)

	// Add current request timestamp
	pipe.ZAdd(ctx, redisKey, redis.Z{
		Score:  float64(now.UnixMicro()),
		Member: fmt.Sprintf("%d", now.UnixNano()),
	})

	// Count requests in window
	countCmd := pipe.ZCard(ctx, redisKey)

	// Set expiry to clean up old keys
	pipe.Expire(ctx, redisKey, r.window*2)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, 0, fmt.Errorf("rateLimitRepository.Allow pipeline: %w", err)
	}

	count := countCmd.Val()
	return count <= int64(limit), count, nil
}

// Reset clears the rate limit counter for a key.
// Used in tests or manual resets.
func (r *RateLimitRepository) Reset(ctx context.Context, key string) error {
	redisKey := rateLimitKeyPrefix + key
	if err := r.client.Del(ctx, redisKey).Err(); err != nil {
		return fmt.Errorf("rateLimitRepository.Reset: %w", err)
	}
	return nil
}
