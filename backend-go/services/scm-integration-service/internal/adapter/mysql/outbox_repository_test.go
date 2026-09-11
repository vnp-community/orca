//go:build integration

package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func setupOutboxRepository(t *testing.T) *OutboxRepository {
	t.Helper()
	return NewOutboxRepository(setupMySQLDB(t))
}

func TestOutboxRepository_EnqueueThenFetchUnpublished(t *testing.T) {
	repo := setupOutboxRepository(t)
	ctx := context.Background()

	event := domain.OutboxEvent{
		ID:          "11111111-1111-1111-1111-111111111111",
		Subject:     "orca.scm.pull_request.created",
		OccurredAt:  time.Now().Truncate(time.Second),
		PayloadJSON: []byte(`{"number":1}`),
	}
	if err := repo.Enqueue(ctx, testTenant1, event); err != nil {
		t.Fatalf("unexpected error enqueuing: %v", err)
	}

	records, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error fetching: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 unpublished record, got %d", len(records))
	}
	if records[0].ID != event.ID || records[0].Subject != event.Subject || records[0].Event.TenantID != testTenant1 {
		t.Errorf("unexpected record: %+v", records[0])
	}
}

func TestOutboxRepository_MarkPublished_ExcludesFromFetchUnpublished(t *testing.T) {
	repo := setupOutboxRepository(t)
	ctx := context.Background()

	event := domain.OutboxEvent{
		ID:          "22222222-2222-2222-2222-222222222222",
		Subject:     "orca.scm.pull_request.created",
		OccurredAt:  time.Now().Truncate(time.Second),
		PayloadJSON: []byte(`{}`),
	}
	if err := repo.Enqueue(ctx, testTenant1, event); err != nil {
		t.Fatalf("unexpected error enqueuing: %v", err)
	}
	if err := repo.MarkPublished(ctx, []string{event.ID}); err != nil {
		t.Fatalf("unexpected error marking published: %v", err)
	}

	records, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error fetching: %v", err)
	}
	for _, rec := range records {
		if rec.ID == event.ID {
			t.Fatalf("expected event %s to be excluded once published", event.ID)
		}
	}
}

// TestOutboxRepository_MarkPublished_EmptyIDsIsNoop mirrors
// usage-service/annotation-service's own empty-guard test — MySQL's
// `IN ()` is a syntax error, so the len(ids)==0 early return in
// MarkPublished must actually short-circuit before building the query.
func TestOutboxRepository_MarkPublished_EmptyIDsIsNoop(t *testing.T) {
	repo := setupOutboxRepository(t)
	if err := repo.MarkPublished(context.Background(), nil); err != nil {
		t.Fatalf("expected a no-op, got error: %v", err)
	}
}

// TestOutboxRepository_FetchUnpublished_ScopedAcrossTenants is this
// table's TASK-BE-DB-003 tenant-isolation-without-RLS test: outbox_events
// has an RLS policy on Postgres (migrations/postgres/0003_outbox_events.up.sql)
// but none on MySQL — FetchUnpublished/MarkPublished are relay-internal
// (not tenant-scoped by design, the relay publishes across all tenants),
// so this test instead proves each row still carries its OWN tenant_id
// through correctly rather than one tenant's write bleeding into another's
// row.
func TestOutboxRepository_FetchUnpublished_ScopedAcrossTenants(t *testing.T) {
	repo := setupOutboxRepository(t)
	ctx := context.Background()

	e1 := domain.OutboxEvent{ID: "33333333-3333-3333-3333-333333333333", Subject: "s1", OccurredAt: time.Now(), PayloadJSON: []byte(`{}`)}
	e2 := domain.OutboxEvent{ID: "44444444-4444-4444-4444-444444444444", Subject: "s2", OccurredAt: time.Now(), PayloadJSON: []byte(`{}`)}
	if err := repo.Enqueue(ctx, testTenant1, e1); err != nil {
		t.Fatalf("unexpected error enqueuing tenant-1 event: %v", err)
	}
	if err := repo.Enqueue(ctx, testTenant2, e2); err != nil {
		t.Fatalf("unexpected error enqueuing tenant-2 event: %v", err)
	}

	records, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("unexpected error fetching: %v", err)
	}
	got := map[string]string{}
	for _, rec := range records {
		got[rec.ID] = rec.Event.TenantID
	}
	if got[e1.ID] != testTenant1 {
		t.Errorf("expected event %s to carry tenant_id=%s, got %s", e1.ID, testTenant1, got[e1.ID])
	}
	if got[e2.ID] != testTenant2 {
		t.Errorf("expected event %s to carry tenant_id=%s, got %s", e2.ID, testTenant2, got[e2.ID])
	}
}
