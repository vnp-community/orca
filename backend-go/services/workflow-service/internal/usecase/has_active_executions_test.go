package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

func TestHasActiveExecutions_TrueForNonTerminalExecution(t *testing.T) {
	repo := newFakeExecutionRepository()
	exec, err := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "project-1")
	if err != nil {
		t.Fatalf("building execution: %v", err)
	}
	_ = repo.CreateExecution(context.Background(), exec)

	uc := NewHasActiveExecutions(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, HasActiveExecutionsInput{ProjectID: "project-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected true for a project with a running execution")
	}
}

func TestHasActiveExecutions_TruePausedExecution(t *testing.T) {
	repo := newFakeExecutionRepository()
	exec, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "project-1")
	exec.Status = domain.StatusPaused
	_ = repo.CreateExecution(context.Background(), exec)

	uc := NewHasActiveExecutions(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, HasActiveExecutionsInput{ProjectID: "project-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected true for a project with a paused execution")
	}
}

func TestHasActiveExecutions_FalseWhenOnlyTerminalExecutions(t *testing.T) {
	repo := newFakeExecutionRepository()
	completed, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "project-1")
	completed.Status = domain.StatusCompleted
	_ = repo.CreateExecution(context.Background(), completed)

	failed, _ := domain.NewWorkflowExecution("exec-2", "tenant-1", "tmpl-1", "trace-2", "project-1")
	failed.Status = domain.StatusFailed
	_ = repo.CreateExecution(context.Background(), failed)

	cancelled, _ := domain.NewWorkflowExecution("exec-3", "tenant-1", "tmpl-1", "trace-3", "project-1")
	cancelled.Status = domain.StatusCancelled
	_ = repo.CreateExecution(context.Background(), cancelled)

	uc := NewHasActiveExecutions(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, HasActiveExecutionsInput{ProjectID: "project-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected false when only terminal-status executions exist for the project")
	}
}

func TestHasActiveExecutions_FalseForUnknownProject(t *testing.T) {
	repo := newFakeExecutionRepository()
	exec, _ := domain.NewWorkflowExecution("exec-1", "tenant-1", "tmpl-1", "trace-1", "project-1")
	_ = repo.CreateExecution(context.Background(), exec)

	uc := NewHasActiveExecutions(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, HasActiveExecutionsInput{ProjectID: "project-does-not-exist"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected false for a project with no executions at all")
	}
}

func TestHasActiveExecutions_RequiresTenantContext(t *testing.T) {
	repo := newFakeExecutionRepository()
	uc := NewHasActiveExecutions(repo)

	_, err := uc.Execute(context.Background(), HasActiveExecutionsInput{ProjectID: "project-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestHasActiveExecutions_RequiresProjectID(t *testing.T) {
	repo := newFakeExecutionRepository()
	uc := NewHasActiveExecutions(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, HasActiveExecutionsInput{ProjectID: ""})
	if err == nil {
		t.Fatal("expected an error when project_id is empty")
	}
}

func TestHasActiveExecutions_RepositoryErrorPropagates(t *testing.T) {
	repo := newFakeExecutionRepository()
	repo.hasActiveExecErr = errors.New("db unavailable")
	uc := NewHasActiveExecutions(repo)
	ctx := withTenantContext(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, HasActiveExecutionsInput{ProjectID: "project-1"})
	if err == nil {
		t.Fatal("expected the repository error to propagate")
	}
}
