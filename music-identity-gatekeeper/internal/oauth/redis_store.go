package oauth

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const stateKeyPrefix = "oauth_state:"

type RedisStateStore struct {
	client *redis.Client
}

func NewRedisStateStore(client *redis.Client) *RedisStateStore {
	return &RedisStateStore{client: client}
}

func (s *RedisStateStore) Save(ctx context.Context, stateHash string, ttl time.Duration) error {
	return s.client.Set(ctx, stateKeyPrefix+stateHash, "1", ttl).Err()
}

func (s *RedisStateStore) Consume(ctx context.Context, stateHash string) (bool, error) {
	_, err := s.client.GetDel(ctx, stateKeyPrefix+stateHash).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
