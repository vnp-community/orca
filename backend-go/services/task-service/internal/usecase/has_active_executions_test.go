package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestHasActiveExecutions_RequiresTenantContext(t *testing.T) {
	uc := NewHasActiveExecutions(newFakeTaskRepository())
	_, err := uc.Execute(context.Background(), HasActiveExecutionsInput{ProjectID: "project-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestHasActiveExecutions_RequiresProjectID(t *testing.T) {
	uc := NewHasActiveExecutions(newFakeTaskRepository())
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, HasActiveExecutionsInput{}); err == nil {
		t.Fatal("expected an error for an empty project_id")
	}
}

func TestHasActiveExecutions_TrueWhenATaskInTheProjectIsInProgress(t *testing.T) {
	repo := newFakeTaskRepository()
	task, err := domain.NewTask("task-1", "tenant-1", "Title", domain.StatusInProgress, "", "project-1")
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	repo.tasks[task.ID] = task
	uc := NewHasActiveExecutions(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, HasActiveExecutionsInput{ProjectID: "project-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected HasActiveExecutions to report true")
	}
}

func TestHasActiveExecutions_FalseWhenOnlyNonActiveStatusesExist(t *testing.T) {
	repo := newFakeTaskRepository()
	for id, status := range map[string]string{
		"task-open":      domain.StatusOpen,
		"task-done":      domain.StatusDone,
		"task-cancelled": domain.StatusCancelled,
	} {
		task, err := domain.NewTask(id, "tenant-1", "Title", status, "", "project-1")
		if err != nil {
			t.Fatalf("building task %s: %v", id, err)
		}
		repo.tasks[id] = task
	}
	uc := NewHasActiveExecutions(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, HasActiveExecutionsInput{ProjectID: "project-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected HasActiveExecutions to report false when no task is in_progress")
	}
}

func TestHasActiveExecutions_FalseForADifferentProject(t *testing.T) {
	repo := newFakeTaskRepository()
	task, err := domain.NewTask("task-1", "tenant-1", "Title", domain.StatusInProgress, "", "project-1")
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	repo.tasks[task.ID] = task
	uc := NewHasActiveExecutions(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, HasActiveExecutionsInput{ProjectID: "project-other"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected HasActiveExecutions to report false for an unrelated project")
	}
}

func TestHasActiveExecutions_RepositoryFailurePropagates(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.hasActiveErr = errors.New("db unavailable")
	uc := NewHasActiveExecutions(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, HasActiveExecutionsInput{ProjectID: "project-1"}); err == nil {
		t.Fatal("expected the repository error to propagate")
	}
}
