package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// TestRecalculateProgress_ThreeLevelCascade matches domain/progress_test.go's
// mixed-depth cascade case: a grandparent -> parent -> two leaves (one Done,
// one not) tree recalculates bottom-up to the expected percentages, and
// BatchUpdateProgress is called exactly once (N+1 regression guard).
func TestRecalculateProgress_ThreeLevelCascade(t *testing.T) {
	tenantID := "tenant-1"

	grandparent, _ := domain.NewTask("gp", tenantID, "gp", domain.StatusOpen, "", "")
	parent, _ := domain.NewTask("p", tenantID, "p", domain.StatusOpen, "gp", "")
	leafDone, _ := domain.NewTask("leaf-done", tenantID, "leaf-done", domain.StatusDone, "p", "")
	leafNotDone, _ := domain.NewTask("leaf-not-done", tenantID, "leaf-not-done", domain.StatusOpen, "p", "")

	tasks := newFakeTaskRepository()
	for _, task := range []domain.Task{grandparent, parent, leafDone, leafNotDone} {
		tasks.tasks[task.ID] = task
	}

	uc := NewRecalculateProgress(tasks)
	ctx := withIdentity(context.Background(), tenantID, "user-1")

	rootPercent, err := uc.Execute(ctx, "gp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rootPercent != 50 {
		t.Errorf("expected root percent 50, got %d", rootPercent)
	}

	if len(tasks.batchUpdateProgressCalls) != 1 {
		t.Fatalf("expected exactly 1 BatchUpdateProgress call, got %d", len(tasks.batchUpdateProgressCalls))
	}
	updates := tasks.batchUpdateProgressCalls[0]
	if updates["leaf-done"] != 100 {
		t.Errorf("expected leaf-done=100, got %d", updates["leaf-done"])
	}
	if updates["leaf-not-done"] != 0 {
		t.Errorf("expected leaf-not-done=0, got %d", updates["leaf-not-done"])
	}
	if updates["p"] != 50 {
		t.Errorf("expected p=50, got %d", updates["p"])
	}
	if updates["gp"] != 50 {
		t.Errorf("expected gp=50, got %d", updates["gp"])
	}

	// Persisted, not just returned.
	if got := tasks.tasks["gp"].ProgressPercent; got != 50 {
		t.Errorf("expected persisted gp.ProgressPercent=50, got %d", got)
	}
}

func TestRecalculateProgress_NotFound(t *testing.T) {
	tasks := newFakeTaskRepository()
	uc := NewRecalculateProgress(tasks)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, "missing"); err == nil {
		t.Fatal("expected an error for a missing root task")
	}
}
