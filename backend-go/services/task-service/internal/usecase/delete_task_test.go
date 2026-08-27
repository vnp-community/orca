package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestDeleteTask_RequiresTenantContext(t *testing.T) {
	uc := NewDeleteTask(newFakeTaskRepository())
	if err := uc.Execute(context.Background(), DeleteTaskInput{ID: "t1"}); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestDeleteTask_RequiresID(t *testing.T) {
	uc := NewDeleteTask(newFakeTaskRepository())
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	if err := uc.Execute(ctx, DeleteTaskInput{}); err == nil {
		t.Fatal("expected an error for a missing id")
	}
}

func TestDeleteTask_DeletesExistingTask(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1"}
	uc := NewDeleteTask(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, DeleteTaskInput{ID: "t1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.tasks["t1"]; ok {
		t.Error("expected task to be removed from repository")
	}
}

func TestDeleteTask_NotFound(t *testing.T) {
	uc := NewDeleteTask(newFakeTaskRepository())
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if err := uc.Execute(ctx, DeleteTaskInput{ID: "does-not-exist"}); err == nil {
		t.Fatal("expected an error for a nonexistent task")
	}
}
