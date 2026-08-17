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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/usage-service/internal/domain"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := testutil.StartPostgres(t, "usage")

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	// Uses the golang-migrate CLI directly rather than importing the
	// library, keeping this test's dependency footprint minimal — swap for
	// the library-based runner once the shared migration-runner helper
	// (referenced in architecture/05-data-architecture.md) exists in common/.
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

func TestRepository_SaveSession_IsIdempotentOnRequestID(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	session, err := domain.NewUsageSession(
		"s1", "tenant-1", "user-1", domain.ProviderClaude, "wt-1",
		100, 50, 0, 0, 0.05, time.Now(), time.Now(), "req-1",
	)
	if err != nil {
		t.Fatalf("building session: %v", err)
	}

	if err := repo.SaveSession(ctx, session); err != nil {
		t.Fatalf("first save: %v", err)
	}
	// Same RequestID, different ID — simulates a client retry after a
	// timed-out-but-actually-succeeded write. Must not double-count.
	session.ID = "s1-retry"
	if err := repo.SaveSession(ctx, session); err != nil {
		t.Fatalf("second save (retry): %v", err)
	}

	rollup, err := repo.GetDailyRollup(ctx, "tenant-1", "user-1", domain.ProviderClaude, domain.DayKey(session.StartedAt))
	if err != nil {
		t.Fatalf("get daily rollup: %v", err)
	}
	if rollup.SessionCount != 1 {
		t.Errorf("expected idempotent save to result in SessionCount=1, got %d", rollup.SessionCount)
	}
}

func TestRepository_ListSessions_FiltersByTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	s1, _ := domain.NewUsageSession("s1", "tenant-1", "user-1", domain.ProviderClaude, "wt-1", 10, 5, 0, 0, 0.01, time.Now(), time.Time{}, "req-1")
	s2, _ := domain.NewUsageSession("s2", "tenant-2", "user-1", domain.ProviderClaude, "wt-1", 10, 5, 0, 0, 0.01, time.Now(), time.Time{}, "req-2")
	_ = repo.SaveSession(ctx, s1)
	_ = repo.SaveSession(ctx, s2)

	sessions, _, err := repo.ListSessions(ctx, "tenant-1", "", "", 50)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].TenantID != "tenant-1" {
		t.Errorf("expected only tenant-1's session, got %+v", sessions)
	}
}
