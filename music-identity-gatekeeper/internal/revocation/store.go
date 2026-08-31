package revocation

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "blacklist:"

var ErrInvalidTTL = errors.New("access token has no remaining lifetime")

type RedisClient interface {
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	Exists(ctx context.Context, keys ...string) *redis.IntCmd
}

type Store struct {
	client RedisClient
}

func NewStore(client RedisClient) *Store {
	return &Store{client: client}
}

func (s *Store) Blacklist(ctx context.Context, jti string, ttl time.Duration) error {
	if ttl <= 0 {
		return ErrInvalidTTL
	}
	return s.client.Set(ctx, keyPrefix+jti, "1", ttl).Err()
}

func (s *Store) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	count, err := s.client.Exists(ctx, keyPrefix+jti).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
