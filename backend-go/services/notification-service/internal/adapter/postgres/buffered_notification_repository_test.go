//go:build integration

// Integration tests run against a real Postgres via testcontainers-go —
// gated behind the "integration" build tag, mirroring repository_test.go's
// existing pattern; run explicitly with
// `go test -tags=integration ./internal/adapter/postgres/...`.
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

func seedSubscription(t *testing.T, repo *Repository, tenantID, userID string) string {
	t.Helper()
	p256dh, auth := "p", "a"
	id := uuid.NewString()
	sub, err := domain.NewPushSubscription(id, tenantID, userID, domain.ChannelWeb,
		"https://push.example/"+id, &p256dh, &auth, "", time.Now())
	if err != nil {
		t.Fatalf("building subscription: %v", err)
	}
	if err := repo.Save(context.Background(), sub); err != nil {
		t.Fatalf("saving subscription: %v", err)
	}
	return id
}

// TestBufferedNotificationStore_Enqueue_CapsAt50PerSubscription is BR-MB-07's
// core contract: the 51st Enqueue for one subscription evicts the oldest
// undelivered row, so the count never exceeds 50.
func TestBufferedNotificationStore_Enqueue_CapsAt50PerSubscription(t *testing.T) {
	repo := setupRepository(t)
	store := NewBufferedNotificationStore(repo.pool)
	ctx := context.Background()

	tenantID, userID := uuid.NewString(), uuid.NewString()
	subID := seedSubscription(t, repo, tenantID, userID)

	for i := 0; i < 51; i++ {
		if err := store.Enqueue(ctx, tenantID, userID, subID, []byte(`{"n":"`+uuid.NewString()+`"}`)); err != nil {
			t.Fatalf("enqueue #%d: %v", i, err)
		}
	}

	pending, err := store.ListPending(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != maxBufferedPerSubscription {
		t.Errorf("expected exactly %d pending rows (cap enforced), got %d", maxBufferedPerSubscription, len(pending))
	}
}

func TestBufferedNotificationStore_MarkDelivered_ExcludesFromListPending(t *testing.T) {
	repo := setupRepository(t)
	store := NewBufferedNotificationStore(repo.pool)
	ctx := context.Background()

	tenantID, userID := uuid.NewString(), uuid.NewString()
	subID := seedSubscription(t, repo, tenantID, userID)

	if err := store.Enqueue(ctx, tenantID, userID, subID, []byte(`{"id":"ne-1","type":"agent_completed"}`)); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	pending, err := store.ListPending(ctx, tenantID, userID)
	if err != nil || len(pending) != 1 {
		t.Fatalf("expected 1 pending row, got %d (err=%v)", len(pending), err)
	}
	if pending[0].Event.ID != "ne-1" {
		t.Errorf("expected decoded event id ne-1, got %q", pending[0].Event.ID)
	}

	if err := store.MarkDelivered(ctx, []string{pending[0].ID}); err != nil {
		t.Fatalf("mark delivered: %v", err)
	}
	pending, err = store.ListPending(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("list pending after delivery: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 pending rows after MarkDelivered, got %d", len(pending))
	}
}
