package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestGetSubtree_ReturnsRootAndDescendants(t *testing.T) {
	repo := newFakeTaskRepository()
	root, _ := domain.NewTask("root", "tenant-1", "Root", domain.StatusOpen, "", "")
	child, _ := domain.NewTask("child", "tenant-1", "Child", domain.StatusOpen, "root", "")
	grandchild, _ := domain.NewTask("grandchild", "tenant-1", "Grandchild", domain.StatusOpen, "child", "")
	sibling, _ := domain.NewTask("sibling", "tenant-1", "Sibling of root", domain.StatusOpen, "", "")
	repo.tasks["root"] = root
	repo.tasks["child"] = child
	repo.tasks["grandchild"] = grandchild
	repo.tasks["sibling"] = sibling

	uc := NewGetSubtree(repo)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	got, err := uc.Execute(ctx, "root")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 tasks (root, child, grandchild), got %d: %+v", len(got), got)
	}
	seen := map[string]bool{}
	for _, task := range got {
		seen[task.ID] = true
	}
	if !seen["root"] || !seen["child"] || !seen["grandchild"] {
		t.Errorf("expected root/child/grandchild in result, got %+v", got)
	}
	if seen["sibling"] {
		t.Error("expected sibling to NOT be included in root's subtree")
	}
}

func TestGetSubtree_RequiresTenantContext(t *testing.T) {
	repo := newFakeTaskRepository()
	uc := NewGetSubtree(repo)
	if _, err := uc.Execute(context.Background(), "root"); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
