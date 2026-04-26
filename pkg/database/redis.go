package database

import (
	"context"
	"fmt"
	"time"

	"github.com/IndraSty/payment-aggregator-golang/config"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// NewRedisClient creates a new Redis client.
// Supports both standard redis:// and TLS rediss:// URLs (Upstash uses rediss://).
func NewRedisClient(cfg *config.RedisConfig) (*redis.Client, error) {
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Connection pool settings
	opts.PoolSize = 10
	opts.MinIdleConns = 2
	opts.ConnMaxLifetime = 30 * time.Minute
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second

	client := redis.NewClient(opts)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	log.Info().Msg("Redis connection established")
	return client, nil
}
