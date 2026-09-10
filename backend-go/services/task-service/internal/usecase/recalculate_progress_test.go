package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestRecalculateProgress_UpdatesAncestorCounts(t *testing.T) {
	repo := newFakeTaskRepository()
	root, err := domain.NewTask("root", "tenant-1", "Root", domain.StatusOpen, "", "")
	if err != nil {
		t.Fatalf("building root: %v", err)
	}
	repo.tasks["root"] = root

	leafDone, err := domain.NewTask("leaf-done", "tenant-1", "Leaf done", domain.StatusOpen, "root", "")
	if err != nil {
		t.Fatalf("building leaf-done: %v", err)
	}
	leafDone.Status = domain.StatusDone
	repo.tasks["leaf-done"] = leafDone

	leafOpen, err := domain.NewTask("leaf-open", "tenant-1", "Leaf open", domain.StatusOpen, "root", "")
	if err != nil {
		t.Fatalf("building leaf-open: %v", err)
	}
	repo.tasks["leaf-open"] = leafOpen

	uc := NewRecalculateProgress(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	if err := uc.Execute(ctx, "leaf-done"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := repo.tasks["root"]
	if got.DoneSubtasks != 1 || got.TotalSubtasks != 2 {
		t.Errorf("expected done=1 total=2, got done=%d total=%d", got.DoneSubtasks, got.TotalSubtasks)
	}
}

func TestRecalculateProgress_NoParent_NoOp(t *testing.T) {
	repo := newFakeTaskRepository()
	root, err := domain.NewTask("root", "tenant-1", "Root", domain.StatusOpen, "", "")
	if err != nil {
		t.Fatalf("building root: %v", err)
	}
	repo.tasks["root"] = root

	uc := NewRecalculateProgress(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	if err := uc.Execute(ctx, "root"); err != nil {
		t.Fatalf("expected no error for a root task, got %v", err)
	}
}
