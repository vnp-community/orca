package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestExecuteTask_RequiresTenantContext(t *testing.T) {
	uc := NewExecuteTask(newFakeTaskRepository(), &fakeEdgeRepository{}, &fakeExecutor{}, &fakeExecutor{})
	_, err := uc.Execute(context.Background(), ExecuteTaskInput{TaskID: "t1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestExecuteTask_SimplePath_NoSubtasksNoDependencies is the core branching
// regression: a task with neither parent_child nor depends_on edges FROM it
// must dispatch to SimpleExecutor, never ComplexExecutor.
func TestExecuteTask_SimplePath_NoSubtasksNoDependencies(t *testing.T) {
	edges := &fakeEdgeRepository{}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc := NewExecuteTask(newFakeTaskRepository(), edges, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	ref, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "infra-fleet-ref-1" {
		t.Errorf("expected the simple executor's ref, got %q", ref)
	}
	if !simple.called {
		t.Error("expected SimpleExecutor to be called")
	}
	if complex.called {
		t.Error("expected ComplexExecutor NOT to be called")
	}
}

func TestExecuteTask_ComplexPath_HasSubtasks(t *testing.T) {
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "task-1", ToTaskID: "subtask-1", Kind: domain.EdgeKindParentChild},
	}}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc := NewExecuteTask(newFakeTaskRepository(), edges, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	ref, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "orchestration-ref-1" {
		t.Errorf("expected the complex executor's ref, got %q", ref)
	}
	if !complex.called {
		t.Error("expected ComplexExecutor to be called")
	}
	if simple.called {
		t.Error("expected SimpleExecutor NOT to be called")
	}
}

func TestExecuteTask_ComplexPath_HasDependencies(t *testing.T) {
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "task-1", ToTaskID: "blocking-task", Kind: domain.EdgeKindDependsOn},
	}}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc := NewExecuteTask(newFakeTaskRepository(), edges, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	ref, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "orchestration-ref-1" {
		t.Errorf("expected the complex executor's ref, got %q", ref)
	}
	if !complex.called || simple.called {
		t.Errorf("expected only ComplexExecutor to be called, complex=%v simple=%v", complex.called, simple.called)
	}
}

func TestExecuteTask_IgnoresEdgesToTheTaskWhenDecidingComplexity(t *testing.T) {
	// task-1 is someone else's dependency (an edge TO it, not FROM it) —
	// that must not make task-1 itself "complex".
	edges := &fakeEdgeRepository{edges: []domain.TaskEdge{
		{FromTaskID: "other-task", ToTaskID: "task-1", Kind: domain.EdgeKindDependsOn},
	}}
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc := NewExecuteTask(newFakeTaskRepository(), edges, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !simple.called || complex.called {
		t.Errorf("expected only SimpleExecutor to be called, simple=%v complex=%v", simple.called, complex.called)
	}
}

func TestExecuteTask_ExecutorFailurePropagates(t *testing.T) {
	edges := &fakeEdgeRepository{}
	simple := &fakeExecutor{err: errors.New("infra-fleet-service unavailable")}
	uc := NewExecuteTask(newFakeTaskRepository(), edges, simple, &fakeExecutor{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err == nil {
		t.Fatal("expected error to propagate from executor failure")
	}
}

func TestExecuteTask_RequiresTaskID(t *testing.T) {
	uc := NewExecuteTask(newFakeTaskRepository(), &fakeEdgeRepository{}, &fakeExecutor{}, &fakeExecutor{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{RequestID: "req-1"}); err == nil {
		t.Fatal("expected an error for an empty task_id")
	}
}

// TestExecuteTask_MarksTaskInProgressBeforeDispatching is the regression for
// this usecase's real state transition (see execute_task.go's doc comment):
// Execute must call TaskRepository.UpdateStatus(StatusInProgress) before
// handing off to either executor.
func TestExecuteTask_MarksTaskInProgressBeforeDispatching(t *testing.T) {
	repo := newFakeTaskRepository()
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	uc := NewExecuteTask(repo, &fakeEdgeRepository{}, simple, &fakeExecutor{})
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.updateStatusCalls) != 1 {
		t.Fatalf("expected exactly one UpdateStatus call, got %d: %+v", len(repo.updateStatusCalls), repo.updateStatusCalls)
	}
	got := repo.updateStatusCalls[0]
	if got.tenantID != "tenant-1" || got.id != "task-1" || got.status != domain.StatusInProgress {
		t.Errorf("unexpected UpdateStatus call: %+v", got)
	}
	if !simple.called {
		t.Error("expected SimpleExecutor to be called after the status update")
	}
}

// TestExecuteTask_StatusUpdateFailurePropagatesAndSkipsDispatch: since the
// status transition is the entire point of this usecase's Epic C addition,
// a failure to persist it must fail Execute outright rather than silently
// dispatching with no recorded state.
func TestExecuteTask_StatusUpdateFailurePropagatesAndSkipsDispatch(t *testing.T) {
	repo := newFakeTaskRepository()
	repo.updateStatusErr = errors.New("db unavailable")
	simple := &fakeExecutor{ref: "infra-fleet-ref-1"}
	complex := &fakeExecutor{ref: "orchestration-ref-1"}
	uc := NewExecuteTask(repo, &fakeEdgeRepository{}, simple, complex)
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, ExecuteTaskInput{TaskID: "task-1", RequestID: "req-1"}); err == nil {
		t.Fatal("expected an error when the status update fails")
	}
	if simple.called || complex.called {
		t.Errorf("expected neither executor to be called, simple=%v complex=%v", simple.called, complex.called)
	}
}
