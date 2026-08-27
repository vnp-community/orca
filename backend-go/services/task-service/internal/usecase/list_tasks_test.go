package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestListTasks_RequiresTenantContext(t *testing.T) {
	uc := NewListTasks(newFakeTaskRepository())
	if _, err := uc.Execute(context.Background(), ListTasksInput{}); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListTasks_FiltersByTenantAndProject(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1", Title: "a"}
	repo.tasks["t2"] = domain.Task{ID: "t2", TenantID: "tenant-1", ProjectID: "p2", Title: "b"}
	repo.tasks["t3"] = domain.Task{ID: "t3", TenantID: "tenant-2", ProjectID: "p1", Title: "c"}
	uc := NewListTasks(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	result, err := uc.Execute(ctx, ListTasksInput{ProjectID: "p1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].ID != "t1" {
		t.Errorf("expected only t1, got %+v", result.Tasks)
	}
}

func TestListTasks_NoProjectFilter_ReturnsAllTenantTasks(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1"}
	repo.tasks["t2"] = domain.Task{ID: "t2", TenantID: "tenant-1", ProjectID: "p2"}
	uc := NewListTasks(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	result, err := uc.Execute(ctx, ListTasksInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tasks) != 2 {
		t.Errorf("expected both tasks, got %+v", result.Tasks)
	}
}
