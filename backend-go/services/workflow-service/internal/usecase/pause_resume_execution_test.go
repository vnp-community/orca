package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// fakeExecutionRepository is an in-memory ExecutionRepository.
type fakeExecutionRepository struct {
	executions map[string]domain.WorkflowExecution
	getErr     error
	updateErr  error
}

func newFakeExecutionRepository() *fakeExecutionRepository {
	return &fakeExecutionRepository{executions: make(map[string]domain.WorkflowExecution)}
}

func (f *fakeExecutionRepository) CreateExecution(ctx context.Context, exec domain.WorkflowExecution) error {
	f.executions[exec.ID] = exec
	return nil
}

func (f *fakeExecutionRepository) GetExecution(ctx context.Context, tenantID, id string) (domain.WorkflowExecution, error) {
	if f.getErr != nil {
		return domain.WorkflowExecution{}, f.getErr
	}
	e, ok := f.executions[id]
	if !ok || e.TenantID != tenantID {
		return domain.WorkflowExecution{}, domain.ErrExecutionNotFound
	}
	return e, nil
}

func (f *fakeExecutionRepository) UpdateExecution(ctx context.Context, exec domain.WorkflowExecution) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.executions[exec.ID] = exec
	return nil
}

func TestPauseExecution_RunningTransitionsToPaused(t *testing.T) {
	repo := newFakeExecutionRepository()
	exec, err := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1")
	if err != nil {
		t.Fatalf("building execution: %v", err)
	}
	_ = repo.CreateExecution(context.Background(), exec)

	uc := NewPauseExecution(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, PauseExecutionInput{ID: "exec-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.StatusPaused {
		t.Errorf("expected status=paused, got %v", got.Status)
	}
	if got.PausedAt == nil {
		t.Error("expected PausedAt to be set")
	}
	if repo.executions["exec-1"].Status != domain.StatusPaused {
		t.Error("expected the repository's stored execution to reflect the pause")
	}
}

func TestPauseExecution_RejectsNonRunningExecution(t *testing.T) {
	repo := newFakeExecutionRepository()
	exec, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1")
	// Force it into a terminal state a real running execution would reach
	// on its own — pausing from here must be rejected.
	exec.Status = domain.StatusCompleted
	_ = repo.CreateExecution(context.Background(), exec)

	uc := NewPauseExecution(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, PauseExecutionInput{ID: "exec-1"})
	if err == nil {
		t.Fatal("expected an error pausing a non-running execution")
	}
}

func TestResumeExecution_PausedTransitionsToRunning(t *testing.T) {
	repo := newFakeExecutionRepository()
	exec, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1")
	_ = exec.Pause(time.Now().UTC())
	_ = repo.CreateExecution(context.Background(), exec)

	uc := NewResumeExecution(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, ResumeExecutionInput{ID: "exec-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.StatusRunning {
		t.Errorf("expected status=running, got %v", got.Status)
	}
	if got.PausedAt != nil {
		t.Error("expected PausedAt to be cleared on resume")
	}
}

func TestResumeExecution_RejectsNonPausedExecution(t *testing.T) {
	repo := newFakeExecutionRepository()
	// Still running — never paused.
	exec, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1")
	_ = repo.CreateExecution(context.Background(), exec)

	uc := NewResumeExecution(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ResumeExecutionInput{ID: "exec-1"})
	if err == nil {
		t.Fatal("expected an error resuming a non-paused (running) execution — this is the real invariant check")
	}
}

func TestResumeExecution_ExecutionNotFound(t *testing.T) {
	repo := newFakeExecutionRepository()
	uc := NewResumeExecution(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ResumeExecutionInput{ID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected a not-found error")
	}
}
