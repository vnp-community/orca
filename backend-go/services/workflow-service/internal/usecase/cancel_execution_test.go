package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func TestCancelExecution_RunningTransitionsToCancelled(t *testing.T) {
	repo := newFakeExecutionRepository()
	exec, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "")
	_ = repo.CreateExecution(context.Background(), exec)

	uc := NewCancelExecution(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, CancelExecutionInput{ID: "exec-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.StatusCancelled {
		t.Errorf("expected status=cancelled, got %v", got.Status)
	}
	if repo.executions["exec-1"].Status != domain.StatusCancelled {
		t.Error("expected the repository's stored execution to reflect the cancellation")
	}
}

func TestCancelExecution_PausedTransitionsToCancelled(t *testing.T) {
	repo := newFakeExecutionRepository()
	exec, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "")
	_ = exec.Pause(time.Now().UTC())
	_ = repo.CreateExecution(context.Background(), exec)

	uc := NewCancelExecution(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, CancelExecutionInput{ID: "exec-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.StatusCancelled {
		t.Errorf("expected status=cancelled, got %v", got.Status)
	}
	if got.PausedAt != nil {
		t.Error("expected PausedAt to be cleared on cancel")
	}
}

func TestCancelExecution_RejectsTerminalExecution(t *testing.T) {
	repo := newFakeExecutionRepository()
	exec, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "")
	exec.Status = domain.StatusCompleted
	_ = repo.CreateExecution(context.Background(), exec)

	uc := NewCancelExecution(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, CancelExecutionInput{ID: "exec-1"})
	if err == nil {
		t.Fatal("expected an error cancelling an already-completed execution")
	}
}

func TestCancelExecution_ExecutionNotFound(t *testing.T) {
	repo := newFakeExecutionRepository()
	uc := NewCancelExecution(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, CancelExecutionInput{ID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected a not-found error")
	}
}
