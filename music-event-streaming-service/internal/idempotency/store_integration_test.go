package idempotency

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeelan-ds786/music-recommender-system/music-event-streaming-service/internal/consumer"
)

func TestStoreClaimLifecycle(t *testing.T) {
	databaseURL := os.Getenv("DB_URL")
	if databaseURL == "" {
		t.Skip("DB_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	store := NewStore(pool)
	eventID := uuid.NewString()
	const handlerName = "integration-handler"

	claim, err := store.Claim(context.Background(), eventID, handlerName)
	if err != nil || claim != consumer.ClaimAcquired {
		t.Fatalf("first claim = %v, %v", claim, err)
	}
	claim, err = store.Claim(context.Background(), eventID, handlerName)
	if err != nil || claim != consumer.ClaimBusy {
		t.Fatalf("concurrent claim = %v, %v", claim, err)
	}
	if err := store.Complete(context.Background(), eventID, handlerName); err != nil {
		t.Fatal(err)
	}
	claim, err = store.Claim(context.Background(), eventID, handlerName)
	if err != nil || claim != consumer.ClaimProcessed {
		t.Fatalf("completed claim = %v, %v", claim, err)
	}

	releasedID := uuid.NewString()
	if _, err := store.Claim(context.Background(), releasedID, handlerName); err != nil {
		t.Fatal(err)
	}
	if err := store.Release(context.Background(), releasedID, handlerName); err != nil {
		t.Fatal(err)
	}
	claim, err = store.Claim(context.Background(), releasedID, handlerName)
	if err != nil || claim != consumer.ClaimAcquired {
		t.Fatalf("claim after release = %v, %v", claim, err)
	}
}
