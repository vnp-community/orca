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
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := testutil.StartPostgres(t, "infra")

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

func TestRepository_ResolveConnection_FoundAndNotFound(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelaySSH)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	if _, err := repo.Register(ctx, ds); err != nil {
		t.Fatalf("registering dev server: %v", err)
	}

	connected, got, err := repo.ResolveConnection(ctx, "tenant-1", "ds1")
	if err != nil {
		t.Fatalf("resolve connection: %v", err)
	}
	if !connected || got.ID != "ds1" {
		t.Errorf("expected connected=true, dev server ds1, got connected=%v dev_server=%+v", connected, got)
	}

	connected, _, err = repo.ResolveConnection(ctx, "tenant-1", "unknown")
	if err != nil {
		t.Fatalf("resolve connection: %v", err)
	}
	if connected {
		t.Error("expected connected=false for an unregistered connectionId")
	}

	// Cross-tenant lookup must never succeed, even for a valid id.
	connected, _, err = repo.ResolveConnection(ctx, "tenant-2", "ds1")
	if err != nil {
		t.Fatalf("resolve connection: %v", err)
	}
	if connected {
		t.Error("expected connected=false when the dev server belongs to a different tenant")
	}
}

func TestRepository_List_FiltersByTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	ds1, _ := domain.NewDevServer("ds1", "tenant-1", "10.0.0.1", domain.ConnectionModeRelaySSH)
	ds2, _ := domain.NewDevServer("ds2", "tenant-2", "10.0.0.2", domain.ConnectionModeRelaySSH)
	_, _ = repo.Register(ctx, ds1)
	_, _ = repo.Register(ctx, ds2)

	got, err := repo.List(ctx, "tenant-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].TenantID != "tenant-1" {
		t.Errorf("expected only tenant-1's dev server, got %+v", got)
	}
}
