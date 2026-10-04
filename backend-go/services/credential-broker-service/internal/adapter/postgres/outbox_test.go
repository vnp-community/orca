//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/usecase"
)

func TestOutbox_EnqueueCommitsWithTxAndRelayStoreRoundTrips(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	ev := domain.OutboxEvent{
		ID: uuid.NewString(), TenantID: uuid.NewString(), Subject: usecase.SubjectCredentialRotated,
		OccurredAt: time.Now().UTC(), Payload: []byte(`{"credential_id":"c1"}`),
	}
	if err := repo.RunInTx(ctx, func(ctx context.Context, _ usecase.CredentialMetadataRepository, a usecase.AuditRepository) error {
		return a.(usecase.EventEnqueuer).EnqueueEvent(ctx, ev)
	}); err != nil {
		t.Fatal(err)
	}
	recs, err := repo.FetchUnpublished(ctx, 10)
	if err != nil || len(recs) != 1 || recs[0].ID != ev.ID || recs[0].Subject != ev.Subject {
		t.Fatalf("fetch = %+v, %v", recs, err)
	}
	if err := repo.MarkPublished(ctx, []string{ev.ID}); err != nil {
		t.Fatal(err)
	}
	if recs, _ = repo.FetchUnpublished(ctx, 10); len(recs) != 0 {
		t.Fatalf("still unpublished: %+v", recs)
	}
}

func TestOutbox_RolledBackTxLeavesNoEvent(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	boom := errors.New("boom")
	err := repo.RunInTx(ctx, func(ctx context.Context, _ usecase.CredentialMetadataRepository, a usecase.AuditRepository) error {
		if err := a.(usecase.EventEnqueuer).EnqueueEvent(ctx, domain.OutboxEvent{
			ID: uuid.NewString(), TenantID: uuid.NewString(), Subject: "s", OccurredAt: time.Now(), Payload: []byte(`{}`),
		}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if recs, _ := repo.FetchUnpublished(ctx, 10); len(recs) != 0 {
		t.Fatalf("event survived rollback: %+v", recs)
	}
}
