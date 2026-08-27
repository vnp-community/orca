//go:build integration

// Integration tests run against a real Postgres via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/postgres/...`.
package postgres

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := testutil.StartPostgres(t, "issuetracking")

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	// Uses the golang-migrate CLI directly rather than importing the
	// library, keeping this test's dependency footprint minimal — matches
	// usage-service's own integration test's precedent.
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	return New(pool)
}

// TestRepository_Outbox_EnqueueFetchMarkPublished exercises the Epic G
// transactional-outbox round trip: Enqueue writes a row, FetchUnpublished
// sees it, and MarkPublished removes it from future fetches — the exact
// cycle common/outbox.Relay drives in production.
func TestRepository_Outbox_EnqueueFetchMarkPublished(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	event := domain.OutboxEvent{ID: uuid.NewString(), Subject: "orca.issuetracking.link.created", OccurredAt: time.Now(), PayloadJSON: []byte(`{"issue_id":"PROJ-1","task_id":"task-1"}`)}

	if err := repo.Enqueue(ctx, "tenant-1", event); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	unpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("fetch unpublished: %v", err)
	}
	if len(unpublished) != 1 || unpublished[0].ID != event.ID || unpublished[0].Subject != event.Subject {
		t.Fatalf("expected exactly the just-enqueued event, got %+v", unpublished)
	}
	if unpublished[0].Event.TenantID != "tenant-1" {
		t.Errorf("expected tenant_id to round-trip, got %q", unpublished[0].Event.TenantID)
	}

	if err := repo.MarkPublished(ctx, []string{event.ID}); err != nil {
		t.Fatalf("mark published: %v", err)
	}

	stillUnpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("fetch unpublished after mark: %v", err)
	}
	if len(stillUnpublished) != 0 {
		t.Errorf("expected no unpublished events after MarkPublished, got %+v", stillUnpublished)
	}
}
