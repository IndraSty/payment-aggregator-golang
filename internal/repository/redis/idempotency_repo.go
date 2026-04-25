package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/IndraSty/payment-aggregator-golang/internal/domain"
	"github.com/redis/go-redis/v9"
)

const idempotencyKeyPrefix = "idempotency:"

type idempotencyRepository struct {
	client *redis.Client
	ttl    time.Duration
}

// NewIdempotencyRepository creates a new Redis-backed idempotency repository.
func NewIdempotencyRepository(client *redis.Client, ttl time.Duration) domain.IdempotencyRepository {
	return &idempotencyRepository{
		client: client,
		ttl:    ttl,
	}
}

func (r *idempotencyRepository) Get(ctx context.Context, key string) (*domain.IdempotencyRecord, error) {
	redisKey := idempotencyKeyPrefix + key

	val, err := r.client.Get(ctx, redisKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Key does not exist — not an error, just cache miss
			return nil, nil
		}
		return nil, fmt.Errorf("idempotencyRepository.Get: %w", err)
	}

	var record domain.IdempotencyRecord
	if err := json.Unmarshal([]byte(val), &record); err != nil {
		return nil, fmt.Errorf("idempotencyRepository.Get unmarshal: %w", err)
	}

	return &record, nil
}

func (r *idempotencyRepository) Set(ctx context.Context, key string, record *domain.IdempotencyRecord) error {
	redisKey := idempotencyKeyPrefix + key

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("idempotencyRepository.Set marshal: %w", err)
	}

	if err := r.client.Set(ctx, redisKey, data, r.ttl).Err(); err != nil {
		return fmt.Errorf("idempotencyRepository.Set: %w", err)
	}

	return nil
}
