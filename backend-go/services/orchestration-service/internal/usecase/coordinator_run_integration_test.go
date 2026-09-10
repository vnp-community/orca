//go:build integration

// Integration test proving BUG-TASKV1-005's core finding is closed: the
// coordinator autonomously advances a run end to end, with no manual
// "promote" call from a test/operator.
//
// Declared as package usecase_test (not usecase) deliberately: this file
// needs a real *postgres.Repository, but internal/adapter/postgres imports
// internal/usecase (for its sentinel errors and UpdateStatusAndPromoteResult
// type) — importing postgres from a usecase-package test file would be an
// import cycle. An external test package has no such restriction since it
// compiles as its own package, only woven into the test binary.
package usecase_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/common/testutil"

	orchpostgres "github.com/stablyai/orca-go/services/orchestration-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/usecase"
)

// integrationWorkerDispatcher immediately "completes" whatever task it's
// asked to dispatch by calling back into UpdateTaskStatusAndPromote — the
// synchronous stand-in for a real AI-agent worker's eventual completion
// signal, letting this test observe the full autonomous chain in one call.
type integrationWorkerDispatcher struct {
	updateStatus *usecase.UpdateTaskStatusAndPromote
}

func (w *integrationWorkerDispatcher) Dispatch(ctx context.Context, tenantID string, task domain.OrchestrationTask, _ string) error {
	taskCtx := withIntegrationTenant(ctx, tenantID)
	_, err := w.updateStatus.Execute(taskCtx, usecase.UpdateTaskStatusAndPromoteInput{
		OrchestrationTaskID: task.ID,
		NewStatus:           string(domain.TaskStatusCompleted),
	})
	return err
}

// integrationTaskServiceReporter records every ReportResult call so the
// test can assert the run's terminal state was actually reported back to
// task-service exactly once.
type integrationTaskServiceReporter struct {
	mu    sync.Mutex
	calls []struct {
		taskID, coordinatorRunID string
		success                  bool
	}
}

func (r *integrationTaskServiceReporter) ReportResult(_ context.Context, taskID, coordinatorRunID string, success bool, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, struct {
		taskID, coordinatorRunID string
		success                  bool
	}{taskID, coordinatorRunID, success})
	return nil
}

func (r *integrationTaskServiceReporter) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func withIntegrationTenant(ctx context.Context, tenantID string) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

func setupIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := testutil.StartPostgres(t, "orchestration")

	migrationsPath, err := filepath.Abs("../../migrations")
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
	return pool
}

// TestCoordinatorRun_AutonomouslyAdvancesToCompletion is the concrete
// end-to-end proof that BUG-TASKV1-005's core finding — "the coordinator
// does not autonomously advance a run" — is closed: StartCoordinatorRun
// (single-node spec, no deps) -> TickDispatch.Execute claims and dispatches
// the ready root -> the fake WorkerDispatcher immediately calls back
// UpdateTaskStatusAndPromote with completed -> the run auto-finalizes to
// completed in Postgres, and TaskServiceReporter.ReportResult is called
// exactly once with success=true.
func TestCoordinatorRun_AutonomouslyAdvancesToCompletion(t *testing.T) {
	pool := setupIntegrationPool(t)
	repo := orchpostgres.New(pool)

	serializer := usecase.NewKeyedSerializer(0)
	reporter := &integrationTaskServiceReporter{}

	updateStatusUC := usecase.NewUpdateTaskStatusAndPromote(repo, serializer, reporter, repo)
	startCoordinatorRunUC := usecase.NewStartCoordinatorRun(repo, serializer)
	createDispatchContextUC := usecase.NewCreateDispatchContext(repo, serializer, repo)
	failDispatchUC := usecase.NewFailDispatch(repo)
	worker := &integrationWorkerDispatcher{updateStatus: updateStatusUC}
	tickUC := usecase.NewTickDispatch(repo, repo, createDispatchContextUC, worker, failDispatchUC, reporter)

	tenantID := "e0000000-0000-0000-0000-000000000001"
	ctx := withIntegrationTenant(context.Background(), tenantID)

	specJSON := []byte(`[{"tempId":"a","title":"Root task","spec":{},"deps":[]}]`)
	run, err := startCoordinatorRunUC.Execute(ctx, usecase.StartCoordinatorRunInput{
		OriginTaskID: "origin-task-1",
		SpecJSON:     specJSON,
	})
	if err != nil {
		t.Fatalf("StartCoordinatorRun: %v", err)
	}
	if run.Status != domain.RunStatusRunning {
		t.Fatalf("expected RunStatusRunning immediately after start, got %s", run.Status)
	}

	if err := tickUC.Execute(ctx); err != nil {
		t.Fatalf("TickDispatch.Execute: %v", err)
	}

	got, err := repo.GetRun(ctx, tenantID, run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != domain.RunStatusCompleted {
		t.Fatalf("expected the run to auto-finalize to completed, got %s", got.Status)
	}

	if reporter.callCount() != 1 {
		t.Fatalf("expected ReportResult called exactly once, got %d", reporter.callCount())
	}
	if !reporter.calls[0].success || reporter.calls[0].coordinatorRunID != run.ID {
		t.Errorf("unexpected report call: %+v", reporter.calls[0])
	}

	if got.ReportedAt.IsZero() {
		t.Error("expected reported_at to be set after a successful report (MarkReported)")
	}

	unreported, err := repo.ListUnreportedTerminal(ctx)
	if err != nil {
		t.Fatalf("ListUnreportedTerminal: %v", err)
	}
	for _, r := range unreported {
		if r.ID == run.ID {
			t.Fatalf("expected run %q to be marked reported, still found in unreported list", run.ID)
		}
	}
}
