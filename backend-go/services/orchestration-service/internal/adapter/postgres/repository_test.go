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
	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := testutil.StartPostgres(t, "orchestration")

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

func seedCoordinatorRun(t *testing.T, repo *Repository, pool *pgxpool.Pool, tenantID, id, handle string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO orchestration.coordinator_runs (id, tenant_id, origin_task_id, coordinator_handle)
		VALUES ($1, $2, 'origin-task-1', $3)
	`, id, tenantID, handle)
	if err != nil {
		t.Fatalf("seeding coordinator run: %v", err)
	}
}

func TestRepository_UpdateStatusAndPromote_PromotesReadySiblingsAtomically(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "11111111-1111-1111-1111-111111111111"
	runID := "22222222-2222-2222-2222-222222222222"

	seedCoordinatorRun(t, repo, repo.pool, tenantID, runID, "coord-1")

	root, err := domain.NewOrchestrationTask("", tenantID, runID, "", "", "root", nil, nil)
	if err != nil {
		t.Fatalf("building root: %v", err)
	}
	root, err = repo.Create(ctx, root)
	if err != nil {
		t.Fatalf("creating root task: %v", err)
	}

	dependent, err := domain.NewOrchestrationTask("", tenantID, runID, "", "", "dependent", nil, []string{root.ID})
	if err != nil {
		t.Fatalf("building dependent: %v", err)
	}
	dependent, err = repo.Create(ctx, dependent)
	if err != nil {
		t.Fatalf("creating dependent task: %v", err)
	}

	updated, promoted, err := repo.UpdateStatusAndPromote(ctx, tenantID, root.ID, domain.TaskStatusCompleted)
	if err != nil {
		t.Fatalf("UpdateStatusAndPromote: %v", err)
	}
	if updated.Status != domain.TaskStatusCompleted {
		t.Errorf("expected root completed, got %s", updated.Status)
	}
	if len(promoted) != 1 || promoted[0] != dependent.ID {
		t.Fatalf("expected dependent promoted, got %v", promoted)
	}

	got, err := repo.Get(ctx, tenantID, dependent.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != domain.TaskStatusReady {
		t.Errorf("expected dependent status ready, got %s", got.Status)
	}
}

func TestRepository_ResolveGate_CannotBeResolvedTwice(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "33333333-3333-3333-3333-333333333333"
	runID := "44444444-4444-4444-4444-444444444444"

	seedCoordinatorRun(t, repo, repo.pool, tenantID, runID, "coord-2")

	task, err := domain.NewOrchestrationTask("", tenantID, runID, "", "", "gated task", nil, nil)
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	task, err = repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}

	dc, err := repo.CreateDispatchContext(ctx, tenantID, "handle-1", runID)
	if err != nil {
		t.Fatalf("creating dispatch context: %v", err)
	}
	// This scaffold's CreateDispatchContext leaves orchestration_task_id
	// NULL (see README "Known gaps") — patch it directly so CreateGate has
	// a task to attach to, exercising the rest of the atomic chain.
	if _, err := repo.pool.Exec(ctx, `UPDATE orchestration.dispatch_contexts SET orchestration_task_id=$1 WHERE id=$2`, task.ID, dc.ID); err != nil {
		t.Fatalf("patching dispatch context: %v", err)
	}

	gate, err := repo.CreateGate(ctx, tenantID, dc.ID, "proceed?", []string{"yes", "no"})
	if err != nil {
		t.Fatalf("CreateGate: %v", err)
	}

	if _, _, err := repo.ResolveGate(ctx, tenantID, gate.ID, "yes"); err != nil {
		t.Fatalf("first ResolveGate: %v", err)
	}

	if _, _, err := repo.ResolveGate(ctx, tenantID, gate.ID, "no"); err == nil {
		t.Fatal("expected an error resolving an already-resolved gate")
	}
}
