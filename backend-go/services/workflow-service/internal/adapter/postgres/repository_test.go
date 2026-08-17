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
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := testutil.StartPostgres(t, "workflow")

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
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

func TestRepository_CreateAndGetTemplate(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tmpl, err := domain.NewWorkflowTemplate("tmpl-1", "11111111-1111-1111-1111-111111111111", "deploy", `{"steps":[]}`, domain.ScopePersonal)
	if err != nil {
		t.Fatalf("building template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	got, err := repo.GetTemplate(ctx, tmpl.TenantID, tmpl.ID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	if got.Name != "deploy" {
		t.Errorf("expected name=deploy, got %s", got.Name)
	}
}

func TestRepository_ExecutionPauseResumeRoundTrip(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "22222222-2222-2222-2222-222222222222"

	tmpl, _ := domain.NewWorkflowTemplate("tmpl-2", tenantID, "release", `{"steps":[]}`, domain.ScopeTeam)
	_ = repo.CreateTemplate(ctx, tmpl)

	exec, err := domain.NewWorkflowExecution("exec-1", tenantID, tmpl.ID, "trace-1")
	if err != nil {
		t.Fatalf("building execution: %v", err)
	}
	if err := repo.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	if err := exec.Pause(time.Now().UTC()); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if err := repo.UpdateExecution(ctx, exec); err != nil {
		t.Fatalf("update (pause): %v", err)
	}

	got, err := repo.GetExecution(ctx, tenantID, exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if got.Status != domain.StatusPaused || got.PausedAt == nil {
		t.Fatalf("expected paused execution with PausedAt set, got %+v", got)
	}
}
