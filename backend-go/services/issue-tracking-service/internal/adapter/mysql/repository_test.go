//go:build integration

// Integration tests run against a real MySQL via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/mysql/...`. Mirrors
// internal/adapter/postgres/repository_test.go's test names/shape 1:1.
package mysql

import (
	"context"
	"database/sql"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	// testutil.StartMySQL returns "mysql://root:orca@tcp(host:port)/db" —
	// valid as-is for golang-migrate's mysql driver CLI (used below), but
	// go-sql-driver/mysql's database/sql driver expects its OWN DSN format
	// with no "mysql://" scheme. parseTime=true is required for TIMESTAMP
	// columns to scan into time.Time instead of []byte — same finding as
	// usage-service's mysql adapter (TASK-BE-DB-005's "Kết quả thực tế" #1).
	rawDSN := testutil.StartMySQL(t, "issuetracking")
	driverDSN := strings.TrimPrefix(rawDSN, "mysql://") + "?parseTime=true"

	migrationsPath, err := filepath.Abs("../../../migrations/mysql")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", rawDSN, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}

	db, err := sql.Open("mysql", driverDSN)
	if err != nil {
		t.Fatalf("connecting to mysql: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("pinging mysql: %v", err)
	}

	return New(db)
}

// TestRepository_Outbox_EnqueueFetchMarkPublished exercises the Epic G
// transactional-outbox round trip: Enqueue writes a row, FetchUnpublished
// sees it, and MarkPublished removes it from future fetches — mirrors
// internal/adapter/postgres/repository_test.go's test of the same name.
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

func TestRepository_MarkPublished_EmptyIDsIsNoop(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	if err := repo.MarkPublished(ctx, nil); err != nil {
		t.Fatalf("MarkPublished(nil) should be a no-op, got error: %v", err)
	}
	if err := repo.MarkPublished(ctx, []string{}); err != nil {
		t.Fatalf("MarkPublished([]string{}) should be a no-op, got error: %v", err)
	}
}
