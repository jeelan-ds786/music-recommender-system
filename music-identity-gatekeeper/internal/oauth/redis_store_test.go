package oauth

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisStateStorePersistsConsumesAndExpiresState(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	store := NewRedisStateStore(client)

	if err := store.Save(ctx, "hash", StateTTL); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if !server.Exists("oauth_state:hash") {
		t.Fatal("hashed state key was not persisted")
	}
	valid, err := store.Consume(ctx, "hash")
	if err != nil || !valid {
		t.Fatalf("Consume() = %t, %v", valid, err)
	}
	valid, err = store.Consume(ctx, "hash")
	if err != nil || valid {
		t.Fatalf("reused Consume() = %t, %v", valid, err)
	}

	if err := store.Save(ctx, "expired", StateTTL); err != nil {
		t.Fatalf("Save() expired error = %v", err)
	}
	server.FastForward(StateTTL)
	valid, err = store.Consume(ctx, "expired")
	if err != nil || valid {
		t.Fatalf("expired Consume() = %t, %v", valid, err)
	}
}
