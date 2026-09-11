//go:build integration

package mysql

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func setupWebhookDeliveryRepository(t *testing.T) *WebhookDeliveryRepository {
	t.Helper()
	return NewWebhookDeliveryRepository(setupMySQLDB(t))
}

func TestWebhookDeliveryRepository_ExistsFalseWhenNothingRecorded(t *testing.T) {
	repo := setupWebhookDeliveryRepository(t)
	ok, err := repo.Exists(context.Background(), domain.ScmProviderGitHub, "delivery-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected Exists to report false for a delivery never recorded")
	}
}

func TestWebhookDeliveryRepository_RecordThenExists(t *testing.T) {
	repo := setupWebhookDeliveryRepository(t)
	ctx := context.Background()

	if err := repo.Record(ctx, domain.ScmProviderGitHub, "delivery-1", "processed"); err != nil {
		t.Fatalf("unexpected error recording: %v", err)
	}
	ok, err := repo.Exists(ctx, domain.ScmProviderGitHub, "delivery-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected Exists to report true right after Record")
	}
}

// TestWebhookDeliveryRepository_Record_DuplicateIsIdempotent proves
// INSERT IGNORE (MySQL's ON CONFLICT DO NOTHING equivalent, see
// Record's doc comment) actually suppresses the duplicate-key error a
// provider retry of the same delivery would otherwise trigger against
// uq_webhook_delivery_log_provider_delivery.
func TestWebhookDeliveryRepository_Record_DuplicateIsIdempotent(t *testing.T) {
	repo := setupWebhookDeliveryRepository(t)
	ctx := context.Background()

	if err := repo.Record(ctx, domain.ScmProviderGitHub, "delivery-1", "processed"); err != nil {
		t.Fatalf("unexpected error on first record: %v", err)
	}
	if err := repo.Record(ctx, domain.ScmProviderGitHub, "delivery-1", "processed"); err != nil {
		t.Fatalf("expected a provider retry of the same delivery to be a no-op, got error: %v", err)
	}
}

// TestWebhookDeliveryRepository_ScopedByProviderNotJustDeliveryID is this
// table's TASK-BE-DB-003-style isolation test — the SAME delivery_id from
// two different providers must be tracked independently (the UNIQUE key is
// (provider, delivery_id), not delivery_id alone), with no RLS backstop on
// MySQL (migrations/mysql/0001_init.up.sql).
func TestWebhookDeliveryRepository_ScopedByProviderNotJustDeliveryID(t *testing.T) {
	repo := setupWebhookDeliveryRepository(t)
	ctx := context.Background()

	if err := repo.Record(ctx, domain.ScmProviderGitHub, "shared-id", "processed"); err != nil {
		t.Fatalf("unexpected error recording github delivery: %v", err)
	}

	ok, err := repo.Exists(ctx, domain.ScmProviderGitLab, "shared-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected gitlab's delivery log to be independent of github's, but Exists reported true")
	}
}
