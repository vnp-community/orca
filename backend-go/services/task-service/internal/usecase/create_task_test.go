package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestCreateTask_RequiresTenantContext(t *testing.T) {
	uc := NewCreateTask(newFakeTaskRepository())
	_, err := uc.Execute(context.Background(), CreateTaskInput{ID: "t1", Title: "Title"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestCreateTask_CreatesARootTask(t *testing.T) {
	repo := newFakeTaskRepository()
	uc := NewCreateTask(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	got, err := uc.Execute(ctx, CreateTaskInput{ID: "t1", Title: "Title"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TenantID != "tenant-1" || got.Status != domain.StatusOpen {
		t.Errorf("unexpected task: %+v", got)
	}
}

func TestCreateTask_RejectsAMissingParent(t *testing.T) {
	repo := newFakeTaskRepository()
	uc := NewCreateTask(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, CreateTaskInput{ID: "t2", Title: "Title", ParentID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected an error for a nonexistent parent")
	}
}

func TestCreateTask_AllowsAnExistingParent(t *testing.T) {
	repo := newFakeTaskRepository()
	uc := NewCreateTask(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, CreateTaskInput{ID: "parent", Title: "Parent"}); err != nil {
		t.Fatalf("unexpected error creating parent: %v", err)
	}
	child, err := uc.Execute(ctx, CreateTaskInput{ID: "child", Title: "Child", ParentID: "parent"})
	if err != nil {
		t.Fatalf("unexpected error creating child: %v", err)
	}
	if child.ParentID != "parent" {
		t.Errorf("expected ParentID=parent, got %q", child.ParentID)
	}
}
