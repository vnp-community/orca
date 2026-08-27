package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

func mustTask(t *testing.T, id, coordinatorRunID string, deps []string) domain.OrchestrationTask {
	t.Helper()
	task, err := domain.NewOrchestrationTask(id, "tenant-1", coordinatorRunID, "", "", "title-"+id, nil, deps)
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	return task
}

func TestUpdateTaskStatusAndPromote_RequiresTenantContext(t *testing.T) {
	uc := NewUpdateTaskStatusAndPromote(newFakeOrchestrationTaskRepository(mustTask(t, "t1", "run-1", nil)), &synchronousSerializer{})
	_, err := uc.Execute(context.Background(), UpdateTaskStatusAndPromoteInput{OrchestrationTaskID: "t1", NewStatus: "completed"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestUpdateTaskStatusAndPromote_RejectsInvalidStatus(t *testing.T) {
	uc := NewUpdateTaskStatusAndPromote(newFakeOrchestrationTaskRepository(mustTask(t, "t1", "run-1", nil)), &synchronousSerializer{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, UpdateTaskStatusAndPromoteInput{OrchestrationTaskID: "t1", NewStatus: "bogus"})
	if err == nil {
		t.Fatal("expected an error for an invalid status")
	}
}

// TestUpdateTaskStatusAndPromote_PromotesReadySiblings is the core
// business-logic test for the atomic promote saga (§8): completing a task
// must promote a sibling whose only dependency was that task, in the same
// call.
func TestUpdateTaskStatusAndPromote_PromotesReadySiblings(t *testing.T) {
	root := mustTask(t, "t1", "run-1", nil)
	dependent := mustTask(t, "t2", "run-1", []string{"t1"})
	stillBlocked := mustTask(t, "t3", "run-1", []string{"t1", "t4"})

	repo := newFakeOrchestrationTaskRepository(root, dependent, stillBlocked)
	ser := &synchronousSerializer{}
	uc := NewUpdateTaskStatusAndPromote(repo, ser)

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, UpdateTaskStatusAndPromoteInput{OrchestrationTaskID: "t1", NewStatus: "completed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Task.Status != domain.TaskStatusCompleted {
		t.Errorf("expected t1 status completed, got %s", out.Task.Status)
	}
	if len(out.PromotedTaskIDs) != 1 || out.PromotedTaskIDs[0] != "t2" {
		t.Fatalf("expected only t2 promoted, got %v", out.PromotedTaskIDs)
	}
	if keys := ser.calledKeys(); len(keys) != 1 || keys[0] != "t1" {
		t.Errorf("expected serializer keyed by orchestration_task_id t1, got %v", keys)
	}

	t2, _ := repo.Get(ctx, "tenant-1", "t2")
	if t2.Status != domain.TaskStatusReady {
		t.Errorf("expected t2 promoted to ready, got %s", t2.Status)
	}
	t3, _ := repo.Get(ctx, "tenant-1", "t3")
	if t3.Status != domain.TaskStatusPending {
		t.Errorf("expected t3 to remain pending (dep t4 not completed), got %s", t3.Status)
	}
}

func TestUpdateTaskStatusAndPromote_TaskNotFound(t *testing.T) {
	uc := NewUpdateTaskStatusAndPromote(newFakeOrchestrationTaskRepository(), &synchronousSerializer{})
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, UpdateTaskStatusAndPromoteInput{OrchestrationTaskID: "missing", NewStatus: "completed"})
	if err == nil {
		t.Fatal("expected an error for a missing task")
	}
}
