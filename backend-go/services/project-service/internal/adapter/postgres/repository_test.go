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
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := testutil.StartPostgres(t, "project")

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

func TestRepository_CreateAndGet_RoundTrips(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	project, err := domain.NewProject("00000000-0000-0000-0000-000000000001", "11111111-1111-1111-1111-111111111111", "my-project", "")
	if err != nil {
		t.Fatalf("building project: %v", err)
	}

	if _, err := repo.Create(ctx, project); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.Get(ctx, project.TenantID, project.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "my-project" || got.DevServerID != "" {
		t.Errorf("unexpected project: %+v", got)
	}
}

func TestRepository_Get_FiltersByTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	p, _ := domain.NewProject("00000000-0000-0000-0000-000000000002", "11111111-1111-1111-1111-111111111111", "proj", "")
	_, _ = repo.Create(ctx, p)

	if _, err := repo.Get(ctx, "22222222-2222-2222-2222-222222222222", p.ID); err != domain.ErrProjectNotFound {
		t.Errorf("expected ErrProjectNotFound for a mismatched tenant, got %v", err)
	}
}

func TestRepository_UpdateDevServerID(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	p, _ := domain.NewProject("00000000-0000-0000-0000-000000000003", "11111111-1111-1111-1111-111111111111", "proj", "")
	_, _ = repo.Create(ctx, p)

	updated, err := repo.UpdateDevServerID(ctx, p.TenantID, p.ID, "33333333-3333-3333-3333-333333333333")
	if err != nil {
		t.Fatalf("update dev server id: %v", err)
	}
	if updated.DevServerID != "33333333-3333-3333-3333-333333333333" {
		t.Errorf("expected dev_server_id to be updated, got %q", updated.DevServerID)
	}
}
