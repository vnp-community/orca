package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestGetTask_RequiresTenantContext(t *testing.T) {
	uc := NewGetTask(newFakeTaskRepository())
	_, err := uc.Execute(context.Background(), "t1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestGetTask_ReturnsAnExistingTask(t *testing.T) {
	repo := newFakeTaskRepository()
	task, _ := domain.NewTask("t1", "tenant-1", "Title", domain.StatusOpen, "", "")
	repo.tasks["t1"] = task

	uc := NewGetTask(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "t1" {
		t.Errorf("expected task t1, got %+v", got)
	}
}

func TestGetTask_NotFound(t *testing.T) {
	uc := NewGetTask(newFakeTaskRepository())
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, "missing"); err == nil {
		t.Fatal("expected an error for a missing task")
	}
}
