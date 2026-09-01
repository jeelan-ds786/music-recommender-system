package revocation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type fakeRedisClient struct {
	key        string
	expiration time.Duration
	exists     int64
	err        error
}

func (c *fakeRedisClient) Set(
	_ context.Context,
	key string,
	_ any,
	expiration time.Duration,
) *redis.StatusCmd {
	c.key = key
	c.expiration = expiration
	cmd := redis.NewStatusCmd(context.Background())
	cmd.SetErr(c.err)
	return cmd
}

func (c *fakeRedisClient) Exists(context.Context, ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(context.Background())
	cmd.SetVal(c.exists)
	cmd.SetErr(c.err)
	return cmd
}

func TestStoreBlacklistsJTIWithTTL(t *testing.T) {
	client := &fakeRedisClient{}
	store := NewStore(client)
	ttl := 10 * time.Minute

	if err := store.Blacklist(context.Background(), "access-token-id", ttl); err != nil {
		t.Fatalf("Blacklist() error = %v", err)
	}
	if client.key != "blacklist:access-token-id" {
		t.Fatalf("key = %q", client.key)
	}
	if client.expiration != ttl {
		t.Fatalf("expiration = %v, want %v", client.expiration, ttl)
	}
}

func TestStoreRejectsExpiredTTL(t *testing.T) {
	store := NewStore(&fakeRedisClient{})
	if err := store.Blacklist(context.Background(), "jti", 0); !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("Blacklist() error = %v, want ErrInvalidTTL", err)
	}
}

func TestStoreReportsBlacklistStateAndErrors(t *testing.T) {
	store := NewStore(&fakeRedisClient{exists: 1})
	blacklisted, err := store.IsBlacklisted(context.Background(), "jti")
	if err != nil || !blacklisted {
		t.Fatalf("IsBlacklisted() = %t, %v", blacklisted, err)
	}

	wantErr := errors.New("redis unavailable")
	store = NewStore(&fakeRedisClient{err: wantErr})
	if _, err := store.IsBlacklisted(context.Background(), "jti"); !errors.Is(err, wantErr) {
		t.Fatalf("IsBlacklisted() error = %v, want %v", err, wantErr)
	}
}
