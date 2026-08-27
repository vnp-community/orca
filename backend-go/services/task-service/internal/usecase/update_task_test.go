package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestUpdateTask_RequiresTenantContext(t *testing.T) {
	uc := NewUpdateTask(newFakeTaskRepository(), &fakeEdgeRepository{})
	if _, err := uc.Execute(context.Background(), UpdateTaskInput{ID: "t1"}); err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestUpdateTask_RejectsTransitionIntoInProgress is TASK-223's core
// regression guard: UpdateTask must never be able to double as an
// execution-completion callback by marking a task in_progress.
func TestUpdateTask_RejectsTransitionIntoInProgress(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", Status: domain.StatusOpen}
	uc := NewUpdateTask(repo, &fakeEdgeRepository{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	status := domain.StatusInProgress
	_, err := uc.Execute(ctx, UpdateTaskInput{ID: "t1", Status: &status})
	if err == nil {
		t.Fatal("expected error: UpdateTask must not be able to transition a task into in_progress")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "TASK_INVALID_STATUS_TRANSITION" {
		t.Fatalf("expected TASK_INVALID_STATUS_TRANSITION, got %v", err)
	}
}

func TestUpdateTask_AllowsOtherTransitions(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", Status: domain.StatusOpen}
	uc := NewUpdateTask(repo, &fakeEdgeRepository{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	status := domain.StatusDone
	got, err := uc.Execute(ctx, UpdateTaskInput{ID: "t1", Status: &status})
	if err != nil {
		t.Fatalf("unexpected error transitioning open -> done: %v", err)
	}
	if got.Status != domain.StatusDone {
		t.Errorf("expected status done, got %q", got.Status)
	}
}

func TestUpdateTask_UpdatesTitleOnly(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", Title: "old", Status: domain.StatusOpen}
	uc := NewUpdateTask(repo, &fakeEdgeRepository{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	newTitle := "new"
	got, err := uc.Execute(ctx, UpdateTaskInput{ID: "t1", Title: &newTitle})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Title != "new" || got.Status != domain.StatusOpen {
		t.Errorf("unexpected task: %+v", got)
	}
}

func TestUpdateTask_NotFound(t *testing.T) {
	uc := NewUpdateTask(newFakeTaskRepository(), &fakeEdgeRepository{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, UpdateTaskInput{ID: "does-not-exist"}); err == nil {
		t.Fatal("expected an error for a nonexistent task")
	}
}

// TestUpdateTask_UnblocksSingleDependencyDependent locks in SOL-TG-01's
// un-block design: a dependent blocked on exactly one dependency transitions
// back to Open the moment that dependency is marked Done via UpdateTask.
func TestUpdateTask_UnblocksSingleDependencyDependent(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["blocker"] = domain.Task{ID: "blocker", TenantID: "tenant-1", Status: domain.StatusOpen}
	repo.tasks["dependent"] = domain.Task{ID: "dependent", TenantID: "tenant-1", Status: domain.StatusBlocked}
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "dependent", ToTaskID: "blocker", Kind: domain.EdgeKindDependsOn},
	}}
	uc := NewUpdateTask(repo, edges)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	status := domain.StatusDone
	if _, err := uc.Execute(ctx, UpdateTaskInput{ID: "blocker", Status: &status}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := repo.tasks["dependent"].Status; got != domain.StatusOpen {
		t.Errorf("expected dependent to be unblocked to open, got %q", got)
	}
}

// TestUpdateTask_MultiDependencyDependentStaysBlockedUntilAllClear is the
// mirror case: a dependent with TWO dependencies stays Blocked until BOTH
// clear, not just the one UpdateTask happens to touch.
func TestUpdateTask_MultiDependencyDependentStaysBlockedUntilAllClear(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.tasks["blocker1"] = domain.Task{ID: "blocker1", TenantID: "tenant-1", Status: domain.StatusOpen}
	repo.tasks["blocker2"] = domain.Task{ID: "blocker2", TenantID: "tenant-1", Status: domain.StatusOpen}
	repo.tasks["dependent"] = domain.Task{ID: "dependent", TenantID: "tenant-1", Status: domain.StatusBlocked}
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "dependent", ToTaskID: "blocker1", Kind: domain.EdgeKindDependsOn},
		{FromTaskID: "dependent", ToTaskID: "blocker2", Kind: domain.EdgeKindDependsOn},
	}}
	uc := NewUpdateTask(repo, edges)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	status := domain.StatusDone
	if _, err := uc.Execute(ctx, UpdateTaskInput{ID: "blocker1", Status: &status}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := repo.tasks["dependent"].Status; got != domain.StatusBlocked {
		t.Errorf("expected dependent to STAY blocked (blocker2 still open), got %q", got)
	}

	if _, err := uc.Execute(ctx, UpdateTaskInput{ID: "blocker2", Status: &status}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := repo.tasks["dependent"].Status; got != domain.StatusOpen {
		t.Errorf("expected dependent to unblock once every dependency cleared, got %q", got)
	}
}
